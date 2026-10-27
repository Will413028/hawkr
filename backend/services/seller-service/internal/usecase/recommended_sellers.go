package usecase

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
	"go.uber.org/zap"
)

type RecommendedSellersUseCase struct {
	orderClient   OrderClient
	sellerRepo    domain.SellerRepository
	buyerClient   BuyerClient
	productClient ProductClient
}

func NewRecommendedSellersUseCase(orderClient OrderClient, sellerRepo domain.SellerRepository, buyerClient BuyerClient, productClient ProductClient) *RecommendedSellersUseCase {
	return &RecommendedSellersUseCase{orderClient: orderClient, sellerRepo: sellerRepo, buyerClient: buyerClient, productClient: productClient}
}

// GetRecommendedSellers builds the public recommendation carousel.
//
// Fan-out shape (this endpoint is a cache-miss hot spot on the storefront, so the
// number of *serialized* hops is what its latency is made of):
//
//	stage 0  product-service gRPC — which stores have a public product (concurrent
//	                                with stage 1: it takes no input from any other
//	                                stage, so it costs zero extra serialized hops)
//	stage 1  order-service gRPC   — revenue ranking (nothing can precede it: every
//	                                downstream lookup is keyed by the store ids it returns)
//	stage 2  local Postgres       — resolve those ids to publicly-showable sellers
//	stage 3  local Postgres       — fallback top-up, only when stage 2 under-fills
//	stage 4  buyer-service gRPC   — subscriber counts for every row that made it in
//
// Stages 1→2→3 are a hard data dependency chain (each input is the previous
// output), so there is no pair to run concurrently. The one degree of freedom was
// stage 4: it used to be issued twice — once for the revenue rows and, when the
// fallback fired, a second time for the fallback rows, strictly after stage 3.
// Both id sets are known before any subscriber count is needed, so they are now
// collected into a single batch call, taking the endpoint from three serialized
// cross-service round trips down to two (the dependency-graph minimum). Stage 0
// runs alongside stage 1 for the same reason, keeping that minimum at two.
//
// Why stage 0 exists at all: neither of the two ways into this list asks
// product-service anything (revenue ranking is `ORDER BY total_revenue` over
// orders; the fallback is `ORDER BY store_id DESC`), so a seller with zero listed
// products used to rank — and on dev 7 of the top 10, including the first card,
// were empty storefronts. Ranking was never the bug; the missing filter was.
//
// Which is also why stage 1 asks for domain.RevenueCandidateFactor x limit rows
// and stage 2 truncates: a slot freed by the filter has to be refilled from the
// revenue ranking, not by stage 3. Filtering a list that was already cut to
// `limit` would leave the shortfall to the fallback, and the fallback's ordering
// is "newest approved seller" — so the endpoint would quietly stop being a
// revenue ranking for exactly as many cards as it filtered out.
//
// product-service being unreachable degrades to the pre-filter behaviour
// (everything shown) rather than to an empty rail — see domain.PublicProductStores.
func (u *RecommendedSellersUseCase) GetRecommendedSellers(ctx context.Context, limit int32) ([]dto.RecommendedSeller, error) {
	if limit <= 0 {
		limit = domain.DefaultLimit
	}
	if limit > domain.MaxLimit {
		limit = domain.MaxLimit
	}

	// Stage 0 + Stage 1 concurrently: the "has public products" set is keyed on
	// nothing the revenue ranking produces, so serializing them would add a
	// cross-service round trip to the critical path for no reason.
	//
	// syncutil.Group rather than a bare goroutine: a panic in a request-scoped
	// goroutine escapes the gRPC recovery interceptor and takes the whole pod
	// down, which would be an absurd blast radius for a best-effort stage. Here
	// it degrades to the same fail-open path as any other error.
	var withProduct domain.PublicProductStores
	g := syncutil.NewGroup(ctx)
	g.Go(func() error {
		storeIDs, perr := u.productClient.ListStoreIDsWithPublicProducts(ctx)
		if perr != nil {
			return perr
		}
		withProduct = domain.NewPublicProductStores(storeIDs)
		return nil
	})

	// Over-fetch: `limit` is how many cards come out, not how many candidates go
	// in. See domain.RevenueCandidateFactor for the sizing and the peer's own cap.
	revenues, revenueErr := u.orderClient.GetTopSellersByRevenue(ctx, domain.RevenueRankingDays, limit*domain.RevenueCandidateFactor)

	if perr := g.Wait(); perr != nil {
		// Fail open: an unreachable product-service must not empty the rail.
		// withProduct is left at its zero value (unknown ⇒ no filtering).
		logger.Warn(ctx, "ListStoreIDsWithPublicProducts failed, recommendation will not filter empty stores", zap.Error(perr))
	}
	if revenueErr != nil {
		return nil, revenueErr
	}

	// Stage 2. Build revenue-based result (only approved sellers that actually have
	// something on the shelf, preserving revenue order)
	result := make([]dto.RecommendedSeller, 0, limit)
	includedIDs := make([]int64, 0, limit)
	// subscriberIDs accumulates every store id that needs a follower count, across
	// both the revenue rows and the fallback rows, so stage 4 is a single round trip.
	subscriberIDs := make([]int64, 0, limit)
	if len(revenues) > 0 {
		storeIDs := make([]int64, 0, len(revenues))
		for _, r := range revenues {
			// Dropping empty stores before the query (rather than after) also keeps
			// them out of includedIDs, so stage 3 is free to use those slots.
			if !withProduct.Has(r.StoreID) {
				continue
			}
			storeIDs = append(storeIDs, r.StoreID)
		}

		// Widened by the over-fetch (up to MaxLimit x RevenueCandidateFactor ids).
		// It stays one batched `store_id = ANY($1)` lookup either way; nothing here
		// is per-id.
		sellers, err := u.sellerRepo.GetByStoreIDs(ctx, storeIDs)
		if err != nil {
			logger.Warn(ctx, "batch get sellers failed", zap.Error(err))
			return nil, err
		}

		sellerMap := make(map[int64]*domain.Seller, len(sellers))
		for _, s := range sellers {
			sellerMap[s.StoreID] = s
		}

		for _, rev := range revenues {
			// Truncate here — after BOTH filters (has a public product, resolves to
			// a publicly-showable seller), never before. Cutting the candidate list
			// earlier is what would hand the freed slots to stage 3, which is the
			// ordering change the over-fetch exists to prevent.
			if int32(len(result)) >= limit {
				break
			}
			s, ok := sellerMap[rev.StoreID]
			if !ok {
				continue
			}
			// Only rows that survived truncation need a follower count; enriching
			// the over-fetched remainder would inflate stage 4's batch by up to
			// RevenueCandidateFactor x for nothing.
			subscriberIDs = append(subscriberIDs, s.StoreID)
			result = append(result, dto.RecommendedSeller{
				StoreID:     s.StoreID,
				StoreCode:   s.StoreCode,
				Name:        s.StoreName,
				Logo:        s.Logo,
				CoverKey:    s.CoverKey,
				Description: s.Description,
				City:        s.City,
				District:    s.District,
				Rating:      domain.RoundRating(s.Rating),
				OrderCount:  rev.OrderCount,
				// Followers is filled in stage 4, after the fallback ids are known.
			})
			includedIDs = append(includedIDs, s.StoreID)
		}
	}

	// Stage 3. Fallback: when revenue ranking yields fewer than the limit (sparse data in
	// test env, or new platform with few recent orders), top up with newest approved
	// sellers so the carousel never looks empty. The same "has public products" predicate
	// is pushed into that query — topping up with empty storefronts is the very thing
	// stage 2's filter is there to prevent, and doing it in SQL keeps LIMIT meaningful.
	// Fallback failures are best-effort — we keep the revenue-based result rather than
	// erroring the whole request (and still enrich it in stage 4).
	if int32(len(result)) < limit {
		remaining := limit - int32(len(result))
		fallbackSellers, ferr := u.sellerRepo.GetApprovedExcluding(ctx, includedIDs, withProduct, remaining)
		if ferr != nil {
			logger.Warn(ctx, "GetApprovedExcluding fallback failed, returning revenue-only result", zap.Error(ferr))
		} else {
			for _, s := range fallbackSellers {
				subscriberIDs = append(subscriberIDs, s.StoreID)
				result = append(result, dto.RecommendedSeller{
					StoreID:     s.StoreID,
					StoreCode:   s.StoreCode,
					Name:        s.StoreName,
					Logo:        s.Logo,
					CoverKey:    s.CoverKey,
					Description: s.Description,
					City:        s.City,
					District:    s.District,
					Rating:      domain.RoundRating(s.Rating),
					OrderCount:  0,
					// Followers is filled in stage 4.
				})
			}
		}
	}

	// Stage 4. One subscriber-count round trip for revenue + fallback rows. Best-effort:
	// buyer-service being unreachable degrades follower counts to 0, it never fails the
	// carousel (same contract as the two calls this replaced).
	if len(subscriberIDs) > 0 {
		followerMap, ferr := u.buyerClient.BatchGetSubscriberCounts(ctx, subscriberIDs)
		if ferr != nil {
			logger.Warn(ctx, "BatchGetSubscriberCounts failed, fallback to 0", zap.Error(ferr))
		} else {
			for i := range result {
				// StoreID carries the store surrogate (see the assignments above),
				// which is the key BatchGetSubscriberCounts was queried with.
				result[i].Followers = int32(followerMap[result[i].StoreID])
			}
		}
	}

	return result, nil
}
