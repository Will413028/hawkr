package lockorder

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestClassifyFindsRowLocksOnExistingRows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
		want []Lock
	}{
		{"select for update", "SELECT 1 FROM seller.stores WHERE store_id = $1 FOR UPDATE",
			[]Lock{{"seller", "stores", "FOR UPDATE", true}}},
		{"for share with join locks every table in from", "SELECT 1 FROM seller.stores s JOIN seller.measures m ON m.store_id = s.store_id FOR SHARE",
			[]Lock{{"seller", "stores", "FOR SHARE", true}, {"seller", "measures", "FOR SHARE", true}}},
		{"of alias locks only the named table", "SELECT 1 FROM seller.stores s JOIN seller.measures m ON m.store_id = s.store_id FOR NO KEY UPDATE OF m",
			[]Lock{{"seller", "measures", "FOR NO KEY UPDATE", true}}},
		{"skip locked does not wait", "SELECT 1 FROM identity.accounts FOR KEY SHARE SKIP LOCKED",
			[]Lock{{"identity", "accounts", "FOR KEY SHARE", false}}},
		{"nowait does not wait", "SELECT 1 FROM identity.accounts FOR UPDATE NOWAIT",
			[]Lock{{"identity", "accounts", "FOR UPDATE", false}}},
		{"update", "-- name: SetX :exec\nUPDATE buyer.buyers SET x = 1 WHERE buyer_id = $1",
			[]Lock{{"buyer", "buyers", "UPDATE", true}}},
		{"delete inside a cte", "WITH d AS (DELETE FROM social.posts WHERE id = $1 RETURNING id) SELECT count(*) FROM d",
			[]Lock{{"social", "posts", "DELETE", true}}},
		{"upsert", "INSERT INTO seller.payment_accounts (store_id) VALUES ($1) ON CONFLICT (store_id) DO UPDATE SET updated_at = $2",
			[]Lock{{"seller", "payment_accounts", "INSERT ON CONFLICT DO UPDATE", true}}},
		{"locking subselect", "UPDATE social.posts SET hidden = true WHERE id IN (SELECT id FROM social.posts WHERE author = $1 FOR UPDATE)",
			[]Lock{{"social", "posts", "UPDATE", true}, {"social", "posts", "FOR UPDATE", true}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := Classify(c.sql)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(sorted(got), sorted(c.want)) {
				t.Fatalf("Classify = %+v，預期 %+v", got, c.want)
			}
		})
	}
}

func TestClassifyIgnoresStatementsThatDoNotLockExistingRows(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"SELECT * FROM seller.stores WHERE store_id = $1",
		"INSERT INTO seller.reports (id) VALUES ($1)",
		"INSERT INTO seller.reports (id) VALUES ($1) ON CONFLICT DO NOTHING",
		"SELECT pg_advisory_xact_lock(hashtextextended('seller.product_views', 0))",
		"WITH s AS (SELECT * FROM seller.stores) SELECT * FROM s FOR UPDATE OF s", // CTE 不帶 schema，鎖的是 CTE 的結果
	} {
		got, err := Classify(sql)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("Classify(%q) = %+v，預期沒有列鎖", sql, got)
		}
	}
}

func TestCheckerFlagsWaitingForALowerModuleAfterLockingAHigherOne(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(int)
	c.observe(conn, "begin")
	c.observe(conn, "-- name: LockStore :one\nSELECT 1 FROM seller.stores WHERE store_id = $1 FOR UPDATE")
	c.observe(conn, "-- name: SuspendAccount :exec\nUPDATE identity.accounts SET suspended_at = $2 WHERE account_id = $1")
	c.observe(conn, "commit")

	vs := c.Violations()
	if len(vs) != 1 {
		t.Fatalf("violations = %v，預期一筆", vs)
	}
	v := vs[0]
	if v.Held.Statement != "LockStore" || v.Then.Statement != "SuspendAccount" || v.Held.Lock.Schema != "seller" || v.Then.Lock.Schema != "identity" {
		t.Fatalf("violation = %+v", v)
	}
	if !strings.Contains(v.String(), "seller.stores") || !strings.Contains(v.String(), "identity.accounts") {
		t.Fatalf("String() = %q", v.String())
	}
}

func TestCheckerAcceptsBottomUpOrderAndIgnoresStatementsOutsideTransactions(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	a, b := new(int), new(int)
	c.observe(a, "begin")
	c.observe(a, "SELECT 1 FROM identity.accounts FOR UPDATE")
	c.observe(a, "UPDATE buyer.buyers SET x = 1")
	c.observe(a, "UPDATE seller.stores SET x = 1")
	c.observe(a, "UPDATE social.posts SET x = 1")
	c.observe(a, "UPDATE social.posts SET y = 1") // 同一層再鎖不算倒退（模組內的順序不在這裡檢查）
	c.observe(a, "commit")

	// 另一條連線不在 transaction 裡：每句各自 commit，沒有順序。
	c.observe(b, "UPDATE social.posts SET x = 1")
	c.observe(b, "UPDATE identity.accounts SET x = 1")

	// 上一個 transaction 結束之後，同一條連線的新 transaction 從頭算。
	c.observe(a, "begin")
	c.observe(a, "UPDATE identity.accounts SET x = 1")
	c.observe(a, "rollback")

	if vs := c.Violations(); len(vs) != 0 {
		t.Fatalf("violations = %v，預期沒有", vs)
	}
}

func TestCheckerAllowsReturningToAModuleLockedEarlierInTheSameTransaction(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(int)
	c.observe(conn, "begin")
	c.observe(conn, "-- name: LockLiveAccount :one\nSELECT 1 FROM identity.accounts WHERE account_id = $1 FOR UPDATE") // 根列
	c.observe(conn, "-- name: LockAccountStores :many\nSELECT 1 FROM seller.stores WHERE account_id = $1 FOR UPDATE")
	c.observe(conn, "-- name: RevokeAccountRole :exec\nDELETE FROM identity.account_roles WHERE account_id = $1") // identity 已經鎖過
	c.observe(conn, "-- name: ClearVisit :exec\nDELETE FROM buyer.visit_history WHERE buyer_id = $1")             // buyer 第一次被鎖：倒退
	c.observe(conn, "-- name: LockBuyer :one\nSELECT 1 FROM buyer.buyers WHERE buyer_id = $1 FOR UPDATE")         // buyer 已經鎖過，不再記
	c.observe(conn, "commit")

	vs := c.Violations()
	if len(vs) != 1 || vs[0].Then.Statement != "ClearVisit" || vs[0].Held.Statement != "LockAccountStores" {
		t.Fatalf("violations = %+v，預期只有 buyer 第一次被鎖的那一筆", vs)
	}
}

func TestCheckerFlagsOrchestratorRowLockedBeforeBuyerAndSeller(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(int)
	c.observe(conn, "begin")
	c.observe(conn, "-- name: LockLiveAccount :one\nSELECT 1 FROM identity.accounts WHERE account_id = $1 FOR UPDATE")
	c.observe(conn, "-- name: PendingRequest :one\nSELECT * FROM accountdeletion.deletion_requests WHERE account_id = $1 FOR UPDATE")
	c.observe(conn, "-- name: LockBuyer :one\nSELECT 1 FROM buyer.buyers WHERE buyer_id = $1 FOR UPDATE")             // 倒退，沒有豁免
	c.observe(conn, "-- name: LockAccountStores :many\nSELECT 1 FROM seller.stores WHERE account_id = $1 FOR UPDATE") // 倒退，沒有豁免
	c.observe(conn, "commit")

	vs := c.Violations()
	if len(vs) != 2 || vs[0].Then.Lock.Schema != "buyer" || vs[1].Then.Lock.Schema != "seller" {
		t.Fatalf("violations = %+v，預期 buyer 與 seller 各一筆", vs)
	}
}

func TestCheckerIgnoresNonWaitingAcquisitionsAndUnknownSchemas(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(int)
	c.observe(conn, "begin")
	c.observe(conn, "UPDATE social.posts SET x = 1")
	c.observe(conn, "SELECT 1 FROM identity.accounts FOR UPDATE SKIP LOCKED") // 不等，不會卡在別人手上
	c.observe(conn, "UPDATE river.river_job SET state = 'available'")         // 不是模組
	c.observe(conn, "commit")
	if vs := c.Violations(); len(vs) != 0 {
		t.Fatalf("violations = %v，預期沒有", vs)
	}
}

func TestCheckerRecordsEachViolationOnce(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(int)
	for range 3 {
		c.observe(conn, "begin")
		c.observe(conn, "UPDATE seller.stores SET x = 1")
		c.observe(conn, "UPDATE buyer.buyers SET x = 1")
		c.observe(conn, "commit")
	}
	if vs := c.Violations(); len(vs) != 1 {
		t.Fatalf("violations = %d 筆，預期同一組只記一次", len(vs))
	}
}

func TestCheckerReportsUnparsableSQLInsideTransactions(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(int)
	c.observe(conn, "begin")
	c.observe(conn, "UPDATE seller.stores SET")
	if len(c.Errors()) != 1 {
		t.Fatalf("Errors = %v，預期一筆", c.Errors())
	}
}

func TestEveryModuleAppearsOnce(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, m := range Modules {
		if seen[m] {
			t.Errorf("Modules 重複列出 %s", m)
		}
		seen[m] = true
	}
}

func sorted(ls []Lock) []Lock {
	out := slices.Clone(ls)
	slices.SortFunc(out, func(a, b Lock) int {
		return strings.Compare(a.Schema+"."+a.Table+" "+a.Kind, b.Schema+"."+b.Table+" "+b.Kind)
	})
	return out
}

var (
	_ pgx.BatchTracer    = (*Checker)(nil)
	_ pgx.CopyFromTracer = (*Checker)(nil)
)

func TestCheckerClassifiesEveryStatementOfABatch(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(pgx.Conn)
	ctx := context.Background()
	c.TraceQueryStart(ctx, conn, pgx.TraceQueryStartData{SQL: "begin"})

	b := &pgx.Batch{}
	b.Queue("-- name: LockStore :one\nSELECT 1 FROM seller.stores WHERE store_id = $1 FOR UPDATE", 1)
	b.Queue("-- name: SuspendAccount :exec\nUPDATE identity.accounts SET suspended_at = $2 WHERE account_id = $1", 1, 2)
	c.TraceBatchStart(ctx, conn, pgx.TraceBatchStartData{Batch: b})
	for _, q := range b.QueuedQueries {
		c.TraceBatchQuery(ctx, conn, pgx.TraceBatchQueryData{SQL: q.SQL})
	}
	c.TraceBatchEnd(ctx, conn, pgx.TraceBatchEndData{})
	c.TraceQueryStart(ctx, conn, pgx.TraceQueryStartData{SQL: "commit"})

	vs := c.Violations()
	if len(vs) != 1 {
		t.Fatalf("violations = %v，預期一筆（batch 裡的語句要和單句一樣分類，而且每句只算一次）", vs)
	}
	if vs[0].Held.Statement != "LockStore" || vs[0].Then.Statement != "SuspendAccount" {
		t.Fatalf("violation = %v", vs[0])
	}
}

func TestCheckerBatchOutsideATransactionIsNotChecked(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	b := &pgx.Batch{}
	b.Queue("SELECT 1 FROM seller.stores FOR UPDATE")
	b.Queue("SELECT 1 FROM identity.accounts FOR UPDATE")
	c.TraceBatchStart(context.Background(), new(pgx.Conn), pgx.TraceBatchStartData{Batch: b})
	if vs := c.Violations(); len(vs) != 0 {
		t.Fatalf("violations = %v", vs)
	}
}

func TestCheckerBatchReportsUnparsableSQL(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(pgx.Conn)
	ctx := context.Background()
	c.TraceQueryStart(ctx, conn, pgx.TraceQueryStartData{SQL: "begin"})
	b := &pgx.Batch{}
	b.Queue("SELEKT nonsense")
	c.TraceBatchStart(ctx, conn, pgx.TraceBatchStartData{Batch: b})
	if len(c.Errors()) != 1 {
		t.Fatalf("errors = %v，預期一筆", c.Errors())
	}
}

// COPY FROM 只新增列、沒有 ON CONFLICT，鎖的情形和單純的 INSERT 一樣：不鎖既有的列，所以不算加鎖，
// 也不會讓先前鎖過的順序倒退。
func TestCheckerCopyFromCountsAsAPlainInsert(t *testing.T) {
	t.Parallel()
	c := NewChecker()
	conn := new(pgx.Conn)
	ctx := context.Background()
	c.TraceQueryStart(ctx, conn, pgx.TraceQueryStartData{SQL: "begin"})
	c.TraceQueryStart(ctx, conn, pgx.TraceQueryStartData{SQL: "SELECT 1 FROM seller.stores FOR UPDATE"})
	ctx = c.TraceCopyFromStart(ctx, conn, pgx.TraceCopyFromStartData{TableName: pgx.Identifier{"identity", "accounts"}, ColumnNames: []string{"account_id"}})
	c.TraceCopyFromEnd(ctx, conn, pgx.TraceCopyFromEndData{})
	if vs := c.Violations(); len(vs) != 0 {
		t.Fatalf("violations = %v", vs)
	}
	if len(c.Errors()) != 0 {
		t.Fatalf("errors = %v", c.Errors())
	}
	if locks := CopyFromLocks(pgx.Identifier{"identity", "accounts"}); len(locks) != 0 {
		t.Fatalf("CopyFromLocks = %v", locks)
	}
}
