package domain

import (
	"context"
	"time"
)

type SystemNotificationLog struct {
	TriggeredAt          time.Time
	CompletedAt          *time.Time
	FCMMessageID         *string
	FailedReason         *string
	ImageKey             *string // image objectKey（顯示 URL 由渲染端組；49 §5.3 #10）
	DeepLinkURL          *string
	NotificationType     string
	Title                string
	Body                 string
	ContentHash          string
	Status               string
	ID                   int64
	TriggeredByAccountID int64
}

type SystemNotificationLogRepository interface {
	Insert(ctx context.Context, log *SystemNotificationLog) error
	FindRecentByContentHash(ctx context.Context, contentHash string, since time.Time) (*SystemNotificationLog, error)
	List(ctx context.Context, notificationType string, limit, offset int64) ([]*SystemNotificationLog, error)
	Count(ctx context.Context, notificationType string) (int64, error)
	UpdateStatus(ctx context.Context, id int64, status string, fcmMessageID, failedReason *string) error
	DeleteOlderThan(ctx context.Context, threshold time.Time) (int64, error)
}
