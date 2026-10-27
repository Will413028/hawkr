package mq

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestQueueTypeArgs_UnsetReturnsNil(t *testing.T) {
	t.Setenv("MQ_QUEUE_TYPE", "")
	if got := queueTypeArgs(); got != nil {
		t.Errorf("expected nil when MQ_QUEUE_TYPE unset, got %v", got)
	}
}

func TestQueueTypeArgs_SetReturnsXQueueType(t *testing.T) {
	t.Setenv("MQ_QUEUE_TYPE", "quorum")
	got := queueTypeArgs()
	if got["x-queue-type"] != "quorum" {
		t.Errorf("expected x-queue-type=quorum, got %v", got)
	}
}

func TestWithDLXArg_NilBaseAllocates(t *testing.T) {
	got := withDLXArg(nil, "notification.shipping.dlx")
	if got["x-dead-letter-exchange"] != "notification.shipping.dlx" {
		t.Errorf("expected x-dead-letter-exchange set, got %v", got)
	}
}

func TestWithDLXArg_SetsNoRoutingKey(t *testing.T) {
	got := withDLXArg(nil, "notification.order.dlx")
	if _, ok := got["x-dead-letter-routing-key"]; ok {
		t.Errorf("fanout DLX must not carry x-dead-letter-routing-key, got %v", got)
	}
}

func TestDLXExchangeOf_IsPerQueue(t *testing.T) {
	if got := DLXExchangeOf("notification.order"); got != "notification.order.dlx" {
		t.Errorf("expected notification.order.dlx, got %q", got)
	}
}

func TestWithDLXArg_PreservesExistingKeys(t *testing.T) {
	base := amqpTableWithQueueType("quorum")
	got := withDLXArg(base, "notification.shipping.dlx")
	if got["x-queue-type"] != "quorum" {
		t.Errorf("expected existing x-queue-type preserved, got %v", got)
	}
	if got["x-dead-letter-exchange"] != "notification.shipping.dlx" {
		t.Errorf("expected x-dead-letter-exchange set, got %v", got)
	}
}

func amqpTableWithQueueType(qt string) amqp.Table {
	return amqp.Table{"x-queue-type": qt}
}
