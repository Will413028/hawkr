package worker

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/outbox"
)

const (
	cleanupInterval     = 6 * time.Hour
	cleanupInitialDelay = 1 * time.Minute // 避免與其他初始化爭搶資源
	cleanupBatch        = 1000
)

// startCleanupJob 定期清理過期紀錄。
// - push_notifications（failed/skipped）：保留 30 天
// - 已刪除的 inbox（notifications.deleted_at IS NOT NULL）：保留 90 天
func startCleanupJob(lc fx.Lifecycle, pool *pgxpool.Pool, logger *zap.Logger) {
	s := newCleanupScheduler(cleanupInterval, cleanupInitialDelay, pool, logger)
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.Stop(); return nil },
	})
}

// newCleanupScheduler isolates runCleanup behind a panic guard so a bug in
// any single tick can NEVER bring down the whole notification-service
// process. Mirrors seller-service's SellerDeletedOutboxScheduler
// (services/seller-service/internal/worker/seller_deleted_outbox_scheduler.go).
// interval/initialDelay are parameters so tests can drive them without
// waiting on the real 6-hour/1-minute production cadence.
func newCleanupScheduler(interval, initialDelay time.Duration, pool *pgxpool.Pool, logger *zap.Logger) *outbox.Scheduler {
	return outbox.NewScheduler(interval,
		func(ctx context.Context) error {
			runCleanup(ctx, pool, logger)
			return nil
		},
		func(_ context.Context, r any) {
			logger.Error("[cleanup] job panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		nil,
		outbox.WithInitialDelay(initialDelay),
	)
}

func runCleanup(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	deleted := batchExec(ctx, pool, logger,
		"delivery_log",
		`DELETE FROM push_notifications
		 WHERE id IN (
		   SELECT id FROM push_notifications
		   WHERE status IN ('failed', 'skipped') AND created_at < NOW() - INTERVAL '30 days'
		   LIMIT $1
		 )`,
	)
	if deleted > 0 {
		logger.Info("[cleanup] purged delivery log", zap.Int64("rows", deleted))
	}

	deleted = batchExec(ctx, pool, logger,
		"soft_deleted_inbox",
		`DELETE FROM notifications
		 WHERE id IN (
		   SELECT id FROM notifications
		   WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - INTERVAL '90 days'
		   LIMIT $1
		 )`,
	)
	if deleted > 0 {
		logger.Info("[cleanup] purged soft-deleted inbox", zap.Int64("rows", deleted))
	}

	// 殭屍 device_token 回收（背景 backstop）：停用 90 天未更新的 active token。
	//
	// 為何用 updated_at：device_tokens.updated_at 由 UpsertDeviceToken 在 token 註冊時
	// touch，而註冊發生在「App 冷啟動（session 還原）+ 登入 + FCM token 輪替 + 回前景
	// （warm-resume，throttle 註冊）」——即 updated_at ≈「最後一次用 App」。因此「90 天
	// 未更新」= 90 天沒開過 App = 已不活躍，停用安全（dev 實測 90d 命中 0、無誤殺）。
	// 誤停用者於下次開 App / 登入時 Upsert 會 is_active=true 自癒；inbox 為 durable
	// channel，期間漏的是 push banner 非通知本身。常見死 token（UNREGISTERED /
	// SENDER_ID_MISMATCH）在送出失敗時已被 classifier 即時停用；此 sweep 只回收
	// 「從未產生明確失敗訊號」而靜默變陳舊的長尾 token（尤其 iOS：FCM 無自動過期）。
	deactivated := batchExec(ctx, pool, logger,
		"stale_device_tokens",
		`UPDATE device_tokens SET is_active = false
		 WHERE id IN (
		   SELECT id FROM device_tokens
		   WHERE is_active = true AND updated_at < NOW() - INTERVAL '90 days'
		   LIMIT $1
		 )`,
	)
	if deactivated > 0 {
		logger.Info("[cleanup] deactivated stale device tokens", zap.Int64("rows", deactivated))
	}
}

// batchExec 以批次方式執行寫入（DELETE 或 UPDATE），每次最多 cleanupBatch 筆，
// 直到沒有更多符合條件的紀錄（受影響筆數 < cleanupBatch）。query 必須以 $1 接
// LIMIT，且每批執行後條件不再命中已處理的列，否則會無限迴圈。
func batchExec(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger, label, query string) int64 {
	var total int64
	for {
		if ctx.Err() != nil {
			return total
		}
		result, err := pool.Exec(ctx, query, cleanupBatch)
		if err != nil {
			logger.Error("[cleanup] batch exec failed", zap.String("label", label), zap.Error(err))
			return total
		}
		affected := result.RowsAffected()
		total += affected
		if affected < cleanupBatch {
			return total
		}
	}
}
