package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type marketWatchRepo struct{ pool *pgxpool.Pool }

func NewMarketWatchRepository(pool *pgxpool.Pool) domain.MarketWatchRepository {
	return &marketWatchRepo{pool: pool}
}

func (r *marketWatchRepo) Create(ctx context.Context, accountID, newID int64, cropCode, marketCode string) (domain.MarketWatchTargetResult, error) {
	result := domain.MarketWatchTargetResult{}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return result, fmt.Errorf("begin market watch: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := gen.New(tx)
	// READ COMMITTED refreshes the statement snapshot after waiting for the
	// account lock, so a concurrent successful create is visible below.
	if err = q.LockMarketWatchAccount(ctx, accountID); err != nil {
		return result, fmt.Errorf("lock market watch account: %w", err)
	}
	existing, err := q.FindMarketWatchTarget(ctx, gen.FindMarketWatchTargetParams{AccountID: accountID, CropCode: cropCode, MarketCode: marketCode})
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return result, fmt.Errorf("commit existing market watch: %w", err)
		}
		return domain.MarketWatchTargetResult{WatchTargetID: existing}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, fmt.Errorf("find market watch: %w", err)
	}
	refs, err := q.ResolveMarketWatchReferences(ctx, gen.ResolveMarketWatchReferencesParams{CropCode: cropCode, MarketCode: marketCode})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, domain.ErrInvalidWatchTarget
	}
	if err != nil {
		return result, fmt.Errorf("resolve market watch: %w", err)
	}
	count, err := q.CountMarketWatchTargets(ctx, accountID)
	if err != nil {
		return result, fmt.Errorf("count market watches: %w", err)
	}
	if count >= domain.MarketWatchTargetLimit {
		return result, domain.ErrWatchLimit
	}
	if err = q.InsertMarketWatchTarget(ctx, gen.InsertMarketWatchTargetParams{ID: newID, AccountID: accountID, CropID: refs.CropID, MarketID: refs.MarketID}); err != nil {
		return result, fmt.Errorf("insert market watch: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit market watch: %w", err)
	}
	return domain.MarketWatchTargetResult{WatchTargetID: newID, Created: true}, nil
}

func (r *marketWatchRepo) Delete(ctx context.Context, accountID, watchID int64) error {
	if err := gen.New(r.pool).DeleteMarketWatchTarget(ctx, gen.DeleteMarketWatchTargetParams{AccountID: accountID, ID: watchID}); err != nil {
		return fmt.Errorf("delete market watch: %w", err)
	}
	return nil
}
