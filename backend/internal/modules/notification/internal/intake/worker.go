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

// deliverWorker 是渠道 job 的位置：A8-3 把推播與 email 的 adapter 接進來。這一步什麼都不做，投遞紀錄留在 PENDING。
type deliverWorker struct {
	river.WorkerDefaults[DeliverArgs]
}

// Work 這一步不送出。
func (w *deliverWorker) Work(context.Context, *river.Job[DeliverArgs]) error { return nil }

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &processWorker{service: s})
	river.AddWorker(workers, &deliverWorker{})
}
