package worker

import (
	"context"
	"runtime/debug"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/social-service/internal/domain"
)

// counterReconcileInterval is the cadence at which stale like_count /
// comment_count / reply_count get re-counted from source-of-truth tables.
//
// 🚨 這支是《後端服務責任邊界規範》§二 的**例外**，不是漏網的違規——理由、四欄判準的
// 逐欄對照（有兩欄半落在禁止側）與撤銷條件寫在 36 號設計書 §3.6d 的 `counter.delta` 段。
//
// 一句話版本：計數器於 2026-07-31 刻意改成非同步 MQ delta（避開熱門貼文的 row-lock
// 競爭），而該事件**非冪等**且不做 consumer 端 dedup，這支是那個架構唯一的收斂路徑。
// 它與 §八 歷史教訓刪掉的 HashtagCounterReconciliationJob 形狀相同但處境相反：那支是
// 多餘的（同事務 row-lock 已是真實來源），這支是承重的。**刪它之前先讀 §3.6d。**
const counterReconcileInterval = 10 * time.Minute

// newCounterReconcileScheduler builds the outbox.Scheduler that drives
// reconcile on a fixed cadence, isolating it behind a panic guard so a bug
// in any single tick can NEVER bring down the whole social-service process.
// Mirrors seller-service's SellerDeletedOutboxScheduler
// (services/seller-service/internal/worker/seller_deleted_outbox_scheduler.go).
// interval is a parameter (not the const inlined) so tests can drive it
// without waiting on the real 10-minute cadence; production always passes
// counterReconcileInterval. No WithInitialDelay: preserves the existing
// behavior of waiting a full interval before the first tick.
func newCounterReconcileScheduler(interval time.Duration, postRepo domain.PostRepository, commentRepo domain.CommentRepository, logger *zap.Logger) *outbox.Scheduler {
	return outbox.NewScheduler(interval,
		func(ctx context.Context) error {
			reconcile(ctx, postRepo, commentRepo, logger)
			return nil
		},
		func(_ context.Context, r any) {
			logger.Error("counter reconciler panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		nil,
	)
}

// reconcile runs a periodic job to fix stale like_count / comment_count / reply_count.
// If any Incr/Decr operation fails (network, crash), counters drift from actual data.
// This worker fixes them by re-counting from the source-of-truth tables.
func reconcile(ctx context.Context, postRepo domain.PostRepository, commentRepo domain.CommentRepository, logger *zap.Logger) {
	if err := postRepo.ReconcileLikeCounts(ctx); err != nil {
		logger.Error("reconcile post like_count failed", zap.Error(err))
	}
	if err := postRepo.ReconcileCommentCounts(ctx); err != nil {
		logger.Error("reconcile post comment_count failed", zap.Error(err))
	}
	if err := commentRepo.ReconcileLikeCounts(ctx); err != nil {
		logger.Error("reconcile comment like_count failed", zap.Error(err))
	}
	if err := commentRepo.ReconcileReplyCounts(ctx); err != nil {
		logger.Error("reconcile comment reply_count failed", zap.Error(err))
	}
	logger.Debug("counter reconciliation completed")
}
