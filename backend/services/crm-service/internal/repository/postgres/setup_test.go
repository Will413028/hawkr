//go:build integration

// 這些測試需要本機 Docker daemon，用 `make test-integration`
// （或 `go test -tags=integration ./...`）跑；CI 由
// .github/workflows/integration-tests.yml 的 crm-service job 執行。
//
// 原本這裡讀 TEST_DATABASE_URL、沒設就 fallback 到 localhost:5432 的 hawkr_crm，
// 連不上則 t.Skipf。TEST_DATABASE_URL 在整個 repo 的 CI 從未被設定過，runner 上也
// 沒有常駐 postgres，所以這 19 支測試的實際狀態是「本機剛好有 DB 才跑、CI 永遠 skip」
// ——而 deploy-k8s.yml 的 `go test ./...` 沒有 -v，Go 的 skip 一行都不印，package
// 照樣顯示 ok。缺席被當成了綠燈。
//
// 因此改成與 account / buyer 相同的 testcontainers harness：
// 拿不到 DB 就是紅的（require.NoError），不再有 skip 這個選項。
package postgres_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

var errTestRollback = errors.New("rollback")

// testIDSeq backs nextTestID.
var testIDSeq atomic.Int64

// nextTestID returns a unique non-zero id for rows a test inserts.
//
// crm_customers.id and crm_tags.id are plain BIGINT PRIMARY KEY with no
// DEFAULT — the bigserial default was dropped in
// 20260405202849_crm_drop_serial.sql, and production supplies a TSID from the
// caller instead. A struct literal that leaves ID unset therefore inserts
// id=0, which silently works for the first row and then collides on the
// primary key for every row after it.
//
// The counter only has to yield distinct values, so it is independent of test
// execution order and safe under t.Parallel().
func nextTestID() int64 {
	return 9_000_000_000_000 + testIDSeq.Add(1)
}

// The package shares one postgres container and hands each test its own
// database cloned from it, instead of booting a container per test. The
// mechanism and its rationale live in go-pkg/testutil/dbtest.
//
// What is specific to this package: most tests here wrap their work in runInTx
// and roll back, so they would survive sharing a database;
// TestCrmRepository_LockOrderForNetting_SerializesConcurrentCallers
// deliberately commits, and ListAndCountCustomers / ListTags assert on counts
// that are statements about the whole table rather than about one row. A
// per-test database keeps those assertions meaning what they say no matter what
// ran before them.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "crm_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// testPool 回傳一個只屬於這個測試的資料庫連線池，裡面是跑完 db/migrations/*.sql
// 的 schema、沒有任何資料列。連線走 database.NewPool——與 production 同一條路徑
// （含 QueryExecModeExec），所以測到的參數推斷行為與線上一致。
func testPool(t *testing.T) *pgxpool.Pool {
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

// runInTx runs fn inside a transaction that is always rolled back at the end,
// so tests remain isolated and leave no residue in the database.
func runInTx(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context)) {
	t.Helper()
	txm := database.NewTxManager(pool)
	err := txm.WithTx(context.Background(), func(ctx context.Context) error {
		fn(ctx)
		return errTestRollback
	})
	if !errors.Is(err, errTestRollback) {
		t.Fatalf("unexpected transaction error: %v", err)
	}
}
