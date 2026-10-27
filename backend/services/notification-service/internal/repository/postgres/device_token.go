package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/repository/postgres/gen"
)

type deviceTokenRepository struct {
	pool *pgxpool.Pool
	tsid func() int64
}

func NewDeviceTokenRepository(pool *pgxpool.Pool, tsid func() int64) domain.DeviceTokenRepository {
	return &deviceTokenRepository{pool: pool, tsid: tsid}
}

func (r *deviceTokenRepository) Upsert(ctx context.Context, userID int64, userType, token, platform string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpsertDeviceToken(ctx, gen.UpsertDeviceTokenParams{
		ID:       r.tsid(),
		UserID:   userID,
		UserType: userType,
		Token:    token,
		Platform: platform,
	})
}

func (r *deviceTokenRepository) Deactivate(ctx context.Context, userID int64, userType, token string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.DeactivateDeviceToken(ctx, gen.DeactivateDeviceTokenParams{
		UserID:   userID,
		UserType: userType,
		Token:    token,
	})
}

func (r *deviceTokenRepository) DeactivateByToken(ctx context.Context, token string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.DeactivateTokenByValue(ctx, token)
}

func (r *deviceTokenRepository) DeactivateByUser(ctx context.Context, userID int64, userType string) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.DeactivateDeviceTokensByUser(ctx, gen.DeactivateDeviceTokensByUserParams{
		UserID:   userID,
		UserType: userType,
	})
}

func (r *deviceTokenRepository) GetActiveByUser(ctx context.Context, userID int64, userType string) ([]*domain.DeviceToken, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetActiveTokensByUser(ctx, gen.GetActiveTokensByUserParams{
		UserID:   userID,
		UserType: userType,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.DeviceToken, len(rows))
	for i, row := range rows {
		result[i] = &domain.DeviceToken{
			ID:       row.ID,
			UserID:   row.UserID,
			UserType: row.UserType,
			Token:    row.Token,
			Platform: row.Platform,
			IsActive: row.IsActive,
		}
	}
	return result, nil
}

func (r *deviceTokenRepository) GetActiveByUserType(ctx context.Context, userType string, limit, offset int64) ([]*domain.DeviceToken, error) {
	const query = `
SELECT id, user_id, user_type, token, platform, is_active, created_at, updated_at
FROM device_tokens
WHERE user_type = $1 AND is_active = true
ORDER BY id
LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userType, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.DeviceToken
	for rows.Next() {
		dt := &domain.DeviceToken{}
		var createdAt, updatedAt interface{}
		if err := rows.Scan(&dt.ID, &dt.UserID, &dt.UserType, &dt.Token, &dt.Platform, &dt.IsActive, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		result = append(result, dt)
	}
	return result, rows.Err()
}
