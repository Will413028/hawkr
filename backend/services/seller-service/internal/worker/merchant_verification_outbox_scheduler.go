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

// merchantVerificationOutboxSchedulerInterval is the cadence at which
// MerchantVerificationOutboxWorker.RunOnce fires. 1 minute — a resubmission
// notice or an approval outcome is user-visible, so keep the poll gap short.
const merchantVerificationOutboxSchedulerInterval = 1 * time.Minute

// MerchantVerificationOutboxScheduler embeds the worker into seller-service's
// main process, running RunOnce on a fixed cadence via fx Lifecycle.
// Multi-replica-safe: pkg/outbox.Worker claims each row with a lease CAS
// before any side effect (docs/開發規範/多副本排程搶佔規範.md §一 ②).
type MerchantVerificationOutboxScheduler struct {
	inner *outbox.Scheduler
}

// startMerchantVerificationOutboxScheduler wires the worker into fx Lifecycle.
// Invoked by worker.Module.
func startMerchantVerificationOutboxScheduler(w *MerchantVerificationOutboxWorker, lc fx.Lifecycle) *MerchantVerificationOutboxScheduler {
	s := &MerchantVerificationOutboxScheduler{
		inner: outbox.NewScheduler(merchantVerificationOutboxSchedulerInterval, w.RunOnce,
			func(ctx context.Context, r any) {
				logger.Error(ctx, "merchant-verification-outbox worker panicked",
					zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
			},
			func(ctx context.Context, err error) {
				logger.Warn(ctx, "merchant-verification-outbox run failed", zap.Error(err))
			},
		),
	}
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.inner.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.inner.Stop(); return nil },
	})
	return s
}
