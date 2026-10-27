package main

import (
	"context"

	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
)

type buyerProducts struct{ client *productservice.Client }

func (a buyerProducts) Cards(ctx context.Context, ids []int64, includeInactive bool) (map[int64]httpbuyer.ProductInfo, error) {
	items, err := a.client.Cards(ctx, ids, includeInactive)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]httpbuyer.ProductInfo, len(items))
	for id, item := range items {
		out[id] = httpbuyer.ProductInfo{ID: item.ID, StoreID: item.StoreID, Name: item.Name, ImageKey: item.ImageKey, MinPrice: item.MinPrice, MaxPrice: item.MaxPrice, Rating: item.Rating, Available: item.Available}
	}
	return out, nil
}

type buyerOrders struct{ client *orderservice.Client }

func (a buyerOrders) MerchantSummary(ctx context.Context, buyerID int64) ([]httpbuyer.MerchantSummary, error) {
	items, err := a.client.MerchantSummary(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	out := make([]httpbuyer.MerchantSummary, 0, len(items))
	for _, item := range items {
		out = append(out, httpbuyer.MerchantSummary{StoreID: item.StoreID, Spent: item.Spent, Refunded: item.Refunded})
	}
	return out, nil
}

func (a buyerOrders) SpendingRecords(ctx context.Context, buyerID, storeID int64, page, size int) (httpbuyer.SpendingPage, error) {
	response, err := a.client.SpendingRecords(ctx, buyerID, storeID, page, size)
	if err != nil {
		return httpbuyer.SpendingPage{}, err
	}
	out := httpbuyer.SpendingPage{Total: response.Total, Records: make([]httpbuyer.SpendingRecord, 0, len(response.Records))}
	for _, record := range response.Records {
		out.Records = append(out.Records, httpbuyer.SpendingRecord{OrderNo: record.OrderNo, OrderTime: record.OrderTime, Amount: record.Amount, Refunded: record.Refunded, CouponApplied: record.CouponApplied})
	}
	return out, nil
}
