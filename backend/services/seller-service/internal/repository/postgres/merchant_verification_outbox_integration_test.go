//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

func newTestVerificationOutboxRow(t *testing.T, id, accountID, storeID int64, eventType string, when time.Time) *domain.MerchantVerificationOutbox {
	t.Helper()
	row, err := domain.NewMerchantVerificationOutboxRow(id, accountID, storeID, eventType, "", when)
	require.NoError(t, err)
	return row
}

func TestMerchantVerificationOutbox_InsertAndFetchPendingDue(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewMerchantVerificationOutboxRepository(pool)
	now := time.Now()

	require.NoError(t, repo.Insert(ctx, newTestVerificationOutboxRow(t, 1, 100, 200, "approved", now)))

	rows, err := repo.FetchPendingDue(ctx, now.Add(time.Second), 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "approved", rows[0].EventType)
	require.Equal(t, int64(100), rows[0].AccountID)
}

func TestMerchantVerificationOutbox_ClaimGatesConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewMerchantVerificationOutboxRepository(pool)
	now := time.Now()
	require.NoError(t, repo.Insert(ctx, newTestVerificationOutboxRow(t, 1, 100, 200, "rejected", now)))

	claimableBefore := now.Add(time.Second)
	affected, err := repo.Claim(ctx, 1, now.Add(time.Minute), claimableBefore)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected, "first claim must succeed")

	affected, err = repo.Claim(ctx, 1, now.Add(time.Minute), claimableBefore)
	require.NoError(t, err)
	require.Equal(t, int64(0), affected, "second claim on an already-leased row must report 0 rows affected")
}

func TestMerchantVerificationOutbox_CountPendingAndDeleteDoneBefore(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewMerchantVerificationOutboxRepository(pool)
	now := time.Now()
	require.NoError(t, repo.Insert(ctx, newTestVerificationOutboxRow(t, 1, 100, 200, "approved", now)))
	require.NoError(t, repo.Insert(ctx, newTestVerificationOutboxRow(t, 2, 101, 201, "rejected", now)))
	require.NoError(t, repo.MarkDone(ctx, 2, now.Add(-60*24*time.Hour)))

	count, err := repo.CountPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	deleted, err := repo.DeleteDoneBefore(ctx, now.Add(-30*24*time.Hour), 500)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
}
