//go:build integration

// 這些測試需要本機 Docker daemon，用 `make test-integration`（或 `go test -tags integration ./...`）跑；
// CI 由 .github/workflows/integration-tests.yml 的 pricebook-service job 執行。
package postgres_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// The package shares one postgres container and hands each test its own
// database cloned from it, instead of booting a container per test. The
// mechanism and its rationale live in go-pkg/testutil/dbtest.
//
// What is specific to this package: every test seeds from the same small pool
// of hard-coded ids (700000000000000001-3, 981000000000001000/2000/3000), so a
// shared database collides on price_observation's primary key before it
// collides on anything interesting. The subtler exposure is that the repository
// methods under test are UNBOUNDED READS — SeriesObservations/RestDays/
// GroupObservations return whatever matches, and the tests assert
// `require.Len(t, got, 1)` or `2` on the result — plus schema_invariants_test.go
// runs a whole-table self-join (`FROM price_observation a JOIN price_observation
// b`) and asserts the violation count is zero. All of those are statements about
// the entire table, so a neighbour's rows change the answer rather than merely
// adding noise.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "pricebook_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupTestDB 回傳一個只屬於這個測試的資料庫連線池，裡面是跑完 db/migrations/*.sql
// 的 schema、沒有任何資料列。簽章與保證跟「每個測試獨佔一個容器」的版本完全相同。
func setupTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	cfg, err := pgxpool.ParseConfig(db.DSN)
	require.NoError(t, err)
	// 對齊 pkg/database/postgres.go:78 的 production 設定，讓測試走與線上相同的
	// 協定路徑（NUMERIC 的參數推斷在兩種 exec mode 下行為不同）。
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
