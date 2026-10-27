package usecase

import (
	"context"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

// GetCategoryPriceRollup serves the read side of category_price_rollup —
// a thin wrapper, same shape as ResolveCategoryMapping (Get), because the
// business logic already lives entirely in the daily batch (RollupBatch);
// this usecase never computes a price itself.
type GetCategoryPriceRollup struct {
	repo domain.CategoryPriceRollupRepository
}

func NewGetCategoryPriceRollup(repo domain.CategoryPriceRollupRepository) *GetCategoryPriceRollup {
	return &GetCategoryPriceRollup{repo: repo}
}

// Get returns nil, nil when no row exists yet — the caller (gRPC handler)
// maps that to the same "unavailable" business outcome as a computed
// unavailable row, not a 404/error: from a client's perspective "never
// computed" and "computed as unavailable" are indistinguishable and don't
// need to be.
func (u *GetCategoryPriceRollup) Get(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
	return u.repo.Get(ctx, categoryID)
}
