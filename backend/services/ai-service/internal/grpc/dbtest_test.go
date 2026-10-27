//go:build integration

package grpc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// Same shared-container-per-package, own-database-per-test mechanism as
// internal/procurement/repository/dbtest_test.go and
// internal/procurement/usecase/dbtest_test.go — see those files for the
// rationale. This package gets its own template DB (procurement_grpc_test)
// so its container/migrations don't collide with either of those.
//
// This is the first dbtest harness inside internal/grpc: only
// TestWatchRunEvents_LiveWakeDeliversNewEvent (Task 11) needs a real
// Postgres — it's exercising the actual LISTEN/NOTIFY wiring
// (repository.Repo.WatchRun/notifyRunWake), which an in-memory RunStore
// fake cannot stand in for.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "procurement_grpc_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupProcurementGRPCTestDB mirrors setupProcurementTestDB in
// internal/procurement/repository/dbtest_test.go: a pgx pool onto a fully
// migrated database of this test's own, through database.NewPool (the same
// path production uses) rather than a bare pgxpool.New.
func setupProcurementGRPCTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}

// testIDGen 回傳可預期的遞增 ID，讓斷言不依賴真實 TSID 的時間成分——同
// repository／usecase 兩個 package 既有的同名 helper。
func testIDGen() func() int64 {
	var n int64
	return func() int64 { n++; return n }
}
