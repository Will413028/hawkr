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

func newTestLifecycleRow(t *testing.T, id, accountID int64, when time.Time) *domain.SellerDeletedOutbox {
	t.Helper()
	row, err := domain.NewLifecycleOutboxRow(id, accountID, domain.SellerLifecycleEventDeleted,
		domain.SellerLifecyclePayload{AccountID: accountID, StoreID: accountID}, when)
	require.NoError(t, err)
	return row
}

func TestSellerDeletedOutbox_OldestPendingAge(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerDeletedOutboxRepository(pool)

	_, ok, err := repo.OldestPendingAge(ctx, time.Now())
	require.NoError(t, err)
	require.False(t, ok, "no rows yet: ok must be false")

	older := time.Now().Add(-2 * time.Hour)
	require.NoError(t, repo.Insert(ctx, newTestLifecycleRow(t, 1, 100, older)))
	require.NoError(t, repo.Insert(ctx, newTestLifecycleRow(t, 2, 101, time.Now())))

	age, ok, err := repo.OldestPendingAge(ctx, time.Now())
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 2*time.Hour, age, float64(5*time.Second), "age should reflect the older row, not the newer one")
}

func TestSellerDeletedOutbox_CountPending(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerDeletedOutboxRepository(pool)

	count, err := repo.CountPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	now := time.Now()
	require.NoError(t, repo.Insert(ctx, newTestLifecycleRow(t, 1, 100, now)))
	require.NoError(t, repo.Insert(ctx, newTestLifecycleRow(t, 2, 101, now)))
	require.NoError(t, repo.MarkDone(ctx, 2, now))

	count, err = repo.CountPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count, "only row 1 is still pending")
}

func TestSellerDeletedOutbox_DeleteDoneBefore(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerDeletedOutboxRepository(pool)
	now := time.Now()

	// Row 1: done long ago — eligible for cleanup.
	require.NoError(t, repo.Insert(ctx, newTestLifecycleRow(t, 1, 100, now)))
	require.NoError(t, repo.MarkDone(ctx, 1, now.Add(-60*24*time.Hour)))

	// Row 2: done recently — NOT eligible.
	require.NoError(t, repo.Insert(ctx, newTestLifecycleRow(t, 2, 101, now)))
	require.NoError(t, repo.MarkDone(ctx, 2, now))

	// Row 3: still pending — NOT eligible regardless of age.
	old := now.Add(-60 * 24 * time.Hour)
	require.NoError(t, repo.Insert(ctx, newTestLifecycleRow(t, 3, 102, old)))

	deleted, err := repo.DeleteDoneBefore(ctx, now.Add(-30*24*time.Hour), 500)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	var remaining int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_deleted_outbox").Scan(&remaining))
	require.Equal(t, 2, remaining, "only the old done row should be deleted")
}
