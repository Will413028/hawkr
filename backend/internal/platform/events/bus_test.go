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

// 「必須有訂閱者」的保證只在 PublishRequired 一處（A7-D27）：沒有 handler 回 ErrNoSubscriber；有 handler 才往下走（這裡沒有 transaction，所以是 ErrNoTransaction）。
func TestPublishRequiredFailsClosedWithoutASubscriber(t *testing.T) {
	b := NewBus()
	if err := PublishRequired(context.Background(), b, thingHappened{ID: 1}); !errors.Is(err, ErrNoSubscriber) {
		t.Fatalf("沒有訂閱者：err = %v，預期 ErrNoSubscriber", err)
	}
	Subscribe(b, func(context.Context, thingHappened) error { return nil })
	if err := PublishRequired(context.Background(), b, thingHappened{ID: 1}); errors.Is(err, ErrNoSubscriber) || !errors.Is(err, ErrNoTransaction) {
		t.Fatalf("有訂閱者：err = %v，預期通過訂閱者檢查、停在 ErrNoTransaction", err)
	}
}
