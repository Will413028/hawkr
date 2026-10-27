package mq

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// mqConsumeCount reads the current mq_consume_total value for (queue, result),
// so tests can assert the delta handleHandlerError produced without resetting
// the shared package-level counter between table entries.
func mqConsumeCount(queue, result string) float64 {
	return testutil.ToFloat64(mqConsumeTotal.WithLabelValues(queue, result))
}

// fakePublisher records Publish calls and can be primed to fail, so
// handleHandlerError's routing decisions are assertable without a live broker.
type fakePublisher struct {
	calls []publishCall
	err   error
}

type publishCall struct {
	exchange string
	key      string
	pub      amqp.Publishing
}

func (f *fakePublisher) Publish(exchange, key string, _, _ bool, msg amqp.Publishing) error {
	f.calls = append(f.calls, publishCall{exchange: exchange, key: key, pub: msg})
	return f.err
}

// fakeAck implements amqp.Acknowledger so we can observe whether the original
// delivery was Ack'd or Nack'd (and with what requeue flag).
type fakeAck struct {
	acked   bool
	nacked  bool
	requeue bool
}

func (f *fakeAck) Ack(_ uint64, _ bool) error    { f.acked = true; return nil }
func (f *fakeAck) Reject(_ uint64, _ bool) error { f.nacked = true; return nil }
func (f *fakeAck) Nack(_ uint64, _ bool, requeue bool) error {
	f.nacked = true
	f.requeue = requeue
	return nil
}

func deliveryWith(ack *fakeAck, rk string, retryCount int32) amqp.Delivery {
	return amqp.Delivery{
		Acknowledger: ack,
		ContentType:  "application/json",
		RoutingKey:   rk,
		Headers:      amqp.Table{retryHeaderKey: retryCount},
		Body:         []byte(`{"poison":true}`),
	}
}

// (a) handler 失敗且 attempt < MaxRetries → republish 到 {Queue}.retry、rk 維持原 rk、
// x-mq-retry-count +1、原訊息被 Ack（不得 Nack）。這是放大修復的核心：retry 打回
// per-queue fanout 而非共用 topic exchange，避免對同 (exchange, rk) 綁定的其他 queue fan-out。
func TestHandleHandlerError_RepublishToPerQueueRetryExchange(t *testing.T) {
	pub := &fakePublisher{}
	ack := &fakeAck{}
	sub := Subscription{Exchange: "hawkr.shipping", Queue: "notification.buyer.delivered", MaxRetries: 3}
	msg := deliveryWith(ack, "shipment.delivered", 1)
	before := mqConsumeCount(sub.Queue, "retry")

	handleHandlerError(pub, sub, msg, errors.New("handler failed"))

	if len(pub.calls) != 1 {
		t.Fatalf("expected exactly 1 publish, got %d", len(pub.calls))
	}
	c := pub.calls[0]
	if c.exchange != "notification.buyer.delivered.retry" {
		t.Errorf("retry must target per-queue fanout exchange {Queue}.retry, got %q", c.exchange)
	}
	if c.key != "shipment.delivered" {
		t.Errorf("original routing key must be preserved on republish, got %q", c.key)
	}
	if got := c.pub.Headers[retryHeaderKey]; got != int32(2) {
		t.Errorf("expected retry counter incremented to 2, got %v", got)
	}
	if !ack.acked {
		t.Error("original delivery must be Ack'd after successful republish")
	}
	if ack.nacked {
		t.Error("must not Nack when republish succeeded")
	}
	if got := mqConsumeCount(sub.Queue, "retry") - before; got != 1 {
		t.Errorf("expected mq_consume_total{result=retry} to increment by 1, got %v", got)
	}
}

// (b) attempt >= MaxRetries → 顯式 publish 到 default exchange ""、routing key = {Queue}.dlq
// （直投自身 DLQ，不經共用 {Exchange}.dlx）、原 rk 存 header、原訊息被 Ack。
func TestHandleHandlerError_DeadLetterDirectToOwnDLQ(t *testing.T) {
	pub := &fakePublisher{}
	ack := &fakeAck{}
	sub := Subscription{Exchange: "hawkr.shipping", Queue: "notification.buyer.delivered", MaxRetries: 3}
	msg := deliveryWith(ack, "shipment.delivered", 3)
	before := mqConsumeCount(sub.Queue, "dlq")

	handleHandlerError(pub, sub, msg, errors.New("still failing"))

	if len(pub.calls) != 1 {
		t.Fatalf("expected exactly 1 publish, got %d", len(pub.calls))
	}
	c := pub.calls[0]
	if c.exchange != "" {
		t.Errorf("direct DLQ publish must use the default exchange, got %q", c.exchange)
	}
	if c.key != "notification.buyer.delivered.dlq" {
		t.Errorf("direct DLQ publish routing key must equal the DLQ name, got %q", c.key)
	}
	if got := c.pub.Headers[originalRoutingKeyHeaderKey]; got != "shipment.delivered" {
		t.Errorf("original routing key must be preserved in header, got %v", got)
	}
	if !ack.acked {
		t.Error("original delivery must be Ack'd after successful direct DLQ publish")
	}
	if ack.nacked {
		t.Error("must not Nack when direct DLQ publish succeeded")
	}
	if got := mqConsumeCount(sub.Queue, "dlq") - before; got != 1 {
		t.Errorf("expected mq_consume_total{result=dlq} to increment by 1, got %v", got)
	}
}

// (c) 死信直投失敗 → fallback 走 Nack(false, false)（交回 broker 共用 DLX），不得 Ack、
// 不得 requeue（requeue=true 會讓 poison message 回主 queue 無限重投）。
func TestHandleHandlerError_DeadLetterFallbackToBrokerDLXOnPublishFailure(t *testing.T) {
	pub := &fakePublisher{err: errors.New("channel closed")}
	ack := &fakeAck{}
	sub := Subscription{Exchange: "hawkr.shipping", Queue: "notification.buyer.delivered", MaxRetries: 3}
	msg := deliveryWith(ack, "shipment.delivered", 3)
	before := mqConsumeCount(sub.Queue, "dlq")

	handleHandlerError(pub, sub, msg, errors.New("still failing"))

	if ack.acked {
		t.Error("must not Ack when direct DLQ publish failed")
	}
	if !ack.nacked {
		t.Error("expected Nack fallback to broker DLX when direct DLQ publish failed")
	}
	if ack.requeue {
		t.Error("dead-letter fallback must Nack with requeue=false (route via DLX), not requeue")
	}
	if got := mqConsumeCount(sub.Queue, "dlq") - before; got != 1 {
		t.Errorf("expected mq_consume_total{result=dlq} to increment by 1 even on publish-failure fallback, got %v", got)
	}
}

// MaxRetries == 0 保護：維持 legacy 無限 requeue，不得 publish（不啟用 retry/DLX）。
func TestHandleHandlerError_LegacyRequeueWhenNoMaxRetries(t *testing.T) {
	pub := &fakePublisher{}
	ack := &fakeAck{}
	sub := Subscription{Exchange: "x", Queue: "q", MaxRetries: 0}
	msg := deliveryWith(ack, "rk", 0)
	before := mqConsumeCount(sub.Queue, "requeue")

	handleHandlerError(pub, sub, msg, errors.New("fail"))

	if len(pub.calls) != 0 {
		t.Errorf("legacy path (MaxRetries==0) must not publish, got %d calls", len(pub.calls))
	}
	if !ack.nacked || !ack.requeue {
		t.Error("legacy path must Nack with requeue=true")
	}
	if got := mqConsumeCount(sub.Queue, "requeue") - before; got != 1 {
		t.Errorf("expected mq_consume_total{result=requeue} to increment by 1, got %v", got)
	}
}

// retry republish 失敗（未達上限）→ fallback Nack(false, true) requeue，保訊息不丟。
func TestHandleHandlerError_RetryRepublishFailureFallsBackToRequeue(t *testing.T) {
	pub := &fakePublisher{err: errors.New("channel closed")}
	ack := &fakeAck{}
	sub := Subscription{Exchange: "x", Queue: "q", MaxRetries: 3}
	msg := deliveryWith(ack, "rk", 1)
	before := mqConsumeCount(sub.Queue, "retry")

	handleHandlerError(pub, sub, msg, errors.New("fail"))

	if ack.acked {
		t.Error("must not Ack when retry republish failed")
	}
	if !ack.nacked || !ack.requeue {
		t.Error("retry republish failure must Nack with requeue=true")
	}
	if got := mqConsumeCount(sub.Queue, "retry") - before; got != 1 {
		t.Errorf("expected mq_consume_total{result=retry} to increment by 1 even on republish-failure fallback, got %v", got)
	}
}
