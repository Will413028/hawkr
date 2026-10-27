package usecase

import "context"

// SellerClient talks to seller-service for seller identity resolution.
type SellerClient interface {
	// ResolvePrincipal returns the seller's login account_id for storeID, or 0
	// when seller-service cannot resolve it (RPC failure, business error, or no
	// live sellers row). 0 is a fail-closed answer, not a fallback: the only
	// consumer of this value is review.created's notify_seller_account_id, and
	// notification-service skips a notification whose recipient id is <= 0.
	// See the implementation's doc comment for why this differs from the
	// same-named method in the other services (#2789).
	ResolvePrincipal(ctx context.Context, storeID int64) int64
	ResolveStoreCode(ctx context.Context, storeID int64) (string, error)
}
