package events

import (
	"context"
	"errors"
	"testing"
)

type thingHappened struct{ ID int }

func TestPublishRefusesToRunOutsideATransaction(t *testing.T) {
	b := NewBus()
	called := false
	Subscribe(b, func(context.Context, thingHappened) error { called = true; return nil })

	if err := Publish(context.Background(), b, thingHappened{ID: 1}); !errors.Is(err, ErrNoTransaction) {
		t.Fatalf("err = %v，預期 ErrNoTransaction", err)
	}
	if called {
		t.Fatal("沒有 transaction 時不得執行 handler")
	}
}

func TestSubscribeRejectsInterfaceEventTypes(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("以 interface 當事件型別應該 panic")
		}
	}()
	Subscribe(NewBus(), func(context.Context, error) error { return nil })
}

// 「指名的 handler 必須存在」的保證只在 PublishRequired 一處（A7-D27）：只有別的訂閱者不算（ErrNoSubscriber）；指名的 handler 在才往下走（這裡沒有 transaction，所以是 ErrNoTransaction）。
func TestPublishRequiredFailsClosedWithoutTheNamedHandler(t *testing.T) {
	b := NewBus()
	ctx := context.Background()
	if err := PublishRequired(ctx, b, thingHappened{ID: 1}, "audit"); !errors.Is(err, ErrNoSubscriber) {
		t.Fatalf("沒有訂閱者：err = %v，預期 ErrNoSubscriber", err)
	}
	Subscribe(b, func(context.Context, thingHappened) error { return nil })
	SubscribeNamed(b, "other", func(context.Context, thingHappened) error { return nil })
	if err := PublishRequired(ctx, b, thingHappened{ID: 1}, "audit"); !errors.Is(err, ErrNoSubscriber) {
		t.Fatalf("只有別的訂閱者：err = %v，預期 ErrNoSubscriber", err)
	}
	SubscribeNamed(b, "audit", func(context.Context, thingHappened) error { return nil })
	if err := PublishRequired(ctx, b, thingHappened{ID: 1}, "audit"); errors.Is(err, ErrNoSubscriber) || !errors.Is(err, ErrNoTransaction) {
		t.Fatalf("指名的 handler 在：err = %v，預期通過檢查、停在 ErrNoTransaction", err)
	}
}
