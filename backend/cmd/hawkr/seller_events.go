package main

import (
	"context"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/sellerrelay"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func subscribeSellerViews(bus *events.Bus, module *seller.Module) {
	events.Subscribe(bus, func(ctx context.Context, e buyer.ProductViewed) error {
		return module.IngestProductView(ctx, seller.ProductView{EventID: e.EventID, ProductID: e.ProductID, OccurredAt: e.OccurredAt})
	})
}

func subscribeSellerRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, sellerrelay.OnStoreRetired(queue))
	events.Subscribe(bus, sellerrelay.OnStoreSuspended(queue))
	events.Subscribe(bus, sellerrelay.OnStoreUnsuspended(queue))
	events.Subscribe(bus, sellerrelay.OnStoreProfileChanged(queue))
	events.Subscribe(bus, sellerrelay.OnStoreExposureChanged(queue))
}

type sellerProducts struct{ catalog *catalog.Module }

func (p sellerProducts) StoreIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	stores, err := p.catalog.ProductStoreIDs(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("商品：%w", err)
	}
	return stores, nil
}
