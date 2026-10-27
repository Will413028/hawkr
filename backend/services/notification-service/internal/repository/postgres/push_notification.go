package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/repository/postgres/gen"
)

type pushNotificationRepository struct {
	pool *pgxpool.Pool
	tsid func() int64
}

func NewPushNotificationRepository(pool *pgxpool.Pool, tsid func() int64) domain.PushNotificationRepository {
	return &pushNotificationRepository{pool: pool, tsid: tsid}
}

func (r *pushNotificationRepository) Create(ctx context.Context, record *domain.PushNotificationRecord) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	var sentAt *time.Time
	if record.Status == domain.StatusFailed || record.Status == domain.StatusSkipped {
		sentAt = nil
	} else {
		now := time.Now()
		sentAt = &now
	}

	return q.CreatePushNotification(ctx, gen.CreatePushNotificationParams{
		ID:          r.tsid(),
		UserID:      record.UserID,
		UserType:    record.UserType,
		EventType:   record.EventType,
		ReferenceID: nullableText(record.ReferenceID),
		Status:      record.Status,
		SentAt:      sentAt,
		ErrorLog:    nullableText(record.ErrorLog),
	})
}

func (r *pushNotificationRepository) ExistsProcessed(ctx context.Context, eventType, referenceID string, userID int64, userType string) (bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.ExistsPushProcessed(ctx, gen.ExistsPushProcessedParams{
		EventType:   eventType,
		ReferenceID: nullableText(referenceID),
		UserID:      userID,
		UserType:    userType,
	})
}
