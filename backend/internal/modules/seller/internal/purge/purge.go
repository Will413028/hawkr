// Package purge 是關店之後的兩段清理（ADR 2026-11-13-seller-platform-review-and-identity-matching「關店與個資保留」）：
//
//   - 第一段（退場，retire）：stint 關閉之後過了 model.StoreRetireAfter（一般關店 30 天；因終止合作而關閉的到終止的申覆期限，且不早於這 30 天），
//     店家的公開內容（說明、獎項、網站）清掉，logo 與封面在這時才釋放，並發出 StoreRetired（relay 轉成 seller.deleted，商品、貼文、聊天的個資等下游
//     在這時刪除）：關店可逆，不可逆的下游動作都在這裡，之後不能恢復營業。
//   - 第二段（個資清理，purge）：已退場而且關店之後過了 model.StoreRetention（184 天，詐欺犯罪危害防制條例第 37 條第 1 項的六個月），註冊、身分與連線資料
//     在同一個 transaction 內一併清掉：店名、聯絡與地址、KYC 申請的個人欄位與證件、登記資料查詢、
//     同意紀錄的 IP 與 User-Agent；收款帳戶由 payment 擁有，同一個 transaction 排入它的刪除。審核的事實與措施的身分快照（措施擁有，隨措施撤銷才刪）保留。
//   - 防詐保存的過期刪除（第三項，不是清理的一段）：account_identity_hashes（身分的 blind index，沒有明文，D3）不隨第二段刪除；來源 stint 目前這次關店
//     滿 model.IdentityHashRetention（5 年）後刪，被主管機關來函的保存或申覆的 legal hold 擋下的略過（StoreHolds）。
//
// 時間條件只有一支查詢（兩段清理是 RetireDueStores 的兩個 stage），保存條件只有 StoreHolds 一支：申覆的 legal hold 擋兩段清理（也讓
// 恢復營業視為保留期還沒過），主管機關來函的保存擋兩段清理（不影響恢復營業）。每一批都是：時間條件選候選並鎖住（FOR UPDATE SKIP LOCKED）
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

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

const batchSize int32 = 100

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	// Media 是 media 模組：退場與個資清理的 transaction 內把店家的圖與證件 MarkDeleted。
	Media model.Media
	// Payment 是收款帳戶的擁有者（payment 模組）：清理的 transaction 內排入刪除店家收款帳戶列的 job。
	Payment Payment
	Now     func() time.Time
}

// Payment 是 payment 模組的公開 API 中清理用到的部分。
type Payment interface {
	ForgetOwners(ctx context.Context, storeIDs []int64) error
}

// Service 執行個資清理。
type Service struct {
	d     Deps
	hooks hooks
}

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Run 依序跑第一段（退場）、第二段（個資清理）與防詐保存的過期刪除，各自分批到沒有候選為止；回傳第二段清理了個資的店家數
// （退場由 retired_at 看得到，不計入）。
func (s *Service) Run(ctx context.Context) (int, error) {
	purged := 0
	for i, stage := range []func(context.Context, *int64) (int, int, *int64, error){s.retireBatch, s.purgeBatch, s.expireIdentityHashBatch} {
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
	return purged, nil
}

func holds(ctx context.Context, q *store.Queries, ids []int64, now time.Time) (map[int64]bool, error) {
	rows, err := q.StoreHolds(ctx, store.StoreHoldsParams{StoreIds: ids, Now: now})
	if err != nil {
		return nil, err
	}
	held := make(map[int64]bool, len(rows))
	for _, r := range rows {
		held[r.StoreID] = r.PreservationHold || r.AppealHold
	}
	return held, nil
}

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
		held, err := holds(ctx, q, candidates, now)
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
		for _, id := range retired {
			row := byID[id]
			if err := s.d.Jobs.Enqueue(ctx, revalidate.Args{Tags: revalidate.StoreTags(row.StoreID, row.StoreCode)}, nil); err != nil {
				return err
			}
			var released []uuid.UUID
			for _, asset := range []*uuid.UUID{row.LogoAssetID, row.CoverAssetID} {
				if asset != nil {
					released = append(released, *asset)
				}
			}
			if len(released) > 0 {
				if err := s.d.Media.MarkDeleted(ctx, released); err != nil {
					return err
				}
			}
			if err := events.Publish(ctx, s.d.Events, model.StoreRetired{
				EventID: uuid.NewV7(), OccurredAt: now, StoreID: row.StoreID, StoreCode: row.StoreCode, AccountID: row.AccountID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return retiredCount, candidateCount, last, err
}

func (s *Service) purgeBatch(ctx context.Context, after *int64) (int, int, *int64, error) {
	purgedCount, candidateCount := 0, 0
	var last *int64
	s.hooks.beforePurgeBatch()
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
		held, err := holds(ctx, q, due, now)
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
		for i, r := range cleared {
			purged[i] = r.StoreID
			if err := s.d.Jobs.Enqueue(ctx, revalidate.Args{Tags: revalidate.StoreTags(r.StoreID, r.StoreCode)}, nil); err != nil {
				return err
			}
		}
		applications, err := q.PurgeKYCApplications(ctx, store.PurgeKYCApplicationsParams{StoreIds: purged, Now: &now})
		if err != nil {
			return err
		}
		if _, err := q.PurgeKYCRegistrationChecks(ctx, applications); err != nil {
			return err
		}
		deleted, err := q.PurgeKYCDocuments(ctx, applications)
		if err != nil {
			return err
		}
		retired := make([]uuid.UUID, 0, len(deleted))
		for _, d := range deleted {
			retired = append(retired, d.AssetID)
		}
		if len(retired) > 0 {
			if err := s.d.Media.MarkDeleted(ctx, retired); err != nil {
				return err
			}
		}
		if _, err := q.ClearContentSnapshotsOfStores(ctx, purged); err != nil {
			return err
		}
		if _, err := q.PurgeAgreementSources(ctx, store.PurgeAgreementSourcesParams{StoreIds: purged, Now: &now}); err != nil {
			return err
		}
		return s.d.Payment.ForgetOwners(ctx, purged)
	})
	return purgedCount, candidateCount, last, err
}

func (s *Service) expireIdentityHashBatch(ctx context.Context, after *int64) (int, int, *int64, error) {
	var deleted, candidates int
	var last *int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		ids, err := q.ExpiredIdentityHashStores(ctx, store.ExpiredIdentityHashStoresParams{
			Now: now, Hours: model.IdentityHashRetentionHours, AfterStoreID: after, BatchLimit: batchSize,
		})
		if err != nil {
			return err
		}
		candidates = len(ids)
		if candidates == 0 {
			return nil
		}
		last = &ids[candidates-1]
		held, err := holds(ctx, q, ids, now)
		if err != nil {
			return err
		}
		var due []int64
		for _, id := range ids {
			if !held[id] {
				due = append(due, id)
			}
		}
		if len(due) == 0 {
			return nil
		}
		if _, err := q.DeleteExpiredAccountIdentityHashes(ctx, store.DeleteExpiredAccountIdentityHashesParams{StoreIds: due, Now: now, Hours: model.IdentityHashRetentionHours}); err != nil {
			return err
		}
		deleted = len(due)
		return nil
	})
	return deleted, candidates, last, err
}
