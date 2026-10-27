package postgres

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversationRepository struct {
	pool *pgxpool.Pool
}

func NewConversationRepository(pool *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{pool: pool}
}

func (r *ConversationRepository) getQueries(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

func (r *ConversationRepository) GetByRoomID(ctx context.Context, roomID string) (gen.Conversation, error) {
	return r.getQueries(ctx).GetConversationByRoomID(ctx, roomID)
}

func (r *ConversationRepository) ListByBuyer(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
	return r.getQueries(ctx).ListConversationsByBuyer(ctx, arg)
}

func (r *ConversationRepository) ListBySeller(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
	return r.getQueries(ctx).ListConversationsBySeller(ctx, arg)
}

func (r *ConversationRepository) Upsert(ctx context.Context, arg gen.UpsertConversationParams) (gen.Conversation, error) {
	return r.getQueries(ctx).UpsertConversation(ctx, arg)
}

func (r *ConversationRepository) IncrementBuyerUnread(ctx context.Context, roomID string) error {
	return r.getQueries(ctx).IncrementBuyerUnread(ctx, roomID)
}

func (r *ConversationRepository) IncrementSellerUnread(ctx context.Context, roomID string) error {
	return r.getQueries(ctx).IncrementSellerUnread(ctx, roomID)
}

func (r *ConversationRepository) ResetBuyerUnread(ctx context.Context, roomID string) error {
	return r.getQueries(ctx).ResetBuyerUnread(ctx, roomID)
}

func (r *ConversationRepository) ResetSellerUnread(ctx context.Context, roomID string) error {
	return r.getQueries(ctx).ResetSellerUnread(ctx, roomID)
}

func (r *ConversationRepository) ResetAllBuyerUnread(ctx context.Context, buyerID int64) error {
	return r.getQueries(ctx).ResetAllBuyerUnread(ctx, buyerID)
}

func (r *ConversationRepository) ResetAllSellerUnread(ctx context.Context, storeID int64) error {
	return r.getQueries(ctx).ResetAllSellerUnread(ctx, storeID)
}

func (r *ConversationRepository) SumBuyerUnreadCount(ctx context.Context, buyerID int64) (int64, error) {
	return r.getQueries(ctx).SumBuyerUnreadCount(ctx, buyerID)
}

func (r *ConversationRepository) SumSellerUnreadCount(ctx context.Context, storeID int64) (int64, error) {
	return r.getQueries(ctx).SumSellerUnreadCount(ctx, storeID)
}

// RedactLastMessageBySender blanks the denormalized last_message_content preview
// on conversations whose last message was sent by any of senderIDs
// (account.deleted/buyer.deleted PII purge, D8).
func (r *ConversationRepository) RedactLastMessageBySender(ctx context.Context, senderIDs []int64, placeholder string) error {
	return r.getQueries(ctx).RedactConversationLastMessageBySender(ctx, gen.RedactConversationLastMessageBySenderParams{
		SenderIds:   senderIDs,
		Placeholder: &placeholder,
	})
}

// BindOrder writes the server-derived order_no binding onto a conversation
// (14 號 §6.5.1, both write paths funnel through this — client has no API to
// write order_no directly).
func (r *ConversationRepository) BindOrder(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error {
	return r.getQueries(ctx).BindConversationOrder(ctx, gen.BindConversationOrderParams{
		RoomID:     roomID,
		OrderNo:    &orderNo,
		VerifiedAt: &verifiedAt,
	})
}

var _ domain.ConversationRepository = (*ConversationRepository)(nil)
