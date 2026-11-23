package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// 機敏上傳的暫存 token 不進 span 的名稱與任何屬性，內層 handler 仍看到真的路徑。
func TestTracingRedactsSecureUploadToken(t *testing.T) {
	const token = "SECRETTOKEN123"
	for _, path := range []string{
		"/api/files/secure_uploads/" + token + "/chunks/0?x=1",
		"/api/files/secure_uploads/" + token,
		"/api/files/secure_uploads//" + token + "/chunks/0",
		"/api//files/secure_uploads/" + token,
	} {
		rec := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
		var seen string
		h := withTracing(tp, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			seen = r.URL.RequestURI()
		}))
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, path, nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
		if seen != path {
			t.Errorf("內層看到 %q，預期 %q", seen, path)
		}
		spans := rec.Ended()
		if len(spans) == 0 {
			t.Fatal("沒有 span")
		}
		for _, s := range spans {
			if strings.Contains(s.Name(), token) {
				t.Errorf("span 名稱含 token：%q", s.Name())
			}
			for _, kv := range s.Attributes() {
				if strings.Contains(kv.Value.String(), token) {
					t.Errorf("屬性 %s 含 token：%s", kv.Key, kv.Value.String())
				}
			}
		}
	}
}
