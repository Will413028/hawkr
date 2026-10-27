//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestPayuniMerchantProfileUpdateRepository_ClaimLifecycle(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPayuniMerchantProfileUpdateRepository(pool)
	changed, _ := json.Marshal(map[string]string{"contactName": "***"})

	row := &domain.PayuniMerchantProfileUpdate{
		ID: 1, StoreID: 999, AccountID: 100,
		IdempotencyKey: "key-lifecycle01", PayloadHash: "hash-a",
		Status: domain.PayuniMerchantProfileUpdateInProgress, ChangedFields: changed,
	}
	inserted, err := repo.Insert(ctx, row)
	require.NoError(t, err)
	require.True(t, inserted)

	// Second claim with the same key: not inserted, not an error.
	inserted, err = repo.Insert(ctx, &domain.PayuniMerchantProfileUpdate{
		ID: 2, StoreID: 999, AccountID: 100,
		IdempotencyKey: "key-lifecycle01", PayloadHash: "hash-b",
		Status: domain.PayuniMerchantProfileUpdateInProgress, ChangedFields: changed,
	})
	require.NoError(t, err)
	require.False(t, inserted)

	existing, err := repo.FindByStoreAndKey(ctx, 999, "key-lifecycle01")
	require.NoError(t, err)
	require.Equal(t, "hash-a", existing.PayloadHash, "the FIRST claim must be the one found")

	require.NoError(t, repo.MarkSucceeded(ctx, existing.ID, "SUCCESS", "", time.Now().UTC()))

	after, err := repo.FindByStoreAndKey(ctx, 999, "key-lifecycle01")
	require.NoError(t, err)
	require.Equal(t, domain.PayuniMerchantProfileUpdateSucceeded, after.Status)

	require.NoError(t, repo.Delete(ctx, after.ID))
	_, err = repo.FindByStoreAndKey(ctx, 999, "key-lifecycle01")
	require.Error(t, err, "row must be gone after Delete")
}
