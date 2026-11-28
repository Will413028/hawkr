package catalog

import (
	"context"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// Restock 缺了，組裝時就 panic，不留到補貨或清訂閱的 worker 執行時才 nil 解參考。
func TestNewRequiresRestock(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("沒有 Restock：應該 panic")
		}
	}()
	New(Deps{})
}

func TestNewRequiresEvents(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("沒有 Events：應該 panic")
		}
	}()
	New(Deps{Restock: stubNotifier{}})
}

func TestNewAcceptsRestockAndEvents(t *testing.T) {
	New(Deps{Restock: stubNotifier{}, Events: events.NewBus()})
}

type stubNotifier struct{}

func (stubNotifier) NotifyRestocked(context.Context, RestockedSKU) error { return nil }

func (stubNotifier) PurgeSKUSubscriptions(context.Context, []uuid.UUID) error { return nil }
