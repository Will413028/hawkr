package postgres

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type marketWatchCleanupRepo struct{ pool *pgxpool.Pool }

func NewMarketWatchCleanupRepository(pool *pgxpool.Pool) domain.MarketWatchCleanupRepository {
	return &marketWatchCleanupRepo{pool: pool}
}

func (r *marketWatchCleanupRepo) DeleteByAccountID(ctx context.Context, accountID int64) error {
	if accountID <= 0 {
		return fmt.Errorf("delete seller market watches: account ID must be positive")
	}
	if err := gen.New(r.pool).DeleteMarketWatchTargetsByAccountID(ctx, accountID); err != nil {
		return fmt.Errorf("delete seller market watches: %w", err)
	}
	return nil
}
