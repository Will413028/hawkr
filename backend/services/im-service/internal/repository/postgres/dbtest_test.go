//go:build integration

// 這個 package 的 DB 測試需要本機 Docker daemon（testcontainers 起 postgres 重放
// db/migrations）。用 `make test-integration`（見 Makefile）跑，或直接
// `go test -tags=integration ./... -count=1 -v`。機制與
// notification-service 的 internal/repository/postgres/dbtest_test.go 相同，
// 見 pkg/testutil/dbtest 套件註解。
package postgres

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The package shares one postgres container and hands each test its own
// database cloned from it — see pkg/testutil/dbtest for the mechanism.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "im_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// getTestPool 回傳一個只屬於這個測試的資料庫連線池，裡面是跑完 db/migrations/*.sql
// 的 schema。連線走 database.NewPool——與 production（di.go:23-24 的 newPool）
// 同一條路徑。
func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	db, drop := pg.Clone(ctx, t)

	pool, err := database.NewPool(ctx, db.DSN)
	if err != nil {
		t.Fatalf("connect test pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		drop()
	})
	return pool
}
