package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

type fakeProducts struct {
	n   int
	err error
	got productservice.SearchQuery
}

func (f *fakeProducts) SearchProducts(_ context.Context, q productservice.SearchQuery) ([]productservice.SearchHit, error) {
	f.got = q
	if f.err != nil {
		return nil, f.err
	}
	out := make([]productservice.SearchHit, f.n)
	for i := range out {
		out[i] = productservice.SearchHit{ID: int64(i + 1), StoreID: int64(i + 1)}
	}
	return out, nil
}

func TestProductSearchTreatsOnlyAShortPageAsTheLastPage(t *testing.T) {
	for _, c := range []struct {
		n    int
		last bool
	}{{0, true}, {productservice.SearchPageSize - 1, true}, {productservice.SearchPageSize, false}} {
		f := &fakeProducts{n: c.n}
		page, err := productSearch{f}.Page(context.Background(), procurement.SearchQuery{CategoryID: 18}, 2)
		if err != nil || page.Last != c.last || len(page.Hits) != c.n {
			t.Errorf("%d 筆：Last=%v hits=%d err=%v，要 Last=%v", c.n, page.Last, len(page.Hits), err, c.last)
		}
		if f.got.CategoryID != 18 || f.got.Page != 2 {
			t.Errorf("查詢沒有原樣轉給 product-service：%+v", f.got)
		}
	}
}

func TestProductSearchReturnsTheClientError(t *testing.T) {
	boom := errors.New("down")
	if _, err := (productSearch{&fakeProducts{err: boom}}).Page(context.Background(), procurement.SearchQuery{Keyword: "x"}, 1); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
