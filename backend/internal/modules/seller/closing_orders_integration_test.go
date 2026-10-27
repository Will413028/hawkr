//go:build integration

package seller_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 自助關店遇到未結訂單（含未付款與售後處理中）回 FailedPrecondition（舊 9205 的語意），店家與 SELLER 都不動；沒有未結訂單
// 時可以關。拿掉 CloseCurrent 的訂單檢查，第一段會直接關店。
func TestSelfCloseIsRejectedWhileTheStoreHasOpenOrders(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	f.orders.open = true
	_, err := f.module.CloseCurrent(ctx, accountID)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("err=%v", err)
	}
	cur, err := f.module.CurrentStore(ctx, accountID)
	must(t, err)
	if cur.Status != Active || cur.StoreID != s.StoreID || !f.isSeller(t, accountID) {
		t.Fatalf("store=%+v seller=%v", cur, f.isSeller(t, accountID))
	}
	if len(f.orders.calls) != 1 || f.orders.calls[0] != s.StoreID {
		t.Fatalf("must ask with the store_id, not the account: %v", f.orders.calls)
	}
	f.orders.open = false
	closed, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if closed.Status != Closed || f.isSeller(t, accountID) {
		t.Fatalf("closed=%+v", closed)
	}
}

// 查詢失敗時 fail-closed：回 Unavailable、不關店。
func TestSelfCloseFailsClosedWhenOrdersCannotBeChecked(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	f.orders.err = errors.New("offline")
	_, err := f.module.CloseCurrent(ctx, accountID)
	if apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("err=%v", err)
	}
	if cur, err := f.module.CurrentStore(ctx, accountID); err != nil || cur.Status != Active || !f.isSeller(t, accountID) {
		t.Fatalf("store=%+v err=%v", cur, err)
	}
}

// 等待刪除的店家不能自己關，也不會去問訂單（錯誤先於遠端呼叫）。
func TestSelfCloseOfAPendingStoreDoesNotAskOrders(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	must(t, f.module.BeginDeletion(ctx, s.StoreID))
	if _, err := f.module.CloseCurrent(ctx, accountID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("err=%v", err)
	}
	if len(f.orders.calls) != 0 {
		t.Fatalf("asked: %v", f.orders.calls)
	}
}

// 查完訂單之後、進交易之前店家關閉又重新入駐：查到的訂單不屬於現在的店家，不關新店、要求重試。
func TestSelfCloseRechecksTheStintInsideTheTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, first := f.open(t)
	f.orders.onCall = func() {
		if _, err := f.module.CloseCurrent(ctx, accountID); err != nil {
			t.Error(err)
			return
		}
		if _, err := f.module.Apply(ctx, validApply(accountID)); err != nil {
			t.Error(err)
		}
	}
	_, err := f.module.CloseCurrent(ctx, accountID)
	if apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("err=%v", err)
	}
	cur, err := f.module.CurrentStore(ctx, accountID)
	must(t, err)
	if cur.StoreID == first.StoreID || cur.Status != Active {
		t.Fatalf("new store was closed: %+v", cur)
	}
}
