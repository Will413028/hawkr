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

func TestGetSellerProfile_JoinsPayuniOnStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	payuniRepo := postgres.NewPayuniRepository(pool)
	now := time.Now()

	const accountID int64 = 100
	const storeID int64 = 999

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    storeID,
		StoreName:  "Divergent Incarnation Store",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	require.NoError(t, payuniRepo.Upsert(ctx, &domain.PayuniRegistration{
		ID:            9708080001,
		AccountID:     accountID,
		StoreID:       accountID,
		ApplicationID: 1,
		MerStatus:     "STALE",
		UpdatedAt:     now,
	}))

	profile, err := sellerRepo.GetProfileByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, storeID, profile.Seller.StoreID,
		"profile query must project store_id (999), not leave it defaulted to 0 — RefreshPayuniStatus forwards this field")
	require.Equal(t, "", profile.PayuniMerStatus,
		"join must key on store_id (999, no registration), not account_id (100, matches the STALE row)")
}

func TestListSellersForAdmin_JoinsPayuniOnStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	payuniRepo := postgres.NewPayuniRepository(pool)
	now := time.Now()

	const accountID int64 = 100
	const storeID int64 = 999
	const certID int64 = 3708080001

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    storeID,
		StoreName:  "Divergent Incarnation Store",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))
	require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
		ID:            certID,
		AccountID:     accountID,
		ApplicationNo: "CERT-INTEG-PAYUNI-JOIN-ADMIN",
		Status:        domain.CertStatusForwarded,
		SellerType:    "individual",
		CreatedAt:     now,
		UpdatedAt:     now,
	}))
	require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, accountID, certID))

	require.NoError(t, payuniRepo.Upsert(ctx, &domain.PayuniRegistration{
		ID:            9708080003,
		AccountID:     accountID,
		StoreID:       accountID,
		ApplicationID: certID,
		MerStatus:     "STALE",
		UpdatedAt:     now,
	}))
	require.NoError(t, payuniRepo.Upsert(ctx, &domain.PayuniRegistration{
		ID:            9708080004,
		AccountID:     accountID,
		StoreID:       storeID,
		ApplicationID: certID,
		MerStatus:     "ACTIVE",
		UpdatedAt:     now,
	}))

	rows, err := sellerRepo.ListForAdmin(ctx, domain.SellerListFilter{Limit: 10, Offset: 0})
	require.NoError(t, err)

	var matches []*domain.AdminSellerRow
	for _, r := range rows {
		if r.AccountID == accountID {
			matches = append(matches, r)
		}
	}
	require.Len(t, matches, 1, "seller must appear exactly once — a duplicate means the join matched both the STALE and CURRENT payuni rows")
	require.Equal(t, "ACTIVE", matches[0].PayuniMerStatus,
		"join must key on store_id (CURRENT), not account_id (also matches the STALE row)")
}
