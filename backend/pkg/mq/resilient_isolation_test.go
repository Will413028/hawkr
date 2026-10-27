package mq

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func queueNames(subs []Subscription) []string {
	out := make([]string, len(subs))
	for i, s := range subs {
		out[i] = s.Queue
	}
	return out
}

// 一個 queue 的 args mismatch(406 PRECONDITION_FAILED) 只能跳過它自己，
// 不可拖垮其餘 subscription（這正是 31s 重投迴圈的成因：共用 channel 被單一 406 拆掉）。
func TestPartitionConsumable_SkipsPreconditionFailed(t *testing.T) {
	subs := []Subscription{{Queue: "q1"}, {Queue: "bad"}, {Queue: "q3"}}
	declare := func(sub Subscription) error {
		if sub.Queue == "bad" {
			return &amqp.Error{Code: amqp.PreconditionFailed, Reason: "inequivalent arg x-dead-letter-exchange"}
		}
		return nil
	}

	consumable, err := partitionConsumable(subs, declare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := queueNames(consumable)
	if len(got) != 2 || got[0] != "q1" || got[1] != "q3" {
		t.Errorf("expected [q1 q3] consumable (bad skipped), got %v", got)
	}
}

// 非 406 的 declare 錯誤（連線層級）應中止整條連線、觸發重連——不可被當成「跳過單一 queue」吞掉。
func TestPartitionConsumable_AbortsOnNonPrecondition(t *testing.T) {
	subs := []Subscription{{Queue: "q1"}, {Queue: "q2"}}
	declare := func(sub Subscription) error {
		if sub.Queue == "q2" {
			return errors.New("connection reset by peer")
		}
		return nil
	}

	if _, err := partitionConsumable(subs, declare); err == nil {
		t.Fatal("expected non-precondition declare error to abort, got nil")
	}
}

// 全部 queue 都 406 時應回報「無可消費 subscription」錯誤（觸發重連退避），而非靜默空跑。
func TestPartitionConsumable_AllSkipped(t *testing.T) {
	subs := []Subscription{{Queue: "q1"}, {Queue: "q2"}}
	declare := func(sub Subscription) error {
		return &amqp.Error{Code: amqp.PreconditionFailed, Reason: "args mismatch"}
	}

	consumable, err := partitionConsumable(subs, declare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(consumable) != 0 {
		t.Errorf("expected 0 consumable, got %v", queueNames(consumable))
	}
}

func TestIsPreconditionFailed(t *testing.T) {
	if !isPreconditionFailed(&amqp.Error{Code: amqp.PreconditionFailed}) {
		t.Error("raw *amqp.Error PRECONDITION_FAILED should be detected")
	}
	// 包裝過的 error 也要認得（connectAndConsume 會 fmt.Errorf %w 包裝）
	wrapped := errors.Join(errors.New("ctx"), &amqp.Error{Code: amqp.PreconditionFailed})
	if !isPreconditionFailed(wrapped) {
		t.Error("wrapped PRECONDITION_FAILED should be detected")
	}
	if isPreconditionFailed(errors.New("plain")) {
		t.Error("plain error must not be treated as precondition-failed")
	}
	if isPreconditionFailed(&amqp.Error{Code: amqp.NotFound}) {
		t.Error("non-precondition amqp error must not match")
	}
}
