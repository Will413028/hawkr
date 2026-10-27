package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// The recommendation carousel used to have no product-side condition anywhere:
// the revenue ranking is `ORDER BY total_revenue` over orders and the fallback
// top-up is `ORDER BY store_id DESC`, so a seller with zero listed products could
// (and on dev did) take the first card. Every test in this file guards one half
// of the fix — filter the revenue rows, filter the fallback rows — plus the
// fail-open contract that keeps the rail alive when product-service is not.

// storeIDsOf is the shared "what actually came out" projection, so each test
// asserts on the set rather than on positional indexes.
func storeIDsOf(rows []dto.RecommendedSeller) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.StoreID)
	}
	return ids
}

// revenueFixture wires a usecase whose revenue ranking returns `revenueIDs` and
// whose seller table happily resolves every one of them. Only the product filter
// can remove anything, so a row surviving to the result proves the filter let it
// through — nothing else in the fixture can.
func revenueFixture(t *testing.T, revenueIDs []int64, fallbackIDs []int64, product ProductClient) (*RecommendedSellersUseCase, *domain.PublicProductStores) {
	t.Helper()

	revenues := make([]SellerRevenue, 0, len(revenueIDs))
	for i, id := range revenueIDs {
		revenues = append(revenues, SellerRevenue{StoreID: id, TotalRevenue: int64(100000 - i), OrderCount: int32(10 - i)})
	}

	var seenFilter domain.PublicProductStores
	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(_ context.Context, _, _ int32) ([]SellerRevenue, error) {
			return revenues, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(_ context.Context, ids []int64) ([]*domain.Seller, error) {
			out := make([]*domain.Seller, 0, len(ids))
			for _, id := range ids {
				out = append(out, &domain.Seller{AccountID: id, StoreID: id, StoreName: fmt.Sprintf("Store %d", id)})
			}
			return out, nil
		},
		GetApprovedExcludingFn: func(_ context.Context, _ []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			// Stand in for the SQL predicate: filter first, LIMIT second — the same
			// order the query uses, and the reason the predicate belongs in the
			// query rather than in the caller. Honouring it here is what lets the
			// end-to-end assertions catch "the usecase dropped the argument": an
			// unresolved filter admits everything, so the empty storefront reappears.
			seenFilter = withPublicProducts
			out := make([]*domain.Seller, 0, len(fallbackIDs))
			for _, id := range fallbackIDs {
				if !withPublicProducts.Has(id) {
					continue
				}
				if int32(len(out)) >= limit {
					break
				}
				out = append(out, &domain.Seller{AccountID: id, StoreID: id, StoreName: fmt.Sprintf("Fallback %d", id)})
			}
			return out, nil
		},
	}
	return NewRecommendedSellersUseCase(orderClient, sellerRepo, &mockBuyerClient{}, product), &seenFilter
}

// TestGetRecommendedSellers_ExcludesStoresWithoutPublicProducts is the headline
// guard: a top-earning seller with nothing on the shelf must not be recommended.
//
// Discriminating power: 1002 and 1001 are identical in every respect the fixture
// models (same seller row shape, adjacent revenue) — the ONLY difference is
// membership of the product set. Drop the filter and 1002 comes back.
func TestGetRecommendedSellers_ExcludesStoresWithoutPublicProducts(t *testing.T) {
	// 1002 out-earns 1003 and would take the second card; it has no products.
	uc, _ := revenueFixture(t,
		[]int64{1001, 1002, 1003},
		nil,
		productStores(1001, 1003))

	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := storeIDsOf(result)
	want := []int64{1001, 1003}
	if len(got) != len(want) {
		t.Fatalf("result store ids = %v, want %v (1002 has no public product)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("result store ids = %v, want %v (revenue order preserved, empty store dropped)", got, want)
		}
	}
}

// TestGetRecommendedSellers_EveryReturnedStoreHasPublicProducts states the
// invariant directly rather than by example, and covers BOTH entry paths in one
// assertion — a revenue row and a fallback row that are each in the set, next to
// one of each that is not.
//
// Discriminating power comes from 1002 and 2002: each is the sibling of a row
// that does survive, differing only in product-set membership. Drop the filter on
// either path and that path's empty store lands back in the result.
func TestGetRecommendedSellers_EveryReturnedStoreHasPublicProducts(t *testing.T) {
	withProducts := []int64{1001, 2001}
	uc, seenFilter := revenueFixture(t,
		[]int64{1001, 1002},
		[]int64{2001, 2002},
		productStores(withProducts...))

	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allowed := map[int64]bool{}
	for _, id := range withProducts {
		allowed[id] = true
	}
	for _, row := range result {
		if !allowed[row.StoreID] {
			t.Errorf("store %d was recommended but has no public product (result=%v)", row.StoreID, storeIDsOf(result))
		}
	}
	if len(result) == 0 {
		t.Fatal("fixture produced no rows at all — the assertion above would be vacuous")
	}

	// The fallback query must receive the predicate, not a fresh zero value:
	// filtering fallback rows in Go instead would silently under-fill the rail,
	// because LIMIT is applied before any Go-side filter could run.
	if !seenFilter.Known() {
		t.Fatal("GetApprovedExcluding was called with an unresolved filter — the fallback would top up with empty storefronts")
	}
	if !seenFilter.Has(2001) || seenFilter.Has(2002) {
		t.Errorf("fallback predicate wrong: Has(2001)=%v (want true), Has(2002)=%v (want false)",
			seenFilter.Has(2001), seenFilter.Has(2002))
	}
}

// TestGetRecommendedSellers_FallbackAlsoFiltersEmptyStores pins the half that is
// easiest to forget: the revenue path can be filtered perfectly and the carousel
// still fills up with empty storefronts through the top-up.
func TestGetRecommendedSellers_FallbackAlsoFiltersEmptyStores(t *testing.T) {
	uc, seenFilter := revenueFixture(t,
		nil, // revenue ranking empty → the whole carousel is fallback
		[]int64{2001, 2002},
		productStores(2001))

	if _, err := uc.GetRecommendedSellers(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !seenFilter.Known() {
		t.Fatal("fallback ran without the product predicate")
	}
	if seenFilter.Has(2002) {
		t.Error("fallback predicate admits 2002, which has no public product")
	}
}

// TestGetRecommendedSellers_FallbackOnlyReturnsStoresWithProducts is the
// end-to-end form of the previous test: with the revenue ranking empty, every
// card on the rail came through the top-up, so the result set alone proves
// whether the top-up was filtered.
func TestGetRecommendedSellers_FallbackOnlyReturnsStoresWithProducts(t *testing.T) {
	uc, _ := revenueFixture(t,
		nil,
		[]int64{2001, 2002, 2003},
		productStores(2001, 2003))

	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := storeIDsOf(result)
	if len(got) != 2 || got[0] != 2001 || got[1] != 2003 {
		t.Fatalf("result store ids = %v, want [2001 2003] (2002 has no public product)", got)
	}
}

// TestGetRecommendedSellers_FailsOpenWhenProductServiceDown: an unreachable
// product-service must degrade to the pre-filter behaviour (show everything),
// never to an empty rail. Treating "could not ask" as "the empty set" would take
// the storefront's first row down whenever product-service hiccups.
func TestGetRecommendedSellers_FailsOpenWhenProductServiceDown(t *testing.T) {
	uc, seenFilter := revenueFixture(t,
		[]int64{1001, 1002},
		[]int64{2001},
		productServiceDown())

	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("product-service failure must not fail the request, got: %v", err)
	}
	got := storeIDsOf(result)
	if len(got) != 3 {
		t.Fatalf("result store ids = %v, want all 3 rows (fail open: no filtering)", got)
	}
	if seenFilter.Known() {
		t.Error("fallback received a resolved filter although product-service failed — it would filter against a set that was never answered")
	}
}

// TestGetRecommendedSellers_EmptyProductSetIsNotFailOpen is the other side of the
// same coin, and the reason PublicProductStores is a type rather than a []int64:
// "nobody has a public product" is a real answer and must filter everything, so
// it cannot share a representation with "product-service did not answer".
func TestGetRecommendedSellers_EmptyProductSetIsNotFailOpen(t *testing.T) {
	uc, seenFilter := revenueFixture(t,
		[]int64{1001, 1002},
		nil,
		productStores()) // resolved, empty

	result, err := uc.GetRecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("result = %v, want empty (no store has a public product)", storeIDsOf(result))
	}
	if !seenFilter.Known() {
		t.Fatal("an empty answer was downgraded to 'unknown' — that is fail-open, and it would put empty stores back on the rail")
	}
}

// TestGetRecommendedSellers_AsksProductServiceOnce guards the fan-out: the set is
// request-scoped and must not be re-fetched per stage.
func TestGetRecommendedSellers_AsksProductServiceOnce(t *testing.T) {
	product := productStores(1001, 2001)
	uc, _ := revenueFixture(t, []int64{1001}, []int64{2001}, product)

	if _, err := uc.GetRecommendedSellers(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := product.calls.Load(); got != 1 {
		t.Errorf("ListStoreIDsWithPublicProducts called %d times, want exactly 1", got)
	}
}

// TestPublicProductStores_ZeroValueFailsOpen pins the type's default. The zero
// value is what a caller gets by forgetting to populate it, so it must be the
// harmless direction (show everything), never "hide every seller".
func TestPublicProductStores_ZeroValueFailsOpen(t *testing.T) {
	var zero domain.PublicProductStores
	if zero.Known() {
		t.Error("zero value reports Known() == true")
	}
	if !zero.Has(12345) {
		t.Error("zero value must admit every store (fail open)")
	}

	resolved := domain.NewPublicProductStores(nil)
	if !resolved.Known() {
		t.Error("NewPublicProductStores(nil) must be a resolved-empty answer, not 'unknown'")
	}
	if resolved.Has(12345) {
		t.Error("resolved-empty must admit nothing")
	}
	if resolved.IDs() == nil {
		t.Error("IDs() must be non-nil so pgx encodes '{}' rather than SQL NULL")
	}

	dedup := domain.NewPublicProductStores([]int64{7, 7, 9})
	if len(dedup.IDs()) != 2 {
		t.Errorf("IDs() = %v, want the input de-duplicated", dedup.IDs())
	}
}

// ---------------------------------------------------------------------------
// Over-fetch: a slot the filter frees belongs to the next earner, not to stage 3
// ---------------------------------------------------------------------------

// rankedFixture differs from revenueFixture in exactly one load-bearing way: its
// order client HONOURS the requested limit, returning the first n of the ranking
// the way order-service's `GROUP BY store_id ... LIMIT n` does. That is what
// makes the over-fetch observable at all — a mock that returns its whole canned
// list regardless of the limit behaves identically whether the usecase
// over-fetched or not, so it would pass either way.
//
// It reports the limit the usecase actually asked for, and whether the fallback
// top-up ran, because "the empty store's slot went to the fallback" is the exact
// regression these tests exist to catch.
type rankedFixture struct {
	uc             *RecommendedSellersUseCase
	buyer          *latencyBuyerClient
	askedForLimit  int32
	fallbackCalled bool
}

func newRankedFixture(t *testing.T, ranking, withProducts, fallbackIDs []int64) *rankedFixture {
	t.Helper()
	f := &rankedFixture{buyer: &latencyBuyerClient{}}

	orderClient := &mockOrderClient{
		GetTopSellersByRevenueFn: func(_ context.Context, _, limit int32) ([]SellerRevenue, error) {
			f.askedForLimit = limit
			n := int(limit)
			if n > len(ranking) {
				n = len(ranking)
			}
			out := make([]SellerRevenue, 0, n)
			for i, id := range ranking[:n] {
				// Strictly descending revenue, so the ranking's slice order IS the
				// revenue order and the assertions can compare against it directly.
				out = append(out, SellerRevenue{StoreID: id, TotalRevenue: int64(1000000 - i), OrderCount: int32(100 - i)})
			}
			return out, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByStoreIDsFn: func(_ context.Context, ids []int64) ([]*domain.Seller, error) {
			out := make([]*domain.Seller, 0, len(ids))
			for _, id := range ids {
				out = append(out, &domain.Seller{AccountID: id, StoreID: id, StoreName: fmt.Sprintf("Store %d", id)})
			}
			return out, nil
		},
		GetApprovedExcludingFn: func(_ context.Context, _ []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
			f.fallbackCalled = true
			out := make([]*domain.Seller, 0, len(fallbackIDs))
			for _, id := range fallbackIDs {
				if !withPublicProducts.Has(id) {
					continue
				}
				if int32(len(out)) >= limit {
					break
				}
				out = append(out, &domain.Seller{AccountID: id, StoreID: id, StoreName: fmt.Sprintf("Fallback %d", id)})
			}
			return out, nil
		},
	}

	f.uc = NewRecommendedSellersUseCase(orderClient, sellerRepo, f.buyer, productStores(withProducts...))
	return f
}

// TestGetRecommendedSellers_FilteredSlotRefilledByNextEarner is the headline
// guard for the over-fetch: dropping an empty storefront must promote the NEXT
// HIGHEST EARNER into its slot, not hand the slot to the fallback top-up.
//
// The two are trivially told apart because the fallback orders by
// `store_id DESC` (newest approved seller) — a completely different ranking. So
// if the filtered slots leak to stage 3, this endpoint silently stops being a
// revenue ranking for exactly as many cards as it filtered, which is a semantic
// change nothing else in the suite would report.
//
// Discriminating power: 9001 IS in the product set, so the fallback is perfectly
// able to supply it — the only reason it must not appear is that revenue ranks
// 1004 and 1005 ahead of it. Ask order-service for `limit` instead of
// `limit * RevenueCandidateFactor` and the ranking is truncated to
// {1001, 1002, 1003} before the filter ever runs, only 1003 survives, and 9001
// takes the second card.
func TestGetRecommendedSellers_FilteredSlotRefilledByNextEarner(t *testing.T) {
	// Revenue order: 1001 > 1002 > 1003 > 1004 > 1005. The top two are empty
	// storefronts, mirroring dev (7 of the top 10 have nothing on the shelf).
	f := newRankedFixture(t,
		[]int64{1001, 1002, 1003, 1004, 1005},
		[]int64{1003, 1004, 1005, 9001},
		[]int64{9001})

	result, err := f.uc.GetRecommendedSellers(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := storeIDsOf(result)
	want := []int64{1003, 1004, 1005}
	if len(got) != len(want) {
		t.Fatalf("result store ids = %v, want %v (ranks 4 and 5 must fill the slots freed by the two empty stores)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("result store ids = %v, want %v (revenue order, refilled from the ranking)", got, want)
		}
	}
	if f.fallbackCalled {
		t.Error("the fallback top-up ran even though the revenue ranking still had qualifying sellers — those cards are now ordered by store_id DESC, not by revenue")
	}
}

// TestGetRecommendedSellers_OverFetchesCandidatesNotResults pins the mechanism
// directly, so a regression names itself instead of showing up as a puzzling
// ordering failure in the test above.
func TestGetRecommendedSellers_OverFetchesCandidatesNotResults(t *testing.T) {
	f := newRankedFixture(t, []int64{1001}, []int64{1001}, nil)

	if _, err := f.uc.GetRecommendedSellers(context.Background(), 4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := 4 * domain.RevenueCandidateFactor; f.askedForLimit != want {
		t.Errorf("asked order-service for limit=%d, want %d (limit x domain.RevenueCandidateFactor)", f.askedForLimit, want)
	}
}

// TestGetRecommendedSellers_TruncatesToLimitPreservingRevenueOrder is the other
// half of the over-fetch: asking for more candidates must not return more cards.
// Without the truncation the rail would silently render up to
// RevenueCandidateFactor x the requested size, and the buyer App / web SSR both
// pass a limit precisely because they lay out that many slots.
//
// It also pins that the over-fetched remainder never reaches buyer-service:
// enriching candidates that were cut would inflate that batch threefold for rows
// nobody will see.
func TestGetRecommendedSellers_TruncatesToLimitPreservingRevenueOrder(t *testing.T) {
	ranking := []int64{1001, 1002, 1003, 1004, 1005, 1006}
	f := newRankedFixture(t, ranking, ranking, nil)

	result, err := f.uc.GetRecommendedSellers(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := storeIDsOf(result)
	want := []int64{1001, 1002, 1003}
	if len(got) != len(want) {
		t.Fatalf("result store ids = %v, want %v (over-fetched candidates must be truncated back to the limit)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("result store ids = %v, want %v (top-of-ranking kept, in revenue order)", got, want)
		}
	}
	if f.fallbackCalled {
		t.Error("fallback ran although the revenue rows already filled the limit")
	}

	if n := f.buyer.batchCalls.Load(); n != 1 {
		t.Fatalf("BatchGetSubscriberCounts called %d times, want exactly 1", n)
	}
	if batched := f.buyer.batchIDs[0]; len(batched) != len(want) {
		t.Errorf("subscriber-count batch = %v, want only the %d rows that survived truncation", batched, len(want))
	}
}
