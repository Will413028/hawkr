package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// panicOnReadBody proves the prod path never even attempts to read the
// request body — reading it at all, redacted or not, panics the test.
type panicOnReadBody struct{}

func (panicOnReadBody) Read(p []byte) (int, error) {
	panic("#2506 regression: prod path must never read the request body")
}
func (panicOnReadBody) Close() error { return nil }

func TestLoggerProd_NeverReadsRequestBody(t *testing.T) {
	t.Setenv("ENV", "prod")
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.POST("/api/v1/app/orders", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/orders", nil)
	req.Body = panicOnReadBody{}
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req) // must not panic

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if _, ok := fields["req_body"]; ok {
		t.Fatalf("prod log must not contain a req_body field at all, got: %v", fields)
	}
	if _, ok := fields["resp_body"]; ok {
		t.Fatalf("prod log must not contain a resp_body field at all, got: %v", fields)
	}
	if route, _ := fields["route"].(string); route != "/api/v1/app/orders" {
		t.Fatalf("route template missing/wrong: %v", fields)
	}
}

func TestLoggerProd_RecordsActorIDAndBoundedError(t *testing.T) {
	t.Setenv("ENV", "prod")
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.GET("/api/v1/app/buyers/me", func(c *gin.Context) {
		c.Set("accountId", int64(700123456))
		c.Error(assertError{"validation failed: " + longString(200)})
		c.Status(http.StatusBadRequest)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/buyers/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := entriesOrFail(t, logs)
	if actorID, ok := fields["actor_id"].(int64); !ok || actorID != 700123456 {
		t.Fatalf("actor_id missing/wrong: %v", fields)
	}
	errMsg, _ := fields["error"].(string)
	if len([]rune(errMsg)) != 32 {
		t.Fatalf("error field must be bounded to 32 runes, got %d: %q", len([]rune(errMsg)), errMsg)
	}
}

// #2506 whole-branch review Fix B: loggerMetadataOnly must still surface
// app_version/app_type — the only fields
// monitoring/grafana/provisioning/dashboards/json/app-versions.json's panels
// and monitoring/loki/rules/fake/app-version.yml's AppVersionGarbageValues
// alert actually query, and prod is the only environment where that
// capability matters. These are bounded via truncateHeaderValue, same class
// as xff (which loggerMetadataOnly already kept).
func TestLoggerProd_RecordsBoundedAppVersionHeaders(t *testing.T) {
	t.Setenv("ENV", "prod")
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.GET("/api/v1/app/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/ping", nil)
	longType := longString(headerLogLimitForTest + 20)
	req.Header.Set("X-App-Version", "1.2.3")
	req.Header.Set("X-App-Type", longType)
	// App 只發 iOS，平台不再記（2026-09-26）；舊 App 送來的 header 也不記。
	req.Header.Set("X-App-Platform", "IOS")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := entriesOrFail(t, logs)
	if v, _ := fields["app_version"].(string); v != "1.2.3" {
		t.Fatalf("app_version missing/wrong: %v", fields)
	}
	appType, _ := fields["app_type"].(string)
	if len([]rune(appType)) != headerLogLimitForTest {
		t.Fatalf("app_type must be bounded to %d runes, got %d: %q", headerLogLimitForTest, len([]rune(appType)), appType)
	}
	if _, ok := fields["app_platform"]; ok {
		t.Fatalf("app_platform must not be logged: %v", fields)
	}
}

// headerLogLimitForTest mirrors the middleware package's unexported
// headerValueLogLimit (32) without importing it — this file is in the
// middleware_test (external) package.
const headerLogLimitForTest = 32

type assertError struct{ msg string }

func (e assertError) Error() string { return e.msg }

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

func entriesOrFail(t *testing.T, logs *observer.ObservedLogs) map[string]interface{} {
	t.Helper()
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	return entries[0].ContextMap()
}

var _ io.ReadCloser = panicOnReadBody{}
