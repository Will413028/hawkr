// Package rabbitmq 是遷移期的暫時轉接：把單體的事件送到舊服務訂閱的 RabbitMQ（ADR 2026-09-24-go-modular-monolith
// D6：單體內用同 transaction 的事件與 River job；還在 RabbitMQ 上的消費者搬進單體之前，由這裡轉送）。
//
// 事件在發生的 transaction 內排入 River job（transactional outbox），worker 在 transaction 外送出並等 broker 確認：
// at-least-once，重送時 event_id 不變，消費者以它去重。最後一個舊的消費者搬進單體後（遷移計畫階段 C）連同 RabbitMQ
// 刪除。
package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/riverqueue/river"
)

// confirmTimeout 是等 broker 確認的上限。
const confirmTimeout = 10 * time.Second

// Publisher 以 publisher confirm 送訊息；連線在第一次送出時建立，斷線後下一次重連。
type Publisher struct {
	url string
	mu  sync.Mutex
	ch  *amqp.Channel
	cn  *amqp.Connection
}

// New 建立 Publisher。url 例如 amqp://guest:guest@localhost:5672/。
func New(url string) (*Publisher, error) {
	if url == "" {
		return nil, errors.New("rabbitmq：缺少網址")
	}
	return &Publisher{url: url}, nil
}

func (p *Publisher) channel() (*amqp.Channel, error) {
	if p.ch != nil && !p.ch.IsClosed() {
		return p.ch, nil
	}
	if p.cn == nil || p.cn.IsClosed() {
		cn, err := amqp.DialConfig(p.url, amqp.Config{Dial: amqp.DefaultDial(5 * time.Second)})
		if err != nil {
			return nil, fmt.Errorf("rabbitmq：連線：%w", err)
		}
		p.cn = cn
	}
	ch, err := p.cn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq：開 channel：%w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("rabbitmq：開啟 publisher confirm：%w", err)
	}
	p.ch = ch
	return ch, nil
}

// Publish 送出一則訊息並等 broker 確認。
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch, err := p.channel()
	if err != nil {
		return err
	}
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Timestamp: time.Now(), Body: body,
	})
	if err != nil {
		p.ch = nil
		return fmt.Errorf("rabbitmq：送出 %s/%s：%w", exchange, routingKey, err)
	}
	cctx, cancel := context.WithTimeout(ctx, confirmTimeout)
	defer cancel()
	acked, err := confirm.WaitContext(cctx)
	if err != nil {
		p.ch = nil
		return fmt.Errorf("rabbitmq：等 broker 確認：%w", err)
	}
	if !acked {
		return fmt.Errorf("rabbitmq：broker 拒絕 %s/%s", exchange, routingKey)
	}
	return nil
}

// Close 關閉連線。
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cn == nil {
		return nil
	}
	return p.cn.Close()
}

// PublishArgs 是送一則事件的 job：body 在排入時就決定（含 event_id），重試送的是同一則。EventID 必須等於 body 的
// event_id：InsertOpts 以它做 River unique，同一則事件不論排入幾次（例如 relay job 在排入後才失敗而重試）都只有一個
// 送出的 job。
type PublishArgs struct {
	EventID    string          `json:"event_id" river:"unique"`
	Exchange   string          `json:"exchange"`
	RoutingKey string          `json:"routing_key"`
	Body       json.RawMessage `json:"body"`
}

func (PublishArgs) Kind() string { return "legacy.rabbitmq_publish" }

// publishMaxAttempts 是送出的 job 最多執行的次數：River 預設的退避（attempt^4 秒）下，第 20 次大約在 2 天之後；
// broker 在那之前恢復就不會掉訊息。
const publishMaxAttempts = 20

// InsertOpts 是排入 PublishArgs 的選項：以 EventID 做 unique（River 預設的 unique 狀態含 completed，已送出的事件
// 在 completed job 保留期間內再排入也會略過）。
func InsertOpts() *river.InsertOpts {
	return &river.InsertOpts{MaxAttempts: publishMaxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// Sender 送出訊息；Publisher 滿足它，測試以假的取代。
type Sender interface {
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
}

// Worker 執行 PublishArgs。
type Worker struct {
	river.WorkerDefaults[PublishArgs]
	s Sender
}

// NewWorker 建立 Worker。
func NewWorker(s Sender) *Worker { return &Worker{s: s} }

func (w *Worker) Work(ctx context.Context, job *river.Job[PublishArgs]) error {
	return w.s.Publish(ctx, job.Args.Exchange, job.Args.RoutingKey, job.Args.Body)
}
