package main

import (
	"context"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type noSeller struct{}

func (noSeller) Store(context.Context, int64) (seller.Store, error) { return seller.Store{}, nil }

type noSocial struct{ delegation.Social }

// 代行寫入的稽核只靠 delegation 對 PostWrittenByActor 的訂閱：沒訂閱啟動就要失敗；newDelegation 接上之後通過。
// 拿掉 newDelegation 裡的 events.Subscribe(bus, m.OnPostWrittenByActor)，這個測試失敗。
func TestStartupRequiresTheDelegatedAuditSubscriber(t *testing.T) {
	bus := events.NewBus()
	if err := requireDelegatedAudit(bus); err == nil {
		t.Fatal("沒有訂閱者卻通過啟動檢查")
	}
	newDelegation(bus, delegation.Deps{Seller: noSeller{}, Social: noSocial{}, NewID: uuid.NewV7})
	if err := requireDelegatedAudit(bus); err != nil {
		t.Fatalf("newDelegation 之後：%v", err)
	}
	if !events.HasSubscriber[social.PostWrittenByActor](bus) {
		t.Fatal("PostWrittenByActor 沒有訂閱者")
	}
}
