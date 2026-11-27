package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
	"uuid"

	httpsocial "github.com/Will413028/hawkr/backend/internal/httpapi/social"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type socialProducts struct{ catalog *catalog.Module }

func (a socialProducts) VerifyProduct(ctx context.Context, productID uuid.UUID, storeID int64) error {
	cards, err := a.catalog.ProductCards(ctx, time.Now(), []uuid.UUID{productID}, false)
	if err != nil {
		return err
	}
	card, ok := cards[productID]
	switch {
	case !ok:
		return social.ErrProductNotAvailable
	case card.StoreID != storeID:
		return social.ErrProductNotOwned
	}
	return nil
}

type socialCards struct{ catalog *catalog.Module }

func (a socialCards) Cards(ctx context.Context, ids []uuid.UUID, includeInactive bool) (map[uuid.UUID]httpsocial.ProductInfo, error) {
	items, err := a.catalog.ProductCards(ctx, time.Now(), ids, includeInactive)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]httpsocial.ProductInfo, len(items))
	for id, item := range items {
		out[id] = httpsocial.ProductInfo{
			ID: item.ID, StoreID: item.StoreID, Name: item.Title, ImageKey: item.CoverKey,
			MinPrice: item.MinPrice, MaxPrice: item.MaxPrice,
			Available: item.Status == catalog.StatusActive && item.TotalStock > 0,
		}
	}
	return out, nil
}

type noStoreBlocks struct{}

func (noStoreBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	return false, errors.New("此組裝不做買家身分的寫入，沒有店家封鎖的來源")
}

func scheduleAuthorizers(delegationAuthorizer social.ScheduleAuthorizer) map[social.ScheduleKind]social.ScheduleAuthorizer {
	return map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: delegationAuthorizer}
}

type socialSellers interface {
	social.Stores
	social.StoreBlocks
}

func newSocial(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers *buyer.Module, sellers socialSellers, med mediaPort, products socialProducts, delegationAuthorizer social.ScheduleAuthorizer, log *slog.Logger) *social.Module {
	m := social.New(social.Deps{
		DB: d, Events: bus, Media: med, Jobs: jc, Buyers: buyers, Stores: sellers, StoreBlocks: sellers, Products: products,
		ScheduleAuthorizers: scheduleAuthorizers(delegationAuthorizer),
		NewID:               uuid.NewV7, Now: time.Now, Logger: log,
	})
	subscribeSocial(bus, m)
	return m
}

func subscribeSocial(bus *events.Bus, m *social.Module) {
	events.Subscribe(bus, m.OnBuyerDeletionRequested)
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	events.Subscribe(bus, m.OnMeasureCreated)
	events.Subscribe(bus, m.OnMeasureRevoked)
}

func runSocialRecount(ctx context.Context, out io.Writer) error {
	e := config.NewEnv()
	dbc := dbConfig(e)
	if err := e.Err(); err != nil {
		return err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return err
	}
	defer d.Close()
	log := newLogger()
	delegationAuthorizer := delegation.NewScheduleAuthorizer(delegation.Deps{
		DB: d, Identity: identity.New(identity.Deps{DB: d, Now: time.Now, Logger: log}), Seller: seller.NewReader(d, time.Now), Now: time.Now, Logger: log,
	})
	m := social.New(social.Deps{DB: d, StoreBlocks: noStoreBlocks{}, ScheduleAuthorizers: scheduleAuthorizers(delegationAuthorizer), NewID: uuid.NewV7, Now: time.Now, Logger: log})
	rep, err := m.RecountAll(ctx)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "重算：貼文 %d、留言 %d、討論區留言 %d；漂移 %d\n", rep.Posts, rep.Comments, rep.ForumEntries, rep.Drifted)
	for _, drift := range rep.Drifts {
		_, _ = fmt.Fprintf(out, "  %s %s：%v → %v\n", drift.Kind, drift.ID, drift.Before, drift.After)
	}
	return nil
}
