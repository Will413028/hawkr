//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestListTaxIDMatchesTrimsCertificationTaxID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	taxID := "12345678"
	p := newTestSellerProspect("已入駐但沒連結", "食品", "台南")
	p.TaxID = &taxID
	created, err := repo.Create(ctx, p)
	require.NoError(t, err)

	const accountID, storeID, certID = int64(880000001), int64(880000002), int64(880000003)
	_, err = pool.Exec(ctx,
		`INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name)
		 VALUES ($1, $2, $3, true, '對帳測試店')`, storeID, storeID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO certification_applications (id, account_id, application_no, status, seller_type, tax_id, company_name)
		 VALUES ($1, $2, 'APP-RECON-1', 'forwarded', 'business', '12345678 ', '對帳測試公司')`,
		certID, accountID)
	require.NoError(t, err)

	matches, err := repo.ListTaxIDMatches(ctx)
	require.NoError(t, err)
	require.Len(t, matches, 1, "尾隨空白的統編必須 btrim 後仍比對得上")
	require.Equal(t, created.ID, matches[0].Prospect.ID)
	require.Equal(t, storeID, matches[0].StoreID)
}

func TestCurrentStoreExists(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	const storeID = int64(880000010)
	_, err := pool.Exec(ctx,
		`INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name)
		 VALUES ($1, $1, 880000011, true, '存在的店')`, storeID)
	require.NoError(t, err)

	ok, err := repo.CurrentStoreExists(ctx, storeID)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = repo.CurrentStoreExists(ctx, 880000999)
	require.NoError(t, err)
	require.False(t, ok, "不存在的 store 必須回 false，不能讓歸因指向不存在的店")
}
