package mq

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/review-service/internal/domain"
)

// Publisher wraps go-pkg/mq.Publisher to implement usecase.ReviewEventPublisher.
type Publisher struct {
	pub *pkgmq.Publisher
}

// NewPublisher creates a new MQ publisher for review events.
func NewPublisher(url string) (*Publisher, error) {
	pub, err := pkgmq.NewPublisher(url)
	if err != nil {
		return nil, err
	}
	return &Publisher{pub: pub}, nil
}

// PublishReviewCreated publishes a REVIEW_CREATED event carrying the full review snapshot.
func (p *Publisher) PublishReviewCreated(ctx context.Context, review *domain.Review, notifySellerAccountID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64, buyerNickname, buyerAvatar string) error {
	event := pkgmq.NewReviewCreatedEvent(
		review.ID, review.OrderNo, review.OrderItemID, review.StoreID, notifySellerAccountID, review.BuyerID,
		review.Rating, review.Content, review.ImageKeys, timeutil.FormatISO8601(review.CreatedAt),
		avgRating, totalCount, review.ProductID, productAvgRating, productReviewCount, buyerNickname, buyerAvatar,
	)
	if err := p.pub.PublishReviewCreated(ctx, event); err != nil {
		zap.L().Warn("failed to publish review.created", zap.Error(err))
		return err
	}
	return nil
}

// PublishReviewHidden publishes a REVIEW_HIDDEN event carrying recomputed seller + product aggregates (#1156).
func (p *Publisher) PublishReviewHidden(ctx context.Context, reviewID, storeID, productID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64) error {
	event := pkgmq.NewReviewHiddenEvent(reviewID, storeID, productID, avgRating, totalCount, productAvgRating, productReviewCount)
	if err := p.pub.PublishReviewHidden(ctx, event); err != nil {
		zap.L().Warn("failed to publish review.hidden", zap.Error(err))
		return err
	}
	return nil
}

// Close closes the publisher connection.
func (p *Publisher) Close() {
	if p.pub != nil {
		_ = p.pub.Close()
	}
}
