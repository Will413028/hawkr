package closing

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/suspension"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// 終止合作生效之後的關店（賣家合作條款第 21.4 條：終止後店家暫停營業，未結訂單完成後關閉）。「待關閉」不存：它就是生效中的終止合作加 stint 還沒關閉。
// 這個 job 只在生效時（starts_at）把停權的效果對齊（第一次發 StoreSuspended）並嘗試關店；有未結訂單就結束這一次，order.completed 的事件加速一次
// （AdvanceStore）、每小時的掃描（ReconcileTerminations）保底，所以 job 遺失或被去重掉都只是晚關，方向安全。

// TerminationEffectArgs 是終止合作生效的 job，以 measure_id 為唯一鍵（重跑沒有副作用：每一步都重讀並重新判斷）。
type TerminationEffectArgs struct {
	MeasureID uuid.UUID `json:"measure_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (TerminationEffectArgs) Kind() string { return "seller.termination_effect" }

// terminationStates 是唯一鍵涵蓋的狀態：還沒有結果的（排隊中、執行中、等重試）。running 不能排除（River 的規定），所以執行期間被去重掉的加速排入
// 由每小時的掃描補上，延遲不超過一小時，方向是「晚關」。不含 completed：完成的 job 在 River 清掉之前會擋下之後同一個鍵的排入。
func terminationStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	}
}

// ScheduleTermination 在 ctx 的 transaction 內排入排在 startsAt 的終止生效 job；同一個措施同時最多一個排隊中的 job。
func ScheduleTermination(ctx context.Context, enqueuer jobs.Enqueuer, measureID uuid.UUID, startsAt time.Time) error {
	return enqueuer.Enqueue(ctx, TerminationEffectArgs{MeasureID: measureID}, &river.InsertOpts{
		ScheduledAt: startsAt,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: terminationStates()},
	})
}

// EffectTermination 是 job 的內容，三步：
//  1. transaction：鎖帳號全部的 stint，重讀措施（已撤銷或已結束就結束），以 max(starts_at, 現在) 重算停權期間（第一次發 StoreSuspended）。
//  2. transaction 外問未結訂單（fail-closed）；有未結訂單就結束這一次。
//  3. 關店的 transaction：identity 帳號鎖 → 鎖帳號全部的 stint → 重讀措施並重新判斷（未撤銷、生效中）、stint 還是第 1 步的那一個而且不在等待刪除，
//     才以 TERMINATION 關店；任一不成立就不關。
//
// 帳號在等待刪除時視為終局、什麼都不做，由刪帳以 ACCOUNT_DELETION 關店。判斷時間是 max(starts_at, 現在)：worker 的時鐘可能比寫入措施的時鐘慢，
// job 在 starts_at 被取出時 now 還在 starts_at 之前，以 now 判斷會把剛生效的措施當成還沒開始。
func (s *Service) EffectTermination(ctx context.Context, measureID uuid.UUID) error {
	storeID, err := s.effect(ctx, measureID)
	if err != nil || storeID == 0 {
		return err
	}
	open, err := s.d.Orders.HasOpenStoreOrders(ctx, storeID)
	if err != nil {
		return fmt.Errorf("seller：確認店家 %d 的未結訂單：%w", storeID, err)
	}
	if open {
		return nil
	}
	return s.closeTerminated(ctx, measureID, storeID)
}

// effect 是第 1 步；回傳要接著查訂單的 stint（沒有事要做時是 0）。
func (s *Service) effect(ctx context.Context, measureID uuid.UUID) (int64, error) {
	var storeID int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		m, err := q.MeasureByID(ctx, measureID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := stint.LockAccountOf(ctx, q, m.StoreID); err != nil {
			return err
		}
		m, err = q.LockMeasure(ctx, measureID)
		if err != nil {
			return err
		}
		if m.Kind != string(model.MeasureTermination) {
			return fmt.Errorf("seller：措施 %s 不是終止合作", measureID)
		}
		at := laterOf(m.StartsAt, s.now())
		if stateOf(m, at) != model.StateActive {
			return nil
		}
		stores, err := q.LockAccountStores(ctx, m.AccountID)
		if err != nil {
			return err
		}
		if current, ok := currentOf(stores); ok && current.Status == string(model.PendingDeletion) {
			return nil
		}
		if err := suspension.Recompute(ctx, q, s.d.Events, s.d.Jobs, s.d.Exposure, m.AccountID, at); err != nil {
			return err
		}
		if current, ok := currentOf(stores); ok && current.Status == string(model.Active) {
			storeID = current.StoreID
		}
		return nil
	})
	return storeID, err
}

// closeTerminated 是第 3 步。
func (s *Service) closeTerminated(ctx context.Context, measureID uuid.UUID, checked int64) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		m, err := q.MeasureByID(ctx, measureID)
		if err != nil {
			return err
		}
		if err := s.d.Identity.LockAccount(ctx, m.AccountID); err != nil {
			return err
		}
		stores, err := q.LockAccountStores(ctx, m.AccountID)
		if err != nil {
			return err
		}
		if m, err = q.LockMeasure(ctx, measureID); err != nil {
			return err
		}
		if stateOf(m, laterOf(m.StartsAt, s.now())) != model.StateActive {
			return nil
		}
		current, ok := currentOf(stores)
		if !ok || current.StoreID != checked || current.Status != string(model.Active) {
			return nil
		}
		return s.close(ctx, q, current, closer{by: model.ClosedByTermination, measureID: &m.ID})
	})
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func stateOf(m store.SellerMeasure, now time.Time) model.MeasureState {
	return model.StateOf(model.MeasureStatus(m.Status), model.MeasureKind(m.Kind), m.StartsAt, m.EndsAt, now)
}

// AdvanceStore 是 order.completed 的加速：店家所屬帳號生效中的終止合作各跑一次 EffectTermination。事件只加速收斂，每小時的掃描是保底；
// 沒有生效中的終止合作時什麼都不做。
func (s *Service) AdvanceStore(ctx context.Context, storeID int64) error {
	owner, err := store.New(s.d.DB.Conn(ctx)).StoreByID(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	sanctions, err := stint.Effective(ctx, store.New(s.d.DB.Conn(ctx)), owner.AccountID, s.now())
	if err != nil {
		return err
	}
	return s.effectAll(ctx, sanctions)
}

func (s *Service) effectAll(ctx context.Context, sanctions *model.Sanctions) error {
	var errs []error
	for _, m := range sanctions.Measures {
		if m.Kind != model.MeasureTermination {
			continue
		}
		if err := s.EffectTermination(ctx, m.MeasureID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ReconcileTerminations 是每小時的保底：此刻生效中的終止合作而帳號還有目前的 stint 的，逐一跑 EffectTermination（遺失的 job、被去重掉的加速、
// 排程前就生效的措施都由它補上）；回傳掃描的措施數。個別失敗不影響其他的，錯誤合併回傳讓 river 重試。
func (s *Service) ReconcileTerminations(ctx context.Context) (int, error) {
	q := store.New(s.d.DB.Conn(ctx))
	byAccount, err := stint.EffectiveAll(ctx, q, s.now())
	if err != nil {
		return 0, err
	}
	scanned := 0
	var errs []error
	for accountID, sanctions := range byAccount {
		terminations := 0
		for _, m := range sanctions.Measures {
			if m.Kind == model.MeasureTermination {
				terminations++
			}
		}
		if terminations == 0 {
			continue
		}
		if _, err := q.CurrentStore(ctx, accountID); errors.Is(err, pgx.ErrNoRows) {
			continue
		} else if err != nil {
			errs = append(errs, err)
			continue
		}
		scanned += terminations
		if err := s.effectAll(ctx, sanctions); err != nil {
			errs = append(errs, fmt.Errorf("account %d: %w", accountID, err))
		}
	}
	return scanned, errors.Join(errs...)
}

// ReconcileTerminationsArgs 是終止合作的對帳。
type ReconcileTerminationsArgs struct{}

// Kind 是 River 的 job 種類。
func (ReconcileTerminationsArgs) Kind() string { return "seller.reconcile_terminations" }

// InsertOpts 把同一小時的排程與重啟合併。
func (ReconcileTerminationsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Hour}}
}

type effectWorker struct {
	river.WorkerDefaults[TerminationEffectArgs]
	service *Service
}

// Work 執行終止合作生效。
func (w *effectWorker) Work(ctx context.Context, job *river.Job[TerminationEffectArgs]) error {
	return w.service.EffectTermination(ctx, job.Args.MeasureID)
}

// Timeout 是一次執行的時間上限（含遠端的訂單查詢）。
func (w *effectWorker) Timeout(*river.Job[TerminationEffectArgs]) time.Duration {
	return 2 * time.Minute
}

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileTerminationsArgs]
	service *Service
}

// Work 執行對帳。
func (w *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileTerminationsArgs]) error {
	_, err := w.service.ReconcileTerminations(ctx)
	return err
}

// Timeout 是一次對帳的時間上限。
func (w *reconcileWorker) Timeout(*river.Job[ReconcileTerminationsArgs]) time.Duration {
	return 10 * time.Minute
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &effectWorker{service: s})
	river.AddWorker(workers, &reconcileWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每小時補跑的終止合作對帳。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return ReconcileTerminationsArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.reconcile_terminations", RunOnStart: true},
	)}
}
