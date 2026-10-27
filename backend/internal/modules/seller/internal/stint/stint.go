// Package stint 提供店家 stint 的鎖定、型別轉換與 website 快取失效，各子領域共用。
package stint

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// Lock 鎖定 stint 序列化它的寫入；已關閉的回 NotFound。
func Lock(ctx context.Context, q *store.Queries, storeID int64) (store.SellerStore, error) {
	if err := model.Positive(storeID); err != nil {
		return store.SellerStore{}, err
	}
	row, err := q.LockStore(ctx, storeID)
	if err != nil {
		return row, model.Error(err)
	}
	if row.Status == string(model.Closed) {
		return row, apperr.NotFound("店家已關閉")
	}
	return row, nil
}

// LockAccountOf 鎖住 storeID 所屬帳號的全部 stint（含已關閉的，依 store_id 的順序）並回傳 storeID 那一列。措施的寫入與
// 停權期間的重算都以帳號為單位、可能碰到同一個帳號的不同 stint，所以一律先以這個固定的順序鎖，兩個流程不會互鎖；
// suspension.Recompute 在鎖內才讀措施。店家不存在回 NotFound。
func LockAccountOf(ctx context.Context, q *store.Queries, storeID int64) (store.SellerStore, error) {
	if err := model.Positive(storeID); err != nil {
		return store.SellerStore{}, err
	}
	// account_id 不會改變，不鎖就讀是安全的。
	owner, err := q.StoreByID(ctx, storeID)
	if err != nil {
		return store.SellerStore{}, model.Error(err)
	}
	rows, err := q.LockAccountStores(ctx, owner.AccountID)
	if err != nil {
		return store.SellerStore{}, err
	}
	for _, row := range rows {
		if row.StoreID == storeID {
			return row, nil
		}
	}
	return store.SellerStore{}, apperr.NotFound("店家不存在")
}

// Effective 是帳號「生效中的措施」的讀取點：帳號任何一個 stint（含已關閉的）在 now 有沒有未撤銷、已開始、沒結束的停權措施
// （定義在 EffectiveMeasures 與 EffectiveMeasuresAll 兩支查詢）。停權狀態（Derive、EffectiveMany）與身分快照的清理
// （PruneIdentities）都用它的結果。
func Effective(ctx context.Context, q *store.Queries, accountID int64, now time.Time) (*model.Sanctions, error) {
	byAccount, err := EffectiveMany(ctx, q, []int64{accountID}, now)
	return byAccount[accountID], err
}

// EffectiveMany 是 Effective 的批次形式（同一支查詢，只差輸入的帳號數，固定一次查詢）：回傳每個帳號生效中的措施，沒有生效中措施
// 的帳號是空集合。可公開曝光的批次判斷與店家的批次讀取用它，不另寫停權的規則。
func EffectiveMany(ctx context.Context, q *store.Queries, accountIDs []int64, now time.Time) (map[int64]*model.Sanctions, error) {
	rows, err := q.EffectiveMeasures(ctx, store.EffectiveMeasuresParams{AccountIds: accountIDs, Now: now})
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*model.Sanctions, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = model.NoSanctions()
	}
	for _, r := range rows {
		add(out, r.AccountID, r.MeasureID, r.Kind, r.TerminationGround, r.Basis, r.RiskGround, r.LeadCategory)
	}
	return out, nil
}

// effectiveAll 問全部的帳號（EffectiveMeasuresAll：只讀 measures，有固定期間與沒有固定期間的各用一個部分索引的 Index Cond 排除已結束與已撤銷的列，讀取量不隨歷史成長），只回傳有生效中措施的帳號。
func effectiveAll(ctx context.Context, q *store.Queries, now time.Time) (map[int64]*model.Sanctions, error) {
	rows, err := q.EffectiveMeasuresAll(ctx, now)
	if err != nil {
		return nil, err
	}
	out := map[int64]*model.Sanctions{}
	for _, r := range rows {
		add(out, r.AccountID, r.MeasureID, r.Kind, r.TerminationGround, r.Basis, r.RiskGround, r.LeadCategory)
	}
	return out, nil
}

func add(out map[int64]*model.Sanctions, accountID int64, measureID uuid.UUID, kind string, terminationGround *string, basis string, riskGround, leadCategory *string) {
	entry := model.EffectiveMeasure{MeasureID: measureID, Kind: model.MeasureKind(kind), Basis: model.Basis(basis)}
	if terminationGround != nil {
		ground := model.TerminationGround(*terminationGround)
		entry.TerminationGround = &ground
	}
	if riskGround != nil {
		ground := model.RiskGround(*riskGround)
		entry.RiskGround = &ground
	}
	if leadCategory != nil {
		category := policy.Category(*leadCategory)
		entry.LeadCategory = &category
	}
	sanctions, ok := out[accountID]
	if !ok {
		sanctions = model.NoSanctions()
	}
	sanctions.Measures = append(sanctions.Measures, entry)
	out[accountID] = sanctions
}

// EffectiveAll 問全部的帳號此刻生效中的措施（EffectiveMeasuresAll），只回傳有生效中措施的帳號；終止合作的對帳用。
func EffectiveAll(ctx context.Context, q *store.Queries, now time.Time) (map[int64]*model.Sanctions, error) {
	return effectiveAll(ctx, q, now)
}

// LockAccounts 依 account_id 升序鎖住這些帳號全部的 stint（每個帳號內依 store_id）。跨帳號的寫入（關聯終止的建立與連帶撤銷）一律走這個固定的順序，
// 兩個流程不會互鎖。
func LockAccounts(ctx context.Context, q *store.Queries, accountIDs []int64) error {
	ids := slices.Clone(accountIDs)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		if _, err := q.LockAccountStores(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// SuspendedAccounts 回傳此刻有生效中措施的全部帳號（EffectiveMeasuresAll）：後台清單的停權篩選與每小時的對帳用，
// 篩選的條件因此和能力檢查是同一個定義。
func SuspendedAccounts(ctx context.Context, q *store.Queries, now time.Time) ([]int64, error) {
	byAccount, err := effectiveAll(ctx, q, now)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(byAccount))
	for id := range byAccount {
		out = append(out, id)
	}
	slices.Sort(out)
	return out, nil
}

// ActiveMeasureIDs 回傳此刻生效中的措施 ID：帳號 accountID 的（0 是全部的帳號，走 EffectiveMeasuresAll）。措施列表的
// state 篩選（ACTIVE、ENDED）用它。
func ActiveMeasureIDs(ctx context.Context, q *store.Queries, accountID int64, now time.Time) ([]uuid.UUID, error) {
	var byAccount map[int64]*model.Sanctions
	var err error
	if accountID == 0 {
		byAccount, err = effectiveAll(ctx, q, now)
	} else {
		byAccount, err = EffectiveMany(ctx, q, []int64{accountID}, now)
	}
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{}
	for _, sanctions := range byAccount {
		ids = append(ids, sanctions.IDs()...)
	}
	return ids, nil
}

// ScheduledMeasureIDs 回傳此刻預定生效（還沒開始、沒有因店家關閉而結束）的措施 ID：帳號 accountID 的（0 是全部的帳號）。措施列表的
// state 篩選（SCHEDULED，以及 ENDED 要排除它們）用它；「預定生效」的定義只有 ScheduledMeasureIDs 那一支查詢。
func ScheduledMeasureIDs(ctx context.Context, q *store.Queries, accountID int64, now time.Time) ([]uuid.UUID, error) {
	var account *int64
	if accountID != 0 {
		account = &accountID
	}
	ids, err := q.ScheduledMeasureIDs(ctx, store.ScheduledMeasureIDsParams{Now: now, AccountID: account})
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, err
}

// Derive 是停權狀態唯一的推導點：帳號任何一個 stint（含已關閉的）在 now 有生效中的措施就是停權。能力檢查、讀店家、開店與
// suspension.Recompute 都經這一個函式；now 是注入的 clock，不是 SQL 的 now()。
func Derive(ctx context.Context, q *store.Queries, accountID int64, now time.Time) (bool, error) {
	sanctions, err := Effective(ctx, q, accountID, now)
	return sanctions.Suspended(), err
}

// PruneIdentities 刪除帳號已結束、已撤銷的措施的身分快照（只留 Effective 回傳的措施的快照）。呼叫方必須已鎖住帳號全部的
// stint（措施的建立與撤銷都先鎖它們，所以讀到生效中的集合到刪除之間不會有新的措施），並回傳這一刻的 Sanctions 給呼叫方用。
func PruneIdentities(ctx context.Context, q *store.Queries, accountID int64, now time.Time) (*model.Sanctions, error) {
	sanctions, err := Effective(ctx, q, accountID, now)
	if err != nil {
		return sanctions, err
	}
	err = q.DeleteIdentitiesOutside(ctx, store.DeleteIdentitiesOutsideParams{AccountID: accountID, Keep: sanctions.IDs()})
	return sanctions, err
}

// Load 把儲存的列轉成公開型別，帶上 now 時推導的停權狀態與生效中的措施（同一次讀取；讀取失敗時回錯誤，呼叫方一律拒絕，
// fail-closed）、能力與曝光的條件（WithConditions），以及核准的食品業者登錄與最近一筆未結束的登錄（AttachFood）。
func Load(ctx context.Context, q *store.Queries, row store.SellerStore, now time.Time) (model.Store, error) {
	sanctions, err := Effective(ctx, q, row.AccountID, now)
	if err != nil {
		return model.Store{}, err
	}
	out := ToStore(row)
	out.Sanctions = sanctions
	one := []model.Store{out}
	if err := WithConditions(ctx, q, one); err != nil {
		return model.Store{}, err
	}
	if err := WithClosures(ctx, q, one); err != nil {
		return model.Store{}, err
	}
	if err := AttachFood(ctx, q, &one[0]); err != nil {
		return model.Store{}, err
	}
	return one[0], nil
}

// ToStore 轉成公開型別。個資已清理的 stint（store_name 是 NULL）StoreName 是空字串，墓碑文字由呈現層依 Status 決定，
// 模組不存也不回傳墓碑字串。
func ToStore(row store.SellerStore) model.Store {
	storeName := ""
	if row.StoreName != nil {
		storeName = *row.StoreName
	}
	out := model.Store{
		Profile: model.Profile{
			StoreName:           storeName,
			Description:         row.Description,
			ContactPhone:        row.ContactPhone,
			ContactName:         row.ContactName,
			ContactMobile:       row.ContactMobile,
			SupportEmail:        row.SupportEmail,
			TownCode:            row.TownCode,
			AddressDetail:       row.AddressDetail,
			Prize:               row.Prize,
			WebsiteURL:          row.WebsiteUrl,
			OnboardingCompleted: row.OnboardingCompleted,
		},
		StoreID:   row.StoreID,
		StoreCode: row.StoreCode,
		AccountID: row.AccountID,
		Version:   row.Version,
		Status:    model.Status(row.Status),
		LogoKey:   row.LogoKey,
		CoverKey:  row.CoverKey,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	return out
}

// WithConditions 在轉好的 stint 上補上能力與曝光的條件（model.Conditions），讀自 seller.store_requirement_flags（條件唯一的家，
// Go 不重算），固定一次查詢。讀店家給能力檢查（model.Authorize、Store.Can、CapabilitiesOf）或曝光說明（MissingExposure）用的路徑都經這裡；
// 讀的是 q 綁著的連線，在呼叫方的 transaction 內就看得到它自己剛寫的列。店家列存在卻沒有條件列表示 view 壞了，回錯誤。
func WithConditions(ctx context.Context, q *store.Queries, stores []model.Store) error {
	if len(stores) == 0 {
		return nil
	}
	ids := make([]int64, len(stores))
	for i, s := range stores {
		ids[i] = s.StoreID
	}
	rows, err := q.StoreRequirementFlags(ctx, ids)
	if err != nil {
		return err
	}
	byStore := make(map[int64]model.Conditions, len(rows))
	for _, r := range rows {
		byStore[r.StoreID] = model.Conditions{
			Derived: true, Active: r.Active, NotPendingDeletion: r.NotPendingDeletion, KYCApproved: r.KycApproved,
			StoreName: r.HasStoreName, Logo: r.HasLogo, TownCode: r.HasTownCode,
		}
	}
	for i := range stores {
		c, ok := byStore[stores[i].StoreID]
		if !ok {
			return fmt.Errorf("seller：店家 %d 沒有條件列（seller.store_requirement_flags）", stores[i].StoreID)
		}
		stores[i].Conditions = c
	}
	return nil
}

// WithClosures 在轉好的 stint 上補上已關閉者目前的關店紀錄（store_closures 目前的那一列，一次查詢）；關店紀錄只有那張表一處，
// 所以讀 stint 要給呼叫方看關店資訊時都經這裡。已關閉而沒有目前紀錄表示不變量被破壞，回錯誤。
func WithClosures(ctx context.Context, q *store.Queries, stores []model.Store) error {
	var ids []int64
	for _, s := range stores {
		if s.Status == model.Closed {
			ids = append(ids, s.StoreID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.CurrentStoreClosures(ctx, ids)
	if err != nil {
		return err
	}
	byStore := make(map[int64]store.SellerStoreClosure, len(rows))
	for _, r := range rows {
		byStore[r.StoreID] = r
	}
	for i := range stores {
		if stores[i].Status != model.Closed {
			continue
		}
		r, ok := byStore[stores[i].StoreID]
		if !ok {
			return fmt.Errorf("seller：已關閉的 stint %d 沒有目前的關店紀錄", stores[i].StoreID)
		}
		stores[i].Closure = &model.Closure{ClosedAt: r.ClosedAt, By: model.ClosedBy(r.ClosedBy), MeasureID: r.ClosedByMeasureID}
	}
	return nil
}

// Invalidate 在呼叫方的 transaction 內排入 website 的快取失效。店家頁的 cache tag 以網址上的 store_id 或
// store_code 為 key（website 的 lib/seo/tags.ts 的 sellerTag），兩個都送。
func Invalidate(ctx context.Context, enqueuer jobs.Enqueuer, row store.SellerStore) error {
	return enqueuer.Enqueue(ctx, revalidate.Args{Tags: revalidate.StoreTags(row.StoreID, row.StoreCode)}, nil)
}

// InvalidateByID 讀出店家再排入快取失效（呼叫方只有 store_id 時用，在同一個 transaction 內）。
func InvalidateByID(ctx context.Context, q *store.Queries, enqueuer jobs.Enqueuer, storeID int64) error {
	row, err := q.StoreByID(ctx, storeID)
	if err != nil {
		return err
	}
	return Invalidate(ctx, enqueuer, row)
}
