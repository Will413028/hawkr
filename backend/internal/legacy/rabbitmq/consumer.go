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
	// drainTimeout 是關機後處理中的訊息還能跑的時間：handler 的 context 不隨關機取消，關機滿 drainTimeout 才取消。
	// 平常運作不設上限，handler 的逾時由它自己的呼叫負責。
	drainTimeout = 5 * time.Second
	// ackMargin 是 handler 被取消後留給 ack 或 nack 的時間。
	ackMargin = time.Second
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

// runOnce 消費到 ctx 取消（關機）或連線、channel 出錯為止。不論哪一種，都先讓每個消費者停止取新訊息、等處理中的跑完，
// 才關連線：返回時沒有 handler 還在執行。handler 拿到的 context 只在關機滿 drain 後取消；因出錯而結束時連線已不可用，
// 等滿 drain 就關，未 ack 的訊息由 broker 重送。
func runOnce(ctx context.Context, url string, bindings []Binding, isPermanent func(error) bool, log *slog.Logger, drain time.Duration) error {
	conn, err := amqp.DialConfig(url, amqp.Config{Dial: amqp.DefaultDial(5 * time.Second)})
	if err != nil {
		return fmt.Errorf("連線：%w", err)
	}
	handlerCtx, cancelHandlers := context.WithCancel(context.WithoutCancel(ctx))
	stopDrainTimer := context.AfterFunc(ctx, func() { time.AfterFunc(drain, cancelHandlers) })
	stop, cancel := context.WithCancel(ctx)
	var consumers sync.WaitGroup
	defer func() {
		cancel()
		waitTimeout(&consumers, drain+ackMargin)
		stopDrainTimer()
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

// waitTimeout 等 wg 歸零，最多等 d；handler 不理會自己的逾時時，連線照樣關閉，未 ack 的訊息由 broker 重送。
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
