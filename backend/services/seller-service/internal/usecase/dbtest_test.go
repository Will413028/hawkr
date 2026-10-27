//go:build integration

package usecase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "seller_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupPayuniIntegrationDB returns a pool onto a database of this test's own,
// holding the fully migrated schema and no rows. Signature and guarantees are
// unchanged from when it returned a pool onto a container of this test's own.
func setupPayuniIntegrationDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := pgxpool.New(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
