package usecase

import "context"

// ReviewStats holds aggregated review statistics for a seller.
type ReviewStats struct {
	TotalCount int32
	AvgRating  float64
}

// ReviewClient defines the interface for review-service operations.
type ReviewClient interface {
	GetSellerReviewStats(ctx context.Context, storeID int64) (*ReviewStats, error)
}
