//go:build integration

package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// Same shared-container-per-package, own-database-per-test mechanism as
// internal/usecase/dbtest_test.go — see that file for the rationale. This
// package gets its own template DB (procurement_test) so the two packages'
// containers/migrations don't collide, and because AppendRevision's
// idempotency test relies on being the only writer of its request_id.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "procurement_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupProcurementTestDB returns a pgx pool onto a database of this test's
// own, holding the fully migrated schema. Mirrors setupAiTestPool in
// internal/usecase/dbtest_test.go. Signature changed from *gorm.DB to
// *pgxpool.Pool when repository switched from GORM to sqlc + pgx/v5 —
// guarantees (own DB, fully migrated, isolated per test) are unchanged.
//
// Pool goes through database.NewPool — the same path production uses
// (module.go's newProcurementPool) — instead of a bare pgxpool.New, so the
// pool's QueryExecMode matches production (pgx.QueryExecModeExec, set for
// PgBouncer compatibility). This branch's json.RawMessage sqlc overrides
// exist specifically because that exec mode skips server-side type
// inference; under pgxpool.New's default QueryExecModeCacheStatement the
// server always tells the client the correct type, so a wrong or missing
// override would go undetected. See notification-service's
// internal/repository/postgres/dbtest_test.go for the same fix applied
// there first (commit 2fca1ab47, issue #1370).
func setupProcurementTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}

func refTime() time.Time { return time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC) }

// testIDGen 回傳可預期的遞增 ID，讓斷言不依賴真實 TSID 的時間成分。
func testIDGen() func() int64 {
	var n int64
	return func() int64 { n++; return n }
}
