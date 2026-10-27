//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestGetStoreIDsByAccountID_ListsAllStints(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const accountID int64 = 5500000001
	const closedStoreID int64 = 5500000099
	const currentStoreID int64 = 5500000123

	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, seller_type, status, deleted_at, created_at, updated_at)
		 VALUES ($1, $2, $2, false, 'Closed Prior Stint', 'individual', 'closed', now(), now(), now())`,
		accountID, closedStoreID)
	require.NoError(t, err)

	// Seed stint 2: today's current active stint.
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    currentStoreID,
		StoreCode:  currentStoreID,
		StoreName:  "Current Store",
		SellerType: domain.SellerTypeIndividual,
	}))

	storeIDs, err := repo.GetStoreIDsByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{closedStoreID, currentStoreID}, storeIDs,
		"must list every stint's store_id, including closed/retired ones, with no is_current filter")
}
