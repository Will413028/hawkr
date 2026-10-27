//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestMarketWatchIdentityAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	crop := mustUpsertCrop(ctx, t, q, "C", "crop")
	market := mustUpsertMarket(ctx, t, q, "M", "market")
	repo := pbpostgres.NewMarketWatchRepository(pool)
	first, err := repo.Create(ctx, 100, 1000, "C", "")
	require.NoError(t, err)
	require.True(t, first.Created)
	duplicate, err := repo.Create(ctx, 100, 1001, "C", "")
	require.NoError(t, err)
	require.Equal(t, first.WatchTargetID, duplicate.WatchTargetID)
	require.False(t, duplicate.Created)
	specific, err := repo.Create(ctx, 100, 1002, "C", "M")
	require.NoError(t, err)
	require.True(t, specific.Created)
	other, err := repo.Create(ctx, 200, 1003, "C", "")
	require.NoError(t, err)
	require.True(t, other.Created)
	for _, mid := range []any{nil, market} {
		_, err = pool.Exec(ctx, "INSERT INTO market_watch_target(id,account_id,crop_id,market_id) VALUES($1,100,$2,$3)", opNextID(), crop, mid)
		var pgerr *pgconn.PgError
		require.ErrorAs(t, err, &pgerr)
		require.Equal(t, "23505", pgerr.Code)
	}
	_, err = pool.Exec(ctx, "UPDATE crop SET active=false WHERE id=$1", crop)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE market SET active=false WHERE id=$1", market)
	require.NoError(t, err)
	duplicate, err = repo.Create(ctx, 100, 1004, "C", "M")
	require.NoError(t, err)
	require.Equal(t, specific.WatchTargetID, duplicate.WatchTargetID)
	require.False(t, duplicate.Created)
	_, err = repo.Create(ctx, 300, 1005, "C", "")
	require.ErrorIs(t, err, domain.ErrInvalidWatchTarget)
	require.NoError(t, repo.Delete(ctx, 200, first.WatchTargetID))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM market_watch_target WHERE id=$1", first.WatchTargetID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, repo.Delete(ctx, 100, first.WatchTargetID))
	require.NoError(t, repo.Delete(ctx, 100, first.WatchTargetID))
	require.NoError(t, repo.Delete(ctx, 100, 999999))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM market_watch_target WHERE account_id=200").Scan(&count))
	require.Equal(t, 1, count)
}

func TestMarketWatchConcurrentLimitAndDuplicateAtCapacity(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	repo := pbpostgres.NewMarketWatchRepository(pool)
	for i := 0; i < 21; i++ {
		mustUpsertCrop(ctx, t, q, fmt.Sprintf("C%02d", i), "crop")
	}
	for i := 0; i < 19; i++ {
		r, err := repo.Create(ctx, 100, int64(1000+i), fmt.Sprintf("C%02d", i), "")
		require.NoError(t, err)
		require.True(t, r.Created)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 19; i < 21; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := repo.Create(ctx, 100, int64(1000+i), fmt.Sprintf("C%02d", i), "")
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	success, limited := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, domain.ErrWatchLimit)
			limited++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, limited)
	duplicate, err := repo.Create(ctx, 100, 9000, "C00", "")
	require.NoError(t, err)
	require.False(t, duplicate.Created)
	require.EqualValues(t, 1000, duplicate.WatchTargetID)
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM market_watch_target WHERE account_id=100").Scan(&count))
	require.Equal(t, 20, count)
	_, err = repo.Create(ctx, 200, 9001, "C00", "")
	require.NoError(t, err)
}

func TestMarketWatchInvalidReferences(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	repo := pbpostgres.NewMarketWatchRepository(pool)
	mustUpsertCrop(ctx, t, q, "C", "crop")
	m := mustUpsertMarket(ctx, t, q, "M", "market")
	_, err := pool.Exec(ctx, "UPDATE market SET active=false WHERE id=$1", m)
	require.NoError(t, err)
	for _, target := range [][2]string{{"missing", ""}, {"C", "missing"}, {"C", "M"}} {
		_, err = repo.Create(ctx, 100, opNextID(), target[0], target[1])
		require.ErrorIs(t, err, domain.ErrInvalidWatchTarget)
	}
}
