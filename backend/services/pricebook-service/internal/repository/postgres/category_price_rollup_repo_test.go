//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
)

func TestCategoryPriceRollupRepo_UpsertThenGet_RoundTrips(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := pbpostgres.NewCategoryPriceRollupRepository(pool)

	latest := 88.1234
	prev := 85.5
	computedAt := time.Date(2026, 8, 30, 3, 0, 0, 0, time.UTC)
	points := []domain.RollupPricePoint{
		{TradeDate: "2026-08-28", AvgPrice: 84.0},
		{TradeDate: "2026-08-29", AvgPrice: 85.5},
		{TradeDate: "2026-08-30", AvgPrice: 88.1234},
	}

	want := domain.CategoryPriceRollup{
		CategoryID:       "cpr-roundtrip",
		Level:            3,
		Status:           domain.RollupStatusReady,
		PriceLayer:       &testRollupLayer,
		Unit:             &testRollupUnit,
		LatestPrice:      &latest,
		PrevPrice:        &prev,
		Points:           points,
		SourceGroupCount: 4,
		ComputedAt:       computedAt,
	}
	require.NoError(t, repo.Upsert(ctx, want))

	got, err := repo.Get(ctx, "cpr-roundtrip")
	require.NoError(t, err)
	require.NotNil(t, got)

	require.Equal(t, want.CategoryID, got.CategoryID)
	require.Equal(t, want.Level, got.Level)
	require.Equal(t, want.Status, got.Status)
	require.NotNil(t, got.LatestPrice)
	require.InDelta(t, *want.LatestPrice, *got.LatestPrice, 0.0001)
	require.NotNil(t, got.PrevPrice)
	require.InDelta(t, *want.PrevPrice, *got.PrevPrice, 0.0001)
	require.Equal(t, want.Points, got.Points)
	require.Equal(t, want.SourceGroupCount, got.SourceGroupCount)
	require.True(t, want.ComputedAt.Equal(got.ComputedAt), "ComputedAt = %v, want %v", got.ComputedAt, want.ComputedAt)
}

func TestCategoryPriceRollupRepo_UnavailableStatus_ReturnsNilsNotZeroValues(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := pbpostgres.NewCategoryPriceRollupRepository(pool)

	computedAt := time.Date(2026, 8, 30, 3, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Upsert(ctx, domain.CategoryPriceRollup{
		CategoryID:       "cpr-unavailable",
		Level:            1,
		Status:           domain.RollupStatusUnavailable,
		LatestPrice:      nil,
		PrevPrice:        nil,
		Points:           nil,
		SourceGroupCount: 0,
		ComputedAt:       computedAt,
	}))

	got, err := repo.Get(ctx, "cpr-unavailable")
	require.NoError(t, err)
	require.NotNil(t, got)

	require.Equal(t, domain.RollupStatusUnavailable, got.Status)
	require.Nil(t, got.LatestPrice, "unavailable 快照的 latest_price 必須是 nil，不能是 0")
	require.Nil(t, got.PrevPrice, "unavailable 快照的 prev_price 必須是 nil，不能是 0")
	require.Empty(t, got.Points, "unavailable 快照沒有 points，必須是空值而非零值 slice")
}

func TestCategoryPriceRollupRepo_UpsertGet_ForecastPointsRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := pbpostgres.NewCategoryPriceRollupRepository(pool)

	p50 := 51.3
	forecastPoints := []domain.RollupForecastPoint{
		{HorizonDays: 1, TargetDate: "2026-09-01", P50: &p50},
	}
	err := repo.Upsert(ctx, domain.CategoryPriceRollup{
		CategoryID:       "test-forecast-1",
		Level:            3,
		Status:           domain.RollupStatusReady,
		PriceLayer:       &testRollupLayer,
		Unit:             &testRollupUnit,
		ForecastPoints:   forecastPoints,
		SourceGroupCount: 1,
		ComputedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)

	got, err := repo.Get(ctx, "test-forecast-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	require.Len(t, got.ForecastPoints, 1)
	require.Equal(t, 1, got.ForecastPoints[0].HorizonDays)
	require.NotNil(t, got.ForecastPoints[0].P50)
	require.InDelta(t, 51.3, *got.ForecastPoints[0].P50, 0.0001)
}

func TestCategoryPriceRollupRepo_Upsert_OverwritesExistingRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := pbpostgres.NewCategoryPriceRollupRepository(pool)

	firstComputedAt := time.Date(2026, 8, 29, 3, 0, 0, 0, time.UTC)
	firstLatest := 50.0
	require.NoError(t, repo.Upsert(ctx, domain.CategoryPriceRollup{
		CategoryID:       "cpr-overwrite",
		Level:            2,
		Status:           domain.RollupStatusReady,
		PriceLayer:       &testRollupLayer,
		Unit:             &testRollupUnit,
		LatestPrice:      &firstLatest,
		SourceGroupCount: 2,
		ComputedAt:       firstComputedAt,
	}))

	secondComputedAt := time.Date(2026, 8, 30, 3, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Upsert(ctx, domain.CategoryPriceRollup{
		CategoryID:       "cpr-overwrite",
		Level:            2,
		Status:           domain.RollupStatusUnavailable,
		LatestPrice:      nil,
		SourceGroupCount: 0,
		ComputedAt:       secondComputedAt,
	}))

	got, err := repo.Get(ctx, "cpr-overwrite")
	require.NoError(t, err)
	require.NotNil(t, got)

	require.Equal(t, domain.RollupStatusUnavailable, got.Status, "第二次寫入必須覆蓋第一次的 status，證明是 ON CONFLICT DO UPDATE 而非 DO NOTHING")
	require.Nil(t, got.LatestPrice)
	require.Equal(t, int16(0), got.SourceGroupCount)
	require.True(t, secondComputedAt.Equal(got.ComputedAt), "ComputedAt = %v, want %v", got.ComputedAt, secondComputedAt)
}

var (
	testRollupLayer = domain.PriceLayerWholesaleAuction
	testRollupUnit  = domain.UnitKilogram
)
