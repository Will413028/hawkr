//go:build integration

package postgres_test

import (
	"context"
	"testing"

	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/stretchr/testify/require"
)

func TestMarketWatchSellerCleanupIsAccountScopedAndIdempotent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	crop := mustUpsertCrop(ctx, t, q, "C", "crop")
	market := mustUpsertMarket(ctx, t, q, "M", "market")
	watch := pbpostgres.NewMarketWatchRepository(pool)
	for i, target := range []struct {
		account int64
		market  string
	}{{777, ""}, {777, "M"}, {100, ""}, {888, "M"}} {
		_, err := watch.Create(ctx, target.account, int64(9000+i), "C", target.market)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, "UPDATE crop SET active=false WHERE id=$1", crop)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE market SET active=false WHERE id=$1", market)
	require.NoError(t, err)
	repo := pbpostgres.NewMarketWatchCleanupRepository(pool)
	for range 2 {
		require.NoError(t, repo.DeleteByAccountID(ctx, 777))
		var accounts []int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT array_agg(account_id ORDER BY account_id) FROM market_watch_target").Scan(&accounts))
		require.Equal(t, []int64{100, 888}, accounts, "store_id-like other account and unrelated seller survive replay")
	}
	require.NoError(t, repo.DeleteByAccountID(ctx, 999))
	for _, id := range []int64{0, -1} {
		require.Error(t, repo.DeleteByAccountID(ctx, id))
	}
	// A real SQL failure must propagate; it cannot become an ACK on the worker.
	_, err = pool.Exec(ctx, "ALTER TABLE market_watch_target RENAME TO unavailable_market_watch_target")
	require.NoError(t, err)
	require.ErrorContains(t, repo.DeleteByAccountID(ctx, 777), "delete seller market watches")
}
