package ranking

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

const (
	// recomputeID 是重算排程的識別，也是 River job 的 kind。
	recomputeID = "discovery.recompute_seller_scores"
	// RecomputeInterval 是重算的間隔：榜單最多落後這麼久（website 的快取是 900 秒，同一個量級）。
	RecomputeInterval = 15 * time.Minute
	// recomputeTimeout 是單次嘗試的上限（River 預設 1 分鐘；四個上游的呼叫加起來可能超過）。
	recomputeTimeout = 10 * time.Minute
	// recomputeAttempts 是失敗時的嘗試次數；下一個週期本來就會重算，不需要 River 預設的 25 次退避。
	recomputeAttempts = 3
)

// RecomputeArgs 是重算分數快照的工作。
type RecomputeArgs struct{}

// Kind 回傳 River 工作種類。
func (RecomputeArgs) Kind() string { return recomputeID }

// InsertOpts 讓同一個間隔內的排程與重啟合併（ByPeriod 只去除重複的「排入」）。它不保證重算不會重疊：重試的嘗試與下一個
// 週期排入的工作、多個節點，都可能讓兩個重算同時或交錯執行。所以正確性不靠它，而是靠 replace 的序列化與開始時間檢查
// （recompute_state）：重疊時較早開始的結果會被丟棄，不會蓋掉較新的快照。
func (RecomputeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: recomputeAttempts,
		UniqueOpts:  river.UniqueOpts{ByPeriod: RecomputeInterval},
	}
}

type recomputeWorker struct {
	river.WorkerDefaults[RecomputeArgs]
	service *Service
}

// Timeout 覆寫 River 預設的 1 分鐘。
func (*recomputeWorker) Timeout(*river.Job[RecomputeArgs]) time.Duration { return recomputeTimeout }

// Work 重算一輪；上游失敗時回 error，快照不變，由 River 重試（失敗也記在 recompute_state）。結果因為有更晚開始的重算
// 已成功而被丟棄不是錯誤，不重試。
func (w *recomputeWorker) Work(ctx context.Context, _ *river.Job[RecomputeArgs]) error {
	_, err := w.service.Recompute(ctx)
	return err
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &recomputeWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行的排程：每 15 分鐘重算一次（新部署不用等一個週期才有榜單）。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(RecomputeInterval),
			func() (river.JobArgs, *river.InsertOpts) { return RecomputeArgs{}, nil },
			&river.PeriodicJobOpts{ID: recomputeID, RunOnStart: true},
		),
	}
}
