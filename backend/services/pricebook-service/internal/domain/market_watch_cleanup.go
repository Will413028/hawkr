package domain

import "context"

// MarketWatchCleanupRepository removes all watch identities for a deleted seller.
// The identifier is account_id, never the seller's store_id.
type MarketWatchCleanupRepository interface {
	DeleteByAccountID(context.Context, int64) error
}
