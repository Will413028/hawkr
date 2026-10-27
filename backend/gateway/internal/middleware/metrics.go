package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// subject_kind and tier make "which axis is this traffic on" answerable —
	// without them a 429 spike cannot be attributed to visitors, SSR or CI.
	// The subject ITSELF is deliberately absent: "v:<ip>" has unbounded
	// cardinality. Which subject got blocked is a log question (logBlocked),
	// not a metric one.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status", "subject_kind", "tier"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// originAuthTotal counts every request by what its X-HK-Origin-Auth header
	// proved. Choosing signatures over a shared plaintext secret was justified
	// by "the failure is measurable" — this is the measurement. Without it, a
	// broken signing chain degrades to per-ClientIP silently: pages 200, CI
	// green, and the whole website back on one shared quota key.
	originAuthTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_origin_auth_total",
			Help: "X-HK-Origin-Auth verification outcomes (ok/absent/bad_sig/expired/malformed)",
		},
		[]string{"result"},
	)

	// originAuthEnabled is 1 when that key is configured, 0 when its
	// verification is disabled. A gauge, not rate(ok)==0: a counter that never
	// fired has no series at all, so the "secret was never injected" case —
	// precisely the one worth alerting on — would evaluate to an empty vector
	// and never fire. The key label is required because the two secrets are
	// independent switches and "only one of the two got created by hand" is the
	// most likely failure shape; any aggregation over it hides exactly that.
	originAuthEnabled = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_origin_auth_enabled",
			Help: "1 = X-HK-Origin-Auth verification active for this key, 0 = disabled (secret unset)",
		},
		[]string{"key"},
	)

	// rateLimitQuota publishes the configured ceiling per axis so utilisation
	// (requests observed / ceiling) is computable in PromQL — the SSR axis's 40%
	// budget alarm needs it, and a quota with no measurable headroom is exactly
	// what 硬上限依賴接入治理規範 forbids.
	rateLimitQuota = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_ratelimit_limit",
			Help: "Configured rate-limit quota (requests per window) per tier and subject kind",
		},
		[]string{"tier", "subject_kind"},
	)
)

// Both gauge series must exist from process start: alert A asks "is the
// mechanism on", and an absent series answers nothing.
func init() {
	originAuthEnabled.WithLabelValues("main").Set(0)
	originAuthEnabled.WithLabelValues("ci").Set(0)
}

// HTTPMetrics returns a Gin middleware that records Prometheus metrics for HTTP requests.
func HTTPMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		// Resolved after c.Next(): by then the limiter has usually memoised it,
		// and for exempt routes (which the limiter early-returns on) this is the
		// resolution.
		_, kind := RateLimitSubject(c)
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status, kind.String(), tierLabel(classifyTier(c.FullPath()))).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
