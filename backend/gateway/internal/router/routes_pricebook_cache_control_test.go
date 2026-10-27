package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestMarketOverviewRoutes_NoStoreBeforeAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerPricebookRoutes(r.Group("/api/v1/app"), &handler.Holder{})
	for _, tc := range []struct{ method, path string }{{"GET", "/overview"}, {"POST", "/watch-targets"}, {"DELETE", "/watch-targets/123"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, "/api/v1/app/sellers/pricebook"+tc.path, nil))
		require.Equal(t, 401, w.Code, tc.path)
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"), tc.path)
	}
}
