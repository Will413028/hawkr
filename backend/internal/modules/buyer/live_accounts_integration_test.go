//go:build integration

package buyer_test

import (
	"context"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ListLiveAccounts 依 buyer_id 由小到大做 keyset 分頁，收營運中與刪除中的買家、排除已關閉的，每筆帶所屬帳號；分頁參數不合法回 InvalidInput。
// mutation：查詢的 status <> 'CLOSED' 改成 true → 已關閉的買家也被列出，失敗。
func TestListLiveAccountsPagesByBuyerIDAndExcludesClosed(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	active := profile(t, fixture, 11)
	closing := profile(t, fixture, 12)
	closed := profile(t, fixture, 13)
	last := profile(t, fixture, 14)
	must(t, fixture.module.BeginDeletion(ctx, closing.BuyerID))
	must(t, fixture.module.CloseForDeletion(ctx, closed.BuyerID))

	want := []AccountRef{{BuyerID: active.BuyerID, AccountID: 11}, {BuyerID: closing.BuyerID, AccountID: 12}, {BuyerID: last.BuyerID, AccountID: 14}}
	var got []AccountRef
	after, pages := int64(0), 0
	for {
		page, err := fixture.module.ListLiveAccounts(ctx, after, 2)
		must(t, err)
		pages++
		got = append(got, page...)
		if len(page) < 2 {
			break
		}
		after = page[len(page)-1].BuyerID
	}
	if len(got) != len(want) || pages != 2 {
		t.Fatalf("得到 %v（%d 頁），預期 %v（2 頁）", got, pages, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 筆 = %v，預期 %v", i, got[i], want[i])
		}
	}
	if page, err := fixture.module.ListLiveAccounts(ctx, last.BuyerID, 5); err != nil || len(page) != 0 {
		t.Errorf("最後一個之後 = %v（%v），預期空", page, err)
	}
	for name, c := range map[string]struct {
		after int64
		limit int
	}{"limit 0": {0, 0}, "limit 過大": {0, MaxLiveAccountsPage + 1}, "after 為負": {-1, 1}} {
		if _, err := fixture.module.ListLiveAccounts(ctx, c.after, c.limit); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v，預期 InvalidInput", name, err)
		}
	}
}

// ListLiveFollowerAccounts 只列追蹤指定店家且尚未關閉的買家（營運中與刪除中），依 buyer_id keyset 分頁；分頁參數不合法回 InvalidInput。
func TestListLiveFollowerAccountsPagesByBuyerIDForOneStore(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	first := profile(t, fixture, 21)
	closing := profile(t, fixture, 22)
	other := profile(t, fixture, 23)
	last := profile(t, fixture, 24)
	for _, b := range []Profile{first, closing, last} {
		must(t, fixture.module.Follow(ctx, b.BuyerID, 5))
	}
	must(t, fixture.module.Follow(ctx, other.BuyerID, 6))
	must(t, fixture.module.BeginDeletion(ctx, closing.BuyerID))

	want := []AccountRef{{BuyerID: first.BuyerID, AccountID: 21}, {BuyerID: closing.BuyerID, AccountID: 22}, {BuyerID: last.BuyerID, AccountID: 24}}
	var got []AccountRef
	after, pages := int64(0), 0
	for {
		page, err := fixture.module.ListLiveFollowerAccounts(ctx, 5, after, 2)
		must(t, err)
		pages++
		got = append(got, page...)
		if len(page) < 2 {
			break
		}
		after = page[len(page)-1].BuyerID
	}
	if len(got) != len(want) || pages != 2 {
		t.Fatalf("得到 %v（%d 頁），預期 %v（2 頁）", got, pages, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 筆 = %v，預期 %v", i, got[i], want[i])
		}
	}
	if page, err := fixture.module.ListLiveFollowerAccounts(ctx, 99, 0, 5); err != nil || len(page) != 0 {
		t.Errorf("沒有人追蹤的店 = %v（%v），預期空", page, err)
	}
	for name, c := range map[string]struct {
		store, after int64
		limit        int
	}{"limit 0": {5, 0, 0}, "limit 過大": {5, 0, MaxLiveAccountsPage + 1}, "after 為負": {5, -1, 1}, "店不合法": {0, 0, 1}} {
		if _, err := fixture.module.ListLiveFollowerAccounts(ctx, c.store, c.after, c.limit); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v，預期 InvalidInput", name, err)
		}
	}
}
