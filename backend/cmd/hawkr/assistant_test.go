package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
)

type fakeOrderClient struct {
	detail orderservice.BuyerOrderDetail
	err    error
}

func (f fakeOrderClient) MyOrders(context.Context, int64, string, int, int) ([]orderservice.BuyerOrder, error) {
	return []orderservice.BuyerOrder{{OrderNo: "O1", Status: "20", SellerName: "店", Amount: 5, CreateTime: time.Unix(1, 0), ItemCount: 2}}, f.err
}

func (f fakeOrderClient) BuyerOrderDetail(context.Context, int64, string) (orderservice.BuyerOrderDetail, error) {
	return f.detail, f.err
}

func TestAssistantOrdersTranslatesOrderNotFound(t *testing.T) {
	notFound := fmt.Errorf("查詢：%w", orderservice.ErrOrderNotFound)
	_, err := assistantOrders{fakeOrderClient{err: notFound}}.OrderDetail(t.Context(), 7, "O1")
	if !errors.Is(err, assistant.ErrOrderNotFound) {
		t.Fatalf("查無訂單沒有轉成 assistant.ErrOrderNotFound：%v", err)
	}
	if !errors.Is(err, orderservice.ErrOrderNotFound) {
		t.Fatalf("原錯誤要保留（%%w）：%v", err)
	}
}

func TestAssistantOrdersKeepsOtherErrorsUntranslated(t *testing.T) {
	boom := errors.New("連不上")
	_, err := assistantOrders{fakeOrderClient{err: boom}}.OrderDetail(t.Context(), 7, "O1")
	if !errors.Is(err, boom) || errors.Is(err, assistant.ErrOrderNotFound) {
		t.Fatalf("其他錯誤要原樣回傳、不能被當成查無訂單：%v", err)
	}
}

func TestAssistantOrdersMapsFields(t *testing.T) {
	got, err := assistantOrders{fakeOrderClient{detail: orderservice.BuyerOrderDetail{OrderNo: "O1", Status: "30", ShippingFee: 9}}}.OrderDetail(t.Context(), 7, "O1")
	if err != nil || got.OrderNo != "O1" || got.Status != "30" || got.ShippingFee != 9 {
		t.Fatalf("OrderDetail = %+v, %v", got, err)
	}
	rows, err := assistantOrders{fakeOrderClient{}}.MyOrders(t.Context(), 7, "", 1, 10)
	if err != nil || len(rows) != 1 || rows[0].OrderNo != "O1" || rows[0].ItemCount != 2 || rows[0].Amount != 5 {
		t.Fatalf("MyOrders = %+v, %v", rows, err)
	}
}
