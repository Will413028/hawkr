//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// Finding B (final whole-branch review): SellerUseCase.Create's collision
// retry depends on the repository translating a REAL Postgres 23505
// unique-violation on uq_sellers_store_code into
// *domain.ErrDuplicate{ConstraintName: "uq_sellers_store_code"} — a load-
// bearing string literal compared exactly in the usecase's retry condition.
// Prior tests only mocked this error; none actually triggered a real DB-level
// collision. This proves the real translation still lines up.
func TestCreateSeller_StoreCodeCollision_ReturnsErrDuplicateWithConstraintName(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerRepository(pool)

	const firstAccountID, firstStoreID, storeCode int64 = 5100, 5100, 2699999001
	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  firstAccountID,
		StoreID:    firstStoreID,
		StoreCode:  storeCode,
		StoreName:  "First Store",
		SellerType: domain.SellerTypeIndividual,
	}))

	const secondAccountID, secondStoreID int64 = 5200, 5200
	err := repo.Create(ctx, &domain.Seller{
		AccountID:  secondAccountID,
		StoreID:    secondStoreID,
		StoreCode:  storeCode, // same store_code as the first seller: must collide
		StoreName:  "Second Store",
		SellerType: domain.SellerTypeIndividual,
	})

	require.Error(t, err, "a second seller with the same store_code must be rejected")
	var dupErr *domain.ErrDuplicate
	require.True(t, errors.As(err, &dupErr),
		"error must be a *domain.ErrDuplicate (repository must translate the real 23505), got %T: %v", err, err)
	require.Equal(t, "uq_sellers_store_code", dupErr.ConstraintName,
		"ConstraintName must be uq_sellers_store_code — this exact string is compared in SellerUseCase.Create's retry condition")
}
