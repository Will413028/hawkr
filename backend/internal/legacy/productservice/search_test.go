package productservice

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	productpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/product/productconnect"
)

type fakeSearch struct {
	productconnect.ProductServiceClient
	resp *productpb.GetProductListResponse
	err  error
	seen *productpb.GetProductListRequest
}

func (f *fakeSearch) GetProductList(_ context.Context, req *connect.Request[productpb.GetProductListRequest]) (*connect.Response[productpb.GetProductListResponse], error) {
	f.seen = req.Msg
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func TestSearchProducts(t *testing.T) {
	f := &fakeSearch{resp: &productpb.GetProductListResponse{Products: []*productpb.ProductListItem{
		{Id: "7", StoreId: "3", Name: "高麗菜", ImageKey: "k/1", PriceMin: "10", PriceMax: "20"},
	}}}
	got, err := (&Client{rpc: f}).SearchProducts(context.Background(), SearchQuery{Keyword: " 高麗菜 ", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (SearchHit{ID: 7, StoreID: 3, Name: "高麗菜", ImageKey: "k/1", MinPrice: 10, MaxPrice: 20}) {
		t.Fatalf("hits = %+v", got)
	}
	r := f.seen
	if r.GetKeyword() != "高麗菜" || r.GetPage() != "1" || r.GetPageSize() != "50" || r.GetStatus() != "4004" || r.GetSort() != "SALES" || len(r.GetCategoryId()) != 0 || r.GetPriceMaxFilter() != "" {
		t.Fatalf("request = %v", r)
	}
}

func TestSearchProductsRejectsBadData(t *testing.T) {
	item := func(mutate func(*productpb.ProductListItem)) *productpb.GetProductListResponse {
		p := &productpb.ProductListItem{Id: "7", StoreId: "3", PriceMin: "1", PriceMax: "2"}
		mutate(p)
		return &productpb.GetProductListResponse{Products: []*productpb.ProductListItem{p}}
	}
	for name, f := range map[string]*fakeSearch{
		"transport": {err: errors.New("offline")},
		"code":      {resp: &productpb.GetProductListResponse{Code: 500}},
		"bad id":    {resp: item(func(p *productpb.ProductListItem) { p.Id = "x" })},
		"bad store": {resp: item(func(p *productpb.ProductListItem) { p.StoreId = "0" })},
		"bad price": {resp: item(func(p *productpb.ProductListItem) { p.PriceMin = "" })},
	} {
		if _, err := (&Client{rpc: f}).SearchProducts(context.Background(), SearchQuery{Keyword: "x", Page: 1}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := (&Client{rpc: &fakeSearch{}}).SearchProducts(context.Background(), SearchQuery{Keyword: "  ", Page: 1}); err == nil {
		t.Error("blank keyword accepted")
	}
}

func TestSearchProductsByCategoryCarriesNoKeywordAndTheRequestedPage(t *testing.T) {
	f := &fakeSearch{resp: &productpb.GetProductListResponse{}}
	if _, err := (&Client{rpc: f}).SearchProducts(context.Background(), SearchQuery{CategoryID: 18, Page: 3}); err != nil {
		t.Fatal(err)
	}
	r := f.seen
	if got := r.GetCategoryId(); len(got) != 1 || got[0] != "18" || r.GetKeyword() != "" || r.GetPage() != "3" || r.GetSort() != "SALES" || r.GetStatus() != "4004" || r.GetPageSize() != "50" {
		t.Fatalf("request = %v", r)
	}
}

func TestSearchProductsRejectsAnInvalidQuery(t *testing.T) {
	for name, q := range map[string]SearchQuery{
		"neither":      {Page: 1},
		"both":         {CategoryID: 1, Keyword: "x", Page: 1},
		"zero page":    {Keyword: "x"},
		"negative cat": {CategoryID: -1, Page: 1},
	} {
		f := &fakeSearch{resp: &productpb.GetProductListResponse{}}
		if _, err := (&Client{rpc: f}).SearchProducts(context.Background(), q); err == nil {
			t.Errorf("%s accepted", name)
		}
		if f.seen != nil {
			t.Errorf("%s reached the service", name)
		}
	}
}
