package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func newModerationRetention(d *db.DB, buyers *buyer.Module, med mediaPort, log *slog.Logger) *moderation.Retention {
	return moderation.NewRetention(moderation.RetentionDeps{DB: d, Buyer: buyers, Media: med, Now: time.Now, Logger: log})
}

func newModeration(d *db.DB, bus *events.Bus, soc *social.Module, rev *review.Module, chat *im.Module, sel *seller.Module, buyers *buyer.Module, suspension *accountsuspension.Module, med mediaPort, log *slog.Logger) *moderation.Module {
	return moderation.New(moderation.Deps{
		DB: d, Events: bus, Social: soc, Review: rev, IM: chat, Seller: sel, Buyer: buyers, Suspension: suspension,
		Media: med, NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
}

func subscribeModeration(bus *events.Bus, r *moderation.Retention) {
	events.Subscribe(bus, r.OnBuyerDeletionRequested)
	events.Subscribe(bus, r.OnBuyerDeleted)
	events.Subscribe(bus, r.OnStoreRetired)
}
