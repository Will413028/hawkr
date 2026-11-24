// Package procurementrelay 將 procurement.RfqPublished 轉成還在 RabbitMQ 上的消費者（notification-service 的賣家通知模板）所需的訊息。
//
// 每個邀請一個 job：OnRfqPublished 在發布的 transaction 內排入（排入失敗時發布一起回滾），worker 組出 payload 後同步送到
// procurement.events 的 procurement.rfq_publication_invite.created；送出成功把邀請的 delivery_state 轉 SENT，重試用盡轉 FAILED。
// 順序固定先送後標：反過來會留下「說已送出但沒送」；標記失敗而重試造成的重送，由 notification 的
// (user_id, user_type, event_type, reference_id) 唯一索引吸收，reference_id 是 inviteId，每個邀請唯一。
//
// 重試與舊 ai-service 的發布 outbox 相同：最多 5 次，第 n 次失敗後等 2^n 分鐘（上限 1 小時），約 30 分鐘後轉 FAILED。
// 狀態寫入用自己的 ctx（不受送出逾時影響）。送出成功後狀態寫入失敗時，不重跑送出：改排一個只寫狀態的小 job
// （InviteStateArgs，以邀請 unique、有限次重試），送出的 job 隨即完成；連小 job 都排不進去才回錯誤（送出 job 重試，重送由 notification 的唯一索引吸收）。
// job 本身不見的情況（程序崩潰後被 River 救援丟棄、panic、狀態 job 用盡）由週期補掃收斂：建立超過重試期間仍是 QUEUED、
// 且沒有進行中的送出 job 或狀態 job 的邀請轉 FAILED。
//
// wire 與舊 ai-service 的 inviteCreatedPayload 逐欄位相同（wire_test、golden_test 守住鍵名與型別），沒有 envelope，body 就是 payload；
// 差別只有 inviteId 與 publicationId 的值是 UUID 字串而非十進位 TSID，storeId 仍是 bigint 字串。
// EXTERNAL 邀請不轉送 procurement.rfq_publication_invite.external_created。
//
// 只在測試啟用，cmd/hawkr 不接線；隨 notification 搬進單體而刪除。
package procurementrelay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	exchangeProcurement  = "procurement.events"
	routingKeyInviteMade = "procurement.rfq_publication_invite.created"

	// 重試次數與退避沿用舊 ai-service 的發布 outbox（publicationoutbox：maxAttempts 5、2^n 分鐘、上限 1 小時）。
	maxAttempts = 5
	baseBackoff = time.Minute
	maxBackoff  = time.Hour

	sendTimeout  = 25 * time.Second
	stateTimeout = 10 * time.Second
	// stateMaxAttempts 是狀態 job 的重試上限（River 預設退避 attempt^4 秒，約一小時內用盡）；用盡後由補掃收斂。
	stateMaxAttempts = 6

	sweepInterval = 5 * time.Minute
	sweepGrace    = 10 * time.Minute
	sweepBatch    = 200
)

// retryDelay 是第 attempt 次嘗試失敗後到下一次嘗試的等待：2^attempt * baseBackoff，上限 maxBackoff。
func retryDelay(attempt int) time.Duration {
	d := baseBackoff
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	return d
}

// retryWindow 是一個邀請從排入到最後一次嘗試結束最長的時間：各次退避加每次嘗試的送出上限。
func retryWindow() time.Duration {
	w := maxAttempts * sendTimeout
	for a := 1; a < maxAttempts; a++ {
		w += retryDelay(a)
	}
	return w
}

// staleAfter 是補掃認定一個 QUEUED 邀請已經超過重試期間的年齡；與重試參數同源。
func staleAfter() time.Duration { return retryWindow() + sweepGrace }

// Delivery states the relay writes; the values are the invite's delivery_state.
const (
	stateSent   = "SENT"
	stateFailed = "FAILED"
)

// InviteStateArgs 只寫一個邀請的終態：送出已經成功（或確定用盡）而狀態寫入失敗時排入，重試的是寫入、不是送出。
type InviteStateArgs struct {
	InviteID uuid.UUID `json:"invite_id" river:"unique"`
	State    string    `json:"state"`
}

func (InviteStateArgs) Kind() string { return "legacy.procurement_invite_state" }

// InsertOpts 以邀請 ID 合併尚未結束的狀態 job，重試次數有限。
func (InviteStateArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(stateMaxAttempts) }

// InviteCreatedArgs 轉成一則 procurement.rfq_publication_invite.created。
type InviteCreatedArgs struct {
	InviteID      uuid.UUID `json:"invite_id" river:"unique"`
	PublicationID uuid.UUID `json:"publication_id"`
	StoreID       int64     `json:"store_id"`
}

func (InviteCreatedArgs) Kind() string { return "legacy.procurement_invite_created" }

// InsertOpts 以邀請 ID 做 unique，重試次數用盡時才標 FAILED，所以上限要固定在排入時。
func (InviteCreatedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: maxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// OnRfqPublished 在發布事件的 transaction 內為每個邀請排入一個 job。
func OnRfqPublished(q jobs.Enqueuer) func(context.Context, procurement.RfqPublished) error {
	return func(ctx context.Context, e procurement.RfqPublished) error {
		for _, inv := range e.Invites {
			if err := q.Enqueue(ctx, InviteCreatedArgs{InviteID: inv.InviteID, PublicationID: e.PublicationID, StoreID: inv.StoreID}, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

// Invites 是邀請投遞狀態的 port；*procurement.Module 實作它。
type Invites interface {
	MarkInviteSent(ctx context.Context, inviteID uuid.UUID) error
	MarkInviteFailed(ctx context.Context, inviteID uuid.UUID) error
	StaleQueuedInvites(ctx context.Context, before time.Time, after procurement.StaleInvite, limit int32) ([]procurement.StaleInvite, error)
}

// Sender 是同步送出一則訊息的 port；*rabbitmq.Worker 實作它。
type Sender interface {
	Send(ctx context.Context, a rabbitmq.PublishArgs) error
}

// LiveJobs 回答某個邀請是否還有進行中（排隊、排程、等重試、執行中）的送出 job 或狀態 job。
type LiveJobs interface {
	HasLiveJob(ctx context.Context, inviteID uuid.UUID) (bool, error)
}

// ClientJobs 以 platform/jobs 的 Client 實作 LiveJobs。
type ClientJobs struct{ Client *jobs.Client }

// HasLiveJob 實作 LiveJobs。
func (c ClientJobs) HasLiveJob(ctx context.Context, inviteID uuid.UUID) (bool, error) {
	return c.Client.HasUnfinishedJob(ctx, []string{InviteCreatedArgs{}.Kind(), InviteStateArgs{}.Kind()}, "invite_id", inviteID.String())
}

// Worker 送出邀請事件並寫投遞狀態。
type Worker struct {
	Invites Invites
	Out     Sender
	Live    LiveJobs
	// Queue 排入狀態 job。
	Queue jobs.Enqueuer
	// Now 是補掃的時鐘；NewWorker 設為 time.Now。
	Now func() time.Time
	// batch 是補掃一頁的大小；NewWorker 設為 sweepBatch，測試縮小它。
	batch int32
}

// NewWorker 建立 Worker。
func NewWorker(invites Invites, out Sender, live LiveJobs, queue jobs.Enqueuer) *Worker {
	return &Worker{Invites: invites, Out: out, Live: live, Queue: queue, Now: time.Now, batch: sweepBatch}
}

type typedWorker struct {
	river.WorkerDefaults[InviteCreatedArgs]
	w *Worker
}

func (t *typedWorker) Work(ctx context.Context, job *river.Job[InviteCreatedArgs]) error {
	return t.w.inviteCreated(ctx, job.Args, job.Attempt >= job.MaxAttempts)
}

// NextRetry 沿用舊 outbox 的退避，取代 River 預設的 attempt^4 秒。
func (t *typedWorker) NextRetry(job *river.Job[InviteCreatedArgs]) time.Time {
	return time.Now().Add(retryDelay(job.Attempt))
}

// SweepArgs 是補掃 job；週期排入，沒有參數。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "legacy.procurement_invite_sweep" }

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	w *Worker
}

func (s *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, err := s.w.Sweep(ctx)
	return err
}

func (s *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return 5 * time.Minute }

type stateWorker struct {
	river.WorkerDefaults[InviteStateArgs]
	w *Worker
}

func (s *stateWorker) Work(ctx context.Context, job *river.Job[InviteStateArgs]) error {
	mark, err := s.w.markFunc(job.Args.State)
	if err != nil {
		return river.JobCancel(err)
	}
	return runWithOwnCtx(ctx, mark, job.Args.InviteID)
}

func (w *Worker) markFunc(state string) (func(context.Context, uuid.UUID) error, error) {
	switch state {
	case stateSent:
		return w.Invites.MarkInviteSent, nil
	case stateFailed:
		return w.Invites.MarkInviteFailed, nil
	}
	return nil, fmt.Errorf("procurementrelay：未知的投遞狀態 %q", state)
}

// AddWorkers 登記 relay 與補掃的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker{w: w})
	river.AddWorker(workers, &stateWorker{w: w})
	river.AddWorker(workers, &sweepWorker{w: w})
}

// PeriodicJobs 回傳補掃的排程（啟動即執行）。
func (w *Worker) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(sweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "legacy.procurement_invite_sweep", RunOnStart: true},
	)}
}

// Sweep 把建立超過重試期間仍是 QUEUED、且沒有進行中 job（送出或狀態）的邀請轉 FAILED，回傳轉了幾個。
// 正常路徑由 inviteCreated 在最後一次嘗試寫 FAILED；補掃只收 job 不見而到不了那裡的邀請。
// 以 (created_at, id) 游標分頁，越過還有進行中 job 的列，不會被排在前面的活躍列擋住。
func (w *Worker) Sweep(ctx context.Context) (int, error) {
	marked := 0
	before := w.Now().Add(-staleAfter())
	var cursor procurement.StaleInvite
	for {
		page, err := w.Invites.StaleQueuedInvites(ctx, before, cursor, w.batch)
		if err != nil {
			return marked, err
		}
		for _, inv := range page {
			live, err := w.Live.HasLiveJob(ctx, inv.ID)
			if err != nil {
				return marked, err
			}
			if live {
				continue
			}
			if err := w.Invites.MarkInviteFailed(ctx, inv.ID); err != nil {
				return marked, err
			}
			marked++
		}
		if len(page) < int(w.batch) {
			return marked, nil
		}
		cursor = page[len(page)-1]
	}
}

// inviteCreatedPayload 是舊 ai-service 的 inviteCreatedPayload（commerce/publication_execute.go）：三個鍵、全字串。
type inviteCreatedPayload struct {
	InviteID      string `json:"inviteId"`
	PublicationID string `json:"publicationId"`
	StoreID       string `json:"storeId"`
}

func payload(a InviteCreatedArgs) ([]byte, error) {
	return json.Marshal(inviteCreatedPayload{
		InviteID: a.InviteID.String(), PublicationID: a.PublicationID.String(), StoreID: strconv.FormatInt(a.StoreID, 10),
	})
}

// inviteCreated 先送後標：送出失敗且不是最後一次嘗試，回錯誤讓 River 重試；最後一次嘗試失敗把邀請標 FAILED 再回原錯誤。
// 送出成功後絕不再送：狀態寫入失敗時只重試寫入（settle）。
func (w *Worker) inviteCreated(ctx context.Context, a InviteCreatedArgs, last bool) error {
	body, err := payload(a)
	if err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	sendErr := w.Out.Send(sendCtx, rabbitmq.PublishArgs{Exchange: exchangeProcurement, RoutingKey: routingKeyInviteMade, Body: body})
	cancel()
	if sendErr == nil {
		return w.settle(ctx, a.InviteID, stateSent)
	}
	if !last {
		return sendErr
	}
	if err := w.settle(ctx, a.InviteID, stateFailed); err != nil {
		return errors.Join(sendErr, err)
	}
	return sendErr
}

// settle 寫投遞狀態，用自己的 ctx：送出用掉的時間或取消不影響它。寫入失敗時改排狀態 job 接手重試並回 nil；
// 連排入都失敗才回兩個錯誤，由補掃收斂。
func (w *Worker) settle(ctx context.Context, id uuid.UUID, state string) error {
	mark, err := w.markFunc(state)
	if err != nil {
		return err
	}
	markErr := runWithOwnCtx(ctx, mark, id)
	if markErr == nil {
		return nil
	}
	ectx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stateTimeout)
	defer cancel()
	if err := w.Queue.Enqueue(ectx, InviteStateArgs{InviteID: id, State: state}, nil); err != nil {
		return errors.Join(markErr, err)
	}
	return nil
}

func runWithOwnCtx(ctx context.Context, mark func(context.Context, uuid.UUID) error, id uuid.UUID) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stateTimeout)
	defer cancel()
	return mark(wctx, id)
}
