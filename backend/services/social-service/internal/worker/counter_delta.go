package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/social-service/internal/domain"
)

type counterDeltaPayload struct {
	EntityType string `json:"entityType"`
	EntityID   int64  `json:"entityId"`
	Field      string `json:"field"`
	Delta      int32  `json:"delta"`
}

// newCounterDeltaHandler applies async +1/-1 updates to the denormalized
// counters that CreateComment/DeleteComment/ToggleLike (both post and comment)
// used to update synchronously. See
// docs/superpowers/specs/2026-07-31-social-counter-async-consistency-design.md.
//
// Failures return an error so the resilient-consumer framework retries and
// eventually dead-letters (36 §1.5) instead of silently dropping — the
// existing 10-minute reconciler (counter_reconcile.go's newCounterReconcileScheduler
// / reconcile, calling PostRepository/CommentRepository's ReconcileLikeCounts /
// ReconcileCommentCounts / ReconcileReplyCounts) is the final backstop for
// anything still lost.
func newCounterDeltaHandler(postRepo domain.PostRepository, commentRepo domain.CommentRepository, logger *zap.Logger) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var payload counterDeltaPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			logger.Error("failed to unmarshal counter.delta event", zap.Error(err))
			return fmt.Errorf("unmarshal counter.delta event: %w", err)
		}

		if payload.EntityID <= 0 {
			logger.Error("counter.delta event has invalid entityId",
				zap.String("entityType", payload.EntityType), zap.Int64("entityId", payload.EntityID),
				zap.String("field", payload.Field))
			return fmt.Errorf("counter.delta event has invalid entityId=%d", payload.EntityID)
		}

		var applyErr error
		switch {
		case payload.EntityType == "post" && payload.Field == "comment_count" && payload.Delta > 0:
			applyErr = postRepo.IncrCommentCount(ctx, payload.EntityID)
		case payload.EntityType == "post" && payload.Field == "comment_count" && payload.Delta < 0:
			applyErr = postRepo.DecrCommentCount(ctx, payload.EntityID)
		case payload.EntityType == "post" && payload.Field == "like_count" && payload.Delta > 0:
			applyErr = postRepo.IncrLikeCount(ctx, payload.EntityID)
		case payload.EntityType == "post" && payload.Field == "like_count" && payload.Delta < 0:
			applyErr = postRepo.DecrLikeCount(ctx, payload.EntityID)
		case payload.EntityType == "comment" && payload.Field == "reply_count" && payload.Delta > 0:
			applyErr = commentRepo.IncrReplyCount(ctx, payload.EntityID)
		case payload.EntityType == "comment" && payload.Field == "reply_count" && payload.Delta < 0:
			applyErr = commentRepo.DecrReplyCount(ctx, payload.EntityID)
		case payload.EntityType == "comment" && payload.Field == "like_count" && payload.Delta > 0:
			applyErr = commentRepo.IncrLikeCount(ctx, payload.EntityID)
		case payload.EntityType == "comment" && payload.Field == "like_count" && payload.Delta < 0:
			applyErr = commentRepo.DecrLikeCount(ctx, payload.EntityID)
		default:
			logger.Error("counter.delta event has unknown entityType/field combination",
				zap.String("entityType", payload.EntityType), zap.String("field", payload.Field), zap.Int32("delta", payload.Delta))
			return fmt.Errorf("counter.delta event has unknown entityType=%q field=%q delta=%d", payload.EntityType, payload.Field, payload.Delta)
		}

		if applyErr != nil {
			logger.Error("failed to apply counter delta",
				zap.String("entityType", payload.EntityType), zap.Int64("entityId", payload.EntityID),
				zap.String("field", payload.Field), zap.Int32("delta", payload.Delta), zap.Error(applyErr))
			return applyErr
		}
		return nil
	}
}
