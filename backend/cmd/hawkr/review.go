package main

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
)

// reviewOrders 以還在舊服務的 order-service 實作 review 的訂單 port（ordering 搬進單體後換成 in-process）：訂單不存在回 review.ErrOrderNotFound，
// 其他錯誤原樣回傳（review 轉成 Unavailable，不當成不存在）。
type reviewOrders struct{ client reviewOrderClient }

// reviewOrderClient 是 reviewOrders 用到的 orderservice.Client 的方法。
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
