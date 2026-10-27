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

// GetPublicProfileByStoreIDOrCode backs the public storefront endpoint
// (/public/sellers/:id and its siblings via the resolver in Task 2). Since
// 2026-07-23 the public URL can carry either the seller's store_id (legacy
// links) or store_code (new canonical short code) — both must resolve to the
// same seller row in a single query, no separate lookup step.
func TestGetPublicProfileByStoreIDOrCode_ResolvesStoreCodeAndStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	now := time.Now()

	const (
		storeID   int64 = 2617700001
		storeCode int64 = 2600123456
	)

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  storeID,
		StoreID:    storeID,
		StoreCode:  storeCode,
		StoreName:  "Store",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	t.Run("resolves by store_id (legacy)", func(t *testing.T) {
		got, err := sellerRepo.GetPublicProfileByStoreIDOrCode(ctx, storeID)
		require.NoError(t, err)
		require.Equal(t, storeID, got.Seller.StoreID)
		require.Equal(t, storeCode, got.Seller.StoreCode)
	})

	t.Run("resolves by store_code (canonical)", func(t *testing.T) {
		got, err := sellerRepo.GetPublicProfileByStoreIDOrCode(ctx, storeCode)
		require.NoError(t, err)
		require.Equal(t, storeID, got.Seller.StoreID)
		require.Equal(t, storeCode, got.Seller.StoreCode)
	})

	t.Run("neither value resolves", func(t *testing.T) {
		_, err := sellerRepo.GetPublicProfileByStoreIDOrCode(ctx, 9999999999)
		require.Error(t, err)
	})
}
