package cron

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// 03:05 Taipei (UTC+8) = 19:05 UTC — 5 minutes after buyer cleanup.
const cleanupHourUTC = 19
const cleanupMinuteUTC = 5

// Module registers cron jobs with the fx lifecycle.
var Module = fx.Module("cron",
	fx.Invoke(startPIICleanupJob),
	fx.Invoke(startPayuniStatusHistoryCleanupJob),
	// B-169 governance
	fx.Invoke(startFoodSafetyPhoneAnonymizeJob),
	fx.Invoke(startSellerStatusRestoreJob),
)

func startPIICleanupJob(lc fx.Lifecycle, repo domain.SellerRepository, logger *zap.Logger) {
	job := NewSellerPIICleanupJob(repo, logger)
	s := outbox.NewDailyScheduler(cleanupHourUTC, cleanupMinuteUTC, "seller_pii_cleanup", job.Run, logger)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.Stop(); return nil },
	})
}

// 19:40 UTC — 10 minutes after food safety phone anonymize.
const payuniStatusHistoryCleanupHourUTC = 19
const payuniStatusHistoryCleanupMinuteUTC = 40

func startPayuniStatusHistoryCleanupJob(lc fx.Lifecycle, repo domain.PayuniRepository, logger *zap.Logger) {
	job := NewPayuniStatusHistoryCleanupJob(repo, logger)
	s := outbox.NewDailyScheduler(payuniStatusHistoryCleanupHourUTC, payuniStatusHistoryCleanupMinuteUTC, "payuni_status_history_cleanup", job.Run, logger)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.Stop(); return nil },
	})
}
