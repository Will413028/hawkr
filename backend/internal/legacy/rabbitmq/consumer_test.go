package rabbitmq

import (
	"context"
	"io"
	"log/slog"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

type countingAck struct{ acks, nacks int }

func (c *countingAck) Ack(uint64, bool) error        { c.acks++; return nil }
func (c *countingAck) Nack(uint64, bool, bool) error { c.nacks++; return nil }
func (c *countingAck) Reject(uint64, bool) error     { c.nacks++; return nil }

// 平常運作時 handler 沒有逾時上限，關機訊號也還沒來：handler 的 context 不能帶 deadline、也不能已取消。
func TestProcessHandlerContextHasNoDeadlineBeforeShutdown(t *testing.T) {
	ack := &countingAck{}
	b := Binding{Queue: "q", Handle: func(ctx context.Context, _ []byte) error {
		if _, ok := ctx.Deadline(); ok {
			t.Error("handler 的 context 有 deadline")
		}
		return ctx.Err()
	}}
	Process(t.Context(), amqp.Delivery{Acknowledger: ack}, b, func(error) bool { return false }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if ack.acks != 1 || ack.nacks != 0 {
		t.Fatalf("acks=%d nacks=%d，要 1 與 0", ack.acks, ack.nacks)
	}
}

// 關機訊號已經來了，handler 仍拿到未取消的 context，處理完 ack。
func TestProcessHandlerContextSurvivesShutdownSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	ack := &countingAck{}
	b := Binding{Queue: "q", Handle: func(ctx context.Context, _ []byte) error { return ctx.Err() }}
	Process(ctx, amqp.Delivery{Acknowledger: ack}, b, func(error) bool { return false }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if ack.acks != 1 {
		t.Fatalf("acks=%d，要 1", ack.acks)
	}
}
