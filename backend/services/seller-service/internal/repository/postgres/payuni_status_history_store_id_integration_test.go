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

func TestInsertStatusHistory_PersistsStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewPayuniRepository(pool)

	const principal, aggregate int64 = 100, 999

	require.NoError(t, repo.InsertStatusHistory(ctx, &domain.PayuniStatusHistory{
		ID: 1, AccountID: principal, StoreID: aggregate,
		PayuniMerID: "HAWK999", Source: "sync", RecordedAt: time.Now().UTC(),
	}))

	var gotAccountID, gotStoreID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT account_id, store_id FROM payuni_status_history WHERE id = $1`, int64(1)).
		Scan(&gotAccountID, &gotStoreID))
	require.Equal(t, principal, gotAccountID)
	require.Equal(t, aggregate, gotStoreID,
		"persisted store_id must come from StoreID, not AccountID")
}
