package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

type deadlineTrackingResponseWriter struct {
	http.ResponseWriter
	deadlines []time.Time
}

func (w *deadlineTrackingResponseWriter) SetReadDeadline(t time.Time) error {
	w.deadlines = append(w.deadlines, t)
	return nil
}

func TestBodyGuard_RejectsOversizedContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyGuard())
	handlerCalled := false
	r.POST("/api/v1/app/whatever", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	body := strings.NewReader(strings.Repeat("a", 3<<20))
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/whatever", body)
	req.ContentLength = 3 << 20
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
	if handlerCalled {
		t.Fatal("handler must not run when body exceeds the size guard")
	}
}

func TestBodyGuard_AllowsWithinDefaultLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyGuard())
	r.POST("/api/v1/app/whatever", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/whatever", strings.NewReader(`{"a":1}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestBodyGuard_RejectsChunkedOversizeWithoutContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyGuard())
	var readErr error
	r.POST("/api/v1/app/whatever", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		if readErr != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	body := strings.NewReader(strings.Repeat("a", 3<<20))
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/whatever", body)
	req.ContentLength = -1
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if readErr == nil {
		t.Fatal("expected MaxBytesReader to error once body exceeds the limit")
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}

func TestBodyGuard_DefaultRouteGetsDefaultReadDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyGuard())
	r.POST("/api/v1/app/whatever", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	dw := &deadlineTrackingResponseWriter{ResponseWriter: httptest.NewRecorder()}
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/whatever", strings.NewReader(`{"a":1}`))

	before := time.Now()
	r.ServeHTTP(dw, req)
	after := time.Now()

	if len(dw.deadlines) != 1 {
		t.Fatalf("expected exactly 1 SetReadDeadline call, got %d", len(dw.deadlines))
	}
	wantMin := before.Add(30 * time.Second)
	wantMax := after.Add(30 * time.Second)
	if got := dw.deadlines[0]; got.Before(wantMin) || got.After(wantMax) {
		t.Fatalf("default route deadline %v not within expected [%v, %v] (want ~30s)", got, wantMin, wantMax)
	}
}

// TestBodyGuard_SkipsReadDeadlineForBodylessRequest guards against issue
// #1624's real root cause: calling ResponseController.SetReadDeadline on a
// request with no body (GET, or any ContentLength==0 request) trips a still-
// open Go stdlib bug (golang/go#70834) where the server misreads "no body to
// read" as a read timeout and cancels that request's context — and on a
// reused keep-alive connection, that canceled state then contaminates every
// subsequent request on the SAME connection, regardless of which unrelated
// endpoint they hit. There's nothing to bound for a bodyless request anyway,
// so BodyGuard must skip the deadline call entirely for these.
func TestBodyGuard_SkipsReadDeadlineForBodylessRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyGuard())
	r.GET("/api/v1/public/products", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	dw := &deadlineTrackingResponseWriter{ResponseWriter: httptest.NewRecorder()}
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/public/products", nil)

	r.ServeHTTP(dw, req)

	if len(dw.deadlines) != 0 {
		t.Fatalf("expected SetReadDeadline to be skipped for a bodyless GET request, got %d call(s): %v", len(dw.deadlines), dw.deadlines)
	}
}

func TestBodyGuard_StillSetsReadDeadlineForChunkedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyGuard())
	r.POST("/api/v1/app/whatever", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	dw := &deadlineTrackingResponseWriter{ResponseWriter: httptest.NewRecorder()}
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/whatever", strings.NewReader(`{"a":1}`))
	req.ContentLength = -1

	r.ServeHTTP(dw, req)

	if len(dw.deadlines) != 1 {
		t.Fatalf("expected SetReadDeadline to still be called for a chunked (ContentLength==-1) body, got %d call(s)", len(dw.deadlines))
	}
}
