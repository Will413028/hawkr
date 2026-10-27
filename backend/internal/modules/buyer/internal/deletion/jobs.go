// Package deletion 提供買家關閉與個資清理。
package deletion

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

// CleanupArgs 清理已關閉且超過保留期限個資的工作。
type CleanupArgs struct{}

// Kind 回傳 River 工作種類。
func (CleanupArgs) Kind() string { return "buyer.cleanup_pii" }

type cleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	service *Service
}

// Work 執行個資清理，分批各自一個 transaction。
func (worker *cleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupArgs]) error {
	return worker.service.CleanupPII(ctx)
}

// AddWorkers 登記 River workers。
func (service *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &cleanupWorker{service: service})
}

// PeriodicJobs 回傳啟動即執行、每小時補跑的 PII 清理排程。
func (service *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return CleanupArgs{}, nil },
			&river.PeriodicJobOpts{ID: "buyer.cleanup_pii", RunOnStart: true},
		),
	}
}

// InsertOpts 將同一小時的排程與重啟合併；下一小時不受已完成工作的保留時間影響。
func (CleanupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Hour}}
}
