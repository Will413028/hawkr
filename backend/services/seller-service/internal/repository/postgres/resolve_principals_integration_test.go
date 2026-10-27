//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestGetPrincipalsByStoreIDs_Grandfathered(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const accountID int64 = 2614000001
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    accountID,
		StoreName:  "Grandfathered Store",
		SellerType: domain.SellerTypeIndividual,
	}))

	principals, unresolvedIDs, err := repo.GetPrincipalsByStoreIDs(ctx, []int64{accountID})
	require.NoError(t, err)
	require.Equal(t, accountID, principals[accountID],
		"grandfathered store_id must resolve to itself")
	require.NotContains(t, unresolvedIDs, accountID,
		"a genuinely resolved store_id must not appear in unresolvedIDs — "+
			"otherwise the grandfathered case (principal == store_id) would be "+
			"indistinguishable from an echo-back, which is exactly what this "+
			"field exists to separate")
}

func TestGetPrincipalsByStoreIDs_Diverged(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const principalAccountID int64 = 100
	const divergedStoreID int64 = 999
	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, seller_type, created_at, updated_at)
		 VALUES ($1, $2, $2, true, 'Diverged Store', 'individual', now(), now())`,
		principalAccountID, divergedStoreID)
	require.NoError(t, err)

	principals, unresolvedIDs, err := repo.GetPrincipalsByStoreIDs(ctx, []int64{divergedStoreID})
	require.NoError(t, err)
	require.Equal(t, principalAccountID, principals[divergedStoreID],
		"diverged store_id must resolve to its real principal account_id, not itself")
	require.NotContains(t, unresolvedIDs, divergedStoreID,
		"a genuinely resolved store_id must not appear in unresolvedIDs")
}

// TestGetPrincipalsByStoreIDs_UnknownIDEchoesBack locks the contract for an id
// that matches no sellers row at all: it still echoes back in the map (so every
// pre-existing caller behaves exactly as before), AND it is now reported in
// unresolvedIDs so a caller that must not trust an echo can tell.
func TestGetPrincipalsByStoreIDs_UnknownIDEchoesBack(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const unknownID int64 = 424242

	principals, unresolvedIDs, err := repo.GetPrincipalsByStoreIDs(ctx, []int64{unknownID})
	require.NoError(t, err)
	require.Equal(t, unknownID, principals[unknownID],
		"an id with no matching seller row must echo back unchanged")
	require.Contains(t, unresolvedIDs, unknownID,
		"an id with no matching seller row must be reported as unresolved")
}

// TestGetPrincipalsByStoreIDs_SoftDeletedEchoesBack proves a soft-deleted
// sellers row (deleted_at IS NOT NULL) is treated as unknown rather than
// resolved — the query's `AND s.deleted_at IS NULL` join condition must
// actually exclude it, not just the store_id match.
//
// This is the case issue #2789 was opened for: before unresolved_ids existed,
// COALESCE(s.account_id, input.id) handed a soft-deleted store_id back as a
// legal non-zero value with code=0 and no log line anywhere, and the caller had
// no way at all to tell it apart from a real resolution.
func TestGetPrincipalsByStoreIDs_SoftDeletedEchoesBack(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const principalAccountID int64 = 200
	const deletedStoreID int64 = 888
	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, seller_type, created_at, updated_at)
		 VALUES ($1, $2, $2, true, 'Deleted Store', 'individual', now(), now())`,
		principalAccountID, deletedStoreID)
	require.NoError(t, err)

	// status='closed' is required alongside deleted_at by the
	// chk_sellers_deleted_requires_closed CHECK constraint (migration
	// 20260801102806) — a soft-deleted stint is always a closed one.
	_, err = pool.Exec(ctx,
		`UPDATE sellers SET deleted_at = now(), status = 'closed' WHERE store_id = $1`, deletedStoreID)
	require.NoError(t, err)

	principals, unresolvedIDs, err := repo.GetPrincipalsByStoreIDs(ctx, []int64{deletedStoreID})
	require.NoError(t, err)
	require.Equal(t, deletedStoreID, principals[deletedStoreID],
		"a soft-deleted seller row must be treated as unknown and echo back unchanged")
	require.NotEqual(t, principalAccountID, principals[deletedStoreID],
		"a soft-deleted stint must NOT resolve to its account_id")
	require.Contains(t, unresolvedIDs, deletedStoreID,
		"a soft-deleted seller row must be reported as unresolved")
}

// TestGetPrincipalsByStoreIDs_MixedBatch proves the resolved and unresolved
// halves of one batch stay separated — a whole-batch shortcut (report all or
// none) would pass every single-id test above.
func TestGetPrincipalsByStoreIDs_MixedBatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const principalAccountID int64 = 300
	const liveStoreID int64 = 777
	const unknownID int64 = 424243
	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, seller_type, created_at, updated_at)
		 VALUES ($1, $2, $2, true, 'Live Store', 'individual', now(), now())`,
		principalAccountID, liveStoreID)
	require.NoError(t, err)

	principals, unresolvedIDs, err := repo.GetPrincipalsByStoreIDs(ctx, []int64{liveStoreID, unknownID})
	require.NoError(t, err)
	require.Equal(t, principalAccountID, principals[liveStoreID])
	require.Equal(t, unknownID, principals[unknownID])
	require.Equal(t, []int64{unknownID}, unresolvedIDs,
		"only the unknown id belongs in unresolvedIDs")
}
