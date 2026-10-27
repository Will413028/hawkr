package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func setupAdminAbnormalOrdersRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/admin/orders/abnormal", h.ListAbnormalOrdersForAdmin)
	return r
}

func TestListAbnormalOrdersForAdmin_NoBuyerFilter_PassesThroughEmpty(t *testing.T) {
	var captured *orderpb.ListAbnormalOrdersForAdminRequest
	orderMock := &mockOrderServiceClient{
		ListAbnormalOrdersForAdminFn: func(ctx context.Context, in *orderpb.ListAbnormalOrdersForAdminRequest, opts ...grpc.CallOption) (*orderpb.ListAbnormalOrdersForAdminResponse, error) {
			captured = in
			return &orderpb.ListAbnormalOrdersForAdminResponse{Code: 0, Data: &orderpb.ListAbnormalOrdersForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminAbnormalOrdersRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/orders/abnormal", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "" {
		t.Errorf("BuyerId = %+v, want empty (no filter supplied)", captured)
	}
}

func TestListAbnormalOrdersForAdmin_BuyerFilter_ResolvesTarget(t *testing.T) {
	var captured *orderpb.ListAbnormalOrdersForAdminRequest
	orderMock := &mockOrderServiceClient{
		ListAbnormalOrdersForAdminFn: func(ctx context.Context, in *orderpb.ListAbnormalOrdersForAdminRequest, opts ...grpc.CallOption) (*orderpb.ListAbnormalOrdersForAdminResponse, error) {
			captured = in
			return &orderpb.ListAbnormalOrdersForAdminResponse{Code: 0, Data: &orderpb.ListAbnormalOrdersForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminAbnormalOrdersRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/orders/abnormal?buyer_id=42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "8001" {
		t.Errorf("BuyerId = %+v, want \"8001\" (resolved surrogate, not raw 42)", captured)
	}
}

func TestListAbnormalOrdersForAdmin_InvalidBuyerFilter_BadRequest(t *testing.T) {
	orderMock := &mockOrderServiceClient{}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminAbnormalOrdersRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/orders/abnormal?buyer_id=not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}
