package worker

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// sellerDeletedOutboxSchedulerInterval is the cadence at which
// SellerDeletedOutboxWorker.RunOnce fires. Kept at 1 minute (shorter than the
// apple-revoke 5-min cadence) because seller.deleted drives product visibility
// and latency matters.
const sellerDeletedOutboxSchedulerInterval = 1 * time.Minute

// SellerDeletedOutboxScheduler embeds the worker into seller-service's main
// process, running RunOnce on a fixed cadence via fx Lifecycle.
//
// Multi-replica deployments are safe because the inner pkg/outbox.Worker
// claims each row with a lease CAS before any side effect (issue #2294); see
// docs/開發規範/多副本排程搶佔規範.md §一 ②.
type SellerDeletedOutboxScheduler struct {
	inner *outbox.Scheduler
}

// startSellerDeletedOutboxScheduler wires the worker into fx Lifecycle.
// Invoked by worker.Module.
func startSellerDeletedOutboxScheduler(w *SellerDeletedOutboxWorker, lc fx.Lifecycle) *SellerDeletedOutboxScheduler {
	s := &SellerDeletedOutboxScheduler{
		inner: outbox.NewScheduler(sellerDeletedOutboxSchedulerInterval, w.RunOnce,
			func(ctx context.Context, r any) {
				logger.Error(ctx, "seller-deleted-outbox worker panicked",
					zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
			},
			func(ctx context.Context, err error) {
				logger.Warn(ctx, "seller-deleted-outbox run failed", zap.Error(err))
			},
		),
	}
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.inner.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.inner.Stop(); return nil },
	})
	return s
}
