package main

import (
	"context"

	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
)

type sellerDashboardOrders struct{ client *orderservice.Client }

func (o sellerDashboardOrders) SalesStats(ctx context.Context, storeID int64, windows []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	legacy := make([]orderservice.Window, len(windows))
	for i, w := range windows {
		legacy[i] = orderservice.Window{Start: w.Start, End: w.End}
	}
	stats, err := o.client.StorePeriodOrderStats(ctx, storeID, legacy)
	if err != nil {
		return nil, err
	}
	out := make([]httpseller.SalesStats, len(stats))
	for i, s := range stats {
		out[i] = httpseller.SalesStats{Orders: s.QualifyingOrders, Customers: s.DistinctBuyers, Revenue: s.Revenue}
	}
	return out, nil
}

func (o sellerDashboardOrders) StatusCounts(ctx context.Context, storeID int64) (httpseller.OrderStatusCounts, error) {
	counts, err := o.client.SellerOrderStatusCounts(ctx, storeID)
	if err != nil {
		return httpseller.OrderStatusCounts{}, err
	}
	return httpseller.OrderStatusCounts(counts), nil
}
