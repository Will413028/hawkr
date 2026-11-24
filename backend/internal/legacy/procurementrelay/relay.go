// Package procurementrelay 將 procurement.RfqPublished 轉成還在 RabbitMQ 上的消費者（notification-service 的賣家通知模板）所需的訊息。
//
// 每個邀請一個 job：OnRfqPublished 在發布的 transaction 內排入（排入失敗時發布一起回滾），worker 組出 payload 後同步送到
// procurement.events 的 procurement.rfq_publication_invite.created；送出成功把邀請的 delivery_state 轉 SENT，重試用盡轉 FAILED
// （A9-D45(6)）。順序固定先送後標：反過來會留下「說已送出但沒送」；標記失敗而重試造成的重送，由 notification 的
// (user_id, user_type, event_type, reference_id) 唯一索引吸收，reference_id 是 inviteId，每個邀請唯一。
//
// wire 與舊 ai-service 的 inviteCreatedPayload 逐欄位相同（wire_test、golden_test 守住鍵名與型別），沒有 envelope，body 就是 payload；
// 差別只有 inviteId 與 publicationId 的值是 UUID 字串而非十進位 TSID，storeId 仍是 bigint 字串（A9-D45(6)）。
// EXTERNAL 邀請隨 A9-D1 退場，不轉送 procurement.rfq_publication_invite.external_created。
//
// 只在測試啟用：cmd/hawkr 不接線（A9-6 才接）。隨 notification 搬進單體（A8-3 的 rfq_invited 意圖 job 接手投遞與狀態寫入）而刪除。
package procurementrelay

import (
	"context"
	"encoding/json"
	"errors"
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

	maxAttempts = 20
)

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

// Invites 是寫邀請投遞狀態的 port；*procurement.Module 實作它。
type Invites interface {
	MarkInviteSent(ctx context.Context, inviteID uuid.UUID) error
	MarkInviteFailed(ctx context.Context, inviteID uuid.UUID) error
}

// Sender 是同步送出一則訊息的 port；*rabbitmq.Worker 實作它。
type Sender interface {
	Send(ctx context.Context, a rabbitmq.PublishArgs) error
}

// Worker 送出邀請事件並寫投遞狀態。
type Worker struct {
	Invites Invites
	Out     Sender
}

// NewWorker 建立 Worker。
func NewWorker(invites Invites, out Sender) *Worker { return &Worker{Invites: invites, Out: out} }

type typedWorker struct {
	river.WorkerDefaults[InviteCreatedArgs]
	w *Worker
}

func (t *typedWorker) Work(ctx context.Context, job *river.Job[InviteCreatedArgs]) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return t.w.inviteCreated(ctx, job.Args, job.Attempt >= job.MaxAttempts)
}

// AddWorkers 登記 relay 的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker{w: w})
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
	sendErr := w.Out.Send(ctx, rabbitmq.PublishArgs{EventID: a.InviteID.String(), Exchange: exchangeProcurement, RoutingKey: routingKeyInviteMade, Body: body})
	if sendErr == nil {
		return w.Invites.MarkInviteSent(ctx, a.InviteID)
	}
	if !last {
		return sendErr
	}
	if err := w.Invites.MarkInviteFailed(ctx, a.InviteID); err != nil {
		return errors.Join(sendErr, err)
	}
	return sendErr
}
