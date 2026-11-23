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
		"/api/files/admin/secure_uploads/" + token + "/chunks/0",
		"/api/files/admin/secure_uploads/" + token + "/chunks/0?x=1",
		"/api/files/admin/secure_uploads/" + token,
		"/api/files/admin/secure_uploads//" + token + "/chunks/0",
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

// token 換成 {token}，路由的其餘部分留著：顧客與後台（代行營運）的兩個前綴各自保留自己的路徑，trace 上分得出是哪一條路由。
func TestRedactedTokenPathKeepsTheRoute(t *testing.T) {
	for in, want := range map[string]string{
		"/api/files/secure_uploads/T/chunks/0":             "/api/files/secure_uploads/{token}/chunks/0",
		"/api/files/secure_uploads/T":                      "/api/files/secure_uploads/{token}",
		"/api/files/admin/secure_uploads/T/chunks/12":      "/api/files/admin/secure_uploads/{token}/chunks/12",
		"/api/files/admin/secure_uploads/T":                "/api/files/admin/secure_uploads/{token}",
		"/api/files/admin/secure_uploads/T/chunks/0/extra": "/api/files/admin/secure_uploads/{redacted}",
		"/api/files/secure_uploads/T/other":                "/api/files/secure_uploads/{redacted}",
		"/api//files/admin/secure_uploads/T":               "/api/files/{redacted}",
		"/api/files/admin/secure_uploads//T/chunks/0":      "/api/files/admin/secure_uploads/{redacted}",
	} {
		if got, ok := redactedTokenPath(in); !ok || got != want {
			t.Errorf("redactedTokenPath(%q) = %q, %v，預期 %q", in, got, ok, want)
		}
	}
	if _, ok := redactedTokenPath("/api/files/admin/seller/kyc/1"); ok {
		t.Error("不是機敏上傳的路徑不該被遮")
	}
}
