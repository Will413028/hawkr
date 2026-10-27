package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// dualWindowEngine wires the middleware under test onto one POST route, with an
// injected limiter so counting is deterministic and never touches a real Redis.
//
// Builds its own miniredis rather than reusing ratelimit_test.go's newLimiter:
// that file is package middleware_test, and these tests need the unexported
// dualWindowRateLimit, so they must live in package middleware.
// Returns the miniredis too: its TTLs advance only via FastForward, never with
// wall-clock time, so a test that needs one window to lapse must say so
// explicitly.
func dualWindowEngine(t *testing.T, burst int, burstWindow time.Duration, sustained int, sustainedWindow time.Duration) (*gin.Engine, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	limiter := ratelimit.New(rdb, 1024, nil)
	r := gin.New()
	r.POST("/api/v1/public/food_safety_reports",
		dualWindowRateLimit(limiter, "food_safety_report", burst, burstWindow, sustained, sustainedWindow, newBlockLogSampler()),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	return r, mr
}

func postFrom(r *gin.Engine, ip string) int {
	req := httptest.NewRequest("POST", "/api/v1/public/food_safety_reports", nil)
	req.RemoteAddr = ip + ":5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestDualWindow_BurstBlocksAfterLimit(t *testing.T) {
	r, _ := dualWindowEngine(t, 5, time.Minute, 10, time.Hour)

	for i := 1; i <= 5; i++ {
		if code := postFrom(r, "203.0.113.7"); code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200 (still inside the burst allowance)", i, code)
		}
	}
	if code := postFrom(r, "203.0.113.7"); code != http.StatusTooManyRequests {
		t.Fatalf("request 6: got %d, want 429 — the burst window is not enforcing", code)
	}
}

// TestDualWindow_BlockedBurstDoesNotSpendSustained is the discriminating test
// for the ordering decision documented on DualWindowRateLimit: the short window
// is checked first precisely so a rejected burst does not consume the hourly
// budget. Reverse the two entries in the loop and this test goes red.
//
// Shape: burst 2 per minute, sustained 4 per hour. Fire 6 requests — 2 pass, 4 are
// burst-blocked. Let only the burst window lapse. If blocked requests had spent
// sustained quota, all 4 remaining would be gone and the next request would 429;
// with the ordering intact, 2 of the 4 sustained slots are still unspent.
func TestDualWindow_BlockedBurstDoesNotSpendSustained(t *testing.T) {
	r, mr := dualWindowEngine(t, 2, time.Minute, 4, time.Hour)

	var blocked int
	for i := 0; i < 6; i++ {
		if postFrom(r, "203.0.113.8") == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked != 4 {
		t.Fatalf("setup wrong: got %d blocked of 6, want 4 (2 should pass the burst)", blocked)
	}

	mr.FastForward(90 * time.Second) // burst window (1m) lapses; sustained (1h) does not

	for i := 1; i <= 2; i++ {
		if code := postFrom(r, "203.0.113.8"); code != http.StatusOK {
			t.Fatalf("post-burst request %d: got %d, want 200 — blocked requests wrongly consumed the sustained window", i, code)
		}
	}
	if code := postFrom(r, "203.0.113.8"); code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429 — the sustained window is not enforcing at all", code)
	}
}

func TestDualWindow_SubjectsCountedSeparately(t *testing.T) {
	r, _ := dualWindowEngine(t, 2, time.Minute, 10, time.Hour)

	for i := 0; i < 2; i++ {
		if code := postFrom(r, "203.0.113.9"); code != http.StatusOK {
			t.Fatalf("first subject request %d: got %d, want 200", i+1, code)
		}
	}
	if code := postFrom(r, "203.0.113.9"); code != http.StatusTooManyRequests {
		t.Fatalf("first subject is not being limited: got %d, want 429", code)
	}

	// A different caller must start with a full allowance — otherwise one abusive
	// reporter would silence food-safety reporting for everyone.
	if code := postFrom(r, "198.51.100.4"); code != http.StatusOK {
		t.Fatalf("second subject: got %d, want 200 — quota is being shared across subjects", code)
	}
}
