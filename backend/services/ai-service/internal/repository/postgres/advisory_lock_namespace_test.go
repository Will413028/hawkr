//go:build integration

// Package postgres holds cross-subsystem integration coverage for the
// hawkr_ai database — coverage that does not belong to either
// subsystem's own package because it is about the resource they share, not
// either one's business logic. ai_usage's tests live in internal/usecase;
// procurement_usage's live in internal/procurement/repository.
package postgres

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// Same shared-container-per-package, own-database-per-test mechanism as
// internal/usecase/dbtest_test.go and internal/procurement/repository/dbtest_test.go.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "advisory_lock_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupAdvisoryLockTestDB returns a pgx pool onto a database of this test's
// own, holding the fully migrated schema. Mirrors setupAiTestPool /
// setupProcurementTestDB in the sibling packages.
func setupAdvisoryLockTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}

// TestAcquireAiUsageLock_UsesNamespacedKey is the positive control that makes
// this file actually discriminate a regression in usage.sql (2026-09-19 code
// review finding on the first version of this file: it hardcoded both sides'
// SQL as literal strings, so it tested Postgres's advisory-lock behaviour in
// the abstract, not this repository's generated code — reverting usage.sql
// to a bare bigint would have left it green).
//
// It holds the lock through the real generated AcquireAiUsageLock function
// (gen.Queries, not a hand-written SQL string) in tx1, then independently
// issues the *exact* formula usage.sql is supposed to compile to —
// pg_advisory_xact_lock(hashtextextended('ai_usage_store', store_id)) — as a
// raw reference call in tx2. If AcquireAiUsageLock really computes that
// formula, tx2 contends with tx1 and blocks until timeout (the asserted,
// "good" outcome below). If usage.sql regresses to a bare bigint key,
// AcquireAiUsageLock computes a different lock than tx2's reference call,
// the two stop contending, and tx2 returns immediately — which this test
// fails on.
//
// See TestAdvisoryLock_StoreAndBuyerWithSameNumericID_DoNotBlock in
// internal/procurement/repository for the actual cross-subsystem property
// (both sides' *real* generated functions, same numeric value, must not
// block each other) and its sibling positive control for quota.sql.
func TestAcquireAiUsageLock_UsesNamespacedKey(t *testing.T) {
	const shared = int64(2636840881) // the value that actually overlaps on dev

	ctx := t.Context()
	pool, cleanup := setupAdvisoryLockTestDB(ctx, t)
	t.Cleanup(cleanup)

	tx1, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx1.Rollback(ctx)
	// The real generated function under test — not a hardcoded SQL string.
	require.NoError(t, gen.New(tx1).AcquireAiUsageLock(ctx, shared))

	// Independent reference oracle: the exact formula usage.sql must compile
	// to. This is deliberately raw SQL — you cannot test "does X compute the
	// same key as Y" without an independent Y — but it is not the thing under
	// test; AcquireAiUsageLock above is.
	done := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			done <- err
			return
		}
		defer tx2.Rollback(ctx)
		_, err = tx2.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('ai_usage_store', $1::bigint))`, shared)
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("reference lock (hashtextextended('ai_usage_store', ...)) was NOT blocked by "+
			"AcquireAiUsageLock (err=%v) — the generated function is no longer computing that key; "+
			"usage.sql likely regressed to a bare bigint", err)
	case <-time.After(2 * time.Second):
		// Expected: tx2's reference call is blocked behind tx1's real lock,
		// proving AcquireAiUsageLock computes exactly this key.
	}
}
