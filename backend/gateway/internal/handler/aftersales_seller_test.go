package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aftersalespb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock AfterSalesServiceClient（賣家 6 個 call site，Plan C Phase 3）====================
// 介面 embed：未 stub 的方法呼叫即 nil-panic（沿用 aftersales_test.go 買家端慣例）。

type mockAftersalesSellerServiceClient struct {
	aftersalespb.AfterSalesServiceClient
	GetSellerAfterSaleCasesFn      func(ctx context.Context, in *aftersalespb.GetSellerAfterSaleCasesRequest, opts ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCasesResponse, error)
	GetSellerAfterSaleCaseDetailFn func(ctx context.Context, in *aftersalespb.GetSellerAfterSaleCaseDetailRequest, opts ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCaseDetailResponse, error)
	ResolveAfterSaleCaseFn         func(ctx context.Context, in *aftersalespb.ResolveAfterSaleCaseRequest, opts ...grpc.CallOption) (*aftersalespb.ResolveAfterSaleCaseResponse, error)
	ConfirmReturnReceiptFn         func(ctx context.Context, in *aftersalespb.ConfirmReturnReceiptRequest, opts ...grpc.CallOption) (*aftersalespb.ConfirmReturnReceiptResponse, error)
	CancelShipmentFn               func(ctx context.Context, in *aftersalespb.CancelShipmentRequest, opts ...grpc.CallOption) (*aftersalespb.CancelShipmentResponse, error)
	GetSellerArbitrationFn         func(ctx context.Context, in *aftersalespb.GetSellerArbitrationRequest, opts ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error)
}

func (m *mockAftersalesSellerServiceClient) GetSellerAfterSaleCases(ctx context.Context, in *aftersalespb.GetSellerAfterSaleCasesRequest, opts ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCasesResponse, error) {
	return m.GetSellerAfterSaleCasesFn(ctx, in, opts...)
}
func (m *mockAftersalesSellerServiceClient) GetSellerAfterSaleCaseDetail(ctx context.Context, in *aftersalespb.GetSellerAfterSaleCaseDetailRequest, opts ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCaseDetailResponse, error) {
	return m.GetSellerAfterSaleCaseDetailFn(ctx, in, opts...)
}
func (m *mockAftersalesSellerServiceClient) ResolveAfterSaleCase(ctx context.Context, in *aftersalespb.ResolveAfterSaleCaseRequest, opts ...grpc.CallOption) (*aftersalespb.ResolveAfterSaleCaseResponse, error) {
	return m.ResolveAfterSaleCaseFn(ctx, in, opts...)
}
func (m *mockAftersalesSellerServiceClient) ConfirmReturnReceipt(ctx context.Context, in *aftersalespb.ConfirmReturnReceiptRequest, opts ...grpc.CallOption) (*aftersalespb.ConfirmReturnReceiptResponse, error) {
	return m.ConfirmReturnReceiptFn(ctx, in, opts...)
}
func (m *mockAftersalesSellerServiceClient) CancelShipment(ctx context.Context, in *aftersalespb.CancelShipmentRequest, opts ...grpc.CallOption) (*aftersalespb.CancelShipmentResponse, error) {
	return m.CancelShipmentFn(ctx, in, opts...)
}
func (m *mockAftersalesSellerServiceClient) GetSellerArbitration(ctx context.Context, in *aftersalespb.GetSellerArbitrationRequest, opts ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error) {
	return m.GetSellerArbitrationFn(ctx, in, opts...)
}

func aftersalesSellerSuccessStubMock() *mockAftersalesSellerServiceClient {
	return &mockAftersalesSellerServiceClient{
		GetSellerAfterSaleCasesFn: func(context.Context, *aftersalespb.GetSellerAfterSaleCasesRequest, ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCasesResponse, error) {
			return &aftersalespb.GetSellerAfterSaleCasesResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseListData{Total: "0"}}, nil
		},
		GetSellerAfterSaleCaseDetailFn: func(context.Context, *aftersalespb.GetSellerAfterSaleCaseDetailRequest, ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCaseDetailResponse, error) {
			return &aftersalespb.GetSellerAfterSaleCaseDetailResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseDetailView{CaseNo: "AS0001"}}, nil
		},
		ResolveAfterSaleCaseFn: func(context.Context, *aftersalespb.ResolveAfterSaleCaseRequest, ...grpc.CallOption) (*aftersalespb.ResolveAfterSaleCaseResponse, error) {
			return &aftersalespb.ResolveAfterSaleCaseResponse{Code: 0, Data: &aftersalespb.ResolveResultData{CaseNo: "AS0001"}}, nil
		},
		ConfirmReturnReceiptFn: func(context.Context, *aftersalespb.ConfirmReturnReceiptRequest, ...grpc.CallOption) (*aftersalespb.ConfirmReturnReceiptResponse, error) {
			return &aftersalespb.ConfirmReturnReceiptResponse{Code: 0}, nil
		},
		CancelShipmentFn: func(context.Context, *aftersalespb.CancelShipmentRequest, ...grpc.CallOption) (*aftersalespb.CancelShipmentResponse, error) {
			return &aftersalespb.CancelShipmentResponse{Code: 0, Data: &aftersalespb.CancelShipmentResultData{OrderNo: "HK20260716000001"}}, nil
		},
		GetSellerArbitrationFn: func(context.Context, *aftersalespb.GetSellerArbitrationRequest, ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error) {
			return &aftersalespb.GetArbitrationResponse{Code: 0, Data: &aftersalespb.ArbitrationData{}}, nil
		},
	}
}

func setupAftersalesSellerRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app", fakeJWTAuth())
	{
		app.GET("/seller/after_sales", h.GetSellerAfterSaleCases)
		app.GET("/seller/after_sales/:case_no", h.GetSellerAfterSaleCaseDetail)
		app.POST("/seller/after_sales/:case_no/resolve", h.ResolveAfterSaleCase)
		app.POST("/seller/after_sales/:case_no/confirm_receipt", h.ConfirmReturnReceipt)
		app.POST("/orders/:order_no/seller_cancellations", h.CancelShipment)
		app.GET("/seller/after_sales/:case_no/arbitration", h.GetSellerArbitration)
	}
	return r
}

const aftersalesSellerTestSurrogate = int64(7001)

func TestGetSellerAfterSaleCases_UsesResolvedSellerID(t *testing.T) {
	var gotStoreID int64
	mock := &mockAftersalesSellerServiceClient{
		GetSellerAfterSaleCasesFn: func(_ context.Context, in *aftersalespb.GetSellerAfterSaleCasesRequest, _ ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCasesResponse, error) {
			gotStoreID = in.StoreId
			return &aftersalespb.GetSellerAfterSaleCasesResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseListData{Total: "0"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), Store: storeResolverForTest(aftersalesSellerTestSurrogate)}
	req := httptest.NewRequest("GET", "/api/v1/app/seller/after_sales", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	setupAftersalesSellerRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, 200)
	if gotStoreID != aftersalesSellerTestSurrogate {
		t.Errorf("GetSellerAfterSaleCasesRequest.StoreId = %d, want resolved surrogate %d (not raw account_id 42)", gotStoreID, aftersalesSellerTestSurrogate)
	}
}

func TestGetSellerAfterSaleCaseDetail_UsesResolvedSellerID(t *testing.T) {
	var gotStoreID int64
	mock := &mockAftersalesSellerServiceClient{
		GetSellerAfterSaleCaseDetailFn: func(_ context.Context, in *aftersalespb.GetSellerAfterSaleCaseDetailRequest, _ ...grpc.CallOption) (*aftersalespb.GetSellerAfterSaleCaseDetailResponse, error) {
			gotStoreID = in.StoreId
			return &aftersalespb.GetSellerAfterSaleCaseDetailResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseDetailView{CaseNo: "AS0001"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), Store: storeResolverForTest(aftersalesSellerTestSurrogate)}
	req := httptest.NewRequest("GET", "/api/v1/app/seller/after_sales/AS0001", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	setupAftersalesSellerRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, 200)
	if gotStoreID != aftersalesSellerTestSurrogate {
		t.Errorf("GetSellerAfterSaleCaseDetailRequest.StoreId = %d, want resolved surrogate %d (not raw account_id 42)", gotStoreID, aftersalesSellerTestSurrogate)
	}
}

func TestResolveAfterSaleCase_UsesResolvedSellerID(t *testing.T) {
	var gotStoreID int64
	mock := &mockAftersalesSellerServiceClient{
		ResolveAfterSaleCaseFn: func(_ context.Context, in *aftersalespb.ResolveAfterSaleCaseRequest, _ ...grpc.CallOption) (*aftersalespb.ResolveAfterSaleCaseResponse, error) {
			gotStoreID = in.StoreId
			return &aftersalespb.ResolveAfterSaleCaseResponse{Code: 0, Data: &aftersalespb.ResolveResultData{CaseNo: "AS0001"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), Store: storeResolverForTest(aftersalesSellerTestSurrogate)}
	req := httptest.NewRequest("POST", "/api/v1/app/seller/after_sales/AS0001/resolve", strings.NewReader(`{"resolutionType":"3"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	setupAftersalesSellerRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, 200)
	if gotStoreID != aftersalesSellerTestSurrogate {
		t.Errorf("ResolveAfterSaleCaseRequest.StoreId = %d, want resolved surrogate %d (not raw account_id 42)", gotStoreID, aftersalesSellerTestSurrogate)
	}
}

func TestConfirmReturnReceipt_UsesResolvedSellerID(t *testing.T) {
	var gotStoreID int64
	mock := &mockAftersalesSellerServiceClient{
		ConfirmReturnReceiptFn: func(_ context.Context, in *aftersalespb.ConfirmReturnReceiptRequest, _ ...grpc.CallOption) (*aftersalespb.ConfirmReturnReceiptResponse, error) {
			gotStoreID = in.StoreId
			return &aftersalespb.ConfirmReturnReceiptResponse{Code: 0}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), Store: storeResolverForTest(aftersalesSellerTestSurrogate)}
	req := httptest.NewRequest("POST", "/api/v1/app/seller/after_sales/AS0001/confirm_receipt", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	setupAftersalesSellerRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, 200)
	if gotStoreID != aftersalesSellerTestSurrogate {
		t.Errorf("ConfirmReturnReceiptRequest.StoreId = %d, want resolved surrogate %d (not raw account_id 42)", gotStoreID, aftersalesSellerTestSurrogate)
	}
}

func TestCancelShipment_UsesResolvedSellerID(t *testing.T) {
	var gotStoreID int64
	mock := &mockAftersalesSellerServiceClient{
		CancelShipmentFn: func(_ context.Context, in *aftersalespb.CancelShipmentRequest, _ ...grpc.CallOption) (*aftersalespb.CancelShipmentResponse, error) {
			gotStoreID = in.StoreId
			return &aftersalespb.CancelShipmentResponse{Code: 0, Data: &aftersalespb.CancelShipmentResultData{OrderNo: "HK20260716000001"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), Store: storeResolverForTest(aftersalesSellerTestSurrogate)}
	req := httptest.NewRequest("POST", "/api/v1/app/orders/HK20260716000001/seller_cancellations", strings.NewReader(`{"reason":"9202"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	setupAftersalesSellerRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, 200)
	if gotStoreID != aftersalesSellerTestSurrogate {
		t.Errorf("CancelShipmentRequest.StoreId = %d, want resolved surrogate %d (not raw account_id 42)", gotStoreID, aftersalesSellerTestSurrogate)
	}
}

func TestGetSellerArbitration_UsesResolvedSellerID(t *testing.T) {
	var gotStoreID int64
	mock := &mockAftersalesSellerServiceClient{
		GetSellerArbitrationFn: func(_ context.Context, in *aftersalespb.GetSellerArbitrationRequest, _ ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error) {
			gotStoreID = in.StoreId
			return &aftersalespb.GetArbitrationResponse{Code: 0, Data: &aftersalespb.ArbitrationData{}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), Store: storeResolverForTest(aftersalesSellerTestSurrogate)}
	req := httptest.NewRequest("GET", "/api/v1/app/seller/after_sales/AS0001/arbitration", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	setupAftersalesSellerRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, 200)
	if gotStoreID != aftersalesSellerTestSurrogate {
		t.Errorf("GetSellerArbitrationRequest.StoreId = %d, want resolved surrogate %d (not raw account_id 42)", gotStoreID, aftersalesSellerTestSurrogate)
	}
}

// ==================== no current seller -> 403（全部 6 個 route）====================

func TestAftersalesSellerHandlers_NoCurrentSeller_Forbidden(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"GetSellerAfterSaleCases", "GET", "/api/v1/app/seller/after_sales", ""},
		{"GetSellerAfterSaleCaseDetail", "GET", "/api/v1/app/seller/after_sales/AS0001", ""},
		{"ResolveAfterSaleCase", "POST", "/api/v1/app/seller/after_sales/AS0001/resolve", `{"resolutionType":"3"}`},
		{"ConfirmReturnReceipt", "POST", "/api/v1/app/seller/after_sales/AS0001/confirm_receipt", ""},
		{"CancelShipment", "POST", "/api/v1/app/orders/HK20260716000001/seller_cancellations", `{"reason":"9202"}`},
		{"GetSellerArbitration", "GET", "/api/v1/app/seller/after_sales/AS0001/arbitration", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Holder{
				Aftersales: grpcclient.NewAftersalesClientForTest(aftersalesSellerSuccessStubMock()),
				Store:      noStoreResolverForTest(),
			}
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("X-Account-Id", "42")
			w := httptest.NewRecorder()
			setupAftersalesSellerRouter(h).ServeHTTP(w, req)

			assertHTTPStatus(t, w, 403)
			respBody := parseJSON(t, w.Body)
			assertCode(t, respBody, 403)
			if respBody["message"] != "seller access required" {
				t.Errorf("message = %v, want %q", respBody["message"], "seller access required")
			}
		})
	}
}
