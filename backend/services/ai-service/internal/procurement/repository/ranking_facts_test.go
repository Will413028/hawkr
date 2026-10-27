//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestRankingFactsRepository_UpsertOrderCompleted_DedupsByEventID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := NewRankingFactsRepository(pool, database.NewTxManager(pool))
	storeID := identity.StoreID(700001)

	require.NoError(t, repo.UpsertOrderCompleted(ctx, storeID, "2026-08", "evt-1"))
	require.NoError(t, repo.UpsertOrderCompleted(ctx, storeID, "2026-08", "evt-1")) // redelivery

	facts, err := repo.BatchGet(ctx, []identity.StoreID{storeID})
	require.NoError(t, err)
	require.Equal(t, 1, facts[storeID].MonthlyCompletedOrders, "duplicate event_id must not double-count")
}

func TestRankingFactsRepository_UpsertOrderCompleted_ResetsOnNewPeriod(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := NewRankingFactsRepository(pool, database.NewTxManager(pool))
	storeID := identity.StoreID(700002)

	require.NoError(t, repo.UpsertOrderCompleted(ctx, storeID, "2026-08", "evt-1"))
	require.NoError(t, repo.UpsertOrderCompleted(ctx, storeID, "2026-08", "evt-2"))
	require.NoError(t, repo.UpsertOrderCompleted(ctx, storeID, "2026-09", "evt-3")) // new month

	facts, err := repo.BatchGet(ctx, []identity.StoreID{storeID})
	require.NoError(t, err)
	require.Equal(t, 1, facts[storeID].MonthlyCompletedOrders, "new period must reset the counter, not add to it")
}

func TestRankingFactsRepository_UpsertTier_StaleRedeliveryDoesNotOverwriteNewerValue(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := NewRankingFactsRepository(pool, database.NewTxManager(pool))
	storeID := identity.StoreID(700003)

	older := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

	require.NoError(t, repo.UpsertTier(ctx, storeID, 2, 1000, newer))
	// Simulates a retry/requeue redelivering an older seller.membership_changed
	// event after the newer one already landed — must be a no-op, not "後到覆蓋前到".
	require.NoError(t, repo.UpsertTier(ctx, storeID, 0, 50, older))

	facts, err := repo.BatchGet(ctx, []identity.StoreID{storeID})
	require.NoError(t, err)
	require.Equal(t, 2, facts[storeID].TierRank, "a stale (older-effectiveAt) redelivery must not revert an already-applied newer tier")
	require.NotNil(t, facts[storeID].MonthlyOrderCap)
	require.Equal(t, 1000, *facts[storeID].MonthlyOrderCap)
}

func TestRankingFactsRepository_UpsertTier_NewerEventOverwritesOlderValue(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := NewRankingFactsRepository(pool, database.NewTxManager(pool))
	storeID := identity.StoreID(700004)

	older := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

	require.NoError(t, repo.UpsertTier(ctx, storeID, 2, 1000, older))
	require.NoError(t, repo.UpsertTier(ctx, storeID, 0, 50, newer))

	facts, err := repo.BatchGet(ctx, []identity.StoreID{storeID})
	require.NoError(t, err)
	require.Equal(t, 0, facts[storeID].TierRank, "a genuinely newer event must apply normally")
	require.NotNil(t, facts[storeID].MonthlyOrderCap)
	require.Equal(t, 50, *facts[storeID].MonthlyOrderCap)
}

func TestRankingFactsRepository_UpsertTier_SameEffectiveAtRedeliveryIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := NewRankingFactsRepository(pool, database.NewTxManager(pool))
	storeID := identity.StoreID(700005)

	at := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

	require.NoError(t, repo.UpsertTier(ctx, storeID, 1, 200, at))
	require.NoError(t, repo.UpsertTier(ctx, storeID, 1, 200, at)) // exact redelivery, same effectiveAt

	facts, err := repo.BatchGet(ctx, []identity.StoreID{storeID})
	require.NoError(t, err)
	require.Equal(t, 1, facts[storeID].TierRank)
	require.NotNil(t, facts[storeID].MonthlyOrderCap)
	require.Equal(t, 200, *facts[storeID].MonthlyOrderCap)
}
