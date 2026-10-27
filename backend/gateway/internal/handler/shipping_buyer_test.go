package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	shippingpb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func setupShippingBuyerRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	app.GET("/orders/:order_no/logistics", h.GetOrderLogistics)
	app.POST("/store_pickups", h.AddBuyerStorePickup)
	app.GET("/store_pickups", h.ListBuyerStorePickups)
	return r
}

func TestGetOrderLogistics_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *shippingpb.GetShipmentTrackingRequest
	mock := &mockShippingServiceClient{
		GetShipmentTrackingFn: func(ctx context.Context, in *shippingpb.GetShipmentTrackingRequest, opts ...grpc.CallOption) (*shippingpb.GetShipmentResponse, error) {
			captured = in
			return &shippingpb.GetShipmentResponse{Code: 0, Data: &shippingpb.ShipmentDetailView{OrderNo: in.OrderNo}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), BuyerID: buyerResolverForTest(9999)}
	r := setupShippingBuyerRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/orders/ORD1/logistics", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("BuyerId = %+v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

func TestAddBuyerStorePickup_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *shippingpb.AddBuyerStorePickupRequest
	mock := &mockShippingServiceClient{
		AddBuyerStorePickupFn: func(ctx context.Context, in *shippingpb.AddBuyerStorePickupRequest, opts ...grpc.CallOption) (*shippingpb.AddBuyerStorePickupResponse, error) {
			captured = in
			return &shippingpb.AddBuyerStorePickupResponse{Code: 0, Data: &shippingpb.BuyerStorePickupView{Id: "1"}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), BuyerID: buyerResolverForTest(9999)}
	r := setupShippingBuyerRouter(h)

	body := `{"storeCode":"S001","storeName":"門市A","storeAddress":"地址A","receiverName":"王小明","receiverPhone":"0912345678","isDefault":false}`
	req := httptest.NewRequest("POST", "/api/v1/app/store_pickups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("BuyerId = %+v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

func TestListBuyerStorePickups_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *shippingpb.ListBuyerStorePickupsRequest
	mock := &mockShippingServiceClient{
		ListBuyerStorePickupsFn: func(ctx context.Context, in *shippingpb.ListBuyerStorePickupsRequest, opts ...grpc.CallOption) (*shippingpb.ListBuyerStorePickupsResponse, error) {
			captured = in
			return &shippingpb.ListBuyerStorePickupsResponse{Code: 0}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), BuyerID: buyerResolverForTest(9999)}
	r := setupShippingBuyerRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/store_pickups", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("BuyerId = %+v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}
