//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

func TestCertReads_ScopedToCurrentStintAfterReonboard(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	past := time.Now().Add(-2 * time.Hour)

	const accountID int64 = 2704100001
	const oldStoreID int64 = 2704100001
	const newStoreID int64 = 3704100099
	const oldCertID int64 = 3704100001
	const oldDraftID int64 = 3704100002

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    oldStoreID,
		StoreCode:  oldStoreID,
		StoreName:  "Store Before Close",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  past,
		UpdatedAt:  past,
	}))
	require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
		ID: oldCertID, AccountID: accountID, ApplicationNo: "CERT-INTEG-REONBOARD-OLD",
		Status: domain.CertStatusForwarded, SellerType: "individual",
		CreatedAt: past, UpdatedAt: past,
	}))
	require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
		ID: oldDraftID, AccountID: accountID, ApplicationNo: "CERT-INTEG-REONBOARD-DRAFT",
		Status: domain.CertStatusDraft, SellerType: "individual",
		CreatedAt: past, UpdatedAt: past,
	}))
	require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, accountID, oldCertID))

	now := time.Now()
	require.NoError(t, sellerRepo.UpdateStatusByStoreID(ctx, oldStoreID, domain.SellerStatusClosed, &now, &now, now))
	require.NoError(t, sellerRepo.RetireCurrentStint(ctx, accountID, now))
	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    newStoreID,
		StoreCode:  newStoreID,
		StoreName:  "Store After Reonboard",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	_, err := certRepo.GetCurrentApplication(ctx, accountID)
	require.Error(t, err)
	require.True(t, pkgerrors.IsNotFound(err), "expected NotFound after re-onboard, got: %v", err)

	_, err = certRepo.GetDraftApplication(ctx, accountID)
	require.Error(t, err)
	require.True(t, pkgerrors.IsNotFound(err), "expected draft NotFound after re-onboard, got: %v", err)
}
