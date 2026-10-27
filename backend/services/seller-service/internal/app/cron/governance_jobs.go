package cron

import (
	"context"
	"runtime/debug"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// B-169 Governance cron jobs (issue #722).
//
//   - foodSafetyPhoneAnonymize: every day 19:30 UTC (03:30 Taipei) — 25 mins
//     after seller PII cleanup.
//   - sellerStatusRestore: every 10 min — restore sellers.status=active for
//     expired full_suspension violations (D11 / task 9.2).

const (
	foodSafetyAnonymizeHourUTC   = 19
	foodSafetyAnonymizeMinuteUTC = 30

	sellerStatusRestoreInterval = 10 * time.Minute
)

// startFoodSafetyPhoneAnonymizeJob registers daily phone anonymize + OSS evidence cleanup.
// Wraps FoodSafetyReportUseCase.AnonymizeOldPhones with daily scheduling via
// pkg/outbox.DailyScheduler (same fixed-time-of-day component this file's
// sibling jobs in di.go use).
func startFoodSafetyPhoneAnonymizeJob(lc fx.Lifecycle, uc *usecase.FoodSafetyReportUseCase, logger *zap.Logger) {
	s := outbox.NewDailyScheduler(foodSafetyAnonymizeHourUTC, foodSafetyAnonymizeMinuteUTC, "food_safety_phone_anonymize", func(ctx context.Context) {
		n, err := uc.AnonymizeOldPhones(ctx)
		if err != nil {
			logger.Error("food_safety_phone_anonymize failed", zap.Error(err))
			return
		}
		logger.Info("food_safety_phone_anonymize done", zap.Int("rows", n))
	}, logger)
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.Stop(); return nil },
	})
}

// startSellerStatusRestoreJob registers the 10-minute interval restore for
// expired full_suspension violations, isolated behind a panic guard so a bug
// in any single tick can NEVER bring down the whole seller-service process.
// Mirrors seller-service's own SellerDeletedOutboxScheduler
// (services/seller-service/internal/worker/seller_deleted_outbox_scheduler.go).
// WithInitialDelay(0) preserves the existing "run once on startup" behavior
// (catches suspensions that expired during downtime).
// 與 admin manual revoke 互補：cron 為 fail-safe，避免 admin 沒留意 expires_at 過期。
func startSellerStatusRestoreJob(lc fx.Lifecycle, uc *usecase.SellerViolationUseCase, logger *zap.Logger) {
	s := newSellerStatusRestoreScheduler(sellerStatusRestoreInterval, uc, logger)
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.Stop(); return nil },
	})
}

func newSellerStatusRestoreScheduler(interval time.Duration, uc *usecase.SellerViolationUseCase, logger *zap.Logger) *outbox.Scheduler {
	return outbox.NewScheduler(interval,
		func(ctx context.Context) error {
			runSellerStatusRestore(ctx, uc, logger)
			return nil
		},
		func(_ context.Context, r any) {
			logger.Error("cron job panicked",
				zap.String("job", "seller_status_restore"),
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		nil,
		outbox.WithInitialDelay(0),
	)
}

// runSellerStatusRestore single iteration body — query expired violations + restore sellers.status=active.
//
// 同 seller 多筆過期 full_suspension 並存時：取最大 expires_at 為基準（已過期則還原），
// 由 in-memory pass 處理（query 已 ORDER BY seller_id, expires_at DESC）。
func runSellerStatusRestore(ctx context.Context, uc *usecase.SellerViolationUseCase, logger *zap.Logger) {
	violations, err := uc.ListExpiredFullSuspensions(ctx)
	if err != nil {
		logger.Error("seller_status_restore: list failed", zap.Error(err))
		return
	}
	now := time.Now()
	seen := make(map[int64]bool, len(violations)) // 同 seller 只處理一次
	restored := 0
	for _, v := range violations {
		if seen[v.SellerID] {
			continue
		}
		seen[v.SellerID] = true
		if v.ExpiresAt == nil || v.ExpiresAt.After(now) {
			continue // 還沒到期（query 已 filter，理論上不會到這）
		}
		if err := uc.RestoreSellerStatusActive(ctx, v.SellerID); err != nil {
			logger.Error("seller_status_restore: update failed",
				zap.Int64("seller_id", v.SellerID),
				zap.Int64("violation_id", v.ID),
				zap.Error(err))
			continue
		}
		restored++
	}
	if restored > 0 {
		logger.Info("seller_status_restore: done", zap.Int("restored", restored))
	}
}
