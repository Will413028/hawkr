//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// The package shares postgres containers and hands each test its own database
// cloned from one, instead of booting a container per test. The mechanism and
// its rationale live in go-pkg/testutil/dbtest; this package had the worst
// exposure of any (40 tests, 40 containers, 47 migration files copied into
// each).
//
// What is specific to this package: ListForAdmin reads the sellers table
// unscoped with LIMIT 10 and the tests then look for their own seeded row in
// the result, and the storecode backfill functions rewrite every NULL row in
// the table and assert how many they touched. Both would silently become
// order-dependent the moment tests shared a database.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "seller_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// excludeMigrations is optional: pass exact migration file basenames (e.g.
// "20260721061000_sellers_store_code_not_null.sql") to skip them when
// building the test DB. Needed by tests that exercise logic written for a
// transitional schema state (e.g. a nullable column a later migration in
// this same directory makes NOT NULL) — that state can only be reconstructed
// by omitting the migration that closes the transition, since a fully
// migrated schema can never hold the value the transitional code is testing
// for. Each distinct set costs one more container; the callers that exclude
// nothing all share one.
//
// setupTestDB returns a pool onto a database of this test's own, holding that
// migrated schema and no rows. Signature and guarantees are unchanged from when
// it returned a pool onto a container of this test's own.
func setupTestDB(ctx context.Context, t *testing.T, excludeMigrations ...string) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t, excludeMigrations...)

	pool, err := pgxpool.New(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
