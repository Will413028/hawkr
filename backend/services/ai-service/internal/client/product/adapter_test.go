package product

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"google.golang.org/grpc"
)

// fakeProductServiceClient satisfies productpb.ProductServiceClient by embedding
// the interface (unimplemented methods panic-if-called) and overriding only
// GetProductList, capturing the outgoing request for assertions.
type fakeProductServiceClient struct {
	productpb.ProductServiceClient
	gotReq *productpb.GetProductListRequest
}

func (f *fakeProductServiceClient) GetProductList(_ context.Context, in *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
	f.gotReq = in
	return &productpb.GetProductListResponse{Code: 0, Products: nil, Total: "0"}, nil
}

// TestListSellerProducts_ForwardsStoreID pins that the storeID param (already
// the caller's CURRENT store_id — resolved by gateway before this call reaches
// ai-service, account_id 命名債正名) is forwarded to product-service as-is via
// GetProductListRequest.StoreId, with no further resolution in this adapter.
func TestListSellerProducts_ForwardsStoreID(t *testing.T) {
	const storeID identity.StoreID = 8001

	fakeClient := &fakeProductServiceClient{}
	c := &productClient{client: fakeClient}

	if _, err := c.ListSellerProducts(context.Background(), storeID, 1, 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeClient.gotReq == nil {
		t.Fatal("GetProductList was never called")
	}
	if got, want := fakeClient.gotReq.GetStoreId(), strconv.FormatInt(int64(storeID), 10); got != want {
		t.Errorf("GetProductListRequest.StoreId = %q, want %q", got, want)
	}
}

func TestSearchProducts_ForwardsKeywordAndPriceMaxFilter(t *testing.T) {
	fakeClient := &fakeProductServiceClient{}
	c := &productClient{client: fakeClient}

	priceMax := 800
	_, err := c.SearchProducts(context.Background(), usecase.ProductSearchFilter{
		Keyword: "寶島梨禮盒", PriceMaxNTD: &priceMax,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeClient.gotReq == nil {
		t.Fatal("GetProductList was never called")
	}
	if got, want := fakeClient.gotReq.GetKeyword(), "寶島梨禮盒"; got != want {
		t.Errorf("Keyword = %q, want %q", got, want)
	}
	if got, want := fakeClient.gotReq.GetPriceMaxFilter(), "800"; got != want {
		t.Errorf("PriceMaxFilter = %q, want %q", got, want)
	}
	if got, want := fakeClient.gotReq.GetStatus(), "4004"; got != want {
		t.Errorf("Status = %q, want %q (listed only)", got, want)
	}
}

func TestSearchProducts_OmitsPriceMaxFilterWhenNil(t *testing.T) {
	fakeClient := &fakeProductServiceClient{}
	c := &productClient{client: fakeClient}

	_, err := c.SearchProducts(context.Background(), usecase.ProductSearchFilter{Keyword: "水果"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := fakeClient.gotReq.GetPriceMaxFilter(); got != "" {
		t.Errorf("PriceMaxFilter = %q, want empty when nil", got)
	}
}

// fakeSellerScopedProductClient overrides only GetProduct (the permission-checked,
// cache-free RPC GetProductStoreID uses), capturing the outgoing request.
type fakeSellerScopedProductClient struct {
	productpb.ProductServiceClient
	gotReq *productpb.GetProductRequest
	resp   *productpb.GetProductResponse
}

func (f *fakeSellerScopedProductClient) GetProduct(_ context.Context, in *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	f.gotReq = in
	return f.resp, nil
}

// TestGetProductStoreID_ReturnsOwnerAndForwardsCaller pins the two things the
// procurement seller-confirm authorization check depends on: the owner comes from
// ProductDetail.seller.store_id (product.proto: 「擁有權見 SellerInfo.store_id」,
// created_by is audit-only), and the caller's own store_id is forwarded so
// product-service applies owner visibility (a seller must still be able to confirm
// an invite for a product they have since delisted).
func TestGetProductStoreID_ReturnsOwnerAndForwardsCaller(t *testing.T) {
	fakeClient := &fakeSellerScopedProductClient{resp: &productpb.GetProductResponse{
		Code: 0,
		Data: &productpb.ProductDetail{Seller: &productpb.SellerInfo{StoreId: "8001"}},
	}}
	c := &productClient{client: fakeClient}

	got, err := c.GetProductStoreID(context.Background(), 101, identity.StoreID(9002))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != identity.StoreID(8001) {
		t.Errorf("GetProductStoreID = %d, want 8001 (the product's owner, not the caller)", got)
	}
	if fakeClient.gotReq.GetProductId() != "101" {
		t.Errorf("ProductId = %q, want %q", fakeClient.gotReq.GetProductId(), "101")
	}
	if fakeClient.gotReq.GetStoreId() != 9002 {
		t.Errorf("StoreId = %d, want 9002 (caller store id forwarded for visibility)", fakeClient.gotReq.GetStoreId())
	}
}

// Every "can't tell who owns it" shape must be an error, never a silent 0 — a 0
// would be compared against the caller's store_id and happen to be safe, which is
// luck rather than design.
func TestGetProductStoreID_UnavailableCases(t *testing.T) {
	cases := []struct {
		name string
		resp *productpb.GetProductResponse
	}{
		{"non-zero in-band code (not found / access denied)", &productpb.GetProductResponse{Code: 404, Message: "商品不存在"}},
		{"no data", &productpb.GetProductResponse{Code: 0}},
		{"no seller block", &productpb.GetProductResponse{Code: 0, Data: &productpb.ProductDetail{}}},
		{"empty store id", &productpb.GetProductResponse{Code: 0, Data: &productpb.ProductDetail{Seller: &productpb.SellerInfo{}}}},
		{"zero store id", &productpb.GetProductResponse{Code: 0, Data: &productpb.ProductDetail{Seller: &productpb.SellerInfo{StoreId: "0"}}}},
		{"non-numeric store id", &productpb.GetProductResponse{Code: 0, Data: &productpb.ProductDetail{Seller: &productpb.SellerInfo{StoreId: "abc"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &productClient{client: &fakeSellerScopedProductClient{resp: tc.resp}}

			got, err := c.GetProductStoreID(context.Background(), 101, identity.StoreID(9001))
			if !errors.Is(err, usecase.ErrProductOwnerUnavailable) {
				t.Fatalf("err = %v, want ErrProductOwnerUnavailable", err)
			}
			if got != 0 {
				t.Errorf("storeID = %d, want 0 alongside the error", got)
			}
		})
	}
}
