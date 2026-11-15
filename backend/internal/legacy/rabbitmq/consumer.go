package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// handlerTimeout 是一則訊息的處理上限。處理中的訊息在關機時照樣跑完並 ack，不受關機訊號取消。
	handlerTimeout = 5 * time.Second
	// closeTimeout 是關閉連線時等 broker 回應的上限；關連線會取消其上的消費者，未 ack 的預取訊息由 broker 放回 queue。
	closeTimeout = 5 * time.Second
)

type Binding struct {
	Queue, Exchange, Key string
	Handle               func(context.Context, []byte) error
}

// Run 訂閱 durable queue，連線中斷後重新宣告；永久錯誤送 dead-letter queue。
func Run(ctx context.Context, url string, bindings []Binding, isPermanent func(error) bool, log *slog.Logger) {
	for ctx.Err() == nil {
		if err := runOnce(ctx, url, bindings, isPermanent, log); err != nil && ctx.Err() == nil {
			log.ErrorContext(ctx, "rabbitmq：RabbitMQ 連線或消費失敗", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func runOnce(ctx context.Context, url string, bindings []Binding, isPermanent func(error) bool, log *slog.Logger) error {
	conn, err := amqp.DialConfig(url, amqp.Config{Dial: amqp.DefaultDial(5 * time.Second)})
	if err != nil {
		return fmt.Errorf("連線：%w", err)
	}
	// 不論因關機、斷線或某個 delivery channel 關閉而結束，都先讓每個消費者停止取新訊息、等處理中的跑完，才關連線：
	// runOnce 返回時沒有 handler 還在執行。
	stop, cancel := context.WithCancel(ctx)
	var consumers sync.WaitGroup
	defer func() {
		cancel()
		waitTimeout(&consumers, handlerTimeout)
		_ = conn.CloseDeadline(time.Now().Add(closeTimeout))
	}()
	errors := make(chan error, len(bindings))
	for _, b := range bindings {
		ch, err := conn.Channel()
		if err != nil {
			return fmt.Errorf("開 channel：%w", err)
		}
		if err := ch.Qos(1, 0, false); err != nil {
			return err
		}
		if err := ch.ExchangeDeclare(b.Exchange, "topic", true, false, false, false, nil); err != nil {
			return err
		}
		if err := ch.ExchangeDeclare("hawkr.monolith.dead", "topic", true, false, false, false, nil); err != nil {
			return err
		}
		deadQueue := b.Queue + ".dead"
		if _, err := ch.QueueDeclare(deadQueue, true, false, false, false, nil); err != nil {
			return err
		}
		if err := ch.QueueBind(deadQueue, b.Queue, "hawkr.monolith.dead", false, nil); err != nil {
			return err
		}
		args := amqp.Table{"x-dead-letter-exchange": "hawkr.monolith.dead", "x-dead-letter-routing-key": b.Queue}
		if _, err := ch.QueueDeclare(b.Queue, true, false, false, false, args); err != nil {
			return err
		}
		if err := ch.QueueBind(b.Queue, b.Key, b.Exchange, false, nil); err != nil {
			return err
		}
		deliveries, err := ch.Consume(b.Queue, b.Queue, false, false, false, false, nil)
		if err != nil {
			return err
		}
		consumers.Go(func() {
			for {
				select {
				case <-stop.Done():
					return
				case d, ok := <-deliveries:
					if !ok {
						if stop.Err() == nil {
							errors <- fmt.Errorf("%s delivery channel closed", b.Queue)
						}
						return
					}
					if stop.Err() != nil {
						return
					}
					Process(stop, d, b, isPermanent, log)
				}
			}
		})
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-errors:
		return err
	}
}

// waitTimeout 等 wg 歸零，最多等 d；handler 不理會自己的逾時時，連線照樣關閉，未 ack 的訊息由 broker 重送。
func waitTimeout(wg *sync.WaitGroup, d time.Duration) {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// Process 處理一則訊息並 ack 或 nack。handler 的 context 不隨 ctx 取消，上限是 handlerTimeout；ctx 只決定暫時失敗後要不要
// 等一秒再接下一則。
func Process(ctx context.Context, delivery amqp.Delivery, binding Binding, isPermanent func(error) bool, log *slog.Logger) {
	hctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), handlerTimeout)
	defer cancel()
	if err := binding.Handle(hctx, delivery.Body); err != nil {
		log.ErrorContext(ctx, "rabbitmq：處理事件失敗", "queue", binding.Queue, "error", err)
		if isPermanent(err) {
			if nackErr := delivery.Nack(false, false); nackErr != nil {
				log.ErrorContext(ctx, "rabbitmq：送無效事件到 DLQ 失敗", "error", nackErr)
			}
			return
		}
		if err := delivery.Nack(false, true); err != nil {
			log.ErrorContext(ctx, "rabbitmq：nack 失敗", "error", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		return
	}
	if err := delivery.Ack(false); err != nil {
		log.ErrorContext(ctx, "rabbitmq：ack 失敗", "error", err)
	}
}
