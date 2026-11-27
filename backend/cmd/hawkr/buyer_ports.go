package main

import (
	"context"
	"time"
	"uuid"

	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

// buyerProducts 以 catalog 實作買家端的商品卡片 port（一次批次讀取）。Available 是「現在可以購買」：商品上架且有庫存，店家是否營業由 handler 另外判斷。
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

// productInfo 把 catalog 的商品卡片轉成買家端的顯示資料：沒有評價時不設評分；可購買是商品上架且有庫存。
func productInfo(card catalog.ProductSummary) httpbuyer.ProductInfo {
	var rating *float32
	if card.ReviewCount > 0 {
		value := float32(card.RatingAverage)
		rating = &value
	}
	return httpbuyer.ProductInfo{
		ID: card.ID, StoreID: card.StoreID, Name: card.Title, ImageKey: card.CoverKey,
		MinPrice: card.MinPrice, MaxPrice: card.MaxPrice, Rating: rating,
		Available: card.Status == catalog.StatusActive && card.TotalStock > 0,
	}
}

// buyerSkus 以 catalog 實作補貨訂閱確認規格現況的 port。有貨是 catalog 的「可買」（D26）：庫存大於零、商品上架、商品與規格沒刪除、店家可曝光；
// 已刪除與還是草稿的規格視為不存在（結帳快照判斷），不可曝光的店家仍可訂閱但算沒貨。
type buyerSkus struct{ catalog *catalog.Module }

func (a buyerSkus) RestockSku(ctx context.Context, skuID uuid.UUID) (httpbuyer.RestockSku, bool, error) {
	snapshots, err := a.catalog.CheckoutSnapshot(ctx, []uuid.UUID{skuID})
	if err != nil {
		return httpbuyer.RestockSku{}, false, err
	}
	snapshot, ok := snapshots[skuID]
	if !ok || snapshot.Deleted || snapshot.Status == catalog.StatusDraft {
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
