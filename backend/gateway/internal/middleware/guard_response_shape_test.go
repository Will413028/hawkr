package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// guard rejections must emit the canonical Envelope shape ("message", never the
// legacy "msg") AND preserve gin's abort semantics (the downstream handler must
// not run). JWTAuth's missing-identity 401 is the representative guard.
func TestJWTAuth_MissingCredential_Reject_ShapeAndAbort(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	called := false
	r.GET("/x", middleware.JWTAuth(), func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil) // 沒有身分 header -> 401
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401; body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v; raw=%s", err, w.Body.String())
	}
	if _, ok := body["message"]; !ok {
		t.Errorf("missing 'message'; body=%v", body)
	}
	if _, ok := body["msg"]; ok {
		t.Errorf("legacy 'msg' present; body=%v", body)
	}
	if called {
		t.Error("downstream handler ran; abort failed")
	}
}
