package mq

import (
	"context"
	"errors"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"go.uber.org/zap"
)

// ErrPublisherUnavailable 是「MQ 連不上，publisher 建不起來」的訊號。
//
// 它必須是 error 而不是 no-op：INV.ob 的 ProcessFunc 回 error ⇒ 那一列留在 pending、
// 下一輪重試；回 nil ⇒ pkg/outbox 會 MarkDone，而事件從來沒有發出去。
var ErrPublisherUnavailable = errors.New("invoice: MQ publisher is unavailable")

// EventPublisher 是出向事件的薄 wrapper（§4.2.1：建不起來回 nil 不 panic）。
//
// 它只有一個方法，因為 outbox 列上已經帶著 exchange 與 routing key —— 那兩個值是
// 業務交易當下決定的，派送時不得再判斷一次（判斷兩次就會有兩個真相）。
type EventPublisher struct {
	pub *pkgmq.Publisher
	log *zap.Logger
}

// NewEventPublisher 建 publisher。url 為空或連線失敗時回一個「一律 fail」的實例，
// 而不是 nil：nil 會讓呼叫端在 INV.ob 的 tick 內 panic，而 panic 被 Scheduler 的
// recover 吞掉之後看起來只是「事件一直沒發出去」。
func NewEventPublisher(url string, log *zap.Logger) *EventPublisher {
	l := orNopLogger(log)
	if url == "" {
		l.Error("invoice.mq.publisher_disabled", zap.String("reason", "RABBITMQ_URL is empty"))
		return &EventPublisher{log: l}
	}
	p, err := pkgmq.NewPublisher(url)
	if err != nil {
		l.Error("invoice.mq.publisher_init_failed", zap.Error(err))
		return &EventPublisher{log: l}
	}
	return &EventPublisher{pub: p, log: l}
}

// PublishRaw 發一則已經序列化好的 payload。
//
// 走 PublishRaw 而不是 Publish：outbox 列上的 payload 是業務交易當下 marshal 好的
// bytes，再 unmarshal 一次然後 marshal 回去只會製造「發出去的內容與落庫的內容不同」
// 的可能性——而那正是 golden 契約 gate 抓不到的那一格（它只驗 template 讀的 key ⊆ golden）。
func (p *EventPublisher) PublishRaw(ctx context.Context, exchange, routingKey string, body []byte) error {
	if p.pub == nil {
		return ErrPublisherUnavailable
	}
	return p.pub.PublishRaw(ctx, exchange, routingKey, body)
}

// Close 關閉底層連線（fx OnStop）。
func (p *EventPublisher) Close() error {
	if p.pub == nil {
		return nil
	}
	return p.pub.Close()
}
