//go:build integration

package main

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
// What is specific to this package, and load-bearing even at two tests: both
// call seedCLIFixture with the same TSID constants, and the 41
// price_observation rows it writes go through InsertPriceObservation — a plain
// INSERT (db/queries/observation.sql:30), not an upsert like the market/crop
// rows around it. On a shared database the second test to run would die on the
// primary key. Quieter but just as real:
// TestRun_SeriesForecast_OriginCutoffShrinksDataThroughDate asserts that an
// earlier --origin yields a smaller data_through_date, which is a statement
// about exactly which observations exist as of that cutoff — leftovers from a
// sibling test change the answer without breaking anything.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "pricebook_cli_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupTestDB 回傳一個只屬於這個測試的資料庫連線池，裡面是跑完 db/migrations/*.sql
// 的 schema、沒有任何資料列，並把 DB_* 環境變數指向同一個資料庫——run() 會呼叫
// config.Load()，讀的就是這些環境變數。簽章與保證跟「每個測試獨佔一個容器」的
// 版本完全相同，呼叫點一行未改。
func setupTestDB(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	cfg, err := pgxpool.ParseConfig(db.DSN)
	require.NoError(t, err)
	// 對齊 pkg/database/postgres.go 的 production 設定，讓測試走與線上相同的協定路徑。
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		// drop 覆蓋在這個 pool 之外開的連線：run() 會自己用 database.NewPool
		// （main.go:132）另開一個並在回傳時關掉，但刪庫不該被還沒消失的 backend
		// 綁住。
		drop()
	})

	// run() 呼叫 config.Load()，讀的是這些環境變數——比照 cmd/pricebook-backfill、
	// cmd/pricebook-export 的既有慣例（DB_HOST/USER/PASSWORD/NAME 走環境變數，
	// 未設時用開發預設值）。DB_NAME 指向本測試專屬的複本，不是 template。
	t.Setenv("DB_HOST", db.Host)
	t.Setenv("DB_PORT", db.Port)
	t.Setenv("DB_POOL_PORT", "") // 清空，避免本機殘留的 DB_POOL_PORT 蓋過 DB_PORT
	t.Setenv("DB_USER", db.User)
	t.Setenv("DB_PASSWORD", db.Password)
	t.Setenv("DB_NAME", db.Name)
	t.Setenv("DB_SSLMODE", "disable")

	return pool
}
