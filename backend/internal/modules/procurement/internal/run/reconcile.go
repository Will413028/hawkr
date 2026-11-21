package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// ReconcileInterval 是 reconciler 的週期。
const ReconcileInterval = time.Minute

const reconcileBatch = 200

// JobStates 讀取 River job 的狀態；job 不存在時 found 為 false。
type JobStates interface {
	JobState(ctx context.Context, jobID int64) (state rivertype.JobState, found bool, err error)
}

type action int

const (
	actionSkip action = iota
	actionFail
	actionCancel
)

type outcome struct {
	action action
	reason string
	refund bool
}

// decide 是 reconciler 的對照表（A9-D22、A9-D23）：run 的狀態與 job 的狀態決定怎麼收斂。
// job 還在進行中（排隊、等重試、執行中）一律略過；job 已死：要求取消中的 run 收斂到 CANCELLED（不退額度），
// 其餘 FAILED 並退回額度（job 正常結束卻沒收尾的是執行者違約，沿用該 run 類型的 pipeline 失敗原因，其餘是 WORKER_LOST）。
func decide(r store.ProcurementAgentRun, state rivertype.JobState, found bool) outcome {
	reason := FailureWorkerLost
	if found {
		switch state {
		case rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable, rivertype.JobStateRunning, rivertype.JobStateScheduled:
			return outcome{action: actionSkip}
		case rivertype.JobStateCompleted:
			reason = failureFor(r.RunType)
		case rivertype.JobStateDiscarded, rivertype.JobStateCancelled:
		default:
			return outcome{action: actionSkip}
		}
	}
	if r.Status == StatusCancelRequested {
		return outcome{action: actionCancel}
	}
	return outcome{action: actionFail, reason: reason, refund: true}
}

// converge 把 run 依 outcome 收斂：失敗退額度，取消不退。
func (l Ledger) converge(ctx context.Context, id uuid.UUID, o outcome) error {
	var err error
	switch o.action {
	case actionFail:
		if o.refund {
			_, err = l.FailAndRefund(ctx, id, o.reason)
		} else {
			_, err = l.Transition(ctx, id, StatusFailed, o.reason)
		}
	case actionCancel:
		_, err = l.Transition(ctx, id, StatusCancelled, "")
	case actionSkip:
	}
	return err
}

// Reconciler 對帳 run 與它的 River job：job 已死而 run 還沒收斂的，轉到終態並寫串流的結尾事件。
// 逐列一個 transaction，一列失敗只記錄、不影響其他列；WAITING_FOR_INPUT 沒有 job，不在對帳範圍。
type Reconciler struct {
	DB     *db.DB
	Ledger Ledger
	Logger *slog.Logger
}

// ReconcileResult 是一次對帳的結果。
type ReconcileResult struct {
	Failed, Cancelled, Errors int
}

// Reconcile 掃過所有 QUEUED、RUNNING、CANCEL_REQUESTED 的 run。
func (r Reconciler) Reconcile(ctx context.Context, states JobStates) (ReconcileResult, error) {
	var total ReconcileResult
	var after uuid.UUID
	for {
		ids, err := store.New(r.DB.Conn(ctx)).ListActiveRunIDsAfter(ctx, store.ListActiveRunIDsAfterParams{After: after, Batch: reconcileBatch})
		if err != nil {
			return total, fmt.Errorf("run：列出要對帳的 run：%w", err)
		}
		res, err := reconcileEach(ctx, r.DB, ids, func(ctx context.Context, id uuid.UUID) (action, error) {
			return r.reconcileOne(ctx, states, id)
		}, r.Logger)
		total.Failed += res.Failed
		total.Cancelled += res.Cancelled
		total.Errors += res.Errors
		if err != nil {
			return total, err
		}
		if len(ids) < reconcileBatch {
			break
		}
		after = ids[len(ids)-1]
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
	if total.Failed+total.Cancelled+total.Errors > 0 {
		r.Logger.WarnContext(ctx, "procurement：run 對帳", "failed", total.Failed, "cancelled", total.Cancelled, "errors", total.Errors)
	}
	return total, nil
}

type txRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// reconcileEach 逐一處理 ids，每個 id 一個 transaction；某個 id 失敗就記錄並繼續下一個。
// ctx 結束（關機）就停下並回傳 ctx 的錯誤，不逐列記錄失敗。
func reconcileEach(ctx context.Context, tx txRunner, ids []uuid.UUID, process func(ctx context.Context, id uuid.UUID) (action, error), log *slog.Logger) (ReconcileResult, error) {
	var res ReconcileResult
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		var act action
		err := tx.InTx(ctx, func(ctx context.Context) error {
			var err error
			act, err = process(ctx, id)
			return err
		})
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			res.Errors++
			log.ErrorContext(ctx, "procurement：run 對帳失敗，略過這一列", "run_id", id.String(), "error", err)
			continue
		}
		switch act {
		case actionFail:
			res.Failed++
		case actionCancel:
			res.Cancelled++
		case actionSkip:
		}
	}
	return res, nil
}

func (r Reconciler) reconcileOne(ctx context.Context, states JobStates, id uuid.UUID) (action, error) {
	row, err := store.New(r.DB.Conn(ctx)).LockActiveRun(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return actionSkip, nil
	}
	if err != nil {
		return actionSkip, fmt.Errorf("run：鎖定：%w", err)
	}
	var state rivertype.JobState
	found := false
	if row.JobID != nil {
		if state, found, err = states.JobState(ctx, *row.JobID); err != nil {
			return actionSkip, fmt.Errorf("run：讀取 job %d：%w", *row.JobID, err)
		}
	}
	o := decide(row, state, found)
	return o.action, r.Ledger.converge(ctx, id, o)
}

// clientJobStates 用 worker ctx 裡的 River client 在目前的 transaction 讀 job。
type clientJobStates struct{}

func (clientJobStates) JobState(ctx context.Context, jobID int64) (rivertype.JobState, bool, error) {
	c, err := river.ClientFromContextSafely[pgx.Tx](ctx)
	if err != nil {
		return "", false, err
	}
	tx, ok := db.TxFrom(ctx)
	if !ok {
		return "", false, errors.New("run：讀取 job 需要 transaction")
	}
	row, err := c.JobGetTx(ctx, tx, jobID)
	if errors.Is(err, river.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.State, true, nil
}

// ReconcileArgs 是週期對帳的 job。
type ReconcileArgs struct{}

func (ReconcileArgs) Kind() string { return "procurement.run_reconcile" }

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	reconciler Reconciler
}

func (w *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	_, err := w.reconciler.Reconcile(ctx, clientJobStates{})
	return err
}

func (w *reconcileWorker) Timeout(*river.Job[ReconcileArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記 reconciler 與事件清除的 River worker。
func AddWorkers(workers *river.Workers, r Reconciler, p Purger) {
	river.AddWorker(workers, &reconcileWorker{reconciler: r})
	river.AddWorker(workers, &purgeWorker{purger: p})
}

type periodicSpec struct {
	id       string
	interval time.Duration
	args     river.JobArgs
}

var periodicSpecs = []periodicSpec{
	{id: "procurement.run_reconcile", interval: ReconcileInterval, args: ReconcileArgs{}},
	{id: "procurement.run_event_purge", interval: PurgeInterval, args: PurgeArgs{}},
}

// PeriodicJobs 回傳 reconciler 與事件清除的排程（啟動即執行一次）。
func PeriodicJobs() []*river.PeriodicJob {
	jobs := make([]*river.PeriodicJob, 0, len(periodicSpecs))
	for _, spec := range periodicSpecs {
		jobs = append(jobs, river.NewPeriodicJob(river.PeriodicInterval(spec.interval),
			func() (river.JobArgs, *river.InsertOpts) { return spec.args, nil },
			&river.PeriodicJobOpts{ID: spec.id, RunOnStart: true}))
	}
	return jobs
}
