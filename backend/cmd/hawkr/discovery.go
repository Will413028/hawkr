package main

import (
	"context"
	"log/slog"
	"time"

	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

func newDiscovery(d *db.DB, orders *orderservice.Client, products *catalog.Module, reviews *review.Module, sellers *seller.Module, log *slog.Logger) *discovery.Module {
	return discovery.New(discovery.Deps{
		DB: d, Orders: discoveryOrders{orders}, Products: products, Reviews: discoveryReviews{reviews}, Stores: sellers,
		Now: time.Now, Logger: log,
	})
}

type discoveryOrders struct{ client *orderservice.Client }

func (o discoveryOrders) StoreOrderStats(ctx context.Context, days int) ([]discovery.OrderStats, error) {
	stats, err := o.client.StoreOrderStats(ctx, days)
	if err != nil {
		return nil, err
	}
	out := make([]discovery.OrderStats, len(stats))
	for i, s := range stats {
		out[i] = discovery.OrderStats(s)
	}
	return out, nil
}

type storeRatings interface {
	StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]review.StoreRating, error)
}

func ratingStats[T ~struct {
	Count   int64
	Average float64
}](ctx context.Context, client storeRatings, storeIDs []int64) (map[int64]T, error) {
	if len(storeIDs) == 0 {
		return map[int64]T{}, nil
	}
	stats, err := client.StoreRatingStats(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]T, len(stats))
	for id, s := range stats {
		out[id] = T(s)
	}
	return out, nil
}

type discoveryReviews struct{ client storeRatings }

func (r discoveryReviews) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]discovery.RatingStats, error) {
	return ratingStats[discovery.RatingStats](ctx, r.client, storeIDs)
}

type sellerRatings struct{ client storeRatings }

func (r sellerRatings) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]httpseller.RatingStats, error) {
	return ratingStats[httpseller.RatingStats](ctx, r.client, storeIDs)
}
