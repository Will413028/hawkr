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

const merchantVerificationOutboxRetention = 30 * 24 * time.Hour
const merchantVerificationOutboxCleanupBatchLimit = 500

type MerchantVerificationOutboxMonitoringScheduler struct {
	inner *outbox.Scheduler
}

func NewMerchantVerificationOutboxMonitoringScheduler(repo domain.MerchantVerificationOutboxRepository, metrics *outbox.Metrics, lc fx.Lifecycle) *MerchantVerificationOutboxMonitoringScheduler {
	run := func(ctx context.Context) error {
		if err := metrics.Refresh(ctx, repo, time.Now()); err != nil {
			return err
		}
		deleted, err := outbox.CleanupDone(ctx, repo, time.Now(), merchantVerificationOutboxRetention, merchantVerificationOutboxCleanupBatchLimit)
		if err != nil {
			return err
		}
		if deleted > 0 {
			logger.Info(ctx, "merchant-verification-outbox cleanup deleted done rows", zap.Int("deleted", deleted))
		}
		return nil
	}

	s := &MerchantVerificationOutboxMonitoringScheduler{
		inner: outbox.NewScheduler(merchantVerificationOutboxSchedulerInterval, run,
			func(ctx context.Context, r any) {
				logger.Error(ctx, "merchant-verification-outbox monitoring panicked",
					zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
			},
			func(ctx context.Context, err error) {
				logger.Warn(ctx, "merchant-verification-outbox monitoring tick failed", zap.Error(err))
			},
		),
	}
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.inner.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.inner.Stop(); return nil },
	})
	return s
}
