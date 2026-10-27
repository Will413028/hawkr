//go:build integration

package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func seedPayuniRegistration(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id, accountID, storeID, applicationID int64, merID, status string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO payuni_registrations
			(id, account_id, store_id, application_id, payuni_mer_id, registration_status, payment_enabled, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, false, now())`,
		id, accountID, storeID, applicationID, merID, status)
	require.NoError(t, err)
}

func TestHandleAgentNotify_KeysOnStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPayuniIntegrationDB(ctx, t)
	defer cleanup()

	payuniRepo := postgres.NewPayuniRepository(pool)

	// application_id=0 short-circuits the cert FSM transition (transitionCertFSM
	// treats 0 as "no application to advance"), so this test isolates the lookup
	// key without needing a seeded certification_applications row or a fake certRepo.
	seedPayuniRegistration(ctx, t, pool, 1, 100, 999, 0, "HAWK999", "pending")

	uc := &PayuniUseCase{payuniRepo: payuniRepo}

	err := uc.HandleAgentNotify(ctx, "HAWK999", "1", map[string]string{"MemberStatus": "1"})
	require.NoError(t, err, "old code looks up account_id=999 -> NotFound")

	var gotStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT registration_status FROM payuni_registrations WHERE id = $1`, int64(1)).
		Scan(&gotStatus))
	require.Equal(t, "active", gotStatus, "the store_id=999 row must be the one looked up and updated")
}

// TestHandleAgentNotify_KeysOnMerID proves the webhook resolves the
// registration by its STORED payuni_mer_id, not by re-parsing/deriving a
// store_id out of the MerID string. Task 5's cutover made MerID =
// HAWK{store_code}, so for any non-grandfathered seller — where store_code
// diverges from the long store_id TSID, the common case going forward —
// treating the parsed MerID suffix as a store_id resolves nothing (or worse,
// a wrong row). Seed a store with a long-TSID store_id and a short, distinct
// store_code the MerID actually derives from, so a regression back to
// store_id-based resolution can't accidentally pass.
func TestHandleAgentNotify_KeysOnMerID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPayuniIntegrationDB(ctx, t)
	defer cleanup()

	payuniRepo := postgres.NewPayuniRepository(pool)

	const longStoreID int64 = 7300000000000000123
	const merID = "HAWK2612345678" // HAWK{store_code}, store_code != store_id

	// application_id=0 short-circuits the cert FSM transition, isolating the
	// lookup key the same way TestHandleAgentNotify_KeysOnStoreID does.
	seedPayuniRegistration(ctx, t, pool, 1, 100, longStoreID, 0, merID, "pending")

	uc := &PayuniUseCase{payuniRepo: payuniRepo}

	err := uc.HandleAgentNotify(ctx, merID, "1", map[string]string{"MemberStatus": "1"})
	require.NoError(t, err, "must resolve by the stored payuni_mer_id, not by parsing/deriving store_id from the MerID string")

	var gotStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT registration_status FROM payuni_registrations WHERE id = $1`, int64(1)).
		Scan(&gotStatus))
	require.Equal(t, "active", gotStatus, "the row seeded with store_id != store_code must still be looked up (by MerID) and updated")
}

// RefreshStatus (seller-triggered refresh, e.g. from grpc RefreshPayuniStatus) must
// key on store_id: the caller already has the seller's aggregate id loaded.
func TestRefreshStatus_KeysOnStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPayuniIntegrationDB(ctx, t)
	defer cleanup()

	payuniRepo := postgres.NewPayuniRepository(pool)

	seedPayuniRegistration(ctx, t, pool, 2, 100, 999, 0, "HAWK999", "pending")

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{
				MerStatus: "1", MemberStatus: "1", CreditStatus: "1",
				ApplePayStatus: "0", GooglePayStatus: "0", ATMStatus: "0",
				RawResponse: json.RawMessage(`{}`),
			}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}

	err := uc.RefreshStatus(ctx, 999)
	require.NoError(t, err, "old code looks up account_id=999 -> NotFound")

	var gotStatus, gotMerStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT registration_status, mer_status FROM payuni_registrations WHERE id = $1`, int64(2)).
		Scan(&gotStatus, &gotMerStatus))
	require.Equal(t, "active", gotStatus)
	require.Equal(t, "1", gotMerStatus)
}

func TestGetPayuniStatus_KeysOnStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPayuniIntegrationDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	payuniRepo := postgres.NewPayuniRepository(pool)

	const principal, stale, current int64 = 100, 100, 999
	const currentCertID int64 = 555

	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, seller_type, current_certification_id, created_at, updated_at)
		 VALUES ($1, $2, $2, true, 'Reopened Store', 'individual', $3, now(), now())`,
		principal, current, currentCertID)
	require.NoError(t, err)

	seedPayuniRegistration(ctx, t, pool, 10, principal, stale, 111, "HAWK100", "active")
	seedPayuniRegistration(ctx, t, pool, 11, principal, current, currentCertID, "HAWK999", "active")

	uc := &SellerUseCase{sellerRepo: sellerRepo, payuniRepo: payuniRepo, txManager: &mockTxManager{}}

	resp, err := uc.GetPayuniStatus(ctx, principal)
	require.NoError(t, err, "old code's account_id=100 lookup can match the stale row -> ApplicationID mismatch -> NotFound")
	require.Equal(t, "HAWK999", resp.PayuniMerID, "must resolve the CURRENT (store_id=999) row, not the stale one")
}
