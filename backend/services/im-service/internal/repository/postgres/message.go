package postgres

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct {
	pool      *pgxpool.Pool
	txManager database.TxManager
}

func NewMessageRepository(pool *pgxpool.Pool, txManager database.TxManager) *MessageRepository {
	return &MessageRepository{pool: pool, txManager: txManager}
}

func (r *MessageRepository) getQueries(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

func (r *MessageRepository) Create(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	return r.getQueries(ctx).CreateMessage(ctx, arg)
}

// CreateWithLock 在 transaction 內先取得 advisory lock 再 INSERT，確保同 room 的 seq_id atomic。
// Transaction 生命週期交給 txManager.WithTx（pkg/database/tx_manager.go）——本檔其餘方法
// 都是透過 getQueries -> database.GetDBTX(ctx, r.pool) 讀交易，這裡改成同一條路徑，不再自己
// 手動 pool.Begin/Rollback/Commit。
func (r *MessageRepository) CreateWithLock(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	var msg gen.Message
	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.getQueries(txCtx)
		if err := q.AcquireRoomLock(txCtx, arg.RoomID); err != nil {
			return fmt.Errorf("acquire room lock: %w", err)
		}
		created, err := q.CreateMessage(txCtx, arg)
		if err != nil {
			return err
		}
		msg = created
		return nil
	})
	if err != nil {
		return gen.Message{}, err
	}
	return msg, nil
}

func (r *MessageRepository) GetByID(ctx context.Context, id int64) (gen.Message, error) {
	return r.getQueries(ctx).GetMessageByID(ctx, id)
}

func (r *MessageRepository) List(ctx context.Context, arg gen.ListMessagesParams) ([]gen.Message, error) {
	return r.getQueries(ctx).ListMessages(ctx, arg)
}

func (r *MessageRepository) ListBefore(ctx context.Context, arg gen.ListMessagesBeforeParams) ([]gen.Message, error) {
	return r.getQueries(ctx).ListMessagesBefore(ctx, arg)
}

func (r *MessageRepository) MarkAsRead(ctx context.Context, arg gen.MarkMessagesAsReadParams) error {
	return r.getQueries(ctx).MarkMessagesAsRead(ctx, arg)
}

func (r *MessageRepository) MarkByIDsAsRead(ctx context.Context, receiverID int64, ids []int64) error {
	return r.getQueries(ctx).MarkMessagesByIDsAsRead(ctx, gen.MarkMessagesByIDsAsReadParams{
		Column1:    ids,
		ReceiverID: receiverID,
	})
}

func (r *MessageRepository) RecalcUnreadForMessages(ctx context.Context, messageIDs []int64) error {
	return r.getQueries(ctx).RecalcUnreadForMessages(ctx, messageIDs)
}

func (r *MessageRepository) MarkAllAsReadByBuyer(ctx context.Context, receiverID int64) error {
	return r.getQueries(ctx).MarkAllAsReadByBuyer(ctx, receiverID)
}

func (r *MessageRepository) MarkAllAsReadBySeller(ctx context.Context, receiverID int64) error {
	return r.getQueries(ctx).MarkAllAsReadBySeller(ctx, receiverID)
}

func (r *MessageRepository) ListAfterSeq(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
	return r.getQueries(ctx).ListMessagesAfterSeq(ctx, arg)
}

// HideMessage marks a message as hidden by platform moderation (B-135).
func (r *MessageRepository) HideMessage(ctx context.Context, id int64, reason string) error {
	return r.getQueries(ctx).HideMessage(ctx, gen.HideMessageParams{
		ID:           id,
		HiddenReason: nullableText(reason),
	})
}

// RedactBySender purges the content of every message sent by any of senderIDs,
// used by the account.deleted/buyer.deleted consumer for 個資法 right-to-erasure
// (D8: senderIDs enumerates every historical buyer_id/store_id). Returns rows redacted.
func (r *MessageRepository) RedactBySender(ctx context.Context, senderIDs []int64) (int64, error) {
	return r.getQueries(ctx).RedactMessagesBySender(ctx, senderIDs)
}

// ListImageMessageIDsBySender returns the IDs of every image message (type=2) sent
// by any of senderIDs, used by the account.deleted/buyer.deleted consumer to
// release media references (D8).
func (r *MessageRepository) ListImageMessageIDsBySender(ctx context.Context, senderIDs []int64) ([]int64, error) {
	return r.getQueries(ctx).ListImageMessageIDsBySender(ctx, senderIDs)
}

var _ domain.MessageRepository = (*MessageRepository)(nil)
