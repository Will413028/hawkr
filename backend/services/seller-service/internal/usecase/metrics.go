package usecase

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var payuniStatusObservationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "payuni_status_observation_total",
	Help: "PAYUNi merchant status observations applied, by source and whether the status actually changed",
}, []string{"source", "changed"})

var payuniSyncAPIErrorTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "payuni_sync_api_error_total",
	Help: "PAYUNi search_merchant_status calls that failed during background reconciliation",
})

var payuniPendingRegistrations = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "payuni_pending_registrations",
	Help: "Pending PAYUNi registrations picked up by the most recent reconciliation tick",
})

func recordPayuniObservation(source string, changed bool) {
	changedLabel := "false"
	if changed {
		changedLabel = "true"
	}
	payuniStatusObservationTotal.WithLabelValues(source, changedLabel).Inc()
}
