// Package sellerconsumer 把 Java payuni-webhook-service 的 AGENT_NOTIFY（RabbitMQ 的 seller.review.result）轉給 seller 模組。
// 它只做轉接：解開 envelope、轉成模組的通知，由模組驗證形狀並排入以 event_key 為唯一鍵的 River job（持久化之後才 ack）；
// 套用在 job。形狀不合的訊息重送也不會好，記錄後進 dead-letter queue，不阻塞後面的訊息；其他失敗 requeue。
// 在 worker（cmd/hawkr 的 runWorker）執行，是 seller.review.result 唯一的消費者；單體用自己的 queue
// （hawkr.monolith.seller.review.result，由 consumer 自己宣告，不在 topology）。
package sellerconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Notifier 是 seller 模組接收 PayUni 通知的入口。
type Notifier interface {
	ReceivePayUniNotification(context.Context, seller.PayUniNotification) error
}

// Consumer 消費 PayUni 的商家審核通知。
type Consumer struct {
	Seller Notifier
}

// envelope 是 Java WebhookReceiveService.buildEventEnvelope 的 envelope（snake_case）：payload 是 PayUni 解密後的欄位
// （AGENT_NOTIFY 有 MerID、MerStatus、MerStatusNote、ExecuteTime），event_key 是 merId:MerStatus:ExecuteTime。
type envelope struct {
	MerID    string            `json:"mer_id"`
	EventKey string            `json:"event_key"`
	Payload  map[string]string `json:"payload"`
}

type permanentError struct{ error }

func (e permanentError) Unwrap() error { return e.error }

func isPermanent(err error) bool {
	var permanent permanentError
	return errors.As(err, &permanent) || errors.Is(err, seller.ErrPayUniPermanent)
}

// ReviewResult 處理一則 seller.review.result：轉成模組的通知並排入 job，回傳時 job 已持久化（之後 RabbitMQ 才 ack）。
func (c Consumer) ReviewResult(ctx context.Context, body []byte) error {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return permanentError{fmt.Errorf("sellerconsumer：seller.review.result payload：%w", err)}
	}
	merID := env.MerID
	if merID == "" {
		merID = env.Payload["MerID"]
	}
	note := env.Payload["MerStatusNote"]
	if note == "" {
		note = env.Payload["Reason"]
	}
	err := c.Seller.ReceivePayUniNotification(ctx, seller.PayUniNotification{
		MerID: merID, EventKey: env.EventKey, MerStatus: env.Payload["MerStatus"], MerStatusNote: note, ExecuteTime: env.Payload["ExecuteTime"],
	})
	if apperr.KindOf(err) == apperr.KindInvalidInput {
		return permanentError{err}
	}
	return err
}

// Run 訂閱 PayUni 的商家審核通知。
func (c Consumer) Run(ctx context.Context, url string, log *slog.Logger) {
	rabbitmq.Run(ctx, url, []rabbitmq.Binding{
		{Queue: "hawkr.monolith.seller.review.result", Exchange: "seller.events", Key: "seller.review.result", Handle: c.ReviewResult},
	}, isPermanent, log)
}
