package buyer

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc"
)

// fakeBuyerServiceClient satisfies pb.BuyerServiceClient by embedding the
// interface (so unimplemented methods panic-if-called rather than requiring
// a full stub) and overriding only the methods a test needs.
type fakeBuyerServiceClient struct {
	pb.BuyerServiceClient
	resolvePrincipalsFn func(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error)
}

func (f *fakeBuyerServiceClient) ResolvePrincipals(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
	return f.resolvePrincipalsFn(ctx, in, opts...)
}

// TestResolvePrincipal_RPCError_FailsOpen mirrors seller.Client's test of the
// same name: an error alongside a non-nil response (with a *different*
// principal) proves ResolvePrincipal checks err != nil before trusting resp.
func TestResolvePrincipal_RPCError_FailsOpen(t *testing.T) {
	const buyerID int64 = 42
	fake := &fakeBuyerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{buyerID: 999},
			}, errors.New("buyer-service unavailable")
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), buyerID)
	if got != buyerID {
		t.Fatalf("ResolvePrincipal() on RPC error = %d, want unchanged input %d", got, buyerID)
	}
}

// TestResolvePrincipal_NonZeroCode_FailsOpen covers the "no transport error,
// but the response carries a business-error code" branch.
func TestResolvePrincipal_NonZeroCode_FailsOpen(t *testing.T) {
	const buyerID int64 = 42
	fake := &fakeBuyerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       1,
				Principals: map[int64]int64{buyerID: 999},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), buyerID)
	if got != buyerID {
		t.Fatalf("ResolvePrincipal() on code != 0 = %d, want unchanged input %d", got, buyerID)
	}
}

// TestResolvePrincipal_IDNotInMap covers a successful RPC response that
// simply doesn't know about the given id (not a buyer_id).
func TestResolvePrincipal_IDNotInMap(t *testing.T) {
	const buyerID int64 = 42
	fake := &fakeBuyerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), buyerID)
	if got != buyerID {
		t.Fatalf("ResolvePrincipal() with id absent from map = %d, want unchanged input %d", got, buyerID)
	}
}

// TestResolvePrincipal_Resolved is the positive-path control: proves the
// method actually returns the resolved principal when the RPC succeeds and
// the id is present, so the fail-open tests above aren't vacuously true.
func TestResolvePrincipal_Resolved(t *testing.T) {
	const buyerID int64 = 42
	const principalAccountID int64 = 7
	fake := &fakeBuyerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{buyerID: principalAccountID},
			}, nil
		},
	}
	c := &Client{client: fake}

	got := c.ResolvePrincipal(context.Background(), buyerID)
	if got != principalAccountID {
		t.Fatalf("ResolvePrincipal() resolved = %d, want %d", got, principalAccountID)
	}
}
