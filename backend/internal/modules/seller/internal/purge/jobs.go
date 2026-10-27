package purge

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

// jobID 是排程的識別，也是 River job 的 kind。
const jobID = "seller.purge_pii"

// Args 是個資清理的工作。
type Args struct{}

// Kind 回傳 River 工作種類。
func (Args) Kind() string { return jobID }

// InsertOpts 將同一小時的排程與重啟合併；下一小時不受已完成工作的保留時間影響（同 buyer 的清理）。
func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Hour}}
}

type worker struct {
	river.WorkerDefaults[Args]
	service *Service
}

// Work 執行清理；每批各一個 transaction，失敗由 River 重試（已清理的列不會再被選中）。
func (w *worker) Work(ctx context.Context, _ *river.Job[Args]) error {
	_, err := w.service.Run(ctx)
	return err
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &worker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每小時補跑的清理排程。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return Args{}, nil },
			&river.PeriodicJobOpts{ID: jobID, RunOnStart: true},
		),
	}
}
