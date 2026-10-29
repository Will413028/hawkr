package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/gin-gonic/gin"
)

// rlEngineStatus registers one default-tier route whose response status the
// caller controls, so the auth-failure counter can be driven directly.
func rlEngineStatus(limiter *ratelimit.Limiter, policy middleware.RateLimitPolicy, status *int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TieredRateLimiter(limiter, policy))
	r.DELETE("/api/v1/app/notifications/device-token", func(c *gin.Context) {
		c.String(*status, "")
	})
	return r
}

func doAuthReq(r *gin.Engine) int {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/app/notifications/device-token", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// authPolicy keeps the per-tier quotas far above the auth-failure threshold, so
// any 429 observed can only have come from the auth-failure axis. This mirrors
// the real incident: the loop stayed inside its tier quota the whole time.
func authPolicy(authFailures int) middleware.RateLimitPolicy {
	return middleware.RateLimitPolicy{
		ModeratePerWindow: 1000, DefaultPerWindow: 1000,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		AuthFailurePerWindow: authFailures,
		Window:               time.Minute,
		FallbackCapacity:     1000,
	}
}

// A source that keeps getting 401 is cut off once it exceeds the threshold.
//
// This also pins that the gate READS the counter rather than incrementing it:
// were it gating with Allow, request #2 would already be refused (its own check
// plus request #1's failure would reach a threshold of 3), instead of #4.
func TestRateLimiter_AuthFailureThrottleTripsAfterThreshold(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	r := rlEngineStatus(limiter, authPolicy(3), &status)

	for i := 1; i <= 3; i++ {
		if code := doAuthReq(r); code != http.StatusUnauthorized {
			t.Fatalf("request #%d: got %d, want 401 (still under the auth-failure threshold)", i, code)
		}
	}
	if code := doAuthReq(r); code != http.StatusTooManyRequests {
		t.Fatalf("request #4: got %d, want 429 (threshold of 3 auth failures exceeded)", code)
	}
	// Still refused while the window holds — the point is that the loop stops.
	if code := doAuthReq(r); code != http.StatusTooManyRequests {
		t.Fatalf("request #5: got %d, want 429 (throttle must persist within the window)", code)
	}
}

// Successful traffic must never advance the auth-failure counter, otherwise a
// busy legitimate client would eventually throttle itself.
func TestRateLimiter_AuthFailureCounterIgnoresNon401(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusOK
	r := rlEngineStatus(limiter, authPolicy(3), &status)

	for i := 1; i <= 20; i++ {
		if code := doAuthReq(r); code != http.StatusOK {
			t.Fatalf("successful request #%d: got %d, want 200 (200s must not count as auth failures)", i, code)
		}
	}
}

// A client that fails a few times and then authenticates keeps working: the
// earlier failures must not have consumed its ability to make requests.
func TestRateLimiter_AuthFailureBelowThresholdDoesNotBlockLaterSuccess(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	r := rlEngineStatus(limiter, authPolicy(5), &status)

	for i := 1; i <= 4; i++ {
		if code := doAuthReq(r); code != http.StatusUnauthorized {
			t.Fatalf("failing request #%d: got %d, want 401", i, code)
		}
	}
	status = http.StatusOK
	for i := 1; i <= 10; i++ {
		if code := doAuthReq(r); code != http.StatusOK {
			t.Fatalf("recovered request #%d: got %d, want 200 (4 prior failures are under the threshold of 5)", i, code)
		}
	}
}

// 0 disables the axis entirely — an escape hatch that must actually escape.
func TestRateLimiter_AuthFailureThrottleDisabledWhenZero(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	r := rlEngineStatus(limiter, authPolicy(0), &status)

	for i := 1; i <= 30; i++ {
		if code := doAuthReq(r); code != http.StatusUnauthorized {
			t.Fatalf("request #%d: got %d, want 401 (throttle disabled by AuthFailurePerWindow=0)", i, code)
		}
	}
}

// Exempt routes stay exempt: they are long-lived streams and callbacks whose
// 401s must not accumulate into a block (a reconnect storm would trip it).
func TestRateLimiter_AuthFailureIgnoresExemptRoutes(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TieredRateLimiter(limiter, authPolicy(3)))
	r.GET("/api/v1/app/notifications/stream", func(c *gin.Context) {
		c.String(http.StatusUnauthorized, "")
	})

	for i := 1; i <= 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/app/notifications/stream", nil)
		req.RemoteAddr = "203.0.113.9:5555"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("exempt request #%d: got %d, want 401 (exempt routes bypass the throttle)", i, w.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// Scope of the auth-failure axis.
//
// The axis is per-IP and one IP is routinely a whole office behind NAT, a shared
// egress or a CI runner. Applying it to routes that need no credentials turns
// "this source cannot log in" into "this source cannot use the site", which on
// 2026-07-29 took the website E2E suite (and with it every web deploy) down:
// its deliberate-401 assertions exhausted the budget and the following
// /public/marketplace, /public/products and /public/posts reads were refused.
// ---------------------------------------------------------------------------

// rlEngineMixed registers one route per tier the axis has to tell apart, each
// returning a status the caller controls, on the SAME production FullPaths so
// classification is exercised black-box.
func rlEngineMixed(limiter *ratelimit.Limiter, policy middleware.RateLimitPolicy, status *int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TieredRateLimiter(limiter, policy))
	echo := func(c *gin.Context) { c.String(*status, "") }

	r.DELETE("/api/v1/app/notifications/device-token", echo) // default — needs auth (#106 loop)
	r.GET("/api/v1/public/products", echo)                   // moderate — OptionalJWT, anonymous OK
	r.GET("/api/v1/public/carts", echo)                      // moderate — OptionalJWT, anonymous OK
	return r
}

func doMixed(r *gin.Engine, method, path string) int {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "203.0.113.9:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// exhaustAuthFailures drives the IP well past the threshold using the very route
// from the #106 incident, and asserts it ends up blocked there.
func exhaustAuthFailures(t *testing.T, r *gin.Engine, status *int) {
	t.Helper()
	*status = http.StatusUnauthorized
	for i := 1; i <= 10; i++ {
		doMixed(r, http.MethodDelete, "/api/v1/app/notifications/device-token")
	}
	if code := doMixed(r, http.MethodDelete, "/api/v1/app/notifications/device-token"); code != http.StatusTooManyRequests {
		t.Fatalf("after 10 auth failures the authenticated route got %d, want 429 (the axis must have tripped)", code)
	}
}

// THE regression this scope fix exists for: once an IP is cut off from
// authenticated routes, the routes that never needed credentials must keep
// serving it. Anonymous reads have their own defence — the moderate tier quota.
func TestRateLimiter_AuthFailureDoesNotBlockPublicReads(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	r := rlEngineMixed(limiter, authPolicy(3), &status)

	exhaustAuthFailures(t, r, &status)

	status = http.StatusOK
	for i := 1; i <= 20; i++ {
		if code := doMixed(r, http.MethodGet, "/api/v1/public/products"); code != http.StatusOK {
			t.Fatalf("public read #%d: got %d, want 200 (anonymous reads must survive an auth-failure block)", i, code)
		}
	}
}

// The other half of the same invariant: the #106 loop stays blocked. Widening
// the axis's exemptions must not have cost it the only thing it is there for.
func TestRateLimiter_AuthFailureStillBlocksAuthenticatedRoutesWhilePublicPasses(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	r := rlEngineMixed(limiter, authPolicy(3), &status)

	exhaustAuthFailures(t, r, &status)

	// Interleaved, because the real failure mode is a public read "unblocking"
	// the authenticated route (or vice versa) via shared state.
	for i := 1; i <= 5; i++ {
		status = http.StatusOK
		if code := doMixed(r, http.MethodGet, "/api/v1/public/products"); code != http.StatusOK {
			t.Fatalf("interleaved public read #%d: got %d, want 200", i, code)
		}
		status = http.StatusUnauthorized
		if code := doMixed(r, http.MethodDelete, "/api/v1/app/notifications/device-token"); code != http.StatusTooManyRequests {
			t.Fatalf("interleaved #106 loop request #%d: got %d, want 429 (the loop must stay stopped)", i, code)
		}
	}
}

// /public/carts is OptionalJWT: it reads an identity when one is present but
// never rejects a caller for lacking or holding a bad one — a failed token
// resolves to "anonymous" and the request proceeds. So there is no failed
// authentication to throttle, and blocking it would only break guest checkout
// for everyone sharing the address.
func TestRateLimiter_AuthFailureDoesNotBlockOptionalJWTPublicRoutes(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	r := rlEngineMixed(limiter, authPolicy(3), &status)

	exhaustAuthFailures(t, r, &status)

	status = http.StatusOK
	for i := 1; i <= 20; i++ {
		if code := doMixed(r, http.MethodGet, "/api/v1/public/carts"); code != http.StatusOK {
			t.Fatalf("OptionalJWT public request #%d: got %d, want 200 (guest cart must survive an auth-failure block)", i, code)
		}
	}
}

// Counting side, kept in step with the gating side: a route that requires no
// credentials cannot produce a failed authentication attempt, so a 401 there
// (necessarily from downstream) must not spend the address's budget. Otherwise
// one misbehaving upstream would silently lock out authenticated traffic.
func TestRateLimiter_AuthFailureCounterIgnoresPublicReads(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	r := rlEngineMixed(limiter, authPolicy(3), &status)

	for i := 1; i <= 20; i++ {
		if code := doMixed(r, http.MethodGet, "/api/v1/public/products"); code != http.StatusUnauthorized {
			t.Fatalf("public 401 #%d: got %d, want 401 (public reads are out of scope in both directions)", i, code)
		}
	}
	// The authenticated route must still have its full budget: 3 failures
	// allowed, blocked on the 4th. If public 401s had counted, it would already
	// be blocked here.
	for i := 1; i <= 3; i++ {
		if code := doMixed(r, http.MethodDelete, "/api/v1/app/notifications/device-token"); code != http.StatusUnauthorized {
			t.Fatalf("authenticated request #%d: got %d, want 401 (20 public 401s must not have spent the budget)", i, code)
		}
	}
	if code := doMixed(r, http.MethodDelete, "/api/v1/app/notifications/device-token"); code != http.StatusTooManyRequests {
		t.Fatalf("authenticated request #4: got %d, want 429 (its own 3 failures should trip it)", code)
	}
}
