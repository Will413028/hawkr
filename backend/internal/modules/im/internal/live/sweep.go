package live

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// PresenceSweepInterval 是存活標記清理的間隔。清理只刪過期超過 10 分鐘的列，讀取與計數都只認未過期的列，所以間隔不影響正確性，只決定表的大小。
const PresenceSweepInterval = 15 * time.Minute

// PresenceSweepArgs 清掉過期超過 10 分鐘的串流存活標記（崩潰的進程沒有 Close）。
type PresenceSweepArgs struct{}

// Kind 是 River 的 job 種類。
func (PresenceSweepArgs) Kind() string { return "im.presence_sweep" }

// InsertOpts 以 PresenceSweepInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併，窗過了就能再排。
func (PresenceSweepArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(PresenceSweepInterval)
	o.MaxAttempts = 1
	return o
}

type presenceSweepWorker struct {
	river.WorkerDefaults[PresenceSweepArgs]
	streams DBStreams
}

// Work 清理過期的存活標記。
func (w *presenceSweepWorker) Work(ctx context.Context, _ *river.Job[PresenceSweepArgs]) error {
	_, err := w.streams.Sweep(ctx)
	return err
}

// Timeout 是一次清理的時間上限。
func (w *presenceSweepWorker) Timeout(*river.Job[PresenceSweepArgs]) time.Duration {
	return time.Minute
}

// AddWorkers 登記存活標記清理的 River worker。
func AddWorkers(workers *river.Workers, s DBStreams) {
	river.AddWorker(workers, &presenceSweepWorker{streams: s})
}

// PeriodicJobs 回傳啟動即執行、每 PresenceSweepInterval 的存活標記清理。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(PresenceSweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return PresenceSweepArgs{}, nil },
			&river.PeriodicJobOpts{ID: "im.presence_sweep", RunOnStart: true},
		),
	}
}
