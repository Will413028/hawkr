package procurement_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type fakeSellers struct {
	calls     [][]int64
	exposable map[int64]bool
}

func (f *fakeSellers) ExposableStores(_ context.Context, ids []int64) ([]seller.ExposableStore, error) {
	f.calls = append(f.calls, append([]int64(nil), ids...))
	var out []seller.ExposableStore
	for _, id := range ids {
		if f.exposable[id] {
			out = append(out, seller.ExposableStore{StoreID: id})
		}
	}
	return out, nil
}

// 超過 seller.MaxBatchSize 的店要分批問，每批不超過上限，結果合併。
func TestOpenStoresBatchesBySellerLimit(t *testing.T) {
	fake := &fakeSellers{exposable: map[int64]bool{1: true, int64(seller.MaxBatchSize) + 1: true}}
	ids := make([]int64, 0, seller.MaxBatchSize+2)
	for i := range seller.MaxBatchSize + 2 {
		ids = append(ids, int64(i+1))
	}
	got, err := procurement.NewOpenStores(fake).OpenStoreIDs(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("分了 %d 批，要 2 批", len(fake.calls))
	}
	for _, c := range fake.calls {
		if len(c) > seller.MaxBatchSize {
			t.Fatalf("一批 %d 個，超過上限 %d", len(c), seller.MaxBatchSize)
		}
	}
	if _, ok := got[1]; !ok || len(got) != 2 {
		t.Fatalf("got = %v", got)
	}
}
