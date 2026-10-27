//go:build integration

package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// loadSellerSnapshotMapping reads seller-service's OWN `sellers` table
// (hawkr_seller), not social-service's — this package's DB-backed test
// therefore replays seller-service's migrations, not social-service's.
// Mirrors cmd/backfill-seller-content-identity/dbtest_test.go exactly (same
// shape of cross-database backfill tool, same test harness need).
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "seller_test",
	MigrationsDir: filepath.Join("..", "..", "..", "seller-service", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

func setupSellerTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := pgxpool.New(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
