package httpclient_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/httpclient"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// 出站請求必須產生 client span。這是這個套件存在的唯一理由——裸 http.Client
// 在 trace 上完全不存在，整條路徑最慢的那一跳看不到。
func TestNewProducesClientSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	resp, err := httpclient.New().Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// otelhttp 的 client span 在 body 被讀到 EOF 或 Close() 時才 End()（見
	// otelhttp transport.go 的 wrappedBody），不是在 RoundTrip 回傳時就結束。
	// 用 t.Cleanup 延後關閉會讓下面的斷言在 span 結束前執行、恆為 0——這裡改成
	// 立即 Close，讓斷言驗到的是「span 真的結束了」而不是「lifecycle 剛好還沒跑到」。
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("Body.Close: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].SpanKind != trace.SpanKindClient {
		t.Errorf("SpanKind = %v, want Client", spans[0].SpanKind)
	}
}

// CheckRedirect 選項必須真的生效——Gemini 那條路徑靠它避免把 X-Goog-Api-Key
// 跟著跨網域 redirect 送出去（2026-08-22 憑證改走 header 時加的）。
func TestWithCheckRedirectApplies(t *testing.T) {
	sentinel := http.ErrUseLastResponse
	c := httpclient.New(httpclient.WithCheckRedirect(func(*http.Request, []*http.Request) error {
		return sentinel
	}))
	if c.CheckRedirect == nil {
		t.Fatal("CheckRedirect is nil")
	}
	if got := c.CheckRedirect(nil, nil); !errors.Is(got, sentinel) {
		t.Errorf("CheckRedirect returned %v, want sentinel", got)
	}
}
