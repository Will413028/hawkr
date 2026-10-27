package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

var errFallbackDB = errors.New("fallback db error")

// latencyBuyerClient simulates buyer-service with a fixed per-call RTT and counts
// how many BatchGetSubscriberCounts round trips the usecase actually performs.
// The call count is the load-bearing assertion (deterministic); the sleep only
// exists so the benchmark reflects the cross-service round trips on the critical
// path rather than pure CPU time.
type latencyBuyerClient struct {
	rtt        time.Duration
	batchCalls atomic.Int32
	mu         sync.Mutex
	batchIDs   [][]int64
}

func (c *latencyBuyerClient) GetSubscriberCount(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (c *latencyBuyerClient) BatchGetSubscriberCounts(_ context.Context, sellerIDs []int64) (map[int64]int64, error) {
	c.batchCalls.Add(1)
	c.mu.Lock()
	ids := make([]int64, len(sellerIDs))
	copy(ids, sellerIDs)
	c.batchIDs = append(c.batchIDs, ids)
	c.mu.Unlock()
	time.Sleep(c.rtt)
	out := make(map[int64]int64, len(sellerIDs))
	for i, id := range sellerIDs {
		out[id] = int64(i + 1)
	}
	return out, nil
}

func (c *latencyBuyerClient) HasActiveBuyer(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

// newLatencyFixture builds a usecase whose dependencies sleep for realistic
// per-hop costs: cross-service gRPC (order / buyer) dominate, the service's own
// Postgres queries are sub-millisecond (measured on dev: 0.061–0.486 ms).
//
// revenueSellers controls how many of the requested `limit` the revenue ranking
// can fill; the remainder exercises the fallback top-up path.
func newLatencyFixture(grpcRTT, dbCost time.Duration, revenueSellers, fallbackSellers int) (*RecommendedSellersUseCase, *latencyBuyerClient) {
	revenues := make([]SellerRevenue, revenueSellers)
	for i := range revenues {
		revenues[i] = SellerRevenue{StoreID: int64(1000 + i), TotalRevenue: int64(10000 - i), OrderCount: int32(10 - i)}
	}

	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(_ context.Context, _, _ int32) ([]SellerRevenue, error) {
			time.Sleep(grpcRTT)
			return revenues, nil
		},
	}

	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(_ context.Context, ids []int64) ([]*domain.Seller, error) {
			time.Sleep(dbCost)
			out := make([]*domain.Seller, 0, len(ids))
			for _, id := range ids {
				out = append(out, &domain.Seller{AccountID: id, StoreID: id, StoreName: "Revenue Store"})
			}
			return out, nil
		},
		GetApprovedExcludingFn: func(_ context.Context, _ []int64, _ domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			time.Sleep(dbCost)
			n := fallbackSellers
			if int32(n) > limit {
				n = int(limit)
			}
			out := make([]*domain.Seller, 0, n)
			for i := 0; i < n; i++ {
				id := int64(9000 + i)
				out = append(out, &domain.Seller{AccountID: id, StoreID: id, StoreName: "Fallback Store"})
			}
			return out, nil
		},
	}

	buyerClient := &latencyBuyerClient{rtt: grpcRTT}

	// The "has public products" lookup pays the same cross-service RTT as the
	// revenue ranking. It is deliberately given a real cost here: if it were ever
	// serialized ahead of stage 1, the measured wall time would grow by a full
	// grpcRTT, which is the whole point of these numbers.
	productIDs := make([]int64, 0, revenueSellers+fallbackSellers)
	for i := 0; i < revenueSellers; i++ {
		productIDs = append(productIDs, int64(1000+i))
	}
	for i := 0; i < fallbackSellers; i++ {
		productIDs = append(productIDs, int64(9000+i))
	}
	productClient := &latencyProductClient{rtt: grpcRTT, storeIDs: productIDs}

	return NewRecommendedSellersUseCase(orderClient, sellerRepo, buyerClient, productClient), buyerClient
}

// latencyProductClient simulates product-service with a fixed per-call RTT.
type latencyProductClient struct {
	rtt      time.Duration
	storeIDs []int64
}

func (c *latencyProductClient) GetProductsByIDs(_ context.Context, _ []int64) ([]*ProductInfo, error) {
	return nil, nil
}

func (c *latencyProductClient) ListStoreIDsWithPublicProducts(_ context.Context) ([]int64, error) {
	time.Sleep(c.rtt)
	return c.storeIDs, nil
}

// BenchmarkRecommendedSellers_FallbackPath measures the end-to-end usecase wall
// time on the worst case (revenue ranking under-fills → fallback top-up fires),
// with 120 ms simulated cross-service gRPC RTT and 1 ms local DB per query.
func BenchmarkRecommendedSellers_FallbackPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		uc, _ := newLatencyFixture(120*time.Millisecond, time.Millisecond, 3, 5)
		if _, err := uc.GetRecommendedSellers(context.Background(), 8); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkRecommendedSellers_NoFallbackPath measures the happy path where the
// revenue ranking already fills the limit (no fallback query at all).
func BenchmarkRecommendedSellers_NoFallbackPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		uc, _ := newLatencyFixture(120*time.Millisecond, time.Millisecond, 8, 0)
		if _, err := uc.GetRecommendedSellers(context.Background(), 8); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// TestGetRecommendedSellers_SingleSubscriberRoundTrip locks the fan-out shape:
// even when the fallback top-up fires, the usecase must ask buyer-service for
// subscriber counts exactly once, with the revenue rows' and the fallback rows'
// store ids in the same batch. Two serialized calls (the previous shape) put an
// extra cross-service round trip on this cache-miss hot path.
func TestGetRecommendedSellers_SingleSubscriberRoundTrip(t *testing.T) {
	uc, buyer := newLatencyFixture(0, 0, 3, 5)
	result, err := uc.GetRecommendedSellers(context.Background(), 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 8 {
		t.Fatalf("expected 8 results (3 revenue + 5 fallback), got %d", len(result))
	}
	if got := buyer.batchCalls.Load(); got != 1 {
		t.Fatalf("BatchGetSubscriberCounts called %d times, want exactly 1", got)
	}

	batched := map[int64]bool{}
	for _, id := range buyer.batchIDs[0] {
		batched[id] = true
	}
	for _, row := range result {
		if !batched[row.StoreID] {
			t.Errorf("store %d landed in the result but was not in the subscriber-count batch %v", row.StoreID, buyer.batchIDs[0])
		}
		// latencyBuyerClient returns index+1 as the count, so every requested id
		// must come back non-zero — catches a batch/result key mismatch.
		if row.Followers == 0 {
			t.Errorf("store %d got Followers=0, want the count from the batched response", row.StoreID)
		}
	}
}

// TestGetRecommendedSellers_FallbackErrorStillEnrichesRevenueRows guards the
// stage reordering: the subscriber-count call now happens after the fallback
// query, so a fallback DB failure must not swallow the revenue rows' follower
// counts (before the merge, those were already fetched by the time the fallback
// ran, and the request degraded to "revenue-only result, followers intact").
func TestGetRecommendedSellers_FallbackErrorStillEnrichesRevenueRows(t *testing.T) {
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(_ context.Context, _, _ int32) ([]SellerRevenue, error) {
			return []SellerRevenue{{StoreID: 1001, TotalRevenue: 500, OrderCount: 3}}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(_ context.Context, _ []int64) ([]*domain.Seller, error) {
			return []*domain.Seller{{AccountID: 1001, StoreID: 1001, StoreName: "A"}}, nil
		},
		GetApprovedExcludingFn: func(_ context.Context, _ []int64, _ domain.PublicProductStores, _ int32) ([]*domain.Seller, error) {
			return nil, errFallbackDB
		},
	}
	buyer := &latencyBuyerClient{}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, buyer, productStores(1001))
	result, err := uc.GetRecommendedSellers(context.Background(), 5)
	if err != nil {
		t.Fatalf("fallback error must stay best-effort, got: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 revenue-only result, got %d", len(result))
	}
	if buyer.batchCalls.Load() != 1 {
		t.Fatalf("expected the subscriber-count call to still happen, got %d calls", buyer.batchCalls.Load())
	}
	if result[0].Followers != 1 {
		t.Errorf("Followers = %d, want 1 (revenue row must still be enriched despite the fallback failure)", result[0].Followers)
	}
}

// TestGetRecommendedSellers_LatencyProfile prints the measured wall time per
// stage shape. It asserts nothing about timing (CI-safe) — the numbers are
// recorded in the delivery report; the assertions here are structural.
func TestGetRecommendedSellers_LatencyProfile(t *testing.T) {
	const grpcRTT = 120 * time.Millisecond
	const dbCost = time.Millisecond

	t.Run("fallback path", func(t *testing.T) {
		uc, buyer := newLatencyFixture(grpcRTT, dbCost, 3, 5)
		start := time.Now()
		result, err := uc.GetRecommendedSellers(context.Background(), 8)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 8 {
			t.Fatalf("expected 8 results (3 revenue + 5 fallback), got %d", len(result))
		}
		t.Logf("fallback path: elapsed=%v, buyer batch calls=%d", elapsed, buyer.batchCalls.Load())
	})

	t.Run("no fallback path", func(t *testing.T) {
		uc, buyer := newLatencyFixture(grpcRTT, dbCost, 8, 0)
		start := time.Now()
		result, err := uc.GetRecommendedSellers(context.Background(), 8)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 8 {
			t.Fatalf("expected 8 results, got %d", len(result))
		}
		t.Logf("no fallback path: elapsed=%v, buyer batch calls=%d", elapsed, buyer.batchCalls.Load())
	})
}

// TestGetRecommendedSellers_ProductLookupRunsConcurrentlyWithRevenue pins that
// the "has public products" lookup does NOT add a serialized cross-service hop.
//
// It is a rendezvous rather than a timing assertion, so it is deterministic on a
// loaded CI box: each of the two clients announces its arrival and then waits for
// the other. If they run concurrently both proceed immediately; if one is
// serialized ahead of the other, whichever runs first waits for a partner that
// cannot start until it returns, and the test fails on the timeout with a message
// naming the culprit. (A wall-clock assertion would have to pick a threshold,
// and any threshold is either flaky or blind.)
func TestGetRecommendedSellers_ProductLookupRunsConcurrentlyWithRevenue(t *testing.T) {
	const rendezvousTimeout = 2 * time.Second

	productArrived := make(chan struct{})
	revenueArrived := make(chan struct{})
	errSerialized := errors.New("serialized: the other stage had not started yet")

	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(_ context.Context, _, _ int32) ([]SellerRevenue, error) {
			close(revenueArrived)
			select {
			case <-productArrived:
			case <-time.After(rendezvousTimeout):
				return nil, fmt.Errorf("revenue ranking: %w", errSerialized)
			}
			return []SellerRevenue{{StoreID: 1001, TotalRevenue: 500, OrderCount: 3}}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(_ context.Context, _ []int64) ([]*domain.Seller, error) {
			return []*domain.Seller{{AccountID: 1001, StoreID: 1001, StoreName: "A"}}, nil
		},
		GetApprovedExcludingFn: func(_ context.Context, _ []int64, _ domain.PublicProductStores, _ int32) ([]*domain.Seller, error) {
			return nil, nil
		},
	}
	product := &rendezvousProductClient{
		arrived: productArrived,
		waitFor: revenueArrived,
		timeout: rendezvousTimeout,
		err:     errSerialized,
		ids:     []int64{1001},
	}

	uc := NewRecommendedSellersUseCase(orderClient, sellerRepo, &latencyBuyerClient{}, product)
	result, err := uc.GetRecommendedSellers(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.serialized.Load() {
		t.Fatal("product lookup waited for the revenue ranking that had not started — the two stages are serialized, adding a cross-service round trip to a cache-miss hot path")
	}
	if len(result) != 1 {
		t.Fatalf("expected the fixture's single row, got %d", len(result))
	}
}

// rendezvousProductClient participates in the concurrency rendezvous above. It
// records rather than returns the "serialized" verdict, because the production
// path swallows this client's error (fail open) — returning it would be invisible.
type rendezvousProductClient struct {
	arrived    chan struct{}
	waitFor    chan struct{}
	timeout    time.Duration
	err        error
	ids        []int64
	serialized atomic.Bool
}

func (c *rendezvousProductClient) GetProductsByIDs(_ context.Context, _ []int64) ([]*ProductInfo, error) {
	return nil, nil
}

func (c *rendezvousProductClient) ListStoreIDsWithPublicProducts(_ context.Context) ([]int64, error) {
	close(c.arrived)
	select {
	case <-c.waitFor:
		return c.ids, nil
	case <-time.After(c.timeout):
		c.serialized.Store(true)
		return nil, c.err
	}
}
