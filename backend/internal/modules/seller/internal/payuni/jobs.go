package payuni

import (
	"context"
	"errors"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// 唯一鍵涵蓋「還沒有結果」的狀態：排隊中、執行中、等重試。不含 completed：完成的 job 在 River 清掉之前（預設一天）仍會
// 擋下同一個鍵的新 job，之後的改動就排不進去。running 不能排除（River 的規定），所以執行期間發生的改動不靠重新排入，
// 而靠 intent_seq：結果寫入後 intent_seq 比讀到的大，Sync 回 again，worker 以 snooze 再跑一輪（rivertype 的狀態集合）。
func openStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	}
}

// SyncArgs 是 PayUni 綁定與同步的 job，以 store_id 為唯一鍵：同一個店家同時最多一個排隊中的 job，連續的改動合併成一次，
// job 讀的是執行當下的資料。
type SyncArgs struct {
	StoreID int64 `json:"store_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (SyncArgs) Kind() string { return "seller.payuni_sync" }

// InsertOpts 是 sync job 的排入選項：以 store_id 去重（不含已完成的），暫時性的失敗最多重試 syncMaxAttempts 次。
func (SyncArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: syncMaxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: openStates()}}
}

func enqueueSync(ctx context.Context, jc jobs.Enqueuer, storeID int64) error {
	args := SyncArgs{StoreID: storeID}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// PollArgs 是向 PayUni 查一次審核狀態的 job（來源是輪詢或手動刷新），以 store_id 為唯一鍵。
type PollArgs struct {
	StoreID int64              `json:"store_id" river:"unique"`
	Source  model.PayUniSource `json:"source"`
}

// Kind 是 River 的 job 種類。
func (PollArgs) Kind() string { return "seller.payuni_poll" }

// InsertOpts 是輪詢 job 的排入選項。
func (PollArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: openStates()}}
}

func enqueuePoll(ctx context.Context, jc jobs.Enqueuer, storeID int64, source model.PayUniSource) error {
	args := PollArgs{StoreID: storeID, Source: source}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// ObserveArgs 是套用一則 PayUni 通知（AGENT_NOTIFY）的 job，以 event_key 為唯一鍵（重送的通知不會另排）；真正的冪等在
// payuni_observations 的 UNIQUE(store_id, event_key)。ExecutedAt 是 PayUni 的 ExecuteTime（UTC）。
type ObserveArgs struct {
	EventKey      string    `json:"event_key" river:"unique"`
	MerID         string    `json:"mer_id"`
	MerStatus     string    `json:"mer_status"`
	MerStatusNote string    `json:"mer_status_note,omitempty"`
	ExecutedAt    time.Time `json:"executed_at"`
}

// Kind 是 River 的 job 種類。
func (ObserveArgs) Kind() string { return "seller.payuni_observe" }

// InsertOpts 以 event_key 去重（預設的狀態集合：完成後在 River 清掉之前也擋下重送）。
func (ObserveArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

func enqueueObserve(ctx context.Context, jc jobs.Enqueuer, args ObserveArgs) error {
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// SweepArgs 是定期的掃描（補排遺失的 sync、排輪詢）。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "seller.payuni_sweep" }

// InsertOpts 把同一段時間內的排程與重啟合併。
func (SweepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 10 * time.Minute}}
}

// resyncDelay 是 sync 結果寫入後發現又有新資料、或別的 job 持有租約時，等多久再跑一輪。
const resyncDelay = 2 * time.Second

// jobError 把永久的失敗轉成 River 的取消（記錄在 job 上、不再重試），其他原樣回傳由 River 重試。
func jobError(err error) error {
	if errors.Is(err, ErrPermanent) {
		return river.JobCancel(err)
	}
	return err
}

type syncWorker struct {
	river.WorkerDefaults[SyncArgs]
	service *Service
}

// Work 執行一次 sync：最後一次嘗試仍暫時性失敗時由 Sync 記錄結果；結果寫入後又有新資料時 snooze 再跑一輪（不計一次嘗試）。
func (w *syncWorker) Work(ctx context.Context, job *river.Job[SyncArgs]) error {
	again, err := w.service.Sync(ctx, job.Args.StoreID, job.Attempt >= job.MaxAttempts)
	if err != nil {
		return err
	}
	if again {
		return river.JobSnooze(resyncDelay)
	}
	return nil
}

// Timeout 是一次 sync 的時間上限（租約比它長）。
func (w *syncWorker) Timeout(*river.Job[SyncArgs]) time.Duration { return jobTimeout }

type pollWorker struct {
	river.WorkerDefaults[PollArgs]
	service *Service
}

// Work 查一次 PayUni 的審核狀態並套用。
func (w *pollWorker) Work(ctx context.Context, job *river.Job[PollArgs]) error {
	return w.service.Poll(ctx, job.Args.StoreID, job.Args.Source)
}

// Timeout 是一次輪詢的時間上限。
func (w *pollWorker) Timeout(*river.Job[PollArgs]) time.Duration { return jobTimeout }

type observeWorker struct {
	river.WorkerDefaults[ObserveArgs]
	service *Service
}

// Work 套用一則通知：失敗回錯誤讓 River 重試（不吞掉，盤點 D9），未知的店家取消、不重試（盤點 D14）。
func (w *observeWorker) Work(ctx context.Context, job *river.Job[ObserveArgs]) error {
	return jobError(w.service.Observe(ctx, job.Args))
}

// Timeout 是一次套用的時間上限。
func (w *observeWorker) Timeout(*river.Job[ObserveArgs]) time.Duration { return 30 * time.Second }

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	service *Service
}

// Work 執行掃描。
func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, _, err := w.service.Sweep(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &syncWorker{service: s})
	river.AddWorker(workers, &pollWorker{service: s})
	river.AddWorker(workers, &observeWorker{service: s})
	river.AddWorker(workers, &sweepWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的掃描。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(15*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.payuni_sweep", RunOnStart: true},
	)}
}
