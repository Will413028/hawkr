//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

func TestUpdatePayuniMerchantProfile_VersionCAS(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	_, err := pool.Exec(ctx,
		`INSERT INTO payuni_registrations (id, account_id, store_id, application_id, payuni_mer_id, registration_status, payment_enabled, updated_at)
		 VALUES (1, 100, 999, 0, 'HAWK999', 'active', true, now())`)
	require.NoError(t, err)

	q := gen.New(pool)

	row, err := q.UpdatePayuniMerchantProfile(ctx, gen.UpdatePayuniMerchantProfileParams{
		StoreID:                999,
		MerchantProfileVersion: 0,
		ContactName:            "王小明",
		ContactMobile:          "",
		ContactTel:             "",
		MerEmail:               "",
		MerAddr:                "",
		MerAddrPost:            "",
		MerTel:                 "",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), row.MerchantProfileVersion, "version must advance from 0 to 1")

	// Stale version must now be rejected (zero rows, not an error at the SQL level).
	_, err = q.UpdatePayuniMerchantProfile(ctx, gen.UpdatePayuniMerchantProfileParams{
		StoreID:                999,
		MerchantProfileVersion: 0, // stale — real version is now 1
		ContactName:            "李小華",
	})
	require.ErrorIs(t, err, pgx.ErrNoRows, "stale expectedVersion must match zero rows")
}

func TestInsertPayuniMerchantProfileUpdate_ClaimConflict(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	changed, _ := json.Marshal(map[string]string{"contactName": "changed"})

	first, err := q.InsertPayuniMerchantProfileUpdate(ctx, gen.InsertPayuniMerchantProfileUpdateParams{
		ID: 1, StoreID: 999, AccountID: 100, IdempotencyKey: "key-abc12345", // gitleaks:allow（測試值）
		PayloadHash: "hash-a", Status: "IN_PROGRESS", ChangedFields: changed,
	})
	require.NoError(t, err)
	require.Equal(t, "IN_PROGRESS", first.Status)

	// Same (store_id, idempotency_key) again -> ON CONFLICT DO NOTHING -> zero rows.
	_, err = q.InsertPayuniMerchantProfileUpdate(ctx, gen.InsertPayuniMerchantProfileUpdateParams{
		ID: 2, StoreID: 999, AccountID: 100, IdempotencyKey: "key-abc12345", // gitleaks:allow（測試值）
		PayloadHash: "hash-b", Status: "IN_PROGRESS", ChangedFields: changed,
	})
	require.ErrorIs(t, err, pgx.ErrNoRows)

	existing, err := q.GetPayuniMerchantProfileUpdateByStoreAndKey(ctx, gen.GetPayuniMerchantProfileUpdateByStoreAndKeyParams{
		StoreID: 999, IdempotencyKey: "key-abc12345", // gitleaks:allow（測試值）
	})
	require.NoError(t, err)
	require.Equal(t, "hash-a", existing.PayloadHash, "the FIRST insert's row must be the one still there")
}
