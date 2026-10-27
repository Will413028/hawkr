package buyer

import (
	"context"
	"errors"
	"fmt"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc"
)

// fakeBuyerServiceClient 只覆寫 GetBuyerProfileByBuyerID，其餘方法由嵌入的介面提供
// （被呼叫才 panic，正好證明本測試沒有意外打到別的 RPC）。
type fakeBuyerServiceClient struct {
	pb.BuyerServiceClient
	getBuyerProfileByBuyerIDFn func(ctx context.Context, in *pb.GetBuyerProfileByBuyerIDRequest, opts ...grpc.CallOption) (*pb.BuyerProfileResponse, error)
	resolvePrincipalsFn        func(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error)
	getSubscribersListFn       func(ctx context.Context, in *pb.GetSubscribersListRequest, opts ...grpc.CallOption) (*pb.SubscribersListResponse, error)
}

func (f *fakeBuyerServiceClient) GetBuyerProfileByBuyerID(ctx context.Context, in *pb.GetBuyerProfileByBuyerIDRequest, opts ...grpc.CallOption) (*pb.BuyerProfileResponse, error) {
	return f.getBuyerProfileByBuyerIDFn(ctx, in, opts...)
}

func (f *fakeBuyerServiceClient) ResolvePrincipals(ctx context.Context, in *pb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
	return f.resolvePrincipalsFn(ctx, in, opts...)
}

func (f *fakeBuyerServiceClient) GetSubscribersList(ctx context.Context, in *pb.GetSubscribersListRequest, opts ...grpc.CallOption) (*pb.SubscribersListResponse, error) {
	return f.getSubscribersListFn(ctx, in, opts...)
}

// TestGetBuyerProfile_NotFound_ReturnsNil 鎖住「查不到 → nil」契約。
// 理由與 seller client 的同名測試相同：空物件無法與真實資料區分。
func TestGetBuyerProfile_NotFound_ReturnsNil(t *testing.T) {
	cases := []struct {
		name string
		fn   func(context.Context, *pb.GetBuyerProfileByBuyerIDRequest, ...grpc.CallOption) (*pb.BuyerProfileResponse, error)
	}{
		{"transport error", func(_ context.Context, _ *pb.GetBuyerProfileByBuyerIDRequest, _ ...grpc.CallOption) (*pb.BuyerProfileResponse, error) {
			return nil, errors.New("buyer-service unavailable")
		}},
		{"business error code", func(_ context.Context, _ *pb.GetBuyerProfileByBuyerIDRequest, _ ...grpc.CallOption) (*pb.BuyerProfileResponse, error) {
			return &pb.BuyerProfileResponse{Code: 1, Message: "not found"}, nil
		}},
		{"code 0 but nil data", func(_ context.Context, _ *pb.GetBuyerProfileByBuyerIDRequest, _ ...grpc.CallOption) (*pb.BuyerProfileResponse, error) {
			return &pb.BuyerProfileResponse{Code: 0, Data: nil}, nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{client: &fakeBuyerServiceClient{getBuyerProfileByBuyerIDFn: tc.fn}}
			info, err := c.GetBuyerProfile(context.Background(), 42)
			if err != nil {
				t.Fatalf("GetBuyerProfile() err = %v, want nil (best-effort)", err)
			}
			if info != nil {
				t.Fatalf("GetBuyerProfile() = %+v, want nil", info)
			}
		})
	}
}

// TestGetBuyerProfile_Found 是正向控制組，證明上面的 nil 斷言不是恆真。
func TestGetBuyerProfile_Found(t *testing.T) {
	c := &Client{client: &fakeBuyerServiceClient{
		getBuyerProfileByBuyerIDFn: func(_ context.Context, in *pb.GetBuyerProfileByBuyerIDRequest, _ ...grpc.CallOption) (*pb.BuyerProfileResponse, error) {
			if in.GetBuyerId() != 42 {
				t.Fatalf("BuyerId = %d, want 42", in.GetBuyerId())
			}
			return &pb.BuyerProfileResponse{
				Code: 0,
				Data: &pb.BuyerInfo{Nickname: "阿明", AvatarKey: "public/buyer_avatar/x.jpg"},
			}, nil
		},
	}}

	info, err := c.GetBuyerProfile(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetBuyerProfile() err = %v", err)
	}
	if info == nil || info.Nickname != "阿明" || info.Avatar != "public/buyer_avatar/x.jpg" {
		t.Fatalf("GetBuyerProfile() = %+v, want populated from the fake response", info)
	}
}

// TestResolvePrincipalStrict_UnresolvedID_NotFound proves Found is now driven
// by ResolvePrincipalsResponse.unresolved_ids, not map-key presence — the map
// still echoes the id back (buyer-service's ResolvePrincipals always returns a
// fully populated map), which is exactly the shape that used to make Found
// wrongly true for an unknown id.
func TestResolvePrincipalStrict_UnresolvedID_NotFound(t *testing.T) {
	const buyerID int64 = 42
	c := &Client{client: &fakeBuyerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, in *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:          0,
				Principals:    map[int64]int64{buyerID: buyerID}, // echoed back, NOT a real resolution
				UnresolvedIds: []int64{buyerID},
			}, nil
		},
	}}

	res, err := c.ResolvePrincipalStrict(context.Background(), buyerID)
	if err != nil {
		t.Fatalf("ResolvePrincipalStrict() err = %v", err)
	}
	if res.Found {
		t.Fatalf("ResolvePrincipalStrict() = %+v, want Found=false for an id in unresolved_ids", res)
	}
}

// TestResolvePrincipalStrict_Resolved_Found is the positive control.
func TestResolvePrincipalStrict_Resolved_Found(t *testing.T) {
	const buyerID int64 = 42
	const accountID int64 = 8001
	c := &Client{client: &fakeBuyerServiceClient{
		resolvePrincipalsFn: func(_ context.Context, in *pb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*pb.ResolvePrincipalsResponse, error) {
			return &pb.ResolvePrincipalsResponse{
				Code:       0,
				Principals: map[int64]int64{buyerID: accountID},
			}, nil
		},
	}}

	res, err := c.ResolvePrincipalStrict(context.Background(), buyerID)
	if err != nil {
		t.Fatalf("ResolvePrincipalStrict() err = %v", err)
	}
	if !res.Found || res.AccountID != accountID {
		t.Fatalf("ResolvePrincipalStrict() = %+v, want Found=true AccountID=%d", res, accountID)
	}
}

// TestGetSubscriberIDs_PaginatesPastServerClamp guards against a regression
// found 2026-08-15: this method requests PageSize=100, but buyer-service
// server-side clamps GetSubscribersList to a lower max (currently 50,
// backend-payuni PR #2402). The loop must keep paging as long as the server
// actually returned a full page (resp.PageSize items) — comparing against
// the *requested* 100 instead made len(resp.Data)==50 < 100 always true, so
// the loop silently stopped after page 1 regardless of how many more
// subscribers existed. Simulates exactly that shape: 2 full pages of 50
// (server-clamped) plus one partial page of 10, 110 subscribers total.
func TestGetSubscriberIDs_PaginatesPastServerClamp(t *testing.T) {
	const storeID int64 = 7487768729113814280
	const serverClampedPageSize = 50
	callCount := 0
	c := &Client{client: &fakeBuyerServiceClient{
		getSubscribersListFn: func(_ context.Context, in *pb.GetSubscribersListRequest, _ ...grpc.CallOption) (*pb.SubscribersListResponse, error) {
			callCount++
			if in.GetStoreId() != storeID {
				t.Fatalf("StoreId = %d, want %d", in.GetStoreId(), storeID)
			}
			if in.GetPageSize() != 100 {
				t.Fatalf("requested PageSize = %d, want 100 (this test doesn't change what's requested, only how the response is interpreted)", in.GetPageSize())
			}
			var n int
			switch in.GetPage() {
			case 1, 2:
				n = serverClampedPageSize // full page — server clamped 100 down to 50
			case 3:
				n = 10 // partial page — last page
			default:
				t.Fatalf("unexpected page %d requested — loop should have stopped after the partial page 3", in.GetPage())
			}
			data := make([]*pb.SubscriberInfo, n)
			for i := range data {
				data[i] = &pb.SubscriberInfo{Id: int64(i + 1), BuyerId: int64(in.GetPage())*1000 + int64(i)}
			}
			return &pb.SubscribersListResponse{
				Code:     0,
				Data:     data,
				Total:    110,
				Page:     fmt.Sprintf("%d", in.GetPage()),
				PageSize: fmt.Sprintf("%d", serverClampedPageSize),
			}, nil
		},
	}}

	ids, err := c.GetSubscriberIDs(context.Background(), storeID)
	if err != nil {
		t.Fatalf("GetSubscriberIDs() err = %v", err)
	}
	if len(ids) != 110 {
		t.Fatalf("GetSubscriberIDs() returned %d ids, want 110 (all 3 pages) — got only page(s) up to the point the loop stopped early", len(ids))
	}
	if callCount != 3 {
		t.Fatalf("GetSubscribersList called %d times, want 3 (2 full server-clamped pages + 1 partial)", callCount)
	}
}
