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

func TestListSellersForAdmin_IgnoresStaleRowAfterReonboard(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	payuniRepo := postgres.NewPayuniRepository(pool)
	now := time.Now()

	const accountID int64 = 2705050001
	const oldCertID int64 = 3705050001
	const newCertID int64 = 3705050002

	const healthyAccountID int64 = 2705050002
	const healthyCertID int64 = 3705050003

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    accountID,
		StoreCode:  accountID,
		StoreName:  "Store Before Delete",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))
	require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
		ID:            oldCertID,
		AccountID:     accountID,
		ApplicationNo: "CERT-INTEG-ADMIN-LIST-OLD",
		Status:        domain.CertStatusForwarded,
		SellerType:    "individual",
		CreatedAt:     now,
		UpdatedAt:     now,
	}))
	require.NoError(t, payuniRepo.Upsert(ctx, &domain.PayuniRegistration{
		ID:            9705050001,
		AccountID:     accountID,
		StoreID:       accountID,
		ApplicationID: oldCertID,
		PayuniMerID:   "HAWK2705050001",
		Status:        domain.PayuniStatusActive,
		MerStatus:     "1",
		UpdatedAt:     now,
	}))

	require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
		ID:            newCertID,
		AccountID:     accountID,
		ApplicationNo: "CERT-INTEG-ADMIN-LIST-NEW",
		Status:        domain.CertStatusSubmitted,
		SellerType:    "individual",
		CreatedAt:     now,
		UpdatedAt:     now,
	}))
	require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, accountID, newCertID))

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  healthyAccountID,
		StoreID:    healthyAccountID,
		StoreCode:  healthyAccountID,
		StoreName:  "Store Healthy Binding",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))
	require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
		ID:            healthyCertID,
		AccountID:     healthyAccountID,
		ApplicationNo: "CERT-INTEG-ADMIN-LIST-HEALTHY",
		Status:        domain.CertStatusForwarded,
		SellerType:    "individual",
		CreatedAt:     now,
		UpdatedAt:     now,
	}))
	require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, healthyAccountID, healthyCertID))
	require.NoError(t, payuniRepo.Upsert(ctx, &domain.PayuniRegistration{
		ID:            9705050002,
		AccountID:     healthyAccountID,
		StoreID:       healthyAccountID,
		ApplicationID: healthyCertID,
		PayuniMerID:   "HAWK2705050002",
		Status:        domain.PayuniStatusActive,
		MerStatus:     "1",
		UpdatedAt:     now,
	}))

	rows, err := sellerRepo.ListForAdmin(ctx, domain.SellerListFilter{Limit: 10, Offset: 0})
	require.NoError(t, err)

	var found *domain.AdminSellerRow
	var foundHealthy *domain.AdminSellerRow
	for _, r := range rows {
		if r.AccountID == accountID {
			found = r
		}
		if r.AccountID == healthyAccountID {
			foundHealthy = r
		}
	}
	require.NotNil(t, found, "expected seeded account to appear in admin list")
	require.Equal(t, "", found.PayuniMerStatus, "stale PayUni registration from a superseded application must not surface")

	require.NotNil(t, foundHealthy, "expected healthy-case seeded account to appear in admin list")
	require.Equal(t, "1", foundHealthy.PayuniMerStatus, "PayUni registration matching the seller's current certification must surface normally")
}
