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

// TestGetProfileByAccountID_ReflectsFoodBusinessRegistration is a regression
// test for profileRowToDomain silently dropping is_processed_food /
// food_business_reg_no when reconstructing the row into a gen.Seller — the
// SELECT fetched both columns but the manual struct literal never copied
// them over, so every GET /app/sellers/me came back "未登錄" no matter what
// UpdateFoodBusinessRegistration had just written.
func TestGetProfileByAccountID_ReflectsFoodBusinessRegistration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	now := time.Now()

	const accountID int64 = 200

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    accountID,
		StoreName:  "Food Reg Store",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	require.NoError(t, sellerRepo.UpdateFoodBusinessRegistration(
		ctx, accountID, true, "A-123456789-00000-1", now,
	))

	profile, err := sellerRepo.GetProfileByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.True(t, profile.Seller.IsProcessedFood,
		"read path must reflect the write — is_processed_food came back false")
	require.Equal(t, "A-123456789-00000-1", profile.Seller.FoodBusinessRegNo,
		"read path must reflect the write — food_business_reg_no came back empty")
}
