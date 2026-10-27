package main

import (
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerrelay"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// subscribeBuyerLifecycle 讓建帳同時建立買家。關閉買家不訂閱 AccountDeleted：唯一的路徑是 accountdeletion 在刪帳的
// transaction 內呼叫 CloseForDeletion。
func subscribeBuyerLifecycle(bus *events.Bus, module *buyer.Module) {
	events.Subscribe(bus, module.OnAccountCreated)
}

func subscribeBuyerEvents(bus *events.Bus, queue jobs.Enqueuer) {
	enqueue := buyerrelay.NewEnqueue(queue)
	events.Subscribe(bus, buyerrelay.OnFavoriteAdded(enqueue))
	events.Subscribe(bus, buyerrelay.OnFavoriteRemoved(enqueue))
	events.Subscribe(bus, buyerrelay.OnSellerFollowed(enqueue))
	events.Subscribe(bus, buyerrelay.OnProductViewed(enqueue))
	events.Subscribe(bus, buyerrelay.OnRestockTargeted(enqueue))
	events.Subscribe(bus, buyerrelay.OnBuyerDeleted(enqueue))
}
