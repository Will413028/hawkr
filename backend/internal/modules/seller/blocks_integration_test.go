//go:build integration

package seller_test

import (
	"context"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 封鎖與解除是冪等的，回「有沒有改變」；店家不存在回 NotFound，不留列。
func TestStoreBlockAndUnblockAreIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)

	if changed, err := f.module.UnblockBuyer(ctx, s.StoreID, 31); err != nil || changed {
		t.Fatalf("解除沒有封鎖的買家要回 false：%v %v", changed, err)
	}
	if changed, err := f.module.BlockBuyer(ctx, s.StoreID, 31); err != nil || !changed {
		t.Fatalf("第一次封鎖要回 true：%v %v", changed, err)
	}
	if changed, err := f.module.BlockBuyer(ctx, s.StoreID, 31); err != nil || changed {
		t.Fatalf("重複封鎖要回 false：%v %v", changed, err)
	}
	var rows int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM seller.store_blocks WHERE store_id = $1", s.StoreID).Scan(&rows))
	if rows != 1 {
		t.Fatalf("重複封鎖只有一列：%d", rows)
	}
	if changed, err := f.module.UnblockBuyer(ctx, s.StoreID, 31); err != nil || !changed {
		t.Fatalf("解除要回 true：%v %v", changed, err)
	}
	if changed, err := f.module.UnblockBuyer(ctx, s.StoreID, 31); err != nil || changed {
		t.Fatalf("重複解除要回 false：%v %v", changed, err)
	}
	if _, err := f.module.BlockBuyer(ctx, s.StoreID+1000, 31); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("店家不存在要 NotFound：%v", err)
	}
	for _, c := range [][2]int64{{0, 1}, {1, 0}, {-1, 1}} {
		if _, err := f.module.BlockBuyer(ctx, c[0], c[1]); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("非正數的 ID %v：%v", c, err)
		}
	}
}

// 封鎖是單向且依 (店, 買家) 配對：店 A 封鎖買家 B，不代表店 B 封鎖買家 A，也不影響別家店。批次只回封鎖了該買家的店。
func TestStoreBlocksAreDirectedAndBatchedPerBuyer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	_, b := f.open(t)
	_, c := f.open(t)
	buyer := int64(77)

	if _, err := f.module.BlockBuyer(ctx, a.StoreID, buyer); err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.BlockBuyer(ctx, c.StoreID, buyer); err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.BlockBuyer(ctx, b.StoreID, 78); err != nil {
		t.Fatal(err)
	}
	check := func(storeID, buyerID int64) bool {
		got, err := f.module.StoreBlocksBuyer(ctx, storeID, buyerID)
		must(t, err)
		return got
	}
	if !check(a.StoreID, buyer) || check(b.StoreID, buyer) || check(a.StoreID, 78) {
		t.Fatal("只有封鎖了該買家的店成立")
	}
	if check(int64(77), a.StoreID) {
		t.Fatal("店與買家的號碼空間不互換：買家 77 封鎖不是店 77")
	}

	got, err := f.module.StoresBlockingBuyer(ctx, buyer, []int64{a.StoreID, b.StoreID, c.StoreID, 999999})
	must(t, err)
	if len(got) != 2 || !got[a.StoreID] || !got[c.StoreID] || got[b.StoreID] {
		t.Fatalf("批次要只含封鎖了該買家的店：%v", got)
	}
	if got, err = f.module.StoresBlockingBuyer(ctx, buyer, nil); err != nil || len(got) != 0 {
		t.Fatalf("空輸入是空結果：%v %v", got, err)
	}
	over := make([]int64, MaxBatchSize+1)
	for i := range over {
		over[i] = int64(i + 1)
	}
	if _, err := f.module.StoresBlockingBuyer(ctx, buyer, over); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("超過批次上限：%v", err)
	}
	if _, err := f.module.StoresBlockingBuyer(ctx, 0, []int64{1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("非正數的買家：%v", err)
	}
}

// 名單新的在前，keyset 分頁跨頁不重不漏；同時間的列以買家 ID 決定順序；只列該店的。
func TestListBlockedBuyersPaginatesNewestFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, other := f.open(t)
	base := f.now
	for _, e := range []struct {
		buyer int64
		at    time.Time
	}{{5, base}, {9, base}, {3, base.Add(time.Minute)}, {8, base.Add(2 * time.Minute)}, {1, base.Add(3 * time.Minute)}} {
		f.now = e.at
		if _, err := f.module.BlockBuyer(ctx, s.StoreID, e.buyer); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.module.BlockBuyer(ctx, other.StoreID, 99); err != nil {
		t.Fatal(err)
	}
	want := []int64{1, 8, 3, 9, 5}
	var got []int64
	var after *BlockedBuyerCursor
	pages := 0
	for {
		page, err := f.module.ListBlockedBuyers(ctx, BlockedBuyerListInput{StoreID: s.StoreID, PageSize: 2, After: after})
		must(t, err)
		pages++
		for _, item := range page.Items {
			got = append(got, item.BuyerID)
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	if pages != 3 || len(got) != len(want) {
		t.Fatalf("pages=%d got=%v", pages, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("順序 %d：要 %v，得到 %v", i, want, got)
		}
	}
	if _, err := f.module.ListBlockedBuyers(ctx, BlockedBuyerListInput{StoreID: s.StoreID, After: &BlockedBuyerCursor{BuyerID: 1}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("不合法的 cursor：%v", err)
	}
	if _, err := f.module.ListBlockedBuyers(ctx, BlockedBuyerListInput{StoreID: 0}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("非正數的店：%v", err)
	}
}

// 買家刪帳時各店對他的封鎖由 ClearBlocksAgainstBuyer 刪掉；別的買家的封鎖不受影響。
func TestClearBlocksAgainstBuyerDeletesOnlyThatBuyersRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	_, b := f.open(t)
	for _, c := range [][2]int64{{a.StoreID, 91}, {b.StoreID, 91}, {a.StoreID, 92}} {
		if _, err := f.module.BlockBuyer(ctx, c[0], c[1]); err != nil {
			t.Fatal(err)
		}
	}
	must(t, f.module.ClearBlocksAgainstBuyer(ctx, 91))
	for _, c := range []struct {
		store, buyer int64
		want         bool
	}{{a.StoreID, 91, false}, {b.StoreID, 91, false}, {a.StoreID, 92, true}} {
		got, err := f.module.StoreBlocksBuyer(ctx, c.store, c.buyer)
		must(t, err)
		if got != c.want {
			t.Fatalf("店 %d 對買家 %d：要 %v", c.store, c.buyer, c.want)
		}
	}
	if err := f.module.ClearBlocksAgainstBuyer(ctx, 0); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("非正數的買家：%v", err)
	}
}
