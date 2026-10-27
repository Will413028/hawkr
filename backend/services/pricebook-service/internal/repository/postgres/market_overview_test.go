//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func overviewQuery() domain.MarketOverviewQuery {
	return domain.MarketOverviewQuery{Scope: codemeta.PricebookOverviewScopeAll.Value, WindowDays: 7, Page: 1, PageSize: 20}
}

func overviewObservation(t *testing.T, pool *pgxpool.Pool, crop, market int64, day string, revision int32, price, volume *string, available time.Time, digest string) {
	t.Helper()
	date := mustParseDate(t, day)
	p := gen.InsertPriceObservationParams{ID: opNextID(), CropID: crop, MarketID: market, TradeDate: pgxutil.DateParam(&date), RevisionNo: revision, AvailableAt: pgxutil.TimestamptzParam(available), Source: "MOA", SourceDigest: digest}
	var err error
	if price != nil {
		p.AvgPrice, err = pbpostgres.NumericFromString(*price)
		require.NoError(t, err)
	}
	if volume != nil {
		p.VolumeKg, err = pbpostgres.NumericFromString(*volume)
		require.NoError(t, err)
	}
	require.NoError(t, gen.New(pool).InsertPriceObservation(context.Background(), p))
}

func TestMarketOverview_MixedIdentitiesNumericCalendarAndInactive(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	crop := mustUpsertCrop(ctx, t, q, "C", "白菜")
	_ = mustUpsertCrop(ctx, t, q, "D", "空資料")
	a := mustUpsertMarket(ctx, t, q, "A", "甲")
	b := mustUpsertMarket(ctx, t, q, "B", "乙")
	z := mustUpsertMarket(ctx, t, q, "Z", "零量")
	cutoff := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)
	str := func(v string) *string { return &v }
	// Previous actual is outside the seven-day chart window; latest market A
	// revision wins, and future revision/date cannot leak through the snapshot.
	for _, m := range []int64{a, b} {
		overviewObservation(t, pool, crop, m, "2026-08-01", 1, str("0"), nil, cutoff.Add(-time.Hour), fmt.Sprint(m))
	}
	overviewObservation(t, pool, crop, a, "2026-09-07", 1, str("9"), str("1"), cutoff.Add(-time.Hour), "old")
	overviewObservation(t, pool, crop, a, "2026-09-07", 2, str("10.0003"), str("1"), cutoff.Add(-time.Minute), "a")
	overviewObservation(t, pool, crop, a, "2026-09-07", 3, str("999"), str("1"), cutoff.Add(time.Hour), "future-revision")
	overviewObservation(t, pool, crop, b, "2026-09-07", 1, str("20.0001"), str("3"), cutoff.Add(-time.Hour), "b")
	overviewObservation(t, pool, crop, z, "2026-09-07", 1, str("999"), str("0"), cutoff, "excluded-zero")
	overviewObservation(t, pool, crop, a, "2026-09-09", 1, str("999"), str("1"), cutoff, "future-date")
	_, err := pool.Exec(ctx, "INSERT INTO market_watch_target (id,account_id,crop_id,market_id,created_at) VALUES (101,42,$1,NULL,$3),(102,42,$1,$2,$3),(103,99,$1,$2,$3)", crop, a, cutoff)
	require.NoError(t, err)
	repo := pbpostgres.NewMarketOverviewRepository(pool)
	query := overviewQuery()
	all, err := repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.EqualValues(t, 2, all.TotalItems)
	require.Len(t, all.Items, 2)
	require.Len(t, all.MarketOptions, 3)
	row := all.Items[0]
	require.Equal(t, "C", row.CropCode)
	require.EqualValues(t, 101, *row.WatchTargetID)
	require.Nil(t, row.MarketCode)
	require.Equal(t, "17.5001500000000000", row.LatestActual.AvgPrice)
	require.Equal(t, "4.000", *row.LatestActual.VolumeKg)
	require.Equal(t, "2026-08-01", row.PreviousActual.TradeDate.Format(time.DateOnly))
	require.Len(t, row.Points, 1)
	require.Nil(t, row.Change.Percent)
	require.Equal(t, codemeta.PricebookChangeDirectionUp.Value, row.Change.Direction)
	require.True(t, row.VolumeWeightFallback) // prior actual had no positive volume
	require.NotContains(t, row.SourceDigests, "old")
	require.NotContains(t, row.SourceDigests, "excluded-zero")
	require.NotContains(t, row.SourceDigests, "future-revision")
	require.Nil(t, all.Items[1].LatestActual)
	require.Empty(t, all.Items[1].Points)
	query.Scope = codemeta.PricebookOverviewScopeWatchlist.Value
	watch, err := repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.Len(t, watch.Items, 2)
	require.EqualValues(t, 102, *watch.Items[0].WatchTargetID)
	require.Equal(t, "A", *watch.Items[0].MarketCode)
	require.Equal(t, "10.0003000000000000", watch.Items[0].LatestActual.AvgPrice)
	require.Equal(t, "17.5001500000000000", watch.Items[1].LatestActual.AvgPrice)
	// Latest NULL revision excludes A instead of resurrecting its old price.
	overviewObservation(t, pool, crop, a, "2026-09-07", 4, nil, str("1"), cutoff, "null-correction")
	watch, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.Equal(t, "2026-08-01", watch.Items[0].LatestActual.TradeDate.Format(time.DateOnly))
	require.Nil(t, watch.Items[0].PreviousActual)
	require.Nil(t, watch.Items[0].Change)
	require.Empty(t, watch.Items[0].Points)
	require.Equal(t, "20.0001000000000000", watch.Items[1].LatestActual.AvgPrice)
	_, err = pool.Exec(ctx, "UPDATE market SET active=false WHERE id=$1", a)
	require.NoError(t, err)
	watch, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.False(t, watch.Items[0].Active)
	require.Equal(t, "A", *watch.Items[0].MarketCode)
	require.Nil(t, watch.Items[0].LatestActual)
	query = overviewQuery()
	query.MarketCode = "A"
	_, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.ErrorIs(t, err, domain.ErrInvalidWatchTarget)
	_, err = pool.Exec(ctx, "UPDATE crop SET active=false WHERE id=$1", crop)
	require.NoError(t, err)
	query = overviewQuery()
	query.Scope = codemeta.PricebookOverviewScopeWatchlist.Value
	watch, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.Len(t, watch.Items, 2)
	require.False(t, watch.Items[1].Active)
	require.Nil(t, watch.Items[1].LatestActual)
}

type overviewTracer struct {
	count           atomic.Int32
	afterCandidates func()
}

func (tr *overviewTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	tr.count.Add(1)
	return context.WithValue(ctx, overviewTraceKey{}, strings.Contains(data.SQL, "-- name: OverviewCandidates"))
}

type overviewTraceKey struct{}

func (tr *overviewTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if candidate, _ := ctx.Value(overviewTraceKey{}).(bool); candidate && tr.afterCandidates != nil {
		fn := tr.afterCandidates
		tr.afterCandidates = nil
		fn()
	}
}

func TestMarketOverview_FixedQueryCountAndRepeatableRead(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	var crop int64
	for i := 0; i < 100; i++ {
		id := mustUpsertCrop(ctx, t, q, fmt.Sprintf("C%03d", i), "crop")
		if i == 0 {
			crop = id
		}
	}
	market := mustUpsertMarket(ctx, t, q, "M", "market")
	cutoff := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)
	str := func(v string) *string { return &v }
	overviewObservation(t, pool, crop, market, "2026-09-07", 1, str("10"), str("1"), cutoff, "old")
	cfg := pool.Config()
	tr := &overviewTracer{}
	cfg.ConnConfig.Tracer = tr
	traced, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer traced.Close()
	repo := pbpostgres.NewMarketOverviewRepository(traced)
	var count int32
	for _, size := range []int{1, 20, 100} {
		query := overviewQuery()
		query.PageSize = size
		tr.count.Store(0)
		got, err := repo.LoadSnapshot(ctx, 42, query, cutoff)
		require.NoError(t, err)
		require.Len(t, got.Items, size)
		if count == 0 {
			count = tr.count.Load()
		} else {
			require.Equal(t, count, tr.count.Load())
		}
	}
	require.EqualValues(t, 5, count) // BEGIN + three batch reads + COMMIT
	tr.afterCandidates = func() {
		overviewObservation(t, pool, crop, market, "2026-09-07", 2, str("90"), str("1"), cutoff, "new")
	}
	got, err := repo.LoadSnapshot(ctx, 42, overviewQuery(), cutoff)
	require.NoError(t, err)
	require.Equal(t, "10.0000000000000000", got.Items[0].LatestActual.AvgPrice)
	next, err := repo.LoadSnapshot(ctx, 42, overviewQuery(), cutoff)
	require.NoError(t, err)
	require.Equal(t, "90.0000000000000000", next.Items[0].LatestActual.AvgPrice)
	query := overviewQuery()
	query.Page = 999
	got, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.EqualValues(t, 100, got.TotalItems)
	require.Empty(t, got.Items)
	query.Page = 1
	query.Query = "c000"
	got, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.EqualValues(t, 1, got.TotalItems)
	query.Query = "%"
	got, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.Zero(t, got.TotalItems)
}

func TestMarketOverview_PercentageAndExactMarketWatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	crop := mustUpsertCrop(ctx, t, q, "C", "crop")
	market := mustUpsertMarket(ctx, t, q, "M", "market")
	cutoff := time.Date(2026, 9, 7, 16, 0, 0, 0, time.UTC) // Taipei Sep 8
	str := func(v string) *string { return &v }
	overviewObservation(t, pool, crop, market, "2026-09-01", 1, str("20"), nil, cutoff, "outside")
	overviewObservation(t, pool, crop, market, "2026-09-02", 1, str("30"), nil, cutoff, "start")
	overviewObservation(t, pool, crop, market, "2026-09-08", 1, str("10"), nil, cutoff, "end")
	_, err := pool.Exec(ctx, "INSERT INTO market_watch_target(id,account_id,crop_id) VALUES (101,42,$1)", crop)
	require.NoError(t, err)
	query := overviewQuery()
	query.MarketCode = "M"
	repo := pbpostgres.NewMarketOverviewRepository(pool)
	got, err := repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	row := got.Items[0]
	require.False(t, row.IsWatched)
	require.Nil(t, row.WatchTargetID)
	require.Equal(t, "M", *row.MarketCode)
	require.Len(t, row.Points, 2)
	require.Equal(t, "2026-09-02", row.Points[0].TradeDate.Format(time.DateOnly))
	require.Equal(t, "2026-09-08", row.Points[1].TradeDate.Format(time.DateOnly))
	require.Equal(t, "-20.0000000000000000", row.Change.Absolute)
	require.Equal(t, "-66.66666666666666666700", *row.Change.Percent)
	require.Equal(t, codemeta.PricebookChangeDirectionDown.Value, row.Change.Direction)
	require.Nil(t, row.LatestActual.VolumeKg)
	require.NotContains(t, row.SourceDigests, "outside")
	overviewObservation(t, pool, crop, market, "2026-09-08", 2, str("30"), nil, cutoff, "flat")
	got, err = repo.LoadSnapshot(ctx, 42, query, cutoff)
	require.NoError(t, err)
	require.Equal(t, codemeta.PricebookChangeDirectionFlat.Value, got.Items[0].Change.Direction)
	query.Scope = codemeta.PricebookOverviewScopeWatchlist.Value
	query.MarketCode = ""
	got, err = repo.LoadSnapshot(ctx, 99, query, cutoff)
	require.NoError(t, err)
	require.Empty(t, got.Items)
	require.Zero(t, got.TotalItems)
}
