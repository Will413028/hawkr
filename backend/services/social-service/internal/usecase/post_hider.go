package usecase

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"go.uber.org/zap"
)

// PostHider hides a POST and fires its side-effects: hashtag counter delta and
// the platform-hidden event (seller notification). It depends only on leaf
// collaborators (postRepo, hashtagUC, eventPublisher), so FanoutHideDispatcher
// can reach it without forming the fx cycle that routing through PostUseCase
// would (PostUseCase -> ReportUseCase -> FanoutHideDispatcher -> PostUseCase).
type PostHider struct {
	postRepo       domain.PostRepository
	hashtagUC      *HashtagUseCase
	eventPublisher ContentEventPublisher
	sellerClient   SellerClient
	revalidate     *revalidate.Client
}

func NewPostHider(
	postRepo domain.PostRepository,
	hashtagUC *HashtagUseCase,
	eventPublisher ContentEventPublisher,
	sellerClient SellerClient,
	revalidateClient *revalidate.Client,
) *PostHider {
	return &PostHider{
		postRepo:       postRepo,
		hashtagUC:      hashtagUC,
		eventPublisher: eventPublisher,
		sellerClient:   sellerClient,
		revalidate:     revalidateClient,
	}
}

// HidePost sets the post hidden with the given reason, decrements hashtag
// counters (best-effort), and publishes PostHiddenByPlatform for the seller
// notification (best-effort). Idempotent: already-hidden posts are a no-op (no
// duplicate event); deleted/missing posts return ErrNotFound.
func (h *PostHider) HidePost(ctx context.Context, postID int64, reason string) error {
	post, err := h.postRepo.GetByID(ctx, postID)
	if err != nil {
		return ErrNotFound
	}
	if post.DeletedAt != nil {
		return ErrNotFound
	}
	if post.HiddenAt != nil {
		return nil
	}
	if err := h.postRepo.HidePost(ctx, postID, reason); err != nil {
		return err
	}
	// Visibility went public → hidden, so -1 for each bound hashtag.
	adjusted, cerr := h.hashtagUC.AdjustCountersForVisibilityChange(ctx, nil, postID, -1)
	if cerr != nil {
		logger.Warn(ctx, "hashtag counter delta failed on hide",
			zap.Int64("post_id", postID), zap.Error(cerr))
	}
	// The post dropped out of the public feed; the feed and trending caches must
	// not keep serving it for the rest of their TTL (§9.3).
	if h.revalidate.Enabled() {
		tags := append([]string{revalidate.PostTag(postID)}, revalidate.HashtagTags(adjusted...)...)
		h.revalidate.BustAsync(ctx, tags)
	}
	if h.eventPublisher != nil {
		resolvedSellerAccountID := h.sellerClient.ResolvePrincipal(ctx, post.SellerID)
		_ = h.eventPublisher.PublishPostHiddenByPlatform(ctx, resolvedSellerAccountID, postID, post.SellerID, reason)
	}
	return nil
}
