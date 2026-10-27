package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type productViewRepository struct {
	pool *pgxpool.Pool
}

func NewProductViewRepository(pool *pgxpool.Pool) domain.ProductViewRepository {
	return &productViewRepository{pool: pool}
}

func (r *productViewRepository) Insert(ctx context.Context, id, sellerID, productID int64, source string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.InsertProductViewLog(ctx, gen.InsertProductViewLogParams{
		ID:        id,
		SellerID:  sellerID,
		ProductID: productID,
		Source:    source,
	})
}

func (r *productViewRepository) CountByDateRange(ctx context.Context, sellerID int64, from, to time.Time) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountProductViewsBySeller(ctx, gen.CountProductViewsBySellerParams{
		SellerID:   sellerID,
		ViewedAt:   timestamptzParam(from),
		ViewedAt_2: timestamptzParam(to),
	})
}
