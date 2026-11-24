package orderservice

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
)

type fakePaymentOrders struct {
	orderconnect.OrderServiceClient
	resp *orderpb.GetOrderForPaymentResponse
	err  error
}

func (f fakePaymentOrders) GetOrderForPayment(context.Context, *connect.Request[orderpb.GetOrderForPaymentRequest]) (*connect.Response[orderpb.GetOrderForPaymentResponse], error) {
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func paymentResponse(status string) *orderpb.GetOrderForPaymentResponse {
	return &orderpb.GetOrderForPaymentResponse{Data: &orderpb.OrderPaymentInfo{
		OrderNo: "N1", BuyerId: "11", StoreId: "22", Amount: "350", Status: status, ProductName: " 紅茶 ",
	}}
}

func TestOrderForPaymentMapsTheOrderAndOnlyAPendingPaymentOrderIsPayable(t *testing.T) {
	t.Parallel()
	got, err := (&Client{rpc: fakePaymentOrders{resp: paymentResponse("0")}}).OrderForPayment(context.Background(), "N1")
	if err != nil {
		t.Fatal(err)
	}
	want := PaymentOrder{OrderNo: "N1", BuyerID: 11, StoreID: 22, Amount: 350, Payable: true, ProductName: "紅茶"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for _, status := range []string{"10", "80"} {
		got, err := (&Client{rpc: fakePaymentOrders{resp: paymentResponse(status)}}).OrderForPayment(context.Background(), "N1")
		if err != nil || got.Payable {
			t.Fatalf("status %s: payable=%v err=%v, want not payable", status, got.Payable, err)
		}
	}
}

func TestOrderForPaymentSeparatesNotFoundFromEveryOtherFailure(t *testing.T) {
	t.Parallel()
	badBuyer := paymentResponse("0")
	badBuyer.Data.BuyerId = "x"
	badAmount := paymentResponse("0")
	badAmount.Data.Amount = "-1"
	unknownStatus := paymentResponse("999")
	wrongOrder := paymentResponse("0")
	wrongOrder.Data.OrderNo = "N2"
	cases := []struct {
		name         string
		fake         fakePaymentOrders
		wantNotFound bool
	}{
		{"code 404", fakePaymentOrders{resp: &orderpb.GetOrderForPaymentResponse{Code: 404}}, true},
		{"code 500", fakePaymentOrders{resp: &orderpb.GetOrderForPaymentResponse{Code: 500}}, false},
		{"no data", fakePaymentOrders{resp: &orderpb.GetOrderForPaymentResponse{}}, false},
		{"transport", fakePaymentOrders{err: errors.New("down")}, false},
		{"bad buyer", fakePaymentOrders{resp: badBuyer}, false},
		{"bad amount", fakePaymentOrders{resp: badAmount}, false},
		{"unknown status", fakePaymentOrders{resp: unknownStatus}, false},
		{"wrong order", fakePaymentOrders{resp: wrongOrder}, false},
	}
	for _, tc := range cases {
		_, err := (&Client{rpc: tc.fake}).OrderForPayment(context.Background(), "N1")
		if err == nil {
			t.Fatalf("%s: want an error", tc.name)
		}
		if got := errors.Is(err, ErrOrderNotFound); got != tc.wantNotFound {
			t.Fatalf("%s: NotFound=%v, want %v (err=%v)", tc.name, got, tc.wantNotFound, err)
		}
	}
}
