package mq

import (
	"errors"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func mqDeadLetterCount(queue, class string) float64 {
	return testutil.ToFloat64(mqDeadLetterTotal.WithLabelValues(queue, class))
}

func TestPermanent_IsAndUnwrapAndReason(t *testing.T) {
	base := errors.New("server error code=404 msg=資源不存在")
	err := Permanent("order_not_found", base)

	if !errors.Is(err, ErrPermanent) {
		t.Error("Permanent() must satisfy errors.Is(err, ErrPermanent)")
	}
	if !errors.Is(err, base) {
		t.Error("Permanent() must keep the cause reachable via errors.Is")
	}
	if errors.Is(base, ErrPermanent) {
		t.Error("a plain error must not be classified permanent")
	}
	wrapped := fmt.Errorf("record refund order_no=O1: %w", err)
	if !errors.Is(wrapped, ErrPermanent) {
		t.Error("classification must survive additional wrapping")
	}
	if got, ok := permanentReasonOf(wrapped); !ok || got != "order_not_found" {
		t.Errorf("reason must survive wrapping, got %q ok=%v", got, ok)
	}
}

func TestHandleHandlerError_PermanentDeadLettersWithoutRetrying(t *testing.T) {
	pub := &fakePublisher{}
	ack := &fakeAck{}
	sub := Subscription{Exchange: "aftersales.events", Queue: "crm.aftersales.refund.completed", MaxRetries: 3}
	msg := deliveryWith(ack, "aftersales.refund.completed", 0)

	handleHandlerError(pub, sub, msg, Permanent("order_not_found", errors.New("code=404")))

	if len(pub.calls) != 1 {
		t.Fatalf("expected exactly 1 publish (direct DLQ), got %d", len(pub.calls))
	}
	c := pub.calls[0]
	if c.exchange != "" || c.key != "crm.aftersales.refund.completed.dlq" {
		t.Errorf("permanent failure must go straight to its own DLQ via the default exchange, got exchange=%q key=%q", c.exchange, c.key)
	}
	if _, retried := c.pub.Headers[retryHeaderKey]; retried && c.pub.Headers[retryHeaderKey] != int32(0) {
		t.Errorf("permanent failure must not consume a retry attempt, headers=%v", c.pub.Headers)
	}
	if !ack.acked || ack.nacked {
		t.Error("original delivery must be Ack'd once the DLQ publish succeeded")
	}
}

func TestHandleHandlerError_PermanentRecordsReasonAndClassOnTheMessage(t *testing.T) {
	pub := &fakePublisher{}
	sub := Subscription{Exchange: "aftersales.events", Queue: "crm.aftersales.refund.completed", MaxRetries: 3}
	msg := deliveryWith(&fakeAck{}, "aftersales.refund.completed", 0)

	handleHandlerError(pub, sub, msg, Permanent("order_not_found", errors.New("code=404")))

	h := pub.calls[0].pub.Headers
	if h[deadLetterReasonHeaderKey] != "order_not_found" {
		t.Errorf("%s must carry the declared reason, got %v", deadLetterReasonHeaderKey, h[deadLetterReasonHeaderKey])
	}
	if h[failureClassHeaderKey] != failureClassPermanent {
		t.Errorf("%s must be %q, got %v", failureClassHeaderKey, failureClassPermanent, h[failureClassHeaderKey])
	}
}

func TestHandleHandlerError_RetryExhaustedIsClassifiedToo(t *testing.T) {
	pub := &fakePublisher{}
	sub := Subscription{Exchange: "aftersales.events", Queue: "crm.aftersales.refund.completed", MaxRetries: 3}
	msg := deliveryWith(&fakeAck{}, "aftersales.refund.completed", 3)

	handleHandlerError(pub, sub, msg, errors.New("downstream timeout"))

	h := pub.calls[0].pub.Headers
	if h[failureClassHeaderKey] != failureClassRetryExhausted {
		t.Errorf("%s must be %q, got %v", failureClassHeaderKey, failureClassRetryExhausted, h[failureClassHeaderKey])
	}
	if _, ok := h[deadLetterReasonHeaderKey]; ok {
		t.Errorf("retry-exhausted has no declared reason; %s must be absent", deadLetterReasonHeaderKey)
	}
}

func TestHandleHandlerError_PermanentKeepsLegacyDlqMetricAndAddsClass(t *testing.T) {
	pub := &fakePublisher{}
	sub := Subscription{Exchange: "aftersales.events", Queue: "crm.metric.probe", MaxRetries: 3}
	msg := deliveryWith(&fakeAck{}, "aftersales.refund.completed", 0)

	beforeLegacy := mqConsumeCount(sub.Queue, "dlq")
	beforeClass := mqDeadLetterCount(sub.Queue, failureClassPermanent)

	handleHandlerError(pub, sub, msg, Permanent("order_not_found", errors.New("code=404")))

	if got := mqConsumeCount(sub.Queue, "dlq") - beforeLegacy; got != 1 {
		t.Errorf(`mq_consume_total{result="dlq"} must still increment by 1, got %v`, got)
	}
	if got := mqDeadLetterCount(sub.Queue, failureClassPermanent) - beforeClass; got != 1 {
		t.Errorf(`mq_dead_letter_total{class="permanent"} must increment by 1, got %v`, got)
	}
}

func TestHandleHandlerError_PermanentOnLegacyQueueDoesNotRequeueForever(t *testing.T) {
	pub := &fakePublisher{}
	ack := &fakeAck{}
	sub := Subscription{Exchange: "aftersales.events", Queue: "legacy.no.retry", MaxRetries: 0}
	msg := deliveryWith(ack, "aftersales.refund.completed", 0)

	handleHandlerError(pub, sub, msg, Permanent("order_not_found", errors.New("code=404")))

	if !ack.nacked {
		t.Fatal("permanent failure on a legacy queue must be Nack'd, not Ack'd")
	}
	if ack.requeue {
		t.Error("permanent failure must never be requeued — it would poison the queue forever")
	}
}

func TestHandleHandlerError_UnclassifiedErrorStillRetries(t *testing.T) {
	pub := &fakePublisher{}
	sub := Subscription{Exchange: "aftersales.events", Queue: "crm.aftersales.refund.completed", MaxRetries: 3}
	msg := deliveryWith(&fakeAck{}, "aftersales.refund.completed", 0)

	handleHandlerError(pub, sub, msg, errors.New("downstream timeout"))

	if len(pub.calls) != 1 || pub.calls[0].exchange != "crm.aftersales.refund.completed.retry" {
		t.Fatalf("transient failure must still republish to the retry exchange, calls=%v", pub.calls)
	}
}
