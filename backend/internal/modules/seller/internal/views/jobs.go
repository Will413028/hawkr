package views

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

const (
	// rollupID 是彙總排程的識別，也是 River job 的 kind。
	rollupID = "seller.product_views_rollup"
	// purgeID 是原始事件清理排程的識別，也是 River job 的 kind。
	purgeID = "seller.product_views_purge"
	// rollupInterval 是彙總的間隔；儀表板的數字最多落後這麼久（延遲式分析）。
	rollupInterval = 5 * time.Minute
	// purgeInterval 是清理的間隔。
	purgeInterval = time.Hour
)

// RollupArgs 是瀏覽事件彙總的工作。
type RollupArgs struct{}

// Kind 回傳 River 工作種類。
func (RollupArgs) Kind() string { return rollupID }

// InsertOpts 將同一個間隔內的排程與重啟合併。
func (RollupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: rollupInterval}}
}

// PurgeArgs 是原始瀏覽事件清理的工作。
type PurgeArgs struct{}

// Kind 回傳 River 工作種類。
func (PurgeArgs) Kind() string { return purgeID }

// InsertOpts 將同一小時的排程與重啟合併。
func (PurgeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: purgeInterval}}
}

type rollupWorker struct {
	river.WorkerDefaults[RollupArgs]
	service *Service
}

// Work 彙總一輪；商品 port 失敗時回 error，沒有事件被標記，由 River 重試。
func (w *rollupWorker) Work(ctx context.Context, _ *river.Job[RollupArgs]) error {
	_, err := w.service.Rollup(ctx)
	return err
}

type purgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	service *Service
}

// Work 清理一輪。
func (w *purgeWorker) Work(ctx context.Context, _ *river.Job[PurgeArgs]) error {
	_, err := w.service.Purge(ctx)
	return err
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &rollupWorker{service: s})
	river.AddWorker(workers, &purgeWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行的排程：每 5 分鐘彙總，每小時清理已過保留期的原始事件。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(rollupInterval),
			func() (river.JobArgs, *river.InsertOpts) { return RollupArgs{}, nil },
			&river.PeriodicJobOpts{ID: rollupID, RunOnStart: true},
		),
		river.NewPeriodicJob(
			river.PeriodicInterval(purgeInterval),
			func() (river.JobArgs, *river.InsertOpts) { return PurgeArgs{}, nil },
			&river.PeriodicJobOpts{ID: purgeID, RunOnStart: true},
		),
	}
}
