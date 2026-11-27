package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func newDelegation(bus *events.Bus, d delegation.Deps) *delegation.Module {
	d.Events = bus
	m := delegation.New(d)
	events.Subscribe(bus, m.OnAdminRoleRevoked)
	events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostPublished)
	events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostCreated)
	events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostUpdated)
	events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostUnpublished)
	events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostScheduled)
	events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostPublishRepeated)
	events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostUnpublishRepeated)
	events.Subscribe(bus, m.OnStoreSuspended)
	events.Subscribe(bus, m.OnStoreClosed)
	events.Subscribe(bus, m.OnStorePendingDeletion)
	return m
}

func newDelegationOffline(d *db.DB, bus *events.Bus, id *identity.Module, log *slog.Logger) (*delegation.Module, error) {
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		return nil, err
	}
	reader := seller.NewReader(d, time.Now)
	deps := delegation.Deps{DB: d, Identity: id, Seller: reader, NewID: uuid.NewV7, Now: time.Now, Logger: log}
	deps.Social = social.New(social.Deps{
		DB: d, Events: bus, Jobs: jc, Stores: reader, StoreBlocks: noStoreBlocks{},
		ScheduleAuthorizers: scheduleAuthorizers(delegation.NewScheduleAuthorizer(deps)),
		NewID:               uuid.NewV7, Now: time.Now, Logger: log,
	})
	return newDelegation(bus, deps), nil
}
