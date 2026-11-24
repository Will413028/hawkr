package charge

import (
	"context"
	"errors"
	"time"

	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
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

// CancelArgs 是取消舊 PaymentIntent 的 job，以嘗試 ID 為唯一鍵：舊 PaymentIntent 晚到成功、付款列已經入帳時，另一個還沒結案的 PaymentIntent 由它取消（transaction 外呼叫 provider）。
type CancelArgs struct {
	AttemptID uuid.UUID `json:"attempt_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (CancelArgs) Kind() string { return "payment.charge_cancel" }

// InsertOpts 是取消 job 的排入選項。
func (CancelArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

func enqueueCancel(ctx context.Context, jc jobs.Enqueuer, attemptID uuid.UUID) error {
	args := CancelArgs{AttemptID: attemptID}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// TransferArgs 是建立轉帳的 job，以轉帳 ID 為唯一鍵：RequestTransfer 在寫列的同一個 transaction 排入，由它在 transaction 外帶列上的冪等鍵呼叫 provider。
// WakeRefundID 不是零值時是「這筆退款進終局，轉帳等的條件變了」的喚醒：它進唯一鍵，所以不會被同一筆轉帳正在 snooze 的 job 合併掉。
type TransferArgs struct {
	TransferID   uuid.UUID `json:"transfer_id" river:"unique"`
	WakeRefundID uuid.UUID `json:"wake_refund_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (TransferArgs) Kind() string { return "payment.transfer_create" }

// InsertOpts 是轉帳 job 的排入選項。
func (TransferArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

func enqueueTransfer(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID) error {
	args := TransferArgs{TransferID: id}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// enqueueTransferWake 在退款進終局的 transaction 內排入喚醒轉帳的 job（同一筆退款只排一次）。
func enqueueTransferWake(ctx context.Context, jc jobs.Enqueuer, id, refundID uuid.UUID) error {
	args := TransferArgs{TransferID: id, WakeRefundID: refundID}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// ReversalArgs 是撤回轉帳的 job，以退款 ID 為唯一鍵：provider 接受退款與轉帳成立兩個事件中後到的那個，在把退款列的撤回狀態推進到 REVERSING 的同一個 transaction 排入（D11）。
type ReversalArgs struct {
	RefundID uuid.UUID `json:"refund_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (ReversalArgs) Kind() string { return "payment.transfer_reversal" }

// InsertOpts 是撤回 job 的排入選項。
func (ReversalArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

func enqueueReversal(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID) error {
	args := ReversalArgs{RefundID: id}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// TopupArgs 是補差額轉帳給賣家的 job，以退款 ID 為唯一鍵：已算進轉帳金額的退款失敗與轉帳成立兩個事件中後到的那個，在把退款列的補差額狀態推進到 SENDING 的同一個 transaction 排入（D11 補充）。
type TopupArgs struct {
	RefundID uuid.UUID `json:"refund_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (TopupArgs) Kind() string { return "payment.transfer_topup" }

// InsertOpts 是補差額 job 的排入選項。
func (TopupArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

func enqueueTopup(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID) error {
	args := TopupArgs{RefundID: id}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// RefundArgs 是呼叫 provider 建立退款的 job，以退款 ID 為唯一鍵：CreateRefund 在寫 PENDING 退款列的同一個 transaction 排入（轉帳的任何狀態都可以退款，向賣家收回在 provider 接受之後才開始）。
type RefundArgs struct {
	RefundID uuid.UUID `json:"refund_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (RefundArgs) Kind() string { return "payment.refund_create" }

// InsertOpts 是退款 job 的排入選項。
func (RefundArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

func enqueueRefund(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID) error {
	args := RefundArgs{RefundID: id}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// RefundSyncArgs 是重讀 provider 的退款並寫入狀態的 job，以退款 ID 為唯一鍵：退款事件的 handler 與掃描只排這個 job。
type RefundSyncArgs struct {
	RefundID uuid.UUID `json:"refund_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (RefundSyncArgs) Kind() string { return "payment.refund_sync" }

// InsertOpts 是退款同步 job 的排入選項。
func (RefundSyncArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

func enqueueRefundSync(ctx context.Context, jc jobs.Enqueuer, id uuid.UUID) error {
	args := RefundSyncArgs{RefundID: id}
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

// closeWaitDelay 是關閉付款等建立收斂時 job 的 snooze 間隔（不計嘗試）。
const closeWaitDelay = 5 * time.Minute

// transferWaitDelay 是轉帳等進行中的退款終局時 job 的 snooze 間隔，與掃描同一個週期；退款進終局時的喚醒 job 才是快路徑。
const transferWaitDelay = pollInterval

// transferJobResult 把一次轉帳的結果轉成 River 的回傳：等進行中的退款是預期的等待，snooze（不計嘗試、不當成錯誤）；其他照 jobError。
func transferJobResult(err error) error {
	if errors.Is(err, errRefundsInFlight) {
		return river.JobSnooze(transferWaitDelay)
	}
	return jobError(err)
}

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

type cancelWorker struct {
	river.WorkerDefaults[CancelArgs]
	service *Service
}

// Work 執行一次取消：provider 的永久拒絕取消 job、不重試。
func (w *cancelWorker) Work(ctx context.Context, job *river.Job[CancelArgs]) error {
	return jobError(w.service.CancelAttempt(ctx, job.Args.AttemptID))
}

// Timeout 是一次取消的時間上限。
func (w *cancelWorker) Timeout(*river.Job[CancelArgs]) time.Duration { return jobTimeout }

type transferWorker struct {
	river.WorkerDefaults[TransferArgs]
	service *Service
}

// Work 執行一次轉帳：provider 的永久拒絕進終局、不重試；平台餘額不足等暫時性失敗回錯誤由 River 重試；等進行中的退款終局時 snooze。
func (w *transferWorker) Work(ctx context.Context, job *river.Job[TransferArgs]) error {
	return transferJobResult(w.service.Transfer(ctx, job.Args.TransferID, job.Attempt >= job.MaxAttempts))
}

// Timeout 是一次轉帳的時間上限。
func (w *transferWorker) Timeout(*river.Job[TransferArgs]) time.Duration { return jobTimeout }

type reversalWorker struct {
	river.WorkerDefaults[ReversalArgs]
	service *Service
}

// Work 執行一次撤回。
func (w *reversalWorker) Work(ctx context.Context, job *river.Job[ReversalArgs]) error {
	return jobError(w.service.Reverse(ctx, job.Args.RefundID, job.Attempt >= job.MaxAttempts))
}

// Timeout 是一次撤回的時間上限。
func (w *reversalWorker) Timeout(*river.Job[ReversalArgs]) time.Duration { return jobTimeout }

type topupWorker struct {
	river.WorkerDefaults[TopupArgs]
	service *Service
}

// Work 執行一次補差額轉帳。
func (w *topupWorker) Work(ctx context.Context, job *river.Job[TopupArgs]) error {
	return jobError(w.service.TopUp(ctx, job.Args.RefundID, job.Attempt >= job.MaxAttempts))
}

// Timeout 是一次補差額轉帳的時間上限。
func (w *topupWorker) Timeout(*river.Job[TopupArgs]) time.Duration { return jobTimeout }

type refundWorker struct {
	river.WorkerDefaults[RefundArgs]
	service *Service
}

// Work 執行一次退款呼叫：provider 的永久拒絕進終局、不重試；暫時性失敗回錯誤由 River 重試。
func (w *refundWorker) Work(ctx context.Context, job *river.Job[RefundArgs]) error {
	return jobError(w.service.Refund(ctx, job.Args.RefundID, job.Attempt >= job.MaxAttempts))
}

// Timeout 是一次退款呼叫的時間上限。
func (w *refundWorker) Timeout(*river.Job[RefundArgs]) time.Duration { return jobTimeout }

type refundSyncWorker struct {
	river.WorkerDefaults[RefundSyncArgs]
	service *Service
}

// Work 執行一次退款同步。
func (w *refundSyncWorker) Work(ctx context.Context, job *river.Job[RefundSyncArgs]) error {
	again, err := w.service.SyncRefund(ctx, job.Args.RefundID)
	if err != nil {
		return jobError(err)
	}
	if again {
		return river.JobSnooze(resyncDelay)
	}
	return nil
}

// Timeout 是一次退款同步的時間上限。
func (w *refundSyncWorker) Timeout(*river.Job[RefundSyncArgs]) time.Duration { return jobTimeout }

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
	river.AddWorker(workers, &cancelWorker{service: s})
	river.AddWorker(workers, &closeWorker{service: s})
	river.AddWorker(workers, &transferWorker{service: s})
	river.AddWorker(workers, &reversalWorker{service: s})
	river.AddWorker(workers, &topupWorker{service: s})
	river.AddWorker(workers, &refundWorker{service: s})
	river.AddWorker(workers, &refundSyncWorker{service: s})
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
//   - 建立：還在建立中、閒置超過 createIdle 的列補排補做 job（結果不明的列由補做 job 依 ResendWindow 同鍵重送或查回）；
//   - 狀態：已建立、未結案、超過 readIdle 沒讀過的列補排同步（掉了 webhook 的保底）；
//   - 舊嘗試：被取代、還沒結案又超過 readIdle 沒讀過的舊 PaymentIntent 所屬的付款補排同步；
//   - 事件：inbox 有超過 eventGrace 還沒處理的事件的付款補排同步（任何狀態）；
//   - 轉帳、撤回、補差額與退款：PENDING 的轉帳、撤回狀態是 REVERSING 或補差額狀態是 SENDING 的退款閒置超過 createIdle 就補排各自的 job，PENDING 的退款補排退款 job（結果不明的由 job 依 ResendWindow 同鍵重送或查回）；
//     REFUNDING 的退款閒置超過 readIdle 補排退款同步；inbox 有超過 eventGrace 還沒處理的退款事件的退款補排退款同步；ANOMALY 的列一律不補排。
//
// 回傳排入補做與同步的付款數（唯一鍵合併掉的也算；同一筆付款符合兩種同步時算兩次；補排的轉帳、撤回與退款呼叫算在 created，退款同步算在 synced）。沒有設定 provider 時什麼都不做。
func (s *Service) Sweep(ctx context.Context) (created, synced int, err error) {
	p := s.d.Provider
	if p == nil {
		return 0, 0, nil
	}
	if _, ok := db.TxFrom(ctx); ok {
		return 0, 0, errInTransaction
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
		ids, err := store.New(s.d.DB.Conn(ctx)).PendingTransferIDs(ctx, store.PendingTransferIDsParams{IdleBefore: createBefore, AfterID: after, Batch: sweepBatch})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueTransfer, id); err != nil {
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
		ids, err := store.New(s.d.DB.Conn(ctx)).ReversingRefundIDs(ctx, store.ReversingRefundIDsParams{IdleBefore: createBefore, AfterID: after, Batch: sweepBatch})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueReversal, id); err != nil {
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
		ids, err := store.New(s.d.DB.Conn(ctx)).SendingTopupRefundIDs(ctx, store.SendingTopupRefundIDsParams{IdleBefore: createBefore, AfterID: after, Batch: sweepBatch})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueTopup, id); err != nil {
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
		ids, err := store.New(s.d.DB.Conn(ctx)).CallableRefundIDs(ctx, store.CallableRefundIDsParams{IdleBefore: createBefore, AfterID: after, Batch: sweepBatch})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueRefund, id); err != nil {
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
		ids, err := store.New(s.d.DB.Conn(ctx)).RefundingRefundIDs(ctx, store.RefundingRefundIDsParams{IdleBefore: readBefore, AfterID: after, Batch: sweepBatch})
		if err != nil {
			return created, synced, err
		}
		for _, id := range ids {
			if err := queue(enqueueRefundSync, id); err != nil {
				return created, synced, err
			}
			synced++
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
	after = uuid.Nil()
	for {
		ids, err := store.New(s.d.DB.Conn(ctx)).StaleAttemptPaymentIDs(ctx, store.StaleAttemptPaymentIDsParams{ReadBefore: readBefore, AfterID: after, Batch: sweepBatch})
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
		refunds, err := store.New(s.d.DB.Conn(ctx)).RefundIDsByExternal(ctx, store.RefundIDsByExternalParams{Provider: p.Name(), ExternalRefundIds: routes})
		if err != nil {
			return created, synced, err
		}
		for _, id := range refunds {
			if err := queue(enqueueRefundSync, id); err != nil {
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
