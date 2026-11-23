package intake

import (
	"context"

	"github.com/riverqueue/river"
)

type processWorker struct {
	river.WorkerDefaults[ProcessArgs]
	service *Service
}

// Work 展開一個意圖；失敗由 River 重試（Process 重試安全）。
func (w *processWorker) Work(ctx context.Context, job *river.Job[ProcessArgs]) error {
	return w.service.Process(ctx, job.Args.IntentID)
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &processWorker{service: s})
}
