package main

import (
	"context"
	"time"
	"uuid"

	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

type buyerProducts struct{ catalog *catalog.Module }

func (a buyerProducts) Cards(ctx context.Context, ids []uuid.UUID, includeInactive bool) (map[uuid.UUID]httpbuyer.ProductInfo, error) {
	cards, err := a.catalog.ProductCards(ctx, time.Now(), ids, includeInactive)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]httpbuyer.ProductInfo, len(cards))
	for id, card := range cards {
		out[id] = productInfo(card)
	}
	return out, nil
}

func productInfo(card catalog.ProductCard) httpbuyer.ProductInfo {
	var rating *float32
	if card.ReviewCount > 0 {
		value := float32(card.RatingAverage)
		rating = &value
	}
	return httpbuyer.ProductInfo{
		ID: card.ID, StoreID: card.StoreID, Name: card.Title, ImageKey: card.CoverKey,
		MinPrice: card.MinPrice, MaxPrice: card.MaxPrice, Rating: rating,
		Available: card.Status == catalog.StatusActive && card.TotalStock > 0,
		Visible:   card.Visible,
	}
}

type buyerSkus struct{ catalog *catalog.Module }

func (a buyerSkus) RestockSku(ctx context.Context, skuID uuid.UUID) (httpbuyer.RestockSku, bool, error) {
	snapshots, err := a.catalog.CheckoutSnapshot(ctx, []uuid.UUID{skuID})
	if err != nil {
		return httpbuyer.RestockSku{}, false, err
	}
	snapshot, ok := snapshots[skuID]
	if !ok || snapshot.Deleted {
		return httpbuyer.RestockSku{}, false, nil
	}
	cards, err := a.catalog.ProductCards(ctx, time.Now(), []uuid.UUID{snapshot.ProductID}, true)
	if err != nil {
		return httpbuyer.RestockSku{}, false, err
	}
	if card, found := cards[snapshot.ProductID]; !found || !card.Visible {
		return httpbuyer.RestockSku{}, false, nil
	}
	purchasable, err := a.catalog.SkuPurchasable(ctx, skuID)
	if err != nil {
		return httpbuyer.RestockSku{}, false, err
	}
	return httpbuyer.RestockSku{ProductID: snapshot.ProductID, InStock: purchasable}, true, nil
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
