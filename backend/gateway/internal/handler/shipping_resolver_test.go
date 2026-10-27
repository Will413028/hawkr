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

// mockShippingServiceClient 只覆寫測試需要的 RPC；其餘靠 interface 內嵌
// （呼叫到未覆寫方法會 nil-panic，即測試失敗——刻意）。
type mockShippingServiceClient struct {
	shippingpb.ShippingServiceClient
	PrintCvsLabelFn               func(ctx context.Context, in *shippingpb.PrintCvsLabelRequest, opts ...grpc.CallOption) (*shippingpb.PrintCvsLabelResponse, error)
	ListSellerShipmentAnomaliesFn func(ctx context.Context, in *shippingpb.ListSellerShipmentAnomaliesRequest, opts ...grpc.CallOption) (*shippingpb.ListSellerShipmentAnomaliesResponse, error)
	ListSellerShippingMethodsFn   func(ctx context.Context, in *shippingpb.ListSellerShippingMethodsRequest, opts ...grpc.CallOption) (*shippingpb.ListSellerShippingMethodsResponse, error)
	GetShipmentTrackingFn         func(ctx context.Context, in *shippingpb.GetShipmentTrackingRequest, opts ...grpc.CallOption) (*shippingpb.GetShipmentResponse, error)
	AddBuyerStorePickupFn         func(ctx context.Context, in *shippingpb.AddBuyerStorePickupRequest, opts ...grpc.CallOption) (*shippingpb.AddBuyerStorePickupResponse, error)
	ListBuyerStorePickupsFn       func(ctx context.Context, in *shippingpb.ListBuyerStorePickupsRequest, opts ...grpc.CallOption) (*shippingpb.ListBuyerStorePickupsResponse, error)
}

func (m *mockShippingServiceClient) PrintCvsLabel(ctx context.Context, in *shippingpb.PrintCvsLabelRequest, opts ...grpc.CallOption) (*shippingpb.PrintCvsLabelResponse, error) {
	return m.PrintCvsLabelFn(ctx, in, opts...)
}

func (m *mockShippingServiceClient) ListSellerShipmentAnomalies(ctx context.Context, in *shippingpb.ListSellerShipmentAnomaliesRequest, opts ...grpc.CallOption) (*shippingpb.ListSellerShipmentAnomaliesResponse, error) {
	return m.ListSellerShipmentAnomaliesFn(ctx, in, opts...)
}

func (m *mockShippingServiceClient) ListSellerShippingMethods(ctx context.Context, in *shippingpb.ListSellerShippingMethodsRequest, opts ...grpc.CallOption) (*shippingpb.ListSellerShippingMethodsResponse, error) {
	return m.ListSellerShippingMethodsFn(ctx, in, opts...)
}

func (m *mockShippingServiceClient) GetShipmentTracking(ctx context.Context, in *shippingpb.GetShipmentTrackingRequest, opts ...grpc.CallOption) (*shippingpb.GetShipmentResponse, error) {
	return m.GetShipmentTrackingFn(ctx, in, opts...)
}

func (m *mockShippingServiceClient) AddBuyerStorePickup(ctx context.Context, in *shippingpb.AddBuyerStorePickupRequest, opts ...grpc.CallOption) (*shippingpb.AddBuyerStorePickupResponse, error) {
	return m.AddBuyerStorePickupFn(ctx, in, opts...)
}

func (m *mockShippingServiceClient) ListBuyerStorePickups(ctx context.Context, in *shippingpb.ListBuyerStorePickupsRequest, opts ...grpc.CallOption) (*shippingpb.ListBuyerStorePickupsResponse, error) {
	return m.ListBuyerStorePickupsFn(ctx, in, opts...)
}

func setupShippingResolverRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.POST("/shipments/:order_no/print_cvs_label", h.PrintCvsLabel)
	app.GET("/shipments/anomalies", h.ListSellerShipmentAnomalies)
	return r
}

func TestPrintCvsLabel_UsesResolvedSurrogate(t *testing.T) {
	var captured *shippingpb.PrintCvsLabelRequest
	mock := &mockShippingServiceClient{
		PrintCvsLabelFn: func(ctx context.Context, in *shippingpb.PrintCvsLabelRequest, opts ...grpc.CallOption) (*shippingpb.PrintCvsLabelResponse, error) {
			captured = in
			return &shippingpb.PrintCvsLabelResponse{Code: 0, Data: &shippingpb.CvsLabelData{ShipmentNo: "S1"}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupShippingResolverRouter(h)

	req := httptest.NewRequest("POST", "/api/v1/app/shipments/ORD1/print_cvs_label", strings.NewReader(`{"shipDate":"20260711"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != 7001 {
		t.Errorf("StoreId = %+v, want 7001 (resolved surrogate)", captured)
	}
}

func TestListSellerShipmentAnomalies_UsesResolvedSurrogate(t *testing.T) {
	var captured *shippingpb.ListSellerShipmentAnomaliesRequest
	mock := &mockShippingServiceClient{
		ListSellerShipmentAnomaliesFn: func(ctx context.Context, in *shippingpb.ListSellerShipmentAnomaliesRequest, opts ...grpc.CallOption) (*shippingpb.ListSellerShipmentAnomaliesResponse, error) {
			captured = in
			return &shippingpb.ListSellerShipmentAnomaliesResponse{Code: 0, Data: &shippingpb.ListSellerShipmentAnomaliesData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupShippingResolverRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/shipments/anomalies", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != "7001" {
		t.Errorf("StoreId = %+v, want \"7001\" (resolved surrogate as string)", captured)
	}
}
