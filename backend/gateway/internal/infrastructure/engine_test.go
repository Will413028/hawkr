package infrastructure

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestNewEngine_BodyGuardRunsBeforeAnyBodyRead(t *testing.T) {
	cfg := &config.Config{}
	r := newEngine(cfg, zap.NewNop(), &handler.Holder{})

	// Even an unmatched route goes through the global middleware chain
	// (Gin includes Use()-registered middleware in its 404 handling), so
	// this proves BodyGuard runs ahead of everything else, including on
	// paths with no registered handler.
	body := strings.NewReader(strings.Repeat("a", 3<<20))
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/does-not-exist", body)
	req.ContentLength = 3 << 20
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("BodyGuard must reject an oversized body before routing/handler logic runs, got %d", w.Code)
	}
}

// End-to-end: proves the write-deadline Unwrap() chain (Task 3's Logger
// wrapping) and the streaming write-deadline code (Tasks 4/5) actually
// compose through the real newEngine(...) middleware stack — each is only
// proven in isolation elsewhere (logger_integration_test.go's
// TestLogger_ResponseControllerCanSetDeadline; sse/stream_test.go and
// handler/ai_stream_test.go's raw fake writers). httptest.NewRecorder()
// doesn't support deadlines, so this needs a real connection.
func TestNewEngine_WriteDeadlineSurvivesFullMiddlewareChain(t *testing.T) {
	cfg := &config.Config{}
	r := newEngine(cfg, zap.NewNop(), &handler.Holder{})

	var controllerErr error
	r.GET("/write-deadline-check", func(c *gin.Context) {
		rc := http.NewResponseController(c.Writer)
		controllerErr = rc.SetWriteDeadline(time.Now().Add(time.Second))
		c.Status(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/write-deadline-check")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if controllerErr != nil {
		t.Fatalf("SetWriteDeadline must succeed through the full newEngine middleware chain (Unwrap chain broken?): %v", controllerErr)
	}
}
