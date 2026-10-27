//go:build integration

// trend_summary_integration_test.go covers the issue #2249 Slice B test
// matrix items unit tests with fakes can't exercise: real Postgres
// date-range queries, real week-boundary/cross-year bucketing against
// seeded data, and the real market-closed-vs-insufficient-data distinction
// driven by real market_rest_day rows (TrendSummary.cropPath has no
// injectable repository — see NewTrendSummary's dependency on
// domain.ForecastRepository, satisfied here by the real
// postgres.NewForecastRepository, same convention as this package's other
// *_integration_test.go files, see dbtest_test.go's header comment).
//
// Same //go:build integration + package usecase_test file group as
// dbtest_test.go/export_integration_test.go/batch_query_integration_test.go
// — reuses setupTestDB/newSeqTSID/mustNumeric from that group rather than
// redeclaring them.
package usecase_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// ==================== seed helpers ====================
//
// None of these reuse a name from internal/repository/postgres's
// observed_price_repo_test.go (mustUpsertMarket/mustUpsertCrop/...) — that
// file is package postgres_test, a different test package from this file's
// package usecase_test (see export_integration_test.go's identical caveat
// for exportSeedSeries). They also don't collide with this package's
// existing seedMarketAndCrops (batch_query_integration_test.go): that
// helper bundles market+group+crops with a hardcoded category for
// LatestObservationRevisions-style tests; trend-summary's golden cases need
// each entity seeded independently (a lone crop with a caller-chosen
// category_code, rest days on specific dates, a coverage-only anchor row
// with no accompanying price data) so they get their own, smaller helpers.

// seedMarket creates one market row, returns its id.
func seedMarket(ctx context.Context, t *testing.T, q *gen.Queries, tsid func() int64, marketCode string) int64 {
	t.Helper()
	marketID, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{ID: tsid(), MarketCode: marketCode, Name: "市場-" + marketCode})
	require.NoError(t, err)
	return marketID
}

// seedCrop creates a crop (with its own dedicated crop_group, mirroring
// mustUpsertCrop's convention in observed_price_repo_test.go — these
// golden cases don't care about crop_group sharing) carrying the given
// category_code, returns the crop id.
func seedCrop(ctx context.Context, t *testing.T, q *gen.Queries, tsid func() int64, cropCode, name, categoryCode string) int64 {
	t.Helper()
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "trend-grp-" + cropCode, Name: name,
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)
	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID: tsid(), CropCode: cropCode, GroupID: groupID, Name: name,
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)
	return cropID
}

// seedObservedPrice inserts one price_observation row. available_at is
// pinned to tradeDate+1h (same convention as observed_price_repo_test.go's
// mustInsertObservation) — comfortably before every golden case's `now`.
func seedObservedPrice(ctx context.Context, t *testing.T, q *gen.Queries, tsid func() int64, marketID, cropID int64, tradeDate string, avgPrice, volumeKg float64) {
	t.Helper()
	d, err := time.Parse("2006-01-02", tradeDate)
	require.NoError(t, err)
	price := mustNumeric(t, strconv.FormatFloat(avgPrice, 'f', -1, 64))
	vol := mustNumeric(t, strconv.FormatFloat(volumeKg, 'f', -1, 64))
	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID:           tsid(),
		MarketID:     marketID,
		CropID:       cropID,
		TradeDate:    pgxutil.DateParam(&d),
		RevisionNo:   1,
		AvailableAt:  pgtype.Timestamptz{Time: d.Add(1 * time.Hour), Valid: true},
		AvgPrice:     price,
		UpperPrice:   price,
		MiddlePrice:  price,
		LowerPrice:   price,
		VolumeKg:     vol,
		Source:       "test",
		SourceDigest: "digest-" + tradeDate,
	}))
}

// seedRestDay declares one market_rest_day row: marketCode is closed for
// marketType on restDate.
func seedRestDay(ctx context.Context, t *testing.T, q *gen.Queries, tsid func() int64, marketID int64, marketType, restDate string) {
	t.Helper()
	d, err := time.Parse("2006-01-02", restDate)
	require.NoError(t, err)
	restDayType := marketType
	require.NoError(t, q.UpsertMarketRestDay(ctx, gen.UpsertMarketRestDayParams{
		ID:         tsid(),
		MarketID:   marketID,
		MarketType: &restDayType,
		RestDate:   pgxutil.DateParam(&d),
		Reason:     nil,
		FetchedAt:  pgtype.Timestamptz{Time: d, Valid: true},
	}))
}

// seedRestDayCoverage establishes "we have a verified rest-day calendar for
// (marketCode, marketType, year)" WITHOUT closing any date the golden cases
// actually query. RestDayCoverageForMarket (forecast.sql) has no separate
// coverage table — coverage is derived as DISTINCT (market_type, year) over
// existing market_rest_day rows (see that query's own doc comment: "該
// (市場, 類別, 年) 至少有一筆休市列"). So this inserts one closure on
// January 1st of `year` — a date outside every golden case's query window
// (all 2026-07/08, YoY baseline 2025-07/08) — purely to make that DISTINCT
// query return a row for `year`, distinct in intent from seedRestDay
// (which declares real closures the assertions care about).
func seedRestDayCoverage(ctx context.Context, t *testing.T, q *gen.Queries, tsid func() int64, marketID int64, marketType string, year int) {
	t.Helper()
	anchor := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	restDayType := marketType
	reason := "coverage-anchor"
	require.NoError(t, q.UpsertMarketRestDay(ctx, gen.UpsertMarketRestDayParams{
		ID:         tsid(),
		MarketID:   marketID,
		MarketType: &restDayType,
		RestDate:   pgxutil.DateParam(&anchor),
		Reason:     &reason,
		FetchedAt:  pgtype.Timestamptz{Time: anchor, Valid: true},
	}))
}

// newPostgresRepo wraps the real repository — the same domain.ForecastRepository
// TrendSummary depends on in production (see cmd/pricebook-service's fx wiring),
// not a fake.
func newPostgresRepo(pool *pgxpool.Pool) domain.ForecastRepository {
	return postgres.NewForecastRepository(pool)
}

// newForecastUC builds a real *usecase.Forecast against the same pool —
// TrendSummary's constructor requires one (group-path spatial weighting via
// ComputeGroupVolumes) even though none of these four golden cases exercise
// the group path.
func newForecastUC(pool *pgxpool.Pool) *usecase.Forecast {
	return usecase.NewForecast(newPostgresRepo(pool), testConfig(), zap.NewNop())
}

// testConfig mirrors forecast_test.go's defaultTestInference()/testConfig()
// (package usecase, unexported — unreachable from this package usecase_test)
// with the same values; nothing in these four golden cases depends on
// non-default thresholds.
func testConfig() *config.Config {
	return &config.Config{Inference: config.InferenceConfig{
		MinRidgeSampleCount: 100, FreshWithinHours: 36, StaleWithinHours: 72,
		GroupVolumeWindowDays: 28, ObservedWindowDays: 30,
		GroupMinMarketSupportCount: forecast.DefaultGroupMinMarketSupportCount,
	}}
}

// ==================== golden cases ====================

// TestTrendSummary_WoWEqualLengthPeriod_Integration: day granularity, one
// point 7 days apart from its WoW baseline — both equal-length (1 day)
// periods, straight subtraction against real seeded rows.
func TestTrendSummary_WoWEqualLengthPeriod_Integration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	tsid := newSeqTSID(971000000000000000)

	marketID := seedMarket(ctx, t, q, tsid, "109")
	cropID := seedCrop(ctx, t, q, tsid, "A1", "花胡瓜", "N04")
	seedObservedPrice(ctx, t, q, tsid, marketID, cropID, "2026-08-01", 42.0, 100)
	seedObservedPrice(ctx, t, q, tsid, marketID, cropID, "2026-07-25", 40.0, 100)

	uc := usecase.NewTrendSummary(newPostgresRepo(pool), newForecastUC(pool), testConfig(), zap.NewNop())
	result, err := uc.Get(ctx, "A1", "", "109",
		forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay,
		[]forecast.TrendComparisonKind{forecast.ComparisonWoW}, time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	wow, ok := result.Comparisons[forecast.ComparisonWoW]
	require.True(t, ok, "want a wow comparison entry")
	require.NotNil(t, wow.ChangeAmount)
	require.Equal(t, 2.0, *wow.ChangeAmount, "42 (8/1) - 40 (7/25, 7 days earlier) = 2")
}

// TestTrendSummary_YoY364WeekAligned_Integration: week granularity, current
// week 2026-07-27..08-02 against its baseline shifted back exactly 364 days
// (52*7 — always lands on the same weekday, unlike a naive "same
// month/day last year" rule that would drift off week boundaries).
func TestTrendSummary_YoY364WeekAligned_Integration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	tsid := newSeqTSID(972000000000000000)

	marketID := seedMarket(ctx, t, q, tsid, "109")
	cropID := seedCrop(ctx, t, q, tsid, "A1", "花胡瓜", "N04")
	seedObservedPrice(ctx, t, q, tsid, marketID, cropID, "2026-08-01", 42.0, 100) // Saturday, in week 2026-07-27..08-02
	seedObservedPrice(ctx, t, q, tsid, marketID, cropID, "2025-08-02", 38.0, 100) // Saturday exactly 364 days earlier, same week-position

	uc := usecase.NewTrendSummary(newPostgresRepo(pool), newForecastUC(pool), testConfig(), zap.NewNop())
	result, err := uc.Get(ctx, "A1", "", "109",
		forecast.NewDate(2026, 7, 27), forecast.NewDate(2026, 8, 2), forecast.GranularityWeek,
		[]forecast.TrendComparisonKind{forecast.ComparisonYoY364}, time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	yoy, ok := result.Comparisons[forecast.ComparisonYoY364]
	require.True(t, ok, "want a yoy364 comparison entry")
	require.NotNil(t, yoy.BaselineValue, "364-day-aligned baseline row must be found")
	require.Equal(t, 38.0, *yoy.BaselineValue)
	require.NotNil(t, yoy.CurrentValue)
	require.Equal(t, 42.0, *yoy.CurrentValue)
}

// TestTrendSummary_MarketClosedWeekVsInsufficientData_Integration: every
// day of the week 2026-08-03..08-09 is a declared rest day for market 109
// category N04 (market_type "V"), and there is ZERO price_observation data
// for that week — this must resolve to market_closed, not
// insufficient_data. price_observation carries no closed-market sentinel
// rows (NewForecastRepository's own doc comment: closure info lives
// entirely in market_rest_day), so this is also the scenario that requires
// TrendSummary.cropPath to resolve categoryCode independently of any
// observed row (via the new domain.ForecastRepository.CropByCode this task
// added — see trend_summary.go's cropPath doc comment).
func TestTrendSummary_MarketClosedWeekVsInsufficientData_Integration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	tsid := newSeqTSID(973000000000000000)

	marketID := seedMarket(ctx, t, q, tsid, "109")
	seedCrop(ctx, t, q, tsid, "A1", "花胡瓜", "N04")
	for _, d := range []string{"2026-08-03", "2026-08-04", "2026-08-05", "2026-08-06", "2026-08-07", "2026-08-08", "2026-08-09"} {
		seedRestDay(ctx, t, q, tsid, marketID, "V", d)
	}
	seedRestDayCoverage(ctx, t, q, tsid, marketID, "V", 2026)

	uc := usecase.NewTrendSummary(newPostgresRepo(pool), newForecastUC(pool), testConfig(), zap.NewNop())
	result, err := uc.Get(ctx, "A1", "", "109",
		forecast.NewDate(2026, 8, 3), forecast.NewDate(2026, 8, 9), forecast.GranularityWeek, nil,
		time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	require.Len(t, result.Points, 1)
	require.Equal(t, forecast.PredictionStatusMarketClosed, result.Points[0].Status,
		"a week with zero data AND a full set of declared rest days must read as market_closed, not insufficient_data")
}

// TestTrendSummary_PartialCurrentWeekNotPresentedAsComplete_Integration:
// the calendar is verified (coverage seeded) and nothing is closed — every
// weekday of 2026-08-03..08-09 is an expected trading day — but only
// 08-03/08-04 have observed rows; 08-05..08-09 deliberately have none (a
// week still "in progress" as of `now`=2026-08-05). Must read as partial,
// not ready: a partial week must never be presented as a complete one.
func TestTrendSummary_PartialCurrentWeekNotPresentedAsComplete_Integration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	tsid := newSeqTSID(974000000000000000)

	marketID := seedMarket(ctx, t, q, tsid, "109")
	cropID := seedCrop(ctx, t, q, tsid, "A1", "花胡瓜", "N04")
	seedRestDayCoverage(ctx, t, q, tsid, marketID, "V", 2026) // verified, nothing closed — every weekday expected
	seedObservedPrice(ctx, t, q, tsid, marketID, cropID, "2026-08-03", 40.0, 100)
	seedObservedPrice(ctx, t, q, tsid, marketID, cropID, "2026-08-04", 41.0, 100)
	// 08-05..08-09 deliberately have no rows — a week still "in progress".

	uc := usecase.NewTrendSummary(newPostgresRepo(pool), newForecastUC(pool), testConfig(), zap.NewNop())
	result, err := uc.Get(ctx, "A1", "", "109",
		forecast.NewDate(2026, 8, 3), forecast.NewDate(2026, 8, 9), forecast.GranularityWeek, nil,
		time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	require.Len(t, result.Points, 1)
	require.Equal(t, forecast.PredictionStatusPartial, result.Points[0].Status,
		"2 of 7 expected days present — must read as partial, not ready (the week isn't over) and not insufficient_data (some real data exists)")
}

// TestTrendSummary_GroupPathMarketClosedWeekVsInsufficientData_Integration
// mirrors TestTrendSummary_MarketClosedWeekVsInsufficientData_Integration
// (crop path, above) for the GROUP path — I1 (final review, 2026-08-08)
// found groupPath built its marketSet/marketIDs/rawMarketCodes ONLY from
// observation rows that survive the active-crop+non-nil-price filter. When
// a group's markets are genuinely closed for the whole window (Lunar New
// Year, typhoon closures), zero rows survive that filter, marketSet ends up
// empty, calendarFor makes zero repo calls, ExpectedTradingDay returns
// unverified for every date, and PeriodStatus can only ever report
// insufficient_data — market_closed becomes structurally unreachable. Same
// defect class Task 11 already root-caused and fixed on the crop path (via
// CropByCode), now confirmed and fixed on its sibling.
//
// The fix seeds marketSet from ComputeGroupVolumes' vols.ByMarket, which is
// sourced from a `now`-relative 28-day recent-volume window — NOT the
// historical [from,to] being queried. So this test seeds a price
// observation OUTSIDE the closed query window (2026-07-15, within 28 days
// of `now`=2026-08-10) purely to give ComputeGroupVolumes something to find
// market 109 with; the query window itself (2026-08-03..08-09) has zero
// price rows, same as the crop-path golden case.
func TestTrendSummary_GroupPathMarketClosedWeekVsInsufficientData_Integration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	tsid := newSeqTSID(975000000000000000)

	marketID := seedMarket(ctx, t, q, tsid, "109")
	cropID := seedCrop(ctx, t, q, tsid, "GA1", "花胡瓜", "N04") // seedCrop creates its own dedicated group "trend-grp-GA1"

	// Outside the closed query window (2026-08-03..08-09) but inside
	// ComputeGroupVolumes' `now`-relative 28-day lookback (now=2026-08-10 ->
	// volFrom=2026-07-13) — this is what makes vols.ByMarket carry market
	// 109 even though the query window itself has zero rows.
	seedObservedPrice(ctx, t, q, tsid, marketID, cropID, "2026-07-15", 35.0, 50)

	for _, d := range []string{"2026-08-03", "2026-08-04", "2026-08-05", "2026-08-06", "2026-08-07", "2026-08-08", "2026-08-09"} {
		seedRestDay(ctx, t, q, tsid, marketID, "V", d)
	}
	seedRestDayCoverage(ctx, t, q, tsid, marketID, "V", 2026)

	uc := usecase.NewTrendSummary(newPostgresRepo(pool), newForecastUC(pool), testConfig(), zap.NewNop())
	result, err := uc.Get(ctx, "", "trend-grp-GA1", "",
		forecast.NewDate(2026, 8, 3), forecast.NewDate(2026, 8, 9), forecast.GranularityWeek, nil,
		time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	require.Len(t, result.Points, 1)
	require.Equal(t, forecast.PredictionStatusMarketClosed, result.Points[0].Status,
		"a group week with zero observation rows AND a full set of declared rest days must read as market_closed, not insufficient_data — mirrors the crop-path fix, on the sibling groupPath code path")
}
