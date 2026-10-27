package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// mockAIAssistantServiceClient 比照既有 mockAIProcurementServiceClient
// （ai_procurement_unary_test.go）的 house pattern：內嵌介面本身、只覆寫本次測試
// 用到的方法，其餘方法呼叫會 panic（測試不會呼叫到，panic 反而能在誤用時立刻曝光）。
// ⚠️ 不是 mockAIImageServiceClient（testutil_test.go）——那個 mock 沒有內嵌介面，
// 而是把 AIImageServiceClient 介面僅有的 2 支方法全部顯式實作，不存在「其餘方法
// panic」這回事，兩者是不同的 mock 慣例。
type mockAIAssistantServiceClient struct {
	aipb.AIAssistantServiceClient
	autofillResp      *aipb.SellerProductAutofillResponse
	autofillErr       error
	latestSessionResp *aipb.LatestSessionResponse
	latestSessionErr  error
	gotAutofillReq    *aipb.SellerProductAutofillRequest
}

func (m *mockAIAssistantServiceClient) SellerProductAutofill(ctx context.Context, in *aipb.SellerProductAutofillRequest, opts ...grpc.CallOption) (*aipb.SellerProductAutofillResponse, error) {
	m.gotAutofillReq = in
	return m.autofillResp, m.autofillErr
}

func (m *mockAIAssistantServiceClient) SellerLatestSession(ctx context.Context, in *aipb.LatestSessionRequest, opts ...grpc.CallOption) (*aipb.LatestSessionResponse, error) {
	return m.latestSessionResp, m.latestSessionErr
}

func (m *mockAIAssistantServiceClient) BuyerLatestSession(ctx context.Context, in *aipb.LatestSessionRequest, opts ...grpc.CallOption) (*aipb.LatestSessionResponse, error) {
	return m.latestSessionResp, m.latestSessionErr
}

func setupSellerAutofillRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	app.POST("/sellers/me/assistant/product-autofill", h.ProxySellerProductAutofill)
	return r
}

func TestProxySellerProductAutofill_MapsRequestAndResponse(t *testing.T) {
	mock := &mockAIAssistantServiceClient{
		autofillResp: &aipb.SellerProductAutofillResponse{
			Data: &aipb.ProductDraft{Name: "台中梨山高麗菜", Origin: "台中"},
		},
	}
	h := &Holder{
		AIAssistant: grpcclient.NewAIAssistantClientForTest(mock),
		Store:       storeResolverForTest(7001),
	}

	body := `{"text":"高麗菜一顆50元","imageUrl":"https://x/y.jpg"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/sellers/me/assistant/product-autofill", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	r := setupSellerAutofillRouter(h)
	r.ServeHTTP(w, req)

	if mock.gotAutofillReq.GetText() != "高麗菜一顆50元" || mock.gotAutofillReq.GetImageUrl() != "https://x/y.jpg" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotAutofillReq)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v (%s)", err, w.Body.String())
	}
	data := resp["data"].(map[string]any)
	if data["name"] != "台中梨山高麗菜" {
		t.Fatalf("response name not mapped to camelCase DTO: %+v", data)
	}
}
