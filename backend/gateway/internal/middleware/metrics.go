package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
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

	originAuthTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_origin_auth_total",
			Help: "X-HK-Origin-Auth verification outcomes (ok/absent/bad_sig/expired/malformed)",
		},
		[]string{"result"},
	)

	originAuthEnabled = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_origin_auth_enabled",
			Help: "1 = X-HK-Origin-Auth verification active for this key, 0 = disabled (secret unset)",
		},
		[]string{"key"},
	)

	rateLimitQuota = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_ratelimit_limit",
			Help: "Configured rate-limit quota (requests per window) per tier and subject kind",
		},
		[]string{"tier", "subject_kind"},
	)
)

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

		_, kind := RateLimitSubject(c)
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status, kind.String(), tierLabel(classifyTier(c.FullPath()))).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
