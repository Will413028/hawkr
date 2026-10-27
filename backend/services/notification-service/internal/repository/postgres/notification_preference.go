package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/repository/postgres/gen"
)

type notificationPreferenceRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationPreferenceRepository(pool *pgxpool.Pool) domain.NotificationPreferenceRepository {
	return &notificationPreferenceRepository{pool: pool}
}

func (r *notificationPreferenceRepository) Upsert(ctx context.Context, userID int64, userType, category string, enabled bool) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpsertNotificationPreference(ctx, gen.UpsertNotificationPreferenceParams{
		UserID:   userID,
		UserType: userType,
		Category: category,
		Enabled:  enabled,
	})
}

func (r *notificationPreferenceRepository) GetByUser(ctx context.Context, userID int64, userType string) ([]*domain.NotificationPreference, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetNotificationPreferencesByUser(ctx, gen.GetNotificationPreferencesByUserParams{
		UserID:   userID,
		UserType: userType,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.NotificationPreference, len(rows))
	for i, row := range rows {
		result[i] = &domain.NotificationPreference{
			UserID:   row.UserID,
			UserType: row.UserType,
			Category: row.Category,
			Enabled:  row.Enabled,
		}
	}
	return result, nil
}

func (r *notificationPreferenceRepository) GetByCategory(ctx context.Context, userID int64, userType, category string) (*domain.NotificationPreference, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetNotificationPreference(ctx, gen.GetNotificationPreferenceParams{
		UserID:   userID,
		UserType: userType,
		Category: category,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &domain.NotificationPreference{
		UserID:   row.UserID,
		UserType: row.UserType,
		Category: row.Category,
		Enabled:  row.Enabled,
	}, nil
}
