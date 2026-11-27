// Package buyerrelay 將 buyer 模組事件轉成尚未搬遷的 RabbitMQ 消費者所需訊息：只有 buyer.deleted。
// 追蹤、商品瀏覽與補貨的轉送已隨 A8-3d1 刪除，收藏數由 catalog 在收藏的同一個 transaction 內更新（B4-4c），通知由 notification 模組訂閱 buyer 事件。
package buyerrelay

import (
	"context"
	"encoding/json"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type eventFields struct {
	EventID  string    `json:"event_id"`
	Occurred time.Time `json:"occurred"`
	BuyerID  int64     `json:"buyer_id"`
}

type BuyerDeletedArgs struct {
	eventFields
	AccountID int64 `json:"account_id"`
}

func (BuyerDeletedArgs) Kind() string { return "legacy.buyer_deleted" }

type Enqueue func(context.Context, river.JobArgs) error

func NewEnqueue(j jobs.Enqueuer) Enqueue {
	return func(ctx context.Context, args river.JobArgs) error { return j.Enqueue(ctx, args, nil) }
}

func OnBuyerDeleted(enqueue Enqueue) func(context.Context, buyer.BuyerDeleted) error {
	return func(ctx context.Context, e buyer.BuyerDeleted) error {
		return enqueue(ctx, BuyerDeletedArgs{eventFields: eventFields{e.EventID.String(), e.OccurredAt, e.BuyerID}, AccountID: e.AccountID})
	}
}

type Worker struct {
	Jobs jobs.Enqueuer
}

func NewWorker(queue jobs.Enqueuer) *Worker {
	return &Worker{Jobs: queue}
}

type typedWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
	work func(context.Context, T) error
}

func (w *typedWorker[T]) Work(ctx context.Context, job *river.Job[T]) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return w.work(ctx, job.Args)
}

func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker[BuyerDeletedArgs]{work: w.buyerDeleted})
}

func base(a eventFields, eventType string) map[string]any {
	return map[string]any{
		"event_id": a.EventID, "event_type": eventType, "version": "1.0",
		"timestamp": a.Occurred.UTC(), "source": "buyer-service", "metadata": map[string]string{},
	}
}

func (w *Worker) enqueuePublish(ctx context.Context, a eventFields, exchange, key string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return w.Jobs.Enqueue(ctx, rabbitmq.PublishArgs{EventID: a.EventID, Exchange: exchange, RoutingKey: key, Body: body}, rabbitmq.InsertOpts())
}

func (w *Worker) buyerDeleted(ctx context.Context, a BuyerDeletedArgs) error {
	payload := base(a.eventFields, "BUYER_DELETED")
	payload["accountId"], payload["buyerId"] = a.AccountID, a.BuyerID
	return w.enqueuePublish(ctx, a.eventFields, "hawkr.buyer", "buyer.deleted", payload)
}
