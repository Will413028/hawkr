package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ==================== router ====================

func setupOrderRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	{
		app.POST("/orders", h.CreateOrder)
		app.GET("/orders", h.GetMyOrders)
		app.GET("/orders/status_counts", h.GetMyOrderStatusCounts)
		app.GET("/orders/:order_no", h.GetOrder)
		app.POST("/orders/:order_no/cancellations", h.CancelOrder)
		app.POST("/orders/:order_no/confirm_receipt", h.ConfirmReceipt)
	}
	return r
}

// setupAdminSearchOrdersRouter：SearchOrders 是 admin-only 端點，handler 本身
// 不驗證身分（沿用既有慣例，見 ListAbnormalOrdersForAdmin），故不掛 fakeJWTAuth。
func setupAdminSearchOrdersRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/admin/orders", h.SearchOrders)
	return r
}

// ==================== GetMyOrders ====================

func TestGetMyOrders_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetMyOrdersFn: func(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error) {
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			if in.Page.GetPage() != "1" {
				t.Errorf("Page = %q, want 1", in.Page.GetPage())
			}
			return &orderpb.GetMyOrdersResponse{
				Code:    0,
				Message: "success",
				Orders: []*orderpb.OrderSummaryView{
					{
						OrderNo:           "ORD20260301001",
						Status:            "10",
						TotalActualAmount: "1250",
						ItemCount:         "2",
						SellerName:        "好農場",
						SellerAvatarKey:   "public/seller/123/avatar.jpg",
					},
				},
				Meta: &orderpb.PageMeta{
					CurrentPage: "1",
					TotalItems:  "1",
				},
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders?page=1&page_size=10", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["total"] != "1" {
		t.Errorf("total = %v, want 1", data["total"])
	}
	records, ok := data["records"].([]interface{})
	if !ok || len(records) != 1 {
		t.Fatalf("records length = %d, want 1", len(records))
	}
	first := records[0].(map[string]interface{})
	if first["orderNo"] != "ORD20260301001" {
		t.Errorf("orderNo = %v, want ORD20260301001", first["orderNo"])
	}
	// Verify canonical logoKey/logoUrl camelCase fields (49 §6.1)
	if got := first["logoKey"]; got != "public/seller/123/avatar.jpg" {
		t.Errorf("logoKey = %v, want %q", got, "public/seller/123/avatar.jpg")
	}
	if got := first["logoUrl"]; got == nil || got == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL", got)
	}
}

// ==================== GetMyOrderStatusCounts ====================

func TestGetMyOrderStatusCounts_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetMyOrderStatusCountsFn: func(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error) {
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &orderpb.GetMyOrderStatusCountsResponse{
				Code:    0,
				Message: "success",
				Counts: []*orderpb.OrderStatusCount{
					{Status: "0", Count: "3"},
					{Status: "20", Count: "5"},
				},
				Total: "8",
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/status_counts", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["total"] != "8" {
		t.Errorf("total = %v, want 8 (order-service 回傳值透傳)", data["total"])
	}
	counts, ok := data["counts"].([]interface{})
	if !ok || len(counts) != 2 {
		t.Fatalf("counts length = %d, want 2", len(counts))
	}
	first := counts[0].(map[string]interface{})
	if first["status"] != "0" || first["count"] != "3" {
		t.Errorf("first = %v, want {status:0, count:3}", first)
	}
}

func TestGetMyOrderStatusCounts_Empty(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetMyOrderStatusCountsFn: func(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error) {
			// Total 留 proto3 預設空字串，驗 gateway 兜 "0"（正常鏈路 order-service 0 筆時明確回 "0"）
			return &orderpb.GetMyOrderStatusCountsResponse{Code: 0, Message: "success"}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/status_counts", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
	data := resp["data"].(map[string]interface{})
	if data["total"] != "0" {
		t.Errorf("total = %v, want 0", data["total"])
	}
	counts, ok := data["counts"].([]interface{})
	if !ok || len(counts) != 0 {
		t.Errorf("counts = %v, want empty array", data["counts"])
	}
}

func TestGetMyOrderStatusCounts_SellerViewRejected(t *testing.T) {
	called := false
	mock := &mockOrderServiceClient{
		GetMyOrderStatusCountsFn: func(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error) {
			called = true
			return &orderpb.GetMyOrderStatusCountsResponse{}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock)}
	router := setupOrderRouter(h)
	for _, view := range []string{"seller", "selling"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/status_counts?view="+view, nil)
		req.Header.Set("X-Account-Id", "100")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusBadRequest)
		if called {
			t.Errorf("view=%s: gRPC should not be called on seller-view rejection", view)
		}
	}
}

func TestGetMyOrders_Unauthorized(t *testing.T) {
	h := &Holder{}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders", nil)
	// No X-Account-Id header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

// ==================== GetOrder ====================

func TestGetOrder_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetOrderFn: func(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
			if in.OrderNo != "ORD20260301001" {
				t.Errorf("OrderNo = %q, want ORD20260301001", in.OrderNo)
			}
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &orderpb.GetOrderResponse{
				Code:    0,
				Message: "success",
				Data: &orderpb.OrderDetailView{
					OrderNo:           "ORD20260301001",
					Status:            "10",
					TotalActualAmount: "1250",
					StoreId:           "200",
				},
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/ORD20260301001", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["orderNo"] != "ORD20260301001" {
		t.Errorf("orderNo = %v, want ORD20260301001", data["orderNo"])
	}
	if data["status"] != "10" {
		t.Errorf("status = %v, want 10", data["status"])
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetOrderFn: func(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
			return nil, status.Error(codes.NotFound, "order not found")
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/NONEXIST", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 404)
}

// TestGetOrderDetail_SellerLogoURLField proves order detail exposes
// canonical camelCase logoKey/logoUrl (49 §6.1 / issue #1782).
func TestGetOrderDetail_SellerLogoURLField(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetOrderFn: func(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
			return &orderpb.GetOrderResponse{Code: 0, Data: &orderpb.OrderDetailView{
				OrderNo: "ORD1", SellerName: "老王鮮魚", SellerAvatarKey: "public/seller/42/logo.png",
			}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/ORD1", nil)
	req.Header.Set("X-Account-Id", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if got := data["logoUrl"]; got == nil || got == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL", got)
	}
	if got := data["logoKey"]; got != "public/seller/42/logo.png" {
		t.Errorf("logoKey = %v, want %q", got, "public/seller/42/logo.png")
	}
}

// ==================== CancelOrder ====================

func TestCancelOrder_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		CancelOrderFn: func(ctx context.Context, in *orderpb.CancelOrderRequest, opts ...grpc.CallOption) (*orderpb.CancelOrderResponse, error) {
			if in.OrderNo != "ORD20260301001" {
				t.Errorf("OrderNo = %q, want ORD20260301001", in.OrderNo)
			}
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &orderpb.CancelOrderResponse{
				Code:    0,
				Message: "success",
				Success: true,
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders/ORD20260301001/cancellations", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["success"] != true {
		t.Errorf("success = %v, want true", data["success"])
	}
}

func TestCancelOrder_GrpcError(t *testing.T) {
	mock := &mockOrderServiceClient{
		CancelOrderFn: func(ctx context.Context, in *orderpb.CancelOrderRequest, opts ...grpc.CallOption) (*orderpb.CancelOrderResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders/ORD20260301001/cancellations", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Non-gRPC error → GrpcErr maps to ServiceUnavailable (503)
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== ConfirmReceipt ====================

func TestConfirmReceipt_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		ConfirmReceiptFn: func(ctx context.Context, in *orderpb.ConfirmReceiptRequest, opts ...grpc.CallOption) (*orderpb.ConfirmReceiptResponse, error) {
			if in.OrderNo != "ORD20260301001" {
				t.Errorf("OrderNo = %q, want ORD20260301001", in.OrderNo)
			}
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &orderpb.ConfirmReceiptResponse{
				Code:    0,
				Message: "success",
				Success: true,
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupOrderRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders/ORD20260301001/confirm_receipt", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["success"] != true {
		t.Errorf("success = %v, want true", data["success"])
	}
}

func TestGetMyOrders_SellerView_UsesResolvedSurrogate(t *testing.T) {
	var captured *orderpb.GetSellerOrdersRequest
	orderMock := &mockOrderServiceClient{
		GetSellerOrdersFn: func(ctx context.Context, in *orderpb.GetSellerOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error) {
			captured = in
			return &orderpb.GetSellerOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{TotalItems: "0", CurrentPage: "1"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: storeResolverForTest(7001)}
	r := setupOrderRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/orders?view=seller", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != 7001 {
		t.Errorf("StoreId = %+v, want 7001 (resolved surrogate, not login id 99)", captured)
	}
}

func TestGetMyOrders_SellerView_NoCurrentSeller403(t *testing.T) {
	orderMock := &mockOrderServiceClient{
		GetSellerOrdersFn: func(ctx context.Context, in *orderpb.GetSellerOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error) {
			t.Fatal("order-service must not be called without a current seller")
			return nil, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: noStoreResolverForTest()}
	r := setupOrderRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/orders?view=seller", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	assertCode(t, parseJSON(t, w.Body), 403)
}

func TestGetOrder_SellerView_UsesResolvedSurrogate(t *testing.T) {
	var captured *orderpb.GetSellerOrderDetailRequest
	orderMock := &mockOrderServiceClient{
		GetSellerOrderDetailFn: func(ctx context.Context, in *orderpb.GetSellerOrderDetailRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrderDetailResponse, error) {
			captured = in
			return &orderpb.GetSellerOrderDetailResponse{
				Code: 0,
				Data: &orderpb.OrderDetailView{OrderNo: "ORD20260301001", Status: "10"},
			}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: storeResolverForTest(7001)}
	r := setupOrderRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/orders/ORD20260301001?view=seller", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != 7001 {
		t.Errorf("StoreId = %+v, want 7001 (resolved surrogate, not login id 99)", captured)
	}
}

func TestGetOrder_SellerView_NoCurrentSeller403(t *testing.T) {
	orderMock := &mockOrderServiceClient{
		GetSellerOrderDetailFn: func(ctx context.Context, in *orderpb.GetSellerOrderDetailRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrderDetailResponse, error) {
			t.Fatal("order-service must not be called without a current seller")
			return nil, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: noStoreResolverForTest()}
	r := setupOrderRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/orders/ORD20260301001?view=seller", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	assertCode(t, parseJSON(t, w.Body), 403)
}

// ==================== ② buyer store-stint re-key: surrogate pinning ====================

func TestCreateOrder_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.CreateOrderRequest
	orderMock := &mockOrderServiceClient{
		CreateOrderFn: func(ctx context.Context, in *orderpb.CreateOrderRequest, opts ...grpc.CallOption) (*orderpb.CreateOrderResponse, error) {
			captured = in
			return &orderpb.CreateOrderResponse{Code: 0, Data: &orderpb.CreateOrderData{OrderNo: "ORD1"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	router := setupOrderRouter(h)

	body := `{"storeId":"200","totalOriginalAmount":"100","totalActualAmount":"100","items":[{"skuId":"SKU1","skuName":"n","quantity":"1","unitPrice":"100"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestCreateOrder_NoCurrentBuyer403(t *testing.T) {
	called := false
	orderMock := &mockOrderServiceClient{
		CreateOrderFn: func(ctx context.Context, in *orderpb.CreateOrderRequest, opts ...grpc.CallOption) (*orderpb.CreateOrderResponse, error) {
			called = true
			return nil, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: noBuyerResolverForTest()}
	router := setupOrderRouter(h)

	body := `{"storeId":"200","totalOriginalAmount":"100","totalActualAmount":"100","items":[{"skuId":"SKU1","skuName":"n","quantity":"1","unitPrice":"100"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	if called {
		t.Error("order-service must not be called without a current buyer")
	}
}

func TestGetMyOrders_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.GetMyOrdersRequest
	orderMock := &mockOrderServiceClient{
		GetMyOrdersFn: func(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error) {
			captured = in
			return &orderpb.GetMyOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{TotalItems: "0", CurrentPage: "1"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	router := setupOrderRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestGetMyOrderStatusCounts_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.GetMyOrderStatusCountsRequest
	orderMock := &mockOrderServiceClient{
		GetMyOrderStatusCountsFn: func(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error) {
			captured = in
			return &orderpb.GetMyOrderStatusCountsResponse{Code: 0}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	router := setupOrderRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/status_counts", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestGetOrder_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.GetOrderRequest
	orderMock := &mockOrderServiceClient{
		GetOrderFn: func(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
			captured = in
			return &orderpb.GetOrderResponse{Code: 0, Data: &orderpb.OrderDetailView{OrderNo: "ORD1"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	router := setupOrderRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders/ORD1", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

// ② IDOR-critical: same as GetOrder.
func TestCancelOrder_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.CancelOrderRequest
	orderMock := &mockOrderServiceClient{
		CancelOrderFn: func(ctx context.Context, in *orderpb.CancelOrderRequest, opts ...grpc.CallOption) (*orderpb.CancelOrderResponse, error) {
			captured = in
			return &orderpb.CancelOrderResponse{Code: 0, Success: true}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	router := setupOrderRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders/ORD1/cancellations", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

// ② IDOR-critical: same as GetOrder.
func TestConfirmReceipt_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.ConfirmReceiptRequest
	orderMock := &mockOrderServiceClient{
		ConfirmReceiptFn: func(ctx context.Context, in *orderpb.ConfirmReceiptRequest, opts ...grpc.CallOption) (*orderpb.ConfirmReceiptResponse, error) {
			captured = in
			return &orderpb.ConfirmReceiptResponse{Code: 0, Success: true}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	router := setupOrderRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders/ORD1/confirm_receipt", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestGetMyOrders_SellerView_BuyerFilter_ResolvesTarget(t *testing.T) {
	var captured *orderpb.GetSellerOrdersRequest
	orderMock := &mockOrderServiceClient{
		GetSellerOrdersFn: func(ctx context.Context, in *orderpb.GetSellerOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error) {
			captured = in
			return &orderpb.GetSellerOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{TotalItems: "0", CurrentPage: "1"}}, nil
		},
	}
	h := &Holder{
		Order:   grpcclient.NewOrderClientForTest(orderMock),
		Store:   storeResolverForTest(7001),
		BuyerID: buyerResolverForTest(8001),
	}
	r := setupOrderRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/orders?view=seller&buyer_id=42", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "8001" {
		t.Errorf("BuyerId = %+v, want \"8001\" (resolved surrogate, not raw 42)", captured)
	}
}

func TestSearchOrders_BuyerFilter_ResolvesTarget(t *testing.T) {
	var captured *orderpb.SearchOrdersRequest
	orderMock := &mockOrderServiceClient{
		SearchOrdersFn: func(ctx context.Context, in *orderpb.SearchOrdersRequest, opts ...grpc.CallOption) (*orderpb.SearchOrdersResponse, error) {
			captured = in
			return &orderpb.SearchOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{TotalItems: "0", CurrentPage: "1"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminSearchOrdersRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/orders?buyer_id=42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "8001" {
		t.Errorf("BuyerId = %+v, want \"8001\" (resolved surrogate, not raw 42)", captured)
	}
}

func TestSearchOrders_NoBuyerFilter_PassesThroughEmpty(t *testing.T) {
	var captured *orderpb.SearchOrdersRequest
	orderMock := &mockOrderServiceClient{
		SearchOrdersFn: func(ctx context.Context, in *orderpb.SearchOrdersRequest, opts ...grpc.CallOption) (*orderpb.SearchOrdersResponse, error) {
			captured = in
			return &orderpb.SearchOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{TotalItems: "0", CurrentPage: "1"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), BuyerID: buyerResolverForTest(8001), Store: storeResolverForTest(7001)}
	r := setupAdminSearchOrdersRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/orders?store_id=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "" {
		t.Errorf("BuyerId = %+v, want empty (no filter supplied)", captured)
	}
}

// 49 §5.3 #3：訂單詳情出貨/收貨照片、評論附圖，proto 值為 objectKey，
// Gateway 回 {語意}Keys（裸 key）+ {語意}Urls（BuildURL 組），值形正確且對存量完整 URL passthrough。
func TestConvertOrderDetail_PhotoAndReviewDualField(t *testing.T) {
	const base = "https://test-media.example.com"
	d := &orderpb.OrderDetailView{
		OrderNo:           "O1",
		ShipmentPhotoKeys: []string{"public/shipping/1/a.jpg"},
		ReceiptPhotoKeys:  []string{"public/order/1/r.jpg", "https://lh3.googleusercontent.com/x"},
		Review: &orderpb.OrderReviewView{
			Rating:    "5",
			ImageKeys: []string{"public/review/1/i.jpg"},
		},
	}
	r := convertOrderDetail(d, base)

	if len(r.ShipmentPhotoKeys) != 1 || r.ShipmentPhotoKeys[0] != "public/shipping/1/a.jpg" {
		t.Errorf("ShipmentPhotoKeys = %v, want bare key", r.ShipmentPhotoKeys)
	}
	if len(r.ShipmentPhotoUrls) != 1 || r.ShipmentPhotoUrls[0] != base+"/i/original/public/shipping/1/a.jpg" {
		t.Errorf("ShipmentPhotoUrls = %v, want composed URL", r.ShipmentPhotoUrls)
	}
	// 收貨照片：本平台 key 組 URL；外部 http（Google）passthrough 不重複組
	if r.ReceiptPhotoUrls[0] != base+"/i/original/public/order/1/r.jpg" {
		t.Errorf("ReceiptPhotoUrls[0] = %q, want composed", r.ReceiptPhotoUrls[0])
	}
	if r.ReceiptPhotoUrls[1] != "https://lh3.googleusercontent.com/x" {
		t.Errorf("ReceiptPhotoUrls[1] = %q, want passthrough external URL", r.ReceiptPhotoUrls[1])
	}
	if r.Review == nil || len(r.Review.ImageKeys) != 1 || r.Review.ImageKeys[0] != "public/review/1/i.jpg" {
		t.Errorf("Review.ImageKeys = %v, want bare key", r.Review)
	}
	if r.Review.ImageUrls[0] != base+"/i/original/public/review/1/i.jpg" {
		t.Errorf("Review.ImageUrls[0] = %q, want composed URL", r.Review.ImageUrls[0])
	}
}

// TestGetMyOrders_PageParamParsing_Unaffected is a characterization test, not a
// bug-reproduction test: pagination.NormalizePage (called right after the parse,
// order.go:647) already clamps page<=0 to 1 and pageSize<=0 to defaultLimit, so
// a non-numeric/negative page/page_size here already resolves to the correct
// default today — strconv.Atoi's discarded-error zero value lands inside
// NormalizePage's <=0 branch by coincidence. This test proves the
// ParsePageParam conversion in this task doesn't change that outcome; it is
// expected to PASS both before and after the refactor.
func TestGetMyOrders_PageParamParsing_Unaffected(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantPage string
		wantSize string
	}{
		{"合法數字", "page=3&page_size=5", "3", "5"},
		{"超過上限會 clamp", "page=3&page_size=999", "3", "50"},
		{"非數字字串 fallback 到 default", "page=abc&page_size=xyz", "1", "10"},
		{"空值 fallback 到 default", "", "1", "10"},
		{"負數 fallback 到 default", "page=-1&page_size=-5", "1", "10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured *orderpb.GetMyOrdersRequest
			mock := &mockOrderServiceClient{
				GetMyOrdersFn: func(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error) {
					captured = in
					return &orderpb.GetMyOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{}}, nil
				},
			}
			h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerResolverForTest(100)}
			r := setupOrderRouter(h)
			url := "/api/v1/app/orders"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-Account-Id", "1")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, http.StatusOK)
			if captured == nil {
				t.Fatal("GetMyOrders was not called")
			}
			if captured.Page.GetPage() != tt.wantPage {
				t.Errorf("Page = %q, want %q", captured.Page.GetPage(), tt.wantPage)
			}
			if captured.Page.GetSize() != tt.wantSize {
				t.Errorf("Page.Size = %q, want %q", captured.Page.GetSize(), tt.wantSize)
			}
		})
	}
}
