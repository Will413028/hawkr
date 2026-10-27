package seller

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"google.golang.org/grpc"
)

// fakeSellerServiceClient satisfies pb.SellerServiceClient by embedding the
// interface (so unimplemented methods panic-if-called rather than requiring
// a full stub) and overriding only ResolvePrincipals / GetSeller.
type fakeSellerServiceClient struct {
	pb.SellerServiceClient
	resolvePrincipalsFn func(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error)
	getSellerFn         func(ctx context.Context, in *pb.GetSellerRequest, opts ...grpc.CallOption) (*pb.SellerResponse, error)
}

func (f *fakeSellerServiceClient) ResolvePrincipals(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
	return f.resolvePrincipalsFn(ctx, in, opts...)
}

func (f *fakeSellerServiceClient) GetSeller(ctx context.Context, in *pb.GetSellerRequest, opts ...grpc.CallOption) (*pb.SellerResponse, error) {
	return f.getSellerFn(ctx, in, opts...)
}

func TestGetSellerInfo_QueriesByStoreID(t *testing.T) {
	const storeID int64 = 7486784183762280868
	var captured *pb.GetSellerRequest
	fake := &fakeSellerServiceClient{
		getSellerFn: func(_ context.Context, in *pb.GetSellerRequest, _ ...grpc.CallOption) (*pb.SellerResponse, error) {
			captured = in
			return &pb.SellerResponse{
				Code: 0,
				Data: &pb.SellerInfo{Name: "測試", LogoKey: "public/user_avatar/x.jpg"},
			}, nil
		},
	}
	c := &Client{client: fake}

	info, err := c.GetSellerInfo(context.Background(), storeID)
	if err != nil {
		t.Fatalf("GetSellerInfo() error = %v", err)
	}
	if captured.GetStoreId() != storeID {
		t.Fatalf("GetSellerRequest.StoreId = %d, want %d", captured.GetStoreId(), storeID)
	}
	if captured.GetAccountId() != 0 {
		t.Fatalf("GetSellerRequest.AccountId = %d, want 0 (must not reuse the store_id as account_id)", captured.GetAccountId())
	}
	if info.Nickname != "測試" || info.Avatar != "public/user_avatar/x.jpg" {
		t.Fatalf("GetSellerInfo() = %+v, want populated from the fake response", info)
	}
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

// TestGetSellerInfo_NotFound_ReturnsNil 鎖住「查不到 → nil」這個契約。
//
// 查不到時回 (nil, nil)，不是 (&usecase.AccountInfo{}, nil)：空物件會讓 caller 無法分辨
// 「不存在」與「查到但名稱空白」。
func TestGetSellerInfo_NotFound_ReturnsNil(t *testing.T) {
	cases := []struct {
		name string
		fn   func(context.Context, *pb.GetSellerRequest, ...grpc.CallOption) (*pb.SellerResponse, error)
	}{
		{"transport error", func(_ context.Context, _ *pb.GetSellerRequest, _ ...grpc.CallOption) (*pb.SellerResponse, error) {
			return nil, errors.New("seller-service unavailable")
		}},
		{"business error code", func(_ context.Context, _ *pb.GetSellerRequest, _ ...grpc.CallOption) (*pb.SellerResponse, error) {
			return &pb.SellerResponse{Code: 1, Message: "not found"}, nil
		}},
		{"code 0 but nil data", func(_ context.Context, _ *pb.GetSellerRequest, _ ...grpc.CallOption) (*pb.SellerResponse, error) {
			return &pb.SellerResponse{Code: 0, Data: nil}, nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{client: &fakeSellerServiceClient{getSellerFn: tc.fn}}
			info, err := c.GetSellerInfo(context.Background(), 42)
			if err != nil {
				t.Fatalf("GetSellerInfo() err = %v, want nil (best-effort，不把查不到當錯誤)", err)
			}
			if info != nil {
				t.Fatalf("GetSellerInfo() = %+v, want nil —— 查不到必須回 nil，"+
					"回空物件會讓 caller 誤以為這個 id 是賣家", info)
			}
		})
	}
}

// TestNewClient_ConnectsToConfiguredTarget is the behavior baseline for the
// "grpc.NewClient boilerplate → grpcclient.Dial + grpcclient.Timeout5s"
// refactor in this file — see
// services/seller-service/internal/client/media/client_test.go (Task 3) for
// the full rationale.
func TestNewClient_ConnectsToConfiguredTarget(t *testing.T) {
	cfg := &config.Config{
		Seller: config.ExternalServiceConfig{Host: "seller-service", Port: "9090"},
	}

	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

// TestResolvePrincipal_UnresolvedID_EchoesBack locks the social-service half of
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

// TestResolvePrincipalStrict_UnresolvedID_NotFound proves Found is now driven
// by unresolved_ids rather than by map presence. Before issue #2789 added the
// field, Found was unconditionally true (seller-service returns a fully
// populated map, echoing unknown ids back), so this method could not tell a
// real resolution from an echo — and dispatcher.go's BanUser, which branches
// on Found, was a no-op.
//
// principals deliberately carries a value for storeID (that is what the real
// server sends: the echo-back), so the only thing that can make Found false
// here is the unresolved_ids check.
func TestResolvePrincipalStrict_UnresolvedID_NotFound(t *testing.T) {
	const storeID int64 = 42
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:          0,
				Principals:    map[int64]int64{storeID: storeID},
				UnresolvedIds: []int64{storeID},
			}, nil
		},
	}
	c := &Client{client: fake}

	res, err := c.ResolvePrincipalStrict(context.Background(), storeID)
	if err != nil {
		t.Fatalf("ResolvePrincipalStrict() err = %v", err)
	}
	if res.Found {
		t.Fatalf("ResolvePrincipalStrict() = %+v, want Found=false for an id in unresolved_ids", res)
	}
}

// TestResolvePrincipalStrict_Resolved_Found is the positive control: without
// it, hardcoding Found=false would pass the test above.
func TestResolvePrincipalStrict_Resolved_Found(t *testing.T) {
	const storeID int64 = 42
	const accountID int64 = 7
	fake := &fakeSellerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, _ *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{storeID: accountID},
			}, nil
		},
	}
	c := &Client{client: fake}

	res, err := c.ResolvePrincipalStrict(context.Background(), storeID)
	if err != nil {
		t.Fatalf("ResolvePrincipalStrict() err = %v", err)
	}
	if !res.Found || res.AccountID != accountID {
		t.Fatalf("ResolvePrincipalStrict() = %+v, want Found=true AccountID=%d", res, accountID)
	}
}
