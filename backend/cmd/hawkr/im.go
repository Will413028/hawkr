package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func newIM(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers im.Buyers, sellers im.Stores, med mediaPort, orders im.Orders, log *slog.Logger) *im.Module {
	m := im.New(im.Deps{
		DB: d, Events: bus, Jobs: jc, Media: med, Buyers: buyers, Stores: sellers, Orders: orders,
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	events.Subscribe(bus, m.OnMeasureCreated)
	events.Subscribe(bus, m.OnMeasureRevoked)
	return m
}
