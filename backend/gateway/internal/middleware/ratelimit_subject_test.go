package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

const (
	testOriginSecret   = "origin-secret-for-tests-0123456789"
	testOriginCISecret = "ci-secret-for-tests-9876543210" // gitleaks:allow（測試用的假 token）
	moderatePath       = "/api/v1/public/products"
)

// subjectPolicy gives each moderate-tier axis a different, tiny quota so which
// axis a request landed on is observable from how soon it is refused.
func subjectPolicy() middleware.RateLimitPolicy {
	return middleware.RateLimitPolicy{
		DefaultPerWindow:     100,
		ModeratePerWindow:    2,
		ModerateSSRPerWindow: 5,
		ModerateCIPerWindow:  8,
		AuthFailurePerWindow: 100,
		Window:               time.Minute,
		FallbackCapacity:     1000,
	}
}

// getWithHeaders fires one moderate-tier GET from a fixed peer address.
func getWithHeaders(t *testing.T, r *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, moderatePath, nil)
	req.RemoteAddr = "203.0.113.7:5555"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func signed(kind middleware.SubjectKind, visitorIP string) map[string]string {
	secret := []byte(testOriginSecret)
	if kind == middleware.SubjectCI {
		secret = []byte(testOriginCISecret)
	}
	return map[string]string{
		"X-HK-Origin-Auth": middleware.SignOriginAuth(secret, kind, visitorIP,
			http.MethodGet, moderatePath, time.Now()),
	}
}

// countUntilBlocked fires up to max requests and reports how many passed before
// the first 429.
func countUntilBlocked(t *testing.T, r *gin.Engine, headers map[string]string, max int) int {
	t.Helper()
	for i := 1; i <= max; i++ {
		if getWithHeaders(t, r, headers).Code == http.StatusTooManyRequests {
			return i - 1
		}
	}
	return max
}

func subjectEngine(t *testing.T) *gin.Engine {
	t.Helper()
	limiter, _ := newLimiter(t, 1000)
	middleware.SetOriginAuthSecrets(testOriginSecret, "", testOriginCISecret, nil)
	t.Cleanup(func() { middleware.SetOriginAuthSecrets("", "", "", nil) })
	return rlEngine(limiter, subjectPolicy())
}

// The three kinds must count on three separate keys with three separate
// quotas. Sharing a key is the whole bug: one website SSR worker (or one CI
// runner) presents a single address for an unbounded number of people, so a
// shared key means whoever arrives first spends everyone's budget.
func TestRateLimitSubject_KindsGetSeparateQuotas(t *testing.T) {
	r := subjectEngine(t)

	if got := countUntilBlocked(t, r, nil, 10); got != 2 {
		t.Fatalf("unsigned (visitor) allowed %d before 429, want 2 (ModeratePerWindow)", got)
	}
	// The visitor axis is now exhausted; SSR and CI must be unaffected by it.
	if got := countUntilBlocked(t, r, signed(middleware.SubjectSSR, ""), 12); got != 5 {
		t.Fatalf("p=ssr allowed %d before 429, want 5 (ModerateSSRPerWindow) — SSR must be its own key", got)
	}
	if got := countUntilBlocked(t, r, signed(middleware.SubjectCI, ""), 12); got != 8 {
		t.Fatalf("p=ci allowed %d before 429, want 8 (ModerateCIPerWindow) — CI must be its own key", got)
	}
}

// A signed p=visitor claim charges the named visitor, not the worker relaying
// for them: two different visitors behind one worker must not share a bucket.
// And they must share the plain-ClientIP bucket's VALUE — same "v:" namespace,
// one quota per human however they reached us.
func TestRateLimitSubject_SignedVisitorIsPerVisitor(t *testing.T) {
	r := subjectEngine(t)

	if got := countUntilBlocked(t, r, signed(middleware.SubjectVisitor, "198.51.100.5"), 10); got != 2 {
		t.Fatalf("visitor A allowed %d before 429, want 2", got)
	}
	if got := countUntilBlocked(t, r, signed(middleware.SubjectVisitor, "198.51.100.6"), 10); got != 2 {
		t.Fatalf("visitor B allowed %d before 429, want 2 — B must not inherit A's spent quota", got)
	}
}

// An unsigned caller must never be able to name its own axis. gin does not
// validate CF-Connecting-IP, so without the "v:" namespace prefix anyone
// reaching the origin directly could send CF-Connecting-IP: s:ssr and help
// themselves to the SSR quota (here 5 vs the visitor's 2).
func TestRateLimitSubject_ForgedHeaderCannotReachSSRAxis(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	middleware.SetOriginAuthSecrets(testOriginSecret, "", testOriginCISecret, nil)
	t.Cleanup(func() { middleware.SetOriginAuthSecrets("", "", "", nil) })

	// Production's setting — without it gin ignores CF-Connecting-IP entirely
	// and this test would prove nothing.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.TrustedPlatform = gin.PlatformCloudflare
	r.Use(middleware.TieredRateLimiter(limiter, subjectPolicy()))
	r.GET(moderatePath, func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	forged := map[string]string{"CF-Connecting-IP": "s:ssr"}
	if got := countUntilBlocked(t, r, forged, 10); got != 2 {
		t.Fatalf("forged CF-Connecting-IP=s:ssr allowed %d before 429, want 2 (visitor quota) — it must not get the SSR quota", got)
	}
	// The sharper half: even while being refused, those requests must not have
	// landed on the SSR key. An unprefixed subject would let this caller INCR
	// "ratelimit:moderate:s:ssr" freely and starve the real website of its
	// quota without ever being admitted to it.
	if got := countUntilBlocked(t, r, signed(middleware.SubjectSSR, ""), 12); got != 5 {
		t.Fatalf("real SSR traffic allowed %d before 429 after 10 forged CF-Connecting-IP=s:ssr requests, want 5 — the forged requests polluted the SSR counter", got)
	}
}

// A signature that does not verify falls open to the visitor axis rather than
// being rejected: a rotation window must not take the website down, and the
// degraded behaviour is the pre-#2264 behaviour.
func TestRateLimitSubject_InvalidSignatureFailsOpenToVisitor(t *testing.T) {
	r := subjectEngine(t)

	cases := map[string]map[string]string{
		"wrong key": {"X-HK-Origin-Auth": middleware.SignOriginAuth([]byte("not-the-secret"),
			middleware.SubjectSSR, "", http.MethodGet, moderatePath, time.Now())},
		"expired": {"X-HK-Origin-Auth": middleware.SignOriginAuth([]byte(testOriginSecret),
			middleware.SubjectSSR, "", http.MethodGet, moderatePath, time.Now().Add(-5*time.Minute))},
		"signed for another path": {"X-HK-Origin-Auth": middleware.SignOriginAuth([]byte(testOriginSecret),
			middleware.SubjectSSR, "", http.MethodGet, "/api/v1/public/posts", time.Now())},
		"CI key claiming ssr": {"X-HK-Origin-Auth": middleware.SignOriginAuth([]byte(testOriginCISecret),
			middleware.SubjectSSR, "", http.MethodGet, moderatePath, time.Now())},
		"garbage": {"X-HK-Origin-Auth": "not-a-signature"},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			r := r
			// Each subtest shares the engine, so assert on the axis rather than
			// an exact count: the request must be served (fail-open, not 4xx on
			// the signature itself) or refused by the visitor quota, never
			// admitted to the SSR axis.
			for i := 0; i < 3; i++ {
				if code := getWithHeaders(t, r, headers).Code; code != http.StatusOK && code != http.StatusTooManyRequests {
					t.Fatalf("%s: got %d, want 200 or 429 (verification failure must fail open, never reject)", name, code)
				}
			}
			if got := countUntilBlocked(t, r, headers, 3); got != 0 {
				t.Fatalf("%s: still allowed %d requests after the visitor quota was spent — it reached a signed axis it did not prove", name, got)
			}
		})
	}
}

// An unset secret DISABLES that key's verification: its signatures stop being
// honoured and fall back to per-ClientIP. It must not verify against the empty
// key (HMAC("", ·) is computable by anyone, so every claim would pass).
// The two keys are independent switches — the likeliest human error is creating
// only one of the two secrets.
func TestRateLimitSubject_EmptySecretDisablesOnlyThatKey(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	middleware.SetOriginAuthSecrets("", "", testOriginCISecret, nil) // main off, CI on
	t.Cleanup(func() { middleware.SetOriginAuthSecrets("", "", "", nil) })
	r := rlEngine(limiter, subjectPolicy())

	// The attack an empty secret would open if it were treated as a key rather
	// than as "off": anyone can compute HMAC("", ·), so every claim would verify
	// — including p=visitor naming a victim's address, which would let a
	// stranger exhaust that person's login and OTP budget while metering as
	// result="ok" so no alert fires.
	forgedWithEmptyKey := map[string]string{
		"X-HK-Origin-Auth": middleware.SignOriginAuth(nil, middleware.SubjectSSR, "",
			http.MethodGet, moderatePath, time.Now()),
	}
	if got := countUntilBlocked(t, r, forgedWithEmptyKey, 10); got != 2 {
		t.Fatalf("signature under the EMPTY key allowed %d before 429, want 2 — an unset secret must disable verification, never become the key", got)
	}
	// Main key off ⇒ an SSR claim (even a correctly-signed one from before the
	// key was cleared) is ignored and shares the visitor bucket. Already spent
	// above, so it must be refused immediately rather than opening a fresh axis.
	if got := countUntilBlocked(t, r, signed(middleware.SubjectSSR, ""), 10); got != 0 {
		t.Fatalf("p=ssr with main secret unset allowed %d more requests, want 0 (shares the already-spent visitor bucket)", got)
	}
	// CI key still on ⇒ unaffected by the other key being off.
	if got := countUntilBlocked(t, r, signed(middleware.SubjectCI, ""), 12); got != 8 {
		t.Fatalf("p=ci allowed %d before 429, want 8 — the CI key must not be disabled by the main key being unset", got)
	}
}

// SSR and CI never attempt authentication (SSR reads anonymously, CI asserts on
// deliberate 401s), so they stay off the auth-failure axis. Visitors must stay
// ON it — that axis is the 2026-07-29 fix and excluding visitors would retire it.
func TestRateLimitSubject_AuthFailureAxisExcludesSSRAndCI(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	middleware.SetOriginAuthSecrets(testOriginSecret, "", testOriginCISecret, nil)
	t.Cleanup(func() { middleware.SetOriginAuthSecrets("", "", "", nil) })

	policy := subjectPolicy()
	policy.AuthFailurePerWindow = 2
	policy.DefaultPerWindow = 1000

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TieredRateLimiter(limiter, policy))
	r.GET("/api/v1/app/orders", func(c *gin.Context) { c.String(http.StatusUnauthorized, "nope") })

	fire := func(headers map[string]string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/app/orders", nil)
		req.RemoteAddr = "203.0.113.7:5555"
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	ssr := map[string]string{
		"X-HK-Origin-Auth": middleware.SignOriginAuth([]byte(testOriginSecret),
			middleware.SubjectSSR, "", http.MethodGet, "/api/v1/app/orders", time.Now()),
	}

	// Well past the threshold of 2 — SSR is not counted and not refused.
	for i := 1; i <= 6; i++ {
		if code := fire(ssr); code != http.StatusUnauthorized {
			t.Fatalf("ssr 401 #%d: got %d, want 401 (SSR must stay off the auth-failure axis)", i, code)
		}
	}
	// The same traffic from a visitor trips the axis.
	for i := 1; i <= 3; i++ {
		_ = fire(nil)
	}
	if code := fire(nil); code != http.StatusTooManyRequests {
		t.Fatalf("visitor after repeated 401s: got %d, want 429 (visitors must stay ON the auth-failure axis)", code)
	}
}

// A 429 must carry enough for a client to back off correctly.
func TestRateLimit429CarriesRetryHeaders(t *testing.T) {
	limiter, _ := newLimiter(t, 1000)
	policy := subjectPolicy()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TieredRateLimiter(limiter, policy))
	r.GET(moderatePath, func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	var w *httptest.ResponseRecorder
	for i := 0; i < 3; i++ {
		w = getWithHeaders(t, r, nil)
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want \"60\" (seconds, not an HTTP-date)", got)
	}
	if got := w.Header().Get("X-RateLimit-Limit"); got != "2" {
		t.Fatalf("X-RateLimit-Limit = %q, want \"2\" (the axis actually applied)", got)
	}
	if got := w.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want \"0\"", got)
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Fatal("X-RateLimit-Reset missing")
	}
}
