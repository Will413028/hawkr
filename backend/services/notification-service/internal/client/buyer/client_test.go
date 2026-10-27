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
// a full stub) and overriding only the methods a test needs. Mirrors
// im-service/internal/client/buyer's fake of the same name.
type fakeBuyerServiceClient struct {
	pb.BuyerServiceClient
	listBuyerIDsByAccountFn func(ctx context.Context, in *pb.ListBuyerIDsByAccountRequest, opts ...grpc.CallOption) (*pb.ListBuyerIDsByAccountResponse, error)
}

func (f *fakeBuyerServiceClient) ListBuyerIDsByAccount(ctx context.Context, in *pb.ListBuyerIDsByAccountRequest, opts ...grpc.CallOption) (*pb.ListBuyerIDsByAccountResponse, error) {
	return f.listBuyerIDsByAccountFn(ctx, in, opts...)
}

// TestListIDsByAccount_ParsesEveryID is the positive-path control: proves
// every returned buyer_id string is parsed into the int64 the redaction
// usecase enumerates against.
func TestListIDsByAccount_ParsesEveryID(t *testing.T) {
	const accountID int64 = 7
	fake := &fakeBuyerServiceClient{
		listBuyerIDsByAccountFn: func(_ context.Context, in *pb.ListBuyerIDsByAccountRequest, _ ...grpc.CallOption) (*pb.ListBuyerIDsByAccountResponse, error) {
			if in.GetAccountId() != accountID {
				t.Fatalf("request AccountId = %d, want %d", in.GetAccountId(), accountID)
			}
			return &pb.ListBuyerIDsByAccountResponse{BuyerIds: []string{"101", "202"}}, nil
		},
	}
	c := &Client{client: fake}

	got, err := c.ListIDsByAccount(context.Background(), accountID)
	if err != nil {
		t.Fatalf("ListIDsByAccount() error = %v", err)
	}
	want := []int64{101, 202}
	if len(got) != len(want) {
		t.Fatalf("ListIDsByAccount() = %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("ListIDsByAccount()[%d] = %d, want %d", i, got[i], id)
		}
	}
}

// TestListIDsByAccount_SkipsMalformedID locks the defensive skip-on-parse-error
// branch: buyer-service's buyer_id field is a TSID-as-string that should always
// parse, but the redaction path must not silently corrupt or abort the whole
// enumeration if it ever doesn't — malformed entries are dropped, not fatal.
func TestListIDsByAccount_SkipsMalformedID(t *testing.T) {
	fake := &fakeBuyerServiceClient{
		listBuyerIDsByAccountFn: func(_ context.Context, _ *pb.ListBuyerIDsByAccountRequest, _ ...grpc.CallOption) (*pb.ListBuyerIDsByAccountResponse, error) {
			return &pb.ListBuyerIDsByAccountResponse{BuyerIds: []string{"101", "not-a-number", "202"}}, nil
		},
	}
	c := &Client{client: fake}

	got, err := c.ListIDsByAccount(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListIDsByAccount() error = %v", err)
	}
	want := []int64{101, 202}
	if len(got) != len(want) {
		t.Fatalf("ListIDsByAccount() = %v, want %v (malformed entry dropped)", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("ListIDsByAccount()[%d] = %d, want %d", i, got[i], id)
		}
	}
}

// TestListIDsByAccount_RPCError_Wrapped proves a transport/business error
// from buyer-service is surfaced to the caller (not swallowed into an empty
// slice) — the redaction usecase's own enumeration step must be able to tell
// "confirmed zero buyer_ids" apart from "buyer-service was unreachable".
func TestListIDsByAccount_RPCError_Wrapped(t *testing.T) {
	rpcErr := errors.New("buyer-service unavailable")
	fake := &fakeBuyerServiceClient{
		listBuyerIDsByAccountFn: func(_ context.Context, _ *pb.ListBuyerIDsByAccountRequest, _ ...grpc.CallOption) (*pb.ListBuyerIDsByAccountResponse, error) {
			return nil, rpcErr
		},
	}
	c := &Client{client: fake}

	_, err := c.ListIDsByAccount(context.Background(), 7)
	if err == nil {
		t.Fatal("ListIDsByAccount() error = nil, want wrapped RPC error")
	}
	if !errors.Is(err, rpcErr) {
		t.Errorf("ListIDsByAccount() error = %v, want it to wrap %v", err, rpcErr)
	}
}
