package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Executor 執行一個已被領走（RUNNING）的 run：LLM 與 provider 呼叫在這裡，不在 transaction 內。
//
// 契約：回傳 nil 之前 run 必須已到終態或 WAITING_FOR_INPUT（經 Ledger.Transition），否則 reconciler 把它標失敗；
// 回傳錯誤表示 run 失敗，Dispatcher 負責轉 FAILED 並退額度；ctx 被取消（取消或逾時）時盡快返回。
// run 的事件與產出經 Ledger 寫入；寫入收到 ErrRunTerminal 代表 run 已被別處收斂，executor 應停止並回傳它（Dispatcher 視為正常結束）。
type Executor interface {
	Execute(ctx context.Context, run store.ProcurementAgentRun) error
}

// RunTimeoutLimit 是一次 run 的執行上限（River job 的 Timeout，A9-D22）；超過的由 ctx 取消。
const RunTimeoutLimit = 2 * time.Minute

// Dispatcher 是 RunArgs 的 worker：領走 QUEUED 的 run、執行，失敗時標 FAILED 並退回額度，取消時收斂到 CANCELLED。
type Dispatcher struct {
	DB       *db.DB
	Ledger   Ledger
	Executor Executor
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

// Run 領走並執行一個 run。run 已被領走、已取消或已終結時什麼都不做（不重驅）。
func (d Dispatcher) Run(ctx context.Context, args RunArgs) error {
	run, claimed, err := d.claim(ctx, args.RunID)
	if err != nil || !claimed {
		return err
	}
	execErr := d.Executor.Execute(ctx, run)
	if execErr == nil || errors.Is(execErr, ErrRunTerminal) {
		return nil
	}
	return d.settleFailure(context.WithoutCancel(ctx), run)
}

func (d Dispatcher) claim(ctx context.Context, id uuid.UUID) (store.ProcurementAgentRun, bool, error) {
	run, err := d.Ledger.Transition(ctx, id, StatusRunning, "")
	var invalid *InvalidTransitionError
	switch {
	case err == nil:
		return run, true, nil
	case errors.Is(err, ErrRunTerminal), errors.As(err, &invalid):
		return store.ProcurementAgentRun{}, false, nil
	default:
		return store.ProcurementAgentRun{}, false, fmt.Errorf("run：領取：%w", err)
	}
}

// settleFailure 在 executor 失敗後收斂 run：已要求取消的收斂到 CANCELLED（不退額度），否則 FAILED 並退額度；已在終態什麼都不做。
func (d Dispatcher) settleFailure(ctx context.Context, run store.ProcurementAgentRun) error {
	return d.DB.InTx(ctx, func(ctx context.Context) error {
		cur, err := store.New(d.DB.Conn(ctx)).LockRun(ctx, run.ID)
		if err != nil {
			return fmt.Errorf("run：讀取 run：%w", err)
		}
		if cur.Status == StatusCancelRequested {
			_, err = d.Ledger.Transition(ctx, run.ID, StatusCancelled, "")
		} else {
			_, err = d.Ledger.FailAndRefund(ctx, run.ID, failureFor(run.RunType))
		}
		if errors.Is(err, ErrRunTerminal) {
			return nil
		}
		return err
	})
}

func failureFor(runType string) string {
	if runType == runTypeCandidateSearch {
		return FailureSearchFailed
	}
	return FailureTurnPipelineErr
}
