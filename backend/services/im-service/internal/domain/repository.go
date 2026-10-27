package domain

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
)

// ConversationRepository defines methods for conversation data
type ConversationRepository interface {
	Upsert(ctx context.Context, arg gen.UpsertConversationParams) (gen.Conversation, error)
	GetByRoomID(ctx context.Context, roomID string) (gen.Conversation, error)
	ListByBuyer(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error)
	ListBySeller(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error)
	IncrementBuyerUnread(ctx context.Context, roomID string) error
	IncrementSellerUnread(ctx context.Context, roomID string) error
	ResetBuyerUnread(ctx context.Context, roomID string) error
	ResetSellerUnread(ctx context.Context, roomID string) error
	ResetAllBuyerUnread(ctx context.Context, buyerID int64) error
	ResetAllSellerUnread(ctx context.Context, storeID int64) error
	SumBuyerUnreadCount(ctx context.Context, buyerID int64) (int64, error)
	SumSellerUnreadCount(ctx context.Context, storeID int64) (int64, error)
	// RedactLastMessageBySender blanks the denormalized preview on conversations
	// whose last message was sent by any of senderIDs (account.deleted/buyer.deleted
	// PII purge — D8: senderIDs enumerates every historical buyer_id/store_id for
	// the deleted account, not a single id).
	RedactLastMessageBySender(ctx context.Context, senderIDs []int64, placeholder string) error
	// BindOrder writes the server-derived order_no binding (14 號 §6.5.1). Both
	// write paths (order-context conversation start, existing-conversation
	// backfill) funnel through this; there is no client-writable API for it.
	BindOrder(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error
}

// MessageRepository defines methods for message data
type MessageRepository interface {
	Create(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error)
	CreateWithLock(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error)
	GetByID(ctx context.Context, id int64) (gen.Message, error)
	List(ctx context.Context, arg gen.ListMessagesParams) ([]gen.Message, error)
	ListBefore(ctx context.Context, arg gen.ListMessagesBeforeParams) ([]gen.Message, error)
	MarkAsRead(ctx context.Context, arg gen.MarkMessagesAsReadParams) error
	MarkByIDsAsRead(ctx context.Context, receiverID int64, ids []int64) error
	RecalcUnreadForMessages(ctx context.Context, messageIDs []int64) error
	MarkAllAsReadByBuyer(ctx context.Context, receiverID int64) error
	MarkAllAsReadBySeller(ctx context.Context, receiverID int64) error
	ListAfterSeq(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error)
	HideMessage(ctx context.Context, id int64, reason string) error
	// RedactBySender purges the content of every message sent by any of senderIDs
	// (account.deleted/buyer.deleted PII erasure, D8); returns the number of rows
	// redacted.
	RedactBySender(ctx context.Context, senderIDs []int64) (int64, error)
	// ListImageMessageIDsBySender returns the IDs of every image message (type=2)
	// sent by any of senderIDs, used by the PII-purge path to release media
	// references.
	ListImageMessageIDsBySender(ctx context.Context, senderIDs []int64) ([]int64, error)
}
