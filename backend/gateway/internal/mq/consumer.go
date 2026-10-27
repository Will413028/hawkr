// Package mq implements gateway-specific RabbitMQ consumer for the SSE platform。
//
// Gateway 首次成為 MQ consumer：每個 pod 開一個 auto-delete broadcast queue，
// bind 多個 routing key（comment.#、notification.inbox.new、notification.unread_changed
// 等），收到訊息後 dispatch 給 realtime.Registry。
//
// 41 號詳細設計書 §5.0 / §5.6：
//   - 啟動策略：async goroutine，連線失敗指數退避重連，**不阻塞** HTTP 服務啟動
//   - Queue 命名：gateway.sse.<pod-hostname>
//   - durable=false, auto-delete=true, exclusive=false
//   - Ack auto（best-effort，推送失敗 log 不重投）
package mq

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Consumer subscribes to all routing keys registered in the realtime.Registry
// and dispatches messages to it。
//
// Lifecycle：
//   - Run is called from a goroutine — it blocks until ctx is cancelled
//   - On connection failure it backs off (5s/15s/45s capped at 60s) and retries
//   - Failed publishes / dispatch errors are logged at registry level; consumer
//     itself never returns an error from a single delivery
type Consumer struct {
	url      string
	registry *realtime.Registry
	log      *zap.Logger

	// connected reflects whether the consumer is currently attached to RabbitMQ。
	// Used as the metric source for gateway_sse_mq_consumer_connected (§5.0)。
	connected atomic.Bool
}

// NewConsumer constructs a consumer。url is sourced from RABBITMQ_URL env at
// the call site. registry is the channel registry that knows how to dispatch
// incoming routing keys。
func NewConsumer(url string, registry *realtime.Registry, log *zap.Logger) *Consumer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Consumer{
		url:      url,
		registry: registry,
		log:      log,
	}
}

// Run consumes messages until ctx is cancelled. Connection failures are
// retried with exponential backoff capped at 60s (5s, 15s, 45s, 60s, 60s, …)。
//
// 設計書 §5.0：失敗只 log + 退避，**永不** return；HTTP server 已 ready，
// MQ 連線狀態僅影響 SSE event 推送，不阻擋其他 endpoint 服務。
func (c *Consumer) Run(ctx context.Context) {
	backoffs := []time.Duration{
		5 * time.Second,
		15 * time.Second,
		45 * time.Second,
		60 * time.Second,
	}
	attempt := 0

	for {
		if ctx.Err() != nil {
			return
		}

		if err := c.runOnce(ctx); err != nil {
			c.connected.Store(false)
			d := backoffs[min(attempt, len(backoffs)-1)]
			attempt++
			c.log.Warn("SSE.mq.consumer.disconnected",
				zap.Error(err),
				zap.Duration("retryAfter", d),
				zap.Int("attempt", attempt))
			select {
			case <-ctx.Done():
				return
			case <-time.After(d):
				continue
			}
		}

		// runOnce returned cleanly (only happens when ctx cancelled inside)。
		return
	}
}

// connCloseError 把 NotifyClose 送來的 *amqp.Error 轉成回傳給重連迴圈的 error。
// nil（channel 關掉，或送來一個 nil）代表連線收掉但沒有帶原因。
//
// 兩個判斷都在這裡，原本都不在：
//
//  1. nil 判斷。原本只判 channel 是否關閉，NotifyClose 送 nil 時會把一個 nil
//     *amqp.Error 包進 error 介面——之後任何人呼叫它的 Error() 都會 panic
//     （value receiver + nil 指標）。pkg/mq/resilient.go 的同款迴圈本來就有
//     這個判斷，這裡補齊。
//  2. 包**值**不包指標。理由與 pkg/mq/resilient.go 的 closeError 相同（見該處
//     完整說明）：value receiver 的 error 包成指標會讓 errors.Is/As 的慣用寫法
//     失效，go1.27 起的 go vet 也會報。
//
// 抽成函式的理由同上：原位置在 runOnce 的 select 裡，要真的連上 broker 才走得到。
func connCloseError(e *amqp.Error) error {
	if e == nil {
		return fmt.Errorf("amqp connection closed")
	}
	return fmt.Errorf("amqp connection error: %w", *e)
}

// runOnce performs a single connect-consume-disconnect cycle。
// Returns nil only when ctx was cancelled cleanly; otherwise returns the
// underlying error so Run can back off.
func (c *Consumer) runOnce(ctx context.Context) error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.Qos(64, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	// Exchange topology 由 infra/mq/ Terraform 單一 ownership，這裡只做 passive 驗存在。
	// 這幾個 exchange（hawkr.content / notification / seller / seller_workspace）
	// 都已登記在 topology.yaml；主動宣告會在 Terraform 調整 args 時撞 406，且破壞單一
	// ownership。404 代表 infra/mq 未佈建 → 明確失敗，不要靜默略過。
	for _, ex := range c.registry.AllExchanges() {
		if err := ch.ExchangeDeclarePassive(ex, "topic", true, false, false, false, nil); err != nil {
			return fmt.Errorf("passive check exchange %s (provisioned by infra/mq?): %w", ex, err)
		}
	}

	// 每次重連都用新名字 + exclusive：cluster 環境的必要條件，見 broadcastQueueName 註解。
	queueName := broadcastQueueName()
	if _, err := ch.QueueDeclare(
		queueName,
		false, // durable — 純訂閱具現化，不需持久化（SSE 推送本就 best-effort）
		true,  // auto-delete
		true,  // exclusive — 連線一斷 broker 立即刪除，不留孤兒
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("declare queue %s: %w", queueName, err)
	}

	// Bind one queue to all configured (exchange, routing key) pairs。
	// We bind each literal routing key (not "comment.#") because Registry.Dispatch
	// does exact-match lookup. Wildcard binds would surface routing keys we
	// don't know how to handle and add noise to logs。
	for ex, rks := range c.registry.BindingsByExchange() {
		for _, rk := range rks {
			if err := ch.QueueBind(queueName, rk, ex, false, nil); err != nil {
				return fmt.Errorf("bind %s -> %s [%s]: %w", queueName, ex, rk, err)
			}
		}
	}

	msgs, err := ch.Consume(
		queueName,
		"",    // consumer tag
		true,  // auto-ack (best-effort, §5.6)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	c.connected.Store(true)
	c.log.Info("SSE.mq.consumer.connected",
		zap.String("queue", queueName),
		zap.Strings("exchanges", c.registry.AllExchanges()))

	// Notify on connection-level errors so we can break out of the delivery
	// loop and trigger reconnect。
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-closeCh:
			if !ok {
				e = nil
			}
			return connCloseError(e)
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			c.registry.Dispatch(d.RoutingKey, d.Body)
		}
	}
}

// IsConnected returns whether the consumer is currently attached. For metrics。
func (c *Consumer) IsConnected() bool {
	return c.connected.Load()
}

// broadcastQueueName returns "gateway.sse.<pod-hostname>.<per-connection-id>"。
// Falls back to "gateway.sse.local-<pid>.<id>" when hostname is unavailable。
//
// 每次呼叫產生不同名字，而 runOnce 每次重連都會呼叫一次——這是 cluster 環境的必要條件：
// non-durable queue 只存在於「建立它的那個節點」，其他節點只有 metadata。固定名字時，
// pod 重連被 k8s Service 導到別的節點，declare 同名 queue 會撞
// `404 NOT_FOUND - failed to perform operation on queue`，然後每次退避重連都重複失敗，
// 該 pod 的 SSE 永久不通（2026-07-26 cutover 到 3-node cluster 後實際發生，兩個 gateway
// pod 壞了一個）。
//
// 搭配 QueueDeclare 的 exclusive=true：連線一斷 broker 立即刪除該 queue。舊做法
// exclusive=false + auto-delete=true 的清理條件是「最後一個 consumer 離開」——declare
// 本身就失敗時 consumer 從未 attach，條件永遠不成立，於是留下永久孤兒 queue。
//
// 這組參數即 RabbitMQ 官方 Publish/Subscribe tutorial 的臨時訂閱模式；此處保留 pod
// hostname 前綴（tutorial 用 broker 產生的隨機名）純粹為了 management UI 與監控可對應
// 到來源 pod。
func broadcastQueueName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = fmt.Sprintf("local-%d", os.Getpid())
	}
	return "gateway.sse." + host + "." + uuid.NewString()[:8]
}
