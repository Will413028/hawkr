package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type fakeStoreReader struct {
	store seller.Store
	err   error
}

func (f fakeStoreReader) Store(context.Context, int64) (seller.Store, error) { return f.store, f.err }

// 停權的店家回 true、沒有生效中停權的回 false；讀不到店家原樣回錯誤，不當成沒有停權（BD28）。
func TestCouponStoresReportsSuspensionAndFailsClosed(t *testing.T) {
	open := seller.Store{Sanctions: seller.NoSanctions()}
	suspended := seller.Store{Sanctions: seller.SanctionedBy(seller.EffectiveMeasure{})}
	for name, c := range map[string]struct {
		reader  fakeStoreReader
		want    bool
		wantErr bool
	}{
		"not suspended": {reader: fakeStoreReader{store: open}},
		"suspended":     {reader: fakeStoreReader{store: suspended}, want: true},
		"not derived":   {reader: fakeStoreReader{store: seller.Store{}}, want: true},
		"read failure":  {reader: fakeStoreReader{err: errors.New("seller down")}, wantErr: true},
	} {
		got, err := couponStores{c.reader}.StoreSuspended(context.Background(), 1)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("%s：suspended=%v err=%v，預期 %v、錯誤=%v", name, got, err, c.want, c.wantErr)
		}
	}
}

// MEMBER 資格的投影還沒有接上：port 回錯誤，不降級成「不是會員」。
func TestCouponMembersFailsUntilTheProjectionIsWired(t *testing.T) {
	if ok, err := (couponMembers{}).IsMember(context.Background(), 1, 2); ok || !errors.Is(err, errMembersNotProjected) {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
