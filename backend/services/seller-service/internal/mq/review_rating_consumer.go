package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/logger"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/zap"
)

func reviewRatingHandler(sellerRepo domain.SellerRepository) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var event pkgmq.ReviewCreatedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			logger.Warn(ctx, "review_rating: malformed event", zap.Error(err))
			return fmt.Errorf("unmarshal review created event: %w", err)
		}

		storeID := event.StoreID
		if storeID == 0 {
			logger.Warn(ctx, "review_rating: missing StoreID")
			return fmt.Errorf("review created event missing StoreID")
		}

		if err := sellerRepo.UpdateRatingByStoreID(ctx, storeID, event.AvgRating); err != nil {
			logger.Warn(ctx, "review_rating: failed to update rating, will requeue",
				zap.Int64("store_id", event.StoreID),
				zap.Float64("avg_rating", event.AvgRating),
				zap.Error(err))
			return err
		}

		logger.Info(ctx, "review_rating: seller rating updated",
			zap.Int64("store_id", event.StoreID),
			zap.Float64("avg_rating", event.AvgRating),
			zap.Int64("total_count", event.TotalCount))
		return nil
	}
}
