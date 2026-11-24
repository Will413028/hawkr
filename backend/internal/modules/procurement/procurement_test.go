package procurement_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
)

type fakeSellers struct {
	calls        [][]int64
	exposable    map[int64]bool
	blocking     map[int64]bool
	blockCalls   [][]int64
	blockErrorOn int
}

func (f *fakeSellers) StoresBlockingBuyer(_ context.Context, _ int64, ids []int64) (map[int64]bool, error) {
	f.blockCalls = append(f.blockCalls, append([]int64(nil), ids...))
	if f.blockErrorOn > 0 && len(f.blockCalls) == f.blockErrorOn {
		return nil, errors.New("boom")
	}
	out := map[int64]bool{}
	for _, id := range ids {
		if f.blocking[id] {
			out[id] = true
		}
	}
	return out, nil
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

// 封鎖查詢同樣依 seller 的批次上限分批，結果合併；任一批失敗整個失敗（fail-closed）。
func TestBlockedStoresBatchesBySellerLimitAndFailsClosed(t *testing.T) {
	last := int64(seller.MaxBatchSize) + 1
	fake := &fakeSellers{blocking: map[int64]bool{2: true, last: true}}
	ids := make([]int64, 0, seller.MaxBatchSize+2)
	for i := range seller.MaxBatchSize + 2 {
		ids = append(ids, int64(i+1))
	}
	got, err := procurement.NewBlockedStores(fake).StoreIDsBlockingBuyer(context.Background(), 5, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.blockCalls) != 2 || len(fake.blockCalls[0]) > seller.MaxBatchSize {
		t.Fatalf("分批 = %d 批", len(fake.blockCalls))
	}
	if _, ok := got[2]; !ok || len(got) != 2 {
		t.Fatalf("got = %v", got)
	}
	failing := &fakeSellers{blockErrorOn: 2}
	if got, err := procurement.NewBlockedStores(failing).StoreIDsBlockingBuyer(context.Background(), 5, ids); err == nil || got != nil {
		t.Fatalf("第二批失敗要整個失敗：%v %v", got, err)
	}
}

type fakeSearch struct{}

func (fakeSearch) Page(context.Context, procurement.SearchQuery, int) (procurement.SearchPage, error) {
	return procurement.SearchPage{Last: true}, nil
}

func TestNewWiresTheSearchExecutorWithoutAnLLMAndRejectsEachMissingSearchDependency(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	disabled := llm.New(llm.Procurement, llm.Config{}, log)
	searchOnly := procurement.New(newDeps(func(d *procurement.Deps) {
		d.Taxonomy, d.Sellers, d.Search = noTaxonomy{}, &fakeSellers{}, fakeSearch{}
	}))
	if got := searchOnly.ExecutorTypes(); !slices.Equal(got, []string{run.RunTypeCandidateSearch, run.RunTypeResume}) {
		t.Errorf("只給搜尋相依：executors = %v，要登記 CANDIDATE_SEARCH 與 CLARIFICATION_RESUME", got)
	}
	turnOnly := procurement.New(newDeps(func(d *procurement.Deps) { d.LLM, d.Taxonomy = disabled, noTaxonomy{} }))
	if got := turnOnly.ExecutorTypes(); !slices.Equal(got, []string{run.RunTypeResume, run.RunTypeTurn}) {
		t.Errorf("只給 TURN 相依：executors = %v，要登記 CLARIFICATION_RESUME 與 TURN", got)
	}
	for name, c := range map[string]struct {
		fn   func(*procurement.Deps)
		want string
	}{
		"搜尋缺 Sellers":    {func(d *procurement.Deps) { d.Taxonomy, d.Search = noTaxonomy{}, fakeSearch{} }, "Deps.Sellers"},
		"搜尋缺 Taxonomy":   {func(d *procurement.Deps) { d.Sellers, d.Search = &fakeSellers{}, fakeSearch{} }, "Deps.Taxonomy"},
		"LLM 缺 Taxonomy": {func(d *procurement.Deps) { d.LLM = disabled }, "Deps.Taxonomy"},
	} {
		func() {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, c.want) {
					t.Errorf("%s：panic 訊息 %q 要指名 %s", name, msg, c.want)
				}
			}()
			procurement.New(newDeps(c.fn))
		}()
	}
}

// CANDIDATE_SEARCH 的 executor 由 New 在給了搜尋、seller 與分類樹時登記；給了搜尋卻缺其他依賴是組裝錯誤。
func TestNewRegistersTheSearchExecutorAndRejectsPartialWiring(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	disabled := llm.New(llm.Procurement, llm.Config{}, log)
	full := procurement.New(newDeps(func(d *procurement.Deps) {
		d.LLM, d.Taxonomy, d.Sellers, d.Search = disabled, noTaxonomy{}, &fakeSellers{}, fakeSearch{}
	}))
	if got := full.ExecutorTypes(); !slices.Equal(got, []string{run.RunTypeCandidateSearch, run.RunTypeResume, run.RunTypeTurn}) {
		t.Errorf("executors = %v，要登記 CANDIDATE_SEARCH、CLARIFICATION_RESUME 與 TURN", got)
	}
	for name, fn := range map[string]func(*procurement.Deps){
		"沒有 Sellers":  func(d *procurement.Deps) { d.LLM, d.Taxonomy, d.Search = disabled, noTaxonomy{}, fakeSearch{} },
		"沒有 Taxonomy": func(d *procurement.Deps) { d.Sellers, d.Search = &fakeSellers{}, fakeSearch{} },
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

func testDeps() procurement.Deps {
	return procurement.Deps{
		DB: new(db.DB), Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Events: events.NewBus(),
	}
}

func TestNewRejectsMissingRequiredDeps(t *testing.T) {
	cases := map[string]func(*procurement.Deps){
		"DB":     func(d *procurement.Deps) { d.DB = nil },
		"Now":    func(d *procurement.Deps) { d.Now = nil },
		"Logger": func(d *procurement.Deps) { d.Logger = nil },
		"Events": func(d *procurement.Deps) { d.Events = nil },
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
	m := procurement.New(procurement.Deps{DB: &db.DB{}, Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Events: events.NewBus()})
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
	d := procurement.Deps{DB: new(db.DB), Now: time.Now, Logger: log, Events: events.NewBus()}
	extra(&d)
	return d
}

// TURN 的 executor 由 New 在給了 LLM 與分類樹時登記；少了它，worker 領到 TURN 會把 run 標失敗。
func TestNewRegistersTheTurnExecutorOnlyWithLLMAndTaxonomy(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	disabled := llm.New(llm.Procurement, llm.Config{}, log)
	with := procurement.New(newDeps(func(d *procurement.Deps) { d.LLM, d.Taxonomy = disabled, noTaxonomy{} }))
	if got := with.ExecutorTypes(); !slices.Equal(got, []string{run.RunTypeResume, run.RunTypeTurn}) {
		t.Errorf("executors = %v，要登記 CLARIFICATION_RESUME 與 TURN", got)
	}
	without := procurement.New(newDeps(func(*procurement.Deps) {}))
	if got := without.ExecutorTypes(); len(got) != 0 {
		t.Errorf("沒有 LLM 與分類樹不該登記：%v", got)
	}
	custom := procurement.New(newDeps(func(d *procurement.Deps) {
		d.LLM, d.Taxonomy = disabled, noTaxonomy{}
		d.Executors = map[string]procurement.Executor{run.RunTypeTurn: nil}
	}))
	if got := custom.ExecutorTypes(); !slices.Equal(got, []string{run.RunTypeResume, run.RunTypeTurn}) {
		t.Errorf("呼叫端給的 TURN 項目要保留：%v", got)
	}
}

// 回答中斷不呼叫模型：只給分類樹就登記 CLARIFICATION_RESUME 的 executor，呼叫端給的項目保留；少了它，worker 領到回答 run 會把它標失敗。
func TestNewRegistersTheResumeExecutorWithTaxonomyAlone(t *testing.T) {
	only := procurement.New(newDeps(func(d *procurement.Deps) { d.Taxonomy = noTaxonomy{} }))
	if got := only.ExecutorTypes(); !slices.Equal(got, []string{run.RunTypeResume}) {
		t.Errorf("只給 Taxonomy：executors = %v，要只登記 CLARIFICATION_RESUME", got)
	}
	custom := procurement.New(newDeps(func(d *procurement.Deps) {
		d.Taxonomy = noTaxonomy{}
		d.Executors = map[string]procurement.Executor{run.RunTypeResume: nil}
	}))
	if got := custom.ExecutorTypes(); !slices.Equal(got, []string{run.RunTypeResume}) {
		t.Errorf("呼叫端給的 CLARIFICATION_RESUME 項目要保留：%v", got)
	}
}

func TestNewPanicsWhenLLMIsGivenWithoutTaxonomy(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for name, fn := range map[string]func(*procurement.Deps){
		"只有 LLM": func(d *procurement.Deps) { d.LLM = llm.New(llm.Procurement, llm.Config{}, log) },
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

// 裝著 nil 指標的 Sellers 不等於 nil 介面：要當成沒給（發布回組裝錯誤），不能接上後在第一次呼叫時 nil dereference。
func TestNewTreatsATypedNilSellersAsMissing(t *testing.T) {
	var typedNil *fakeSellers
	if procurement.New(newDeps(func(d *procurement.Deps) { d.Sellers = typedNil })).RfqStorePortsWired() {
		t.Error("typed-nil Sellers 不得接上發布的店家檢查")
	}
	if !procurement.New(newDeps(func(d *procurement.Deps) { d.Sellers = &fakeSellers{} })).RfqStorePortsWired() {
		t.Error("給了 Sellers 要接上發布的店家檢查")
	}
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "Deps.Sellers") {
			t.Errorf("搜尋搭 typed-nil Sellers：panic 訊息 %q 要指名 Deps.Sellers", msg)
		}
	}()
	procurement.New(newDeps(func(d *procurement.Deps) { d.Taxonomy, d.Sellers, d.Search = noTaxonomy{}, typedNil, fakeSearch{} }))
}
