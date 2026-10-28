//go:build integration

// 這個 package 的 DB 測試需要本機 Docker daemon，用 `make test-integration`
// （或 `go test -tags=integration ./...`）跑；CI 由
// .github/workflows/integration-tests.yml 的 notification-service job 執行。
//
// 原本 getTestPool 讀 TEST_DATABASE_URL、沒設就 t.Skip，而且沒有 fallback DSN，
// 所以是「必定 skip」。TEST_DATABASE_URL 在整個 repo 的 CI 從未被設定過——這 2 支
// 測試從寫下來到現在一次都沒執行過。加上 deploy-k8s.yml 的 `go test ./...` 沒有
// -v，Go 的 skip 一行都不印，package 照樣顯示 ok，所以沒有人看得出來。
//
// 現在改成與 account / buyer / crm 相同的 testcontainers harness：
// 拿不到 DB 就是紅的，不再有 skip 這個選項。
package postgres

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// The package shares one postgres container and hands each test its own
// database cloned from it, instead of booting a container per test. The
// mechanism and its rationale live in go-pkg/testutil/dbtest — with 73
// migration scripts to copy, this service is on the expensive end of that.
//
// What is specific to this package: neither test rolls back, and
// TestSystemNotificationLogRepository_UpdateStatus asserts against
// List(ctx, "TERMS_UPDATE", 10, 0) — a LIMIT 10 read of the whole table. That
// assertion holds today because exactly one row of that type exists; on a
// shared database it would start depending on how many rows the tests before it
// happened to leave behind.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "notif_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// getTestPool 回傳一個只屬於這個測試的資料庫連線池，裡面是跑完 db/migrations/*.sql
// 的 schema。連線走 database.NewPool——與 production（di.go:29）同一條路徑，含
// QueryExecModeExec，所以測到的參數推斷行為與線上一致。
func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	db, drop := pg.Clone(ctx, t)

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		drop()
	})
	return pool
}
