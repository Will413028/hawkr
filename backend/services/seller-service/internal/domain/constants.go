package domain

const (
	DefaultPageSize    int32 = 20
	MaxPageSize        int32 = 500
	DefaultLimit       int32 = 10
	MaxLimit           int32 = 50
	RevenueRankingDays int32 = 30

	// RevenueCandidateFactor over-fetches the revenue ranking so that a slot
	// freed by the "has a public product" filter is refilled from the revenue
	// ranking itself rather than handed to the fallback top-up.
	//
	// Without it the filter silently rewrites what the rail means: the fallback
	// orders by `store_id DESC` (newest approved seller), so every card dropped
	// for being an empty storefront would come back as "newest seller" instead
	// of "next highest earner". On dev 7 of the top 10 earners are empty, i.e.
	// 7 of 10 cards would stop being revenue-ranked at all — a change nobody
	// asked for and nothing would report.
	//
	// 3 is sized against that measured 70% miss rate and costs essentially
	// nothing: order-service answers with `GROUP BY store_id ... LIMIT n`, so a
	// wider n only takes more rows off an aggregate it already computed.
	//
	// Reach: order-service clamps this RPC at 50 rows (OrderService
	// .getTopSellersByRevenue), so the over-fetch is fully effective up to
	// limit=16 and tapers above it, reaching parity with the old behaviour at
	// limit=50. Every real caller asks for 10 (web SSR and the buyer App's
	// /public/marketplace both pass seller_limit=10). Asking for more than the
	// peer will return is deliberate: it is clamped, never an error, and the
	// over-fetch widens on its own if that cap is ever raised.
	RevenueCandidateFactor int32 = 3
)
