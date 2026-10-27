package productservice

import (
	"context"
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	productpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/product/productconnect"
)

type fakeStoreIDs struct {
	productconnect.ProductServiceClient
	response *productpb.ListStoreIDsWithPublicProductsInternalResponse
	err      error
}

func (f fakeStoreIDs) ListStoreIDsWithPublicProductsInternal(ctx context.Context, _ *connect.Request[productpb.ListStoreIDsWithPublicProductsInternalRequest]) (*connect.Response[productpb.ListStoreIDsWithPublicProductsInternalResponse], error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("missing deadline")
	}
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.response), nil
}

func TestStoresWithPublicProducts(t *testing.T) {
	got, err := (&Client{rpc: fakeStoreIDs{response: &productpb.ListStoreIDsWithPublicProductsInternalResponse{StoreIds: []string{"3", "12"}}}}).StoresWithPublicProducts(context.Background())
	if err != nil || !slices.Equal(got, []int64{3, 12}) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	got, err = (&Client{rpc: fakeStoreIDs{response: &productpb.ListStoreIDsWithPublicProductsInternalResponse{}}}).StoresWithPublicProducts(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("empty set: got=%v err=%v", got, err)
	}
}

func TestStoresWithPublicProductsFailsClosed(t *testing.T) {
	for name, f := range map[string]fakeStoreIDs{
		"transport error": {err: errors.New("down")},
		"business code":   {response: &productpb.ListStoreIDsWithPublicProductsInternalResponse{Code: 500}},
		"bad id":          {response: &productpb.ListStoreIDsWithPublicProductsInternalResponse{StoreIds: []string{"3", "x"}}},
		"zero id":         {response: &productpb.ListStoreIDsWithPublicProductsInternalResponse{StoreIds: []string{"0"}}},
		"duplicate":       {response: &productpb.ListStoreIDsWithPublicProductsInternalResponse{StoreIds: []string{"3", "3"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := (&Client{rpc: f}).StoresWithPublicProducts(context.Background()); err == nil {
				t.Fatalf("accepted: %v", got)
			}
		})
	}
}
