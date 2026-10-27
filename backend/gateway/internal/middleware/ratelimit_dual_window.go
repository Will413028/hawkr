package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DualWindowRateLimit throttles one route with two windows at once: a short one
// that stops bursts and a long one that caps sustained volume.
//
// Why a route-scoped limiter on top of the global tiered one (#2295 ①):
// the global limiter classifies by sensitivity tier and answers "is this caller
// hammering the gateway". That is the wrong question for an anonymous WRITE
// endpoint like /public/food_safety_reports — 120 requests/minute is nowhere
// near abusive for browsing, and catastrophic for filing reports. B-169/B2
// specified 10 min / 5 burst + 1 hour / 10 sustained for exactly this reason.
//
// ⚠️ The subject is middleware.RateLimitSubject, NOT c.ClientIP() — see
// ruleguard banClientIPForQuota (#2264). ClientIP answers "which machine sent
// this", which for the website's SSR worker or a CI runner is one address for
// an unbounded number of people. Charging an anonymous-write quota to that
// address would let one shared egress lock out everyone behind it.
//
// Both windows must pass. The short one is checked first so a burst is rejected
// without spending the sustained counter — otherwise a caller who trips the
// burst limit would still burn their hourly budget on requests that were never
// served.
func DualWindowRateLimit(name string, burst int, burstWindow time.Duration, sustained int, sustainedWindow time.Duration) gin.HandlerFunc {
	// nil limiter = resolve the shared one on first request; see
	// sharedRouteLimiter for why it cannot be built here.
	return dualWindowRateLimit(nil, name, burst, burstWindow, sustained, sustainedWindow, newBlockLogSampler())
}

func dualWindowRateLimit(
	injected *ratelimit.Limiter,
	name string,
	burst int, burstWindow time.Duration,
	sustained int, sustainedWindow time.Duration,
	blockLog *logSampler,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		limiter := injected
		if limiter == nil {
			limiter = sharedRouteLimiter()
		}
		subject, _ := RateLimitSubject(c)
		ctx := c.Request.Context()

		// Short window first — see the doc comment for why the order matters.
		for _, w := range []struct {
			suffix string
			limit  int
			window time.Duration
		}{
			{"burst", burst, burstWindow},
			{"sustained", sustained, sustainedWindow},
		} {
			key := "ratelimit:route:" + name + ":" + w.suffix + ":" + subject
			if !limiter.Allow(ctx, key, w.limit, w.window) {
				logBlocked(c, blockLog, "route_"+name+"_"+w.suffix, subject)
				response.SetRateLimitHeaders(c, w.limit, 0, w.window)
				response.Err(c, http.StatusTooManyRequests, 429, "rate limit exceeded")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// sharedRouteLimiter hands every route-scoped limiter the same instance, built
// on the same Redis client the global limiter uses so counting stays shared
// across pods.
//
// It is lazy rather than constructed in di.go because route registration
// (router.SetupRoutes) runs before SetRedisClient has necessarily been called,
// and a limiter built with a nil client would silently degrade to the in-memory
// fallback forever — per-pod counters on a quota whose entire purpose is to be
// global. Resolving on first request instead means the client is already in
// place. ratelimit.New itself tolerates a nil client (degrade-open), so the
// worst case here is the same fallback the global limiter already accepts.
var (
	routeLimiterOnce sync.Once
	routeLimiter     *ratelimit.Limiter
)

func sharedRouteLimiter() *ratelimit.Limiter {
	routeLimiterOnce.Do(func() {
		routeLimiter = ratelimit.New(GuardRedisClient(), routeLimiterFallbackCapacity, zap.L())
	})
	return routeLimiter
}

// routeLimiterFallbackCapacity bounds the in-memory fallback used when Redis is
// unavailable. Route-scoped quotas cover a handful of endpoints with low
// traffic, so this is deliberately much smaller than the global limiter's.
const routeLimiterFallbackCapacity = 4096
