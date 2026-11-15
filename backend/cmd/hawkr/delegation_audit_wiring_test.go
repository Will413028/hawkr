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

// newDelegation 訂閱 social 每一種貼文寫入事件（代行寫入的稽核靠它們，A7-D27）。「必須有訂閱者」的保證在 events.PublishRequired，不在這裡；
// 這個測試只確認正式組裝真的接了這些事件。
func TestNewDelegationSubscribesEveryPostWriteEvent(t *testing.T) {
	bus := events.NewBus()
	newDelegation(bus, delegation.Deps{Seller: noSeller{}, Social: noSocial{}, NewID: uuid.NewV7})
	for name, ok := range map[string]bool{
		"PostCreated":           events.HasSubscriber[social.PostCreated](bus),
		"PostUpdated":           events.HasSubscriber[social.PostUpdated](bus),
		"PostPublished":         events.HasSubscriber[social.PostPublished](bus),
		"PostUnpublished":       events.HasSubscriber[social.PostUnpublished](bus),
		"PostScheduled":         events.HasSubscriber[social.PostScheduled](bus),
		"PostPublishRepeated":   events.HasSubscriber[social.PostPublishRepeated](bus),
		"PostUnpublishRepeated": events.HasSubscriber[social.PostUnpublishRepeated](bus),
	} {
		if !ok {
			t.Errorf("%s 沒有訂閱者", name)
		}
	}
}
