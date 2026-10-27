//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestMerchantUC_Create_ReonboardInsertsNewStint(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPayuniIntegrationDB(ctx, t)
	defer cleanup()

	const accountID, oldStoreID int64 = 100, 100

	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, seller_type, status, created_at, updated_at, deleted_at)
		 VALUES ($1, $2, $2, true, 'Old Store', 'individual', 'closed', now(), now(), now())`,
		accountID, oldStoreID)
	require.NoError(t, err)

	seedPayuniRegistration(ctx, t, pool, 1, accountID, oldStoreID, 111, "HAWK100", "active")

	uc := &SellerUseCase{
		sellerRepo:    postgres.NewSellerRepository(pool),
		payuniRepo:    postgres.NewPayuniRepository(pool),
		outboxRepo:    postgres.NewSellerDeletedOutboxRepository(pool),
		txManager:     database.NewTxManager(pool),
		accountClient: testAccountClient,
		mediaBaseURL:  "https://cdn.test",
		tsid:          func() int64 { return 999 },
		storeCode:     testStoreCode,
	}

	_, err = uc.Create(ctx, accountID, "Reopened Store", "0912345678")
	require.NoError(t, err)

	// Old stint: retired tombstone with original ids intact.
	var oldCurrent bool
	var oldStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT is_current, status FROM sellers WHERE store_id = $1`, oldStoreID).
		Scan(&oldCurrent, &oldStatus))
	require.False(t, oldCurrent, "old stint must be retired (is_current=false)")
	require.Equal(t, "closed", oldStatus, "old stint stays closed (terminal)")

	// New stint: fresh surrogate, active, current.
	var newStatus, newStore string
	var newCurrent bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT is_current, status, store_name FROM sellers WHERE store_id = $1`, int64(999)).
		Scan(&newCurrent, &newStatus, &newStore))
	require.True(t, newCurrent)
	require.Equal(t, "active", newStatus)
	require.Equal(t, "Reopened Store", newStore)

	// PayUni history retained under the OLD surrogate; new stint reads
	// "unregistered" by construction (keyed on the new surrogate).
	var payuniCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM payuni_registrations WHERE store_id = $1`, oldStoreID).
		Scan(&payuniCount))
	require.Equal(t, 1, payuniCount, "old-stint payuni row must be retained")

	// seller.onboarded enqueued with the enqueue-time snapshot.
	var eventType string
	var payload []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT event_type, payload FROM seller_deleted_outbox WHERE account_id = $1 ORDER BY created_at DESC LIMIT 1`, accountID).
		Scan(&eventType, &payload))
	require.Equal(t, "seller.onboarded", eventType)
	require.JSONEq(t, `{"accountId":100,"storeId":999}`, string(payload))
}
