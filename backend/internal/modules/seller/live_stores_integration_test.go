//go:build integration

package seller_test

import (
	"context"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ListLiveStoreAccounts 依 store_id 由小到大做 keyset 分頁，收營運中、關店未退場與等待刪除的店，排除已退場（retired_at 有值）的，每筆帶所屬帳號；分頁參數不合法回 InvalidInput。
// mutation：查詢的 retired_at IS NULL 改成 true → 已退場的店也被列出，失敗。
func TestListLiveStoreAccountsPagesByStoreIDAndExcludesRetired(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	activeAccount, active := f.open(t)
	_, pending := f.open(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, pending.StoreID) }))
	retiredAccount, retired := f.open(t)
	_, err := f.module.CloseCurrent(ctx, retiredAccount)
	must(t, err)
	f.now = f.now.Add(StoreRetireAfter + time.Second)
	_, err = f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, retired.StoreID) {
		t.Fatal("要先退場")
	}
	closedAccount, closed := f.closedStore(t)
	if f.retired(t, closed.StoreID) {
		t.Fatal("剛關店的店不該退場")
	}

	want := []StoreAccount{{StoreID: active.StoreID, AccountID: activeAccount}, {StoreID: pending.StoreID, AccountID: pending.AccountID}, {StoreID: closed.StoreID, AccountID: closedAccount}}
	var got []StoreAccount
	after, pages := int64(0), 0
	for {
		page, err := f.module.ListLiveStoreAccounts(ctx, after, 2)
		must(t, err)
		pages++
		got = append(got, page...)
		if len(page) < 2 {
			break
		}
		after = page[len(page)-1].StoreID
	}
	if len(got) != len(want) || pages != 2 {
		t.Fatalf("得到 %v（%d 頁），預期 %v（2 頁）", got, pages, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 筆 = %v，預期 %v", i, got[i], want[i])
		}
	}
	for name, c := range map[string]struct {
		after int64
		limit int
	}{"limit 0": {0, 0}, "limit 過大": {0, MaxLiveStoresPage + 1}, "after 為負": {-1, 1}} {
		if _, err := f.module.ListLiveStoreAccounts(ctx, c.after, c.limit); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v，預期 InvalidInput", name, err)
		}
	}
}
