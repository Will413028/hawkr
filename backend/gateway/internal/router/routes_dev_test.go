package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

// TestDevRoutes_EnvGate covers issue #1397: production had an unauthenticated
// GET cache-flush route reachable from the public group. registerDevRoutes
// must now only register under APP_ENV=local/dev/development, and only accept
// POST (GET must 404, matching Gin's "no route" behavior for an unregistered
// method+path pair).
func TestDevRoutes_EnvGate(t *testing.T) {
	h := &handler.Holder{}
	gin.SetMode(gin.TestMode)

	newRouter := func(appEnv string) *gin.Engine {
		t.Setenv("APP_ENV", appEnv)
		r := gin.New()
		v1 := r.Group(APIV1Path)
		public := v1.Group(PublicPath)
		registerDevRoutes(public, h)
		return r
	}

	do := func(r http.Handler, method, path string) int {
		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("unset APP_ENV fails closed", func(t *testing.T) {
		r := newRouter("")
		if code := do(r, http.MethodPost, "/api/v1/public/caches/flush"); code != http.StatusNotFound {
			t.Fatalf("POST flush with unset APP_ENV: want 404, got %d", code)
		}
	})

	t.Run("production fails closed", func(t *testing.T) {
		r := newRouter("production")
		if code := do(r, http.MethodPost, "/api/v1/public/caches/flush"); code != http.StatusNotFound {
			t.Fatalf("POST flush with APP_ENV=production: want 404, got %d", code)
		}
	})

	t.Run("dev registers POST only, GET is gone", func(t *testing.T) {
		r := newRouter("dev")
		if code := do(r, http.MethodGet, "/api/v1/public/caches/flush"); code != http.StatusNotFound {
			t.Fatalf("GET flush must no longer exist (issue #1397): want 404, got %d", code)
		}
		// FlushAllCaches itself will fail (no Redis wired) but that's a 503 from
		// the handler, not a 404 from the router — proves the route is reachable.
		if code := do(r, http.MethodPost, "/api/v1/public/caches/flush"); code == http.StatusNotFound {
			t.Fatalf("POST flush under APP_ENV=dev must be registered, got 404")
		}
	})
}
