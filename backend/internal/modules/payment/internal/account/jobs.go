package account

import (
	"context"
	"errors"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// 唯一鍵涵蓋「還沒有結果」的狀態：排隊中、執行中、等重試。不含 completed：完成的 job 在 River 清掉之前仍會擋下同一個鍵的新 job。
// running 不能排除（River 的規定），所以執行期間到的事件不靠重新排入，而靠 inbox：結果寫入後仍有未涵蓋的事件時 Sync 回 again，worker 以 snooze 再跑一輪。
func openStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	}
}

// EnsureArgs 是建立外部帳戶的 job，以 store_id 為唯一鍵：同一個擁有者同時最多一個排隊中的建立。
type EnsureArgs struct {
	StoreID int64 `json:"store_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (EnsureArgs) Kind() string { return "payment.account_ensure" }

// InsertOpts 是建立 job 的排入選項：以 store_id 去重（不含已完成的），暫時性的失敗最多重試 maxAttempts 次。
func (EnsureArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: maxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: openStates()}}
}

func enqueueEnsure(ctx context.Context, jc jobs.Enqueuer, storeID int64) error {
	args := EnsureArgs{StoreID: storeID}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// SyncArgs 是重讀外部帳戶並覆寫快取的 job，以 store_id 為唯一鍵：webhook、輪詢與管理員的重新整理都只排這個 job，連續的請求合併成一次。
type SyncArgs struct {
	StoreID int64 `json:"store_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (SyncArgs) Kind() string { return "payment.account_sync" }

// InsertOpts 是同步 job 的排入選項。
func (SyncArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: maxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: openStates()}}
}

func enqueueSync(ctx context.Context, jc jobs.Enqueuer, storeID int64) error {
	args := SyncArgs{StoreID: storeID}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// SweepArgs 是定期的掃描：輪詢補漏（排 UNDER_REVIEW、ACTION_REQUIRED 與原因碼非終局的 DISABLED 帳戶的同步，等待清理的帳戶除外）、重排等待清理的清理與補排閒置的建立。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "payment.account_sweep" }

// InsertOpts 把同一段時間內的排程與重啟合併。
func (SweepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 10 * time.Minute}}
}

// resyncDelay 是結果寫入後仍有未涵蓋的事件時，等多久再跑一輪。
const resyncDelay = 2 * time.Second

// pollInterval 是輪詢補漏的週期。
const pollInterval = 15 * time.Minute

func jobError(err error) error {
	if errors.Is(err, ErrPermanent) {
		return river.JobCancel(err)
	}
	return err
}

type ensureWorker struct {
	river.WorkerDefaults[EnsureArgs]
	service *Service
}

// Work 執行一次建立；最後一次嘗試仍失敗時由 Ensure 記錄建立失敗。
func (w *ensureWorker) Work(ctx context.Context, job *river.Job[EnsureArgs]) error {
	return w.service.Ensure(ctx, job.Args.StoreID, job.Attempt >= job.MaxAttempts)
}

// Timeout 是一次建立的時間上限。
func (w *ensureWorker) Timeout(*river.Job[EnsureArgs]) time.Duration { return jobTimeout }

type syncWorker struct {
	river.WorkerDefaults[SyncArgs]
	service *Service
}

// Work 執行一次同步：結果寫入後仍有未涵蓋的事件時 snooze 再跑一輪（不計一次嘗試）；外部帳戶不存在等永久失敗取消、不重試。
func (w *syncWorker) Work(ctx context.Context, job *river.Job[SyncArgs]) error {
	again, err := w.service.Sync(ctx, job.Args.StoreID)
	if err != nil {
		return jobError(err)
	}
	if again {
		return river.JobSnooze(resyncDelay)
	}
	return nil
}

// Timeout 是一次同步的時間上限。
func (w *syncWorker) Timeout(*river.Job[SyncArgs]) time.Duration { return jobTimeout }

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
	river.AddWorker(workers, &ensureWorker{service: s})
	river.AddWorker(workers, &syncWorker{service: s})
	river.AddWorker(workers, &sweepWorker{service: s})
	river.AddWorker(workers, &purgeWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的掃描。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(pollInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "payment.account_sweep", RunOnStart: true},
	)}
}

// PurgeArgs 是擁有者個資清理時「關閉外部帳戶後刪除帳戶列」的 job（ForgetOwners 在清理的 transaction 內排入），以 store_id 為唯一鍵。
type PurgeArgs struct {
	StoreID int64 `json:"store_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (PurgeArgs) Kind() string { return "payment.account_purge" }

// InsertOpts 是清理 job 的排入選項：以 store_id 去重（不含已完成的），暫時性的失敗最多重試 maxAttempts 次。
func (PurgeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: maxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: openStates()}}
}

func enqueuePurge(ctx context.Context, jc jobs.Enqueuer, storeID int64) error {
	args := PurgeArgs{StoreID: storeID}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

type purgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	service *Service
}

// Work 執行一次「關閉後刪除」；列在呼叫 provider 之前已由 Purge 標成等待清理，最後一次嘗試仍失敗時留著給週期掃描。
func (w *purgeWorker) Work(ctx context.Context, job *river.Job[PurgeArgs]) error {
	return w.service.Purge(ctx, job.Args.StoreID, job.Attempt >= job.MaxAttempts)
}

// Timeout 是一次清理的時間上限：一次關閉呼叫加資料庫。
func (w *purgeWorker) Timeout(*river.Job[PurgeArgs]) time.Duration { return jobTimeout }

// Sweep 是 payment 自己的輪詢補漏（D29 的補掃分工：platform 只重派沒有派送成功的 inbox 列），各自分批、各批一個 transaction：
//   - 狀態：UNDER_REVIEW、ACTION_REQUIRED、原因碼不是 rejected*／unsupported* 的 DISABLED 帳戶排同步（掉了 webhook 時的保底，DISABLED 可能被解除）；
//   - 事件：inbox 有超過 eventGrace 還沒處理的事件的帳戶排同步（任何狀態）；
//   - 清理：等待清理的帳戶（purge_pending_at，不進上面兩種同步）重排「關閉後刪除」job；沒有人工處理可做，所以不進後台待辦，警示在 job 的 log；
//   - 建立：閒置的建立列，擁有者營運中時排建立 job。
//
// 回傳排入同步與建立的帳戶數（唯一鍵合併掉的也算；兩種同步的帳戶重複時算兩次）。沒有設定 provider 時什麼都不做。
func (s *Service) Sweep(ctx context.Context) (synced, ensured int, err error) {
	p := s.d.Provider
	if p == nil {
		return 0, 0, nil
	}
	queueSync := func(id int64) error {
		return s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueueSync(ctx, s.d.Jobs, id) })
	}
	var after int64
	for {
		ids, err := s.pollable(ctx, after)
		if err != nil {
			return synced, ensured, err
		}
		for _, id := range ids {
			if err := queueSync(id); err != nil {
				return synced, ensured, err
			}
			synced++
			after = id
		}
		if len(ids) < sweepBatch {
			break
		}
	}
	route := ""
	for {
		ids, next, err := s.staleEventStores(ctx, p.Name(), route)
		if err != nil {
			return synced, ensured, err
		}
		for _, id := range ids {
			if err := queueSync(id); err != nil {
				return synced, ensured, err
			}
			synced++
		}
		if next == "" {
			break
		}
		route = next
	}
	after = 0
	for {
		ids, err := s.purgePending(ctx, after)
		if err != nil {
			return synced, ensured, err
		}
		for _, id := range ids {
			after = id
			if err := s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueuePurge(ctx, s.d.Jobs, id) }); err != nil {
				return synced, ensured, err
			}
		}
		if len(ids) < sweepBatch {
			break
		}
	}
	after = 0
	for {
		ids, err := s.stale(ctx, after)
		if err != nil {
			return synced, ensured, err
		}
		for _, id := range ids {
			after = id
			active, err := s.d.Owners.OwnerActive(ctx, id)
			if err != nil {
				return synced, ensured, err
			}
			if !active {
				continue
			}
			if err := s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueueEnsure(ctx, s.d.Jobs, id) }); err != nil {
				return synced, ensured, err
			}
			ensured++
		}
		if len(ids) < sweepBatch {
			return synced, ensured, nil
		}
	}
}
