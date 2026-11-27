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
	drainTimeout = 5 * time.Second
	ackMargin = time.Second
	closeTimeout = 5 * time.Second
)

type Binding struct {
	Queue, Exchange, Key string
	Handle               func(context.Context, []byte) error
}

// Run 訂閱 durable queue，連線中斷後重新宣告；永久錯誤送 dead-letter queue。
func Run(ctx context.Context, url string, bindings []Binding, isPermanent func(error) bool, log *slog.Logger) {
	for ctx.Err() == nil {
		if err := runOnce(ctx, url, bindings, isPermanent, log, drainTimeout); err != nil && ctx.Err() == nil {
			log.ErrorContext(ctx, "rabbitmq：RabbitMQ 連線或消費失敗", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func runOnce(ctx context.Context, url string, bindings []Binding, isPermanent func(error) bool, log *slog.Logger, drain time.Duration) error {
	conn, err := amqp.DialConfig(url, amqp.Config{Dial: amqp.DefaultDial(5 * time.Second)})
	if err != nil {
		return fmt.Errorf("連線：%w", err)
	}
	handlerCtx, cancelHandlers := context.WithCancel(context.WithoutCancel(ctx))
	stop, cancel := context.WithCancel(ctx)
	var consumers sync.WaitGroup
	defer func() {
		cancel()
		drainTimer := time.AfterFunc(drain, cancelHandlers)
		waitTimeout(&consumers, drain+ackMargin)
		drainTimer.Stop()
		cancelHandlers()
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
					if !Process(handlerCtx, d, b, isPermanent, log) {
						select {
						case <-stop.Done():
						case <-time.After(time.Second):
						}
					}
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

func waitTimeout(wg *sync.WaitGroup, d time.Duration) {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// Process 以 ctx 執行 handler 並 ack 或 nack，回傳是否成功（永久錯誤送 DLQ 也算處理完）；暫時失敗時 nack 放回 queue，
// 由呼叫端決定要不要等一下再接下一則。
func Process(ctx context.Context, delivery amqp.Delivery, binding Binding, isPermanent func(error) bool, log *slog.Logger) bool {
	if err := binding.Handle(ctx, delivery.Body); err != nil {
		log.ErrorContext(ctx, "rabbitmq：處理事件失敗", "queue", binding.Queue, "error", err)
		if isPermanent(err) {
			if nackErr := delivery.Nack(false, false); nackErr != nil {
				log.ErrorContext(ctx, "rabbitmq：送無效事件到 DLQ 失敗", "error", nackErr)
			}
			return true
		}
		if err := delivery.Nack(false, true); err != nil {
			log.ErrorContext(ctx, "rabbitmq：nack 失敗", "error", err)
		}
		return false
	}
	if err := delivery.Ack(false); err != nil {
		log.ErrorContext(ctx, "rabbitmq：ack 失敗", "error", err)
	}
	return true
}
