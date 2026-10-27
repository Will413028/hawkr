// Package suspension 是措施造成的停權的推導與檢查（ADR 2026-09-29-seller-suspension-and-business-identity-in-monolith、
// 2026-10-18-seller-governance-consolidated）：能力檢查、停權期間與事件的維護。措施本身的寫入在 measure 套件。到期與對帳
// 的 job 在 jobs.go。
package suspension

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.Enqueuer
	Exposure *exposure.Service
	Now      func() time.Time
}

// Service 是停權的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// ActingStore 回傳帳號目前的店家，只在它有能力 c 時成功（能力由 model.CapabilitiesOf 推導，停權狀態由 stint.Derive 在
// 這一刻以處分推導）：停權造成的拒絕回 PermissionDenied，等待刪除造成的回 FailedPrecondition，沒有店家回 NotFound。
// 每次讀 DB，不快取；讀取失敗時回錯誤，呼叫方一律拒絕（fail-closed），不得把錯誤當成有能力。
func (s *Service) ActingStore(ctx context.Context, accountID int64, c model.Capability) (model.Store, error) {
	if err := model.Positive(accountID); err != nil {
		return model.Store{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.CurrentStore(ctx, accountID)
	if err != nil {
		return model.Store{}, model.Error(err)
	}
	current, err := stint.Load(ctx, q, row, s.d.Now().UTC())
	if err != nil {
		return model.Store{}, err
	}
	if err := model.Authorize(current, c); err != nil {
		return model.Store{}, err
	}
	return current, nil
}

// RecordIdentities 在停權措施建立的 transaction 內，把被處分帳號全部 KYC 申請（跨 stint）的身分 blind index 快照到措施
// 擁有的 measure_identities（ADR 2026-10-19-seller-kyc-payuni-consolidated）：之後別的帳號以同一個身分申請認證時，
// 由快照比對。措施結束或撤銷時由 Recompute 刪除。呼叫方已鎖住帳號。
func RecordIdentities(ctx context.Context, q *store.Queries, measureID uuid.UUID, accountID int64) error {
	return q.SnapshotMeasureIdentities(ctx, store.SnapshotMeasureIdentitiesParams{MeasureID: measureID, AccountID: accountID})
}

// Recompute 讓帳號的停權期間與措施一致，並發出對應的事件；停權狀態本身不存，每次讀取時由 stint.Derive 推導，這裡只
// 負責「期間」與 StoreSuspended／StoreUnsuspended 兩個副作用（只有這裡寫 suspension_periods）。
//
// 先鎖住帳號全部的 stint（依 store_id 的順序；呼叫方通常已經鎖過），鎖住之後才讀措施：先讀再鎖，會讓兩個同時進行
// 的重算各自拿著對方還沒 commit 之前的措施判斷，把生效中的停權結束掉（CR-1）。
//
// 帳號在 now 有生效中的措施時，期間記在帳號目前的 stint 上；沒有目前的 stint（所有 stint 都關閉了）時，沿用已經進行
// 中的那一段，沒有進行中的就不開始（沒有店面可以下架，停權照樣由措施擋下開店）。其他 stint 上進行中的期間（措施結束、
// 期間還沒被 job 結束時開了新店）結束並發 StoreUnsuspended。帳號沒有生效中的措施時結束全部進行中的期間，發帶同一個
// PeriodID 的 StoreUnsuspended。持續停權中措施的新增、結束或撤銷（只要還有一個生效中）是同一段期間，不發事件。呼叫方在
// transaction 內。
func Recompute(ctx context.Context, q *store.Queries, bus *events.Bus, enqueuer jobs.Enqueuer, ex *exposure.Service, accountID int64, now time.Time) error {
	stores, err := q.LockAccountStores(ctx, accountID)
	if err != nil {
		return err
	}
	if len(stores) == 0 {
		return nil
	}
	if err := recomputePeriods(ctx, q, bus, enqueuer, stores, accountID, now); err != nil {
		return err
	}
	// 停權是帳號的可曝光輸入：帳號還沒關閉的店家（鎖順序：店家列在前，公告列最後）在措施生效或結束後重新公告，
	// 以重算期間用的同一個時間點（now）判斷停權，期間與公告不會對同一刻有兩種答案。
	var live []int64
	for _, row := range stores {
		if row.Status != string(model.Closed) {
			live = append(live, row.StoreID)
		}
	}
	return ex.Announce(ctx, q, now, live...)
}

// recomputePeriods 是 Recompute 的期間與事件部分；stores 是已鎖住的帳號全部的 stint。
func recomputePeriods(ctx context.Context, q *store.Queries, bus *events.Bus, enqueuer jobs.Enqueuer, stores []store.SellerStore, accountID int64, now time.Time) error {
	// 已撤銷或已結束的措施不再需要它的身分快照（撤銷、結束、到期 job 與對帳都經這裡，在同一個 transaction 刪除）；生效中的
	// 措施與停權狀態來自同一次 stint.Effective。
	sanctions, err := stint.PruneIdentities(ctx, q, accountID, now)
	if err != nil {
		return err
	}
	suspended := sanctions.Suspended()
	open, err := q.OpenSuspensionPeriods(ctx, accountID)
	if err != nil {
		return err
	}
	byID := make(map[int64]store.SellerStore, len(stores))
	var holder *store.SellerStore // 期間應該在的 stint
	for i, row := range stores {
		byID[row.StoreID] = row
		if row.Status != string(model.Closed) {
			holder = &stores[i]
		}
	}
	if suspended && holder == nil && len(open) > 0 {
		kept := byID[open[0].StoreID]
		holder = &kept
	}
	hasOpen := false
	for _, period := range open {
		if suspended && holder != nil && period.StoreID == holder.StoreID {
			hasOpen = true
			continue
		}
		if err := endPeriod(ctx, q, bus, enqueuer, byID[period.StoreID], period, now); err != nil {
			return err
		}
	}
	if !suspended || holder == nil || hasOpen {
		return nil
	}
	period, err := q.StartSuspensionPeriod(ctx, store.StartSuspensionPeriodParams{StoreID: holder.StoreID, Now: now})
	if err != nil {
		return fmt.Errorf("seller：開始停權期間：%w", err)
	}
	err = events.Publish(ctx, bus, model.StoreSuspended{
		EventID: uuid.NewV7(), OccurredAt: now, StoreID: holder.StoreID, AccountID: holder.AccountID, PeriodID: period.PeriodID,
	})
	if err != nil {
		return err
	}
	return stint.Invalidate(ctx, enqueuer, *holder)
}

// endPeriod 結束一段進行中的停權期間，發 StoreUnsuspended 並排入快取失效。
func endPeriod(ctx context.Context, q *store.Queries, bus *events.Bus, enqueuer jobs.Enqueuer, row store.SellerStore, period store.SellerSuspensionPeriod, now time.Time) error {
	ended, err := q.EndSuspensionPeriod(ctx, store.EndSuspensionPeriodParams{PeriodID: period.PeriodID, Now: &now})
	if err != nil {
		return fmt.Errorf("seller：結束停權期間：%w", err)
	}
	err = events.Publish(ctx, bus, model.StoreUnsuspended{
		EventID: uuid.NewV7(), OccurredAt: now, StoreID: row.StoreID, AccountID: row.AccountID, PeriodID: ended.PeriodID,
	})
	if err != nil {
		return err
	}
	return stint.Invalidate(ctx, enqueuer, row)
}
