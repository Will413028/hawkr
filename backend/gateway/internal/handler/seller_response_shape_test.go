package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	crmpb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// shapeCrmMock is a local CRM mock that only implements GetCustomerStats,
// used to assert the gRPC-passthrough response shape (biz-code preservation).
// Defined locally so this file does not depend on the shared mocks in
// testutil_test.go (parallel domain PRs run in the same package).
type shapeCrmMock struct {
	crmpb.CrmServiceClient
	GetCustomerStatsFn func(ctx context.Context, in *crmpb.GetCustomerStatsRequest, opts ...grpc.CallOption) (*crmpb.CustomerStatsResponse, error)
	GetCustomerListFn  func(ctx context.Context, in *crmpb.GetCustomerListRequest, opts ...grpc.CallOption) (*crmpb.CustomerListResponse, error)
}

func (m *shapeCrmMock) GetCustomerStats(ctx context.Context, in *crmpb.GetCustomerStatsRequest, opts ...grpc.CallOption) (*crmpb.CustomerStatsResponse, error) {
	return m.GetCustomerStatsFn(ctx, in, opts...)
}

func (m *shapeCrmMock) GetCustomerList(ctx context.Context, in *crmpb.GetCustomerListRequest, opts ...grpc.CallOption) (*crmpb.CustomerListResponse, error) {
	return m.GetCustomerListFn(ctx, in, opts...)
}

// TestSellerShape_GetCustomerStats_BizCodePreserved is the load-bearing test:
// a seller_crm.go gRPC passthrough that returns a NON-ZERO business code must
// keep that exact code (NOT clobber to 0), stay HTTP 200, expose "message"
// (not legacy "msg"), and carry no "data".
func TestSellerShape_GetCustomerStats_BizCodePreserved(t *testing.T) {
	crmMock := &shapeCrmMock{
		GetCustomerStatsFn: func(ctx context.Context, in *crmpb.GetCustomerStatsRequest, opts ...grpc.CallOption) (*crmpb.CustomerStatsResponse, error) {
			return &crmpb.CustomerStatsResponse{Code: 4040, Message: "customer not found"}, nil
		},
	}
	h := &Holder{Crm: grpcclient.NewCrmClientForTest(crmMock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := gin.New()
	v1 := r.Group("/api/v1/app/sellers/me")
	v1.Use(fakeSellerAuth())
	v1.GET("/customers/:buyerId", h.GetCustomerStats)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// HTTP status must stay 200 even though biz code is non-zero.
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	// Biz code preserved verbatim — must NOT be mapped to 0.
	assertCode(t, body, 4040)
	if _, ok := body["message"]; !ok {
		t.Errorf("missing 'message'; body=%v", body)
	}
	if _, ok := body["msg"]; ok {
		t.Errorf("legacy 'msg' still present; body=%v", body)
	}
}

// TestSellerShape_GetSellerInfo_Success asserts the typed-SellerResponse success
// path emits "message" (not "msg") with data.
func TestSellerShape_GetSellerInfo_Success(t *testing.T) {
	mock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{AccountId: in.AccountId, Name: "Shop"}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}

	r := gin.New()
	r.GET("/public/sellers/:id", h.GetSellerInfo)

	req, _ := http.NewRequest("GET", "/public/sellers/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	if _, ok := body["message"]; !ok {
		t.Errorf("missing 'message'; body=%v", body)
	}
	if _, ok := body["msg"]; ok {
		t.Errorf("legacy 'msg' still present; body=%v", body)
	}
	if _, ok := body["data"]; !ok {
		t.Errorf("missing 'data'; body=%v", body)
	}
}

// TestSellerShape_GetSellerInfo_InvalidID asserts the validation-error path
// emits "message" (not "msg") and HTTP 400.
func TestSellerShape_GetSellerInfo_InvalidID(t *testing.T) {
	h := &Holder{}
	r := gin.New()
	r.GET("/public/sellers/:id", h.GetSellerInfo)

	req, _ := http.NewRequest("GET", "/public/sellers/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
	if _, ok := body["message"]; !ok {
		t.Errorf("missing 'message'; body=%v", body)
	}
	if _, ok := body["msg"]; ok {
		t.Errorf("legacy 'msg' still present; body=%v", body)
	}
}
