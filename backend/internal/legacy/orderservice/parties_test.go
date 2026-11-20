package orderservice

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
)

type fakeParties struct {
	orderconnect.OrderServiceClient
	afterSales *orderpb.GetOrderForAfterSalesResponse
	between    *orderpb.HasOrderBetweenResponse
	err        error
}

func (f fakeParties) GetOrderForAfterSales(context.Context, *connect.Request[orderpb.GetOrderForAfterSalesRequest]) (*connect.Response[orderpb.GetOrderForAfterSalesResponse], error) {
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.afterSales), nil
}

func (f fakeParties) HasOrderBetween(context.Context, *connect.Request[orderpb.HasOrderBetweenRequest]) (*connect.Response[orderpb.HasOrderBetweenResponse], error) {
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.between), nil
}

func afterSalesResponse(mut func(*orderpb.OrderAfterSalesView)) *orderpb.GetOrderForAfterSalesResponse {
	v := &orderpb.OrderAfterSalesView{OrderNo: "N1", BuyerId: "11", StoreId: "22", Status: "40", BuyerName: "王小明", BuyerPhone: "0912"}
	if mut != nil {
		mut(v)
	}
	return &orderpb.GetOrderForAfterSalesResponse{Data: v}
}

func TestOrderPartiesReturnsOnlyTheFourFields(t *testing.T) {
	got, err := (&Client{rpc: fakeParties{afterSales: afterSalesResponse(nil)}}).OrderParties(context.Background(), "N1")
	if err != nil {
		t.Fatal(err)
	}
	if want := (OrderParties{OrderNo: "N1", BuyerID: 11, StoreID: 22, Status: "40"}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// 只有回應內 code 404 是「訂單不存在」；傳輸錯誤、其他 code、不合法的回應都不是。
func TestOrderPartiesSeparatesNotFoundFromEveryOtherFailure(t *testing.T) {
	cases := []struct {
		name         string
		fake         fakeParties
		wantNotFound bool
	}{
		{"code 404", fakeParties{afterSales: &orderpb.GetOrderForAfterSalesResponse{Code: 404}}, true},
		{"transport error", fakeParties{err: errors.New("connection refused")}, false},
		{"connect not found status", fakeParties{err: connect.NewError(connect.CodeNotFound, errors.New("no route"))}, false},
		{"other code", fakeParties{afterSales: &orderpb.GetOrderForAfterSalesResponse{Code: 500}}, false},
		{"missing data", fakeParties{afterSales: &orderpb.GetOrderForAfterSalesResponse{}}, false},
		{"bad buyer", fakeParties{afterSales: afterSalesResponse(func(v *orderpb.OrderAfterSalesView) { v.BuyerId = "x" })}, false},
		{"zero store", fakeParties{afterSales: afterSalesResponse(func(v *orderpb.OrderAfterSalesView) { v.StoreId = "0" })}, false},
		{"order number differs", fakeParties{afterSales: afterSalesResponse(func(v *orderpb.OrderAfterSalesView) { v.OrderNo = "N2" })}, false},
		{"blank status", fakeParties{afterSales: afterSalesResponse(func(v *orderpb.OrderAfterSalesView) { v.Status = "" })}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&Client{rpc: tc.fake}).OrderParties(context.Background(), "N1")
			if err == nil {
				t.Fatal("要回錯誤")
			}
			if got := errors.Is(err, ErrOrderNotFound); got != tc.wantNotFound {
				t.Fatalf("errors.Is(ErrOrderNotFound) = %v, want %v（err = %v）", got, tc.wantNotFound, err)
			}
		})
	}
}

func TestHasOrderBetween(t *testing.T) {
	cases := []struct {
		name    string
		fake    fakeParties
		want    bool
		orderNo string
		wantErr bool
	}{
		{"none", fakeParties{between: &orderpb.HasOrderBetweenResponse{}}, false, "", false},
		{"has", fakeParties{between: &orderpb.HasOrderBetweenResponse{HasOrder: true, LatestOrderNo: "N9"}}, true, "N9", false},
		{"has but blank order number", fakeParties{between: &orderpb.HasOrderBetweenResponse{HasOrder: true}}, false, "", true},
		{"non-zero code", fakeParties{between: &orderpb.HasOrderBetweenResponse{Code: 500, HasOrder: true, LatestOrderNo: "N9"}}, false, "", true},
		{"transport error", fakeParties{err: errors.New("boom")}, false, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			has, no, err := (&Client{rpc: tc.fake}).HasOrderBetween(context.Background(), 11, 22)
			if (err != nil) != tc.wantErr || has != tc.want || no != tc.orderNo {
				t.Fatalf("has=%v no=%q err=%v", has, no, err)
			}
		})
	}
	if _, _, err := (&Client{rpc: fakeParties{}}).HasOrderBetween(context.Background(), 0, 22); err == nil {
		t.Fatal("buyer 0 要回錯誤")
	}
}
