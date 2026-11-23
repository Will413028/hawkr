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

// 守門（LockNotRetired，FOR KEY SHARE）與同一家店的業務寫入（stint.Lock，FOR NO KEY UPDATE）不互相等待：守門的 transaction 還沒結束，
// 業務寫入就能完成。業務寫入若改成 FOR UPDATE，會等到 context 的期限而失敗。
func TestOwnerGuardDoesNotBlockBusinessWritesOnTheSameStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	err := f.db.InTx(ctx, func(txCtx context.Context) error {
		if err := f.module.LockNotRetired(txCtx, s.StoreID); err != nil {
			return err
		}
		writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := f.module.UpdateProfile(writeCtx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, Description: ptr("守門不擋業務寫入")})
		return err
	})
	must(t, err)
}

// 對已退場與不存在的店，守門不取任何鎖：同一個 transaction 內另一條連線對該列 FOR UPDATE NOWAIT 立刻成功，
// 所以打到已退場店家的流量不會讓 RetireDueStores 與個資清理（SKIP LOCKED）跳過它。
func TestLockNotRetiredOnRetiredStoreHoldsNoLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetireAfter + time.Second)
	_, err = f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, s.StoreID) {
		t.Fatal("要先退場")
	}
	err = f.db.InTx(ctx, func(txCtx context.Context) error {
		if err := f.module.LockNotRetired(txCtx, s.StoreID); apperr.KindOf(err) != apperr.KindNotFound {
			t.Errorf("已退場要 NotFound：%v", err)
		}
		other, err := f.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer other.Rollback(ctx) //nolint:errcheck
		var id int64
		return other.QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 FOR UPDATE NOWAIT", s.StoreID).Scan(&id)
	})
	if err != nil {
		t.Fatalf("守門對已退場的店不該持鎖：%v", err)
	}
}

// ShareLockStore 回報店家是否已退場：營運中、關店與等待刪除不算退場，已退場回 retired=true，不存在回 NotFound，沒有 transaction 時拒絕；
// 持有共享鎖期間退場的 RetireDueStores 跳過那家店，提交之後下一輪才退場。
func TestShareLockStoreReportsRetiredAndHoldsTheStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, active := f.open(t)
	_, pending := f.open(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, pending.StoreID) }))
	retiredAccount, retired := f.open(t)
	_, err := f.module.CloseCurrent(ctx, retiredAccount)
	must(t, err)
	closingAccount, closing := f.open(t)
	_, err = f.module.CloseCurrent(ctx, closingAccount)
	must(t, err)
	f.now = f.now.Add(StoreRetireAfter + time.Second)

	if _, err := f.module.ShareLockStore(ctx, active.StoreID); err == nil {
		t.Fatal("沒有 transaction 時要拒絕：鎖在語句結束就放掉")
	}
	err = f.db.InTx(ctx, func(txCtx context.Context) error {
		for _, id := range []int64{active.StoreID, pending.StoreID} {
			if gone, err := f.module.ShareLockStore(txCtx, id); err != nil || gone {
				t.Errorf("店 %d 未退場：retired=%v err=%v", id, gone, err)
			}
		}
		if _, err := f.module.ShareLockStore(txCtx, 999999); apperr.KindOf(err) != apperr.KindNotFound {
			t.Errorf("不存在的店要 NotFound：%v", err)
		}
		if gone, err := f.module.ShareLockStore(txCtx, closing.StoreID); err != nil || gone {
			t.Errorf("關店未退場的店：retired=%v err=%v", gone, err)
		}
		// 鎖住 closing 之後，退場這一輪要跳過它。
		if _, err := f.module.PurgePII(ctx); err != nil {
			return err
		}
		if f.retired(t, closing.StoreID) {
			t.Error("共享鎖還在時退場要跳過這家店")
		}
		return nil
	})
	must(t, err)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if !f.retired(t, closing.StoreID) || !f.retired(t, retired.StoreID) {
		t.Fatal("提交之後下一輪要退場")
	}
	err = f.db.InTx(ctx, func(ctx context.Context) error {
		for _, id := range []int64{retired.StoreID, closing.StoreID} {
			if gone, err := f.module.ShareLockStore(ctx, id); err != nil || !gone {
				t.Errorf("店 %d 已退場：retired=%v err=%v", id, gone, err)
			}
		}
		return nil
	})
	must(t, err)
}
