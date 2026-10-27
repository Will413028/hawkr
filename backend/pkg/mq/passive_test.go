package mq

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestPassiveDeclareEnvGate(t *testing.T) {
	if passiveDeclare() {
		t.Fatal("unset 時應為 active")
	}
	t.Setenv("MQ_PASSIVE_DECLARE", "true")
	if !passiveDeclare() {
		t.Fatal(`MQ_PASSIVE_DECLARE=true 時應為 passive`)
	}
	t.Setenv("MQ_PASSIVE_DECLARE", "1")
	if passiveDeclare() {
		t.Fatal(`只接受字面 "true"（與 queueTypeArgs 的嚴格比對一致）`)
	}
}

func TestIsNotFound(t *testing.T) {
	if !isNotFound(&amqp.Error{Code: amqp.NotFound}) {
		t.Fatal("404 應命中")
	}
	if isNotFound(&amqp.Error{Code: amqp.PreconditionFailed}) {
		t.Fatal("406 不是 404")
	}
	if isNotFound(errors.New("boom")) {
		t.Fatal("非 amqp error 不命中")
	}
}

// 404（passive 模式下 queue 未佈建）要跟 406 一樣被 partition 成 skip，
// 不是 abort 整條連線進重連風暴。寫法鏡射 resilient_isolation_test.go:55-68 的
// TestPartitionConsumable_AllSkipped（簽名：partitionConsumable(subs, declareFunc)
// 回傳 ([]Subscription, error)——skipped 只 log 不回傳）。
func TestPartitionConsumableSkipsNotFound(t *testing.T) {
	subs := []Subscription{{Queue: "missing.queue", Exchange: "x"}}
	declare := func(sub Subscription) error {
		return &amqp.Error{Code: amqp.NotFound, Reason: "NOT_FOUND - no queue"}
	}
	consumable, err := partitionConsumable(subs, declare)
	if err != nil {
		t.Fatalf("404 應被 skip 而非回傳 error，got %v", err)
	}
	if len(consumable) != 0 {
		t.Fatalf("404 subscription 應被 skip，got consumable=%d", len(consumable))
	}
}
