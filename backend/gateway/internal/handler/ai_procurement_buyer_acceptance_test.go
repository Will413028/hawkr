package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// acceptanceRecordingClient 只記錄 AcceptSellerQuote 有沒有被呼叫。零呼叫是這組測試
// 的重點：缺 CAS token 的請求必須在 Gateway 就被擋下，不能讓它抵達 ai-service。
type acceptanceRecordingClient struct {
	aipb.AIProcurementServiceClient

	calls int
	got   *aipb.BuyerQuoteAcceptanceRequest
	resp  *aipb.BuyerQuoteAcceptanceResponse
	err   error
}

func (c *acceptanceRecordingClient) AcceptSellerQuote(
	_ context.Context, in *aipb.BuyerQuoteAcceptanceRequest, _ ...grpc.CallOption,
) (*aipb.BuyerQuoteAcceptanceResponse, error) {
	c.calls++
	c.got = in
	return c.resp, c.err
}

func newAcceptanceTestHolder(mock *acceptanceRecordingClient) *Holder {
	return &Holder{
		AIProcurement: grpcclient.NewAIProcurementClientForTest(mock),
		BuyerID:       buyerResolverForTest(8001),
		Store:         storeResolverForTest(7001),
		MediaBaseURL:  "https://media.test",
	}
}

func setupBuyerAcceptanceRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	app.POST("/buyers/me/procurement/requests/:id/buyer-quote-acceptances",
		h.ProxyBuyerProcurementQuoteAcceptance)
	return r
}

func postAcceptance(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v1/app/buyers/me/procurement/requests/77/buyer-quote-acceptances",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	req.Header.Set("Idempotency-Key", "idem-a")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestBuyerAcceptance_RejectsMissingCASField：doc 64 §3.2 要求 acceptance 同時 CAS
// seller response 的 exact version 與 RFQ snapshot hash。三個欄位都是 CAS 欄位，
// 少任何一個都必須在邊界被擋下，而不是讓 server 拿權威值補齊後照樣成交。
func TestBuyerAcceptance_RejectsMissingCASField(t *testing.T) {
	cases := map[string]string{
		"missing rfq snapshot hash": `{"sellerResponseId":"300","expectedSellerResponseVersion":"2"}`,
		"empty rfq snapshot hash":   `{"sellerResponseId":"300","expectedSellerResponseVersion":"2","expectedRfqSnapshotHash":""}`,
		"missing seller version":    `{"sellerResponseId":"300","expectedRfqSnapshotHash":"snaphash"}`,
		"missing seller response":   `{"expectedSellerResponseVersion":"2","expectedRfqSnapshotHash":"snaphash"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			mock := &acceptanceRecordingClient{}
			w := postAcceptance(t, setupBuyerAcceptanceRouter(newAcceptanceTestHolder(mock)), body)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
			}
			if mock.calls != 0 {
				t.Fatalf("downstream AcceptSellerQuote called %d times for an invalid request", mock.calls)
			}
		})
	}
}

// TestBuyerAcceptance_CompleteRequestReachesService 是上一條的正控制：三個 CAS 欄位
// 齊全時照樣原樣透傳，不得因為加了必填驗證而改寫任何值。
func TestBuyerAcceptance_CompleteRequestReachesService(t *testing.T) {
	mock := &acceptanceRecordingClient{
		resp: &aipb.BuyerQuoteAcceptanceResponse{
			BuyerAcceptanceId: "700", SellerResponseId: "300", SellerResponseVersion: "2",
		},
	}
	body := `{"sellerResponseId":"300","expectedSellerResponseVersion":"2","expectedRfqSnapshotHash":"snaphash"}`
	w := postAcceptance(t, setupBuyerAcceptanceRouter(newAcceptanceTestHolder(mock)), body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if mock.calls != 1 {
		t.Fatalf("AcceptSellerQuote calls = %d, want 1", mock.calls)
	}
	if mock.got.GetExpectedRfqSnapshotHash() != "snaphash" ||
		mock.got.GetExpectedSellerResponseVersion() != "2" ||
		mock.got.GetSellerResponseId() != "300" ||
		mock.got.GetClientRequestId() != "idem-a" {
		t.Fatalf("downstream request mangled: %+v", mock.got)
	}
}
