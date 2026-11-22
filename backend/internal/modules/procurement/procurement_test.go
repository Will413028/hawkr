package procurement_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
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

func registered[T river.JobArgs](w *river.Workers) bool {
	return river.AddWorkerSafely(w, river.WorkFunc(func(context.Context, *river.Job[T]) error { return nil })) != nil
}

// 漏登記執行 run 的 worker，送出的 run 永遠停在 QUEUED；漏了 reconciler 或清除，死掉的 run 與過期事件沒有人收。
func TestAddWorkersRegistersTheRunReconcilerAndPurgeWorkers(t *testing.T) {
	m := procurement.New(procurement.Deps{DB: &db.DB{}, Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	w := river.NewWorkers()
	m.AddWorkers(w)
	if !registered[run.RunArgs](w) {
		t.Error("沒有登記執行 run 的 worker（procurement.run）")
	}
	if !registered[run.ReconcileArgs](w) {
		t.Error("沒有登記 reconciler")
	}
	if !registered[run.PurgeArgs](w) {
		t.Error("沒有登記事件清除")
	}
}

type noTaxonomy struct{}

func (noTaxonomy) CategoryTree(context.Context) ([]procurement.TaxonomyNode, error) { return nil, nil }

func newDeps(extra func(*procurement.Deps)) procurement.Deps {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := procurement.Deps{DB: new(db.DB), Now: time.Now, Logger: log}
	extra(&d)
	return d
}

// TURN 的 executor 由 New 在給了 LLM 與分類樹時登記；少了它，worker 領到 TURN 會把 run 標失敗。
func TestNewRegistersTheTurnExecutorOnlyWithLLMAndTaxonomy(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	disabled := llm.New(llm.Procurement, llm.Config{}, log)
	with := procurement.New(newDeps(func(d *procurement.Deps) { d.LLM, d.Taxonomy = disabled, noTaxonomy{} }))
	if got := with.ExecutorTypes(); len(got) != 1 || got[0] != run.RunTypeTurn {
		t.Errorf("executors = %v，要登記 TURN", got)
	}
	without := procurement.New(newDeps(func(*procurement.Deps) {}))
	if got := without.ExecutorTypes(); len(got) != 0 {
		t.Errorf("沒有 LLM 與分類樹不該登記：%v", got)
	}
	custom := procurement.New(newDeps(func(d *procurement.Deps) {
		d.LLM, d.Taxonomy = disabled, noTaxonomy{}
		d.Executors = map[string]procurement.Executor{run.RunTypeTurn: nil}
	}))
	if got := custom.ExecutorTypes(); len(got) != 1 {
		t.Errorf("呼叫端給的 TURN 項目要保留：%v", got)
	}
}

func TestNewPanicsWhenOnlyOneOfLLMAndTaxonomyIsGiven(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for name, fn := range map[string]func(*procurement.Deps){
		"只有 LLM":      func(d *procurement.Deps) { d.LLM = llm.New(llm.Procurement, llm.Config{}, log) },
		"只有 Taxonomy": func(d *procurement.Deps) { d.Taxonomy = noTaxonomy{} },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s：要 panic", name)
				}
			}()
			procurement.New(newDeps(fn))
		}()
	}
}
