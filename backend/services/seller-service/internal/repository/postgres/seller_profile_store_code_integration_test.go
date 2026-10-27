//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// Finding A (final whole-branch review): the profile-read queries
// (GetSellerProfileByAccountID / GetPublicSellerProfileByStoreIDOrCode /
// GetSellerProfileByStoreID) and their mappers omitted store_code,
// so every seller-profile read silently returned the Go zero-value 0 for a
// real seller's store_code. These tests prove store_code round-trips
// through all three profile-read paths, not just the store-creation /
// GetByAccountID path that toDomainSeller already covered.

func TestGetProfileByAccountID_ReturnsStoreCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const accountID, storeID, storeCode int64 = 4100, 4100, 2612345678
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    storeID,
		StoreCode:  storeCode,
		StoreName:  "Store Code Profile Test",
		SellerType: domain.SellerTypeIndividual,
	}))

	profile, err := repo.GetProfileByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, storeCode, profile.Seller.StoreCode,
		"GetProfileByAccountID must return the seller's real store_code, not the zero value")
}

func TestGetPublicProfileByStoreIDOrCode_ReturnsStoreCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const accountID, storeID, storeCode int64 = 4200, 4200, 2612345679
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    storeID,
		StoreCode:  storeCode,
		StoreName:  "Public Store Code Profile Test",
		SellerType: domain.SellerTypeIndividual,
	}))

	profile, err := repo.GetPublicProfileByStoreIDOrCode(ctx, storeID)
	require.NoError(t, err)
	require.Equal(t, storeCode, profile.Seller.StoreCode,
		"GetPublicProfileByStoreIDOrCode must return the seller's real store_code, not the zero value")
}

func TestGetProfileByStoreID_ReturnsStoreCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const accountID, storeID, storeCode int64 = 4300, 4300, 2612345680
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    storeID,
		StoreCode:  storeCode,
		StoreName:  "Class-3 Store Code Profile Test",
		SellerType: domain.SellerTypeIndividual,
	}))

	profile, err := repo.GetProfileByStoreID(ctx, storeID)
	require.NoError(t, err)
	require.Equal(t, storeCode, profile.Seller.StoreCode,
		"GetProfileByStoreID must return the seller's real store_code, not the zero value")
}
