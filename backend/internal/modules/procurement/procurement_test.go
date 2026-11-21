package procurement_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
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

func testDeps() procurement.Deps {
	return procurement.Deps{
		DB: new(db.DB), Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestNewRejectsMissingRequiredDeps(t *testing.T) {
	cases := map[string]func(*procurement.Deps){
		"DB":     func(d *procurement.Deps) { d.DB = nil },
		"Now":    func(d *procurement.Deps) { d.Now = nil },
		"Logger": func(d *procurement.Deps) { d.Logger = nil },
	}
	for name, clear := range cases {
		d := testDeps()
		clear(&d)
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("缺 Deps.%s 要在 New panic", name)
				}
			}()
			procurement.New(d)
		}()
	}
	procurement.New(testDeps())
}
