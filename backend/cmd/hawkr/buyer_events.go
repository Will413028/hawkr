package main

import (
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerrelay"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func subscribeBuyerLifecycle(bus *events.Bus, module *buyer.Module) {
	events.Subscribe(bus, module.OnAccountCreated)
}

func subscribeBuyerEvents(bus *events.Bus, queue jobs.Enqueuer) {
	enqueue := buyerrelay.NewEnqueue(queue)
	events.Subscribe(bus, buyerrelay.OnBuyerDeleted(enqueue))
}
