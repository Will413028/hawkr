//go:build integration

package seller_test

import (
	"context"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// pub_retired_stores 只含已退場的店：營運中、關店未退場、等待刪除、不存在都不在其中。LockNotRetired 在沒有 transaction 時拒絕，
// 營運中、關店與等待刪除放行，已退場與不存在回 NotFound。
func TestRetiredStoresAndLockNotRetired(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, active := f.open(t)
	_, pending := f.open(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, pending.StoreID) }))
	retiredAccount, retired := f.open(t)
	_, err := f.module.CloseCurrent(ctx, retiredAccount)
	must(t, err)
	closedAt := f.now
	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	_, err = f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, retired.StoreID) {
		t.Fatal("要先退場")
	}
	_, closed := f.closedStore(t)
	if _, err := f.module.PurgePII(ctx); err != nil || f.retired(t, closed.StoreID) {
		t.Fatalf("剛關店的店不該退場：%v", err)
	}
	isRetired := func(id int64) int64 {
		return f.count(t, "SELECT count(*) FROM seller.pub_retired_stores() r WHERE r.store_id = $1", id)
	}
	if isRetired(active.StoreID) != 0 || isRetired(pending.StoreID) != 0 || isRetired(closed.StoreID) != 0 || isRetired(999999) != 0 {
		t.Fatal("營運中、等待刪除、關店未退場與不存在的店不該在 pub_retired_stores")
	}
	if isRetired(retired.StoreID) != 1 {
		t.Fatal("已退場的店要在 pub_retired_stores")
	}
	if err := f.module.LockNotRetired(ctx, active.StoreID); err == nil {
		t.Fatal("沒有 transaction 時要拒絕：鎖在語句結束就放掉")
	}
	for _, c := range []struct {
		id int64
		ok bool
	}{{active.StoreID, true}, {pending.StoreID, true}, {closed.StoreID, true}, {retired.StoreID, false}, {999999, false}} {
		err := f.db.InTx(ctx, func(ctx context.Context) error { return f.module.LockNotRetired(ctx, c.id) })
		if c.ok {
			must(t, err)
		} else if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("店 %d：要 NotFound，得到 %v", c.id, err)
		}
	}
}

// 守門持有期間，退場的 RetireDueStores（FOR UPDATE SKIP LOCKED）跳過那家店：不選它、也不等它；提交之後下一輪才退場並發 StoreRetired。
func TestLockNotRetiredSkipsTheStoreInRetireBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	retired := record[StoreRetired](f)
	accountID, s := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetireAfter + time.Second)
	selected := false
	f.module.SetPurgeAfterCandidates(func() { selected = true })
	t.Cleanup(func() { f.module.SetPurgeAfterCandidates(nil) })

	err = f.db.InTx(ctx, func(txCtx context.Context) error {
		if err := f.module.LockNotRetired(txCtx, s.StoreID); err != nil {
			return err
		}
		n, err := f.module.PurgePII(ctx)
		if err != nil {
			return err
		}
		if n != 0 || selected || f.retired(t, s.StoreID) || len(retired.all()) != 0 {
			t.Errorf("守門鎖還在時退場要跳過這家店：n=%d selected=%v retired=%v", n, selected, f.retired(t, s.StoreID))
		}
		return nil
	})
	must(t, err)

	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if !f.retired(t, s.StoreID) || len(retired.all()) != 1 {
		t.Fatalf("守門提交之後下一輪要退場並發一次 StoreRetired：retired=%v events=%d", f.retired(t, s.StoreID), len(retired.all()))
	}
	err = f.db.InTx(ctx, func(ctx context.Context) error { return f.module.LockNotRetired(ctx, s.StoreID) })
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("退場之後的寫入要被拒絕：%v", err)
	}
}

// 消費端的 EXISTS 要能把 pub_retired_stores() 內聯（單一 SELECT，計畫測試鎖住）：計畫裡沒有 Function Scan。
func TestPubRetiredStoresIsInlined(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "EXPLAIN SELECT 1 FROM seller.stores o WHERE o.store_id = 1 AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() r WHERE r.store_id = o.store_id)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, line)
	}
	if got := strings.Join(plan, "\n"); strings.Contains(got, "Function Scan") || strings.Contains(got, "pub_retired_stores") {
		t.Fatalf("pub_retired_stores 沒有被內聯：\n%s", got)
	}
}
