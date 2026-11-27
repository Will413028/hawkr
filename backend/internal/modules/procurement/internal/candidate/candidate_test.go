package candidate

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"uuid"
)

type fakeOpen struct {
	open map[int64]struct{}
	err  error
	got  []int64
}

func (f *fakeOpen) OpenStoreIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	f.got = ids
	return f.open, f.err
}

func productID(n int64) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
}

func found(ids ...int64) []Hit {
	out := make([]Hit, 0, len(ids))
	for _, id := range ids {
		out = append(out, Hit{ProductID: productID(id * 100), StoreID: id})
	}
	return out
}

func ids(fs []Hit) []int64 {
	out := []int64{}
	for _, f := range fs {
		out = append(out, f.StoreID)
	}
	return out
}

// S9：候選只收 OPEN 的店，維持輸入順序；暫停或不存在的店不會成為候選。
func TestKeepOpenDropsStoresThatAreNotOpen(t *testing.T) {
	gate := &fakeOpen{open: map[int64]struct{}{30: {}, 10: {}}}
	got, err := KeepOpen(context.Background(), gate, found(10, 20, 30))
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{10, 30}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("候選 = %v，要 %v", ids(got), want)
	}
	if want := []int64{10, 20, 30}; !reflect.DeepEqual(gate.got, want) {
		t.Fatalf("查了 %v，要把全部都問過 %v", gate.got, want)
	}
}

// fail-closed：讀不到店家狀態時不回任何候選。
func TestKeepOpenFailsClosed(t *testing.T) {
	boom := errors.New("boom")
	got, err := KeepOpen(context.Background(), &fakeOpen{err: boom}, found(1))
	if !errors.Is(err, boom) || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestKeepOpenWithNothingFoundSkipsTheLookup(t *testing.T) {
	gate := &fakeOpen{err: errors.New("不該被呼叫")}
	got, err := KeepOpen(context.Background(), gate, nil)
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

// 同一家店重複出現只留第一筆，也只問一次。
func TestKeepOpenDeduplicatesStoresKeepingTheFirst(t *testing.T) {
	gate := &fakeOpen{open: map[int64]struct{}{10: {}, 20: {}}}
	in := found(10, 20, 10)
	in[2].ProductID = productID(7)
	got, err := KeepOpen(context.Background(), gate, in)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{10, 20}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("候選 = %v，要 %v", ids(got), want)
	}
	if got[0].ProductID == productID(7) {
		t.Fatal("重複時要留第一筆")
	}
	if want := []int64{10, 20}; !reflect.DeepEqual(gate.got, want) {
		t.Fatalf("查了 %v，要 %v", gate.got, want)
	}
}

const endCursor Cursor = "end"

type fakeSearcher struct {
	pages []Page
	err   map[int]error
	calls []Cursor
	query []Query
}

func (f *fakeSearcher) Page(_ context.Context, q Query, after Cursor) (Page, error) {
	f.calls = append(f.calls, after)
	f.query = append(f.query, q)
	i := 0
	if after != "" {
		if _, err := fmt.Sscanf(string(after), "p%d", &i); err != nil {
			return Page{}, fmt.Errorf("游標 %q 不是 fake 產生的", after)
		}
	}
	if err := f.err[i]; err != nil {
		return Page{}, err
	}
	if i >= len(f.pages) {
		return Page{}, nil
	}
	pg := f.pages[i]
	if pg.Next == endCursor {
		pg.Next = ""
	} else {
		pg.Next = Cursor(fmt.Sprintf("p%d", i+1))
	}
	return pg, nil
}

type fakeBlocks struct {
	blocked map[int64]struct{}
	err     error
	batches [][]int64
}

func (f *fakeBlocks) StoreIDsBlockingBuyer(_ context.Context, _ int64, ids []int64) (map[int64]struct{}, error) {
	f.batches = append(f.batches, ids)
	return f.blocked, f.err
}

type allOpen struct{}

func (allOpen) OpenStoreIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	out := map[int64]struct{}{}
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

func seq(from, to int64) []Hit {
	var out []Hit
	for i := from; i <= to; i++ {
		out = append(out, Hit{ProductID: productID(i * 100), StoreID: i})
	}
	return out
}

func search(t *testing.T, s Searcher, open OpenStores, b BlockedStores) ([]Hit, error) {
	t.Helper()
	return Search(context.Background(), s, open, b, 5, Query{CategoryID: productID(9)})
}

func TestSearchStopsOnceTwentyStoresAreKept(t *testing.T) {
	s := &fakeSearcher{pages: []Page{{Hits: seq(1, 15)}, {Hits: seq(16, 30)}, {Hits: seq(31, 45)}}}
	got, err := search(t, s, allOpen{}, &fakeBlocks{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxStores || got[0].StoreID != 1 || got[MaxStores-1].StoreID != 20 {
		t.Fatalf("stores = %v", ids(got))
	}
	if !reflect.DeepEqual(s.calls, []Cursor{"", "p1"}) {
		t.Fatalf("pages = %v，滿 20 家就不再翻頁", s.calls)
	}
}

func TestSearchStopsAtAShortLastPage(t *testing.T) {
	s := &fakeSearcher{pages: []Page{{Hits: seq(1, 3), Next: endCursor}, {Hits: seq(4, 6)}}}
	got, err := search(t, s, allOpen{}, &fakeBlocks{})
	if err != nil || len(got) != 3 || !reflect.DeepEqual(s.calls, []Cursor{""}) {
		t.Fatalf("got=%v err=%v pages=%v", ids(got), err, s.calls)
	}
}

func TestSearchStopsAfterMaxPages(t *testing.T) {
	var pages []Page
	for p := range int64(8) {
		pages = append(pages, Page{Hits: seq(p*2+1, p*2+2)})
	}
	s := &fakeSearcher{pages: pages}
	got, err := search(t, s, allOpen{}, &fakeBlocks{})
	if err != nil || len(got) != 2*MaxPages || !reflect.DeepEqual(s.calls, []Cursor{"", "p1", "p2", "p3", "p4"}) {
		t.Fatalf("got=%d err=%v pages=%v", len(got), err, s.calls)
	}
}

func TestSearchKeepsTheFirstProductOfEachStore(t *testing.T) {
	s := &fakeSearcher{pages: []Page{
		{Hits: []Hit{{ProductID: productID(1), StoreID: 10}, {ProductID: productID(2), StoreID: 10}, {ProductID: productID(3), StoreID: 20}}},
		{Hits: []Hit{{ProductID: productID(4), StoreID: 20}, {ProductID: productID(5), StoreID: 30}}, Next: endCursor},
	}}
	got, err := search(t, s, allOpen{}, &fakeBlocks{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Hit{{ProductID: productID(1), StoreID: 10}, {ProductID: productID(3), StoreID: 20}, {ProductID: productID(5), StoreID: 30}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %v，要 %v", got, want)
	}
}

func TestSearchDropsClosedAndBlockingStoresBeforeCounting(t *testing.T) {
	s := &fakeSearcher{pages: []Page{{Hits: seq(1, 6), Next: endCursor}}}
	open := &fakeOpen{open: map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}, 5: {}}}
	blocks := &fakeBlocks{blocked: map[int64]struct{}{2: {}}}
	got, err := search(t, s, open, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{1, 3, 4, 5}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("stores = %v，要 %v", ids(got), want)
	}
	if want := []int64{1, 2, 3, 4, 5}; len(blocks.batches) != 1 || !reflect.DeepEqual(blocks.batches[0], want) {
		t.Fatalf("封鎖只查通過 KeepOpen 的店：%v", blocks.batches)
	}
}

func TestSearchCountsOnlyFilteredStoresTowardTheCap(t *testing.T) {
	blocked := map[int64]struct{}{}
	for i := int64(1); i <= 10; i++ {
		blocked[i] = struct{}{}
	}
	s := &fakeSearcher{pages: []Page{{Hits: seq(1, 15)}, {Hits: seq(16, 40)}}}
	got, err := search(t, s, allOpen{}, &fakeBlocks{blocked: blocked})
	if err != nil || len(got) != MaxStores || got[0].StoreID != 11 || !reflect.DeepEqual(s.calls, []Cursor{"", "p1"}) {
		t.Fatalf("got=%v err=%v pages=%v", ids(got), err, s.calls)
	}
}

func TestSearchFailsWhenAFilterOrAPageFails(t *testing.T) {
	boom := errors.New("boom")
	pages := []Page{{Hits: seq(1, 2)}, {Hits: seq(3, 4), Next: endCursor}}
	for name, c := range map[string]struct {
		s *fakeSearcher
		o OpenStores
		b BlockedStores
	}{
		"blocks":     {&fakeSearcher{pages: pages}, allOpen{}, &fakeBlocks{err: boom}},
		"open":       {&fakeSearcher{pages: pages}, &fakeOpen{err: boom}, &fakeBlocks{}},
		"first page": {&fakeSearcher{pages: pages, err: map[int]error{0: boom}}, allOpen{}, &fakeBlocks{}},
		"later page": {&fakeSearcher{pages: pages, err: map[int]error{1: boom}}, allOpen{}, &fakeBlocks{}},
	} {
		got, err := search(t, c.s, c.o, c.b)
		if !errors.Is(err, boom) || got != nil {
			t.Errorf("%s：got=%v err=%v，任一失敗整次搜尋失敗", name, got, err)
		}
	}
}

func TestSearchPassesTheQueryThrough(t *testing.T) {
	s := &fakeSearcher{pages: []Page{{Next: endCursor}}}
	if _, err := Search(context.Background(), s, allOpen{}, &fakeBlocks{}, 5, Query{Keyword: "高麗菜"}); err != nil {
		t.Fatal(err)
	}
	if len(s.query) != 1 || s.query[0] != (Query{Keyword: "高麗菜"}) {
		t.Fatalf("query = %v", s.query)
	}
}
