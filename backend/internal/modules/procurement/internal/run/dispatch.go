package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Executor 執行一個已被領走（RUNNING）的 run：LLM 與 provider 呼叫在這裡，不在 transaction 內。
//
// 契約：回傳 nil 之前 run 必須已到終態（經 Ledger.Transition）或 WAITING_FOR_INPUT（經 Ledger.Interrupt），否則 reconciler 把它標失敗；
// 回傳錯誤表示 run 失敗，Dispatcher 負責轉 FAILED 並退額度（ctx 被關機或逾時取消的是 WORKER_LOST）；ctx 被取消（取消、關機或逾時）時盡快返回。
// run 的事件與產出經 Ledger 寫入；寫入收到 ErrRunTerminal 代表 run 已被別處收斂，executor 應停止並回傳它（Dispatcher 視為正常結束）。
type Executor interface {
	Execute(ctx context.Context, run store.ProcurementAgentRun) error
}

// RunTimeoutLimit 是一次 run 的執行上限（River job 的 Timeout，A9-D22）；超過的由 ctx 取消。
const RunTimeoutLimit = 2 * time.Minute

// Dispatcher 是 RunArgs 的 worker：領走 QUEUED 的 run、執行，失敗時標 FAILED 並退回額度，取消時收斂到 CANCELLED。
//
// Executors 依 run 類型登記執行者；沒有登記執行者的類型當成該類型的 pipeline 失敗收斂（記 log），不會留在 RUNNING 等 reconciler。
type Dispatcher struct {
	DB        *db.DB
	Ledger    Ledger
	Executors map[string]Executor
	Logger    *slog.Logger
}

type runWorker struct {
	river.WorkerDefaults[RunArgs]
	d Dispatcher
}

func (w *runWorker) Work(ctx context.Context, job *river.Job[RunArgs]) error {
	return w.d.Run(ctx, job.Args)
}

func (w *runWorker) Timeout(*river.Job[RunArgs]) time.Duration { return RunTimeoutLimit }

// AddWorker 登記執行 run 的 River worker。
func (d Dispatcher) AddWorker(workers *river.Workers) { river.AddWorker(workers, &runWorker{d: d}) }

// Run 領走並執行一個 run。run 已被領走、已取消、已終結或已被刪除時什麼都不做（不重驅）。
func (d Dispatcher) Run(ctx context.Context, args RunArgs) error {
	run, err := d.Ledger.Transition(ctx, args.RunID, StatusRunning, "")
	if claimed, err := claimResult(err); err != nil || !claimed {
		return err
	}
	executor, ok := d.Executors[run.RunType]
	if !ok {
		d.logger().ErrorContext(ctx, "procurement：這個 run 類型沒有登記執行者，標為失敗", "run_id", run.ID.String(), "run_type", run.RunType)
		return d.settleFailure(context.WithoutCancel(ctx), run, nil, nil)
	}
	execErr := executor.Execute(ctx, run)
	if execErr == nil || errors.Is(execErr, ErrRunTerminal) {
		return nil
	}
	return d.settleFailure(context.WithoutCancel(ctx), run, ctx.Err(), execErr)
}

func claimResult(err error) (claimed bool, ret error) {
	var invalid *InvalidTransitionError
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrRunTerminal), errors.As(err, &invalid), apperr.KindOf(err) == apperr.KindNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("run：領取：%w", err)
	}
}

func settleOutcome(status, runType string, jobCtxErr, execErr error) outcome {
	switch {
	case status == StatusCancelRequested:
		return outcome{action: actionCancel}
	case jobCtxErr != nil:
		return outcome{action: actionFail, reason: FailureWorkerLost, refund: true}
	default:
		return outcome{action: actionFail, reason: failureReasonOf(execErr, runType), refund: true}
	}
}

// FailureError 是 executor 回傳的可分類失敗：Reason 是 run 的失敗原因（Failure* 常數），Dispatcher 依它收斂。
// 沒有包成它的錯誤是該 run 類型的 pipeline 失敗。
type FailureError struct {
	Reason string
	Err    error
}

func (e *FailureError) Error() string { return fmt.Sprintf("run：%s：%v", e.Reason, e.Err) }
func (e *FailureError) Unwrap() error { return e.Err }

func failureReasonOf(execErr error, runType string) string {
	var f *FailureError
	if errors.As(execErr, &f) && knownFailureReason(f.Reason) {
		return f.Reason
	}
	return failureFor(runType)
}

func (d Dispatcher) settleFailure(ctx context.Context, run store.ProcurementAgentRun, jobCtxErr, execErr error) error {
	return d.DB.InTx(ctx, func(ctx context.Context) error {
		cur, err := d.Ledger.lockRun(ctx, run.ID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		err = d.Ledger.converge(ctx, run.ID, settleOutcome(cur.Status, run.RunType, jobCtxErr, execErr))
		if errors.Is(err, ErrRunTerminal) {
			return nil
		}
		return err
	})
}

func failureFor(runType string) string {
	if runType == RunTypeCandidateSearch {
		return FailureSearchFailed
	}
	return FailureTurnPipelineErr
}

func (d Dispatcher) logger() *slog.Logger {
	if d.Logger == nil {
		return slog.Default()
	}
	return d.Logger
}
