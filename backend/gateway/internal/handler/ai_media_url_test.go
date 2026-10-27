package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 49 號 §6.1：ai_styles 回應必須成對回 thumbnailKey（裸 objectKey）＋ thumbnailUrl（Gateway 以
// MEDIA_BASE_URL 組出的顯示 URL）。此測試釘住「URL 值不變」——給定 key，thumbnailUrl 必須等於
// 現行 CDN 域名拼接結果（Phase 7 前不換域名）。
func TestProxyListAiStyles_ThumbnailKeyAndURLPaired(t *testing.T) {
	const key = "public/ai-style/7114/20260717/abc.jpg"
	h := &Holder{
		MediaBaseURL: "https://test-media.example.com",
		AI: grpcclient.NewAIImageClientForTest(&mockAIImageServiceClient{
			ListAiStylesFn: func(ctx context.Context, in *aipb.ListAiStylesRequest, opts ...grpc.CallOption) (*aipb.ListAiStylesResponse, error) {
				return &aipb.ListAiStylesResponse{
					Code:    0,
					Message: "success",
					Data: &aipb.AiStyleListData{
						DefaultPrompt: "dp",
						WeeklyUsage:   "1",
						WeeklyLimit:   "10",
						Styles: []*aipb.AiStyleItem{
							{Id: "1", Name: "n", Label: "l", Prompt: "p", ThumbnailKey: key, SortOrder: "0"},
						},
					},
				}, nil
			},
		}),
		Store: storeResolverForTest(7114),
	}

	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	app.GET("/ai_styles", h.ProxyListAiStyles)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/ai_styles", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data, _ := body["data"].(map[string]interface{})
	styles, _ := data["styles"].([]interface{})
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	s0, _ := styles[0].(map[string]interface{})
	if got := s0["thumbnailKey"]; got != key {
		t.Errorf("thumbnailKey = %v, want raw key %q", got, key)
	}
	const wantURL = "https://test-media.example.com/i/original/" + key
	if got := s0["thumbnailUrl"]; got != wantURL {
		t.Errorf("thumbnailUrl = %v, want %q (Image Gateway 原圖路徑 /i/original/)", got, wantURL)
	}
}
