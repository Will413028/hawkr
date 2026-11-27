package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	trackRemainingMin = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_remaining_min",
		Help: "Minimum remaining invoice-number capacity across all active sellers (58 M9)",
	})

	trackLowSellers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_low_sellers",
		Help: "Sellers whose current-period track balance is below the warn threshold (58 M10)",
	})

	trackExhaustedSellers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_exhausted_sellers",
		Help: "Sellers whose current-period track is exhausted or expired (58 M21)",
	})

	trackNextPeriodInsufficientSellers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_next_period_insufficient_sellers",
		Help: "Sellers whose next-period track allocation is below projected demand (58 M24)",
	})
)

func registerTrackMetrics() {
	trackRemainingMin.Set(0)
	trackLowSellers.Set(0)
	trackExhaustedSellers.Set(0)
	trackNextPeriodInsufficientSellers.Set(0)
}
