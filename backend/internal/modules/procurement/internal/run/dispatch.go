package run

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Executor 執行一個已被領走（RUNNING）的 run：LLM 與 provider 呼叫在這裡，不在 transaction 內。回傳錯誤表示 run 失敗。
// run 自己的事件與產出經 Ledger 寫入；成功時 Executor 負責把 run 轉成終態（Ledger.Finish）。
type Executor interface {
	Execute(ctx context.Context, run store.ProcurementAgentRun) error
}

// RunTimeoutLimit 是一次 run 的執行上限（River job 的 Timeout）；超過的由 ctx 取消，sweeper 再標失敗。
const RunTimeoutLimit = RunningTimeout

// Dispatcher 是 RunArgs 的 worker：領走 QUEUED 的 run、執行，失敗時標 FAILED 並退回額度。
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

// Run 領走並執行一個 run。run 已被領走或已終結時什麼都不做（不重驅）。
func (d Dispatcher) Run(ctx context.Context, args RunArgs) error {
	var run store.ProcurementAgentRun
	claimed := false
	err := d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(d.DB.Conn(ctx))
		n, err := q.ClaimRun(ctx, store.ClaimRunParams{ID: args.RunID, Now: d.Ledger.Now()})
		if err != nil {
			return fmt.Errorf("run：領取：%w", err)
		}
		if n == 0 {
			return nil
		}
		claimed = true
		run, err = q.GetRun(ctx, args.RunID)
		return err
	})
	if err != nil || !claimed {
		return err
	}
	execErr := d.Executor.Execute(ctx, run)
	if execErr == nil {
		return nil
	}
	ctx = context.WithoutCancel(ctx)
	return d.DB.InTx(ctx, func(ctx context.Context) error {
		done, err := d.Ledger.Finish(ctx, args.RunID, StatusFailed, failureFor(run.RunType))
		if err != nil || !done {
			return err
		}
		_, err = store.New(d.DB.Conn(ctx)).RefundUsage(ctx, store.RefundUsageParams{BuyerID: args.BuyerID, PeriodStart: args.PeriodStart, Now: d.Ledger.Now()})
		return err
	})
}

func failureFor(runType string) string {
	if runType == runTypeCandidateSearch {
		return FailureSearchFailed
	}
	return FailureTurnPipelineErr
}
