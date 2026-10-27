//go:build integration

package usecase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// 同 internal/procurement/repository/dbtest_test.go 的 shared-container-per-package、
// own-database-per-test 機制。這個 package 需要自己的 template DB：命令層級冪等的
// 測試會斷言「procurement_requests 只有一列」，任何來自兄弟測試的殘留列都會讓那個
// 斷言失去意義（而且是靜默失去——多一列只會讓它變紅，少一列則會讓它假綠）。
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "procurement_usecase_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// Pool goes through database.NewPool — the same path production uses
// (module.go's newProcurementPool) — instead of a bare pgxpool.New, so the
// pool's QueryExecMode matches production (pgx.QueryExecModeExec, set for
// PgBouncer compatibility). See repository/dbtest_test.go's
// setupProcurementTestDB for the full rationale.
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

// testIDGen 回傳可預期的遞增 ID，讓斷言不依賴真實 TSID 的時間成分。
func testIDGen() func() int64 {
	var n int64
	return func() int64 { n++; return n }
}
