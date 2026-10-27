// Package purge 是關店之後的兩段清理（ADR 2026-10-19-seller-kyc-payuni-consolidated「關店與個資保留」與 2026-10-20 修訂）：
//
//   - 第一段（退場，retire）：stint 關閉之後過了 model.StoreRetireAfter（一般關店 30 天；因終止合作而關閉的到終止的申覆期限，且不早於這 30 天），
//     店家的公開內容（說明、獎項、網站）清掉，logo 與封面在這時才釋放，並發出 StoreRetired（relay 轉成 seller.deleted，商品、貼文、聊天的個資等下游
//     在這時刪除）：關店可逆，不可逆的下游動作都在這裡，之後不能恢復營業。食品業者登錄待審的撤回、核准中的結束（STORE_RETIRED，字號留到第二段）。
//   - 第二段（個資清理，purge）：已退場而且關店之後過了 model.StoreRetention（184 天，詐欺犯罪危害防制條例第 37 條第 1 項的六個月），註冊、身分與連線資料
//     在同一個 transaction 內一併清掉：店名、聯絡與地址、KYC 申請的個人欄位與證件、登記資料查詢、PayUni 註冊與觀測的自由文字、食品業者登錄的字號與理由、
//     同意紀錄的 IP 與 User-Agent。審核的事實、PayUni 的狀態碼與措施的身分快照（措施擁有，隨措施撤銷才刪）保留。
//   - 食品業者登錄的審核紀錄保留到關店（store_closures 目前這一筆的 closed_at）後 2 個日曆年再整列刪除。
//
// 時間條件各只有一支查詢（兩段清理是 RetireDueStores 的兩個 stage，審核紀錄是 ExpiredFoodRegistrations），保存條件只有 StoreHolds 一支：申覆的 legal hold 擋兩段清理（也讓
// 恢復營業視為保留期還沒過），主管機關來函的保存擋兩段清理與審核紀錄的刪除（不影響恢復營業）。每一批都是：時間條件選候選並鎖住（FOR UPDATE SKIP LOCKED）
// → 鎖住之後以新的語句再判斷（時間條件以同一支查詢限定在候選上；保存問 StoreHolds）→ 只處理仍然成立而且沒有被保存的。寫入保存條件的一方都先鎖被保護帳號的
// 店家列，所以候選鎖住之後不會再出現新的保存（step10e-3cb §2.4 的證明）。恢復營業（closing.ReopenPlan）問同一支 RetireDueStores 與 StoreHolds 的申覆 hold，
// 所以「恢復或只撤銷紀錄」只由資料決定，不受清理 job 的延遲影響。
//
// 形狀與 buyer 的清理相同：每小時的 River 排程（啟動即執行），以 keyset 分批（被保存而略過的列不會讓下一批一直選到同一批）、每批一個 transaction，
// 迴圈到取不滿為止（沒有每日上限）。stores_retired_check、stores_pii_purged_check 是保底；子表只清店家列已退場或已清理的列。重跑是 no-op。
// media 的引用不在 transaction 內解除：把 logo、封面與證件記成退場並排入引用的同步 job。
package purge

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// FoodReviewRetentionYears 是 stint 關閉之後保留食品業者登錄審核紀錄（不含字號與理由，清理時已清）的年數，期滿整列刪除，
// 綁在這一列的告知同意紀錄隨之刪除（〈食品業者登錄個人資料蒐集告知事項〉第 4 項）。年數照日曆算（台北時區），不是固定的小時數。
const FoodReviewRetentionYears = 2

// FoodReviewCutoff 是刪除審核紀錄的界線：關店時間（store_closures.closed_at）嚴格早於它的店家，審核紀錄期滿。
func FoodReviewCutoff(now time.Time) time.Time {
	return now.In(twtime.Taipei).AddDate(-FoodReviewRetentionYears, 0, 0).UTC()
}

// batchSize 是一個 transaction 清理的店家數上限（批次大小，不是總量上限）。
const batchSize int32 = 100

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Now    func() time.Time
}

// Service 執行個資清理。
type Service struct {
	d Deps
	// hooks 是測試用的同步點（hooks_integration.go，只在 integration build tag 下有內容，正式程式是空的）。
	hooks hooks
}

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Run 依序跑第一段（退場）、第二段（個資清理）與食品登錄審核紀錄的刪除，各自分批到沒有候選為止；回傳第二段清理了個資的店家數
// （退場由 retired_at 看得到，不計入）。
func (s *Service) Run(ctx context.Context) (int, error) {
	purged := 0
	for i, stage := range []func(context.Context, *int64) (int, int, *int64, error){s.retireBatch, s.purgeBatch} {
		var after *int64
		for {
			done, candidates, last, err := stage(ctx, after)
			if i == 1 {
				purged += done
			}
			if err != nil {
				return purged, err
			}
			if candidates < int(batchSize) {
				break
			}
			after = last
		}
	}
	return purged, s.deleteExpiredFoodReviews(ctx)
}

// holds 回傳候選之中沒有被保存擋下的（StoreHolds 是保存條件的唯一定義）；appeal 為 false 時只看來函的保存（審核紀錄的刪除）。
func holds(ctx context.Context, q *store.Queries, ids []int64, now time.Time, appeal bool) (map[int64]bool, error) {
	rows, err := q.StoreHolds(ctx, store.StoreHoldsParams{StoreIds: ids, Now: now})
	if err != nil {
		return nil, err
	}
	held := make(map[int64]bool, len(rows))
	for _, r := range rows {
		held[r.StoreID] = r.PreservationHold || (appeal && r.AppealHold)
	}
	return held, nil
}

// deleteExpiredFoodReviews 每批一個 transaction，以 keyset 迴圈到取不滿為止。每批：ExpiredFoodRegistrations 找候選並鎖住登錄列與店家列，鎖住之後以候選的店家問
// StoreHolds（新的語句），DeleteFoodRegistrations 只刪沒有被來函保存的。時間條件單調（已清理的店家不會恢復），所以不用再判斷一次時間。
func (s *Service) deleteExpiredFoodReviews(ctx context.Context) error {
	var after *uuid.UUID
	for {
		var candidates int
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			now := s.d.Now().UTC()
			rows, err := q.ExpiredFoodRegistrations(ctx, store.ExpiredFoodRegistrationsParams{Cutoff: FoodReviewCutoff(now), BatchLimit: batchSize, AfterID: after})
			if err != nil {
				return err
			}
			candidates = len(rows)
			if len(rows) == 0 {
				return nil
			}
			last := rows[len(rows)-1].ID
			after = &last
			s.hooks.afterFoodCandidates()
			storeIDs := make([]int64, 0, len(rows))
			for _, r := range rows {
				storeIDs = append(storeIDs, r.StoreID)
			}
			held, err := holds(ctx, q, storeIDs, now, false)
			if err != nil {
				return err
			}
			var ids []uuid.UUID
			for _, r := range rows {
				if !held[r.StoreID] {
					ids = append(ids, r.ID)
				}
			}
			if len(ids) == 0 {
				return nil
			}
			_, err = q.DeleteFoodRegistrations(ctx, ids)
			return err
		})
		if err != nil || candidates < int(batchSize) {
			return err
		}
	}
}

// retireBatch 在一個 transaction 內讓一批店家退場（第一段）；回傳這批實際退場的店家數、第一次選到的候選數（滿批次大小才需要再跑一批）與最後一個候選的
// store_id（下一批的 keyset）。
//
// 候選用 RetireDueStores（第一段）選並鎖住，然後以同一支查詢、新的語句限定在候選上再判斷一次時間條件，再以 StoreHolds 判斷保存：第一個語句開始之後、掃到那一列
// 之前才 commit 的寫入（READ COMMITTED 下第一個語句的快照看不到它，那一列也沒有被它鎖）在之後的語句看得到。
func (s *Service) retireBatch(ctx context.Context, after *int64) (int, int, *int64, error) {
	retiredCount, candidateCount := 0, 0
	var last *int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		params := store.RetireDueStoresParams{
			Now: now, AppealWindowHours: model.AppealWindowHours, Hours: model.StoreRetireAfterHours, BatchLimit: batchSize, AfterStoreID: after,
		}
		due, err := q.RetireDueStores(ctx, params)
		if err != nil {
			return err
		}
		if len(due) == 0 {
			return nil
		}
		candidateCount = len(due)
		lastID := due[len(due)-1].StoreID
		last = &lastID
		s.hooks.afterCandidates()
		candidates := make([]int64, len(due))
		for i, row := range due {
			candidates[i] = row.StoreID
		}
		params.StoreIds, params.AfterStoreID = candidates, nil
		if due, err = q.RetireDueStores(ctx, params); err != nil {
			return err
		}
		held, err := holds(ctx, q, candidates, now, true)
		if err != nil {
			return err
		}
		candidates = candidates[:0]
		byID := make(map[int64]store.RetireDueStoresRow, len(due))
		for _, row := range due {
			if held[row.StoreID] {
				continue
			}
			candidates = append(candidates, row.StoreID)
			byID[row.StoreID] = row
		}
		if len(candidates) == 0 {
			return nil
		}
		retired, err := q.RetireStores(ctx, store.RetireStoresParams{StoreIds: candidates, Now: &now})
		if err != nil {
			return err
		}
		retiredCount = len(retired)
		// 說明與圖片清掉之後店家頁（與卡片）要失效：同一個 transaction 內排入快取失效；logo 與封面在這時才釋放（記成退場，引用由同步 job 解除）。
		// 退場之後恢復營業不可能，下游的資料可以刪除：發出 StoreRetired。
		for _, id := range retired {
			row := byID[id]
			if err := s.d.Jobs.Enqueue(ctx, revalidate.Args{Tags: revalidate.StoreTags(row.StoreID, row.StoreCode)}, nil); err != nil {
				return err
			}
			for _, image := range []struct {
				refType string
				asset   *string
			}{{media.RefTypeSellerLogo, row.LogoAssetID}, {media.RefTypeSellerCover, row.CoverAssetID}} {
				var released []string
				if image.asset != nil {
					released = append(released, *image.asset)
				}
				if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.StoreRef(image.refType, row.StoreID), released...); err != nil {
					return err
				}
			}
			if err := events.Publish(ctx, s.d.Events, model.StoreRetired{
				EventID: uuid.NewV7(), OccurredAt: now, StoreID: row.StoreID, StoreCode: row.StoreCode, AccountID: row.AccountID,
			}); err != nil {
				return err
			}
		}
		// 退場的店家不會再重開：待審的食品業者登錄撤回，仍核准中的結束（STORE_RETIRED）；字號、理由與內部註記留到第二段清。
		if _, err := q.WithdrawPendingFoodRegistrations(ctx, store.WithdrawPendingFoodRegistrationsParams{StoreIds: retired, Now: now}); err != nil {
			return err
		}
		_, err = q.RetireApprovedFoodRegistrations(ctx, store.RetireApprovedFoodRegistrationsParams{StoreIds: retired, Now: now})
		return err
	})
	return retiredCount, candidateCount, last, err
}

// purgeBatch 在一個 transaction 內清理一批已退場店家的註冊、身分與連線資料（第二段）；回傳值與鎖住之後再判斷的做法同 retireBatch。
func (s *Service) purgeBatch(ctx context.Context, after *int64) (int, int, *int64, error) {
	purgedCount, candidateCount := 0, 0
	var last *int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		params := store.RetireDueStoresParams{
			SecondStage: true, Now: now, AppealWindowHours: model.AppealWindowHours, Hours: model.StoreRetentionHours, BatchLimit: batchSize, AfterStoreID: after,
		}
		rows, err := q.RetireDueStores(ctx, params)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		candidateCount = len(rows)
		lastID := rows[len(rows)-1].StoreID
		last = &lastID
		s.hooks.afterPurgeCandidates()
		due := make([]int64, len(rows))
		for i, r := range rows {
			due[i] = r.StoreID
		}
		params.StoreIds, params.AfterStoreID = due, nil
		rechecked, err := q.RetireDueStores(ctx, params)
		if err != nil {
			return err
		}
		recheck := make([]int64, len(rechecked))
		for i, r := range rechecked {
			recheck[i] = r.StoreID
		}
		held, err := holds(ctx, q, due, now, true)
		if err != nil {
			return err
		}
		var candidates []int64
		for _, id := range recheck {
			if !held[id] {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		cleared, err := q.PurgeStores(ctx, store.PurgeStoresParams{StoreIds: candidates, Now: &now})
		if err != nil {
			return err
		}
		purgedCount = len(cleared)
		purged := make([]int64, len(cleared))
		// 店名清掉之後卡片沒有店名：同一個 transaction 內排入店家頁（與卡片）的快取失效。
		for i, r := range cleared {
			purged[i] = r.StoreID
			if err := s.d.Jobs.Enqueue(ctx, revalidate.Args{Tags: revalidate.StoreTags(r.StoreID, r.StoreCode)}, nil); err != nil {
				return err
			}
		}
		if _, err := q.PurgeFoodRegistrations(ctx, store.PurgeFoodRegistrationsParams{StoreIds: purged, Now: &now}); err != nil {
			return err
		}
		applications, err := q.PurgeKYCApplications(ctx, store.PurgeKYCApplicationsParams{StoreIds: purged, Now: &now})
		if err != nil {
			return err
		}
		if err := q.PurgeKYCIdentityHashes(ctx, applications); err != nil {
			return err
		}
		// 登記資料的查詢快照含公司名稱與地址，與申請的個資同時清除；審核紀錄的引用由外鍵（ON DELETE SET NULL）置 NULL。
		if _, err := q.PurgeKYCRegistrationChecks(ctx, applications); err != nil {
			return err
		}
		deleted, err := q.PurgeKYCDocuments(ctx, applications)
		if err != nil {
			return err
		}
		retired := make(map[uuid.UUID][]string, len(applications))
		for _, d := range deleted {
			retired[d.ApplicationID] = append(retired[d.ApplicationID], d.AssetID)
		}
		for _, id := range applications {
			if len(retired[id]) == 0 {
				continue
			}
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.CertificationRef(id), retired[id]...); err != nil {
				return err
			}
		}
		if _, err := q.PurgePayUniRegistrations(ctx, store.PurgePayUniRegistrationsParams{StoreIds: purged, Now: &now}); err != nil {
			return err
		}
		// 入駐同意紀錄的來源 IP 與 User-Agent 是連線紀錄，清掉；種類、版本、時間、帳號與送出保留（審核的事實）。
		if _, err := q.PurgeAgreementSources(ctx, store.PurgeAgreementSourcesParams{StoreIds: purged, Now: &now}); err != nil {
			return err
		}
		_, err = q.PurgePayUniObservations(ctx, purged)
		return err
	})
	return purgedCount, candidateCount, last, err
}
