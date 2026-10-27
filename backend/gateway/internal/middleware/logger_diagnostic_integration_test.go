package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogger_DiagnosticOverrideRevealsOnlyListedFields(t *testing.T) {
	t.Setenv("ENV", "prod")
	future := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	dir := t.TempDir()
	path := filepath.Join(dir, "diagnostic-body-logging.yaml")
	os.WriteFile(path, []byte(`
diagnostic_body_logging:
  - route: "POST /api/v1/app/buyers/me/addresses"
    fields: ["city"]
    owner: "will"
    expires_at: "`+future+`"
`), 0o600)
	middleware.SetDiagnosticBodyLoggingConfigPathForTest(t, path)

	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.POST("/api/v1/app/buyers/me/addresses", func(c *gin.Context) { c.Status(http.StatusOK) })

	body := `{"city":"台北市","receiverPhone":"0912345678"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/addresses", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	reqBody, _ := entries[0].ContextMap()["req_body"].(string)
	if !strings.Contains(reqBody, "台北市") {
		t.Fatalf("diagnostic-allowed field 'city' must survive: %q", reqBody)
	}
	if strings.Contains(reqBody, "0912345678") {
		t.Fatalf("field not in the diagnostic rule's allowlist must stay redacted: %q", reqBody)
	}
}

func TestLogger_NoDiagnosticRule_ProdStillMetadataOnly(t *testing.T) {
	t.Setenv("ENV", "prod")
	middleware.SetDiagnosticBodyLoggingConfigPathForTest(t, filepath.Join(t.TempDir(), "absent.yaml"))

	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.POST("/api/v1/app/buyers/me/addresses", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/addresses", strings.NewReader(`{"city":"台北市"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := logs.All()
	if _, ok := entries[0].ContextMap()["req_body"]; ok {
		t.Fatalf("no diagnostic rule matching this route: prod must stay metadata-only, got: %v", entries[0].ContextMap())
	}
}
