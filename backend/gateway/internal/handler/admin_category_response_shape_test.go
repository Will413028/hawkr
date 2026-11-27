package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func assertMessageNoMsg(t *testing.T, body map[string]interface{}) {
	t.Helper()
	if _, ok := body["message"]; !ok {
		t.Errorf("missing 'message'; body=%v", body)
	}
	if _, ok := body["msg"]; ok {
		t.Errorf("legacy 'msg' still present; body=%v", body)
	}
}

// TestCategoryEmitter_MessageShape exercises a converted category emitter via an
// early-return validation path (no Category gRPC client needed). It must emit
// "message" (not "msg") at the original HTTP status.
func TestCategoryEmitter_MessageShape(t *testing.T) {
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.GET("/public/categories/:id", (&Holder{}).GetCategoryByID)
	v1.POST("/app/categories", (&Holder{}).CreateCategory)

	t.Run("BadRequest message shape", func(t *testing.T) {
		rr := gin.New()
		rr.GET("/c", func(c *gin.Context) { (&Holder{}).GetCategoryByID(c) })
		req := httptest.NewRequest("GET", "/c", nil)
		w := httptest.NewRecorder()
		rr.ServeHTTP(w, req)
		assertHTTPStatus(t, w, http.StatusBadRequest)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 400)
		assertMessageNoMsg(t, body)
	})

	t.Run("Unauthorized preserves verbatim message at 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/app/categories", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assertHTTPStatus(t, w, http.StatusUnauthorized)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 401)
		assertMessageNoMsg(t, body)
		if msg, _ := body["message"].(string); msg != "未授權：缺少用戶身份" {
			t.Errorf("message = %q, want verbatim 未授權：缺少用戶身份", msg)
		}
	})
}
