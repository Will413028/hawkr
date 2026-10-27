package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// POST /app/image_transformations，額度用完時 ai-service 回 resp.Code=4291（err=nil）；
// EnvelopeGuardInterceptor 把它轉成 gRPC error（2026-07-13 起 4291 已映射 ResourceExhausted，
// 見 envelope_guard_interceptor.go），此時 h.AI.TransformImage 回傳的 err 一定非 nil，
// resp 一定是 nil。handler 舊版在這裡硬編 502、下面再檢查 resp.Code == 4291 是打不到的死碼，
// 造成額度用完被誤判成 502（2026-07-13 dev 環境 e2e 冒煙實測發現：這條 trace 跟同一天稍早修的
// rolling-deploy 502 是兩個完全不同的根因）。regression test。

func setupTransformImageRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	app.POST("/image_transformations", h.ProxyTransformImage)
	return r
}

func newTransformImageRequest(t *testing.T) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("image", "test.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte("fake-image-bytes")); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := w.WriteField("prompt", "remove background"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/image_transformations", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Account-Id", "99")
	return req
}

func TestProxyTransformImage_QuotaExceeded_Returns429NotDeadCode(t *testing.T) {
	h := &Holder{
		AI: grpcclient.NewAIImageClientForTest(&mockAIImageServiceClient{
			TransformImageFn: func(ctx context.Context, in *aipb.TransformImageRequest, opts ...grpc.CallOption) (*aipb.TransformImageResponse, error) {
				// Simulates what the wire actually carries once EnvelopeGuardInterceptor
				// has run: err != nil, resp is never populated (nil), and the business
				// code rides along as a "[4291] " prefix on the message. The mock means
				// the real interceptor never executes here — envelope_guard_interceptor_test.go
				// is what pins the prefix being produced; this test pins it being consumed.
				return nil, grpcstatus.Error(codes.ResourceExhausted, "[4291] 本週 AI 修圖額度已用完（10/10），下週一重置")
			},
		}),
		Store: storeResolverForTest(7001),
	}

	r := setupTransformImageRouter(h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTransformImageRequest(t))

	assertHTTPStatus(t, w, http.StatusTooManyRequests)

	body := w.Body.String()
	if strings.Contains(body, "rpc error") || strings.Contains(body, "FailedPrecondition") {
		t.Errorf("raw gRPC status string leaked into response body: %s", body)
	}

	// GrpcErr's ResourceExhausted branch restores the business code from the
	// message prefix, so the envelope carries 4291 and the display message is
	// prefix-free. HTTP 429 alone must never be read as "quota exhausted" —
	// the rate limiter emits HTTP 429 with envelope code 429, and both
	// frontends now discriminate on `code` (#2272).
	respBody := parseJSON(t, strings.NewReader(body))
	assertCode(t, respBody, 4291)
	if msg, _ := respBody["message"].(string); msg != "本週 AI 修圖額度已用完（10/10），下週一重置" {
		t.Errorf("message = %q, want the accurate quota-exceeded text (hawkr_seller falls back to this field)", msg)
	}
}

// 上游若送出沒有業務碼前綴的 ResourceExhausted（並發 job 上限這類容量拒絕），仍走
// 泛用 429 塌縮——前綴還原是加法，不得改變既有無前綴路徑的形狀。
func TestProxyTransformImage_ResourceExhaustedWithoutBizCode_Returns429Envelope(t *testing.T) {
	h := &Holder{
		AI: grpcclient.NewAIImageClientForTest(&mockAIImageServiceClient{
			TransformImageFn: func(ctx context.Context, in *aipb.TransformImageRequest, opts ...grpc.CallOption) (*aipb.TransformImageResponse, error) {
				return nil, grpcstatus.Error(codes.ResourceExhausted, "too many concurrent jobs")
			},
		}),
		Store: storeResolverForTest(7001),
	}

	r := setupTransformImageRouter(h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTransformImageRequest(t))

	assertHTTPStatus(t, w, http.StatusTooManyRequests)
	respBody := parseJSON(t, strings.NewReader(w.Body.String()))
	assertCode(t, respBody, 429)
}

func TestProxyTransformImage_TransportFailure_Returns503(t *testing.T) {
	h := &Holder{
		AI: grpcclient.NewAIImageClientForTest(&mockAIImageServiceClient{
			TransformImageFn: func(ctx context.Context, in *aipb.TransformImageRequest, opts ...grpc.CallOption) (*aipb.TransformImageResponse, error) {
				return nil, grpcstatus.Error(codes.Unavailable, "connection refused")
			},
		}),
		Store: storeResolverForTest(7001),
	}

	r := setupTransformImageRouter(h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newTransformImageRequest(t))

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}
