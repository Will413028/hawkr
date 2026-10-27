package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aftersalespb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	paymentpb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	shippingpb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Adversarial-review sweep #2: 4 more admin filter
// call sites with the same unresolved-raw-login-id bug class as
// SearchOrders/ListAbnormalOrdersForAdmin (order.go / admin_order.go), just
// on aftersales / payment / shipping-service instead of order-service —
// which is why the first D7 sweep missed them. All 4 now resolve through
// resolveOptionalBuyerFilterQuery / resolveOptionalSellerFilterQuery instead
// of forwarding c.Query(...) raw. ====================

// ---- mocks (admin-only RPCs; embed full interface so any other method call
// nil-panics — same "must not be silently invoked" contract as sibling mocks
// in aftersales_test.go / payment_test.go / shipping_resolver_test.go) ----

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

type mockShippingAdminServiceClient struct {
	shippingpb.ShippingServiceClient
	ListShipmentsForAdminFn         func(ctx context.Context, in *shippingpb.ListShipmentsForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentsForAdminResponse, error)
	ListShipmentAnomaliesForAdminFn func(ctx context.Context, in *shippingpb.ListShipmentAnomaliesForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentAnomaliesForAdminResponse, error)
}

func (m *mockShippingAdminServiceClient) ListShipmentsForAdmin(ctx context.Context, in *shippingpb.ListShipmentsForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentsForAdminResponse, error) {
	return m.ListShipmentsForAdminFn(ctx, in, opts...)
}

func (m *mockShippingAdminServiceClient) ListShipmentAnomaliesForAdmin(ctx context.Context, in *shippingpb.ListShipmentAnomaliesForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentAnomaliesForAdminResponse, error) {
	return m.ListShipmentAnomaliesForAdminFn(ctx, in, opts...)
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

// ==================== 3) admin_shipping.go: ListShipmentsForAdmin ====================

func setupAdminShipmentsRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/admin/shipments", h.ListShipmentsForAdmin)
	return r
}

func TestListShipmentsForAdmin_SellerFilter_ResolvesTarget(t *testing.T) {
	var captured *shippingpb.ListShipmentsForAdminRequest
	mock := &mockShippingAdminServiceClient{
		ListShipmentsForAdminFn: func(ctx context.Context, in *shippingpb.ListShipmentsForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentsForAdminResponse, error) {
			captured = in
			return &shippingpb.ListShipmentsForAdminResponse{Code: 0, Data: &shippingpb.ListShipmentsForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupAdminShipmentsRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/shipments?store_id=55", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != "7001" {
		t.Errorf("StoreId = %+v, want \"7001\" (resolved surrogate, not raw 55)", captured)
	}
}

func TestListShipmentsForAdmin_NoSellerFilter_PassesThroughEmpty(t *testing.T) {
	var captured *shippingpb.ListShipmentsForAdminRequest
	mock := &mockShippingAdminServiceClient{
		ListShipmentsForAdminFn: func(ctx context.Context, in *shippingpb.ListShipmentsForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentsForAdminResponse, error) {
			captured = in
			return &shippingpb.ListShipmentsForAdminResponse{Code: 0, Data: &shippingpb.ListShipmentsForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupAdminShipmentsRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/shipments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != "" {
		t.Errorf("StoreId = %+v, want empty (no filter supplied)", captured)
	}
}

func TestListShipmentsForAdmin_InvalidSellerFilter_BadRequest(t *testing.T) {
	mock := &mockShippingAdminServiceClient{}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupAdminShipmentsRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/shipments?store_id=not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

// ==================== 4) shipment_anomaly.go: ListShipmentAnomaliesForAdmin ====================
//
// This call site is NOT a simple "-1" sentinel forward like the other three:
// its downstream (ShipmentAnomalyQueryMapper.selectAdminAnomalies /
// countAdminAnomalies) uses a MyBatis `<if test='sellerId != null and
// sellerId > 0'>` guard — "-1" would be treated as "no filter" there (since
// -1 is not > 0), not "match nothing", which is the OPPOSITE of every other
// call site's sentinel semantics and would silently show every seller's
// anomalies to an admin who filtered by an account with no current seller
// stint. The handler therefore short-circuits locally instead of forwarding
// "-1" downstream — see the last test below.

func setupAdminShipmentAnomaliesRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/admin/shipments/anomalies", h.ListShipmentAnomaliesForAdmin)
	return r
}

func TestListShipmentAnomaliesForAdmin_SellerFilter_ResolvesTarget(t *testing.T) {
	var captured *shippingpb.ListShipmentAnomaliesForAdminRequest
	mock := &mockShippingAdminServiceClient{
		ListShipmentAnomaliesForAdminFn: func(ctx context.Context, in *shippingpb.ListShipmentAnomaliesForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentAnomaliesForAdminResponse, error) {
			captured = in
			return &shippingpb.ListShipmentAnomaliesForAdminResponse{Code: 0, Data: &shippingpb.ListShipmentAnomaliesForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupAdminShipmentAnomaliesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/shipments/anomalies?store_id=55", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != "7001" {
		t.Errorf("StoreId = %+v, want \"7001\" (resolved surrogate, not raw 55)", captured)
	}
}

func TestListShipmentAnomaliesForAdmin_NoSellerFilter_PassesThroughEmpty(t *testing.T) {
	var captured *shippingpb.ListShipmentAnomaliesForAdminRequest
	mock := &mockShippingAdminServiceClient{
		ListShipmentAnomaliesForAdminFn: func(ctx context.Context, in *shippingpb.ListShipmentAnomaliesForAdminRequest, opts ...grpc.CallOption) (*shippingpb.ListShipmentAnomaliesForAdminResponse, error) {
			captured = in
			return &shippingpb.ListShipmentAnomaliesForAdminResponse{Code: 0, Data: &shippingpb.ListShipmentAnomaliesForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupAdminShipmentAnomaliesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/shipments/anomalies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != "" {
		t.Errorf("StoreId = %+v, want empty (no filter supplied)", captured)
	}
}

func TestListShipmentAnomaliesForAdmin_InvalidSellerFilter_BadRequest(t *testing.T) {
	mock := &mockShippingAdminServiceClient{}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupAdminShipmentAnomaliesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/shipments/anomalies?store_id=not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

// TestListShipmentAnomaliesForAdmin_NoCurrentSeller_ShortCircuitsWithoutSentinelDownstream
// is the regression test for the "-1"-vs-"sellerId > 0" mismatch: the account
// behind store_id has no current seller stint, so
// resolveOptionalSellerFilterQuery returns the "-1" sentinel. Unlike the
// other 3 call sites, this handler must NOT forward "-1" to the Java service
// (it would be silently treated as "no filter" and leak every seller's
// anomalies). ListShipmentAnomaliesForAdminFn is deliberately left nil — if
// the handler still calls through to gRPC, the mock's embedded interface
// zero value nil-panics, failing the test.
func TestListShipmentAnomaliesForAdmin_NoCurrentSeller_ShortCircuitsWithoutSentinelDownstream(t *testing.T) {
	mock := &mockShippingAdminServiceClient{}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: noStoreResolverForTest()}
	r := setupAdminShipmentAnomaliesRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/admin/shipments/anomalies?store_id=55", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["total"] != "0" {
		t.Errorf("total = %v, want \"0\" (short-circuited empty page, no current seller for filter)", data["total"])
	}
	records, ok := data["records"].([]interface{})
	if !ok || len(records) != 0 {
		t.Errorf("records = %v, want empty slice", data["records"])
	}
}
