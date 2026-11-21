package run

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// 時間參數。QueuedTimeout 與 RunningTimeout 是推論值（A9-D6 只決定逾時標失敗、不重驅，沒有訂數字）：
// RUNNING 以事件寫入（updated_at）為心跳，圖的各段逾時加總約 83 秒（57 號設計書 §4.2），gateway 串流上限 120 秒，
// 取 5 分鐘讓正常的慢輪次不被誤殺；QUEUED 沒被領走代表 worker 不在，取 10 分鐘。
const (
	QueuedTimeout  = 10 * time.Minute
	RunningTimeout = 5 * time.Minute
	SweepInterval  = time.Minute
	sweepBatch     = 200
)

// Sweeper 把逾時的 run 標成 FAILED（WORKER_LOST）並發出串流的結尾事件，不重新驅動（A9-D6）。
type Sweeper struct {
	DB            *db.DB
	Ledger        Ledger
	QueuedTimeout time.Duration
	RunTimeout    time.Duration
	Logger        *slog.Logger
}

// Sweep 處理一批逾時的 run，回傳標成失敗的數量。
func (s Sweeper) Sweep(ctx context.Context) (int, error) {
	now := s.Ledger.Now()
	failed := 0
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		ids, err := store.New(s.DB.Conn(ctx)).ListTimedOutRunIDs(ctx, store.ListTimedOutRunIDsParams{
			QueuedBefore: now.Add(-s.QueuedTimeout), RunningBefore: now.Add(-s.RunTimeout), Batch: sweepBatch,
		})
		if err != nil {
			return fmt.Errorf("run：列出逾時的 run：%w", err)
		}
		for _, id := range ids {
			ok, err := s.Ledger.Finish(ctx, id, StatusFailed, FailureWorkerLost)
			if err != nil {
				return err
			}
			if ok {
				failed++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if failed > 0 {
		s.Logger.WarnContext(ctx, "procurement：逾時的 run 已標成失敗", "runs", failed)
	}
	return failed, nil
}

// SweepArgs 是週期掃描的 job。
type SweepArgs struct{}

func (SweepArgs) Kind() string { return "procurement.run_sweep" }

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	sweeper Sweeper
}

func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, err := w.sweeper.Sweep(ctx)
	return err
}

func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return time.Minute }

// RetentionPeriod 是 run 事件在 run 結束後保留的時間（A9-D20 R1）。
const RetentionPeriod = 7 * 24 * time.Hour

// PurgeInterval 是事件清除的週期。
const PurgeInterval = time.Hour

// Purger 清除結束超過保留期的 run 的事件並推進 min_available_seq。
type Purger struct {
	DB     *db.DB
	Ledger Ledger
}

// purgeBatch 是一次清除處理的 run 數上限，避免一個 statement 持有大量列鎖與長時間的 transaction。
const purgeBatch = 100

// Purge 分批清除，直到沒有符合條件的 run，回傳刪除的事件數。
func (p Purger) Purge(ctx context.Context) (int64, error) {
	cutoff := p.Ledger.Now().Add(-RetentionPeriod)
	return drainExpired(ctx, func(ctx context.Context) (int64, int64, error) {
		r, err := store.New(p.DB.Conn(ctx)).ExpireRunEvents(ctx, store.ExpireRunEventsParams{Cutoff: cutoff, Batch: purgeBatch})
		if err != nil {
			return 0, 0, fmt.Errorf("run：清除過期事件：%w", err)
		}
		return r.Runs, r.Events, nil
	})
}

// drainExpired 重複呼叫 step 直到它推進的 run 數為 0，回傳刪除的事件總數。
func drainExpired(ctx context.Context, step func(context.Context) (runs, events int64, err error)) (int64, error) {
	var total int64
	for {
		runs, events, err := step(ctx)
		if err != nil {
			return total, err
		}
		total += events
		if runs == 0 {
			return total, nil
		}
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
}

// PurgeArgs 是週期清除的 job。
type PurgeArgs struct{}

func (PurgeArgs) Kind() string { return "procurement.run_event_purge" }

type purgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	purger Purger
}

func (w *purgeWorker) Work(ctx context.Context, _ *river.Job[PurgeArgs]) error {
	_, err := w.purger.Purge(ctx)
	return err
}

func (w *purgeWorker) Timeout(*river.Job[PurgeArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記 sweeper 與事件清除的 River worker。
func AddWorkers(workers *river.Workers, s Sweeper, p Purger) {
	river.AddWorker(workers, &sweepWorker{sweeper: s})
	river.AddWorker(workers, &purgeWorker{purger: p})
}

// PeriodicJobs 回傳 sweeper 與事件清除的排程（啟動即執行一次）。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
			&river.PeriodicJobOpts{ID: "procurement.run_sweep", RunOnStart: true}),
		river.NewPeriodicJob(river.PeriodicInterval(PurgeInterval),
			func() (river.JobArgs, *river.InsertOpts) { return PurgeArgs{}, nil },
			&river.PeriodicJobOpts{ID: "procurement.run_event_purge", RunOnStart: true}),
	}
}
