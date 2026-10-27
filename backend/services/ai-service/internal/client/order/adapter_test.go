package order

import (
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"google.golang.org/grpc"
)

// fakeOrderServiceClient satisfies orderpb.OrderServiceClient by embedding
// the interface (unimplemented methods panic-if-called) and overriding only
// GetSellerOrders and GetSellerOrderDetail, capturing the outgoing request for assertions.
//
// The four `*Code`/`*Message` fields let a test simulate order-service's
// "200-shaped business error" contract (OrderGrpcService returns auth
// failures, permission denials, and not-found via onNext with a non-zero
// Code and no Data — never onError). Zero value (Code: 0) preserves each
// method's original always-succeeds behavior, so existing tests are
// unaffected.
type fakeOrderServiceClient struct {
	orderpb.OrderServiceClient
	gotSellerOrdersReq *orderpb.GetSellerOrdersRequest
	gotOrderDetailReq  *orderpb.GetSellerOrderDetailRequest
	gotMyOrdersReq     *orderpb.GetMyOrdersRequest
	gotGetOrderReq     *orderpb.GetOrderRequest

	sellerOrdersCode      int32
	sellerOrdersMessage   string
	sellerOrderDetailCode int32
	sellerOrderDetailMsg  string
	myOrdersCode          int32
	myOrdersMessage       string
	getOrderCode          int32
	getOrderMessage       string
}

func (f *fakeOrderServiceClient) GetSellerOrders(_ context.Context, in *orderpb.GetSellerOrdersRequest, _ ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error) {
	f.gotSellerOrdersReq = in
	if f.sellerOrdersCode != 0 {
		return &orderpb.GetSellerOrdersResponse{Code: f.sellerOrdersCode, Message: f.sellerOrdersMessage}, nil
	}
	return &orderpb.GetSellerOrdersResponse{Code: 0, Orders: nil}, nil
}

func (f *fakeOrderServiceClient) GetSellerOrderDetail(_ context.Context, in *orderpb.GetSellerOrderDetailRequest, _ ...grpc.CallOption) (*orderpb.GetSellerOrderDetailResponse, error) {
	f.gotOrderDetailReq = in
	if f.sellerOrderDetailCode != 0 {
		return &orderpb.GetSellerOrderDetailResponse{Code: f.sellerOrderDetailCode, Message: f.sellerOrderDetailMsg}, nil
	}
	return &orderpb.GetSellerOrderDetailResponse{
		Code: 0,
		Data: &orderpb.OrderDetailView{
			OrderNo:           "ORDER123",
			Status:            "pending",
			TotalActualAmount: "1000",
			BuyerId:           "99",
			ShipmentNo:        "SHIP123",
			ShippingStatus:    "pending",
		},
	}, nil
}

func (f *fakeOrderServiceClient) GetMyOrders(_ context.Context, in *orderpb.GetMyOrdersRequest, _ ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error) {
	f.gotMyOrdersReq = in
	if f.myOrdersCode != 0 {
		return &orderpb.GetMyOrdersResponse{Code: f.myOrdersCode, Message: f.myOrdersMessage}, nil
	}
	return &orderpb.GetMyOrdersResponse{Code: 0, Orders: nil}, nil
}

func (f *fakeOrderServiceClient) GetOrder(_ context.Context, in *orderpb.GetOrderRequest, _ ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
	f.gotGetOrderReq = in
	if f.getOrderCode != 0 {
		return &orderpb.GetOrderResponse{Code: f.getOrderCode, Message: f.getOrderMessage}, nil
	}
	return &orderpb.GetOrderResponse{
		Code: 0,
		Data: &orderpb.OrderDetailView{
			OrderNo:           "ORDER456",
			Status:            "pending",
			TotalActualAmount: "500",
			ShippingFee:       "50",
		},
	}, nil
}

// TestGetSellerOrders_ForwardsStoreID pins that the storeID param (already the
// caller's CURRENT store_id — resolved by gateway before this call reaches
// ai-service, account_id 命名債正名) is forwarded to order-service as-is via
// GetSellerOrdersRequest.StoreId, with no further resolution in this adapter.
func TestGetSellerOrders_ForwardsStoreID(t *testing.T) {
	const storeID identity.StoreID = 8001

	fakeClient := &fakeOrderServiceClient{}
	c := &Client{client: fakeClient}

	if _, err := c.GetSellerOrders(context.Background(), storeID, "", 1, 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeClient.gotSellerOrdersReq == nil {
		t.Fatal("GetSellerOrders was never called")
	}
	if got := fakeClient.gotSellerOrdersReq.GetStoreId(); got != int64(storeID) {
		t.Errorf("GetSellerOrdersRequest.StoreId = %d, want %d", got, int64(storeID))
	}
}

// TestGetSellerOrderDetail_ForwardsStoreID ensures the same passthrough happens
// in GetSellerOrderDetail.
func TestGetSellerOrderDetail_ForwardsStoreID(t *testing.T) {
	const storeID identity.StoreID = 8001

	fakeClient := &fakeOrderServiceClient{}
	c := &Client{client: fakeClient}

	if _, err := c.GetSellerOrderDetail(context.Background(), storeID, "ORDER123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeClient.gotOrderDetailReq == nil {
		t.Fatal("GetSellerOrderDetail was never called")
	}
	if got := fakeClient.gotOrderDetailReq.GetStoreId(); got != int64(storeID) {
		t.Errorf("GetSellerOrderDetailRequest.StoreId = %d, want %d", got, int64(storeID))
	}
}

func TestGetMyOrders_ForwardsBuyerID(t *testing.T) {
	const buyerID identity.BuyerID = 55321

	fakeClient := &fakeOrderServiceClient{}
	c := &Client{client: fakeClient}

	if _, err := c.GetMyOrders(context.Background(), buyerID, "", 1, 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeClient.gotMyOrdersReq == nil {
		t.Fatal("GetMyOrders was never called")
	}
	if got := fakeClient.gotMyOrdersReq.GetBuyerId(); got != int64(buyerID) {
		t.Errorf("GetMyOrdersRequest.BuyerId = %d, want %d", got, int64(buyerID))
	}
}

// TestGetBuyerOrderDetail_ForwardsBuyerID ensures the same passthrough happens
// in GetBuyerOrderDetail.
func TestGetBuyerOrderDetail_ForwardsBuyerID(t *testing.T) {
	const buyerID identity.BuyerID = 55321

	fakeClient := &fakeOrderServiceClient{}
	c := &Client{client: fakeClient}

	if _, err := c.GetBuyerOrderDetail(context.Background(), buyerID, "ORDER456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeClient.gotGetOrderReq == nil {
		t.Fatal("GetOrder was never called")
	}
	if got := fakeClient.gotGetOrderReq.GetBuyerId(); got != int64(buyerID) {
		t.Errorf("GetOrderRequest.BuyerId = %d, want %d", got, int64(buyerID))
	}
}

// The four tests below guard against order-service's "200-shaped business
// error" contract being silently swallowed: OrderGrpcService reports auth
// failures / permission denials / not-found via onNext with a non-zero Code
// and no Data — never onError — so `err != nil` alone can't detect them.
// Before this fix, GetMyOrders/GetSellerOrders ignored Code entirely (a
// non-zero-code response with an empty Orders slice looked identical to a
// legitimately empty list), and GetBuyerOrderDetail/GetSellerOrderDetail
// only checked `Data == nil`, losing the actual Code/Message.

func TestGetMyOrders_NonZeroCode_ReturnsError(t *testing.T) {
	fakeClient := &fakeOrderServiceClient{myOrdersCode: 4030, myOrdersMessage: "permission denied"}
	c := &Client{client: fakeClient}

	orders, err := c.GetMyOrders(context.Background(), 55321, "", 1, 100)
	if err == nil {
		t.Fatal("GetMyOrders() error = nil, want non-nil for a non-zero response code")
	}
	if orders != nil {
		t.Errorf("GetMyOrders() orders = %v, want nil on error", orders)
	}
	if !strings.Contains(err.Error(), "4030") || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("GetMyOrders() error = %q, want it to surface code 4030 and the message", err.Error())
	}
}

func TestGetSellerOrders_NonZeroCode_ReturnsError(t *testing.T) {
	fakeClient := &fakeOrderServiceClient{sellerOrdersCode: 4030, sellerOrdersMessage: "permission denied"}
	c := &Client{client: fakeClient}

	orders, err := c.GetSellerOrders(context.Background(), 8001, "", 1, 100)
	if err == nil {
		t.Fatal("GetSellerOrders() error = nil, want non-nil for a non-zero response code")
	}
	if orders != nil {
		t.Errorf("GetSellerOrders() orders = %v, want nil on error", orders)
	}
	if !strings.Contains(err.Error(), "4030") || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("GetSellerOrders() error = %q, want it to surface code 4030 and the message", err.Error())
	}
}

func TestGetBuyerOrderDetail_NonZeroCode_ReturnsError(t *testing.T) {
	fakeClient := &fakeOrderServiceClient{getOrderCode: 4040, getOrderMessage: "not found"}
	c := &Client{client: fakeClient}

	detail, err := c.GetBuyerOrderDetail(context.Background(), 55321, "ORDER456")
	if err == nil {
		t.Fatal("GetBuyerOrderDetail() error = nil, want non-nil for a non-zero response code")
	}
	if detail != nil {
		t.Errorf("GetBuyerOrderDetail() detail = %v, want nil on error", detail)
	}
	if !strings.Contains(err.Error(), "4040") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("GetBuyerOrderDetail() error = %q, want it to surface code 4040 and the message", err.Error())
	}
}

func TestGetSellerOrderDetail_NonZeroCode_ReturnsError(t *testing.T) {
	fakeClient := &fakeOrderServiceClient{sellerOrderDetailCode: 4040, sellerOrderDetailMsg: "not found"}
	c := &Client{client: fakeClient}

	detail, err := c.GetSellerOrderDetail(context.Background(), 8001, "ORDER123")
	if err == nil {
		t.Fatal("GetSellerOrderDetail() error = nil, want non-nil for a non-zero response code")
	}
	if detail != nil {
		t.Errorf("GetSellerOrderDetail() detail = %v, want nil on error", detail)
	}
	if !strings.Contains(err.Error(), "4040") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("GetSellerOrderDetail() error = %q, want it to surface code 4040 and the message", err.Error())
	}
}
