package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type storeVisitorRepository struct {
	pool *pgxpool.Pool
}

func NewStoreVisitorRepository(pool *pgxpool.Pool) domain.StoreVisitorRepository {
	return &storeVisitorRepository{pool: pool}
}

func (r *storeVisitorRepository) UpsertVisit(ctx context.Context, storeID, visitorBuyerID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpsertStoreVisitor(ctx, gen.UpsertStoreVisitorParams{
		StoreID:        storeID,
		VisitorBuyerID: visitorBuyerID,
	})
}

func (r *storeVisitorRepository) List(ctx context.Context, storeID int64, limit, offset int32) ([]*domain.StoreVisitorRow, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListStoreVisitors(ctx, gen.ListStoreVisitorsParams{
		StoreID: storeID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.StoreVisitorRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.StoreVisitorRow{
			VisitorBuyerID: row.VisitorBuyerID,
			VisitCount:     row.VisitCount,
			LastVisitedAt:  row.LastVisitedAt.Time,
		})
	}
	return result, nil
}

func (r *storeVisitorRepository) Count(ctx context.Context, storeID int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountStoreVisitors(ctx, storeID)
}

func (r *storeVisitorRepository) CountByDateRange(ctx context.Context, storeID int64, from, to time.Time) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountVisitorsByDateRange(ctx, gen.CountVisitorsByDateRangeParams{
		StoreID:         storeID,
		LastVisitedAt:   timestamptzParam(from),
		LastVisitedAt_2: timestamptzParam(to),
	})
}
