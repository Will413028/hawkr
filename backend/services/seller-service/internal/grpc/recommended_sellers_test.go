package grpc

import (
	"context"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// recoOrderClientStub feeds the revenue ranking stage a single fixed row.
type recoOrderClientStub struct{}

func (recoOrderClientStub) GetTopSellersByRevenue(_ context.Context, _, _ int32) ([]usecase.SellerRevenue, error) {
	return []usecase.SellerRevenue{{StoreID: 1001, TotalRevenue: 50000, OrderCount: 10}}, nil
}

// recoSellerRepoStub only implements the two reads the recommendation use case
// makes; the embedded interface supplies the rest of domain.SellerRepository
// (nil — any unexpected call panics loudly rather than passing silently).
type recoSellerRepoStub struct {
	domain.SellerRepository
	revenueRows  []*domain.Seller
	fallbackRows []*domain.Seller
}

func (s *recoSellerRepoStub) GetByStoreIDs(_ context.Context, _ []int64) ([]*domain.Seller, error) {
	return s.revenueRows, nil
}

func (s *recoSellerRepoStub) GetApprovedExcluding(_ context.Context, _ []int64, _ domain.PublicProductStores, _ int32) ([]*domain.Seller, error) {
	return s.fallbackRows, nil
}

// recoProductClientStub answers "these stores have a public product". The two ids
// are the fixture's own rows, so the recommendation filter lets both through and
// this test stays about the wire contract rather than about the filter.
type recoProductClientStub struct{}

func (recoProductClientStub) GetProductsByIDs(_ context.Context, _ []int64) ([]*usecase.ProductInfo, error) {
	return nil, nil
}

func (recoProductClientStub) ListStoreIDsWithPublicProducts(_ context.Context) ([]int64, error) {
	return []int64{1001, 2001}, nil
}

// TestGRPC_GetRecommendedSellers_EmitsStoreCode pins store_code onto the wire
// contract of this RPC (web#365). The domain carries the display code as an
// int64 while every response-side identifier is a string (§4 / protonumericlint),
// so the conversion at this boundary is exactly the step that can silently
// vanish — leaving the storefront rails to fall back to the numeric store id and
// pay a 308 redirect per link.
func TestGRPC_GetRecommendedSellers_EmitsStoreCode(t *testing.T) {
	repo := &recoSellerRepoStub{
		revenueRows: []*domain.Seller{
			{AccountID: 1001, StoreID: 1001, StoreCode: 2612345678, StoreName: "Store A"},
		},
		fallbackRows: []*domain.Seller{
			{AccountID: 2001, StoreID: 2001, StoreCode: 2687654321, StoreName: "Store B"},
		},
	}
	uc := usecase.NewRecommendedSellersUseCase(recoOrderClientStub{}, repo, &testBuyerClient{}, recoProductClientStub{})
	srv := &SellerServer{recommendedSellersUC: uc}

	resp, err := srv.GetRecommendedSellers(context.Background(), &pb.GetRecommendedSellersRequest{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("Code = %d, want 0 (message=%q)", resp.Code, resp.Message)
	}
	if len(resp.Sellers) != 2 {
		t.Fatalf("Sellers len = %d, want 2 (1 revenue + 1 fallback)", len(resp.Sellers))
	}
	if got := resp.Sellers[0].GetStoreCode(); got != "2612345678" {
		t.Errorf("revenue row store_code = %q, want %q", got, "2612345678")
	}
	if got := resp.Sellers[1].GetStoreCode(); got != "2687654321" {
		t.Errorf("fallback row store_code = %q, want %q", got, "2687654321")
	}
}
