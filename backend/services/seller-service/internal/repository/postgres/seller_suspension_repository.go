package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

// Admin manual seller suspension repository (32 §13.2 / 44 §13.2).

type sellerSuspensionRepository struct {
	pool *pgxpool.Pool
}

// NewSellerSuspensionRepository constructs the repository.
func NewSellerSuspensionRepository(pool *pgxpool.Pool) domain.SellerSuspensionRepository {
	return &sellerSuspensionRepository{pool: pool}
}

func (r *sellerSuspensionRepository) Insert(ctx context.Context, s *domain.SellerSuspension) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.InsertSellerSuspension(ctx, gen.InsertSellerSuspensionParams{
		ID:                 s.ID,
		SellerID:           s.AccountID,
		ReasonCode:         s.ReasonCode,
		ReasonDetail:       nullableText(s.ReasonDetail),
		SuspendedByAdminID: s.SuspendedByAdminID,
		SuspendedAt:        timestamptzParam(s.SuspendedAt),
		CreatedAt:          timestamptzParam(s.CreatedAt),
	})
	if err != nil {
		if cn, ok := isUniqueViolation(err); ok {
			return &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return err
	}
	*s = *toDomainSellerSuspension(row)
	return nil
}

func (r *sellerSuspensionRepository) GetActiveByAccountID(ctx context.Context, accountID int64) (*domain.SellerSuspension, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetActiveSellerSuspensionByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainSellerSuspension(row), nil
}

func (r *sellerSuspensionRepository) HasActiveByAccountID(ctx context.Context, accountID int64) (bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.HasActiveSellerSuspensionByAccountID(ctx, accountID)
}

func (r *sellerSuspensionRepository) GetByID(ctx context.Context, id int64) (*domain.SellerSuspension, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerSuspensionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller suspension not found")
		}
		return nil, err
	}
	return toDomainSellerSuspension(row), nil
}

func (r *sellerSuspensionRepository) Lift(ctx context.Context, id, liftedByAdminID int64, now time.Time) (*domain.SellerSuspension, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.LiftSellerSuspension(ctx, gen.LiftSellerSuspensionParams{
		ID:              id,
		LiftedByAdminID: int64Pointer(liftedByAdminID),
		LiftedAt:        nullableTime(now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already lifted (or missing id) — caller distinguishes via GetByID.
			return nil, nil
		}
		return nil, err
	}
	return toDomainSellerSuspension(row), nil
}

func (r *sellerSuspensionRepository) ListByAccountID(ctx context.Context, accountID int64, limit, offset int32) ([]*domain.SellerSuspension, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListSellerSuspensionsByAccountID(ctx, gen.ListSellerSuspensionsByAccountIDParams{
		SellerID: accountID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerSuspension, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainSellerSuspension(row))
	}
	return out, nil
}

func toDomainSellerSuspension(row gen.SellerSuspension) *domain.SellerSuspension {
	s := &domain.SellerSuspension{
		ID:                 row.ID,
		AccountID:          row.SellerID,
		ReasonCode:         row.ReasonCode,
		ReasonDetail:       stringValue(row.ReasonDetail),
		Status:             domain.SellerSuspensionStatus(row.Status),
		SuspendedByAdminID: row.SuspendedByAdminID,
		SuspendedAt:        row.SuspendedAt.Time,
		LiftedByAdminID:    row.LiftedByAdminID,
		LiftedAt:           row.LiftedAt,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
	return s
}
