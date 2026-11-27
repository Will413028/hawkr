package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

var searchNow = time.Date(2026, 11, 27, 3, 0, 0, 0, time.UTC)

func searchUUID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
}

// fakeCatalog 是依 (排序鍵, ID) 遞減排列的商品清單，游標語意與 catalog 的 keyset 相同：回傳游標之後的 size 筆，還有更多時帶 Next。
type fakeCatalog struct {
	items []catalog.ProductSummary
	err   error

	gotAt     time.Time
	gotFilter catalog.PublicFilter
	gotSort   catalog.ProductSort
	gotSize   int32
	gotAfter  []*catalog.ListCursor
}

func newFakeCatalog(n int) *fakeCatalog {
	f := &fakeCatalog{}
	for i := range n {
		f.items = append(f.items, catalog.ProductSummary{ID: searchUUID(n - i), StoreID: int64(n - i)})
	}
	return f
}

func (f *fakeCatalog) ListPublicProducts(_ context.Context, at time.Time, filter catalog.PublicFilter, sort catalog.ProductSort, after *catalog.ListCursor, size int32) (catalog.ProductPage, error) {
	f.gotAt, f.gotFilter, f.gotSort, f.gotSize = at, filter, sort, size
	f.gotAfter = append(f.gotAfter, after)
	if f.err != nil {
		return catalog.ProductPage{}, f.err
	}
	start := 0
	if after != nil {
		start = len(f.items)
		for i, it := range f.items {
			if it.ID == after.ID {
				start = i + 1
				break
			}
		}
	}
	end := min(start+int(size), len(f.items))
	page := catalog.ProductPage{Items: f.items[start:end]}
	if end < len(f.items) {
		last := f.items[end-1]
		page.Next = &catalog.ListCursor{Key: int64(end), ID: last.ID}
	}
	return page, nil
}

func searcherOf(c publicProducts) productSearch {
	return productSearch{catalog: c, now: func() time.Time { return searchNow }}
}

// 翻頁用上一頁的游標：每一頁都不重複、不遺漏，最後一頁沒有 Next。
func TestProductSearchKeysetPagesNeitherRepeatNorSkip(t *testing.T) {
	const total = searchPageSize*2 + 20
	f := newFakeCatalog(total)
	s := searcherOf(f)
	q := procurement.SearchQuery{CategoryID: searchUUID(900)}

	seen := map[uuid.UUID]bool{}
	var after procurement.SearchCursor
	pages := 0
	for {
		pg, err := s.Page(context.Background(), q, after)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, h := range pg.Hits {
			if seen[h.ProductID] {
				t.Fatalf("第 %d 頁重複了商品 %s", pages, h.ProductID)
			}
			seen[h.ProductID] = true
		}
		if pg.Next == "" {
			break
		}
		if pages > 10 {
			t.Fatal("翻不完")
		}
		after = pg.Next
	}
	if pages != 3 || len(seen) != total {
		t.Fatalf("pages=%d 商品 %d 件，要 3 頁 %d 件", pages, len(seen), total)
	}
	if f.gotAfter[0] != nil || f.gotAfter[1] == nil || f.gotAfter[2] == nil || f.gotAfter[1].ID == f.gotAfter[2].ID {
		t.Fatalf("每一頁要帶上一頁的游標：%+v", f.gotAfter)
	}
}

func TestProductSearchOnlyAFullPageHasANextCursor(t *testing.T) {
	for _, c := range []struct {
		n    int
		last bool
	}{{0, true}, {searchPageSize - 1, true}, {searchPageSize, true}, {searchPageSize + 1, false}} {
		pg, err := searcherOf(newFakeCatalog(c.n)).Page(context.Background(), procurement.SearchQuery{Keyword: "蘋果"}, "")
		if err != nil || (pg.Next == "") != c.last {
			t.Errorf("%d 筆：Next=%q err=%v，要 last=%v", c.n, pg.Next, err, c.last)
		}
	}
}

// 分類查詢只帶分類篩選、關鍵字查詢只帶關鍵字；一律依銷量、用模組的時鐘、每頁 searchPageSize 筆。
func TestProductSearchMapsTheQueryOntoCatalogsFilter(t *testing.T) {
	cat := searchUUID(18)
	f := newFakeCatalog(3)
	pg, err := searcherOf(f).Page(context.Background(), procurement.SearchQuery{CategoryID: cat}, "")
	if err != nil || len(pg.Hits) != 3 {
		t.Fatalf("hits=%v err=%v", pg.Hits, err)
	}
	if len(f.gotFilter.CategoryIDs) != 1 || f.gotFilter.CategoryIDs[0] != cat || f.gotFilter.Query != "" ||
		f.gotSort != catalog.SortSales || !f.gotAt.Equal(searchNow) || f.gotSize != searchPageSize {
		t.Fatalf("category：filter=%+v sort=%s at=%v size=%d", f.gotFilter, f.gotSort, f.gotAt, f.gotSize)
	}
	if pg.Hits[0].ProductID != f.items[0].ID || pg.Hits[0].StoreID != f.items[0].StoreID {
		t.Fatalf("命中沒有原樣帶商品與店家：%+v", pg.Hits[0])
	}

	f = newFakeCatalog(1)
	if _, err := searcherOf(f).Page(context.Background(), procurement.SearchQuery{Keyword: "  高麗菜 "}, ""); err != nil {
		t.Fatal(err)
	}
	if len(f.gotFilter.CategoryIDs) != 0 || f.gotFilter.Query != "高麗菜" {
		t.Fatalf("keyword：filter=%+v", f.gotFilter)
	}
}

func TestProductSearchRejectsAnAmbiguousQueryAndABrokenCursor(t *testing.T) {
	f := newFakeCatalog(1)
	s := searcherOf(f)
	for name, c := range map[string]struct {
		q      procurement.SearchQuery
		cursor procurement.SearchCursor
	}{
		"neither":         {procurement.SearchQuery{}, ""},
		"blank keyword":   {procurement.SearchQuery{Keyword: "  "}, ""},
		"both":            {procurement.SearchQuery{CategoryID: searchUUID(1), Keyword: "x"}, ""},
		"no separator":    {procurement.SearchQuery{Keyword: "x"}, "garbage"},
		"key not integer": {procurement.SearchQuery{Keyword: "x"}, procurement.SearchCursor("k:" + searchUUID(1).String())},
		"id not uuid":     {procurement.SearchQuery{Keyword: "x"}, "5:nope"},
		"zero id":         {procurement.SearchQuery{Keyword: "x"}, procurement.SearchCursor("5:" + uuid.Nil().String())},
	} {
		if _, err := s.Page(context.Background(), c.q, c.cursor); err == nil {
			t.Errorf("%s：該被拒絕", name)
		}
	}
	if len(f.gotAfter) != 0 {
		t.Fatalf("被拒絕的查詢不該打到 catalog：%d 次", len(f.gotAfter))
	}
}

func TestProductSearchReturnsTheCatalogError(t *testing.T) {
	boom := errors.New("down")
	f := newFakeCatalog(1)
	f.err = boom
	if _, err := searcherOf(f).Page(context.Background(), procurement.SearchQuery{Keyword: "x"}, ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

type fakeCategories struct {
	cats []catalog.Category
	err  error
}

func (f fakeCategories) Categories(context.Context) ([]catalog.Category, error) { return f.cats, f.err }

func TestTaxonomyMapsCatalogCategoriesToNodes(t *testing.T) {
	root, leaf := searchUUID(1), searchUUID(2)
	nodes, err := taxonomy{fakeCategories{cats: []catalog.Category{
		{ID: root, NameZhTW: "水果", NameEn: "Fruit"},
		{ID: leaf, ParentID: &root, NameZhTW: "蘋果", NameEn: "Apple"},
	}}}.CategoryTree(context.Background())
	if err != nil || len(nodes) != 2 {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
	if nodes[0].ID != root || nodes[0].ParentID != nil || nodes[0].Name != "水果" {
		t.Errorf("根：%+v", nodes[0])
	}
	if nodes[1].ID != leaf || nodes[1].ParentID == nil || *nodes[1].ParentID != root || nodes[1].Name != "蘋果" {
		t.Errorf("葉：%+v", nodes[1])
	}
	boom := errors.New("down")
	if _, err := (taxonomy{fakeCategories{err: boom}}).CategoryTree(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
