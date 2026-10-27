//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestCreateSeller_PersistsStoreID_NotAccountID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const principal, aggregate int64 = 100, 999
	m := &domain.Seller{
		AccountID:  principal,
		StoreID:    aggregate,
		StoreName:  "T",
		SellerType: domain.SellerTypeIndividual,
	}
	require.NoError(t, repo.Create(ctx, m))

	var got int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT store_id FROM sellers WHERE account_id = $1`, principal).
		Scan(&got))
	require.Equal(t, aggregate, got,
		"persisted store_id must come from StoreID, not AccountID")
}

// Same contract for the primary production PayUni writer (Upsert).
func TestUpsertPayuni_PersistsStoreID_NotAccountID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewPayuniRepository(pool)

	const principal, aggregate int64 = 100, 999
	reg := &domain.PayuniRegistration{
		ID:            1,
		AccountID:     principal,
		StoreID:       aggregate,
		ApplicationID: 1,
	}
	require.NoError(t, repo.Upsert(ctx, reg))

	var got int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT store_id FROM payuni_registrations WHERE id = $1`, int64(1)).
		Scan(&got))
	require.Equal(t, aggregate, got,
		"persisted store_id must come from StoreID, not AccountID")
}

// TestReonboard_RetireThenInsert_PersistsFreshSurrogate proves the W1-2 writer
// pair: RetireCurrentStint flips the closed stint's is_current, and CreateSeller
// persists the caller's freshly minted StoreID on the successor row.
func TestReonboard_RetireThenInsert_PersistsFreshSurrogate(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const accountID, oldStoreID, newStoreID int64 = 100, 100, 999
	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, status, deleted_at)
		 VALUES ($1, $2, $2, true, 'Old Store', 'closed', now())`,
		accountID, oldStoreID)
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, repo.RetireCurrentStint(ctx, accountID, now))
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    newStoreID,
		StoreCode:  newStoreID,
		StoreName:  "Reopened",
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	var gotStatus string
	var gotCurrent bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, is_current FROM sellers WHERE store_id = $1`, newStoreID).
		Scan(&gotStatus, &gotCurrent))
	require.Equal(t, "active", gotStatus)
	require.True(t, gotCurrent)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, is_current FROM sellers WHERE store_id = $1`, oldStoreID).
		Scan(&gotStatus, &gotCurrent))
	require.Equal(t, "closed", gotStatus, "old stint retained, still closed")
	require.False(t, gotCurrent, "old stint retired")
}
