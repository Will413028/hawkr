package seller

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"google.golang.org/grpc"
)

// fakeSellerServiceClient satisfies pb.SellerServiceClient by embedding the
// interface (so unimplemented methods panic-if-called rather than requiring
// a full stub) and overriding only the methods a test needs.
type fakeSellerServiceClient struct {
	pb.SellerServiceClient
	resolvePrincipalsFn func(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error)
	getSellerBatchFn    func(ctx context.Context, in *pb.GetSellerBatchRequest, opts ...grpc.CallOption) (*pb.SellerBatchResponse, error)
}

func (f *fakeSellerServiceClient) ResolvePrincipals(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
	return f.resolvePrincipalsFn(ctx, in, opts...)
}

func (f *fakeSellerServiceClient) GetSellerBatch(ctx context.Context, in *pb.GetSellerBatchRequest, opts ...grpc.CallOption) (*pb.SellerBatchResponse, error) {
	return f.getSellerBatchFn(ctx, in, opts...)
}

// TestResolvePrincipal_RPCError_FailsOpen exercises the real client's error
// handling directly (not the usecase-layer mockSellerClient), which was
// previously untested. The fake deliberately returns a non-nil response
// with a *different* principal alongside the error: if ResolvePrincipal
// ever stopped checking `err != nil` first, this test would observe the
// wrong (resolved) id instead of the unchanged input and fail.
func TestResolvePrincipal_RPCError_FailsOpen(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{storeID: 999},
			}, errors.New("seller-service unavailable")
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != storeID {
		t.Fatalf("ResolvePrincipal() on RPC error = %d, want unchanged input %d", got, storeID)
	}
}

// TestResolvePrincipal_NonZeroCode_FailsOpen covers the "no transport error,
// but the response carries a business-error code" branch.
func TestResolvePrincipal_NonZeroCode_FailsOpen(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       1,
				Principals: map[int64]int64{storeID: 999},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != storeID {
		t.Fatalf("ResolvePrincipal() on code != 0 = %d, want unchanged input %d", got, storeID)
	}
}

// TestResolvePrincipal_IDNotInMap covers a successful RPC response that
// simply doesn't know about the given id (not a store_id).
func TestResolvePrincipal_IDNotInMap(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != storeID {
		t.Fatalf("ResolvePrincipal() with id absent from map = %d, want unchanged input %d", got, storeID)
	}
}

// TestResolvePrincipal_Resolved is the positive-path control: proves the
// method actually returns the resolved principal when the RPC succeeds and
// the id is present, so the fail-open tests above aren't vacuously true.
func TestResolvePrincipal_Resolved(t *testing.T) {
	const storeID int64 = 42
	const principalAccountID int64 = 7
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{storeID: principalAccountID},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != principalAccountID {
		t.Fatalf("ResolvePrincipal() resolved = %d, want %d", got, principalAccountID)
	}
}

// TestGetProfileBatch_DivergedClosed exercises the real re-key + StoreClosed
// derivation directly (the usecase-layer mockSellerClient bypasses this
// parsing entirely). A diverged incarnation carries a distinct StoreId
// surrogate (777) that differs from its principal AccountId (42) — the
// result map MUST key on the surrogate, because callers look it up by the
// room id's s side. An inverted fallback would key on 42, the seller
// couldn't be found by 777, and conversations would silently vanish.
func TestGetProfileBatch_DivergedClosed(t *testing.T) {
	fake := &fakeSellerServiceClient{
		getSellerBatchFn: func(_ context.Context, _ *pb.GetSellerBatchRequest, _ ...grpc.CallOption) (*pb.SellerBatchResponse, error) {
			return &pb.SellerBatchResponse{
				Data: []*pb.SellerInfo{
					{AccountId: 42, StoreId: "777", LifecycleStatus: "closed", Name: "老王鮮魚", LogoKey: "logo.png"},
				},
			}, nil
		},
	}
	c := &Client{client: fake}

	result, err := c.GetProfileBatch(context.Background(), []int64{777})
	if err != nil {
		t.Fatalf("GetProfileBatch() error: %v", err)
	}
	if _, ok := result[42]; ok {
		t.Errorf("result keyed on principal 42; want surrogate 777 only")
	}
	p, ok := result[777]
	if !ok {
		t.Fatalf("result missing surrogate key 777; keys = %v", keysOf(result))
	}
	if !p.StoreClosed {
		t.Errorf("StoreClosed = false, want true for LifecycleStatus=closed")
	}
	if p.StoreName != "老王鮮魚" {
		t.Errorf("StoreName = %q, want 老王鮮魚", p.StoreName)
	}
}

// TestGetProfileBatch_RejectsMissingStoreID prevents AccountId from silently
// becoming a surrogate map key when seller-service violates the batch contract.
func TestGetProfileBatch_RejectsMissingStoreID(t *testing.T) {
	fake := &fakeSellerServiceClient{
		getSellerBatchFn: func(_ context.Context, _ *pb.GetSellerBatchRequest, _ ...grpc.CallOption) (*pb.SellerBatchResponse, error) {
			return &pb.SellerBatchResponse{
				Data: []*pb.SellerInfo{
					{AccountId: 42, StoreId: "", LifecycleStatus: "active", Name: "鮮魚舖"},
				},
			}, nil
		},
	}
	c := &Client{client: fake}

	result, err := c.GetProfileBatch(context.Background(), []int64{42})
	if err == nil {
		t.Fatalf("GetProfileBatch() result = %v, want error for missing store_id", result)
	}
}

// TestGetProfileBatch_DeletedIsClosed locks the ||-"deleted" branch of the
// StoreClosed derivation (reshape-residual value maps to a tombstone too).
func TestGetProfileBatch_DeletedIsClosed(t *testing.T) {
	fake := &fakeSellerServiceClient{
		getSellerBatchFn: func(_ context.Context, _ *pb.GetSellerBatchRequest, _ ...grpc.CallOption) (*pb.SellerBatchResponse, error) {
			return &pb.SellerBatchResponse{
				Data: []*pb.SellerInfo{
					{AccountId: 42, StoreId: "777", LifecycleStatus: "deleted", Name: "舊舖"},
				},
			}, nil
		},
	}
	c := &Client{client: fake}

	result, err := c.GetProfileBatch(context.Background(), []int64{777})
	if err != nil {
		t.Fatalf("GetProfileBatch() error: %v", err)
	}
	if !result[777].StoreClosed {
		t.Errorf("StoreClosed = false, want true for LifecycleStatus=deleted")
	}
}

func keysOf(m map[int64]*ProfileInfo) []int64 {
	ks := make([]int64, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// TestResolvePrincipal_UnresolvedID_EchoesBack locks the im-service half of
// issue #2789's contract fix: an id seller-service reports in unresolved_ids
// (soft-deleted stint, or unknown store_id) must still return the raw store_id.
//
// The fake deliberately returns principals[storeID] = 999 alongside the
// unresolved_ids entry. That is exactly the shape the real server cannot
// produce (it echoes the input back), and it is chosen on purpose: if this
// method ever started trusting the map over unresolved_ids, the assertion
// would observe 999 instead of the input and fail. Against a realistic
// echo-back payload the test would be vacuously true.
func TestResolvePrincipal_UnresolvedID_EchoesBack(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:          0,
				Principals:    map[int64]int64{storeID: 999},
				UnresolvedIds: []int64{storeID},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != storeID {
		t.Fatalf("ResolvePrincipal() for an id in unresolved_ids = %d, want unchanged input %d", got, storeID)
	}
}

// TestResolvePrincipal_UnresolvedIDsIgnoredForOtherIDs is the negative control
// for the loop above: an unresolved_ids list that does NOT contain the queried
// id must not suppress a genuine resolution. Without this, "return storeID
// whenever unresolved_ids is non-empty" would pass the test above.
func TestResolvePrincipal_UnresolvedIDsIgnoredForOtherIDs(t *testing.T) {
	const storeID int64 = 42
	const principalAccountID int64 = 7
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:          0,
				Principals:    map[int64]int64{storeID: principalAccountID},
				UnresolvedIds: []int64{storeID + 1},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != principalAccountID {
		t.Fatalf("ResolvePrincipal() = %d, want %d (an unrelated id in unresolved_ids must not suppress this resolution)", got, principalAccountID)
	}
}
