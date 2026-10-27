package postgres

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/repository/postgres/gen"
)

type notificationInboxRepository struct {
	pool *pgxpool.Pool
	tsid func() int64
}

func NewNotificationInboxRepository(pool *pgxpool.Pool, tsid func() int64) domain.NotificationInboxRepository {
	return &notificationInboxRepository{pool: pool, tsid: tsid}
}

func (r *notificationInboxRepository) Create(ctx context.Context, item *domain.InboxItem) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	var dataBytes []byte
	if len(item.Data) > 0 {
		var err error
		dataBytes, err = json.Marshal(item.Data)
		if err != nil {
			return 0, err
		}
	}
	id := r.tsid()
	item.ID = id
	return q.CreateInboxNotification(ctx, gen.CreateInboxNotificationParams{
		ID:          id,
		UserID:      item.UserID,
		UserType:    item.UserType,
		EventType:   item.EventType,
		ReferenceID: nullableText(item.ReferenceID),
		Title:       item.Title,
		Body:        item.Body,
		Data:        dataBytes,
	})
}

func (r *notificationInboxRepository) ListByUser(ctx context.Context, userID int64, userType string, cursorID int64, limit int32, category string) ([]*domain.InboxItem, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListByUser(ctx, gen.ListByUserParams{
		UserType:   userType,
		UserID:     userID,
		CursorID:   cursorID,
		LimitCount: limit,
		Category:   category,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*domain.InboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, rowToInboxItem(row))
	}
	return items, nil
}

func (r *notificationInboxRepository) CountByUser(ctx context.Context, userID int64, userType string) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountByUser(ctx, gen.CountByUserParams{UserType: userType, UserID: userID})
}

func (r *notificationInboxRepository) CountUnread(ctx context.Context, userID int64, userType string) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountUnread(ctx, gen.CountUnreadParams{UserType: userType, UserID: userID})
}

func (r *notificationInboxRepository) MarkAsRead(ctx context.Context, id, userID int64, userType string) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.MarkAsRead(ctx, gen.MarkAsReadParams{ID: id, UserType: userType, UserID: userID})
}

func (r *notificationInboxRepository) MarkAllAsRead(ctx context.Context, userID int64, userType string) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.MarkAllAsRead(ctx, gen.MarkAllAsReadParams{UserType: userType, UserID: userID})
}

func (r *notificationInboxRepository) SoftDeleteByIDs(ctx context.Context, ids []int64, userID int64, userType string) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.SoftDeleteByIDs(ctx, gen.SoftDeleteByIDsParams{IDs: ids, UserType: userType, UserID: userID})
}

func (r *notificationInboxRepository) RedactChatBySender(ctx context.Context, senderIDs []int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	// senderId is stored as a string in data->>'senderId' (TSID string convention);
	// convert the caller's full historical id set ([]int64) to the []string the
	// generated batch query expects.
	ids := make([]string, len(senderIDs))
	for i, id := range senderIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	return q.RedactChatNotificationsBySender(ctx, ids)
}

func (r *notificationInboxRepository) SoftDeleteByHistoricalBuyerIDs(ctx context.Context, buyerIDs []int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.SoftDeleteByHistoricalBuyerIDs(ctx, buyerIDs)
}

func (r *notificationInboxRepository) SoftDeleteByHistoricalStoreIDs(ctx context.Context, storeIDs []int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.SoftDeleteByHistoricalStoreIDs(ctx, storeIDs)
}

func rowToInboxItem(row gen.Notification) *domain.InboxItem {
	item := &domain.InboxItem{
		ID:        row.ID,
		UserID:    row.UserID,
		UserType:  row.UserType,
		EventType: row.EventType,
		Title:     row.Title,
		Body:      row.Body,
		IsRead:    row.IsRead,
	}
	if row.ReferenceID != nil {
		item.ReferenceID = *row.ReferenceID
	}
	if row.CreatedAt.Valid {
		item.CreatedAt = timeutil.FormatISO8601(row.CreatedAt.Time)
	}
	if row.ReadAt != nil {
		s := timeutil.FormatISO8601(*row.ReadAt)
		item.ReadAt = &s
	}
	if len(row.Data) > 0 {
		var data map[string]string
		if err := json.Unmarshal(row.Data, &data); err == nil {
			item.Data = data
		}
	}
	return item
}
