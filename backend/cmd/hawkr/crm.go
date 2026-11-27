package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func newCRM(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers *buyer.Module, sellers *seller.Module, log *slog.Logger) *crm.Module {
	m := crm.New(crm.Deps{DB: d, Jobs: jc, Buyers: buyers, Stores: sellers, NewID: uuid.NewV7, Now: time.Now, Logger: log})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	return m
}
