package cron_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/seller-service/internal/app/cron"
)

type fakePIIRepo struct {
	pendingIDs []int64
	clearedIDs []int64
}

func (r *fakePIIRepo) ListPendingPIICleanup(_ context.Context, _ time.Time, _ int) ([]int64, error) {
	return r.pendingIDs, nil
}

func (r *fakePIIRepo) ClearPIIFields(_ context.Context, storeID int64, _ time.Time) error {
	r.clearedIDs = append(r.clearedIDs, storeID)
	return nil
}

func TestSellerPIICleanup_ClearsExpired(t *testing.T) {
	repo := &fakePIIRepo{pendingIDs: []int64{1, 2, 3}}
	job := cron.NewSellerPIICleanupJob(repo, zap.NewNop())
	job.Run(context.Background())
	if len(repo.clearedIDs) != 3 {
		t.Fatalf("expected 3 cleared, got %d", len(repo.clearedIDs))
	}
}

func TestSellerPIICleanup_Idempotent(t *testing.T) {
	// First run clears records.
	repo := &fakePIIRepo{pendingIDs: []int64{10, 11}}
	job := cron.NewSellerPIICleanupJob(repo, zap.NewNop())
	job.Run(context.Background())

	// Second run: no more pending (simulating DB pii_cleared_at IS NULL gate).
	repo.pendingIDs = []int64{}
	job.Run(context.Background())

	// Total cleared should still be 2 (only from first run).
	if len(repo.clearedIDs) != 2 {
		t.Fatalf("expected 2 total cleared, got %d", len(repo.clearedIDs))
	}
}
