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

// TestGetProfileByAccountID_ReflectsWebsiteURL is a regression guard for the
// exact bug class food_registration hit: a SELECT that lists columns
// explicitly (not SELECT *) plus a manual row→domain struct literal is easy
// to update on one side and forget the other, so the write succeeds but the
// read silently comes back empty with no error anywhere.
func TestGetProfileByAccountID_ReflectsWebsiteURL(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	now := time.Now()
	const accountID int64 = 201

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    accountID,
		StoreName:  "Website URL Store",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	m, err := sellerRepo.GetByAccountID(ctx, accountID)
	require.NoError(t, err)
	m.UpdateProfile(m.StoreName, m.Description, m.Logo, m.CoverKey, m.ContactPhone,
		m.City, m.District, m.AddressDetail, m.PostalCode, m.Birthday, m.Gender,
		m.Prize, "https://my-shop.example.com", now)
	require.NoError(t, sellerRepo.Update(ctx, m, nil))

	profile, err := sellerRepo.GetProfileByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, "https://my-shop.example.com", profile.Seller.WebsiteURL,
		"read path must reflect the write — website_url came back empty")

	reread, err := sellerRepo.GetByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, "https://my-shop.example.com", reread.WebsiteURL,
		"GetByAccountID (SELECT *) must also reflect the write")

	// GetProfileByStoreID has its own independent manual gen.Seller{}
	// literal (not a call to profileRowToDomain), so it must be exercised
	// separately — it's the mapper most likely to be missed when adding a
	// new column, since it's easy to think the mapper is already handled
	// after fixing GetProfileByAccountID's read path above.
	profileByStoreID, err := sellerRepo.GetProfileByStoreID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, "https://my-shop.example.com", profileByStoreID.Seller.WebsiteURL,
		"GetProfileByStoreID must also reflect the write — its manual gen.Seller{} literal is independent of profileRowToDomain")
}
