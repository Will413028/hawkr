//go:build integration

package usecase_test

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
// mechanism and its rationale live in go-pkg/testutil/dbtest; this package
// rolled the #2070 dice 15 times per run, the most of anywhere in the repo.
//
// What is specific to this package — and this is where the per-test database is
// most load-bearing: nearly every test here asserts against a WHOLE TABLE
// rather than against one row — `SELECT count(*) FROM price_observation` = 1,
// `count(*) FROM market` = 2, `count(*) FROM market_rest_day` = 10,
// `count(*) FROM ingest_run` = 1 — and the exporter tests read the entire
// observation table through ObservationCSV and assert the exact rows that come
// back. A shared database turns every one of those into a statement about
// whatever the previously-run tests happened to leave behind. Worse, the
// fixtures collide on purpose-built keys: market code "104" and crop "LA1" on
// trade date 2026-07-26 appear in most files, so a second test's "insert" would
// be deduplicated by the digest/ON CONFLICT paths under test — the very
// behaviour being measured — and two backfill tests share the identical TSID
// seed 970000000000000000, which is a primary-key collision waiting to happen.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "pricebook_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
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
