// seller_deleted_outbox_monitoring.go: refreshes seller_deleted_outbox's
// backlog/staleness metrics and deletes old 'done' rows on a shared
// low-frequency tick. Both operations are cheap and idempotent, so sharing
// sellerDeletedOutboxSchedulerInterval with the worker tick (rather than
// introducing a second interval constant) is deliberate simplicity.
package worker

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// sellerDeletedOutboxRetention bounds how long a 'done' seller_deleted_outbox
// row survives before cleanup deletes it. Pre-launch volume is low; 30 days
// is a conservative default, not a tuned value.
const sellerDeletedOutboxRetention = 30 * 24 * time.Hour

// sellerDeletedOutboxCleanupBatchLimit bounds how many rows a single cleanup
// tick deletes.
const sellerDeletedOutboxCleanupBatchLimit = 500

// SellerDeletedOutboxMonitoringScheduler periodically refreshes
// pending/oldest-age metrics and deletes old 'done' rows.
type SellerDeletedOutboxMonitoringScheduler struct {
	inner *outbox.Scheduler
}

// NewSellerDeletedOutboxMonitoringScheduler wires seller_deleted_outbox's
// outbox.Metrics and cleanup into fx Lifecycle. metrics must be the same
// instance passed to newSellerDeletedOutboxWorker (both are fx-provided
// singletons of *outbox.Metrics via newOutboxMetrics in di.go) — constructing
// a second outbox.Metrics for the same outbox name would panic on duplicate
// Prometheus registration.
func NewSellerDeletedOutboxMonitoringScheduler(repo domain.SellerDeletedOutboxRepository, metrics *outbox.Metrics, lc fx.Lifecycle) *SellerDeletedOutboxMonitoringScheduler {
	run := func(ctx context.Context) error {
		if err := metrics.Refresh(ctx, repo, time.Now()); err != nil {
			return err
		}
		deleted, err := outbox.CleanupDone(ctx, repo, time.Now(), sellerDeletedOutboxRetention, sellerDeletedOutboxCleanupBatchLimit)
		if err != nil {
			return err
		}
		if deleted > 0 {
			logger.Info(ctx, "seller-deleted-outbox cleanup deleted done rows", zap.Int("deleted", deleted))
		}
		return nil
	}

	s := &SellerDeletedOutboxMonitoringScheduler{
		inner: outbox.NewScheduler(sellerDeletedOutboxSchedulerInterval, run,
			func(ctx context.Context, r any) {
				logger.Error(ctx, "seller-deleted-outbox monitoring panicked",
					zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
			},
			func(ctx context.Context, err error) {
				logger.Warn(ctx, "seller-deleted-outbox monitoring tick failed", zap.Error(err))
			},
		),
	}
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.inner.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.inner.Stop(); return nil },
	})
	return s
}
