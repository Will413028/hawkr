package seller

import (
	"context"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"google.golang.org/grpc"
)

// fakeSellerServiceClient satisfies sellerpb.SellerServiceClient by embedding the
// interface (unimplemented methods panic-if-called) and overriding only
// GetSellerStats.
type fakeSellerServiceClient struct {
	sellerpb.SellerServiceClient
	getSellerStatsFn func(ctx context.Context, in *sellerpb.GetSellerStatsRequest, opts ...grpc.CallOption) (*sellerpb.SellerStatsResponse, error)
}

func (f *fakeSellerServiceClient) GetSellerStats(ctx context.Context, in *sellerpb.GetSellerStatsRequest, opts ...grpc.CallOption) (*sellerpb.SellerStatsResponse, error) {
	return f.getSellerStatsFn(ctx, in, opts...)
}

// TestGetSellerStats_SendsStoreIdNotAccountId is the sibling regression to issue
// #2056: fetchAndScore (recommend_usecase.go) collects the id it passes here from
// ProductListItem.account_id, which product-service documents as actually
// carrying product.store_id (the store surrogate), not a login account_id.
// GetSellerStatsRequest has a dedicated store_id field for exactly this PK lookup
// (② re-key); sending the value via account_id instead only coincidentally works
// for grandfathered sellers (account_id == store_id) and silently 404s once a
// seller has deleted and re-onboarded their store. This test fails if the adapter
// regresses to populating AccountId instead of StoreId.
func TestGetSellerStats_SendsStoreIdNotAccountId(t *testing.T) {
	const storeID identity.StoreID = 9009 // deliberately reopened-store shaped: != any account_id
	fake := &fakeSellerServiceClient{
		getSellerStatsFn: func(_ context.Context, in *sellerpb.GetSellerStatsRequest, _ ...grpc.CallOption) (*sellerpb.SellerStatsResponse, error) {
			if in.GetStoreId() != int64(storeID) {
				t.Fatalf("GetSellerStatsRequest.StoreId = %d, want %d", in.GetStoreId(), storeID)
			}
			if in.GetAccountId() != 0 {
				t.Fatalf("GetSellerStatsRequest.AccountId = %d, want 0 (store surrogate must go through store_id, not account_id)", in.GetAccountId())
			}
			return &sellerpb.SellerStatsResponse{
				Code: 0,
				Data: &sellerpb.SellerStats{
					StoreId:       strconv.FormatInt(int64(storeID), 10),
					TotalProducts: 3,
					Rating:        4.5,
				},
			}, nil
		},
	}
	c := &sellerClient{client: fake}

	got, err := c.GetSellerStats(context.Background(), storeID)
	if err != nil {
		t.Fatalf("GetSellerStats() unexpected error: %v", err)
	}
	if got.StoreID != storeID {
		t.Fatalf("GetSellerStats().StoreID = %d, want %d", got.StoreID, storeID)
	}
	if got.TotalProducts != 3 {
		t.Fatalf("GetSellerStats().TotalProducts = %d, want 3", got.TotalProducts)
	}
}
