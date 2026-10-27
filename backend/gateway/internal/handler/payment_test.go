package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock PaymentServiceClient ====================

type mockPaymentServiceClient struct {
	pb.PaymentServiceClient
	CreatePaymentFn           func(ctx context.Context, in *pb.CreatePaymentRequest, opts ...grpc.CallOption) (*pb.CreatePaymentResponse, error)
	GetPaymentFn              func(ctx context.Context, in *pb.GetPaymentRequest, opts ...grpc.CallOption) (*pb.GetPaymentResponse, error)
	SyncPaymentStatusFn       func(ctx context.Context, in *pb.SyncPaymentStatusRequest, opts ...grpc.CallOption) (*pb.SyncPaymentStatusResponse, error)
	ClosePaymentFn            func(ctx context.Context, in *pb.ClosePaymentRequest, opts ...grpc.CallOption) (*pb.ClosePaymentResponse, error)
	GetRefundFn               func(ctx context.Context, in *pb.GetRefundRequest, opts ...grpc.CallOption) (*pb.GetRefundResponse, error)
	GetRefundsByTradeNoFn     func(ctx context.Context, in *pb.GetRefundsByTradeNoRequest, opts ...grpc.CallOption) (*pb.GetRefundsByTradeNoResponse, error)
	GetPaymentReturnPageFn    func(ctx context.Context, in *pb.GetPaymentReturnPageRequest, opts ...grpc.CallOption) (*pb.GetPaymentReturnPageResponse, error)
	HandlePaymentPageReturnFn func(ctx context.Context, in *pb.HandlePaymentPageReturnRequest, opts ...grpc.CallOption) (*pb.HandlePaymentPageReturnResponse, error)
	GetPublicPaymentStatusFn  func(ctx context.Context, in *pb.GetPublicPaymentStatusRequest, opts ...grpc.CallOption) (*pb.GetPublicPaymentStatusResponse, error)
	GetPaymentHistoryFn       func(ctx context.Context, in *pb.GetPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetPaymentHistoryResponse, error)
	GetSellerPaymentHistoryFn func(ctx context.Context, in *pb.GetSellerPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetSellerPaymentHistoryResponse, error)
}

func (m *mockPaymentServiceClient) CreatePayment(ctx context.Context, in *pb.CreatePaymentRequest, opts ...grpc.CallOption) (*pb.CreatePaymentResponse, error) {
	if m.CreatePaymentFn != nil {
		return m.CreatePaymentFn(ctx, in, opts...)
	}
	return &pb.CreatePaymentResponse{}, nil
}
func (m *mockPaymentServiceClient) GetPayment(ctx context.Context, in *pb.GetPaymentRequest, opts ...grpc.CallOption) (*pb.GetPaymentResponse, error) {
	if m.GetPaymentFn != nil {
		return m.GetPaymentFn(ctx, in, opts...)
	}
	return &pb.GetPaymentResponse{}, nil
}
func (m *mockPaymentServiceClient) SyncPaymentStatus(ctx context.Context, in *pb.SyncPaymentStatusRequest, opts ...grpc.CallOption) (*pb.SyncPaymentStatusResponse, error) {
	if m.SyncPaymentStatusFn != nil {
		return m.SyncPaymentStatusFn(ctx, in, opts...)
	}
	return &pb.SyncPaymentStatusResponse{}, nil
}
func (m *mockPaymentServiceClient) ClosePayment(ctx context.Context, in *pb.ClosePaymentRequest, opts ...grpc.CallOption) (*pb.ClosePaymentResponse, error) {
	if m.ClosePaymentFn != nil {
		return m.ClosePaymentFn(ctx, in, opts...)
	}
	return &pb.ClosePaymentResponse{}, nil
}
func (m *mockPaymentServiceClient) GetRefund(ctx context.Context, in *pb.GetRefundRequest, opts ...grpc.CallOption) (*pb.GetRefundResponse, error) {
	if m.GetRefundFn != nil {
		return m.GetRefundFn(ctx, in, opts...)
	}
	return &pb.GetRefundResponse{}, nil
}
func (m *mockPaymentServiceClient) GetRefundsByTradeNo(ctx context.Context, in *pb.GetRefundsByTradeNoRequest, opts ...grpc.CallOption) (*pb.GetRefundsByTradeNoResponse, error) {
	if m.GetRefundsByTradeNoFn != nil {
		return m.GetRefundsByTradeNoFn(ctx, in, opts...)
	}
	return &pb.GetRefundsByTradeNoResponse{}, nil
}
func (m *mockPaymentServiceClient) GetPaymentReturnPage(ctx context.Context, in *pb.GetPaymentReturnPageRequest, opts ...grpc.CallOption) (*pb.GetPaymentReturnPageResponse, error) {
	if m.GetPaymentReturnPageFn != nil {
		return m.GetPaymentReturnPageFn(ctx, in, opts...)
	}
	return &pb.GetPaymentReturnPageResponse{}, nil
}
func (m *mockPaymentServiceClient) HandlePaymentPageReturn(ctx context.Context, in *pb.HandlePaymentPageReturnRequest, opts ...grpc.CallOption) (*pb.HandlePaymentPageReturnResponse, error) {
	if m.HandlePaymentPageReturnFn != nil {
		return m.HandlePaymentPageReturnFn(ctx, in, opts...)
	}
	return &pb.HandlePaymentPageReturnResponse{}, nil
}
func (m *mockPaymentServiceClient) GetPublicPaymentStatus(ctx context.Context, in *pb.GetPublicPaymentStatusRequest, opts ...grpc.CallOption) (*pb.GetPublicPaymentStatusResponse, error) {
	if m.GetPublicPaymentStatusFn != nil {
		return m.GetPublicPaymentStatusFn(ctx, in, opts...)
	}
	return &pb.GetPublicPaymentStatusResponse{}, nil
}
func (m *mockPaymentServiceClient) GetPaymentHistory(ctx context.Context, in *pb.GetPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetPaymentHistoryResponse, error) {
	if m.GetPaymentHistoryFn != nil {
		return m.GetPaymentHistoryFn(ctx, in, opts...)
	}
	return &pb.GetPaymentHistoryResponse{}, nil
}
func (m *mockPaymentServiceClient) GetSellerPaymentHistory(ctx context.Context, in *pb.GetSellerPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetSellerPaymentHistoryResponse, error) {
	if m.GetSellerPaymentHistoryFn != nil {
		return m.GetSellerPaymentHistoryFn(ctx, in, opts...)
	}
	return &pb.GetSellerPaymentHistoryResponse{}, nil
}
func (m *mockPaymentServiceClient) ValidateApplePayMerchant(ctx context.Context, in *pb.ValidateApplePayMerchantRequest, opts ...grpc.CallOption) (*pb.ValidateApplePayMerchantResponse, error) {
	return &pb.ValidateApplePayMerchantResponse{}, nil
}

// ==================== Router Setup ====================

func setupPaymentRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	app := v1.Group("/app")
	app.Use(fakeJWTAuth())
	{
		app.POST("/payments", h.CreatePayment)
		app.GET("/payments/:trade_no", h.GetPayment)
		app.POST("/payments/:trade_no/status_syncs", h.SyncPaymentStatus)
		app.PATCH("/payments/:trade_no", h.ClosePayment)
		app.GET("/payments/refunds/:refund_no", h.GetRefund)
		app.GET("/payments/:trade_no/refunds", h.GetRefundsByTradeNo)
	}
	return r
}

// ==================== CreatePayment ====================

func TestCreatePayment_Success(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		CreatePaymentFn: func(ctx context.Context, in *pb.CreatePaymentRequest, opts ...grpc.CallOption) (*pb.CreatePaymentResponse, error) {
			if in.OrderNo != "ORD20260328001" {
				t.Errorf("OrderNo = %q, want ORD20260328001", in.OrderNo)
			}
			if in.PaymentType != "CREDIT_CARD" {
				t.Errorf("PaymentType = %q, want CREDIT_CARD", in.PaymentType)
			}
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &pb.CreatePaymentResponse{
				Code: 0,
				Data: &pb.PaymentResult{
					TradeNo:     "TRD001",
					OrderId:     "ORD20260328001",
					PaymentType: "CREDIT_CARD",
					Amount:      "1500",
					Status:      "0",
				},
			}, nil
		},
	}
	orderMock := &mockOrderServiceClient{
		SnapshotPaymentInfoFn: func(ctx context.Context, in *orderpb.SnapshotPaymentInfoRequest, opts ...grpc.CallOption) (*orderpb.SnapshotPaymentInfoResponse, error) {
			return &orderpb.SnapshotPaymentInfoResponse{}, nil
		},
	}

	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		Order:   grpcclient.NewOrderClientForTest(orderMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"orderNo":     "ORD20260328001",
		"paymentType": "CREDIT_CARD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/payments", bytes.NewReader(body))
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
	if data["tradeNo"] != "TRD001" {
		t.Errorf("tradeNo = %v, want TRD001", data["tradeNo"])
	}
	if data["amount"] != "1500" {
		t.Errorf("amount = %v, want 1500", data["amount"])
	}
}

func TestCreatePayment_BadRequest_MissingOrderNo(t *testing.T) {
	h := &Holder{}
	router := setupPaymentRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"paymentType": "CREDIT_CARD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

func TestCreatePayment_GrpcError(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		CreatePaymentFn: func(ctx context.Context, in *pb.CreatePaymentRequest, opts ...grpc.CallOption) (*pb.CreatePaymentResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"orderNo":     "ORD001",
		"paymentType": "CREDIT_CARD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== GetPayment ====================

func TestGetPayment_Success(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		GetPaymentFn: func(ctx context.Context, in *pb.GetPaymentRequest, opts ...grpc.CallOption) (*pb.GetPaymentResponse, error) {
			if in.TradeNo != "TRD001" {
				t.Errorf("TradeNo = %q, want TRD001", in.TradeNo)
			}
			return &pb.GetPaymentResponse{
				Code: 0,
				Data: &pb.PaymentDetail{
					TradeNo:     "TRD001",
					OrderId:     "ORD001",
					PaymentType: "ATM",
					Amount:      "2000",
					Status:      "1",
				},
			}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/TRD001", nil)
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
	if data["tradeNo"] != "TRD001" {
		t.Errorf("tradeNo = %v, want TRD001", data["tradeNo"])
	}
	if data["paymentType"] != "ATM" {
		t.Errorf("paymentType = %v, want ATM", data["paymentType"])
	}
	if data["status"] != "1" {
		t.Errorf("status = %v, want 1", data["status"])
	}
}

func TestGetPayment_GrpcError(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		GetPaymentFn: func(ctx context.Context, in *pb.GetPaymentRequest, opts ...grpc.CallOption) (*pb.GetPaymentResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/TRD001", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== SyncPaymentStatus ====================

func TestSyncPaymentStatus_Success(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		SyncPaymentStatusFn: func(ctx context.Context, in *pb.SyncPaymentStatusRequest, opts ...grpc.CallOption) (*pb.SyncPaymentStatusResponse, error) {
			if in.TradeNo != "TRD001" {
				t.Errorf("TradeNo = %q, want TRD001", in.TradeNo)
			}
			return &pb.SyncPaymentStatusResponse{
				Code: 0,
				Data: &pb.PaymentDetail{
					TradeNo: "TRD001",
					OrderId: "ORD001",
					Status:  "1",
				},
			}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/payments/TRD001/status_syncs", nil)
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
	if data["tradeNo"] != "TRD001" {
		t.Errorf("tradeNo = %v, want TRD001", data["tradeNo"])
	}
}

// ==================== ClosePayment ====================

func TestClosePayment_Success(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		ClosePaymentFn: func(ctx context.Context, in *pb.ClosePaymentRequest, opts ...grpc.CallOption) (*pb.ClosePaymentResponse, error) {
			if in.TradeNo != "TRD001" {
				t.Errorf("TradeNo = %q, want TRD001", in.TradeNo)
			}
			return &pb.ClosePaymentResponse{
				Code:    0,
				Message: "closed",
			}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/app/payments/TRD001", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

// ==================== GetRefund ====================

func TestGetRefund_Success(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		GetRefundFn: func(ctx context.Context, in *pb.GetRefundRequest, opts ...grpc.CallOption) (*pb.GetRefundResponse, error) {
			if in.RefundNo != "RFD001" {
				t.Errorf("RefundNo = %q, want RFD001", in.RefundNo)
			}
			return &pb.GetRefundResponse{
				Code: 0,
				Data: &pb.RefundDetail{
					RefundNo: "RFD001",
					TradeNo:  "TRD001",
					Amount:   "500",
					Status:   "0",
				},
			}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/refunds/RFD001", nil)
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
	if data["refundNo"] != "RFD001" {
		t.Errorf("refundNo = %v, want RFD001", data["refundNo"])
	}
}

// ==================== GetRefundsByTradeNo ====================

func TestGetRefundsByTradeNo_Success(t *testing.T) {
	paymentMock := &mockPaymentServiceClient{
		GetRefundsByTradeNoFn: func(ctx context.Context, in *pb.GetRefundsByTradeNoRequest, opts ...grpc.CallOption) (*pb.GetRefundsByTradeNoResponse, error) {
			if in.TradeNo != "TRD001" {
				t.Errorf("TradeNo = %q, want TRD001", in.TradeNo)
			}
			return &pb.GetRefundsByTradeNoResponse{
				Code: 0,
				Refunds: []*pb.RefundDetail{
					{RefundNo: "RFD001", TradeNo: "TRD001", Amount: "300", Status: "0"},
					{RefundNo: "RFD002", TradeNo: "TRD001", Amount: "200", Status: "1"},
				},
			}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupPaymentRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/TRD001/refunds", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) != 2 {
		t.Errorf("refund count = %d, want 2", len(data))
	}
}

func TestCreatePayment_ResolvesDivergedBuyerID(t *testing.T) {
	const rawAccountID = 100
	const resolvedBuyerID = 8001

	paymentMock := &mockPaymentServiceClient{
		CreatePaymentFn: func(ctx context.Context, in *pb.CreatePaymentRequest, opts ...grpc.CallOption) (*pb.CreatePaymentResponse, error) {
			if in.BuyerId != resolvedBuyerID {
				t.Errorf("BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", in.BuyerId, resolvedBuyerID, rawAccountID)
			}
			return &pb.CreatePaymentResponse{
				Code: 0,
				Data: &pb.PaymentResult{TradeNo: "TRD002", OrderId: "ORD002"},
			}, nil
		},
	}
	orderMock := &mockOrderServiceClient{
		SnapshotPaymentInfoFn: func(ctx context.Context, in *orderpb.SnapshotPaymentInfoRequest, opts ...grpc.CallOption) (*orderpb.SnapshotPaymentInfoResponse, error) {
			return &orderpb.SnapshotPaymentInfoResponse{}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		Order:   grpcclient.NewOrderClientForTest(orderMock),
		BuyerID: buyerResolverForTest(resolvedBuyerID),
	}

	router := setupPaymentRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"orderNo":     "ORD002",
		"paymentType": "CREDIT_CARD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

// ==================== GetPayment / GetPaymentHistory buyer_id resolution ====================
//
// fakeJWTAuth() only does c.Set("accountId", ...) — it never attaches the Go
// context value that AuthMetadataInterceptor forwards as x-account-id metadata
// (that's applyIdentity's job in the real auth.go middleware, via
// grpcclient.WithAuthContext + c.Request.WithContext). Without replicating that
// attachment here, whether GetPayment/GetPaymentHistory actually resolve the
// buyer_id before forwarding can't be observed at all in this test harness.
func setupPaymentRouterWithGRPCAuthContext(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	app := v1.Group("/app")
	app.Use(fakeJWTAuth())
	app.Use(func(c *gin.Context) {
		if raw, ok := c.Get("accountId"); ok {
			if id, ok := raw.(int64); ok {
				ctx := grpcclient.WithAuthContext(c.Request.Context(), id, "1")
				c.Request = c.Request.WithContext(ctx)
			}
		}
		c.Next()
	})
	{
		app.GET("/payments/:trade_no", h.GetPayment)
		app.GET("/payments/orders/:order_id/history", h.GetPaymentHistory)
		app.POST("/payments/:trade_no/status_syncs", h.SyncPaymentStatus)
		app.PATCH("/payments/:trade_no", h.ClosePayment)
		app.GET("/payments/refunds/:refund_no", h.GetRefund)
		app.GET("/payments/:trade_no/refunds", h.GetRefundsByTradeNo)
	}
	return r
}

func TestGetPayment_ResolvesDivergedBuyerID(t *testing.T) {
	const rawAccountID = 100
	const resolvedBuyerID = 8001
	var capturedAccountID interface{} = "GetPaymentFn NOT CALLED"

	paymentMock := &mockPaymentServiceClient{
		GetPaymentFn: func(ctx context.Context, in *pb.GetPaymentRequest, opts ...grpc.CallOption) (*pb.GetPaymentResponse, error) {
			capturedAccountID = ctx.Value(grpcclient.CtxKeyAccountID)
			return &pb.GetPaymentResponse{
				Code: 0,
				Data: &pb.PaymentDetail{TradeNo: in.TradeNo, OrderId: "ORD001"},
			}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerResolverForTest(resolvedBuyerID),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/TRD001", nil)
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if capturedAccountID != int64(resolvedBuyerID) {
		t.Errorf("outgoing gRPC account context = %v, want resolved buyer_id %d (not raw account_id %d) — "+
			"must resolve via resolvePaymentBuyerID like CreatePayment does. A buyer whose buyer_id has "+
			"diverged from account_id (re-registered as buyer at least once) would get a false 404 on "+
			"their own payment otherwise, because payment-service's verifyPaymentOwnership compares this "+
			"value against the stored resolved buyer_id.",
			capturedAccountID, resolvedBuyerID, rawAccountID)
	}
}

func TestGetPaymentHistory_ResolvesDivergedBuyerID(t *testing.T) {
	const rawAccountID = 100
	const resolvedBuyerID = 8001
	var capturedAccountID interface{} = "GetPaymentHistoryFn NOT CALLED"

	paymentMock := &mockPaymentServiceClient{
		GetPaymentHistoryFn: func(ctx context.Context, in *pb.GetPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetPaymentHistoryResponse, error) {
			capturedAccountID = ctx.Value(grpcclient.CtxKeyAccountID)
			return &pb.GetPaymentHistoryResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerResolverForTest(resolvedBuyerID),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/orders/ORD001/history", nil)
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if capturedAccountID != int64(resolvedBuyerID) {
		t.Errorf("outgoing gRPC account context = %v, want resolved buyer_id %d (not raw account_id %d) — "+
			"GetPaymentHistory must resolve buyer_id the same way GetPayment does.",
			capturedAccountID, resolvedBuyerID, rawAccountID)
	}
}

func TestSyncPaymentStatus_ResolvesDivergedBuyerID(t *testing.T) {
	const rawAccountID = 100
	const resolvedBuyerID = 8001
	var capturedAccountID interface{} = "SyncPaymentStatusFn NOT CALLED"

	paymentMock := &mockPaymentServiceClient{
		SyncPaymentStatusFn: func(ctx context.Context, in *pb.SyncPaymentStatusRequest, opts ...grpc.CallOption) (*pb.SyncPaymentStatusResponse, error) {
			capturedAccountID = ctx.Value(grpcclient.CtxKeyAccountID)
			return &pb.SyncPaymentStatusResponse{Code: 0, Data: &pb.PaymentDetail{TradeNo: in.TradeNo}}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerResolverForTest(resolvedBuyerID),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/payments/TRD001/status_syncs", nil)
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if capturedAccountID != int64(resolvedBuyerID) {
		t.Errorf("outgoing gRPC account context = %v, want resolved buyer_id %d (not raw account_id %d) — "+
			"SyncPaymentStatus must resolve buyer_id like GetPayment does, otherwise payment-service's "+
			"verifyPaymentOwnership compares raw account_id against the resolved buyer_id CreatePayment "+
			"stored and a diverged buyer gets a false 404.",
			capturedAccountID, resolvedBuyerID, rawAccountID)
	}
}

func TestClosePayment_ResolvesDivergedBuyerID(t *testing.T) {
	const rawAccountID = 100
	const resolvedBuyerID = 8001
	var capturedAccountID interface{} = "ClosePaymentFn NOT CALLED"

	paymentMock := &mockPaymentServiceClient{
		ClosePaymentFn: func(ctx context.Context, in *pb.ClosePaymentRequest, opts ...grpc.CallOption) (*pb.ClosePaymentResponse, error) {
			capturedAccountID = ctx.Value(grpcclient.CtxKeyAccountID)
			return &pb.ClosePaymentResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerResolverForTest(resolvedBuyerID),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/app/payments/TRD001", nil)
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if capturedAccountID != int64(resolvedBuyerID) {
		t.Errorf("outgoing gRPC account context = %v, want resolved buyer_id %d (not raw account_id %d) — "+
			"ClosePayment must resolve buyer_id like GetPayment does.",
			capturedAccountID, resolvedBuyerID, rawAccountID)
	}
}

func TestGetRefund_ResolvesDivergedBuyerID(t *testing.T) {
	const rawAccountID = 100
	const resolvedBuyerID = 8001
	var capturedAccountID interface{} = "GetRefundFn NOT CALLED"

	paymentMock := &mockPaymentServiceClient{
		GetRefundFn: func(ctx context.Context, in *pb.GetRefundRequest, opts ...grpc.CallOption) (*pb.GetRefundResponse, error) {
			capturedAccountID = ctx.Value(grpcclient.CtxKeyAccountID)
			return &pb.GetRefundResponse{Code: 0, Data: &pb.RefundDetail{RefundNo: in.RefundNo}}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerResolverForTest(resolvedBuyerID),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/refunds/RFD001", nil)
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if capturedAccountID != int64(resolvedBuyerID) {
		t.Errorf("outgoing gRPC account context = %v, want resolved buyer_id %d (not raw account_id %d) — "+
			"GetRefund must resolve buyer_id like GetPayment does.",
			capturedAccountID, resolvedBuyerID, rawAccountID)
	}
}

func TestGetRefundsByTradeNo_ResolvesDivergedBuyerID(t *testing.T) {
	const rawAccountID = 100
	const resolvedBuyerID = 8001
	var capturedAccountID interface{} = "GetRefundsByTradeNoFn NOT CALLED"

	paymentMock := &mockPaymentServiceClient{
		GetRefundsByTradeNoFn: func(ctx context.Context, in *pb.GetRefundsByTradeNoRequest, opts ...grpc.CallOption) (*pb.GetRefundsByTradeNoResponse, error) {
			capturedAccountID = ctx.Value(grpcclient.CtxKeyAccountID)
			return &pb.GetRefundsByTradeNoResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		BuyerID: buyerResolverForTest(resolvedBuyerID),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/TRD001/refunds", nil)
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if capturedAccountID != int64(resolvedBuyerID) {
		t.Errorf("outgoing gRPC account context = %v, want resolved buyer_id %d (not raw account_id %d) — "+
			"GetRefundsByTradeNo must resolve buyer_id like GetPayment does.",
			capturedAccountID, resolvedBuyerID, rawAccountID)
	}
}

// ==================== #2501 GetPaymentHistory seller view (store-scoped) ====================

// 賣家視角（?view=seller）必須走 store-scoped 的 GetSellerPaymentHistory，帶 server 解出的
// store_id，且不得落入買家 GetPaymentHistory 路徑。
func TestGetPaymentHistory_SellerView_UsesStoreScopedRPC(t *testing.T) {
	const rawAccountID = 100
	const resolvedStoreID = 7001
	var capturedStoreID int64 = -1
	var capturedOrderID string

	paymentMock := &mockPaymentServiceClient{
		GetSellerPaymentHistoryFn: func(ctx context.Context, in *pb.GetSellerPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetSellerPaymentHistoryResponse, error) {
			capturedStoreID = in.StoreId
			capturedOrderID = in.OrderId
			return &pb.GetSellerPaymentHistoryResponse{
				Code:    0,
				Records: []*pb.PaymentDetail{{TradeNo: "ORD001001", OrderId: in.OrderId}},
			}, nil
		},
		GetPaymentHistoryFn: func(ctx context.Context, in *pb.GetPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetPaymentHistoryResponse, error) {
			t.Fatalf("buyer GetPaymentHistory must NOT be called for view=seller")
			return nil, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		Store:   storeResolverForTest(resolvedStoreID),
		BuyerID: buyerResolverForTest(8001),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/orders/ORD001/history?view=seller", nil)
	req.Header.Set("X-Account-Id", strconv.Itoa(rawAccountID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if capturedStoreID != int64(resolvedStoreID) {
		t.Errorf("GetSellerPaymentHistory store_id = %d, want server-resolved store_id %d "+
			"(must NOT trust a client-supplied store)", capturedStoreID, resolvedStoreID)
	}
	if capturedOrderID != "ORD001" {
		t.Errorf("GetSellerPaymentHistory order_id = %q, want ORD001", capturedOrderID)
	}
}

// 賣家視角但帳號無現任 store → resolveStoreID fail-closed，不得呼叫任何付款 RPC。
func TestGetPaymentHistory_SellerView_NoStore_FailsClosed(t *testing.T) {
	sellerCalled := false
	paymentMock := &mockPaymentServiceClient{
		GetSellerPaymentHistoryFn: func(ctx context.Context, in *pb.GetSellerPaymentHistoryRequest, opts ...grpc.CallOption) (*pb.GetSellerPaymentHistoryResponse, error) {
			sellerCalled = true
			return &pb.GetSellerPaymentHistoryResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Payment: grpcclient.NewPaymentClientForTest(paymentMock),
		Store:   noStoreResolverForTest(),
	}

	router := setupPaymentRouterWithGRPCAuthContext(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/payments/orders/ORD001/history?view=seller", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("status = 200, want non-200 (fail-closed when account has no current store)")
	}
	if sellerCalled {
		t.Errorf("GetSellerPaymentHistory must NOT be called when store cannot be resolved")
	}
}
