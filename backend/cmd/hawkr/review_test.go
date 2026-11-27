package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
)

// 型別層級的保證：reviewOrders 滿足 review.Orders，所以 review 的組裝點只能接這個轉接器（或日後的 in-process 實作）。
var _ review.Orders = reviewOrders{}

type reviewOrderRPC struct {
	order orderservice.ReviewOrder
	err   error
}

func (f reviewOrderRPC) OrderForReview(context.Context, string) (orderservice.ReviewOrder, error) {
	return f.order, f.err
}

func TestReviewOrdersMapsNotFoundAndKeepsOtherErrors(t *testing.T) {
	ok := orderservice.ReviewOrder{
		OrderNo: "N1", BuyerID: 11, StoreID: 22, Status: "50", CompletedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Items: []orderservice.ReviewOrderItem{{OrderItemID: 5, ProductID: uuid.UUID{15: 7}, SkuName: "紅茶", Price: 120, ImageKey: "k/1.jpg"}},
	}
	got, err := reviewOrders{reviewOrderRPC{order: ok}}.OrderForReview(context.Background(), "N1")
	if err != nil || got.StoreID != 22 || len(got.Items) != 1 || got.Items[0] != (review.OrderItem{OrderItemID: 5, ProductID: uuid.UUID{15: 7}, SkuName: "紅茶", ImageKey: "k/1.jpg", Price: 120}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	_, err = reviewOrders{reviewOrderRPC{err: fmt.Errorf("包住：%w", orderservice.ErrOrderNotFound)}}.OrderForReview(context.Background(), "N1")
	if !errors.Is(err, review.ErrOrderNotFound) {
		t.Fatalf("ErrOrderNotFound 要對應 review.ErrOrderNotFound：%v", err)
	}
	_, err = reviewOrders{reviewOrderRPC{err: errors.New("boom")}}.OrderForReview(context.Background(), "N1")
	if err == nil || errors.Is(err, review.ErrOrderNotFound) {
		t.Fatalf("其他錯誤不是訂單不存在：%v", err)
	}
}
