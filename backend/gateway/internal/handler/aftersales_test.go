package handler

import (
	"context"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aftersalespb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock AfterSalesServiceClient（買家 8 個 call site）====================
// 介面 embed：未 stub 的方法呼叫即 nil-panic（沿用 seller_rekey_test.go 慣例）。

type mockAftersalesBuyerServiceClient struct {
	aftersalespb.AfterSalesServiceClient
	CreateAfterSaleCaseFn    func(ctx context.Context, in *aftersalespb.CreateAfterSaleCaseRequest, opts ...grpc.CallOption) (*aftersalespb.CreateAfterSaleCaseResponse, error)
	GetMyAfterSaleCasesFn    func(ctx context.Context, in *aftersalespb.GetMyAfterSaleCasesRequest, opts ...grpc.CallOption) (*aftersalespb.GetMyAfterSaleCasesResponse, error)
	GetAfterSaleCaseDetailFn func(ctx context.Context, in *aftersalespb.GetAfterSaleCaseDetailRequest, opts ...grpc.CallOption) (*aftersalespb.GetAfterSaleCaseDetailResponse, error)
	SupplementEvidenceFn     func(ctx context.Context, in *aftersalespb.SupplementEvidenceRequest, opts ...grpc.CallOption) (*aftersalespb.SupplementEvidenceResponse, error)
	CancelAfterSaleCaseFn    func(ctx context.Context, in *aftersalespb.CancelAfterSaleCaseRequest, opts ...grpc.CallOption) (*aftersalespb.CancelAfterSaleCaseResponse, error)
	GetRefundPageUrlFn       func(ctx context.Context, in *aftersalespb.GetRefundPageUrlRequest, opts ...grpc.CallOption) (*aftersalespb.GetRefundPageUrlResponse, error)
	SubmitAppealFn           func(ctx context.Context, in *aftersalespb.SubmitAppealRequest, opts ...grpc.CallOption) (*aftersalespb.SubmitAppealResponse, error)
	GetArbitrationFn         func(ctx context.Context, in *aftersalespb.GetArbitrationRequest, opts ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error)
}

func (m *mockAftersalesBuyerServiceClient) CreateAfterSaleCase(ctx context.Context, in *aftersalespb.CreateAfterSaleCaseRequest, opts ...grpc.CallOption) (*aftersalespb.CreateAfterSaleCaseResponse, error) {
	return m.CreateAfterSaleCaseFn(ctx, in, opts...)
}
func (m *mockAftersalesBuyerServiceClient) GetMyAfterSaleCases(ctx context.Context, in *aftersalespb.GetMyAfterSaleCasesRequest, opts ...grpc.CallOption) (*aftersalespb.GetMyAfterSaleCasesResponse, error) {
	return m.GetMyAfterSaleCasesFn(ctx, in, opts...)
}
func (m *mockAftersalesBuyerServiceClient) GetAfterSaleCaseDetail(ctx context.Context, in *aftersalespb.GetAfterSaleCaseDetailRequest, opts ...grpc.CallOption) (*aftersalespb.GetAfterSaleCaseDetailResponse, error) {
	return m.GetAfterSaleCaseDetailFn(ctx, in, opts...)
}
func (m *mockAftersalesBuyerServiceClient) SupplementEvidence(ctx context.Context, in *aftersalespb.SupplementEvidenceRequest, opts ...grpc.CallOption) (*aftersalespb.SupplementEvidenceResponse, error) {
	return m.SupplementEvidenceFn(ctx, in, opts...)
}
func (m *mockAftersalesBuyerServiceClient) CancelAfterSaleCase(ctx context.Context, in *aftersalespb.CancelAfterSaleCaseRequest, opts ...grpc.CallOption) (*aftersalespb.CancelAfterSaleCaseResponse, error) {
	return m.CancelAfterSaleCaseFn(ctx, in, opts...)
}
func (m *mockAftersalesBuyerServiceClient) GetRefundPageUrl(ctx context.Context, in *aftersalespb.GetRefundPageUrlRequest, opts ...grpc.CallOption) (*aftersalespb.GetRefundPageUrlResponse, error) {
	return m.GetRefundPageUrlFn(ctx, in, opts...)
}
func (m *mockAftersalesBuyerServiceClient) SubmitAppeal(ctx context.Context, in *aftersalespb.SubmitAppealRequest, opts ...grpc.CallOption) (*aftersalespb.SubmitAppealResponse, error) {
	return m.SubmitAppealFn(ctx, in, opts...)
}
func (m *mockAftersalesBuyerServiceClient) GetArbitration(ctx context.Context, in *aftersalespb.GetArbitrationRequest, opts ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error) {
	return m.GetArbitrationFn(ctx, in, opts...)
}

// successStubMock 每個買家 call site 都回傳一組合法的成功 response（不追蹤
// BuyerId）——TestAftersalesBuyerHandlers_NoCurrentBuyer_Forbidden 用它確認
// resolver guard 真的擋在 gRPC 呼叫「之前」：若 guard 漏接，這組合法 stub 會讓
// handler 順利跑到底回 200（或因跳過 resolve 直接綁 JSON body 失敗回 400），
// 而非 403——測試用「非 403」這種乾淨的 assertion failure 曝露漏接，而非
// nil-Fn panic。
func successStubMock() *mockAftersalesBuyerServiceClient {
	return &mockAftersalesBuyerServiceClient{
		CreateAfterSaleCaseFn: func(context.Context, *aftersalespb.CreateAfterSaleCaseRequest, ...grpc.CallOption) (*aftersalespb.CreateAfterSaleCaseResponse, error) {
			return &aftersalespb.CreateAfterSaleCaseResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseSummary{CaseNo: "AS0001"}}, nil
		},
		GetMyAfterSaleCasesFn: func(context.Context, *aftersalespb.GetMyAfterSaleCasesRequest, ...grpc.CallOption) (*aftersalespb.GetMyAfterSaleCasesResponse, error) {
			return &aftersalespb.GetMyAfterSaleCasesResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseListData{Total: "0"}}, nil
		},
		GetAfterSaleCaseDetailFn: func(context.Context, *aftersalespb.GetAfterSaleCaseDetailRequest, ...grpc.CallOption) (*aftersalespb.GetAfterSaleCaseDetailResponse, error) {
			return &aftersalespb.GetAfterSaleCaseDetailResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseDetailView{CaseNo: "AS0001"}}, nil
		},
		SupplementEvidenceFn: func(context.Context, *aftersalespb.SupplementEvidenceRequest, ...grpc.CallOption) (*aftersalespb.SupplementEvidenceResponse, error) {
			return &aftersalespb.SupplementEvidenceResponse{Code: 0}, nil
		},
		CancelAfterSaleCaseFn: func(context.Context, *aftersalespb.CancelAfterSaleCaseRequest, ...grpc.CallOption) (*aftersalespb.CancelAfterSaleCaseResponse, error) {
			return &aftersalespb.CancelAfterSaleCaseResponse{Code: 0}, nil
		},
		GetRefundPageUrlFn: func(context.Context, *aftersalespb.GetRefundPageUrlRequest, ...grpc.CallOption) (*aftersalespb.GetRefundPageUrlResponse, error) {
			return &aftersalespb.GetRefundPageUrlResponse{Code: 0, Data: &aftersalespb.RefundPageUrlData{RedirectUrl: "https://pay.example/refund"}}, nil
		},
		SubmitAppealFn: func(context.Context, *aftersalespb.SubmitAppealRequest, ...grpc.CallOption) (*aftersalespb.SubmitAppealResponse, error) {
			return &aftersalespb.SubmitAppealResponse{Code: 0, Data: &aftersalespb.SubmitAppealData{AppealId: "AP0001", AppealState: "APPEALED"}}, nil
		},
		GetArbitrationFn: func(context.Context, *aftersalespb.GetArbitrationRequest, ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error) {
			return &aftersalespb.GetArbitrationResponse{Code: 0, Data: &aftersalespb.ArbitrationData{}}, nil
		},
	}
}

// ==================== router ====================

func setupAftersalesBuyerRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app", fakeJWTAuth())
	{
		app.POST("/after_sales", h.CreateAfterSaleCase)
		app.GET("/after_sales", h.GetMyAfterSaleCases)
		app.GET("/after_sales/:case_no", h.GetAfterSaleCaseDetail)
		app.POST("/after_sales/:case_no/supplement", h.SupplementEvidence)
		app.POST("/after_sales/:case_no/cancel", h.CancelAfterSaleCase)
		app.POST("/orders/:order_no/refund_page", h.GetRefundPageUrl)
		app.POST("/after_sales/:case_no/appeal", h.SubmitAppeal)
		app.GET("/after_sales/:case_no/arbitration", h.GetArbitration)
	}
	return r
}

const (
	aftersalesTestAccountID = int64(42)
	aftersalesTestBuyerID   = int64(9999)
)

func doAftersalesBuyerReq(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Account-Id", strconv.FormatInt(aftersalesTestAccountID, 10))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ==================== happy path：BuyerId 必須是 resolved buyer_id ====================

func TestCreateAfterSaleCase_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		CreateAfterSaleCaseFn: func(_ context.Context, in *aftersalespb.CreateAfterSaleCaseRequest, _ ...grpc.CallOption) (*aftersalespb.CreateAfterSaleCaseResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.CreateAfterSaleCaseResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseSummary{CaseNo: "AS0001"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "POST", "/api/v1/app/after_sales", `{"orderNo":"HK20260716000001","reason":"5001"}`)

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("CreateAfterSaleCaseRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

func TestGetMyAfterSaleCases_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		GetMyAfterSaleCasesFn: func(_ context.Context, in *aftersalespb.GetMyAfterSaleCasesRequest, _ ...grpc.CallOption) (*aftersalespb.GetMyAfterSaleCasesResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.GetMyAfterSaleCasesResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseListData{Total: "0"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "GET", "/api/v1/app/after_sales", "")

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("GetMyAfterSaleCasesRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d) — load-bearing: this query bypasses order_no entirely", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

func TestGetAfterSaleCaseDetail_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		GetAfterSaleCaseDetailFn: func(_ context.Context, in *aftersalespb.GetAfterSaleCaseDetailRequest, _ ...grpc.CallOption) (*aftersalespb.GetAfterSaleCaseDetailResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.GetAfterSaleCaseDetailResponse{Code: 0, Data: &aftersalespb.AfterSaleCaseDetailView{CaseNo: "AS0001"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "GET", "/api/v1/app/after_sales/AS0001", "")

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("GetAfterSaleCaseDetailRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

func TestSupplementEvidence_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		SupplementEvidenceFn: func(_ context.Context, in *aftersalespb.SupplementEvidenceRequest, _ ...grpc.CallOption) (*aftersalespb.SupplementEvidenceResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.SupplementEvidenceResponse{Code: 0}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "POST", "/api/v1/app/after_sales/AS0001/supplement", `{"additionalDescription":"more detail"}`)

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("SupplementEvidenceRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

func TestCancelAfterSaleCase_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		CancelAfterSaleCaseFn: func(_ context.Context, in *aftersalespb.CancelAfterSaleCaseRequest, _ ...grpc.CallOption) (*aftersalespb.CancelAfterSaleCaseResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.CancelAfterSaleCaseResponse{Code: 0}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "POST", "/api/v1/app/after_sales/AS0001/cancel", "")

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("CancelAfterSaleCaseRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

func TestGetRefundPageUrl_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		GetRefundPageUrlFn: func(_ context.Context, in *aftersalespb.GetRefundPageUrlRequest, _ ...grpc.CallOption) (*aftersalespb.GetRefundPageUrlResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.GetRefundPageUrlResponse{Code: 0, Data: &aftersalespb.RefundPageUrlData{RedirectUrl: "https://pay.example/refund"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "POST", "/api/v1/app/orders/HK20260716000001/refund_page", "")

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("GetRefundPageUrlRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

func TestSubmitAppeal_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		SubmitAppealFn: func(_ context.Context, in *aftersalespb.SubmitAppealRequest, _ ...grpc.CallOption) (*aftersalespb.SubmitAppealResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.SubmitAppealResponse{Code: 0, Data: &aftersalespb.SubmitAppealData{AppealId: "AP0001", AppealState: "APPEALED"}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "POST", "/api/v1/app/after_sales/AS0001/appeal", `{"reason":"disagree with rejection"}`)

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("SubmitAppealRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

func TestGetArbitration_UsesResolvedBuyerID(t *testing.T) {
	var gotBuyerID int64
	mock := &mockAftersalesBuyerServiceClient{
		GetArbitrationFn: func(_ context.Context, in *aftersalespb.GetArbitrationRequest, _ ...grpc.CallOption) (*aftersalespb.GetArbitrationResponse, error) {
			gotBuyerID = in.BuyerId
			return &aftersalespb.GetArbitrationResponse{Code: 0, Data: &aftersalespb.ArbitrationData{}}, nil
		},
	}
	h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock), BuyerID: buyerResolverForTest(aftersalesTestBuyerID)}
	w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), "GET", "/api/v1/app/after_sales/AS0001/arbitration", "")

	assertHTTPStatus(t, w, 200)
	if gotBuyerID != aftersalesTestBuyerID {
		t.Errorf("GetArbitrationRequest.BuyerId = %d, want resolved buyer_id %d (not raw account_id %d)", gotBuyerID, aftersalesTestBuyerID, aftersalesTestAccountID)
	}
}

// ==================== no current buyer -> 403（全部 8 個 route）====================

func TestAftersalesBuyerHandlers_NoCurrentBuyer_Forbidden(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"CreateAfterSaleCase", "POST", "/api/v1/app/after_sales"},
		{"GetMyAfterSaleCases", "GET", "/api/v1/app/after_sales"},
		{"GetAfterSaleCaseDetail", "GET", "/api/v1/app/after_sales/AS0001"},
		{"SupplementEvidence", "POST", "/api/v1/app/after_sales/AS0001/supplement"},
		{"CancelAfterSaleCase", "POST", "/api/v1/app/after_sales/AS0001/cancel"},
		{"GetRefundPageUrl", "POST", "/api/v1/app/orders/HK20260716000001/refund_page"},
		{"SubmitAppeal", "POST", "/api/v1/app/after_sales/AS0001/appeal"},
		{"GetArbitration", "GET", "/api/v1/app/after_sales/AS0001/arbitration"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Holder{
				Aftersales: grpcclient.NewAftersalesClientForTest(successStubMock()),
				BuyerID:    noBuyerResolverForTest(),
			}
			w := doAftersalesBuyerReq(t, setupAftersalesBuyerRouter(h), tc.method, tc.path, "")

			assertHTTPStatus(t, w, 403)
			body := parseJSON(t, w.Body)
			assertCode(t, body, 403)
			if body["message"] != "buyer access required" {
				t.Errorf("message = %v, want %q", body["message"], "buyer access required")
			}
		})
	}
}
