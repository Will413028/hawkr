package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDelegatedStorefrontUnavailableAnswers503WithoutTouchingUpstreams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Holder{}
	r := gin.New()
	r.GET("/s", h.DelegatedStorefrontUnavailable)
	r.PATCH("/s", h.DelegatedStorefrontUnavailable)
	r.POST("/s/logo", h.DelegatedStorefrontUnavailable)
	r.POST("/s/cover", h.DelegatedStorefrontUnavailable)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/s"}, {http.MethodPatch, "/s"}, {http.MethodPost, "/s/logo"}, {http.MethodPost, "/s/cover"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		var env struct {
			Code int `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if w.Code != http.StatusServiceUnavailable || env.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s：HTTP %d code %d，want 503", tc.method, tc.path, w.Code, env.Code)
		}
	}
}
