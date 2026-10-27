package suspension

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// ExpiryArgs 在措施結束時重算帳號的停權期間。EndsAt 是排入當時的結束時間：改結束時間時以新的值再排一個，舊的
// 那個照樣執行，重算是冪等的。
type ExpiryArgs struct {
	MeasureID uuid.UUID `json:"measure_id"`
	EndsAt    time.Time `json:"ends_at"`
}

// Kind 是 River 的 job 種類。
func (ExpiryArgs) Kind() string { return "seller.measure_expiry" }

// ScheduleExpiry 在 ctx 的 transaction 內排入排在 endsAt 的到期重算；同一個措施、同一個結束時間只排一個。
func ScheduleExpiry(ctx context.Context, enqueuer jobs.Enqueuer, measureID uuid.UUID, endsAt time.Time) error {
	return enqueuer.Enqueue(ctx, ExpiryArgs{MeasureID: measureID, EndsAt: endsAt.UTC()}, &river.InsertOpts{
		ScheduledAt: endsAt,
		UniqueOpts:  river.UniqueOpts{ByArgs: true},
	})
}

// Expire 是到期 job 的內容：鎖住措施所在帳號的 stint，以 max(endsAt, 現在) 為判斷時間重算停權期間（結束的措施不再算
// 生效，恢復時發 StoreUnsuspended）。用 endsAt 當下限是因為 worker 的時鐘可能比寫入措施的時鐘慢：job 在 endsAt 排程、
// 照 DB 的時間準時被取出，但 worker 的 now 還在 endsAt 之前時，以 now 判斷會把剛結束的措施當成還生效、什麼都不做，
// 這一輪就白跑（CR-3）。endsAt 是排入時的結束時間；措施後來延後時，判斷時間仍是舊的 endsAt，新的結束時間更晚，
// 措施繼續生效，不會被提早結束。措施已撤銷、或不存在時，重算的結果不變，不寫入。
func (s *Service) Expire(ctx context.Context, measureID uuid.UUID, endsAt time.Time) error {
	measure, err := store.New(s.d.DB.Conn(ctx)).MeasureByID(ctx, measureID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := stint.LockAccountOf(ctx, q, measure.StoreID)
		if err != nil {
			return err
		}
		at := s.d.Now().UTC()
		if endsAt.After(at) {
			at = endsAt.UTC()
		}
		return Recompute(ctx, q, s.d.Events, s.d.Jobs, s.d.Exposure, locked.AccountID, at)
	})
}

// ReconcileArgs 是措施的對帳：到期 job 遺失或寫入的 transaction 之外出了差錯時的保底。
type ReconcileArgs struct{}

// Kind 是 River 的 job 種類。
func (ReconcileArgs) Kind() string { return "seller.reconcile_suspensions" }

// InsertOpts 把同一小時的排程與重啟合併。
func (ReconcileArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Hour}}
}

// reconcileBatch 是對帳一次讀取的帳號數。
const reconcileBatch = 100

// Reconcile 修補停權期間與措施不一致的帳號：有進行中的期間卻沒有生效中的措施（到期 job 遺失或晚到），或有生效中的
// 措施卻沒有進行中的期間。只掃描有進行中期間（suspension_periods_open）或此刻有生效中停權措施（EffectiveMeasuresAll，
// 只讀 measures、用部分索引排除已結束與已撤銷的列，讀取量不隨歷史成長）的帳號，各自在自己的 transaction 內以 Recompute 重算（它先鎖再讀）；回傳掃描的帳號數。
// 權限不靠它：停權狀態每次讀取都由措施推導，這個 job 只讓期間與事件追上來。
func (s *Service) Reconcile(ctx context.Context) (int, error) {
	// 措施結束後留下的身分快照（到期 job 遺失、帳號沒有任何期間可掃時）也在這裡刪掉；比對本身不靠它，每次都經 stint.Derive。
	if err := s.pruneIdentities(ctx); err != nil {
		return 0, err
	}
	effective, err := stint.SuspendedAccounts(ctx, store.New(s.d.DB.Conn(ctx)), s.d.Now().UTC())
	if err != nil {
		return 0, err
	}
	scanned := 0
	var after int64
	for {
		ids, err := s.accountsToReconcile(ctx, effective, after)
		if err != nil {
			return scanned, err
		}
		for _, id := range ids {
			err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
				return Recompute(ctx, store.New(s.d.DB.Conn(ctx)), s.d.Events, s.d.Jobs, s.d.Exposure, id, s.d.Now().UTC())
			})
			if err != nil {
				return scanned, err
			}
			scanned++
			after = id
		}
		if len(ids) < reconcileBatch {
			return scanned, nil
		}
	}
}

// accountsToReconcile 合併兩個來源（進行中的期間、此刻生效中停權措施的帳號 effective，已排序），回傳 after 之後最小的
// reconcileBatch 個帳號；進行中的期間取最小的 reconcileBatch 個，合併後取前 reconcileBatch 個不會漏。
func (s *Service) accountsToReconcile(ctx context.Context, effective []int64, after int64) ([]int64, error) {
	q := store.New(s.d.DB.Conn(ctx))
	open, err := q.AccountsWithOpenPeriod(ctx, store.AccountsWithOpenPeriodParams{AfterAccountID: after, BatchSize: reconcileBatch})
	if err != nil {
		return nil, err
	}
	start, _ := slices.BinarySearch(effective, after+1)
	merged := slices.Concat(open, effective[start:min(start+reconcileBatch, len(effective))])
	slices.Sort(merged)
	merged = slices.Compact(merged)
	if len(merged) > reconcileBatch {
		merged = merged[:reconcileBatch]
	}
	return merged, nil
}

type expiryWorker struct {
	river.WorkerDefaults[ExpiryArgs]
	service *Service
}

// Work 執行到期的重算。
func (w *expiryWorker) Work(ctx context.Context, job *river.Job[ExpiryArgs]) error {
	return w.service.Expire(ctx, job.Args.MeasureID, job.Args.EndsAt)
}

// Timeout 是一次重算的時間上限。
func (w *expiryWorker) Timeout(*river.Job[ExpiryArgs]) time.Duration { return 30 * time.Second }

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	service *Service
}

// Work 執行對帳。
func (w *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	_, err := w.service.Reconcile(ctx)
	return err
}

// Timeout 是一次對帳的時間上限。
func (w *reconcileWorker) Timeout(*river.Job[ReconcileArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &expiryWorker{service: s})
	river.AddWorker(workers, &reconcileWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每小時補跑的措施對帳。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.reconcile_suspensions", RunOnStart: true},
	)}
}

// pruneIdentities 逐一清理還有身分快照的帳號：在自己的 transaction 內先鎖帳號全部的 stint（與措施的寫入同一個順序），
// 再只留生效中的措施的快照（stint.PruneIdentities）。
func (s *Service) pruneIdentities(ctx context.Context) error {
	accounts, err := store.New(s.d.DB.Conn(ctx)).AccountsWithMeasureSnapshots(ctx)
	if err != nil {
		return err
	}
	for _, id := range accounts {
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			if _, err := q.LockAccountStores(ctx, id); err != nil {
				return err
			}
			_, err := stint.PruneIdentities(ctx, q, id, s.d.Now().UTC())
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
