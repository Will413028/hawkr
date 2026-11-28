package amego

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var amegoOutcomes = []string{
	outcomeOK,
	outcomeBizError,
	outcomeTransportError,
	outcomeTimeout,
}

const (
	outcomeOK             = "ok"
	outcomeBizError       = "biz_error"
	outcomeTransportError = "transport_error"
	outcomeTimeout        = "timeout"
)

var amegoPreExpandedCodes = []string{"0", CodeOther}

var amegoRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "invoice_amego_request_duration_seconds",
	Help:    "AMEGO round-trip latency by endpoint and outcome (58 M2)",
	Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 20, 30},
}, []string{"endpoint", "outcome"})

var amegoResponseCode = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "invoice_amego_response_code_total",
	Help: "AMEGO response codes, whitelisted (unknown codes collapse to \"other\") (58 M3)",
}, []string{"endpoint", "code"})

var amegoRateLimited = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "invoice_amego_rate_limited_total",
	Help: "Rate limit hits; source=amego means AMEGO rejected us, source=local means our own limiter did (58 M4)",
}, []string{"source"})

var amegoAuditWriteFailed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_audit_write_failed_total",
	Help: "Provider audit-log writes that failed (the only visible form of the swallowed defer) (58 M14)",
})

func observedEndpoints() []string {
	all := Endpoints()
	out := make([]string, 0, len(all))
	for _, ep := range all {
		if ep == EndpointServerTime {
			continue
		}
		out = append(out, ep)
	}
	return out
}

func init() {
	for _, ep := range observedEndpoints() {
		for _, oc := range amegoOutcomes {
			amegoRequestDuration.WithLabelValues(ep, oc)
		}
		for _, code := range amegoPreExpandedCodes {
			amegoResponseCode.WithLabelValues(ep, code).Add(0)
		}
	}
	for _, src := range []string{rateLimitSourceAMEGO, rateLimitSourceLocal} {
		amegoRateLimited.WithLabelValues(src).Add(0)
	}
	amegoAuditWriteFailed.Add(0)
}

const (
	rateLimitSourceAMEGO = "amego"
	rateLimitSourceLocal = "local"
)

const amegoRateLimitCode = "21"

func observeCall(endpoint string, apiErr *APIError, seconds float64) {
	outcome := outcomeOK
	code := "0"
	if apiErr != nil {
		outcome = outcomeFor(apiErr)
		code = apiErr.MetricCodeLabel()
	}
	amegoRequestDuration.WithLabelValues(endpoint, outcome).Observe(seconds)
	amegoResponseCode.WithLabelValues(endpoint, code).Inc()
	if apiErr != nil && apiErr.Code == amegoRateLimitCode {
		amegoRateLimited.WithLabelValues(rateLimitSourceAMEGO).Inc()
	}
}

func outcomeFor(e *APIError) string {
	switch e.Kind {
	case ErrorKindTimeout:
		return outcomeTimeout
	case ErrorKindProvider:
		return outcomeBizError
	case ErrorKindConn, ErrorKindDecode:
		return outcomeTransportError
	}
	return outcomeTransportError
}

func observeLocalRateLimited() {
	amegoRateLimited.WithLabelValues(rateLimitSourceLocal).Inc()
}

// ObserveAuditWriteFailed 遞增 M14。
//
// 匯出是因為唯一的呼叫端在 internal/client（repoAuditSink.logFail）——它 import
// amego，反向不成立。這也是這支指標從宣告到今天從未被遞增過的原因：它原本被放在
// internal/worker 上，而那個 package 在依賴圖的另一邊。
func ObserveAuditWriteFailed() { amegoAuditWriteFailed.Inc() }
