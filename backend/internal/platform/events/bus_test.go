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
