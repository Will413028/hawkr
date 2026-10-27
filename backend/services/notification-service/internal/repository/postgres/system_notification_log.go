package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

type systemNotificationLogRepository struct {
	pool *pgxpool.Pool
	tsid func() int64
}

// NewSystemNotificationLogRepository 建立 SystemNotificationLogRepository（pgx 直接查詢，不依賴 sqlc gen）。
func NewSystemNotificationLogRepository(pool *pgxpool.Pool, tsid func() int64) domain.SystemNotificationLogRepository {
	return &systemNotificationLogRepository{pool: pool, tsid: tsid}
}

const systemLogColumns = `
  id, notification_type, title, body, image_key, deep_link_url,
  content_hash, triggered_by_account_id, triggered_at, fcm_message_id,
  status, failed_reason, completed_at`

func scanSystemLog(row interface{ Scan(...any) error }) (*domain.SystemNotificationLog, error) {
	l := &domain.SystemNotificationLog{}
	var imageURL, deepLinkURL, fcmMessageID, failedReason *string
	var completedAt *time.Time
	if err := row.Scan(
		&l.ID, &l.NotificationType, &l.Title, &l.Body, &imageURL, &deepLinkURL,
		&l.ContentHash, &l.TriggeredByAccountID, &l.TriggeredAt, &fcmMessageID,
		&l.Status, &failedReason, &completedAt,
	); err != nil {
		return nil, err
	}
	l.ImageKey = imageURL
	l.DeepLinkURL = deepLinkURL
	l.FCMMessageID = fcmMessageID
	l.FailedReason = failedReason
	l.CompletedAt = completedAt
	return l, nil
}

func (r *systemNotificationLogRepository) Insert(ctx context.Context, l *domain.SystemNotificationLog) error {
	// Always generate a fresh TSID regardless of caller-supplied ID,
	// matching the pattern in notification_template.go to prevent accidental zero or hardcoded IDs.
	l.ID = r.tsid()
	row := r.pool.QueryRow(ctx, `
INSERT INTO system_notification_logs (
  id, notification_type, title, body, image_key, deep_link_url,
  content_hash, triggered_by_account_id, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
RETURNING triggered_at`,
		l.ID, l.NotificationType, l.Title, l.Body, l.ImageKey, l.DeepLinkURL,
		l.ContentHash, l.TriggeredByAccountID,
	)
	if err := row.Scan(&l.TriggeredAt); err != nil {
		return err
	}
	// SQL always inserts 'pending' regardless of caller's Status field;
	// sync in-memory state with the DB so the returned log reflects what was persisted.
	l.Status = "pending"
	return nil
}

func (r *systemNotificationLogRepository) FindRecentByContentHash(ctx context.Context, contentHash string, since time.Time) (*domain.SystemNotificationLog, error) {
	row := r.pool.QueryRow(ctx, `SELECT`+systemLogColumns+`
FROM system_notification_logs
WHERE content_hash = $1 AND triggered_at > $2
ORDER BY triggered_at DESC
LIMIT 1`, contentHash, since)
	l, err := scanSystemLog(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return l, err
}

func (r *systemNotificationLogRepository) List(ctx context.Context, notificationType string, limit, offset int64) ([]*domain.SystemNotificationLog, error) {
	var rows pgx.Rows
	var err error
	if notificationType == "" {
		rows, err = r.pool.Query(ctx, `SELECT`+systemLogColumns+`
FROM system_notification_logs
ORDER BY triggered_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT`+systemLogColumns+`
FROM system_notification_logs
WHERE notification_type = $1
ORDER BY triggered_at DESC LIMIT $2 OFFSET $3`, notificationType, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.SystemNotificationLog
	for rows.Next() {
		l, err := scanSystemLog(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *systemNotificationLogRepository) Count(ctx context.Context, notificationType string) (int64, error) {
	var count int64
	var row pgx.Row
	if notificationType == "" {
		row = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM system_notification_logs`)
	} else {
		row = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM system_notification_logs WHERE notification_type = $1`, notificationType)
	}
	err := row.Scan(&count)
	return count, err
}

func (r *systemNotificationLogRepository) UpdateStatus(ctx context.Context, id int64, status string, fcmMessageID, failedReason *string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE system_notification_logs
SET status = $2, fcm_message_id = $3, failed_reason = $4, completed_at = CURRENT_TIMESTAMP
WHERE id = $1`, id, status, fcmMessageID, failedReason)
	return err
}

func (r *systemNotificationLogRepository) DeleteOlderThan(ctx context.Context, threshold time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM system_notification_logs WHERE triggered_at < $1`, threshold)
	return tag.RowsAffected(), err
}
