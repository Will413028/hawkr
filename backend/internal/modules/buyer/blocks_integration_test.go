//go:build integration

package buyer_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func isBlockingStore(t *testing.T, f *fixture, buyerID, storeID int64) bool {
	t.Helper()
	got, err := f.module.IsBlockingStore(context.Background(), buyerID, storeID)
	must(t, err)
	return got
}

func isBlockedBetween(t *testing.T, f *fixture, a, b int64) bool {
	t.Helper()
	got, err := f.module.IsBlockedBetween(context.Background(), a, b)
	must(t, err)
	return got
}

// 封鎖與解除是冪等的，回「有沒有改變」：重複封鎖回 false，解除沒封鎖的對象回 false，而且重複操作不留多餘的列。兩種對象各一組。
func TestBlockAndUnblockAreIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a, b := profile(t, f, 101), profile(t, f, 102)
	const storeID = 7

	if changed, err := f.module.UnblockStore(ctx, a.BuyerID, storeID); err != nil || changed {
		t.Fatalf("解除沒有封鎖的店要回 false：%v %v", changed, err)
	}
	if changed, err := f.module.BlockStore(ctx, a.BuyerID, storeID); err != nil || !changed {
		t.Fatalf("第一次封鎖店要回 true：%v %v", changed, err)
	}
	if changed, err := f.module.BlockStore(ctx, a.BuyerID, storeID); err != nil || changed {
		t.Fatalf("重複封鎖店要回 false：%v %v", changed, err)
	}
	if n := count(t, f, "SELECT count(*) FROM buyer.blocked_stores WHERE blocker_buyer_id = $1", a.BuyerID); n != 1 {
		t.Fatalf("重複封鎖只有一列：%d", n)
	}
	if changed, err := f.module.UnblockStore(ctx, a.BuyerID, storeID); err != nil || !changed {
		t.Fatalf("解除要回 true：%v %v", changed, err)
	}
	if changed, err := f.module.UnblockStore(ctx, a.BuyerID, storeID); err != nil || changed {
		t.Fatalf("重複解除要回 false：%v %v", changed, err)
	}
	if isBlockingStore(t, f, a.BuyerID, storeID) {
		t.Fatal("解除之後不再封鎖")
	}

	if changed, err := f.module.UnblockBuyer(ctx, a.BuyerID, b.BuyerID); err != nil || changed {
		t.Fatalf("解除沒有封鎖的買家要回 false：%v %v", changed, err)
	}
	if changed, err := f.module.BlockBuyer(ctx, a.BuyerID, b.BuyerID); err != nil || !changed {
		t.Fatalf("第一次封鎖買家要回 true：%v %v", changed, err)
	}
	if changed, err := f.module.BlockBuyer(ctx, a.BuyerID, b.BuyerID); err != nil || changed {
		t.Fatalf("重複封鎖買家要回 false：%v %v", changed, err)
	}
	if n := count(t, f, "SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = $1", a.BuyerID); n != 1 {
		t.Fatalf("重複封鎖只有一列：%d", n)
	}
	if changed, err := f.module.UnblockBuyer(ctx, a.BuyerID, b.BuyerID); err != nil || !changed {
		t.Fatalf("解除要回 true：%v %v", changed, err)
	}
	if isBlockedBetween(t, f, a.BuyerID, b.BuyerID) {
		t.Fatal("解除之後不再封鎖")
	}
}

func TestBlockRejectsSelfAndInvalidTargets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a := profile(t, f, 111)
	if _, err := f.module.BlockBuyer(ctx, a.BuyerID, a.BuyerID); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("封鎖自己要 InvalidInput：%v", err)
	}
	for name, err := range map[string]error{
		"非正數的對象買家": func() error { _, err := f.module.BlockBuyer(ctx, a.BuyerID, 0); return err }(),
		"非正數的發起者":  func() error { _, err := f.module.BlockBuyer(ctx, 0, 5); return err }(),
		"非正數的店":    func() error { _, err := f.module.BlockStore(ctx, a.BuyerID, 0); return err }(),
		"非正數的店發起者": func() error { _, err := f.module.BlockStore(ctx, 0, 5); return err }(),
	} {
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s：要 InvalidInput，得到 %v", name, err)
		}
	}
	if _, err := f.module.BlockBuyer(ctx, a.BuyerID, a.BuyerID+9999); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("封鎖不存在的買家要 NotFound：%v", err)
	}
	// 資料庫層也拒絕封鎖自己（繞過 use case 的寫入）。
	if _, err := f.database.Conn(ctx).Exec(ctx, "INSERT INTO buyer.blocked_buyers (blocker_buyer_id, blocked_buyer_id, created_at) VALUES ($1, $1, now())", a.BuyerID); err == nil {
		t.Fatal("DB CHECK 要拒絕封鎖自己")
	}
	// 店 ID 與買家 ID 是不同的號碼空間：店號等於自己的買家號碼不是封鎖自己。
	if changed, err := f.module.BlockStore(ctx, a.BuyerID, a.BuyerID); err != nil || !changed {
		t.Fatalf("封鎖與自己同號的店不是封鎖自己：%v %v", changed, err)
	}
	if n := count(t, f, "SELECT (SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = $1) + (SELECT count(*) FROM buyer.blocked_stores WHERE blocker_buyer_id = $1)", a.BuyerID); n != 1 {
		t.Fatalf("被拒絕的封鎖不留列：%d", n)
	}
}

// 買家 7 與店 7 是不同的對象（D2 的撞號缺陷）：兩張表分開，封鎖其一不影響另一個，解除也是。
func TestBuyerAndStoreBlocksWithTheSameIDAreIndependent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a := profile(t, f, 121)
	other := profile(t, f, 122)
	id := other.BuyerID

	if changed, err := f.module.BlockStore(ctx, a.BuyerID, id); err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	if !isBlockingStore(t, f, a.BuyerID, id) || isBlockedBetween(t, f, a.BuyerID, id) {
		t.Fatal("封鎖店不等於封鎖同號的買家")
	}
	if changed, err := f.module.BlockBuyer(ctx, a.BuyerID, id); err != nil || !changed {
		t.Fatalf("同號的買家要能另外封鎖：%v %v", changed, err)
	}
	if changed, err := f.module.UnblockStore(ctx, a.BuyerID, id); err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	if isBlockingStore(t, f, a.BuyerID, id) || !isBlockedBetween(t, f, a.BuyerID, id) {
		t.Fatal("解除店不影響同號的買家")
	}
	if changed, err := f.module.BlockStore(ctx, a.BuyerID, id); err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	if changed, err := f.module.UnblockBuyer(ctx, a.BuyerID, id); err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	if !isBlockingStore(t, f, a.BuyerID, id) || isBlockedBetween(t, f, a.BuyerID, id) {
		t.Fatal("解除買家不影響同號的店")
	}
}

// 買家之間的查詢任一方向有封鎖就成立；無關的買家不受影響；買家封鎖店只看發起者。
func TestIsBlockedBetweenCoversBothDirections(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a, b, c := profile(t, f, 131), profile(t, f, 132), profile(t, f, 133)
	if isBlockedBetween(t, f, a.BuyerID, b.BuyerID) {
		t.Fatal("沒有封鎖")
	}
	if _, err := f.module.BlockBuyer(ctx, a.BuyerID, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	if !isBlockedBetween(t, f, a.BuyerID, b.BuyerID) || !isBlockedBetween(t, f, b.BuyerID, a.BuyerID) {
		t.Fatal("a 封鎖 b：兩個方向的查詢都要成立")
	}
	if isBlockedBetween(t, f, a.BuyerID, c.BuyerID) || isBlockedBetween(t, f, c.BuyerID, b.BuyerID) {
		t.Fatal("無關的買家不受影響")
	}
	// 反方向另外封鎖：解除其一之後另一個方向仍成立。
	if _, err := f.module.BlockBuyer(ctx, b.BuyerID, a.BuyerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.UnblockBuyer(ctx, a.BuyerID, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	if !isBlockedBetween(t, f, a.BuyerID, b.BuyerID) {
		t.Fatal("b 仍封鎖 a")
	}
	if _, err := f.module.IsBlockedBetween(ctx, 0, b.BuyerID); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("非正數的 ID：%v", err)
	}
	if _, err := f.module.BlockStore(ctx, a.BuyerID, 55); err != nil {
		t.Fatal(err)
	}
	if !isBlockingStore(t, f, a.BuyerID, 55) || isBlockingStore(t, f, b.BuyerID, 55) {
		t.Fatal("封鎖店只看發起者")
	}
}

// 兩種名單新的在前，keyset 分頁跨頁不重不漏；同一時間的列以 ID 決定順序；只列發起者自己的。
func TestListBlockedBuyersAndStoresPaginateNewestFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a := profile(t, f, 141)
	other := profile(t, f, 142)
	peers := []int64{profile(t, f, 143).BuyerID, profile(t, f, 144).BuyerID, profile(t, f, 145).BuyerID, profile(t, f, 146).BuyerID, profile(t, f, 147).BuyerID}
	base := f.now
	offsets := []time.Duration{0, 0, time.Minute, 2 * time.Minute, 3 * time.Minute}
	for i, peer := range peers {
		f.now = base.Add(offsets[i])
		if _, err := f.module.BlockBuyer(ctx, a.BuyerID, peer); err != nil {
			t.Fatal(err)
		}
		if _, err := f.module.BlockStore(ctx, a.BuyerID, peer); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.module.BlockBuyer(ctx, other.BuyerID, peers[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.BlockStore(ctx, other.BuyerID, 99); err != nil {
		t.Fatal(err)
	}
	want := []int64{peers[4], peers[3], peers[2], peers[1], peers[0]}

	var gotBuyers []int64
	var afterBuyer *BlockedBuyerCursor
	pages := 0
	for {
		page, err := f.module.ListBlockedBuyers(ctx, BlockedBuyerListInput{BuyerID: a.BuyerID, PageSize: 2, After: afterBuyer})
		must(t, err)
		pages++
		for _, item := range page.Items {
			gotBuyers = append(gotBuyers, item.BuyerID)
		}
		if page.Next == nil {
			break
		}
		afterBuyer = page.Next
	}
	if pages != 3 || !equalIDs(gotBuyers, want) {
		t.Fatalf("買家名單 pages=%d 要 %v，得到 %v", pages, want, gotBuyers)
	}

	var gotStores []int64
	var afterStore *BlockedStoreCursor
	pages = 0
	for {
		page, err := f.module.ListBlockedStores(ctx, BlockedStoreListInput{BuyerID: a.BuyerID, PageSize: 2, After: afterStore})
		must(t, err)
		pages++
		for _, item := range page.Items {
			gotStores = append(gotStores, item.StoreID)
		}
		if page.Next == nil {
			break
		}
		afterStore = page.Next
	}
	if pages != 3 || !equalIDs(gotStores, want) {
		t.Fatalf("店名單 pages=%d 要 %v，得到 %v", pages, want, gotStores)
	}

	if _, err := f.module.ListBlockedBuyers(ctx, BlockedBuyerListInput{BuyerID: a.BuyerID, After: &BlockedBuyerCursor{BuyerID: 0, Time: base}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("不合法的買家 cursor：%v", err)
	}
	if _, err := f.module.ListBlockedStores(ctx, BlockedStoreListInput{BuyerID: a.BuyerID, After: &BlockedStoreCursor{StoreID: 1}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("不合法的店 cursor：%v", err)
	}
	if _, err := f.module.ListBlockedStores(ctx, BlockedStoreListInput{BuyerID: a.BuyerID, PageSize: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("負的頁大小：%v", err)
	}
}

func equalIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// 買家刪帳（關閉）時兩個方向都刪：他發起的兩種封鎖，以及以他為對象的買家封鎖買家；別人之間的封鎖與別人封鎖店的列不受影響。
func TestClosingABuyerDeletesBlocksInBothDirections(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	leaving, other, bystander := profile(t, f, 151), profile(t, f, 152), profile(t, f, 153)
	for _, err := range []error{
		func() error { _, err := f.module.BlockStore(ctx, leaving.BuyerID, 7); return err }(),
		func() error { _, err := f.module.BlockBuyer(ctx, leaving.BuyerID, other.BuyerID); return err }(),
		func() error { _, err := f.module.BlockBuyer(ctx, other.BuyerID, leaving.BuyerID); return err }(),
		func() error { _, err := f.module.BlockBuyer(ctx, bystander.BuyerID, leaving.BuyerID); return err }(),
		func() error { _, err := f.module.BlockBuyer(ctx, other.BuyerID, bystander.BuyerID); return err }(),
		func() error { _, err := f.module.BlockStore(ctx, other.BuyerID, 7); return err }(),
	} {
		must(t, err)
	}
	must(t, f.module.BeginDeletion(ctx, leaving.BuyerID))
	if _, err := f.module.BlockStore(ctx, leaving.BuyerID, 8); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("刪除中的買家不能再封鎖：%v", err)
	}
	if n := count(t, f, "SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = $1 OR blocked_buyer_id = $1", leaving.BuyerID); n != 3 {
		t.Fatalf("申請刪除時還沒清：%d", n)
	}
	must(t, f.module.CloseForDeletion(ctx, leaving.BuyerID))
	if n := count(t, f, "SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = $1 OR blocked_buyer_id = $1", leaving.BuyerID); n != 0 {
		t.Fatalf("關閉後他發起與以他為對象的買家封鎖要消失：%d", n)
	}
	if n := count(t, f, "SELECT count(*) FROM buyer.blocked_stores WHERE blocker_buyer_id = $1", leaving.BuyerID); n != 0 {
		t.Fatalf("關閉後他封鎖店的列要消失：%d", n)
	}
	if isBlockedBetween(t, f, other.BuyerID, leaving.BuyerID) || isBlockedBetween(t, f, bystander.BuyerID, leaving.BuyerID) {
		t.Fatal("以他為對象的封鎖要消失")
	}
	if !isBlockedBetween(t, f, other.BuyerID, bystander.BuyerID) || !isBlockingStore(t, f, other.BuyerID, 7) {
		t.Fatal("無關的封鎖不受影響")
	}
	page, err := f.module.ListBlockedBuyers(ctx, BlockedBuyerListInput{BuyerID: other.BuyerID})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].BuyerID != bystander.BuyerID {
		t.Fatalf("對方的名單不再列出已關閉的買家：%+v", page.Items)
	}
	if _, err := f.module.BlockBuyer(ctx, other.BuyerID, leaving.BuyerID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不能封鎖已關閉的買家：%v", err)
	}
}

// pub_blocked_buyers()、pub_blocked_stores() 的列等於各自表的列；消費端的 EXISTS 查詢內聯之後走索引，不是整張表掃描或 Function Scan。
func TestPubBlocksIsInlinedAndUsesTheIndexes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a, b := profile(t, f, 161), profile(t, f, 162)
	if _, err := f.module.BlockStore(ctx, a.BuyerID, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.BlockBuyer(ctx, a.BuyerID, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	store := `SELECT count(*) FROM buyer.pub_blocked_stores() b WHERE b.blocker_buyer_id = $1 AND b.store_id = $2`
	if count(t, f, store, a.BuyerID, 7) != 1 || count(t, f, store, b.BuyerID, 7) != 0 || count(t, f, store, a.BuyerID, b.BuyerID) != 0 {
		t.Fatal("pub_blocked_stores 要依 (發起者, 店) 回列")
	}
	buyers := `SELECT count(*) FROM buyer.pub_blocked_buyers() b WHERE b.blocker_buyer_id = $1 AND b.blocked_buyer_id = $2`
	if count(t, f, buyers, a.BuyerID, b.BuyerID) != 1 || count(t, f, buyers, b.BuyerID, a.BuyerID) != 0 || count(t, f, buyers, a.BuyerID, 7) != 0 {
		t.Fatal("pub_blocked_buyers 要依 (發起者, 對象) 回列")
	}

	// 大量資料（對象買家在 buyers 沒有列，這個連線略過外鍵檢查）。
	err := f.database.InTx(ctx, func(ctx context.Context) error {
		conn := f.database.Conn(ctx)
		for _, q := range []string{
			"SET LOCAL session_replication_role = replica",
		} {
			if _, err := conn.Exec(ctx, q); err != nil {
				return err
			}
		}
		if _, err := conn.Exec(ctx, "INSERT INTO buyer.blocked_stores (blocker_buyer_id, store_id, created_at) SELECT $1, g, now() FROM generate_series(1000, 30000) g", b.BuyerID); err != nil {
			return err
		}
		_, err := conn.Exec(ctx, "INSERT INTO buyer.blocked_buyers (blocker_buyer_id, blocked_buyer_id, created_at) SELECT $1, g, now() FROM generate_series(1000, 30000) g", b.BuyerID)
		return err
	})
	must(t, err)
	for _, table := range []string{"buyer.blocked_stores", "buyer.blocked_buyers"} {
		if _, err := f.database.Conn(ctx).Exec(ctx, "ANALYZE "+table); err != nil {
			t.Fatal(err)
		}
	}
	for name, c := range map[string]struct {
		query string
		args  []any
		index string
	}{
		"觀看者是否封鎖了店": {
			`SELECT 1 WHERE EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() b WHERE b.blocker_buyer_id = $1 AND b.store_id = $2)`,
			[]any{b.BuyerID, int64(2000)}, "blocked_stores_",
		},
		"觀看者與買家之間任一方向有封鎖": {
			`SELECT 1 WHERE EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() b WHERE b.blocker_buyer_id = $1 AND b.blocked_buyer_id = $2)
			   OR EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() b WHERE b.blocker_buyer_id = $2 AND b.blocked_buyer_id = $1)`,
			[]any{b.BuyerID, int64(2000)}, "blocked_buyers_",
		},
		"誰封鎖了這家店": {
			`SELECT b.blocker_buyer_id FROM buyer.pub_blocked_stores() b WHERE b.store_id = $1`,
			[]any{int64(2000)}, "blocked_stores_store",
		},
		"誰封鎖了這位買家": {
			`SELECT b.blocker_buyer_id FROM buyer.pub_blocked_buyers() b WHERE b.blocked_buyer_id = $1`,
			[]any{int64(2000)}, "blocked_buyers_blocked",
		},
	} {
		rows, err := f.database.Conn(ctx).Query(ctx, "EXPLAIN (COSTS OFF) "+c.query, c.args...)
		must(t, err)
		var plan []string
		for rows.Next() {
			var line string
			must(t, rows.Scan(&line))
			plan = append(plan, line)
		}
		must(t, rows.Err())
		text := strings.Join(plan, "\n")
		if strings.Contains(text, "Function Scan") || strings.Contains(text, "Seq Scan") || !strings.Contains(text, c.index) {
			t.Fatalf("%s：要內聯並走索引 %s：\n%s", name, c.index, text)
		}
		t.Logf("%s\n%s", name, text)
	}
}

// 兩位買家同時互相封鎖：各自排他鎖自己、共享鎖對方，若不依 buyer_id 排序會互等（40P01）。固定順序之後每一輪兩邊都成功。
func TestMutualBlocksDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a, b := profile(t, f, 161), profile(t, f, 162)
	for round := 0; round < 100; round++ {
		start := make(chan struct{})
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i, pair := range [][2]int64{{a.BuyerID, b.BuyerID}, {b.BuyerID, a.BuyerID}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = f.module.BlockBuyer(ctx, pair[0], pair[1])
			}()
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("第 %d 輪第 %d 個封鎖失敗：%v", round, i, err)
			}
		}
		if n := count(t, f, "SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id IN ($1, $2)", a.BuyerID, b.BuyerID); n != 2 {
			t.Fatalf("第 %d 輪兩個方向都要有列：%d", round, n)
		}
		for _, pair := range [][2]int64{{a.BuyerID, b.BuyerID}, {b.BuyerID, a.BuyerID}} {
			if _, err := f.module.UnblockBuyer(ctx, pair[0], pair[1]); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// 對象關閉之後封鎖回 NotFound 而且不留列。
func TestBlockingAClosedBuyerFails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a, gone := profile(t, f, 171), profile(t, f, 172)
	must(t, f.module.CloseForDeletion(ctx, gone.BuyerID))
	_, err := f.module.BlockBuyer(ctx, a.BuyerID, gone.BuyerID)
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("封鎖已關閉的買家要 NotFound：%v", err)
	}
	if n := count(t, f, "SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = $1", a.BuyerID); n != 0 {
		t.Fatalf("不留列：%d", n)
	}
}

// 封鎖買家與對象的刪帳關閉同時進行：對象的共享鎖與關閉的排他鎖互斥，鎖到之後才檢查狀態，所以每一輪的結果只有兩種——封鎖回 NotFound 且沒有列，或封鎖
// 先成功、隨後的關閉把列刪掉。結束時不存在以已關閉買家為對象的列。若鎖前先讀對象狀態再判斷，封鎖會在關閉提交前通過檢查、關閉後才寫入，留下這種列。
func TestBlockingWhileTargetClosesLeavesNoRowForClosedBuyer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for round := 0; round < 100; round++ {
		target := profile(t, f, int64(91000+round))
		// 偶數輪發起者 ID 大於對象、奇數輪小於對象，兩個 lockPair 分支各賽一半。
		initiatorID := int64(181000 + round)
		if round%2 == 1 {
			initiatorID = int64(1000 + round)
		}
		a := profile(t, f, initiatorID)
		start := make(chan struct{})
		var blockErr, closeErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, blockErr = f.module.BlockBuyer(ctx, a.BuyerID, target.BuyerID)
		}()
		go func() {
			defer wg.Done()
			<-start
			closeErr = f.module.CloseForDeletion(ctx, target.BuyerID)
		}()
		close(start)
		wg.Wait()
		must(t, closeErr)
		if blockErr != nil && apperr.KindOf(blockErr) != apperr.KindNotFound {
			t.Fatalf("第 %d 輪封鎖只能成功或 NotFound：%v", round, blockErr)
		}
		if n := count(t, f, "SELECT count(*) FROM buyer.blocked_buyers WHERE blocked_buyer_id = $1", target.BuyerID); n != 0 {
			t.Fatalf("第 %d 輪（封鎖結果 %v）結束時不得留下以已關閉買家為對象的列：%d", round, blockErr, n)
		}
	}
}
