package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// A blocked request leaves no access-log entry (Logger runs after the cheap
// gates) and Prometheus carries no client IP, so the block-log line is the only
// record of WHO was throttled. These drive the real middleware — a 429 through
// gin, the line read back off a zap observer — because the previous suite
// exercised the sampler's backing limiter directly and therefore never executed
// logBlocked at all (0% coverage) while appearing to cover it.

// fakeClock lets a test cross a window boundary without sleeping through one.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

// blockLogPolicy gives the default tier a quota of 1 so the second request from
// any source is blocked, which is the only way to reach logBlocked.
func blockLogPolicy() RateLimitPolicy {
	return RateLimitPolicy{
		ModeratePerWindow: 1, DefaultPerWindow: 1,
		ModerateSSRPerWindow: 1, ModerateCIPerWindow: 1,
		Window:           time.Minute,
		FallbackCapacity: 200000,
	}
}

// blockLogEngine wires the real middleware with an injectable sampler clock and
// captures whatever it logs. Trusted proxies are cleared so ClientIP is the peer
// address and never the (test-supplied, deliberately hostile) XFF header.
func blockLogEngine(sampler *logSampler) (*gin.Engine, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.WarnLevel)
	obs := zap.New(core)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(pkglogger.WithLogger(c.Request.Context(), obs))
		c.Next()
	})
	r.Use(tieredRateLimiter(ratelimit.New(nil, 200000, zap.NewNop()), blockLogPolicy(), sampler))
	r.GET("/api/v1/app/orders", func(c *gin.Context) { c.String(http.StatusOK, "") })
	return r, logs
}

func blockLogSamplerAt(clk *fakeClock) *logSampler {
	s := newBlockLogSampler()
	s.now = clk.Now
	return s
}

// hit sends one request from remoteIP and returns the status.
func hit(r *gin.Engine, remoteIP string, xff string) int {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders", nil)
	req.RemoteAddr = remoteIP + ":40000"
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func blockedLines(logs *observer.ObservedLogs) []observer.LoggedEntry {
	return logs.FilterMessage("[RateLimit] request blocked").All()
}

// The whole point of the line: a 429 must leave behind the source that caused it.
func TestBlockLog_NamesTheBlockedSource(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	r, logs := blockLogEngine(blockLogSamplerAt(clk))

	if code := hit(r, "203.0.113.7", ""); code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200 (within quota)", code)
	}
	if code := hit(r, "203.0.113.7", ""); code != http.StatusTooManyRequests {
		t.Fatalf("second request: got %d, want 429 (quota of 1 exhausted)", code)
	}

	lines := blockedLines(logs)
	if len(lines) != 1 {
		t.Fatalf("got %d block-log lines, want 1 — a 429 with no log line makes the source unidentifiable", len(lines))
	}
	fields := lines[0].ContextMap()
	if fields["ip"] != "203.0.113.7" {
		t.Fatalf("ip field = %v, want 203.0.113.7 (the line exists to name the source)", fields["ip"])
	}
	if fields["reason"] != "tier_default" {
		t.Fatalf("reason field = %v, want tier_default", fields["reason"])
	}
	if fields["path"] != "/api/v1/app/orders" {
		t.Fatalf("path field = %v, want the matched route", fields["path"])
	}
}

// A sustained flood from one source is one incident, not one incident per request.
func TestBlockLog_SuppressesRepeatsWithinWindow(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	r, logs := blockLogEngine(blockLogSamplerAt(clk))

	for i := 0; i < 300; i++ {
		hit(r, "203.0.113.8", "")
	}
	if n := len(blockedLines(logs)); n != 1 {
		t.Fatalf("got %d lines for one source in one window, want 1 — a 5 req/s loop would otherwise write ~430k lines/day", n)
	}
}

// The regression this issue is about: with LRU eviction, 50k sources blocked
// twice each produced ~100k lines (every repeat looked like a fresh key), so the
// throttle failed in precisely the distributed-abuse case it exists for. Line
// count must be bounded by the cap, not by source cardinality.
func TestBlockLog_HighCardinalityStaysUnderHardCap(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	r, logs := blockLogEngine(blockLogSamplerAt(clk))

	const sources = 50000
	for i := 0; i < sources; i++ {
		ip := fmt.Sprintf("10.0.%d.%d", (i/256)%256, i%256)
		hit(r, ip, "") // consumes the quota of 1
		hit(r, ip, "") // blocked #1
		hit(r, ip, "") // blocked #2
	}

	n := len(blockedLines(logs))
	if n > blockLogMaxLinesPerWindow {
		t.Fatalf("got %d lines from %d blocked sources, want <= %d — log volume must be capped, not proportional to cardinality",
			n, sources, blockLogMaxLinesPerWindow)
	}
	if n != blockLogMaxLinesPerWindow {
		t.Fatalf("got %d lines, want exactly %d — the budget should be spent before suppression starts", n, blockLogMaxLinesPerWindow)
	}
}

// Capping must not silently hide the scale of what was dropped, and must not
// silence an ongoing flood forever: the next window resurfaces both.
func TestBlockLog_ReportsDropsAndResurfacesNextWindow(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	r, logs := blockLogEngine(blockLogSamplerAt(clk))

	// Exhaust the per-window budget, then keep blocking so drops accumulate.
	for i := 0; i < blockLogMaxLinesPerWindow+50; i++ {
		ip := fmt.Sprintf("10.1.%d.%d", (i/256)%256, i%256)
		hit(r, ip, "")
		hit(r, ip, "")
	}
	if n := len(blockedLines(logs)); n != blockLogMaxLinesPerWindow {
		t.Fatalf("got %d lines in the first window, want %d", n, blockLogMaxLinesPerWindow)
	}

	clk.Advance(blockLogWindow + time.Second)

	// A source already logged in the previous window must be logged again: an
	// ongoing incident must not go permanently silent.
	hit(r, "10.1.0.0", "")
	if n := len(blockedLines(logs)); n != blockLogMaxLinesPerWindow+1 {
		t.Fatalf("got %d lines after the window rolled, want %d — a continuing flood must resurface", n, blockLogMaxLinesPerWindow+1)
	}
	truncated := logs.FilterMessage("[RateLimit] block log truncated").All()
	if len(truncated) != 1 {
		t.Fatalf("got %d truncation lines, want 1 — the dropped count is the only record of the real scale", len(truncated))
	}
	if got := truncated[0].ContextMap()["dropped"]; got != int64(50) {
		t.Fatalf("dropped = %v, want 50 (sources beyond the budget)", got)
	}
}

// X-Forwarded-For is caller-controlled and, on the origin-direct path, bounded
// only by net/http's 1MB header limit. One request must not be able to write an
// arbitrarily long line.
func TestBlockLog_TruncatesLongXFF(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	r, logs := blockLogEngine(blockLogSamplerAt(clk))

	long := strings.Repeat("198.51.100.1, ", 4000) // ~56KB
	hit(r, "203.0.113.10", long)
	hit(r, "203.0.113.10", long)

	lines := blockedLines(logs)
	if len(lines) != 1 {
		t.Fatalf("got %d block-log lines, want 1", len(lines))
	}
	xff, _ := lines[0].ContextMap()["xff"].(string)
	if len(xff) > blockLogXFFMaxLen+len("...(truncated)") {
		t.Fatalf("logged xff is %d bytes from a %d-byte header, want <= %d", len(xff), len(long), blockLogXFFMaxLen+len("...(truncated)"))
	}
	if !strings.HasSuffix(xff, "...(truncated)") {
		t.Fatalf("truncated xff must say so, got %q — otherwise a cut chain reads as the whole chain", xff)
	}
}
