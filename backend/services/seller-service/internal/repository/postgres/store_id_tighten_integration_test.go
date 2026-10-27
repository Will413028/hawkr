//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestCreateSeller_WritesStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const accountID int64 = 2612345679
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    accountID,
		StoreName:  "Test Store",
		SellerType: domain.SellerTypeIndividual,
	}))

	var storeID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT store_id FROM sellers WHERE account_id = $1`, accountID).
		Scan(&storeID))
	require.Equal(t, accountID, storeID,
		"sellers.store_id must grandfather := account_id")
}

// TestSellersStoreID_NotNullEnforced proves the migration actually
// tightened the column — a direct INSERT that omits store_id must be
// rejected by the DB, not silently accepted as NULL.
func TestSellersStoreID_NotNullEnforced(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_name) VALUES ($1, 'No Seller Account Id')`,
		int64(2612345680))
	require.Error(t, err, "expected NOT NULL violation inserting a seller without store_id")
}

func TestPayuniRegistrations_UniqueMovedToStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	seed := func(id, accountID, storeID, applicationID int64) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO payuni_registrations (id, account_id, store_id, application_id) VALUES ($1, $2, $3, $4)`,
			id, accountID, storeID, applicationID)
		return err
	}

	require.NoError(t, seed(1, 100, 999, 1))
	err := seed(2, 200, 999, 2)
	require.Error(t, err, "expected UNIQUE violation on store_id=999")
	require.NoError(t, seed(3, 100, 888, 3))
}

func TestUpsertPayuniRegistration_OnConflictTargetsStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewPayuniRepository(pool)

	const accountID int64 = 2613000777

	reg := &domain.PayuniRegistration{
		ID:            10,
		AccountID:     accountID,
		StoreID:       accountID,
		ApplicationID: 1,
		PayuniMerID:   "HAWK777",
		Status:        domain.PayuniStatusPending,
	}
	require.NoError(t, repo.Upsert(ctx, reg), "first Upsert")

	reg2 := &domain.PayuniRegistration{
		ID:             20,
		AccountID:      accountID,
		StoreID:        accountID,
		ApplicationID:  2,
		PayuniMerID:    "HAWK777",
		Status:         domain.PayuniStatusActive,
		PaymentEnabled: true,
	}
	require.NoError(t, repo.Upsert(ctx, reg2), "second Upsert (conflict on store_id)")

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM payuni_registrations WHERE store_id = $1`, accountID).
		Scan(&count))
	require.Equal(t, 1, count, "expected exactly 1 row for store_id after upsert conflict")

	var status string
	var paymentEnabled bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT registration_status, payment_enabled FROM payuni_registrations WHERE store_id = $1`, accountID).
		Scan(&status, &paymentEnabled))
	require.Equal(t, domain.PayuniStatusActive.String(), status,
		"second upsert must update the existing row in place, not insert a duplicate")
	require.True(t, paymentEnabled)
}
