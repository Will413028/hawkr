package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aftersalespb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	paymentpb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Adversarial-review sweep #2: more admin filter
// call sites with the same unresolved-raw-login-id bug class as
// SearchOrders/ListAbnormalOrdersForAdmin (order.go / admin_order.go), just
// on aftersales / payment instead of order-service —
// which is why the first D7 sweep missed them. All 4 now resolve through
// resolveOptionalBuyerFilterQuery / resolveOptionalSellerFilterQuery instead
// of forwarding c.Query(...) raw. ====================

// ---- mocks (admin-only RPCs; embed full interface so any other method call
// nil-panics — same "must not be silently invoked" contract as sibling mocks
// in aftersales_test.go / payment_test.go) ----

type mockAftersalesAdminServiceClient struct {
	aftersalespb.AfterSalesServiceClient
	ListAfterSaleCasesForAdminFn func(ctx context.Context, in *aftersalespb.ListAfterSaleCasesForAdminRequest, opts ...grpc.CallOption) (*aftersalespb.ListAfterSaleCasesForAdminResponse, error)
}

func (m *mockAftersalesAdminServiceClient) ListAfterSaleCasesForAdmin(ctx context.Context, in *aftersalespb.ListAfterSaleCasesForAdminRequest, opts ...grpc.CallOption) (*aftersalespb.ListAfterSaleCasesForAdminResponse, error) {
	return m.ListAfterSaleCasesForAdminFn(ctx, in, opts...)
}

type mockPaymentAdminServiceClient struct {
	paymentpb.PaymentServiceClient
	ListPaymentTransactionsForAdminFn func(ctx context.Context, in *paymentpb.ListPaymentTransactionsForAdminRequest, opts ...grpc.CallOption) (*paymentpb.ListPaymentTransactionsForAdminResponse, error)
}

func (m *mockPaymentAdminServiceClient) ListPaymentTransactionsForAdmin(ctx context.Context, in *paymentpb.ListPaymentTransactionsForAdminRequest, opts ...grpc.CallOption) (*paymentpb.ListPaymentTransactionsForAdminResponse, error) {
	return m.ListPaymentTransactionsForAdminFn(ctx, in, opts...)
}

// ==================== 1) admin_aftersales.go: ListAfterSaleCasesForAdmin ====================

func setupAdminAfterSaleCasesRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.Use(fakeAdminAuth())
	r.GET("/api/v1/admin/after_sales", h.ListAfterSaleCasesForAdmin)
	return r
}

func TestListAfterSaleCasesForAdmin_BuyerFilter_ResolvesTarget(t *testing.T) {
	var captured *aftersalespb.ListAfterSaleCasesForAdminRequest
	mock := &mockAftersalesAdminServiceClient{
		ListAfterSaleCasesForAdminFn: func(ctx context.Context, in *aftersalespb.ListAfterSaleCasesForAdminRequest, opts ...grpc.CallOption) (*aftersalespb.ListAfterSaleCasesForAdminResponse, error) {
			captured = in
			return &aftersalespb.ListAfterSaleCasesForAdminResponse{Code: 0, Data: &aftersalespb.ListAfterSaleCasesForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminAfterSaleCasesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/after_sales?buyer_id=42", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "8001" {
		t.Errorf("BuyerId = %+v, want \"8001\" (resolved surrogate, not raw 42)", captured)
	}
}

func TestListAfterSaleCasesForAdmin_SellerFilter_ResolvesTarget(t *testing.T) {
	var captured *aftersalespb.ListAfterSaleCasesForAdminRequest
	mock := &mockAftersalesAdminServiceClient{
		ListAfterSaleCasesForAdminFn: func(ctx context.Context, in *aftersalespb.ListAfterSaleCasesForAdminRequest, opts ...grpc.CallOption) (*aftersalespb.ListAfterSaleCasesForAdminResponse, error) {
			captured = in
			return &aftersalespb.ListAfterSaleCasesForAdminResponse{Code: 0, Data: &aftersalespb.ListAfterSaleCasesForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupAdminAfterSaleCasesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/after_sales?store_id=55", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != "7001" {
		t.Errorf("StoreId = %+v, want \"7001\" (resolved surrogate, not raw 55)", captured)
	}
}

func TestListAfterSaleCasesForAdmin_NoFilters_PassesThroughEmpty(t *testing.T) {
	var captured *aftersalespb.ListAfterSaleCasesForAdminRequest
	mock := &mockAftersalesAdminServiceClient{
		ListAfterSaleCasesForAdminFn: func(ctx context.Context, in *aftersalespb.ListAfterSaleCasesForAdminRequest, opts ...grpc.CallOption) (*aftersalespb.ListAfterSaleCasesForAdminResponse, error) {
			captured = in
			return &aftersalespb.ListAfterSaleCasesForAdminResponse{Code: 0, Data: &aftersalespb.ListAfterSaleCasesForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(8001), Store: storeResolverForTest(7001)}
	r := setupAdminAfterSaleCasesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/after_sales", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "" || captured.StoreId != "" {
		t.Errorf("BuyerId/StoreId = %+v, want both empty (no filter supplied)", captured)
	}
}

func TestListAfterSaleCasesForAdmin_InvalidBuyerFilter_BadRequest(t *testing.T) {
	mock := &mockAftersalesAdminServiceClient{}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminAfterSaleCasesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/after_sales?buyer_id=not-a-number", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

// ==================== 2) admin_payment.go: ListPaymentTransactionsForAdmin ====================

func setupAdminPaymentTransactionsRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/admin/payment_transactions", h.ListPaymentTransactionsForAdmin)
	return r
}

func TestListPaymentTransactionsForAdmin_BuyerFilter_ResolvesTarget(t *testing.T) {
	var captured *paymentpb.ListPaymentTransactionsForAdminRequest
	mock := &mockPaymentAdminServiceClient{
		ListPaymentTransactionsForAdminFn: func(ctx context.Context, in *paymentpb.ListPaymentTransactionsForAdminRequest, opts ...grpc.CallOption) (*paymentpb.ListPaymentTransactionsForAdminResponse, error) {
			captured = in
			return &paymentpb.ListPaymentTransactionsForAdminResponse{Code: 0, Data: &paymentpb.ListPaymentTransactionsForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Payment: grpcclient.NewPaymentClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminPaymentTransactionsRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/payment_transactions?buyer_id=42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "8001" {
		t.Errorf("BuyerId = %+v, want \"8001\" (resolved surrogate, not raw 42)", captured)
	}
}

func TestListPaymentTransactionsForAdmin_NoBuyerFilter_PassesThroughEmpty(t *testing.T) {
	var captured *paymentpb.ListPaymentTransactionsForAdminRequest
	mock := &mockPaymentAdminServiceClient{
		ListPaymentTransactionsForAdminFn: func(ctx context.Context, in *paymentpb.ListPaymentTransactionsForAdminRequest, opts ...grpc.CallOption) (*paymentpb.ListPaymentTransactionsForAdminResponse, error) {
			captured = in
			return &paymentpb.ListPaymentTransactionsForAdminResponse{Code: 0, Data: &paymentpb.ListPaymentTransactionsForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Payment: grpcclient.NewPaymentClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminPaymentTransactionsRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/payment_transactions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != "" {
		t.Errorf("BuyerId = %+v, want empty (no filter supplied)", captured)
	}
}

func TestListPaymentTransactionsForAdmin_InvalidBuyerFilter_BadRequest(t *testing.T) {
	mock := &mockPaymentAdminServiceClient{}
	h := &Holder{Payment: grpcclient.NewPaymentClientForTest(mock), BuyerID: buyerResolverForTest(8001)}
	r := setupAdminPaymentTransactionsRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/payment_transactions?buyer_id=not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}
