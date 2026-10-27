package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type payuniMerchantProfileUpdateRepository struct {
	pool *pgxpool.Pool
}

func NewPayuniMerchantProfileUpdateRepository(pool *pgxpool.Pool) domain.PayuniMerchantProfileUpdateRepository {
	return &payuniMerchantProfileUpdateRepository{pool: pool}
}

func (r *payuniMerchantProfileUpdateRepository) Insert(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	_, err := q.InsertPayuniMerchantProfileUpdate(ctx, gen.InsertPayuniMerchantProfileUpdateParams{
		ID:             u.ID,
		StoreID:        u.StoreID,
		AccountID:      u.AccountID,
		IdempotencyKey: u.IdempotencyKey,
		PayloadHash:    u.PayloadHash,
		Status:         string(u.Status),
		ChangedFields:  pgxutil.JSONBytes(u.ChangedFields),
		Reason:         nullableText(u.Reason),
		TraceID:        nullableText(u.TraceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *payuniMerchantProfileUpdateRepository) FindByStoreAndKey(ctx context.Context, storeID int64, idempotencyKey string) (*domain.PayuniMerchantProfileUpdate, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetPayuniMerchantProfileUpdateByStoreAndKey(ctx, gen.GetPayuniMerchantProfileUpdateByStoreAndKeyParams{
		StoreID: storeID, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("payuni merchant profile update not found")
		}
		return nil, err
	}
	return &domain.PayuniMerchantProfileUpdate{
		ID:              row.ID,
		StoreID:         row.StoreID,
		AccountID:       row.AccountID,
		IdempotencyKey:  row.IdempotencyKey,
		PayloadHash:     row.PayloadHash,
		Status:          domain.PayuniMerchantProfileUpdateStatus(row.Status),
		ChangedFields:   pgxutil.JSONRawMessage(row.ChangedFields),
		Reason:          stringValue(row.Reason),
		TraceID:         stringValue(row.TraceID),
		ProviderStatus:  stringValue(row.ProviderStatus),
		ProviderMessage: stringValue(row.ProviderMessage),
		CreatedAt:       row.CreatedAt.Time,
		CompletedAt:     row.CompletedAt,
	}, nil
}

func (r *payuniMerchantProfileUpdateRepository) MarkSucceeded(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.MarkPayuniMerchantProfileUpdateSucceeded(ctx, gen.MarkPayuniMerchantProfileUpdateSucceededParams{
		ID: id, ProviderStatus: nullableText(providerStatus), ProviderMessage: nullableText(providerMessage),
		CompletedAt: nullableTime(completedAt),
	})
}

func (r *payuniMerchantProfileUpdateRepository) MarkFailed(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.MarkPayuniMerchantProfileUpdateFailed(ctx, gen.MarkPayuniMerchantProfileUpdateFailedParams{
		ID: id, ProviderStatus: nullableText(providerStatus), ProviderMessage: nullableText(providerMessage),
		CompletedAt: nullableTime(completedAt),
	})
}

func (r *payuniMerchantProfileUpdateRepository) Delete(ctx context.Context, id int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.DeletePayuniMerchantProfileUpdate(ctx, id)
}
