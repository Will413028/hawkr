package cron

import (
	"context"
	"time"

	"go.uber.org/zap"
)

const sellerPIICleanupBatchSize = 100

// SellerPIICleanupRepository is the minimal interface needed by the job.
//
// Both methods work on stint PKs (store_id), not login account_ids —
// ListPendingPIICleanup returns store_ids and ClearPIIFields is keyed by
// store_id (see domain.SellerRepository / seller_repository.go). The param was
// named sellerAccountID here until 2026-08-05, diverging from the domain
// interface's own storeID for the identical method.
type SellerPIICleanupRepository interface {
	ListPendingPIICleanup(ctx context.Context, before time.Time, limit int) ([]int64, error)
	ClearPIIFields(ctx context.Context, storeID int64, clearedAt time.Time) error
}

// SellerPIICleanupJob clears PII for sellers soft-deleted more than 30 days ago.
type SellerPIICleanupJob struct {
	repo   SellerPIICleanupRepository
	logger *zap.Logger
}

// NewSellerPIICleanupJob creates a new PII cleanup job.
func NewSellerPIICleanupJob(repo SellerPIICleanupRepository, logger *zap.Logger) *SellerPIICleanupJob {
	return &SellerPIICleanupJob{repo: repo, logger: logger}
}

// Run clears PII for sellers deleted more than 30 days ago.
// Safe to call multiple times — idempotent via pii_cleared_at IS NULL gate in SQL.
func (j *SellerPIICleanupJob) Run(ctx context.Context) {
	before := time.Now().AddDate(0, 0, -30)
	ids, err := j.repo.ListPendingPIICleanup(ctx, before, sellerPIICleanupBatchSize)
	if err != nil {
		j.logger.Error("seller_pii_cleanup: list failed", zap.Error(err))
		return
	}
	now := time.Now()
	for _, storeID := range ids {
		if err := j.repo.ClearPIIFields(ctx, storeID, now); err != nil {
			j.logger.Error("seller_pii_cleanup: clear failed", zap.Int64("store_id", storeID), zap.Error(err))
		}
	}
	j.logger.Info("seller_pii_cleanup: done", zap.Int("cleared", len(ids)))
}
