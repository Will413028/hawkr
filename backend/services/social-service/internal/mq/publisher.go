package mq

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"go.uber.org/zap"
)

// ContentEventPublisher defines the interface for publishing content events.
// 2026-06: 配合 41 號設計書 SSE realtime 改造 — comment.created/replied 加 fat snapshot + clientRequestId；
//
//	新增 comment.updated/deleted/like_changed；刪除 comment.liked（無過渡期）。
type ContentEventPublisher interface {
	PublishPostCreated(ctx context.Context, postID, storeID, followerID int64, sellerName, sellerAvatar, contentPreview string) error
	PublishPostLiked(ctx context.Context, sellerID, postID, storeID int64, likerName string) error
	PublishPostHiddenByPlatform(ctx context.Context, sellerID, postID, storeID int64, reason string) error

	// Comment events — 41 號設計書 §6.1
	// commenterPrincipalID / replierPrincipalID 與 commenterID / replierID 是同一個人的
	// 兩個值：後者是 SSE 用的 surrogate（不解析），前者是 self-skip 用的、與收件人欄位
	// 同身分空間的值。詳見 pkgmq.CommentCreatedEvent 欄位註解。
	PublishCommentCreated(
		ctx context.Context,
		commentID, postID, commenterID, commenterPrincipalID int64,
		commenterName string,
		postAuthorID, postAuthorStoreID int64,
		postAuthorType, contentPreview string,
		parentCommentID int64,
		clientRequestID string,
		snapshot *pkgmq.CommentSnapshot,
	) error
	PublishCommentReplied(
		ctx context.Context,
		replyToUserID, replierID, replierPrincipalID, replyToUserStoreID int64,
		replyToUserType, replierName string,
		postID, commentID int64,
		contentPreview string,
		parentCommentID int64,
		clientRequestID string,
		snapshot *pkgmq.CommentSnapshot,
	) error
	PublishCommentUpdated(ctx context.Context, commentID, postID int64, content, updatedAt string) error
	PublishCommentDeleted(ctx context.Context, commentID, postID, parentCommentID int64) error
	PublishCommentLikeChanged(
		ctx context.Context,
		commentID, postID int64, likeCount int32, action string,
		userID, commentAuthorID, commentAuthorStoreID, likerPrincipalID int64, likerName, commentAuthorType string,
	) error

	// PublishCounterDelta requests an async +1/-1 update to a denormalized
	// counter. entityType is "post" or "comment"; field is "comment_count",
	// "reply_count", or "like_count". See
	// docs/superpowers/specs/2026-07-31-social-counter-async-consistency-design.md.
	PublishCounterDelta(ctx context.Context, entityType string, entityID int64, field string, delta int32) error
}

// Publisher wraps go-pkg/mq.Publisher to implement ContentEventPublisher.
type Publisher struct {
	pub *pkgmq.Publisher
}

// NewPublisher creates a new MQ publisher for content events.
func NewPublisher(url string) (*Publisher, error) {
	pub, err := pkgmq.NewPublisher(url)
	if err != nil {
		return nil, err
	}
	return &Publisher{pub: pub}, nil
}

// PublishPostCreated publishes a POST_CREATED event targeting a specific follower.
func (p *Publisher) PublishPostCreated(ctx context.Context, postID, storeID, followerID int64, sellerName, sellerAvatar, contentPreview string) error {
	event := pkgmq.NewPostCreatedEvent(postID, storeID, followerID, sellerName, sellerAvatar, contentPreview)
	if err := p.pub.PublishPostCreated(ctx, event); err != nil {
		zap.L().Error("failed to publish post.created", zap.Error(err))
		return err
	}
	return nil
}

// PublishCommentCreated publishes a COMMENT_CREATED event with full fat snapshot.
// 41 號設計書 §3.3.1:publisher 一律 publish,self-author skip 下放 notification consumer
// (template_handler.go) 判斷。確保 gateway SSE 不漏推給留言者跨 tab / 跨裝置(§一.3 設計目標)。
// 注意:fat event payload 含完整 CommentSnapshot,供 gateway SSE 直接 dispatch(無需二次 gRPC)。
func (p *Publisher) PublishCommentCreated(
	ctx context.Context,
	commentID, postID, commenterID, commenterPrincipalID int64,
	commenterName string,
	postAuthorID, postAuthorStoreID int64,
	postAuthorType, contentPreview string,
	parentCommentID int64,
	clientRequestID string,
	snapshot *pkgmq.CommentSnapshot,
) error {
	event := pkgmq.NewCommentCreatedEvent(commentID, postID, commenterID, commenterPrincipalID, commenterName, postAuthorID, postAuthorStoreID, postAuthorType, contentPreview, parentCommentID, clientRequestID, snapshot)
	if err := p.pub.PublishCommentCreated(ctx, event); err != nil {
		zap.L().Error("failed to publish comment.created", zap.Error(err))
		return err
	}
	return nil
}

func (p *Publisher) PublishPostLiked(ctx context.Context, sellerID, postID, storeID int64, likerName string) error {
	event := pkgmq.NewPostLikedEvent(sellerID, postID, storeID, likerName)
	if err := p.pub.PublishPostLiked(ctx, event); err != nil {
		zap.L().Error("failed to publish post.liked", zap.Error(err))
		return err
	}
	return nil
}

// PublishCommentReplied publishes a COMMENT_REPLIED event with full fat snapshot.
// 41 號設計書 §3.3.1:publisher 一律 publish,self-reply skip 下放 notification consumer
// (template_handler.go) 判斷。確保 gateway SSE 不漏推給回覆者跨 tab / 跨裝置(§一.3)。
func (p *Publisher) PublishCommentReplied(
	ctx context.Context,
	replyToUserID, replierID, replierPrincipalID, replyToUserStoreID int64,
	replyToUserType, replierName string,
	postID, commentID int64,
	contentPreview string,
	parentCommentID int64,
	clientRequestID string,
	snapshot *pkgmq.CommentSnapshot,
) error {
	event := pkgmq.NewCommentRepliedEvent(replyToUserID, replierID, replierPrincipalID, replyToUserStoreID, replyToUserType, replierName, postID, commentID, contentPreview, parentCommentID, clientRequestID, snapshot)
	if err := p.pub.PublishCommentReplied(ctx, event); err != nil {
		zap.L().Error("failed to publish comment.replied", zap.Error(err))
		return err
	}
	return nil
}

// PublishCommentUpdated 41 號設計書 §6.1 新增。Gateway SSE consumer only。
func (p *Publisher) PublishCommentUpdated(ctx context.Context, commentID, postID int64, content, updatedAt string) error {
	event := pkgmq.NewCommentUpdatedEvent(commentID, postID, content, updatedAt)
	if err := p.pub.PublishCommentUpdated(ctx, event); err != nil {
		zap.L().Error("failed to publish comment.updated", zap.Error(err))
		return err
	}
	return nil
}

// PublishCommentDeleted 41 號設計書 §6.1 新增。Gateway SSE consumer only。
func (p *Publisher) PublishCommentDeleted(ctx context.Context, commentID, postID, parentCommentID int64) error {
	event := pkgmq.NewCommentDeletedEvent(commentID, postID, parentCommentID)
	if err := p.pub.PublishCommentDeleted(ctx, event); err != nil {
		zap.L().Error("failed to publish comment.deleted", zap.Error(err))
		return err
	}
	return nil
}

// PublishCommentLikeChanged 41 號設計書 §6.1 新增（取代 PublishCommentLiked，無過渡期）。
// Consumer：① notification-service（過濾 action="LIKE" 才推播）② gateway SSE。
func (p *Publisher) PublishCommentLikeChanged(
	ctx context.Context,
	commentID, postID int64, likeCount int32, action string,
	userID int64, likerKind int32, commentAuthorID, commentAuthorStoreID, likerPrincipalID int64, likerName, commentAuthorType string,
) error {
	event := pkgmq.NewCommentLikeChangedEvent(commentID, postID, likeCount, action, userID, likerKind, commentAuthorID, commentAuthorStoreID, likerPrincipalID, likerName, commentAuthorType)
	if err := p.pub.PublishCommentLikeChanged(ctx, event); err != nil {
		zap.L().Error("failed to publish comment.like_changed", zap.Error(err))
		return err
	}
	return nil
}

// PublishCounterDelta publishes a COUNTER_DELTA event for async counter maintenance.
// See docs/superpowers/specs/2026-07-31-social-counter-async-consistency-design.md.
func (p *Publisher) PublishCounterDelta(ctx context.Context, entityType string, entityID int64, field string, delta int32) error {
	event := pkgmq.NewCounterDeltaEvent(entityType, entityID, field, delta)
	if err := p.pub.PublishCounterDelta(ctx, event); err != nil {
		zap.L().Error("failed to publish counter.delta", zap.Error(err))
		return err
	}
	return nil
}

func (p *Publisher) PublishPostHiddenByPlatform(ctx context.Context, sellerID, postID, storeID int64, reason string) error {
	event := pkgmq.NewPostHiddenByPlatformEvent(sellerID, postID, storeID, reason)
	if err := p.pub.PublishPostHiddenByPlatform(ctx, event); err != nil {
		zap.L().Error("failed to publish post.hidden_by_platform", zap.Error(err))
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
