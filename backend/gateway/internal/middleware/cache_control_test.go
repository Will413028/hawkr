package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

const wantCacheControl = "private, no-store"

func TestPrivateNoStore_SetsHeaderOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.PrivateNoStore())
	r.GET("/api/v1/app/whatever", func(c *gin.Context) { c.Status(http.StatusOK) })

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/whatever", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != wantCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, wantCacheControl)
	}
}

// TestPrivateNoStore_SetsHeaderOnAbortedRequest 釘住這支中介層存在的主要理由：
// JWTAuth 這類 gate 以 AbortWithStatusJSON 早退，那條 401 一樣不能被任何快取留下。
// 只有「header 在 c.Next() 之前寫」且「中介層排在 gate 之前」兩件事同時成立，
// 這個測試才會綠。
func TestPrivateNoStore_SetsHeaderOnAbortedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.PrivateNoStore())
	r.Use(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401})
	})
	handlerCalled := false
	r.GET("/api/v1/app/whatever", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/whatever", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if handlerCalled {
		t.Fatal("handler should not run after abort")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != wantCacheControl {
		t.Fatalf("Cache-Control on 401 = %q, want %q", got, wantCacheControl)
	}
}

// TestPrivateNoStore_HandlerCanOverride 記錄 gin c.Header 的 Set 語意：handler 內
// 後寫的值會覆蓋中介層。兩處自己寫 SSE header 的地方因此必須引用同一個常數，
// 否則掛了中介層也會被蓋回去——這個測試就是那條約束的可執行版本。
func TestPrivateNoStore_HandlerCanOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.PrivateNoStore())
	r.GET("/api/v1/app/whatever", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/whatever", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want the handler's own value", got)
	}
}
