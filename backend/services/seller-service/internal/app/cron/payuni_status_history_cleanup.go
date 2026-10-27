package cron

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// PayuniStatusHistoryCleanupRepository is the minimal interface needed by the job.
type PayuniStatusHistoryCleanupRepository interface {
	DeleteStatusHistoryOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// PayuniStatusHistoryCleanupJob deletes payuni_status_history rows older than
// the retention window.
type PayuniStatusHistoryCleanupJob struct {
	repo   PayuniStatusHistoryCleanupRepository
	logger *zap.Logger
}

// NewPayuniStatusHistoryCleanupJob creates a new cleanup job.
func NewPayuniStatusHistoryCleanupJob(repo PayuniStatusHistoryCleanupRepository, logger *zap.Logger) *PayuniStatusHistoryCleanupJob {
	return &PayuniStatusHistoryCleanupJob{repo: repo, logger: logger}
}

// payuniStatusHistoryRetentionDays mirrors the payuni-webhook-service
// (Java) DataCleanupScheduler pattern for its longest-lived table
// (webhook_inbox, 90d) at double the window since this is the seller-facing
// audit trail of cert review outcomes, not a transient delivery inbox.
// ponytail: arbitrary default, tune once compliance gives a real audit-retention number.
const payuniStatusHistoryRetentionDays = 180

// Run deletes payuni_status_history rows older than the retention window.
func (j *PayuniStatusHistoryCleanupJob) Run(ctx context.Context) {
	before := time.Now().AddDate(0, 0, -payuniStatusHistoryRetentionDays)
	deleted, err := j.repo.DeleteStatusHistoryOlderThan(ctx, before)
	if err != nil {
		j.logger.Error("payuni_status_history_cleanup: delete failed", zap.Error(err))
		return
	}
	j.logger.Info("payuni_status_history_cleanup: done", zap.Int64("deleted", deleted))
}
