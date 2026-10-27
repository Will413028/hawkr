package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MonitorRepo is implemented per service to support Metrics.Refresh and
// CleanupDone. It is not generic over Row/T because none of its methods
// return typed rows.
type MonitorRepo interface {
	CountPending(ctx context.Context) (int, error)
	// OldestPendingAge returns the age of the oldest pending row relative to
	// now. ok is false when there are no pending rows.
	OldestPendingAge(ctx context.Context, now time.Time) (age time.Duration, ok bool, err error)
	// DeleteDoneBefore deletes up to limit 'done' rows completed before
	// cutoff, returning the number deleted.
	DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (deleted int, err error)
}

// Metrics holds the standard set of Prometheus metrics every outbox instance
// should expose: current backlog size, staleness of the oldest pending row,
// and cumulative terminal failures. Construct one per outbox table with a
// distinct outboxName; each service registers into its own process-global
// registry, so cross-service name collisions are not a concern — but two
// Metrics with the same outboxName in the same process will panic on the
// second promauto registration.
type Metrics struct {
	pending       prometheus.Gauge
	oldestAge     prometheus.Gauge
	failed        prometheus.Counter
	attemptFailed *prometheus.CounterVec
}

// NewMetrics registers (via promauto — call it exactly once per outboxName
// within a process) the standard outbox metrics, labeled with
// outbox_name=outboxName: outbox_pending_rows, outbox_oldest_pending_age_seconds,
// outbox_failed_total, outbox_attempt_failed_total (further labeled by cause).
func NewMetrics(outboxName string) *Metrics {
	labels := prometheus.Labels{"outbox_name": outboxName}
	return &Metrics{
		pending: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "outbox_pending_rows",
			Help:        "Current count of pending outbox rows",
			ConstLabels: labels,
		}),
		oldestAge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "outbox_oldest_pending_age_seconds",
			Help:        "Age in seconds of the oldest pending outbox row (0 when none pending)",
			ConstLabels: labels,
		}),
		failed: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "outbox_failed_total",
			Help:        "Cumulative count of outbox rows reaching a terminal failed state",
			ConstLabels: labels,
		}),
		attemptFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "outbox_attempt_failed_total",
			Help:        "Cumulative count of failed outbox attempts (any cause, terminal or retried), labeled by cause",
			ConstLabels: labels,
		}, []string{"cause"}),
	}
}

// FailedTotal returns the counter to wire into Hooks.OnTerminalFailed.
func (m *Metrics) FailedTotal() prometheus.Counter { return m.failed }

// AttemptFailedTotal returns the counter for one Hooks.OnAttemptFailed cause
// ("claim", "process", "mark_done", or "update_attempt" — see Hooks'
// doc comment).
func (m *Metrics) AttemptFailedTotal(cause string) prometheus.Counter {
	return m.attemptFailed.WithLabelValues(cause)
}

// Refresh samples repo and updates the pending/oldest-age gauges. Intended to
// be called from a low-frequency Scheduler.
func (m *Metrics) Refresh(ctx context.Context, repo MonitorRepo, now time.Time) error {
	count, err := repo.CountPending(ctx)
	if err != nil {
		return fmt.Errorf("count pending: %w", err)
	}
	m.pending.Set(float64(count))

	age, ok, err := repo.OldestPendingAge(ctx, now)
	if err != nil {
		return fmt.Errorf("oldest pending age: %w", err)
	}
	if !ok {
		m.oldestAge.Set(0)
		return nil
	}
	m.oldestAge.Set(age.Seconds())
	return nil
}

// CleanupDone deletes up to batchLimit 'done' rows older than retention,
// bounding table growth. Intended to be called from a low-frequency
// Scheduler (e.g. alongside Refresh).
func CleanupDone(ctx context.Context, repo MonitorRepo, now time.Time, retention time.Duration, batchLimit int) (int, error) {
	return repo.DeleteDoneBefore(ctx, now.Add(-retention), batchLimit)
}
