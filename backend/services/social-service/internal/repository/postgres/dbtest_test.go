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

// The package shares one postgres container and hands each test its own
// database cloned from it, instead of booting a container per test. The
// mechanism and its rationale live in go-pkg/testutil/dbtest — mirrors
// seller-service/internal/repository/postgres/dbtest_test.go exactly (this
// package's first use of the harness, added for
// seller_identity_backfill_integration_test.go).
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "social_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupTestDB returns a pool onto a database of this test's own, holding the
// fully migrated schema and no rows.
func setupTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := pgxpool.New(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
