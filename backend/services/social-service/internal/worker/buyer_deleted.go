package worker

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/social-service/internal/domain"
)

func newBuyerDeletedHandler(postRepo domain.PostRepository, commentRepo domain.CommentRepository, forumRepo domain.ForumRepository, logger *zap.Logger) func(ctx context.Context, routingKey string, body []byte) error {
	return mq.BindTypedHandler(logger, func(ctx context.Context, routingKey string, payload *mq.BuyerDeletedEvent) error {
		if payload.BuyerID == 0 {
			logger.Warn("buyer.deleted event missing buyerId")
			return fmt.Errorf("buyer.deleted event missing buyerId")
		}

		logger.Info("processing buyer.deleted event", zap.Int64("account_id", payload.AccountID), zap.Int64("buyer_id", payload.BuyerID))

		if err := commentRepo.SoftDeleteByActor(ctx, domain.ActorKindBuyer, payload.BuyerID); err != nil {
			logger.Error("failed to soft-delete comments for buyer", zap.Int64("buyer_id", payload.BuyerID), zap.Error(err))
			return err
		}

		if err := forumRepo.SoftDeleteByActor(ctx, domain.ActorKindBuyer, payload.BuyerID); err != nil {
			logger.Error("failed to soft-delete forum entries for buyer", zap.Int64("buyer_id", payload.BuyerID), zap.Error(err))
			return err
		}

		if err := postRepo.DeleteLikesByActor(ctx, domain.ActorKindBuyer, payload.BuyerID); err != nil {
			logger.Error("failed to delete post likes for buyer", zap.Int64("buyer_id", payload.BuyerID), zap.Error(err))
			return err
		}

		if err := commentRepo.DeleteLikesByActor(ctx, domain.ActorKindBuyer, payload.BuyerID); err != nil {
			logger.Error("failed to delete comment likes for buyer", zap.Int64("buyer_id", payload.BuyerID), zap.Error(err))
			return err
		}

		logger.Info("soft-deleted comments, forum entries and likes for deleted buyer stint", zap.Int64("buyer_id", payload.BuyerID))
		return nil
	})
}
