package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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
	defer func() { _ = conn.Close() }()
	channels := make([]*amqp.Channel, 0, len(bindings))
	defer func() {
		for _, ch := range channels {
			_ = ch.Close()
		}
	}()
	errors := make(chan error, len(bindings))
	for _, b := range bindings {
		ch, err := conn.Channel()
		if err != nil {
			return fmt.Errorf("開 channel：%w", err)
		}
		channels = append(channels, ch)
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
		deliveries, err := ch.ConsumeWithContext(ctx, b.Queue, b.Queue, false, false, false, false, nil)
		if err != nil {
			return err
		}
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case d, ok := <-deliveries:
					if !ok {
						errors <- fmt.Errorf("%s delivery channel closed", b.Queue)
						return
					}
					Process(ctx, d, b, isPermanent, log)
				}
			}
		}()
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-errors:
		return err
	}
}

func Process(ctx context.Context, delivery amqp.Delivery, binding Binding, isPermanent func(error) bool, log *slog.Logger) {
	if err := binding.Handle(ctx, delivery.Body); err != nil {
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
