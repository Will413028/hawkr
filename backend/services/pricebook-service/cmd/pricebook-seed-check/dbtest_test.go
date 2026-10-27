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
// mechanism and its rationale live in go-pkg/testutil/dbtest; this package
// rolled the #2070 dice 5 times per run and now rolls it once. Starting the
// container lazily keeps main_test.go — which carries no build tag and needs no
// Docker — free.
//
// What is specific to this package, in a way no constraint would announce: four
// of the five tests seed the crop_group name "甘藍", and the subject under test
// — FindCropGroupByName — is a :many query whose whole point is to report HOW
// MANY rows a name resolves to.
// TestPgSeedCheckDBFindCropGroupByNameReturnsAllCandidatesWhenAmbiguous
// deliberately seeds two of them (N04 and N05, distinct group_codes, so the
// upserts do not collide); on a shared database every later test that expects
// "甘藍" to resolve to exactly one row gets AMBIGUOUS instead of OK. Nothing
// errors — the assertion simply measures the previous test's fixture.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "pricebook_seed_check_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupTestDB 回傳一個只屬於這個測試的資料庫連線池，裡面是跑完 db/migrations/*.sql
// 的 schema、沒有任何資料列，以及一個 cleanup。簽章與保證跟「每個測試獨佔一個容器」
// 的版本完全相同，五個呼叫點一行未改。
func setupTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	cfg, err := pgxpool.ParseConfig(db.DSN)
	require.NoError(t, err)
	// 對齊 pkg/database/postgres.go 的 production 設定，讓測試走與線上相同的協定路徑。
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
