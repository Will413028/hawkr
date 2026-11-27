package main

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/reviewrelay"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func newReview(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers *buyer.Module, sellers *seller.Module, cat *catalog.Module, med mediaPort, orders reviewOrderClient, log *slog.Logger) *review.Module {
	m := review.New(review.Deps{
		DB: d, Events: bus, Media: med, Jobs: jc, Buyers: buyers, Stores: sellers, Orders: reviewOrders{orders}, Catalog: cat,
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	events.Subscribe(bus, m.OnBuyerDeletionRequested)
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnMeasureCreated)
	events.Subscribe(bus, m.OnMeasureRevoked)
	subscribeReviewRelay(bus, jc)
	return m
}

func subscribeReviewRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, reviewrelay.OnReviewCreated(queue))
}

type reviewOrders struct{ client reviewOrderClient }

type reviewOrderClient interface {
	OrderForReview(ctx context.Context, orderNo string) (orderservice.ReviewOrder, error)
}

func (a reviewOrders) OrderForReview(ctx context.Context, orderNo string) (review.OrderForReview, error) {
	o, err := a.client.OrderForReview(ctx, orderNo)
	if errors.Is(err, orderservice.ErrOrderNotFound) {
		return review.OrderForReview{}, review.ErrOrderNotFound
	}
	if err != nil {
		return review.OrderForReview{}, err
	}
	return mapReviewOrder(o), nil
}

func mapReviewOrder(o orderservice.ReviewOrder) review.OrderForReview {
	out := review.OrderForReview{OrderNo: o.OrderNo, BuyerID: o.BuyerID, StoreID: o.StoreID, Status: o.Status, CompletedAt: o.CompletedAt, Items: make([]review.OrderItem, len(o.Items))}
	for i, it := range o.Items {
		out.Items[i] = review.OrderItem{OrderItemID: it.OrderItemID, ProductID: it.ProductID, SkuName: it.SkuName, ImageKey: it.ImageKey, Price: it.Price}
	}
	return out
}
