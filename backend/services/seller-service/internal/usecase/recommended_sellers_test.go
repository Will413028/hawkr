package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

type mockOrderClient struct {
	GetTopSellersByRevenueFn func(ctx context.Context, days, limit int32) ([]SellerRevenue, error)
}

func (m *mockOrderClient) GetTopSellersByRevenue(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
	return m.GetTopSellersByRevenueFn(ctx, days, limit)
}

func TestGetRecommendedSellers_Success(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			if days != 30 {
				t.Errorf("days = %d, want 30", days)
			}
			// The revenue ranking is over-fetched: `limit` bounds the cards that
			// come out, not the candidates that go in (domain.RevenueCandidateFactor).
			if want := domain.DefaultLimit * domain.RevenueCandidateFactor; limit != want {
				t.Errorf("limit = %d, want %d (default limit over-fetched)", limit, want)
			}
			return []SellerRevenue{
				{StoreID: 1001, TotalRevenue: 50000, OrderCount: 10},
				{StoreID: 1002, TotalRevenue: 30000, OrderCount: 5},
			}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			if len(ids) != 2 {
				t.Errorf("expected 2 IDs, got %d", len(ids))
			}
			return []*domain.Seller{
				{AccountID: 1001, StoreID: 1001, StoreName: "Store A", Logo: "a.jpg", CoverKey: "ca.jpg", Description: "Desc A", Rating: 4.8, City: "台北市", District: "大安區"},
				{AccountID: 1002, StoreID: 1002, StoreName: "Store B", Logo: "b.jpg", CoverKey: "cb.jpg", Description: "Desc B", Rating: 4.5, City: "桃園市", District: "桃園區"},
			}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(1001, 1002))
	result, err := uc.GetRecommendedSellers(context.Background(), 0) // 0 → default 10
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].Name != "Store A" {
		t.Errorf("first seller = %q, want Store A", result[0].Name)
	}
	if result[1].Name != "Store B" {
		t.Errorf("second seller = %q, want Store B", result[1].Name)
	}
	if result[0].OrderCount != 10 {
		t.Errorf("first order_count = %d, want 10", result[0].OrderCount)
	}
	if result[0].City != "台北市" {
		t.Errorf("first city = %q, want 台北市", result[0].City)
	}
	if result[0].District != "大安區" {
		t.Errorf("first district = %q, want 大安區", result[0].District)
	}
	if result[1].City != "桃園市" {
		t.Errorf("second city = %q, want 桃園市", result[1].City)
	}
	if result[1].District != "桃園區" {
		t.Errorf("second district = %q, want 桃園區", result[1].District)
	}
}

func TestGetRecommendedSellers_EmptyRevenue(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			return []*domain.Seller{}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores())
	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

// TestGetRecommendedSellers_FiltersInactiveSellers was deleted: it asserted in-usecase
// filtering of non-certified sellers by OverallCertificationStatus, a column that no longer
// exists on domain.Seller. That filtering has moved to the repository layer — GetByStoreIDs
// now filters to fully-certified sellers via a SQL EXISTS join (see its doc comment on
// domain.SellerRepository and the query in internal/repository/postgres/seller_repository.go)
// — so the usecase itself has no filtering logic left to unit-test here.

func TestGetRecommendedSellers_MaintainsRevenueOrder(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{
				{StoreID: 2001, TotalRevenue: 90000, OrderCount: 20},
				{StoreID: 2002, TotalRevenue: 50000, OrderCount: 10},
				{StoreID: 2003, TotalRevenue: 10000, OrderCount: 2},
			}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			// Return in different order than revenue ranking
			return []*domain.Seller{
				{AccountID: 2003, StoreID: 2003, StoreName: "C"},
				{AccountID: 2001, StoreID: 2001, StoreName: "A"},
				{AccountID: 2002, StoreID: 2002, StoreName: "B"},
			}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(2001, 2002, 2003))
	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	// Should follow revenue order: A(90k) → B(50k) → C(10k)
	expected := []string{"A", "B", "C"}
	for i, name := range expected {
		if result[i].Name != name {
			t.Errorf("result[%d].Name = %q, want %q", i, result[i].Name, name)
		}
	}
}

func TestGetRecommendedSellers_LimitCapped(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			// The cap applies to the output size first, the over-fetch multiplies
			// afterwards — so a request for 100 asks the peer for 50x3, not 100x3.
			if want := domain.MaxLimit * domain.RevenueCandidateFactor; limit != want {
				t.Errorf("limit = %d, want %d (capped from 100, then over-fetched)", limit, want)
			}
			return []SellerRevenue{}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			// The fallback tops up the OUTPUT, so it is bounded by the capped
			// limit — the over-fetch must not leak into it.
			if limit != domain.MaxLimit {
				t.Errorf("fallback limit = %d, want %d (capped from 100, not over-fetched)", limit, domain.MaxLimit)
			}
			return []*domain.Seller{}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores())
	_, err := uc.GetRecommendedSellers(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetRecommendedSellers_OrderServiceError(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, nil, &mockBuyerClient{}, productStores())
	_, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetRecommendedSellers_FallbackFillsWhenRevenuesInsufficient(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{
				{StoreID: 1001, TotalRevenue: 50000, OrderCount: 10},
				{StoreID: 1002, TotalRevenue: 30000, OrderCount: 5},
			}, nil
		},
	}
	var fallbackExcludeIDs []int64
	var fallbackLimit int32
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			return []*domain.Seller{
				{AccountID: 1001, StoreID: 1001, StoreName: "Revenue A"},
				{AccountID: 1002, StoreID: 1002, StoreName: "Revenue B"},
			}, nil
		},
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			fallbackExcludeIDs = excludeIDs
			fallbackLimit = limit
			return []*domain.Seller{
				{AccountID: 2001, StoreID: 2001, StoreName: "Fallback A"},
				{AccountID: 2002, StoreID: 2002, StoreName: "Fallback B"},
				{AccountID: 2003, StoreID: 2003, StoreName: "Fallback C"},
			}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(1001, 1002, 2001, 2002, 2003))
	result, err := uc.GetRecommendedSellers(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Fatalf("expected 5 results (2 revenue + 3 fallback), got %d", len(result))
	}
	// Revenue sellers first, in revenue order
	expectedOrder := []string{"Revenue A", "Revenue B", "Fallback A", "Fallback B", "Fallback C"}
	for i, want := range expectedOrder {
		if result[i].Name != want {
			t.Errorf("result[%d].Name = %q, want %q", i, result[i].Name, want)
		}
	}
	// Fallback was called with revenue seller IDs excluded and remaining limit
	if fallbackLimit != 3 {
		t.Errorf("fallback limit = %d, want 3 (5 - 2 revenue)", fallbackLimit)
	}
	if len(fallbackExcludeIDs) != 2 {
		t.Fatalf("fallback excludeIDs len = %d, want 2", len(fallbackExcludeIDs))
	}
	seen := map[int64]bool{}
	for _, id := range fallbackExcludeIDs {
		seen[id] = true
	}
	if !seen[1001] || !seen[1002] {
		t.Errorf("fallback excludeIDs = %v, want to contain 1001 and 1002", fallbackExcludeIDs)
	}
}

func TestGetRecommendedSellers_FallbackFiresWhenRevenueEmpty(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			if limit != 10 {
				t.Errorf("fallback limit = %d, want 10 (full request)", limit)
			}
			if len(excludeIDs) != 0 {
				t.Errorf("fallback excludeIDs len = %d, want 0", len(excludeIDs))
			}
			return []*domain.Seller{
				{AccountID: 2001, StoreName: "X"},
				{AccountID: 2002, StoreName: "Y"},
				{AccountID: 2003, StoreName: "Z"},
			}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(2001, 2002, 2003))
	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 results from fallback, got %d", len(result))
	}
}

func TestGetRecommendedSellers_NoFallbackWhenFull(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{
				{StoreID: 1001, TotalRevenue: 50000, OrderCount: 10},
				{StoreID: 1002, TotalRevenue: 30000, OrderCount: 5},
			}, nil
		},
	}
	fallbackCalled := false
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			return []*domain.Seller{
				{AccountID: 1001, StoreID: 1001, StoreName: "A"},
				{AccountID: 1002, StoreID: 1002, StoreName: "B"},
			}, nil
		},
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			fallbackCalled = true
			return nil, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(1001, 1002))
	result, err := uc.GetRecommendedSellers(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	if fallbackCalled {
		t.Error("fallback should not be called when revenues already fill limit")
	}
}

func TestGetRecommendedSellers_FallbackErrorDoesNotFailRequest(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{
				{StoreID: 1001, TotalRevenue: 50000, OrderCount: 10},
			}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			return []*domain.Seller{
				{AccountID: 1001, StoreID: 1001, StoreName: "A"},
			}, nil
		},
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			return nil, fmt.Errorf("fallback db error")
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(1001))
	result, err := uc.GetRecommendedSellers(context.Background(), 5)
	if err != nil {
		t.Fatalf("fallback error should be best-effort, got error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 result (revenue seller only, fallback failed gracefully), got %d", len(result))
	}
	if result[0].Name != "A" {
		t.Errorf("result[0].Name = %q, want A", result[0].Name)
	}
}

func TestGetRecommendedSellers_KeyedByStoreID_NotAccountID(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{
				{StoreID: 8001, TotalRevenue: 20000, OrderCount: 20},
				{StoreID: 42, TotalRevenue: 999999, OrderCount: 999},
			}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			return []*domain.Seller{
				{AccountID: 42, StoreID: 8001, StoreName: "Current Store"},
			}, nil
		},
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			if len(excludeIDs) != 1 || excludeIDs[0] != 8001 {
				t.Errorf("GetApprovedExcluding excludeIDs = %v, want [8001] (excluded by store_id surrogate, not login account_id 42)", excludeIDs)
			}
			return []*domain.Seller{}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(8001, 42))
	result, err := uc.GetRecommendedSellers(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected exactly 1 result (only the live store's own revenue resolves), got %d: %+v", len(result), result)
	}
	if result[0].Name != "Current Store" {
		t.Errorf("result[0].Name = %q, want %q", result[0].Name, "Current Store")
	}
	if result[0].StoreID != 8001 {
		t.Errorf("result[0].StoreID (surrogate) = %d, want 8001", result[0].StoreID)
	}
	if result[0].OrderCount != 20 {
		t.Errorf("result[0].OrderCount = %d, want 20 (the live store's own count) — 999 would mean the closed stint's revenue bled in", result[0].OrderCount)
	}
}

func TestGetRecommendedSellers_RepoError(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{{StoreID: 1001}}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			return nil, fmt.Errorf("db connection error")
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(1001))
	_, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestGetRecommendedSellers_CarriesStoreCodeOnBothPaths pins the seller display
// code through this use case (web#365). The storefront rails link to a seller by
// store_code and fall back to the raw store id whenever it is missing, so a row
// that loses the code silently costs a 308 redirect on every click. Both result
// paths are asserted deliberately: the revenue-ranked rows and the fallback
// top-up rows are two independent literals, so covering one proves nothing about
// the other.
func TestGetRecommendedSellers_CarriesStoreCodeOnBothPaths(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(ctx context.Context, days, limit int32) ([]SellerRevenue, error) {
			return []SellerRevenue{{StoreID: 1001, TotalRevenue: 50000, OrderCount: 10}}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
			return []*domain.Seller{
				{AccountID: 1001, StoreID: 1001, StoreCode: 2612345678, StoreName: "Revenue A"},
			}, nil
		},
		GetApprovedExcludingFn: func(ctx context.Context, excludeIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			return []*domain.Seller{
				{AccountID: 2001, StoreID: 2001, StoreCode: 2687654321, StoreName: "Fallback A"},
			}, nil
		},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, productStores(1001, 2001))
	result, err := uc.GetRecommendedSellers(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results (1 revenue + 1 fallback), got %d", len(result))
	}
	if result[0].StoreCode != 2612345678 {
		t.Errorf("revenue row StoreCode = %d, want 2612345678", result[0].StoreCode)
	}
	if result[1].StoreCode != 2687654321 {
		t.Errorf("fallback row StoreCode = %d, want 2687654321", result[1].StoreCode)
	}
}
