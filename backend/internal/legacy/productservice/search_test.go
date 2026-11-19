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
	got, err := (&Client{rpc: f}).SearchProducts(context.Background(), " 高麗菜 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (SearchHit{ID: 7, StoreID: 3, Name: "高麗菜", ImageKey: "k/1", MinPrice: 10, MaxPrice: 20}) {
		t.Fatalf("hits = %+v", got)
	}
	r := f.seen
	if r.GetKeyword() != "高麗菜" || r.GetPage() != "1" || r.GetPageSize() != "50" || r.GetStatus() != "4004" || r.GetPriceMaxFilter() != "" {
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
		if _, err := (&Client{rpc: f}).SearchProducts(context.Background(), "x"); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := (&Client{rpc: &fakeSearch{}}).SearchProducts(context.Background(), "  "); err == nil {
		t.Error("blank keyword accepted")
	}
}
