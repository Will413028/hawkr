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

func TestUpdateRatingByStoreID_UpdatesCurrentStintForDivergedSeller(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	now := time.Now()

	const (
		accountID     int64 = 2613300001
		firstStoreID  int64 = 2613300001
		secondStoreID int64 = 2613300099
	)

	seed := func(storeID int64, name string) {
		require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
			AccountID:  accountID,
			StoreID:    storeID,
			StoreCode:  storeID,
			StoreName:  name,
			Status:     domain.SellerStatusActive,
			SellerType: domain.SellerTypeIndividual,
			CreatedAt:  now,
			UpdatedAt:  now,
		}))
	}

	seed(firstStoreID, "Store A (closed stint)")
	require.NoError(t, sellerRepo.UpdateStatusByStoreID(ctx, firstStoreID, domain.SellerStatusClosed, nil, nil, now))
	require.NoError(t, sellerRepo.RetireCurrentStint(ctx, accountID, now))
	seed(secondStoreID, "Store B (current stint)")

	require.NoError(t, sellerRepo.UpdateRatingByStoreID(ctx, secondStoreID, 4.7))

	got, err := sellerRepo.GetByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, secondStoreID, got.StoreID, "GetByAccountID must resolve the current (second) stint")
	require.Equal(t, 4.7, got.Rating, "rating update must land on the current stint, keyed by store_id")
}
