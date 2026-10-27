//go:build integration

package postgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func seedPayuniActive(ctx context.Context, t *testing.T, pool *pgxpool.Pool, accountID, storeID, certID int64, status domain.PayuniStatus) {
	t.Helper()
	require.NoError(t, postgres.NewPayuniRepository(pool).Upsert(ctx, &domain.PayuniRegistration{
		ID:            storeID,
		AccountID:     accountID,
		StoreID:       storeID,
		ApplicationID: certID,
		PayuniMerID:   "HAWK" + strconv.FormatInt(storeID, 10),
		Status:        status,
		UpdatedAt:     time.Now(),
	}))
}

func TestGetByStoreIDs_OnlyReturnsApprovedCertifiedSellers(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	now := time.Now()

	const (
		approvedAccountID  int64 = 2613100001
		forwardedAccountID int64 = 2613100002
		noCertAccountID    int64 = 2613100003

		approvedStoreID  int64 = 7613100001000000001
		forwardedStoreID int64 = 7613100002000000002
		noCertStoreID    int64 = 7613100003000000003
	)

	seedSeller := func(accountID, storeID int64) {
		require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
			AccountID:  accountID,
			StoreID:    storeID,
			StoreCode:  accountID,
			StoreName:  "Store",
			Status:     domain.SellerStatusActive,
			SellerType: domain.SellerTypeIndividual,
			CreatedAt:  now,
			UpdatedAt:  now,
		}))
	}

	seedCertAndLink := func(accountID, certID int64, status domain.CertificationStatus) {
		require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
			ID:            certID,
			AccountID:     accountID,
			ApplicationNo: "CERT-INTEG-" + strconv.FormatInt(certID, 10),
			Status:        status,
			SellerType:    "individual",
			CreatedAt:     now,
			UpdatedAt:     now,
		}))
		require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, accountID, certID))
	}

	seedSeller(approvedAccountID, approvedStoreID)
	seedCertAndLink(approvedAccountID, 3613100001, domain.CertStatusForwarded)
	seedPayuniActive(ctx, t, pool, approvedAccountID, approvedStoreID, 3613100001, domain.PayuniStatusActive)

	seedSeller(forwardedAccountID, forwardedStoreID)
	seedCertAndLink(forwardedAccountID, 3613100002, domain.CertStatusForwarded)
	seedPayuniActive(ctx, t, pool, forwardedAccountID, forwardedStoreID, 3613100002, domain.PayuniStatusPending)

	seedSeller(noCertAccountID, noCertStoreID)

	got, err := sellerRepo.GetByStoreIDs(ctx, []int64{approvedStoreID, forwardedStoreID, noCertStoreID})
	require.NoError(t, err)

	require.Len(t, got, 1, "only the PayUni-approved seller should be returned")
	require.Equal(t, approvedStoreID, got[0].StoreID)
	require.Equal(t, approvedAccountID, got[0].AccountID)

	byAccount, err := sellerRepo.GetByStoreIDs(ctx, []int64{approvedAccountID, forwardedAccountID, noCertAccountID})
	require.NoError(t, err)
	require.Empty(t, byAccount, "GetByStoreIDs must key on store_id only — account_ids must resolve to nothing")
}

func TestRecommendedSurface_ExcludesNonActiveSellers(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	now := time.Now()

	const (
		activeSellerID int64 = 2613200001
		closedSellerID int64 = 2613200002
	)

	seedApprovedSeller := func(accountID, certID int64) {
		require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
			AccountID:  accountID,
			StoreID:    accountID,
			StoreCode:  accountID,
			StoreName:  "Store",
			Status:     domain.SellerStatusActive,
			SellerType: domain.SellerTypeIndividual,
			CreatedAt:  now,
			UpdatedAt:  now,
		}))
		require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
			ID:            certID,
			AccountID:     accountID,
			ApplicationNo: "CERT-INTEG-" + strconv.FormatInt(certID, 10),
			Status:        domain.CertStatusForwarded,
			SellerType:    "individual",
			CreatedAt:     now,
			UpdatedAt:     now,
		}))
		require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, accountID, certID))
		seedPayuniActive(ctx, t, pool, accountID, accountID, certID, domain.PayuniStatusActive)
	}

	seedApprovedSeller(activeSellerID, 3613200001)
	seedApprovedSeller(closedSellerID, 3613200002)

	require.NoError(t, sellerRepo.UpdateStatusCurrentByAccountID(ctx, closedSellerID, domain.SellerStatusClosed, now))

	got, err := sellerRepo.GetByStoreIDs(ctx, []int64{activeSellerID, closedSellerID})
	require.NoError(t, err)
	require.Len(t, got, 1, "GetByStoreIDs must exclude the non-active (closed, deleted_at NULL) seller")
	require.Equal(t, activeSellerID, got[0].AccountID)

	fallback, err := sellerRepo.GetApprovedExcluding(ctx, []int64{}, domain.UnknownPublicProductStores(), 10)
	require.NoError(t, err)
	for _, s := range fallback {
		require.NotEqual(t, closedSellerID, s.AccountID, "GetApprovedExcluding must not surface the closed seller")
	}
	require.Contains(t, accountIDsOf(fallback), activeSellerID, "the active approved seller should still be eligible for fallback")
}

func accountIDsOf(sellers []*domain.Seller) []int64 {
	ids := make([]int64, 0, len(sellers))
	for _, s := range sellers {
		ids = append(ids, s.AccountID)
	}
	return ids
}

func TestRecommendedSurface_HydratesStoreCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	now := time.Now()

	const (
		sellerID  int64 = 2613400001
		certID    int64 = 3613400001
		storeCode int64 = 2699000001
	)

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  sellerID,
		StoreID:    sellerID,
		StoreCode:  storeCode,
		StoreName:  "Store",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))
	require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
		ID:            certID,
		AccountID:     sellerID,
		ApplicationNo: "CERT-INTEG-" + strconv.FormatInt(certID, 10),
		Status:        domain.CertStatusForwarded,
		SellerType:    "individual",
		CreatedAt:     now,
		UpdatedAt:     now,
	}))
	require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, sellerID, certID))
	seedPayuniActive(ctx, t, pool, sellerID, sellerID, certID, domain.PayuniStatusActive)

	got, err := sellerRepo.GetByStoreIDs(ctx, []int64{sellerID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, storeCode, got[0].StoreCode, "GetByStoreIDs must hydrate store_code")
	require.Equal(t, sellerID, got[0].StoreID, "store_id must not be displaced by the added column")

	fallback, err := sellerRepo.GetApprovedExcluding(ctx, []int64{}, domain.UnknownPublicProductStores(), 10)
	require.NoError(t, err)
	var seen bool
	for _, s := range fallback {
		if s.StoreID == sellerID {
			seen = true
			require.Equal(t, storeCode, s.StoreCode, "GetApprovedExcluding must hydrate store_code")
		}
	}
	require.True(t, seen, "seeded seller should be eligible for fallback top-up")
}

func TestGetApprovedExcluding_FiltersToStoresWithPublicProducts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	now := time.Now()

	const (
		withProductsHigh int64 = 2614300003
		emptyStore       int64 = 2614300002
		withProductsLow  int64 = 2614300001
	)

	seedApproved := func(accountID, certID int64) {
		require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
			AccountID:  accountID,
			StoreID:    accountID,
			StoreCode:  accountID,
			StoreName:  "Store",
			Status:     domain.SellerStatusActive,
			SellerType: domain.SellerTypeIndividual,
			CreatedAt:  now,
			UpdatedAt:  now,
		}))
		require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
			ID:            certID,
			AccountID:     accountID,
			ApplicationNo: "CERT-INTEG-" + strconv.FormatInt(certID, 10),
			Status:        domain.CertStatusForwarded,
			SellerType:    "individual",
			CreatedAt:     now,
			UpdatedAt:     now,
		}))
		require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, accountID, certID))
		seedPayuniActive(ctx, t, pool, accountID, accountID, certID, domain.PayuniStatusActive)
	}

	seedApproved(withProductsLow, 3614300001)
	seedApproved(emptyStore, 3614300002)
	seedApproved(withProductsHigh, 3614300003)

	hasProducts := domain.NewPublicProductStores([]int64{withProductsHigh, withProductsLow})

	got, err := sellerRepo.GetApprovedExcluding(ctx, []int64{}, hasProducts, 10)
	require.NoError(t, err)
	ids := storeIDsOfSellers(got)
	require.Contains(t, ids, withProductsHigh)
	require.Contains(t, ids, withProductsLow)
	require.NotContains(t, ids, emptyStore, "fallback top-up must not surface a store with zero public products")

	got, err = sellerRepo.GetApprovedExcluding(ctx, []int64{}, domain.UnknownPublicProductStores(), 10)
	require.NoError(t, err)
	require.Contains(t, storeIDsOfSellers(got), emptyStore, "an unresolved filter must not filter anything")

	got, err = sellerRepo.GetApprovedExcluding(ctx, []int64{withProductsHigh}, hasProducts, 1)
	require.NoError(t, err)
	require.Len(t, got, 1, "LIMIT must count only rows that passed the product predicate")
	require.Equal(t, withProductsLow, got[0].StoreID)

	got, err = sellerRepo.GetApprovedExcluding(ctx, []int64{}, domain.NewPublicProductStores(nil), 10)
	require.NoError(t, err)
	require.Empty(t, got, "a resolved-empty set must filter everything out (it is an answer, not a missing answer)")
}

func storeIDsOfSellers(sellers []*domain.Seller) []int64 {
	ids := make([]int64, 0, len(sellers))
	for _, s := range sellers {
		ids = append(ids, s.StoreID)
	}
	return ids
}
