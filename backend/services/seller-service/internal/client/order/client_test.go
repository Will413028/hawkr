package order

import (
	"context"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"google.golang.org/grpc"
)

// fakeOrderServiceClient satisfies pb.OrderServiceClient by embedding the
// interface (unimplemented methods panic-if-called) and overriding only
// GetTopSellersByRevenue, capturing the outgoing request for assertions.
type fakeOrderServiceClient struct {
	pb.OrderServiceClient
	gotReq *pb.GetTopSellersByRevenueRequest
	resp   *pb.GetTopSellersByRevenueResponse
}

func (f *fakeOrderServiceClient) GetTopSellersByRevenue(_ context.Context, in *pb.GetTopSellersByRevenueRequest, _ ...grpc.CallOption) (*pb.GetTopSellersByRevenueResponse, error) {
	f.gotReq = in
	return f.resp, nil
}

func TestGetTopSellersByRevenue_ParsesStoreIdString(t *testing.T) {
	const wireStoreID = "1700000000123"
	fakeClient := &fakeOrderServiceClient{
		resp: &pb.GetTopSellersByRevenueResponse{
			Code: 0,
			Sellers: []*pb.SellerRevenue{
				{StoreId: wireStoreID, TotalRevenue: 50000, OrderCount: 10},
			},
		},
	}
	c := &Client{client: fakeClient}

	result, err := c.GetTopSellersByRevenue(context.Background(), 30, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if got, want := result[0].StoreID, int64(1700000000123); got != want {
		t.Errorf("StoreID = %d, want %d (parsed from wire store_id %q)", got, want, wireStoreID)
	}
}

// TestGetTopSellersByRevenue_InvalidStoreIdReturnsError guards the failure path:
// a non-numeric wire value must surface as an error, not silently zero out (which
// would make a store indistinguishable from "no seller", corrupting the
// recommended-sellers ranking).
func TestGetTopSellersByRevenue_InvalidStoreIdReturnsError(t *testing.T) {
	fakeClient := &fakeOrderServiceClient{
		resp: &pb.GetTopSellersByRevenueResponse{
			Code:    0,
			Sellers: []*pb.SellerRevenue{{StoreId: "not-a-number", TotalRevenue: 1, OrderCount: 1}},
		},
	}
	c := &Client{client: fakeClient}

	if _, err := c.GetTopSellersByRevenue(context.Background(), 30, 10); err == nil {
		t.Fatal("expected error for non-numeric store_id, got nil")
	}
}
