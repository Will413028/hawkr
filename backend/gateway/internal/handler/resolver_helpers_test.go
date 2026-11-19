package handler

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"google.golang.org/grpc"
)

// sellerClientResolvingStoreIDForTest wires a Seller client whose
// GetSellerByStoreID (used by requireStore) always
// resolves to storeID, regardless of the id it was called with — enough to
// exercise handlers that resolve a storefront path id (store_id or
// store_code) before querying a downstream service keyed by store_id.
func sellerClientResolvingStoreIDForTest(storeID string) *grpcclient.SellerClient {
	return grpcclient.NewSellerClientForTest(&mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{StoreId: storeID}}, nil
		},
	})
}

// mustNotResolveBuyer returns a resolver that fails the test if consulted —
// proves an anonymous (accountID==0) call never attempts buyer resolution
// (resolveOptionalBuyerID short-circuits before touching the resolver).
func mustNotResolveBuyer(t *testing.T) *buyerid.Resolver {
	t.Helper()
	return buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		t.Fatal("buyer resolver must not be called for anonymous request (accountID==0)")
		return nil, nil
	}))
}
