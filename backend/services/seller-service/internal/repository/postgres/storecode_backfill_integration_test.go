//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// storeCodeNotNullMigration is the migration that closes the nullable →
// backfill → NOT NULL transition for sellers.store_code (Step 9 of this
// task). The backfill tests below (BackfillStoreCodeFromMerchantCode /
// BackfillFreshStoreCodes, including its collision-retry and
// give-up-after-max-attempts branches / VerifyStoreCodeBackfillComplete)
// exercise those functions against exactly the schema state they're written
// for — store_code nullable, unique index already in place — by excluding
// this one migration from the replay. That state isn't a fiction: it's the
// real state every environment (dev already, prod later) is in for the
// window between the two migrations, and this job is designed to be re-run
// there before that env's own NOT NULL migration lands. Once this migration
// is included (any other test in this package, or these same tests without
// the exclude), the column can never hold NULL again — see
// TestSellersStoreCode_NotNullEnforced below for that side of the contract.
const storeCodeNotNullMigration = "20260721061000_sellers_store_code_not_null.sql"

// dropMerchantCodeMigration is Task 6's migration retiring
// payuni_registrations.merchant_code (the column BackfillStoreCodeFromMerchantCode
// reads from). TestBackfillStoreCode_CopiesFromMerchantCode below needs the
// column to still exist to seed its fixture, so it excludes this migration
// too — the same transitional state every environment (dev already, prod
// later) is actually in for the window before its own copy of this migration
// lands, matching storeCodeNotNullMigration's rationale above.
const dropMerchantCodeMigration = "20260721062000_drop_payuni_merchant_code.sql"

// repairDropMerchantCodeMigration (added 2026-07-23) is an idempotent
// re-assertion of dropMerchantCodeMigration — dev's ledger had recorded that
// migration as applied without the DDL ever actually running there (a real
// ledger-vs-reality drift found live 2026-07-23), so this repair migration
// exists to close the gap through the normal CI apply path. It must be
// excluded here for the same reason as dropMerchantCodeMigration above: this
// test needs merchant_code to still exist, and IF EXISTS makes this
// migration a no-op replay away, not a schema no-op.
const repairDropMerchantCodeMigration = "20260722182626_repair_drift_drop_payuni_merchant_code.sql"

func TestBackfillStoreCode_CopiesFromMerchantCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t, storeCodeNotNullMigration, dropMerchantCodeMigration, repairDropMerchantCodeMigration)
	defer cleanup()

	_, err := pool.Exec(ctx, `INSERT INTO sellers (store_id, account_id, is_current, store_name, created_at, updated_at) VALUES (1001, 1001, true, 'Seed Store', now(), now())`)
	if err != nil {
		t.Fatalf("seed sellers: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO payuni_registrations (id, account_id, store_id, merchant_code, application_id, updated_at) VALUES (5001, 1001, 1001, 1001, 9001, now())`)
	if err != nil {
		t.Fatalf("seed payuni_registrations: %v", err)
	}

	n, err := postgres.BackfillStoreCodeFromMerchantCode(ctx, pool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows backfilled = %d, want 1", n)
	}

	var storeCode int64
	if err := pool.QueryRow(ctx, `SELECT store_code FROM sellers WHERE store_id = 1001`).Scan(&storeCode); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if storeCode != 1001 {
		t.Fatalf("store_code = %d, want 1001 (copied verbatim from merchant_code)", storeCode)
	}
}

func TestBackfillStoreCode_MintsFreshForSellersWithoutPayuniRegistration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t, storeCodeNotNullMigration)
	defer cleanup()

	_, err := pool.Exec(ctx, `INSERT INTO sellers (store_id, account_id, is_current, store_name, created_at, updated_at) VALUES (2002, 2002, true, 'Seed Store 2', '2026-07-21 00:00:00+00', now())`)
	if err != nil {
		t.Fatalf("seed sellers: %v", err)
	}

	n, err := postgres.BackfillFreshStoreCodes(ctx, pool, domain.GenerateStoreCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows backfilled = %d, want 1", n)
	}

	var storeCode int64
	if err := pool.QueryRow(ctx, `SELECT store_code FROM sellers WHERE store_id = 2002`).Scan(&storeCode); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if storeCode < 2_600_000_000 || storeCode > 2_699_999_999 {
		t.Fatalf("store_code = %d, want in 2026 range [2600000000, 2699999999] (created_at year)", storeCode)
	}
}

// Finding C (final whole-branch review): BackfillFreshStoreCodes's per-row
// retry loop previously called domain.GenerateStoreCode directly, so no test
// could force a collision or exhaust all 5 attempts. The generate parameter
// makes both branches testable with a fake, mirroring the usecase layer's
// injected storeCode func() (int64, error).
//
// This test seeds a colliding row FIRST, then backfills a second (NULL
// store_code) row with a fake generator that returns the colliding value on
// attempt 1 and a fresh value on attempt 2 — proving the retry loop recovers
// from a REAL Postgres unique-violation (not a mocked one) and persists the
// fresh value, not the colliding one.
func TestBackfillFreshStoreCodes_RetriesOnCollisionAndPersistsFreshValue(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t, storeCodeNotNullMigration)
	defer cleanup()

	const collidingCode, freshCode int64 = 9_900_000_001, 9_900_000_002

	_, err := pool.Exec(ctx, `INSERT INTO sellers (store_id, account_id, is_current, store_name, store_code, created_at, updated_at) VALUES (6001, 6001, true, 'Colliding Seed', $1, now(), now())`, collidingCode)
	if err != nil {
		t.Fatalf("seed colliding seller: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO sellers (store_id, account_id, is_current, store_name, created_at, updated_at) VALUES (6002, 6002, true, 'Target Seller', now(), now())`)
	if err != nil {
		t.Fatalf("seed target seller: %v", err)
	}

	var generateCalls int
	fakeGenerate := func(_ time.Time) (int64, error) {
		generateCalls++
		if generateCalls == 1 {
			return collidingCode, nil
		}
		return freshCode, nil
	}

	n, err := postgres.BackfillFreshStoreCodes(ctx, pool, fakeGenerate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows backfilled = %d, want 1", n)
	}
	if generateCalls != 2 {
		t.Fatalf("generate calls = %d, want 2 (1 collision + 1 success)", generateCalls)
	}

	var storeCode int64
	if err := pool.QueryRow(ctx, `SELECT store_code FROM sellers WHERE store_id = 6002`).Scan(&storeCode); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if storeCode != freshCode {
		t.Fatalf("store_code = %d, want %d (the fresh value from the second attempt, not the colliding first attempt)", storeCode, freshCode)
	}
}

// Mirrors TestBackfillFreshStoreCodes_RetriesOnCollisionAndPersistsFreshValue
// but the fake generator NEVER returns a non-colliding value, proving the
// retry loop actually gives up after maxAttempts (5) rather than looping
// forever or silently succeeding, and returns a wrapped error.
func TestBackfillFreshStoreCodes_GivesUpAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t, storeCodeNotNullMigration)
	defer cleanup()

	const collidingCode int64 = 9_900_000_101

	_, err := pool.Exec(ctx, `INSERT INTO sellers (store_id, account_id, is_current, store_name, store_code, created_at, updated_at) VALUES (6101, 6101, true, 'Colliding Seed', $1, now(), now())`, collidingCode)
	if err != nil {
		t.Fatalf("seed colliding seller: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO sellers (store_id, account_id, is_current, store_name, created_at, updated_at) VALUES (6102, 6102, true, 'Target Seller', now(), now())`)
	if err != nil {
		t.Fatalf("seed target seller: %v", err)
	}

	var generateCalls int
	fakeGenerate := func(_ time.Time) (int64, error) {
		generateCalls++
		return collidingCode, nil
	}

	_, err = postgres.BackfillFreshStoreCodes(ctx, pool, fakeGenerate)
	if err == nil {
		t.Fatal("expected error after exhausting all retry attempts")
	}
	if generateCalls != 5 {
		t.Fatalf("generate calls = %d, want 5 (maxAttempts)", generateCalls)
	}

	var storeCode *int64
	if err := pool.QueryRow(ctx, `SELECT store_code FROM sellers WHERE store_id = 6102`).Scan(&storeCode); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if storeCode != nil {
		t.Fatalf("store_code = %v, want NULL (every attempt collided, row must be untouched)", *storeCode)
	}
}

func TestVerifyStoreCodeBackfillComplete_ErrorsOnRemainingNulls(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t, storeCodeNotNullMigration)
	defer cleanup()

	_, err := pool.Exec(ctx, `INSERT INTO sellers (store_id, account_id, is_current, store_name, created_at, updated_at) VALUES (3003, 3003, true, 'Seed Store 3', now(), now())`)
	if err != nil {
		t.Fatalf("seed sellers: %v", err)
	}

	if err := postgres.VerifyStoreCodeBackfillComplete(ctx, pool); err == nil {
		t.Fatal("expected error: seeded row has NULL store_code, verify should not pass")
	}
}

// TestSellersStoreCode_NotNullEnforced proves the Step 9 migration actually
// tightened the column — a direct INSERT that omits store_code must be
// rejected by the DB once the migration is applied (no exclude here, unlike
// the three tests above: this one wants the full, final migration set).
// Mirrors the established pattern for this same nullable-add → backfill →
// NOT NULL flow at TestSellersStoreID_NotNullEnforced
// (store_id_tighten_integration_test.go).
func TestSellersStoreCode_NotNullEnforced(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (store_id, account_id, is_current, store_name, created_at, updated_at) VALUES (4004, 4004, true, 'No Store Code', now(), now())`)
	if err == nil {
		t.Fatal("expected NOT NULL violation inserting a seller without store_code")
	}
}
