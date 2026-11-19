package orderservice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
)

type fakeReviewOrders struct {
	orderconnect.OrderServiceClient
	resp *orderpb.GetOrderForReviewResponse
	err  error
}

func (f fakeReviewOrders) GetOrderForReview(context.Context, *connect.Request[orderpb.GetOrderForReviewRequest]) (*connect.Response[orderpb.GetOrderForReviewResponse], error) {
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func validReviewResponse() *orderpb.GetOrderForReviewResponse {
	return &orderpb.GetOrderForReviewResponse{
		Data: &orderpb.OrderForReviewData{OrderNo: "N1", BuyerId: "11", StoreId: "22", Status: "50", CompletedAt: "2026-11-01T00:00:00Z"},
		Items: []*orderpb.OrderItemForReview{
			{OrderItemId: "5", SkuName: "紅茶", Price: "120", ProductId: "7", ImageObjectKey: "k/1.jpg", ImageUrl: "https://cdn/k/1.jpg"},
			{OrderItemId: "6", SkuName: "綠茶", Price: "0", ProductId: "8"},
		},
	}
}

// RD9：只有回應內 code 404 是「訂單不存在」；傳輸錯誤、其他 code、不合法的回應都不是，呼叫端才能分流成 Unavailable。
func TestOrderForReviewSeparatesNotFoundFromEveryOtherFailure(t *testing.T) {
	notFound := &orderpb.GetOrderForReviewResponse{Code: 404, Message: "資源不存在"}
	other := &orderpb.GetOrderForReviewResponse{Code: 500}
	noData := &orderpb.GetOrderForReviewResponse{}
	badBuyer := validReviewResponse()
	badBuyer.Data.BuyerId = "x"
	badTime := validReviewResponse()
	badTime.Data.CompletedAt = "yesterday"
	noProduct := validReviewResponse()
	noProduct.Items[0].ProductId = ""
	wrongOrder := validReviewResponse()
	wrongOrder.Data.OrderNo = "N2"
	longSku := validReviewResponse()
	longSku.Items[0].SkuName = strings.Repeat("茶", 256)
	cases := []struct {
		name         string
		fake         fakeReviewOrders
		wantNotFound bool
	}{
		{"code 404", fakeReviewOrders{resp: notFound}, true},
		{"transport error", fakeReviewOrders{err: errors.New("connection refused")}, false},
		{"connect not found status", fakeReviewOrders{err: connect.NewError(connect.CodeNotFound, errors.New("no route"))}, false},
		{"other code", fakeReviewOrders{resp: other}, false},
		{"missing data", fakeReviewOrders{resp: noData}, false},
		{"bad buyer", fakeReviewOrders{resp: badBuyer}, false},
		{"bad completed_at", fakeReviewOrders{resp: badTime}, false},
		{"order number differs from request", fakeReviewOrders{resp: wrongOrder}, false},
		{"sku name too long", fakeReviewOrders{resp: longSku}, false},
		{"item without product", fakeReviewOrders{resp: noProduct}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&Client{rpc: tc.fake}).OrderForReview(context.Background(), "N1")
			if err == nil {
				t.Fatal("要回錯誤")
			}
			if got := errors.Is(err, ErrOrderNotFound); got != tc.wantNotFound {
				t.Fatalf("errors.Is(ErrOrderNotFound) = %v, want %v（err = %v）", got, tc.wantNotFound, err)
			}
		})
	}
}

func TestOrderForReviewParsesSnapshot(t *testing.T) {
	got, err := (&Client{rpc: fakeReviewOrders{resp: validReviewResponse()}}).OrderForReview(context.Background(), "N1")
	if err != nil {
		t.Fatal(err)
	}
	if got.BuyerID != 11 || got.StoreID != 22 || got.Status != "50" || !got.CompletedAt.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("order = %+v", got)
	}
	want := []ReviewOrderItem{
		{OrderItemID: 5, ProductID: 7, SkuName: "紅茶", ImageKey: "k/1.jpg", Price: 120},
		{OrderItemID: 6, ProductID: 8, SkuName: "綠茶", ImageKey: "", Price: 0},
	}
	if len(got.Items) != 2 || got.Items[0] != want[0] || got.Items[1] != want[1] {
		t.Fatalf("items = %+v", got.Items)
	}
}

func TestOrderForReviewRejectsOrderNumberBeyondTheTableLimit(t *testing.T) {
	long := strings.Repeat("N", 65)
	resp := validReviewResponse()
	resp.Data.OrderNo = long
	_, err := (&Client{rpc: fakeReviewOrders{resp: resp}}).OrderForReview(context.Background(), long)
	if err == nil || errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("err = %v", err)
	}
}
