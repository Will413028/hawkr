package payuni

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var payuniClientThrottledTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "seller_payuni_client_throttled_total",
	Help: "PAYUNi API calls delayed by the local rate limiter",
})

var payuniClientDegradedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "seller_payuni_client_degraded_total",
	Help: "Times the local PAYUNi rate limit was halved after a suspected upstream block",
})
