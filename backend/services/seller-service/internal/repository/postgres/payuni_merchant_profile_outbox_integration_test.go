//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

func newTestProfileOutboxRow(t *testing.T, id, storeID, accountID int64, merAddr, post string, when time.Time) *domain.PayuniMerchantProfileOutbox {
	t.Helper()
	row, err := domain.NewPayuniMerchantProfileOutboxRow(id, storeID, accountID, merAddr, post, when)
	require.NoError(t, err)
	return row
}

func TestPayuniMerchantProfileOutbox_InsertFetchClaimDone(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPayuniMerchantProfileOutboxRepository(pool)
	now := time.Now()

	require.NoError(t, repo.Insert(ctx, newTestProfileOutboxRow(t, 9001, 9002, 9003, "台北市信義區松高路1號", "110", now)))

	due, err := repo.FetchPendingDue(ctx, now.Add(time.Second), 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, int64(9002), due[0].StoreID)
	require.Equal(t, int64(9003), due[0].AccountID)

	// payload 必須原樣往返：worker 只認得這兩個 key。
	var p domain.PayuniMerchantProfilePayload
	require.NoError(t, json.Unmarshal(due[0].Payload, &p))
	require.Equal(t, "台北市信義區松高路1號", p.MerAddr)
	require.Equal(t, "110", p.MerAddrPost)

	// Claim 是跨 replica 的互斥點：第一次拿得到，第二次拿不到。
	claimableBefore := now.Add(time.Second)
	affected, err := repo.Claim(ctx, 9001, now.Add(time.Minute), claimableBefore)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected, "first claim must succeed")

	affected, err = repo.Claim(ctx, 9001, now.Add(2*time.Minute), claimableBefore)
	require.NoError(t, err)
	require.Equal(t, int64(0), affected, "second claim on an already-leased row must report 0 rows affected")

	require.NoError(t, repo.MarkDone(ctx, 9001, now))
	after, err := repo.FetchPendingDue(ctx, now.Add(time.Hour), 10)
	require.NoError(t, err)
	require.Empty(t, after, "done row must not be returned as pending")
}

func TestPayuniMerchantProfileOutbox_CountPendingOldestAgeAndCleanup(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPayuniMerchantProfileOutboxRepository(pool)
	now := time.Now()

	// 空佇列不是故障：OldestPendingAge 回 ok=false 而不是 error。
	_, ok, err := repo.OldestPendingAge(ctx, now)
	require.NoError(t, err)
	require.False(t, ok, "empty queue must report ok=false, not an error")

	require.NoError(t, repo.Insert(ctx, newTestProfileOutboxRow(t, 9101, 9102, 9103, "A", "100", now.Add(-2*time.Hour))))
	require.NoError(t, repo.Insert(ctx, newTestProfileOutboxRow(t, 9111, 9112, 9113, "B", "110", now.Add(-time.Hour))))

	count, err := repo.CountPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	age, ok, err := repo.OldestPendingAge(ctx, now)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, age, 90*time.Minute, "oldest pending age must come from the older row")

	require.NoError(t, repo.MarkDone(ctx, 9101, now.Add(-90*time.Minute)))
	deleted, err := repo.DeleteDoneBefore(ctx, now.Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	count, err = repo.CountPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count, "cleanup must not touch pending rows")
}

func TestPayuniMerchantProfileOutbox_UpdateAttemptAndMarkFailed(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPayuniMerchantProfileOutboxRepository(pool)
	now := time.Now()
	require.NoError(t, repo.Insert(ctx, newTestProfileOutboxRow(t, 9201, 9202, 9203, "A", "100", now)))

	require.NoError(t, repo.UpdateAttempt(ctx, 9201, 2, now.Add(5*time.Minute), "AGENT07031"))
	due, err := repo.FetchPendingDue(ctx, now.Add(10*time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, 2, due[0].AttemptCount)
	require.Equal(t, "AGENT07031", due[0].LastError)

	require.NoError(t, repo.MarkFailed(ctx, 9201, "gave up", now))
	after, err := repo.FetchPendingDue(ctx, now.Add(time.Hour), 10)
	require.NoError(t, err)
	require.Empty(t, after, "failed row must not be retried as pending")
}
