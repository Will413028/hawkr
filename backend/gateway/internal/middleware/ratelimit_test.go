package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// rlRoutes registers a representative route for every tier using the SAME
// FullPath patterns as production, so classifyTier is exercised black-box.
func rlEngine(limiter *ratelimit.Limiter, policy middleware.RateLimitPolicy) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TieredRateLimiter(limiter, policy))
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }

	// exempt — SSE / WS / beacon / health / PayUni callbacks.
	// The AI assistant and Hawkr 幫找 routes used to be here; #2505 moved them into
	// the default tier, and they are registered under "tiered" below.
	r.GET("/api/v1/health", ok)
	r.GET("/api/v1/app/chat/ws", ok)
	r.GET("/api/v1/app/notifications/stream", ok)
	r.GET("/api/v1/public/posts/:id/comments/stream", ok)
	r.POST("/api/v1/public/products/:id/view", ok)
	r.POST("/webhook/payuni/notify", ok)
	r.GET("/payment/return", ok)
	r.POST("/store_map/return", ok)

	// tiered
	r.GET("/api/v1/public/products", ok)                         // moderate
	r.GET("/api/v1/app/orders", ok)                              // default
	r.POST("/api/v1/app/sellers/me/assistant/chat", ok)          // default (#2505)
	r.POST("/api/v1/app/buyers/me/assistant/orders", ok)         // default (#2505)
	r.POST("/api/v1/app/buyers/me/procurement/turns/stream", ok) // default (#2505)
	return r
}

func doReq(r *gin.Engine, method, path string) int {
	req := httptest.NewRequest(method, path, nil)
	// Stable client IP so counting is deterministic across a test's requests.
	req.RemoteAddr = "203.0.113.7:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// newLimiter builds a limiter over an in-process miniredis (shared cross-pod
// counting). Returns the mr so a test can Close() it to simulate a Redis outage.
func newLimiter(t *testing.T, capacity int) (*ratelimit.Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return ratelimit.New(rdb, capacity, nil), mr
}

func TestRateLimiter_ExemptPathsNeverLimited(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	// Tightest possible non-exempt quotas — exempt routes must ignore them.
	policy := middleware.RateLimitPolicy{
		ModeratePerWindow: 1, DefaultPerWindow: 1,
		ModerateSSRPerWindow: 1, ModerateCIPerWindow: 1,
		Window: time.Minute, FallbackCapacity: 1000,
	}
	r := rlEngine(limiter, policy)

	exempt := []struct{ method, path string }{
		{"GET", "/api/v1/health"},
		{"GET", "/api/v1/app/chat/ws"},
		{"GET", "/api/v1/app/notifications/stream"},
		{"GET", "/api/v1/public/posts/42/comments/stream"},
		{"POST", "/api/v1/public/products/42/view"},
		{"POST", "/webhook/payuni/notify"},
		{"GET", "/payment/return"},
		{"POST", "/store_map/return"},
	}
	for _, e := range exempt {
		for i := 0; i < 5; i++ {
			if code := doReq(r, e.method, e.path); code != http.StatusOK {
				t.Fatalf("exempt %s %s: request #%d got %d, want 200 (must never be rate limited)", e.method, e.path, i+1, code)
			}
		}
	}
}

// TestRateLimiter_AIRoutesEnforce429 is the behavioural half of #2505: the three
// AI trees are not merely classified as tierDefault, they are actually refused
// once that tier's quota is spent. The classification table
// (ratelimit_ai_routes_test.go) proves the verdict; this proves the consequence.
//
// The three probes are one per tree, and deliberately include the SSE turn
// endpoint: the exemption existed for its sake, and it is the one that bills a
// paid model per request.
func TestRateLimiter_AIRoutesEnforce429(t *testing.T) {
	policy := middleware.RateLimitPolicy{
		ModeratePerWindow: 100, DefaultPerWindow: 2,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		Window: time.Minute, FallbackCapacity: 1000,
	}

	for _, probe := range []struct{ method, path string }{
		{"POST", "/api/v1/app/sellers/me/assistant/chat"},
		{"POST", "/api/v1/app/buyers/me/assistant/orders"},
		{"POST", "/api/v1/app/buyers/me/procurement/turns/stream"},
	} {
		// All three share ONE default bucket per subject, so each probe needs a
		// fresh limiter — otherwise probe #2 starts against a drained quota and
		// would "pass" for the wrong reason.
		limiter, _ := newLimiter(t, 1000)
		r := rlEngine(limiter, policy)
		for i := 1; i <= 2; i++ {
			if code := doReq(r, probe.method, probe.path); code != http.StatusOK {
				t.Fatalf("%s %s request #%d: got %d, want 200", probe.method, probe.path, i, code)
			}
		}
		if code := doReq(r, probe.method, probe.path); code != http.StatusTooManyRequests {
			t.Errorf("%s %s request #3: got %d, want 429 — this route must not be rate-limit exempt (#2505)", probe.method, probe.path, code)
		}
	}
}

func TestRateLimiter_TierEnforces429(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	policy := middleware.RateLimitPolicy{
		ModeratePerWindow: 3, DefaultPerWindow: 100,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		Window: time.Minute, FallbackCapacity: 1000,
	}
	r := rlEngine(limiter, policy)

	// moderate quota = 3: first 3 pass, 4th is 429.
	for i := 1; i <= 3; i++ {
		if code := doReq(r, "GET", "/api/v1/public/products"); code != http.StatusOK {
			t.Fatalf("moderate request #%d: got %d, want 200", i, code)
		}
	}
	if code := doReq(r, "GET", "/api/v1/public/products"); code != http.StatusTooManyRequests {
		t.Fatalf("moderate request #4: got %d, want 429", code)
	}
}

// 各層各有自己的桶：moderate 用完了，同一個來源打 default 的路由仍然照 default 的配額。
func TestRateLimiter_TiersHaveSeparateBuckets(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	policy := middleware.RateLimitPolicy{
		ModeratePerWindow: 2, DefaultPerWindow: 10,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		Window: time.Minute, FallbackCapacity: 1000,
	}
	r := rlEngine(limiter, policy)

	_ = doReq(r, "GET", "/api/v1/public/products")
	_ = doReq(r, "GET", "/api/v1/public/products")
	if code := doReq(r, "GET", "/api/v1/public/products"); code != http.StatusTooManyRequests {
		t.Fatalf("moderate request #3: got %d, want 429", code)
	}
	for i := 1; i <= 3; i++ {
		if code := doReq(r, "GET", "/api/v1/app/orders"); code != http.StatusOK {
			t.Fatalf("default request #%d: got %d, want 200 (default has its own bucket)", i, code)
		}
	}
}

func TestRateLimiter_RedisSharedAcrossPods(t *testing.T) {
	// Two limiters over the SAME Redis simulate two pods. A per-pod limiter would
	// let each pod allow the full quota; a shared counter must not.
	mr := miniredis.RunT(t)
	rdbA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdbB := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdbA.Close(); _ = rdbB.Close() })

	policy := middleware.RateLimitPolicy{
		ModeratePerWindow: 4, DefaultPerWindow: 100,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		Window: time.Minute, FallbackCapacity: 1000,
	}
	podA := rlEngine(ratelimit.New(rdbA, 1000, nil), policy)
	podB := rlEngine(ratelimit.New(rdbB, 1000, nil), policy)

	// Same client IP, moderate quota 4: 2 via pod A + 2 via pod B = 4 (all OK),
	// then the 5th (either pod) is blocked by the shared counter.
	if c := doReq(podA, "GET", "/api/v1/public/products"); c != http.StatusOK {
		t.Fatalf("podA #1: got %d, want 200", c)
	}
	if c := doReq(podA, "GET", "/api/v1/public/products"); c != http.StatusOK {
		t.Fatalf("podA #2: got %d, want 200", c)
	}
	if c := doReq(podB, "GET", "/api/v1/public/products"); c != http.StatusOK {
		t.Fatalf("podB #1: got %d, want 200", c)
	}
	if c := doReq(podB, "GET", "/api/v1/public/products"); c != http.StatusOK {
		t.Fatalf("podB #2: got %d, want 200", c)
	}
	if c := doReq(podB, "GET", "/api/v1/public/products"); c != http.StatusTooManyRequests {
		t.Fatalf("shared 5th request: got %d, want 429 (quota must be shared across pods, not per-pod)", c)
	}
}

func TestRateLimiter_FallbackWhenRedisDown(t *testing.T) {
	limiter, mr := newLimiter(t, 1000)
	mr.Close() // simulate Redis outage → Eval errors → in-memory fallback

	policy := middleware.RateLimitPolicy{
		ModeratePerWindow: 2, DefaultPerWindow: 100,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		Window: time.Minute, FallbackCapacity: 1000,
	}
	r := rlEngine(limiter, policy)

	// Fallback must still serve traffic (degrade-open, not hard-block) AND still
	// enforce the quota per-pod.
	if c := doReq(r, "GET", "/api/v1/public/products"); c != http.StatusOK {
		t.Fatalf("fallback #1: got %d, want 200 (Redis down must not block)", c)
	}
	if c := doReq(r, "GET", "/api/v1/public/products"); c != http.StatusOK {
		t.Fatalf("fallback #2: got %d, want 200", c)
	}
	if c := doReq(r, "GET", "/api/v1/public/products"); c != http.StatusTooManyRequests {
		t.Fatalf("fallback #3: got %d, want 429 (fallback must still enforce quota)", c)
	}
}

// TestTrustedProxies_ForgedXFFIgnored asserts the #1416 invariant: a forged
// X-Forwarded-For from a non-trusted peer (no Cloudflare header) must not move
// ClientIP off the real socket peer.
func TestTrustedProxies_ForgedXFFIgnored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.TrustedPlatform = gin.PlatformCloudflare
	if err := r.SetTrustedProxies([]string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
	r.GET("/whoami", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	// (1) Untrusted peer forging XFF → ClientIP stays the socket peer.
	req := httptest.NewRequest("GET", "/whoami", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Body.String(); got != "203.0.113.9" {
		t.Fatalf("forged XFF from untrusted peer: ClientIP=%q, want socket peer 203.0.113.9", got)
	}

	// (2) Trusted proxy (Traefik CIDR) setting XFF → that XFF is honoured.
	req = httptest.NewRequest("GET", "/whoami", nil)
	req.RemoteAddr = "10.1.2.3:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Body.String(); got != "1.2.3.4" {
		t.Fatalf("XFF from trusted proxy: ClientIP=%q, want 1.2.3.4", got)
	}

	// (3) Cloudflare platform header wins for normal ingress.
	req = httptest.NewRequest("GET", "/whoami", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	req.Header.Set("CF-Connecting-IP", "5.6.7.8")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Body.String(); got != "5.6.7.8" {
		t.Fatalf("CF-Connecting-IP: ClientIP=%q, want 5.6.7.8", got)
	}
}

// TestTrustedProxies_InClusterPeerIsAlwaysTrusted pins the uncomfortable half of
// the #1416 story, because a comment in di.go used to claim the opposite.
//
// In this cluster klipper-lb SNATs every inbound request (POSTROUTING -j
// MASQUERADE), so the peer SetTrustedProxies inspects is ALWAYS 10.42.0.1 — and
// that address is inside the configured 10.0.0.0/8. Case (1) above passes only
// because the test dials from a public address, which no real request ever does.
// Case (2) is what production actually looks like: the allowlist is satisfied by
// everybody, Cloudflare edge and origin-direct attacker alike, and a forged XFF
// is honoured. What keeps forged headers out today is traefik overwriting them,
// not this setting.
//
// Consequence, and the reason this test exists: if traefik ever gains a
// forwardedHeaders.trustedIPs entry it will start passing XFF through, and
// nothing in Go will notice. The invariant that must survive that is the
// narrower one — an unsigned request can only land in the "v:" namespace and can
// never name the SSR or CI axis. That is asserted here so the two facts stay
// stated together.
func TestTrustedProxies_InClusterPeerIsAlwaysTrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.TrustedPlatform = gin.PlatformCloudflare
	if err := r.SetTrustedProxies([]string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
	r.GET("/whoami", func(c *gin.Context) {
		subject, kind := middleware.RateLimitSubject(c)
		c.String(http.StatusOK, c.ClientIP()+"|"+subject+"|"+kind.String())
	})

	// 10.42.0.1 is the klipper-lb SNAT address every real request arrives from.
	fire := func(header, value string) string {
		req := httptest.NewRequest("GET", "/whoami", nil)
		req.RemoteAddr = "10.42.0.1:5555"
		req.Header.Set(header, value)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Body.String()
	}

	// (1) The hole: a forged XFF from the SNAT peer IS honoured, because that
	// peer is inside the trusted CIDR — as every request's peer always is.
	if got, want := fire("X-Forwarded-For", "1.2.3.4"), "1.2.3.4|v:1.2.3.4|visitor"; got != want {
		t.Fatalf("got %q, want %q — SetTrustedProxies does not (and here cannot) stop a forged XFF; if this ever starts passing again, re-read whether traefik still overwrites the header", got, want)
	}

	// (2) The guarantee that must survive it: gin does NOT validate the
	// Cloudflare platform header, so an origin-direct caller can put any string
	// in it. Charging that string straight to a quota key would let it name the
	// SSR axis (20x quota). The "v:" namespace plus the parse is what stops it.
	if got, want := fire("CF-Connecting-IP", "s:ssr"), "s:ssr|v:invalid|visitor"; got != want {
		t.Fatalf("got %q, want %q — ClientIP=s:ssr is expected (gin passes the platform header through unvalidated); the subject must still be v:invalid / visitor, never the SSR axis", got, want)
	}
}
