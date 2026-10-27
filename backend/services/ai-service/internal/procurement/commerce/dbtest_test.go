//go:build integration

package commerce

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// 同 internal/procurement/usecase/dbtest_test.go（以及 repository 那份）的
// shared-container-per-package、own-database-per-test 機制。commerce 抽成
// 獨立套件後需要自己的 template DB，理由跟 usecase／repository 兩份彼此
// 獨立同一個道理：命令層級冪等的測試會斷言「procurement_requests 只有一列」，
// 任何來自兄弟套件測試的殘留列都會讓那個斷言失去意義。
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "procurement_commerce_test",
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
