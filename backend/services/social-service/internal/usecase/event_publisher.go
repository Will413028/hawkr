package usecase

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

// ContentEventPublisher defines the interface for publishing content events (posts, comments).
// 2026-06: 配合 41 號設計書 §6.1 — comment.created/replied 加 fat snapshot + clientRequestId；
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
	// PublishCommentLikeChanged 的 userID、likerKind 是按讚者的 (kind, id)，SSE 依它算 isMine；
	// likerPrincipalID 是 self-skip 用的、與 commentAuthorID 同身分空間的值。詳見
	// pkgmq.CommentLikeChangedEvent 欄位註解。
	PublishCommentLikeChanged(
		ctx context.Context,
		commentID, postID int64, likeCount int32, action string,
		userID int64, likerKind int32, commentAuthorID, commentAuthorStoreID, likerPrincipalID int64, likerName, commentAuthorType string,
	) error

	// PublishCounterDelta requests an async +1/-1 update to a denormalized
	// counter. entityType is "post" or "comment"; field is "comment_count",
	// "reply_count", or "like_count". See
	// docs/superpowers/specs/2026-07-31-social-counter-async-consistency-design.md.
	PublishCounterDelta(ctx context.Context, entityType string, entityID int64, field string, delta int32) error
}
