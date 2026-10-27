package usecase

import (
	"context"

	"github.com/Will413028/hawkr/review-service/internal/domain"
)

type ReviewEventPublisher interface {
	PublishReviewCreated(ctx context.Context, review *domain.Review, notifySellerAccountID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64, buyerNickname, buyerAvatar string) error
	PublishReviewHidden(ctx context.Context, reviewID, storeID, productID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64) error
}
