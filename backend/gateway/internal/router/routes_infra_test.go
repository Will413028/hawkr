package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

// TestInfraRoutes_MetricsNotPublic covers B-1420: /metrics used to share the
// public Gin engine with the rest of the API, so anyone could read Prometheus
// metrics via the public Ingress. It now runs on a separate internal listener
// (infrastructure.startMetrics), so the public engine must not know this path.
func TestInfraRoutes_MetricsNotPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Holder{}
	r := gin.New()
	registerInfraRoutes(r, h)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /metrics on public engine: want 404, got %d", w.Code)
	}
}
