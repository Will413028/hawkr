// Package procurementrelay 將 procurement.RfqPublished 轉成還在 RabbitMQ 上的消費者（notification-service 的賣家通知模板）所需的訊息。
//
// 每個邀請一個 job：OnRfqPublished 在發布的 transaction 內排入（排入失敗時發布一起回滾），worker 組出 payload 後同步送到
// procurement.events 的 procurement.rfq_publication_invite.created；送出成功把邀請的 delivery_state 轉 SENT，重試用盡轉 FAILED。
// 順序固定先送後標：反過來會留下「說已送出但沒送」；標記失敗而重試造成的重送，由 notification 的
// (user_id, user_type, event_type, reference_id) 唯一索引吸收，reference_id 是 inviteId，每個邀請唯一。
//
// 重試與舊 ai-service 的發布 outbox 相同：最多 5 次，第 n 次失敗後等 2^n 分鐘（上限 1 小時），約 30 分鐘後轉 FAILED。
// 狀態寫入用自己的 ctx（不受送出逾時影響）；最後一次嘗試的狀態寫入失敗時 job 延後重跑（不消耗嘗試次數），不會丟棄。
// job 本身不見的情況（程序崩潰後被 River 救援丟棄、panic）由週期補掃收斂：建立超過重試期間仍是 QUEUED、
// 且沒有進行中的 job 的邀請轉 FAILED。
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
	"strconv"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
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
	// stateRetryDelay 是最後一次嘗試的狀態寫入失敗時，job 延後重跑的間隔。
	stateRetryDelay = time.Minute

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
	StaleQueuedInvites(ctx context.Context, before time.Time, limit int32) ([]uuid.UUID, error)
}

// Sender 是同步送出一則訊息的 port；*rabbitmq.Worker 實作它。
type Sender interface {
	Send(ctx context.Context, a rabbitmq.PublishArgs) error
}

// LiveJobs 回答某個邀請是否還有進行中（排隊、排程、等重試、執行中）的轉送 job。
type LiveJobs interface {
	HasLiveJob(ctx context.Context, inviteID uuid.UUID) (bool, error)
}

// PoolJobs 以 River 的 job 表實作 LiveJobs。
type PoolJobs struct{ Pool *pgxpool.Pool }

// HasLiveJob 實作 LiveJobs。
func (p PoolJobs) HasLiveJob(ctx context.Context, inviteID uuid.UUID) (bool, error) {
	var live bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+jobs.Schema+`.river_job
		WHERE kind = $1 AND args->>'invite_id' = $2 AND state IN ('available', 'pending', 'retryable', 'running', 'scheduled'))`,
		InviteCreatedArgs{}.Kind(), inviteID.String()).Scan(&live)
	return live, err
}

// Worker 送出邀請事件並寫投遞狀態。
type Worker struct {
	Invites Invites
	Out     Sender
	Live    LiveJobs
	// Now 是補掃的時鐘；NewWorker 設為 time.Now。
	Now func() time.Time
}

// NewWorker 建立 Worker。
func NewWorker(invites Invites, out Sender, live LiveJobs) *Worker {
	return &Worker{Invites: invites, Out: out, Live: live, Now: time.Now}
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

// AddWorkers 登記 relay 與補掃的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker{w: w})
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

// Sweep 把建立超過重試期間仍是 QUEUED、且沒有進行中 job 的邀請轉 FAILED，回傳轉了幾個。
// 正常路徑由 inviteCreated 在最後一次嘗試寫 FAILED；補掃只收 job 不見而到不了那裡的邀請。
func (w *Worker) Sweep(ctx context.Context) (int, error) {
	marked := 0
	for {
		ids, err := w.Invites.StaleQueuedInvites(ctx, w.Now().Add(-staleAfter()), sweepBatch)
		if err != nil {
			return marked, err
		}
		progress := 0
		for _, id := range ids {
			live, err := w.Live.HasLiveJob(ctx, id)
			if err != nil {
				return marked, err
			}
			if live {
				continue
			}
			if err := w.Invites.MarkInviteFailed(ctx, id); err != nil {
				return marked, err
			}
			progress++
		}
		marked += progress
		if len(ids) < sweepBatch || progress == 0 {
			return marked, nil
		}
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
func (w *Worker) inviteCreated(ctx context.Context, a InviteCreatedArgs, last bool) error {
	body, err := payload(a)
	if err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	sendErr := w.Out.Send(sendCtx, rabbitmq.PublishArgs{Exchange: exchangeProcurement, RoutingKey: routingKeyInviteMade, Body: body})
	cancel()
	if sendErr == nil {
		return writeState(ctx, last, w.Invites.MarkInviteSent, a.InviteID)
	}
	if !last {
		return sendErr
	}
	if err := writeState(ctx, last, w.Invites.MarkInviteFailed, a.InviteID); err != nil {
		return err
	}
	return sendErr
}

// writeState 寫投遞狀態，用自己的 ctx：送出用掉的時間或取消不影響它。最後一次嘗試寫失敗時回延後重跑（不消耗嘗試次數），
// 否則 River 會丟棄 job 而邀請停在 QUEUED；其餘嘗試回錯誤，走一般重試。
func writeState(ctx context.Context, last bool, mark func(context.Context, uuid.UUID) error, id uuid.UUID) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stateTimeout)
	defer cancel()
	err := mark(wctx, id)
	if err == nil {
		return nil
	}
	if last {
		return river.JobSnooze(stateRetryDelay)
	}
	return err
}
