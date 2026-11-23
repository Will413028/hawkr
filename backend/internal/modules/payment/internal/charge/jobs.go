package charge

import (
	"context"
	"errors"
	"time"

	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// CreateArgs 是建立 PaymentIntent 的補做 job，以付款 ID 為唯一鍵：同一筆付款同時最多一個排隊中的補做。請求內的建立失敗或逾時之後由它以列上同一把冪等鍵接手。
type CreateArgs struct {
	PaymentID uuid.UUID `json:"payment_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (CreateArgs) Kind() string { return "payment.charge_create" }

// InsertOpts 是補做 job 的排入選項：以付款 ID 去重（不含已完成的），暫時性的失敗最多重試 maxAttempts 次。
func (CreateArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

// enqueueCreate 立刻排入補做（掃描用：列已經閒置）。
func enqueueCreate(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID) error {
	return enqueueCreateAt(ctx, jc, id, time.Time{})
}

// enqueueCreateAt 排入補做，at 不是零值時延到那個時刻才跑：請求內的建立還在進行時，補做不能與它搶同一把冪等鍵（Stripe 對並行的同鍵請求回 409，也會讓先寫入的一方
// 讓請求拿不到 client_secret），所以建立付款時排在請求內呼叫的上限之後。
func enqueueCreateAt(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID, at time.Time) error {
	args := CreateArgs{PaymentID: id}
	opts := args.InsertOpts()
	if !at.IsZero() {
		opts.ScheduledAt = at
	}
	return jc.Enqueue(ctx, args, &opts)
}

// SyncArgs 是重讀 PaymentIntent 並寫入狀態的 job，以付款 ID 為唯一鍵：webhook、輪詢與 GetPayment 發現 provider 已往前都只排這個 job，連續的請求合併成一次。
type SyncArgs struct {
	PaymentID uuid.UUID `json:"payment_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (SyncArgs) Kind() string { return "payment.charge_sync" }

// InsertOpts 是同步 job 的排入選項。
func (SyncArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

func enqueueSync(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID) error {
	args := SyncArgs{PaymentID: id}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// SweepArgs 是定期的掃描：補排閒置的建立與未結案付款的同步，以及有舊的未處理事件的付款的同步。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "payment.charge_sweep" }

// InsertOpts 把同一段時間內的排程與重啟合併。
func (SweepArgs) InsertOpts() river.InsertOpts { return jobs.SweepOpts(10 * time.Minute) }

// resyncDelay 是結果寫入後仍有未涵蓋的事件時，等多久再跑一輪。
const resyncDelay = 2 * time.Second

// pollInterval 是掃描的週期。
const pollInterval = 15 * time.Minute

func jobError(err error) error {
	if errors.Is(err, ErrPermanent) {
		return river.JobCancel(err)
	}
	return err
}

type createWorker struct {
	river.WorkerDefaults[CreateArgs]
	service *Service
}

// Work 執行一次補做；最後一次嘗試仍失敗時由 Create 記警示並留給週期掃描；快照與列不一致（ErrAnomaly）是永久失敗，取消、不重試。
func (w *createWorker) Work(ctx context.Context, job *river.Job[CreateArgs]) error {
	return jobError(w.service.Create(ctx, job.Args.PaymentID, job.Attempt >= job.MaxAttempts))
}

// Timeout 是一次補做的時間上限。
func (w *createWorker) Timeout(*river.Job[CreateArgs]) time.Duration { return jobTimeout }

type syncWorker struct {
	river.WorkerDefaults[SyncArgs]
	service *Service
}

// Work 執行一次同步：結果寫入後仍有未涵蓋的事件時 snooze 再跑一輪（不計一次嘗試）；快照與列不一致等永久失敗取消、不重試。
func (w *syncWorker) Work(ctx context.Context, job *river.Job[SyncArgs]) error {
	again, err := w.service.Sync(ctx, job.Args.PaymentID)
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
	river.AddWorker(workers, &createWorker{service: s})
	river.AddWorker(workers, &syncWorker{service: s})
	river.AddWorker(workers, &sweepWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的掃描。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(pollInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "payment.charge_sweep", RunOnStart: true},
	)}
}

// Sweep 是付款自己的補掃（D63 的分工同收款帳戶：platform 只重派沒有派送成功的 inbox 列），每個付款在自己的 transaction 內排入一個 job：
//   - 建立：還在建立中、閒置超過 createIdle 的列補排補做 job（呼叫過、沒有外部 ID 的列在 UnresolvedDeadline 內以同一把鍵重送，超過由補做 job 進終局）；
//   - 狀態：已建立、未結案、超過 readIdle 沒讀過的列補排同步（掉了 webhook 的保底）；
//   - 事件：inbox 有超過 eventGrace 還沒處理的事件的付款補排同步（任何狀態）。
//
// 回傳排入補做與同步的付款數（唯一鍵合併掉的也算；同一筆付款符合兩種同步時算兩次）。沒有設定 provider 時什麼都不做。
func (s *Service) Sweep(ctx context.Context) (created, synced int, err error) {
	p := s.d.Provider
	if p == nil {
		return 0, 0, nil
	}
	queue := func(enqueue func(context.Context, jobs.Enqueuer, uuid.UUID) error, id uuid.UUID) error {
		return s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueue(ctx, s.d.Jobs, id) })
	}
	now := s.d.Now().UTC()
	createBefore, readBefore := now.Add(-createIdle), now.Add(-readIdle)
	var after uuid.UUID
	for {
		ids, err := store.New(s.d.DB.Conn(ctx)).CreatingPaymentIDs(ctx, store.CreatingPaymentIDsParams{IdleBefore: &createBefore, AfterID: after, Batch: sweepBatch})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueCreate, id); err != nil {
				return created, synced, err
			}
			created++
			after = id
		}
		if len(ids) < sweepBatch {
			break
		}
	}
	after = uuid.Nil()
	for {
		ids, err := store.New(s.d.DB.Conn(ctx)).PollablePaymentIDs(ctx, store.PollablePaymentIDsParams{ReadBefore: &readBefore, AfterID: after, Batch: sweepBatch})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueSync, id); err != nil {
				return created, synced, err
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
		routes, err := s.d.Inbox.StaleRoutes(ctx, p.Name(), now.Add(-eventGrace), route, sweepBatch)
		if err != nil || len(routes) == 0 {
			return created, synced, err
		}
		ids, err := store.New(s.d.DB.Conn(ctx)).PaymentIDsByExternal(ctx, store.PaymentIDsByExternalParams{Provider: p.Name(), ExternalPaymentIds: routes})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueSync, id); err != nil {
				return created, synced, err
			}
			synced++
		}
		if len(routes) < sweepBatch {
			return created, synced, nil
		}
		route = routes[len(routes)-1]
	}
}
