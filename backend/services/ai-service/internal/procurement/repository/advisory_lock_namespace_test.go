//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	// aigen is ai-service's own postgres/gen package — a different sqlc entry
	// (services/ai-service/db/sqlc.yaml has two, see its comment) than this
	// package's own gen below, but both point at the same physical
	// hawkr_ai database (module.go's newProcurementPool). Importing it
	// from here follows the existing precedent in ranking_facts.go: it is a
	// Go-level code-generation boundary, not a connectivity or layering one.
	// scripts/check-layer-imports.py's ALLOW table only scopes the eight
	// top-level architecture layers (domain/usecase/repository/...);
	// internal/procurement/** is not one of them, so files under it —
	// sources and imports alike — are outside that gate's enforcement, same
	// as ranking_facts.go's existing import of this exact package.
	aigen "github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
)

// TestAcquireProcurementUsageLock_UsesNamespacedKey is the positive control
// for quota.sql, mirroring TestAcquireAiUsageLock_UsesNamespacedKey in
// internal/repository/postgres (see that file's doc comment for the full
// rationale: reverting either query file to a bare bigint must fail some
// test, and a bare "the two real functions don't block" assertion alone
// cannot distinguish "both fixed" from "exactly one fixed" — the reference
// oracle here is what makes this specific file's regression catchable).
//
// Holds the lock through the real generated AcquireProcurementUsageLock
// (gen.Queries) in tx1, then independently issues the exact expected raw
// formula — pg_advisory_xact_lock(hashtextextended('procurement_usage_buyer',
// buyer_id)) — in tx2. Contention (tx2 blocks until timeout) proves the
// generated function computes that formula; if quota.sql regresses to a bare
// bigint, tx2 would return immediately instead, and this test fails.
func TestAcquireProcurementUsageLock_UsesNamespacedKey(t *testing.T) {
	const shared = int64(2636840881) // the value that actually overlaps on dev

	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tx1, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx1.Rollback(ctx)
	require.NoError(t, gen.New(tx1).AcquireProcurementUsageLock(ctx, shared))

	done := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			done <- err
			return
		}
		defer tx2.Rollback(ctx)
		_, err = tx2.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('procurement_usage_buyer', $1::bigint))`, shared)
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("reference lock (hashtextextended('procurement_usage_buyer', ...)) was NOT blocked by "+
			"AcquireProcurementUsageLock (err=%v) — the generated function is no longer computing that key; "+
			"quota.sql likely regressed to a bare bigint", err)
	case <-time.After(2 * time.Second):
		// Expected: contention proves the generated function computes this key.
	}
}

// TestAdvisoryLock_StoreAndBuyerWithSameNumericID_DoNotBlock is the
// regression test for issue #3519 slice B / Task 2 (2026-09-19 dev finding):
// ai_usage.store_id ranges 1 – 7487768729113814280 (26 rows), procurement_usage.
// buyer_id ranges 2636840881 – 7506715228034301952 (248 rows), and the two
// ranges intersect at exactly one key: 2636840881. Before this fix both
// AcquireAiUsageLock and AcquireProcurementUsageLock took pg_advisory_xact_lock
// on a bare bigint — and advisory lock keys are namespaced per-database, not
// per-table or per-subsystem, so that one seller's image-transform usage
// tracking and that one buyer's procurement-quota tracking would queue behind
// each other in hawkr_ai.
//
// Both sides here go through the real generated functions — AcquireAiUsageLock
// via aigen (ai-service's postgres/gen) and AcquireProcurementUsageLock via
// this package's own gen — so this is the actual production call path, not a
// hand-written stand-in for it. Catching a regression in exactly one of the
// two query files is this file's and the sibling package's positive-control
// tests' job (see their doc comments); a bare "these two don't block" check
// alone cannot tell "both fixed" apart from "exactly one fixed", since an
// unnamespaced key and an unrelated namespaced hash essentially never
// collide by chance either way. This test's job is narrower and is the
// actual point of the whole slice: prove the two real, already-fixed
// functions really don't block each other when called with the same numeric
// value.
func TestAdvisoryLock_StoreAndBuyerWithSameNumericID_DoNotBlock(t *testing.T) {
	const shared = int64(2636840881) // the value that actually overlaps on dev

	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tx1, err := pool.Begin(ctx) // holds ai_usage's store lock, via the real function
	require.NoError(t, err)
	defer tx1.Rollback(ctx)
	require.NoError(t, aigen.New(tx1).AcquireAiUsageLock(ctx, shared))

	// A second transaction takes procurement's buyer lock, same numeric
	// value, also via the real function.
	done := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			done <- err
			return
		}
		defer tx2.Rollback(ctx)
		done <- gen.New(tx2).AcquireProcurementUsageLock(ctx, shared)
	}()

	select {
	case err := <-done:
		require.NoError(t, err, "second lock failed")
	case <-time.After(2 * time.Second):
		t.Fatal("second lock was blocked by the first — namespacing did not take effect")
	}
}
