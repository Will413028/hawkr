package productservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	productpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/product/productconnect"
)

type fakeProducts struct {
	productconnect.ProductServiceClient
	called   bool
	response *productpb.GetProductsByIDsResponse
	err      error
}

func (f *fakeProducts) GetProductsByIDs(ctx context.Context, req *connect.Request[productpb.GetProductsByIDsRequest]) (*connect.Response[productpb.GetProductsByIDsResponse], error) {
	f.called = true
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("missing deadline")
	}
	if len(req.Msg.GetProductIds()) != 2 || req.Msg.GetProductIds()[0] != "7" || !req.Msg.GetIncludeInactive() {
		return nil, errors.New("bad request")
	}
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.response), nil
}

func TestCardsBatch(t *testing.T) {
	f := &fakeProducts{response: &productpb.GetProductsByIDsResponse{Products: []*productpb.ProductListItem{{Id: "7", StoreId: "3", Name: "蘋果", PriceMin: "10", PriceMax: "20", StockQty: "2", Status: "ACTIVE"}}}}
	c := &Client{rpc: f}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cards, err := c.Cards(ctx, []int64{7, 8}, true)
	if err != nil || !f.called || cards[7].Name != "蘋果" || !cards[7].Available || cards[8].ID != 0 {
		t.Fatalf("cards=%v err=%v called=%v", cards, err, f.called)
	}
	f.response.Code = 500
	if _, err := c.Cards(ctx, []int64{7, 8}, true); err == nil {
		t.Fatal("business error ignored")
	}
}

func TestProductRatingWithoutReviews(t *testing.T) {
	for _, raw := range []string{"", "0", "0.0"} {
		rating, err := productRating(raw)
		if err != nil || rating != nil {
			t.Fatalf("rating %q = %v, %v; want absent", raw, rating, err)
		}
	}
	rating, err := productRating("4.5")
	if err != nil || rating == nil || *rating != 4.5 {
		t.Fatalf("rating 4.5 = %v, %v", rating, err)
	}
	card, err := parseCard(&productpb.ProductListItem{Id: "7", StoreId: "3", PriceMin: "10", PriceMax: "20", StockQty: "0", Status: "ACTIVE", Rating: "0"})
	if err != nil || card.Available || card.Rating != nil {
		t.Fatalf("card=%+v err=%v", card, err)
	}
	if _, err := parseCard(&productpb.ProductListItem{Id: "7", StoreId: "bad", PriceMin: "10", PriceMax: "20", StockQty: "1"}); err == nil {
		t.Fatal("invalid store ID accepted")
	}
	if _, err := parseCard(&productpb.ProductListItem{Id: "7", StoreId: "3", PriceMin: "-1", PriceMax: "20", StockQty: "1"}); err == nil {
		t.Fatal("negative price accepted")
	}
}
