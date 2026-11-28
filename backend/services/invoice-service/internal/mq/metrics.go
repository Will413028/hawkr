package mq

import (
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var eventDroppedReasons = []string{
	"unmarshal_failed",
	"empty_order_no",
	"empty_refund_ref",
	"empty_buyer_id",
}

var eventDroppedTypes = []string{
	string(pkgmq.EventTypeOrderPaid),
	string(pkgmq.EventTypeOrderRefundFinalized),
	string(pkgmq.EventTypeOrderCancelled),
}

var eventDropped = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "invoice_event_dropped_total",
	Help: "Inbound events that produced no invoice_inbox_events row at all (58 M30)",
}, []string{"event_type", "reason"})

func init() {
	for _, t := range eventDroppedTypes {
		for _, r := range eventDroppedReasons {
			eventDropped.WithLabelValues(t, r).Add(0)
		}
	}
}

func observeEventDropped(eventType, reason string) {
	if !containsMQString(eventDroppedTypes, eventType) || !containsMQString(eventDroppedReasons, reason) {
		return
	}
	eventDropped.WithLabelValues(eventType, reason).Inc()
}

func containsMQString(vals []string, v string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}
