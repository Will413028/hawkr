//go:build integration

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var errBoom = errors.New("boom")

func setup(t *testing.T) *db.DB {
	t.Helper()
	d := pg.NewDB(t)
	if _, err := d.Conn(context.Background()).Exec(context.Background(), "CREATE TABLE items (id int PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	return d
}

func insert(ctx context.Context, d *db.DB, id int) error {
	_, err := d.Conn(ctx).Exec(ctx, "INSERT INTO items (id) VALUES ($1)", id)
	return err
}

func count(t *testing.T, d *db.DB) int {
	t.Helper()
	var n int
	if err := d.Conn(context.Background()).QueryRow(context.Background(), "SELECT count(*) FROM items").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInTxCommits(t *testing.T) {
	t.Parallel()
	d := setup(t)
	err := d.InTx(context.Background(), func(ctx context.Context) error {
		return insert(ctx, d, 1)
	})
	if err != nil || count(t, d) != 1 {
		t.Fatalf("err = %v, rows = %d，預期 commit 一列", err, count(t, d))
	}
}

func TestInTxRollsBackOnError(t *testing.T) {
	t.Parallel()
	d := setup(t)
	err := d.InTx(context.Background(), func(ctx context.Context) error {
		if err := insert(ctx, d, 1); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v，預期原本的錯誤", err)
	}
	if n := count(t, d); n != 0 {
		t.Fatalf("rows = %d，預期 rollback", n)
	}
}

func TestNestedInTxJoinsTheOuterTransaction(t *testing.T) {
	t.Parallel()
	d := setup(t)
	err := d.InTx(context.Background(), func(outer context.Context) error {
		outerTx, _ := db.TxFrom(outer)
		if err := insert(outer, d, 1); err != nil {
			return err
		}
		if err := d.InTx(outer, func(inner context.Context) error {
			if innerTx, _ := db.TxFrom(inner); innerTx != outerTx {
				t.Error("內層沒有加入外層的 transaction")
			}
			return insert(inner, d, 2)
		}); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if n := count(t, d); n != 0 {
		t.Fatalf("rows = %d，外層 rollback 時內層的寫入也要一起 rollback", n)
	}
}

func TestInTxRollsBackOnPanic(t *testing.T) {
	t.Parallel()
	d := setup(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic 應該繼續往外傳")
			}
		}()
		_ = d.InTx(context.Background(), func(ctx context.Context) error {
			if err := insert(ctx, d, 1); err != nil {
				return err
			}
			panic("boom")
		})
	}()
	if n := count(t, d); n != 0 {
		t.Fatalf("rows = %d，預期 rollback", n)
	}
}

func TestConnOutsideTransactionAutocommits(t *testing.T) {
	t.Parallel()
	d := setup(t)
	if err := insert(context.Background(), d, 1); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d); n != 1 {
		t.Fatalf("rows = %d", n)
	}
}

// 快照內的多個語句看見同一份資料：快照開始之後別的連線提交的改動，同一次讀取內看不到；快照結束後看得到。
func TestInReadSnapshotSeesOneSnapshot(t *testing.T) {
	t.Parallel()
	d := setup(t)
	ctx := context.Background()
	if err := insert(ctx, d, 1); err != nil {
		t.Fatal(err)
	}
	var before, after int
	err := d.InReadSnapshot(ctx, func(ctx context.Context) error {
		if err := d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM items").Scan(&before); err != nil {
			return err
		}
		if err := insert(context.Background(), d, 2); err != nil { // 另一條連線，立刻提交
			return err
		}
		return d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM items").Scan(&after)
	})
	if err != nil || before != 1 || after != 1 {
		t.Fatalf("err=%v before=%d after=%d", err, before, after)
	}
	if got := count(t, d); got != 2 {
		t.Fatalf("after the snapshot count=%d", got)
	}
}

// 快照是唯讀的：寫入以 read_only_sql_transaction（SQLSTATE 25006）失敗，錯誤傳回並 rollback。
func TestInReadSnapshotIsReadOnly(t *testing.T) {
	t.Parallel()
	d := setup(t)
	err := d.InReadSnapshot(context.Background(), func(ctx context.Context) error { return insert(ctx, d, 1) })
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" || count(t, d) != 0 {
		t.Fatalf("err=%v count=%d", err, count(t, d))
	}
}

// fn 回傳錯誤：錯誤原樣傳回，連線已歸還連線池、沒有留下進行中的 transaction（唯讀的 rollback 與 commit 看不出差別，
// 保證的是 transaction 結束、連線釋放）。後續查詢帶短期限，連線洩漏時會失敗而不是卡住。
func TestInReadSnapshotReturnsTheErrorAndReleasesTheConnection(t *testing.T) {
	t.Parallel()
	d := setup(t)
	ctx := context.Background()
	err := d.InReadSnapshot(ctx, func(ctx context.Context) error {
		if got := count(t, d); got != 0 {
			t.Errorf("count=%d", got)
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err=%v", err)
	}
	if n := d.Pool().Stat().AcquiredConns(); n != 0 {
		t.Fatalf("acquired connections=%d，快照結束後連線要歸還", n)
	}
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var open int
	if err := d.Conn(short).QueryRow(short, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state = 'idle in transaction'").Scan(&open); err != nil || open != 0 {
		t.Fatalf("open transactions=%d err=%v", open, err)
	}
}

// fn panic：往外傳，transaction 已 rollback、連線已釋放。
func TestInReadSnapshotRollsBackOnPanicAndRepanics(t *testing.T) {
	t.Parallel()
	d := setup(t)
	ctx := context.Background()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic 應該繼續往外傳")
			}
		}()
		_ = d.InReadSnapshot(ctx, func(ctx context.Context) error {
			var one int
			if err := d.Conn(ctx).QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
				t.Error(err)
			}
			panic("boom")
		})
	}()
	var open int
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state = 'idle in transaction'").Scan(&open); err != nil || open != 0 {
		t.Fatalf("open transactions=%d err=%v", open, err)
	}
}

// 已經在 transaction 內時拒絕，不加入外層，也不執行 fn。
func TestInReadSnapshotRejectsAnOuterTransaction(t *testing.T) {
	t.Parallel()
	d := setup(t)
	called := false
	err := d.InTx(context.Background(), func(ctx context.Context) error {
		return d.InReadSnapshot(ctx, func(context.Context) error { called = true; return nil })
	})
	if !errors.Is(err, db.ErrSnapshotInTx) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}
