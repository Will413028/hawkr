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
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// End-to-end: the Logger middleware must redact secrets in BOTH the request
// body it captures and the response body it echoes into the access log.
func TestLogger_RedactsRequestAndResponseBodies(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.POST("/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":          0,
			"access_token":  "RESP-SECRET-ACCESS",
			"refresh_token": "RESP-SECRET-REFRESH",
		})
	})

	body := `{"phone":"0912345678","accountId":"700123456","password":"REQ-SECRET-PW"}`
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	reqLogged, _ := fields["req_body"].(string)
	respLogged, _ := fields["resp_body"].(string)

	for _, leak := range []string{"REQ-SECRET-PW", "RESP-SECRET-ACCESS", "RESP-SECRET-REFRESH"} {
		if strings.Contains(reqLogged+respLogged, leak) {
			t.Fatalf("secret %q leaked into access log: req=%q resp=%q", leak, reqLogged, respLogged)
		}
	}
	// #2506: phone is general PII and must now default to redacted — this
	// assertion is the direct regression test for the issue's finding.
	if strings.Contains(reqLogged, "0912345678") {
		t.Fatalf("PII phone field leaked into req_body: %q", reqLogged)
	}
	// Non-sensitive fields must still survive for the log to stay useful.
	if !strings.Contains(reqLogged, "700123456") {
		t.Fatalf("benign accountId field lost from req_body: %q", reqLogged)
	}
	if !strings.Contains(respLogged, "[REDACTED]") {
		t.Fatalf("resp_body should contain redaction marker: %q", respLogged)
	}
}

func TestLogger_BoundsRequestReadButPreservesFullBodyForHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())

	fullBody := strings.Repeat("x", 5<<20) // 5MiB — larger than scanLimit (16KB)
	var gotLen int
	r.POST("/upload", func(c *gin.Context) {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("handler failed to read body: %v", err)
		}
		gotLen = len(b)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodPost, "/upload", strings.NewReader(fullBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if gotLen != len(fullBody) {
		t.Fatalf("handler must still see the full body: got %d bytes, want %d", gotLen, len(fullBody))
	}
}

func TestLogger_ResponseControllerCanSetDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())

	var controllerErr error
	r.GET("/ping", func(c *gin.Context) {
		rc := http.NewResponseController(c.Writer)
		controllerErr = rc.SetWriteDeadline(time.Now().Add(time.Second))
		c.Status(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if controllerErr != nil {
		t.Fatalf("SetWriteDeadline must succeed through Logger's wrapped ResponseWriter (Unwrap chain broken?): %v", controllerErr)
	}
}

// Behind Cloudflare the access log's ip must resolve from CF-Connecting-IP
// (attested by CF, not client-controllable), never the left-most
// X-Forwarded-For which any caller can forge. TrustedPlatform mirrors the
// production wiring in infrastructure.newEngine. The XFF chain is still
// captured in its own field (bounded to headerValueLogLimit, same as every
// other client-controlled header — see TestLogger_XFFTruncatedToHeaderLimit)
// for audit/forensics.
func TestLogger_ClientIPFromCloudflareNotForgedXFF(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.TrustedPlatform = gin.PlatformCloudflare
	r.Use(middleware.Logger())
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.77")  // attacker-forged
	req.Header.Set("CF-Connecting-IP", "198.51.100.5") // CF-attested real client
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	ip, _ := fields["ip"].(string)
	xff, _ := fields["xff"].(string)

	if ip != "198.51.100.5" {
		t.Fatalf("ip must resolve from CF-Connecting-IP, got %q (forged XFF must not win)", ip)
	}
	if xff != "203.0.113.77" {
		t.Fatalf("raw XFF chain must be preserved for audit, got %q", xff)
	}
}

// #2506: Authorization is a full bearer credential; unlike client-controlled
// metadata headers it is never partially useful for debugging, so it must be
// fully redacted, not just prefix-truncated.
func TestLogger_AuthorizationFullyRedacted(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.super-secret-jwt-payload")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	auth, _ := entries[0].ContextMap()["auth"].(string)
	if auth != "[REDACTED]" {
		t.Fatalf("auth field must be exactly %q, got %q", "[REDACTED]", auth)
	}
	if strings.Contains(auth, "eyJhbGciOiJIUzI1NiJ9") { // gitleaks:allow（測試用的假 token）
		t.Fatalf("JWT prefix leaked into auth field: %q", auth)
	}
}

// #2506: X-Forwarded-For is client-controlled and must be bounded the same
// way X-App-Version/-Type/-Platform already are — no more "raw chain kept
// for audit" exception.
func TestLogger_XFFTruncatedToHeaderLimit(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	longXFF := "203.0.113.99, " + strings.Repeat("A", 100)
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Forwarded-For", longXFF)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	xff, _ := entries[0].ContextMap()["xff"].(string)
	if len([]rune(xff)) != 32 {
		t.Fatalf("xff must be truncated to 32 runes, got %d runes: %q", len([]rune(xff)), xff)
	}
	if xff != longXFF[:32] {
		t.Fatalf("xff truncation mismatch: got %q, want prefix %q", xff, longXFF[:32])
	}
}
