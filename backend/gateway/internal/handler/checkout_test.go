package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ==================== router ====================

func setupCheckoutRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app/checkout")
	app.Use(fakeJWTAuth())
	{
		app.POST("/validate", h.CheckoutValidate)
		app.POST("/shipping_fee", h.CheckoutShippingFee)
		app.POST("/preview", h.CheckoutPreview)
		app.POST("/confirm", h.CheckoutConfirm)
	}
	return r
}

// ==================== CheckoutValidate ====================

func TestCheckoutValidate_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutValidateFn: func(ctx context.Context, in *orderpb.CheckoutValidateRequest, opts ...grpc.CallOption) (*orderpb.CheckoutValidateResponse, error) {
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			if len(in.Items) != 1 {
				t.Errorf("Items length = %d, want 1", len(in.Items))
			}
			return &orderpb.CheckoutValidateResponse{
				Code:    0,
				Message: "success",
				Data: &orderpb.CheckoutValidateData{
					Valid:                    true,
					DeliveryTempIntersection: []string{"CHILLED"},
					AvailableShippingOptions: []*orderpb.AvailableShippingOption{
						{
							ShippingMethod: "TAQBIN",
							DeliveryTemp:   "CHILLED",
							Label:          "宅配冷藏",
							Fee:            "150",
							FeePayer:       "BUYER",
						},
					},
				},
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if data["valid"] != true {
		t.Errorf("valid = %v, want true", data["valid"])
	}
	temps, ok := data["deliveryTempIntersection"].([]interface{})
	if !ok || len(temps) != 1 {
		t.Fatalf("deliveryTempIntersection length = %v, want 1", temps)
	}
	if temps[0] != "CHILLED" {
		t.Errorf("deliveryTempIntersection[0] = %v, want CHILLED", temps[0])
	}
}

func TestCheckoutValidate_BadRequest(t *testing.T) {
	h := &Holder{BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCheckoutRouter(h)
	// Missing required "items" field
	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

// ==================== CheckoutPreview ====================

func TestCheckoutPreview_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutPreviewFn: func(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error) {
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			if in.ShippingMethod != "TAQBIN" {
				t.Errorf("ShippingMethod = %q, want TAQBIN", in.ShippingMethod)
			}
			return &orderpb.CheckoutPreviewResponse{
				Code:    0,
				Message: "success",
				Data: &orderpb.CheckoutPreviewData{
					CheckoutId:          "chk-uuid-001",
					TotalOriginalAmount: "1000",
					TotalShippingFee:    "150",
					TotalDiscount:       "0",
					TotalActualAmount:   "1150",
				},
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if data["checkoutId"] != "chk-uuid-001" {
		t.Errorf("checkoutId = %v, want chk-uuid-001", data["checkoutId"])
	}
	if data["totalActualAmount"] != "1150" {
		t.Errorf("totalActualAmount = %v, want 1150", data["totalActualAmount"])
	}
}

func TestCheckoutPreview_GrpcError(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutPreviewFn: func(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error) {
			return nil, status.Error(codes.Internal, "internal error")
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusInternalServerError)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 500)
}

// ==================== CheckoutConfirm ====================

func TestCheckoutConfirm_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutConfirmFn: func(ctx context.Context, in *orderpb.CheckoutConfirmRequest, opts ...grpc.CallOption) (*orderpb.CheckoutConfirmResponse, error) {
			if in.CheckoutId != "chk-uuid-001" {
				t.Errorf("CheckoutId = %q, want chk-uuid-001", in.CheckoutId)
			}
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &orderpb.CheckoutConfirmResponse{
				Code:    0,
				Message: "success",
				Data: &orderpb.CheckoutConfirmData{
					CheckoutId:     "chk-uuid-001",
					OrderNos:       []string{"ORD20260301001"},
					TotalPayAmount: "1150",
				},
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"checkoutId":     "chk-uuid-001",
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"recipientName":  "王小明",
		"recipientPhone": "0912345678",
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if data["checkoutId"] != "chk-uuid-001" {
		t.Errorf("checkoutId = %v, want chk-uuid-001", data["checkoutId"])
	}
	if data["totalPayAmount"] != "1150" {
		t.Errorf("totalPayAmount = %v, want 1150", data["totalPayAmount"])
	}
	orderNos, ok := data["orderNos"].([]interface{})
	if !ok || len(orderNos) != 1 {
		t.Fatalf("orderNos length = %v, want 1", orderNos)
	}
	if orderNos[0] != "ORD20260301001" {
		t.Errorf("orderNos[0] = %v, want ORD20260301001", orderNos[0])
	}
}

func TestCheckoutConfirm_BadRequest(t *testing.T) {
	h := &Holder{BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCheckoutRouter(h)
	// Missing required "checkoutId" field
	body, _ := json.Marshal(map[string]interface{}{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"recipientName":  "王小明",
		"recipientPhone": "0912345678",
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

// ==================== CheckoutShippingFee ====================

func TestCheckoutShippingFee_Success(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutShippingFeeFn: func(ctx context.Context, in *orderpb.CheckoutShippingFeeRequest, opts ...grpc.CallOption) (*orderpb.CheckoutShippingFeeResponse, error) {
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &orderpb.CheckoutShippingFeeResponse{
				Code: 0, Message: "success",
				Data: &orderpb.CheckoutShippingFeeData{ShippingMethod: "TAQBIN", DeliveryTemp: "CHILLED", ShippingFee: "150", EstimatedTotal: "1150"},
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/shipping_fee", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if data["shippingFee"] != "150" {
		t.Errorf("shippingFee = %v, want 150", data["shippingFee"])
	}
}

// ==================== ② buyer store-stint re-key: surrogate pinning ====================

func TestCheckoutValidate_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.CheckoutValidateRequest
	mock := &mockOrderServiceClient{
		CheckoutValidateFn: func(ctx context.Context, in *orderpb.CheckoutValidateRequest, opts ...grpc.CallOption) (*orderpb.CheckoutValidateResponse, error) {
			captured = in
			return &orderpb.CheckoutValidateResponse{Code: 0, Data: &orderpb.CheckoutValidateData{Valid: true}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]string{{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestCheckoutShippingFee_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.CheckoutShippingFeeRequest
	mock := &mockOrderServiceClient{
		CheckoutShippingFeeFn: func(ctx context.Context, in *orderpb.CheckoutShippingFeeRequest, opts ...grpc.CallOption) (*orderpb.CheckoutShippingFeeResponse, error) {
			captured = in
			return &orderpb.CheckoutShippingFeeResponse{Code: 0, Data: &orderpb.CheckoutShippingFeeData{}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"items":          []map[string]string{{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/shipping_fee", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestCheckoutPreview_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.CheckoutPreviewRequest
	mock := &mockOrderServiceClient{
		CheckoutPreviewFn: func(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error) {
			captured = in
			return &orderpb.CheckoutPreviewResponse{Code: 0, Data: &orderpb.CheckoutPreviewData{CheckoutId: "chk-1"}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"items":          []map[string]string{{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestCheckoutConfirm_UsesResolvedBuyerID(t *testing.T) {
	var captured *orderpb.CheckoutConfirmRequest
	mock := &mockOrderServiceClient{
		CheckoutConfirmFn: func(ctx context.Context, in *orderpb.CheckoutConfirmRequest, opts ...grpc.CallOption) (*orderpb.CheckoutConfirmResponse, error) {
			captured = in
			return &orderpb.CheckoutConfirmResponse{Code: 0, Data: &orderpb.CheckoutConfirmData{CheckoutId: "chk-1", OrderNos: []string{"ORD1"}}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"checkoutId":     "chk-1",
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"recipientName":  "王小明",
		"recipientPhone": "0912345678",
		"items":          []map[string]string{{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 8001 {
		t.Errorf("BuyerId = %+v, want 8001 (resolved buyer_id, not JWT account_id 100)", captured)
	}
}

func TestCheckoutConfirm_NoCurrentBuyer403(t *testing.T) {
	called := false
	mock := &mockOrderServiceClient{
		CheckoutConfirmFn: func(ctx context.Context, in *orderpb.CheckoutConfirmRequest, opts ...grpc.CallOption) (*orderpb.CheckoutConfirmResponse, error) {
			called = true
			return nil, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: noBuyerResolverForTest()}
	router := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"checkoutId":     "chk-1",
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"recipientName":  "王小明",
		"recipientPhone": "0912345678",
		"items":          []map[string]string{{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	if called {
		t.Error("order-service must not be called without a current buyer")
	}
}
