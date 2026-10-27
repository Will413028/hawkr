//go:build integration

package seller_test

import (
	"context"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 全量重新公告：每一列公告列（含從來沒公告過、與已關閉的店）都更新並發事件，不比對最後公告的值，版本照常加一；算出來的可曝光與
// 一般的公告是同一個計算（這裡可曝光的店仍是 true，不可曝光的仍是 false）。
func TestReannounceExposuresAnnouncesEveryRowWithoutComparing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, exposed := f.exposable(t)
	_, hidden := f.open(t)
	accountID, closed := f.exposable(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if _, version := f.announced(t, hidden.StoreID); version != 0 {
		t.Fatalf("沒有公告過的店版本是 %d，want 0", version)
	}
	_, exposedBefore := f.announced(t, exposed.StoreID)
	_, closedBefore := f.announced(t, closed.StoreID)
	announcements := record[StoreExposureChanged](f)

	n, err := f.module.ReannounceExposures(ctx)
	must(t, err)
	if n != 3 {
		t.Fatalf("公告了 %d 家，want 3（兩家營運中、一家已關閉，不論有沒有變化）", n)
	}
	got := map[int64]StoreExposureChanged{}
	for _, e := range announcements.all() {
		if _, dup := got[e.StoreID]; dup {
			t.Fatalf("同一家店公告了兩次：%+v", announcements.all())
		}
		got[e.StoreID] = e
	}
	for _, c := range []struct {
		id        int64
		exposable bool
		version   int64
	}{
		{exposed.StoreID, true, exposedBefore + 1},
		{hidden.StoreID, false, 1},
		{closed.StoreID, false, closedBefore + 1},
	} {
		e, ok := got[c.id]
		if !ok || e.Exposable != c.exposable || e.Version != c.version || e.EventID[6]>>4 != 7 {
			t.Errorf("店 %d 的事件 = %+v（有 %v），want exposable=%v version=%d 與 UUIDv7 的事件 ID", c.id, e, ok, c.exposable, c.version)
		}
		if exposable, version := f.announced(t, c.id); exposable != c.exposable || version != c.version {
			t.Errorf("店 %d 的公告列 = (%v, %d)，want (%v, %d)", c.id, exposable, version, c.exposable, c.version)
		}
	}

	n, err = f.module.ReannounceExposures(ctx)
	must(t, err)
	if n != 3 || len(announcements.all()) != 6 {
		t.Fatalf("第二次：公告 %d 家、累計 %d 個事件，want 3 與 6（版本照常再加一）", n, len(announcements.all()))
	}
	if _, version := f.announced(t, hidden.StoreID); version != 2 {
		t.Fatalf("第二次之後沒有公告過的店版本是 %d，want 2", version)
	}
}
