package usecase

import "context"

// BuyerClient defines the interface for buyer-service operations.
type BuyerClient interface {
	GetSubscriberCount(ctx context.Context, storeID int64) (int64, error)
	BatchGetSubscriberCounts(ctx context.Context, sellerIDs []int64) (map[int64]int64, error)
	// HasActiveBuyer reports whether the account still has an active current buyer stint.
	HasActiveBuyer(ctx context.Context, accountID int64) (bool, error)
}
