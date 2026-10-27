package seller

import (
	"context"
	"errors"
	"testing"

	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"google.golang.org/grpc"
)

// fakeSellerServiceClient satisfies sellerpb.SellerServiceClient by embedding
// the interface (so unimplemented methods panic-if-called rather than
// requiring a full stub) and overriding only ResolvePrincipals.
type fakeSellerServiceClient struct {
	sellerpb.SellerServiceClient
	resolvePrincipalsFn func(ctx context.Context, in *sellerpb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error)
}

func (f *fakeSellerServiceClient) ResolvePrincipals(ctx context.Context, in *sellerpb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
	return f.resolvePrincipalsFn(ctx, in, opts...)
}

// TestResolvePrincipal_UnresolvedID_ReturnsZero pins the semantic inversion
// made in #2789 stage B2a. This test previously asserted the OPPOSITE — that an
// id in unresolved_ids comes back as the raw store_id — and the fail-open
// behaviour it locked in was described there as deliberately out of scope.
// It is now in scope and reversed: this is review-service's notification path,
// whose value's only destination is review.created's notify_seller_account_id.
// Echoing a store-space id into an account-space recipient field addresses
// nobody and does it silently; 0 makes notification-service's extractUserID
// (id <= 0 -> skip + WARN) take over. Seeing storeID here again is a
// REGRESSION, not the old design being restored.
//
// The fake deliberately returns principals[storeID] = 999 alongside the
// unresolved_ids entry — a shape the real server cannot produce (it echoes the
// input back), chosen on purpose: if this method ever started trusting the map
// over unresolved_ids, the assertion would observe 999 instead of 0 and fail.
// Against a realistic echo-back payload it could not tell 0 from "trusted the
// map", because the map's value would equal the input.
func TestResolvePrincipal_UnresolvedID_ReturnsZero(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *sellerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
			return &sellerpb.ResolvePrincipalsResponse{
				Code:          0,
				Principals:    map[int64]int64{storeID: 999},
				UnresolvedIds: []int64{storeID},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != 0 {
		t.Fatalf("ResolvePrincipal() for an id in unresolved_ids = %d, want 0 (echoing the input %d back would publish a store-space id into an account-space recipient field)", got, storeID)
	}
}

// TestResolvePrincipal_IDNotInMap_ReturnsZero covers the fourth failure branch:
// a clean response whose principals map simply omits the queried id. The real
// server's unnest LEFT JOIN always fills the map, so this is unreachable in
// production today — it is pinned anyway because it is the one branch a future
// server-side query rewrite could start producing, and the wrong answer there
// (the raw store_id) is invisible rather than loud.
func TestResolvePrincipal_IDNotInMap_ReturnsZero(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *sellerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
			return &sellerpb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{storeID + 1: 999},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != 0 {
		t.Fatalf("ResolvePrincipal() with the id missing from principals = %d, want 0", got)
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
		resolvePrincipalsFn: func(_ context.Context, _ *sellerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
			return &sellerpb.ResolvePrincipalsResponse{
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

// TestResolvePrincipal_RPCError_ReturnsZero and _NonZeroCode_ReturnsZero cover
// the two transport/business failure branches. They too asserted the raw
// store_id before #2789 stage B2a; same inversion and same reasoning as
// TestResolvePrincipal_UnresolvedID_ReturnsZero above.
//
// Both fakes return a principals map containing a DIFFERENT value (999) so the
// assertion cannot pass by accident: 0 can only come from the failure branch,
// never from reading the map.
func TestResolvePrincipal_RPCError_ReturnsZero(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *sellerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
			return &sellerpb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{storeID: 999},
			}, errors.New("seller-service unavailable")
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != 0 {
		t.Fatalf("ResolvePrincipal() on RPC error = %d, want 0 (not the raw input %d)", got, storeID)
	}
}

func TestResolvePrincipal_NonZeroCode_ReturnsZero(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *sellerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
			return &sellerpb.ResolvePrincipalsResponse{
				Code:       1,
				Principals: map[int64]int64{storeID: 999},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), storeID)
	if got != 0 {
		t.Fatalf("ResolvePrincipal() on code != 0 = %d, want 0 (not the raw input %d)", got, storeID)
	}
}

// TestResolvePrincipal_Resolved is the positive-path control, so the
// return-zero tests above aren't vacuously true: a method that returned 0
// unconditionally would fail here.
func TestResolvePrincipal_Resolved(t *testing.T) {
	const storeID int64 = 42
	const principalAccountID int64 = 7
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *sellerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
			return &sellerpb.ResolvePrincipalsResponse{
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
