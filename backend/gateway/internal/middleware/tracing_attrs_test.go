package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// #2451 privacy 條 9：span name 與 attributes 不含 raw path（dynamic ID）
// 與 raw query——那些留在 access log 層，span 只承載低基數 route template。
func TestTracingMiddleware_SpanAttrs_NoRawPathOrQuery(t *testing.T) {
	exporter := setupTracingTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TracingMiddleware())
	r.GET("/api/v1/users/:id", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest("GET", "/api/v1/users/987654321?email=leak%40example.com", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name != "/api/v1/users/:id" {
		t.Fatalf("span name 不是 route template: %q", span.Name)
	}
	var (
		hasRoute  bool
		gotMethod string
		gotStatus int64
	)
	for _, attr := range span.Attributes {
		v := attr.Value.String()
		if strings.Contains(v, "987654321") || strings.Contains(v, "leak") {
			t.Fatalf("span attr %s 洩漏 raw ID/query: %q", attr.Key, v)
		}
		switch string(attr.Key) {
		case "http.route":
			hasRoute = true
		case "http.request.method":
			gotMethod = attr.Value.AsString()
		case "http.response.status_code":
			gotStatus = attr.Value.AsInt64()
		}
	}
	if !hasRoute {
		t.Fatal("缺 http.route attribute")
	}
	if gotMethod != "GET" {
		t.Fatalf("http.request.method=%q，want GET", gotMethod)
	}
	if gotStatus != 200 {
		t.Fatalf("http.response.status_code=%d，want 200", gotStatus)
	}
}

// unmatched route（404 probe、掃描器亂打的路徑）不得把 raw path 當 span
// name——高基數又可能含垃圾輸入；固定值即可，路徑本身看 access log。
func TestTracingMiddleware_UnmatchedRoute_LowCardinalitySpanName(t *testing.T) {
	exporter := setupTracingTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TracingMiddleware())

	req := httptest.NewRequest("GET", "/secret-probe/../.env.bak", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	if got := spans[0].Name; got != "unmatched" {
		t.Fatalf(`unmatched route 的 span name 應為固定值 "unmatched"，got %q`, got)
	}
}
