package mq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var mqConsumeTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mq_consume_total",
		Help: "Count of MQ consumer handler outcomes by queue and result (ack/retry/dlq/requeue).",
	},
	[]string{"queue", "result"},
)

var mqDeadLetterTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mq_dead_letter_total",
		Help: "Count of MQ messages dead-lettered by queue and failure class (permanent/retry_exhausted).",
	},
	[]string{"queue", "class"},
)

const retryHeaderKey = "x-mq-retry-count"

const deadLetterReasonHeaderKey = "x-mq-dead-letter-reason"

const failureClassHeaderKey = "x-mq-failure-class"

const originalRoutingKeyHeaderKey = "x-mq-original-routing-key"

type HandlerFunc func(ctx context.Context, routingKey string, body []byte) error

type Subscription struct {
	Exchange    string
	Queue       string
	RoutingKeys []string
	Handler     HandlerFunc
	MaxRetries  int
}

func DLXExchangeOf(queue string) string {
	return queue + ".dlx"
}

func DLQQueueOf(queue string) string {
	return queue + ".dlq"
}

func RetryExchangeOf(queue string) string {
	return queue + ".retry"
}

func queueTypeArgs() amqp.Table {
	qt := os.Getenv("MQ_QUEUE_TYPE")
	if qt == "" {
		return nil
	}
	return amqp.Table{"x-queue-type": qt}
}

func passiveDeclare() bool {
	return os.Getenv("MQ_PASSIVE_DECLARE") == "true"
}

func closeError(what string, e *amqp.Error) error {
	if e == nil {
		return fmt.Errorf("%s closed", what)
	}
	return fmt.Errorf("%s: %w", what, *e)
}

func isNotFound(err error) bool {
	var amqpErr *amqp.Error
	return errors.As(err, &amqpErr) && amqpErr.Code == amqp.NotFound
}

func withDLXArg(args amqp.Table, dlx string) amqp.Table {
	if args == nil {
		args = amqp.Table{}
	}
	args["x-dead-letter-exchange"] = dlx
	return args
}

type ResilientConsumer struct {
	url      string
	subs     []Subscription
	prefetch int
	metrics  consumeMetrics
}

type consumeMetrics interface {
	Inc(context.Context, string, string)
}

type prometheusConsumeMetrics struct{}

func (prometheusConsumeMetrics) Inc(_ context.Context, queue, result string) {
	mqConsumeTotal.WithLabelValues(queue, result).Inc()
}

type otelConsumeMetrics struct {
	counter metric.Int64Counter
}

func (m otelConsumeMetrics) Inc(ctx context.Context, queue, result string) {
	if m.counter == nil {
		return
	}
	m.counter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("queue", queue),
		attribute.String("result", result),
	))
}

type noopConsumeMetrics struct{}

func (noopConsumeMetrics) Inc(context.Context, string, string) {}

type ConsumerOption func(*ResilientConsumer)

func WithOTelMetrics(provider metric.MeterProvider) ConsumerOption {
	return func(consumer *ResilientConsumer) {
		if provider == nil {
			provider = otel.GetMeterProvider()
		}
		counter, err := provider.Meter("github.com/Will413028/hawkr/go-pkg/mq").Int64Counter(
			"mq_consume_total",
			metric.WithDescription("Count of MQ consumer handler outcomes by queue and result (ack/retry/dlq/requeue)."),
		)
		if err != nil {
			log.Printf("[MQ] failed to create OTel consumer metric: %v; metrics disabled", err)
			consumer.metrics = noopConsumeMetrics{}
			return
		}
		consumer.metrics = otelConsumeMetrics{counter: counter}
	}
}

func NewResilientConsumer(url string, subs []Subscription, options ...ConsumerOption) *ResilientConsumer {
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	consumer := &ResilientConsumer{
		url:      url,
		subs:     subs,
		prefetch: 10,
		metrics:  prometheusConsumeMetrics{},
	}
	for _, option := range options {
		if option != nil {
			option(consumer)
		}
	}
	return consumer
}

func (rc *ResilientConsumer) SetPrefetch(n int) {
	rc.prefetch = n
}

func (rc *ResilientConsumer) Run(ctx context.Context) {
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return
		}

		start := time.Now()
		err := rc.connectAndConsume(ctx)
		if ctx.Err() != nil {
			return
		}

		if time.Since(start) > time.Minute {
			attempt = 0
		}

		delay := reconnectBackoff(attempt)
		log.Printf("[MQ] Disconnected (%v), reconnecting in %v (attempt %d)", err, delay, attempt)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (rc *ResilientConsumer) connectAndConsume(ctx context.Context) error {
	conn, err := amqp.Dial(rc.url)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.Qos(rc.prefetch, 0, false); err != nil {
		return fmt.Errorf("qos: %w", err)
	}

	declare := func(sub Subscription) error {
		dch, err := conn.Channel()
		if err != nil {
			return fmt.Errorf("declare channel: %w", err)
		}
		defer func() { _ = dch.Close() }()

		if passiveDeclare() {

			if _, err := dch.QueueDeclarePassive(sub.Queue, true, false, false, false, nil); err != nil {
				return fmt.Errorf("passive check queue %q (provisioned by infra/mq?): %w", sub.Queue, err)
			}
			if sub.MaxRetries > 0 {

				if err := dch.ExchangeDeclarePassive(RetryExchangeOf(sub.Queue), "fanout", true, false, false, false, nil); err != nil {
					return fmt.Errorf("passive check retry exchange for %q: %w", sub.Queue, err)
				}
			}
			return nil
		}

		if err := dch.ExchangeDeclare(sub.Exchange, "topic", true, false, false, false, nil); err != nil {
			return fmt.Errorf("exchange %s: %w", sub.Exchange, err)
		}

		queueArgs := queueTypeArgs()
		if sub.MaxRetries > 0 {
			dlqQueue := DLQQueueOf(sub.Queue)
			dlxExchange := DLXExchangeOf(sub.Queue)
			if err := dch.ExchangeDeclare(dlxExchange, "fanout", true, false, false, false, nil); err != nil {
				return fmt.Errorf("dlx exchange %s: %w", dlxExchange, err)
			}
			if _, err := dch.QueueDeclare(dlqQueue, true, false, false, false, queueTypeArgs()); err != nil {
				return fmt.Errorf("dlq queue %s: %w", dlqQueue, err)
			}

			if err := dch.QueueBind(dlqQueue, "", dlxExchange, false, nil); err != nil {
				return fmt.Errorf("dlq bind %s→%s: %w", dlqQueue, dlxExchange, err)
			}
			queueArgs = withDLXArg(queueArgs, dlxExchange)
		}

		if _, err := dch.QueueDeclare(sub.Queue, true, false, false, false, queueArgs); err != nil {
			return fmt.Errorf("queue %s: %w", sub.Queue, err)
		}
		for _, rk := range sub.RoutingKeys {
			if err := dch.QueueBind(sub.Queue, rk, sub.Exchange, false, nil); err != nil {
				return fmt.Errorf("bind %s→%s: %w", sub.Queue, rk, err)
			}
		}

		if sub.MaxRetries > 0 {
			retryExchange := RetryExchangeOf(sub.Queue)
			if err := dch.ExchangeDeclare(retryExchange, "fanout", true, false, false, false, nil); err != nil {
				return fmt.Errorf("retry exchange %s: %w", retryExchange, err)
			}
			if err := dch.QueueBind(sub.Queue, "", retryExchange, false, nil); err != nil {
				return fmt.Errorf("retry bind %s←%s: %w", sub.Queue, retryExchange, err)
			}
		}
		return nil
	}

	consumable, err := partitionConsumable(rc.subs, declare)
	if err != nil {
		return err
	}
	if len(consumable) == 0 {
		return fmt.Errorf("no consumable subscriptions (all %d queue declares failed precondition)", len(rc.subs))
	}

	deliveryClosed := make(chan string, len(consumable))
	for _, sub := range consumable {
		msgs, err := ch.Consume(sub.Queue, "", false, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("consume %s: %w", sub.Queue, err)
		}

		log.Printf("[MQ] Consuming from queue: %s (maxRetries=%d)", sub.Queue, sub.MaxRetries)

		go func(msgs <-chan amqp.Delivery, sub Subscription, publishCh *amqp.Channel) {
			for msg := range msgs {
				if err := sub.Handler(ctx, msg.RoutingKey, msg.Body); err != nil {
					log.Printf("[MQ] Handler error on %s: %v", sub.Queue, err)
					handleHandlerErrorWithMetrics(ctx, rc.metrics, publishCh, sub, msg, err)
					continue
				}
				rc.metrics.Inc(ctx, sub.Queue, "ack")
				_ = msg.Ack(false)
			}
			deliveryClosed <- sub.Queue
		}(msgs, sub, ch)
	}

	connErr := conn.NotifyClose(make(chan *amqp.Error, 1))
	chErr := ch.NotifyClose(make(chan *amqp.Error, 1))

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-connErr:
		return closeError("connection", err)
	case err := <-chErr:
		return closeError("channel", err)
	case queue := <-deliveryClosed:
		return fmt.Errorf("delivery channel closed for queue %s", queue)
	}
}

type messagePublisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

func handleHandlerError(pub messagePublisher, sub Subscription, msg amqp.Delivery, err error) {
	handleHandlerErrorWithMetrics(context.Background(), prometheusConsumeMetrics{}, pub, sub, msg, err)
}

func handleHandlerErrorWithMetrics(ctx context.Context, metrics consumeMetrics, pub messagePublisher, sub Subscription, msg amqp.Delivery, err error) {
	reason, permanent := permanentReasonOf(err)

	if sub.MaxRetries <= 0 {
		if permanent {
			metrics.Inc(ctx, sub.Queue, "dlq")
			mqDeadLetterTotal.WithLabelValues(sub.Queue, failureClassPermanent).Inc()
			log.Printf("[MQ] Permanent failure on %s (reason=%s); MaxRetries=0 so there is no framework DLQ — Nack to the broker DLX instead of requeueing: %v",
				sub.Queue, reason, err)
			_ = msg.Nack(false, false)
			return
		}
		metrics.Inc(ctx, sub.Queue, "requeue")
		_ = msg.Nack(false, true)
		return
	}

	attempt := retryCountOf(msg.Headers)
	if permanent || attempt >= sub.MaxRetries {
		class, dlReason := failureClassRetryExhausted, ""
		if permanent {
			class, dlReason = failureClassPermanent, reason
		}
		metrics.Inc(ctx, sub.Queue, "dlq")
		mqDeadLetterTotal.WithLabelValues(sub.Queue, class).Inc()

		if deadLetterToOwnDLQ(pub, sub, msg, class, dlReason) {
			_ = msg.Ack(false)
			return
		}

		log.Printf("[MQ] Dead-letter on %s (class=%s) failed to publish directly; falling back to broker DLX via Nack",
			sub.Queue, class)
		_ = msg.Nack(false, false)
		return
	}

	newHeaders := amqp.Table{}
	for k, v := range msg.Headers {
		newHeaders[k] = v
	}
	newHeaders[retryHeaderKey] = int32(attempt + 1)

	if err := pub.Publish(RetryExchangeOf(sub.Queue), msg.RoutingKey, false, false, amqp.Publishing{
		ContentType:  msg.ContentType,
		Headers:      newHeaders,
		Body:         msg.Body,
		DeliveryMode: amqp.Persistent,
	}); err != nil {
		metrics.Inc(ctx, sub.Queue, "retry")
		log.Printf("[MQ] Retry republish failed on %s (attempt %d/%d): %v; falling back to requeue",
			sub.Queue, attempt+1, sub.MaxRetries, err)
		_ = msg.Nack(false, true)
		return
	}
	metrics.Inc(ctx, sub.Queue, "retry")
	_ = msg.Ack(false)
}

func deadLetterToOwnDLQ(pub messagePublisher, sub Subscription, msg amqp.Delivery, class, reason string) bool {
	dlq := DLQQueueOf(sub.Queue)

	headers := amqp.Table{}
	for k, v := range msg.Headers {
		headers[k] = v
	}
	headers[originalRoutingKeyHeaderKey] = msg.RoutingKey
	headers[failureClassHeaderKey] = class
	if reason != "" {
		headers[deadLetterReasonHeaderKey] = reason
	}

	if err := pub.Publish("", dlq, false, false, amqp.Publishing{
		ContentType:  msg.ContentType,
		Headers:      headers,
		Body:         msg.Body,
		DeliveryMode: amqp.Persistent,
	}); err != nil {
		log.Printf("[MQ] Direct DLQ publish to %s failed: %v", dlq, err)
		return false
	}
	log.Printf("[MQ] Dead-lettered to %s (class=%s reason=%s)", dlq, class, reason)
	return true
}

func retryCountOf(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	raw, ok := headers[retryHeaderKey]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	}
	return 0
}

type declareFunc func(sub Subscription) error

func partitionConsumable(subs []Subscription, declare declareFunc) ([]Subscription, error) {
	consumable := make([]Subscription, 0, len(subs))
	for _, sub := range subs {
		if err := declare(sub); err != nil {
			if isPreconditionFailed(err) || isNotFound(err) {
				log.Printf("[MQ][ERROR] skipping queue %s (NOT consumed): %v. "+
					"The queue is missing on the broker, or its args differ from this service's "+
					"declaration (expected x-dead-letter-exchange=%s). Topology is owned by Terraform: "+
					"check that %s is declared in infra/mq/topology/topology.yaml and fix it via PR "+
					"(CI applies; destructive changes need a manual run with allow_destructive=true). "+
					"Do NOT delete the queue by hand on dev/prod - nothing will re-create it. "+
					"SOP: docs/基礎建設/MQ Topology 管理（Terraform）.md",
					sub.Queue, err, DLXExchangeOf(sub.Queue), sub.Queue)
				continue
			}
			return nil, fmt.Errorf("declare %s: %w", sub.Queue, err)
		}
		consumable = append(consumable, sub)
	}
	return consumable, nil
}

func isPreconditionFailed(err error) bool {
	var amqpErr *amqp.Error
	return errors.As(err, &amqpErr) && amqpErr.Code == amqp.PreconditionFailed
}

func reconnectBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	if attempt > 5 {
		return 30 * time.Second
	}

	return time.Second << uint(attempt-1)
}
