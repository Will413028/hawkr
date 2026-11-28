package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type tier int

const (
	tierExempt tier = iota
	tierModerate
	tierDefault
)

var tierNames = map[tier]string{
	tierModerate: "moderate",
	tierDefault:  "default",
}

func tierLabel(t tier) string {
	if t == tierExempt {
		return "exempt"
	}
	return tierNames[t]
}

// RateLimitPolicy carries the per-tier quotas (requests per Window) and the
// fallback sizing. Values come from config (env-tunable); see config.Config.
type RateLimitPolicy struct {
	ModeratePerWindow int
	// ModerateSSRPerWindow / ModerateCIPerWindow are the moderate tier's quotas
	// for our own server-side callers. Only this tier splits by kind: it is the
	// anonymous-public-read tier, and 7 days of measurement found no CI traffic
	// in any other tier at all. Inventing values for axes with zero producers
	// would put unmeasured numbers in production dressed as measured ones.
	ModerateSSRPerWindow int
	ModerateCIPerWindow  int
	DefaultPerWindow     int
	// AuthFailurePerWindow caps how many 401 responses one IP may collect per
	// Window before its further AUTHENTICATED requests are refused (see
	// authFailureAxisApplies for the scope). This is a separate axis from the
	// per-tier quotas: a client stuck in a failure loop stays well inside its
	// tier's quota and is never throttled by it. 0 disables.
	AuthFailurePerWindow int
	Window               time.Duration
	FallbackCapacity     int
}

func (p RateLimitPolicy) limitFor(t tier, k SubjectKind) int {
	switch t {
	case tierModerate:
		switch k {
		case SubjectSSR:
			if p.ModerateSSRPerWindow > 0 {
				return p.ModerateSSRPerWindow
			}
		case SubjectCI:
			if p.ModerateCIPerWindow > 0 {
				return p.ModerateCIPerWindow
			}
		}
		return p.ModeratePerWindow
	default:
		return p.DefaultPerWindow
	}
}

const publicNamespace = "/api/v1/public"

var exemptExact = map[string]struct{}{
	"/api/v1/health":      {},
	"/store_map/return":   {},
	"/store_map/redirect": {},
	"/.well-known/apple-developer-merchantid-domain-association.txt": {},
}

var exemptPrefixes = []string{
	"/.well-known/",
}

func classifyTier(fullPath string) tier {
	if _, ok := exemptExact[fullPath]; ok {
		return tierExempt
	}
	for _, p := range exemptPrefixes {
		if strings.HasPrefix(fullPath, p) {
			return tierExempt
		}
	}
	if strings.HasPrefix(fullPath, publicNamespace+"/") {
		return tierModerate
	}
	return tierDefault
}

func authFailureKey(subject string) string { return "ratelimit:authfail:" + subject }

func authFailureAxisApplies(t tier, k SubjectKind) bool {
	switch k {
	case SubjectSSR, SubjectCI:
		return false
	}
	switch t {
	case tierExempt, tierModerate:
		return false
	default:
		return true
	}
}

const blockLogWindow = time.Minute

const blockLogMaxLinesPerWindow = 1000

const blockLogXFFMaxLen = 256

func newBlockLogSampler() *logSampler {
	return newLogSampler(blockLogWindow, blockLogMaxLinesPerWindow)
}

// TieredRateLimiter is the global limiter middleware. It classifies each request
// by route into a tier and enforces that tier's per-IP quota via the shared
// (cross-pod) limiter. Exempt routes pass straight through.
//
// It additionally throttles sources that keep failing authentication. The
// per-tier quotas alone do not cover this: on 2026-07-29 a broken client looped
// `DELETE /app/notifications/device-token` at 4.8 req/s — every response a 401 —
// for hours, because tierDefault allows 300 req/60s (5.0 req/s) and the loop sat
// 4% under that ceiling. It never tripped a single limit. A request that cannot
// authenticate is worth strictly less than one that can, so it gets its own,
// much tighter budget — spent only on authenticated traffic, never on the
// anonymous public reads (authFailureAxisApplies).
func TieredRateLimiter(limiter *ratelimit.Limiter, policy RateLimitPolicy) gin.HandlerFunc {
	return tieredRateLimiter(limiter, policy, newBlockLogSampler())
}

func tieredRateLimiter(limiter *ratelimit.Limiter, policy RateLimitPolicy, blockLog *logSampler) gin.HandlerFunc {
	return func(c *gin.Context) {
		t := classifyTier(c.FullPath())
		if t == tierExempt {
			c.Next()
			return
		}
		subject, kind := RateLimitSubject(c)
		authAxis := policy.AuthFailurePerWindow > 0 && authFailureAxisApplies(t, kind)

		if authAxis &&
			limiter.Count(c.Request.Context(), authFailureKey(subject), policy.Window) >= int64(policy.AuthFailurePerWindow) {
			logBlocked(c, blockLog, "auth_failures", subject)
			response.SetRateLimitHeaders(c, policy.AuthFailurePerWindow, 0, policy.Window)
			response.Err(c, http.StatusTooManyRequests, throttleBizCode(c), "too many authentication failures")
			c.Abort()
			return
		}

		limit := policy.limitFor(t, kind)
		key := "ratelimit:" + tierNames[t] + ":" + subject
		if !limiter.Allow(c.Request.Context(), key, limit, policy.Window) {
			logBlocked(c, blockLog, "tier_"+tierNames[t], subject)
			response.SetRateLimitHeaders(c, limit, 0, policy.Window)
			response.Err(c, http.StatusTooManyRequests, throttleBizCode(c), "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()

		if authAxis && c.Writer.Status() == http.StatusUnauthorized {
			_ = limiter.Allow(c.Request.Context(), authFailureKey(subject),
				policy.AuthFailurePerWindow, policy.Window)
		}
	}
}

func throttleBizCode(c *gin.Context) int32 {
	if IsDelegatedRoute(c) {
		return bizcode.DelegationRateLimited
	}
	return 429
}

func logBlocked(c *gin.Context, sampler *logSampler, reason, subject string) {
	if !sampler.admitWithNotice(c.Request.Context(), pkglogger.Warn,
		reason+":"+subject, "[RateLimit] block log truncated") {
		return
	}
	pkglogger.Warn(c.Request.Context(), "[RateLimit] request blocked",
		zap.String("reason", reason),
		zap.String("ip", forensicClientIP(c)),
		zap.String("subject", subject),
		zap.String("xff", truncateForLog(c.GetHeader("X-Forwarded-For"), blockLogXFFMaxLen)),
		zap.String("method", c.Request.Method),
		zap.String("path", routeKey(c)),
	)
}

func truncateForLog(v string, max int) string {
	if len(v) <= max {
		return v
	}
	return v[:max] + "...(truncated)"
}

// NewRateLimiter builds the global tiered limiter, reusing the Redis client
// already injected via SetRedisClient so counting is shared across pods; when
// Redis is unavailable it falls back to a bounded in-memory limiter. Called once
// at engine construction.
func NewRateLimiter(policy RateLimitPolicy, logger *zap.Logger) gin.HandlerFunc {
	policy.publishQuotas(logger)
	limiter := ratelimit.New(GuardRedisClient(), policy.FallbackCapacity, logger)
	return TieredRateLimiter(limiter, policy)
}

func (p RateLimitPolicy) publishQuotas(logger *zap.Logger) {
	for _, q := range []struct {
		kind  SubjectKind
		field string
		value int
	}{
		{SubjectVisitor, "ModeratePerWindow", p.ModeratePerWindow},
		{SubjectSSR, "ModerateSSRPerWindow", p.ModerateSSRPerWindow},
		{SubjectCI, "ModerateCIPerWindow", p.ModerateCIPerWindow},
	} {
		if q.value <= 0 && logger != nil {
			logger.Error("rate-limit quota unset — this axis falls back to the visitor quota; check the RateLimitPolicy literal and its env",
				zap.String("field", q.field), zap.String("subject_kind", q.kind.String()))
		}
		rateLimitQuota.WithLabelValues(tierNames[tierModerate], q.kind.String()).
			Set(float64(p.limitFor(tierModerate, q.kind)))
	}
	rateLimitQuota.WithLabelValues(tierNames[tierDefault], SubjectVisitor.String()).
		Set(float64(p.limitFor(tierDefault, SubjectVisitor)))
}
