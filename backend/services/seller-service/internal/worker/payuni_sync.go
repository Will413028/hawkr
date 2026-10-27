package worker

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/zap"
)

const payuniSyncInterval = 15 * time.Minute

type PayuniSyncWorker struct {
	payuniUC  *usecase.PayuniUseCase
	scheduler *outbox.Scheduler
}

func NewPayuniSyncWorker(payuniUC *usecase.PayuniUseCase) *PayuniSyncWorker {
	return &PayuniSyncWorker{payuniUC: payuniUC}
}

func (w *PayuniSyncWorker) Start(ctx context.Context) {
	w.scheduler = outbox.NewScheduler(payuniSyncInterval,
		func(ctx context.Context) error {
			return w.payuniUC.SyncPendingStatuses(ctx)
		},
		func(ctx context.Context, r any) {
			logger.Error(ctx, "PAYUNi periodic sync panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		func(ctx context.Context, err error) {
			logger.Error(ctx, "PAYUNi periodic sync failed", zap.Error(err))
		},
		outbox.WithInitialDelay(0),
	)
	w.scheduler.Start(ctx)
}

func (w *PayuniSyncWorker) Stop() {
	if w.scheduler != nil {
		w.scheduler.Stop()
	}
}
