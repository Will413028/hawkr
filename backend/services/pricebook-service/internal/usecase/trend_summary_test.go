package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// fakeTrendRepo implements the 5 domain.ForecastRepository methods
// TrendSummary actually calls; every other method panics if hit (via the
// nil-embedded interface in trendRepoAdapter below) — a hit means this
// usecase started depending on something new and the fake needs updating,
// same convention as fakeObservedPriceRepo/observedPriceRepoAdapter
// (observed_price_test.go).
type fakeTrendRepo struct {
	byCrop          func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error)
	byGroup         func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error)
	cropsInGroup    func(ctx context.Context, groupCode string) ([]domain.CropRef, error)
	restDays        func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error)
	restDayCoverage func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error)

	// cropByCode 供 cropPath 在觀測列讀不到 categoryCode 時的 fallback
	// （見 trend_summary.go cropPath 的說明）——沒有任何既有測試需要它
	// （nil 時呼叫會 panic，同本檔其餘欄位的「沒實作到就代表用到新東西」
	// 慣例），只有新加的 fallback 測試會設它。
	cropByCode func(ctx context.Context, cropCode string) (domain.CropRef, error)
}

func (f *fakeTrendRepo) ObservedPricesByCrop(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return f.byCrop(ctx, marketCode, cropCode, from, to, cutoff)
}
func (f *fakeTrendRepo) ObservedPricesByGroup(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return f.byGroup(ctx, groupCode, marketCode, from, to, cutoff)
}
func (f *fakeTrendRepo) CropsInGroup(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
	return f.cropsInGroup(ctx, groupCode)
}
func (f *fakeTrendRepo) RestDays(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
	return f.restDays(ctx, marketCode, from, to)
}
func (f *fakeTrendRepo) RestDayCoverage(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
	return f.restDayCoverage(ctx, marketCode)
}
func (f *fakeTrendRepo) CropByCode(ctx context.Context, cropCode string) (domain.CropRef, error) {
	return f.cropByCode(ctx, cropCode)
}

// trendRepoAdapter promotes fakeTrendRepo's 5 methods to a full
// domain.ForecastRepository by embedding the interface as nil for every
// other method — calling any of them panics, which is the point (same
// convention as observedPriceRepoAdapter, observed_price_test.go:39-64).
type trendRepoAdapter struct {
	domain.ForecastRepository
	fake *fakeTrendRepo
}

func (a *trendRepoAdapter) ObservedPricesByCrop(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return a.fake.ObservedPricesByCrop(ctx, marketCode, cropCode, from, to, cutoff)
}
func (a *trendRepoAdapter) ObservedPricesByGroup(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return a.fake.ObservedPricesByGroup(ctx, groupCode, marketCode, from, to, cutoff)
}
func (a *trendRepoAdapter) CropsInGroup(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
	return a.fake.CropsInGroup(ctx, groupCode)
}
func (a *trendRepoAdapter) RestDays(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
	return a.fake.RestDays(ctx, marketCode, from, to)
}
func (a *trendRepoAdapter) RestDayCoverage(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
	return a.fake.RestDayCoverage(ctx, marketCode)
}
func (a *trendRepoAdapter) CropByCode(ctx context.Context, cropCode string) (domain.CropRef, error) {
	return a.fake.CropByCode(ctx, cropCode)
}

func wrapFakeTrendRepo(f *fakeTrendRepo) domain.ForecastRepository {
	return &trendRepoAdapter{fake: f}
}

func TestTrendSummaryCropPathDayGranularity(t *testing.T) {
	cat := "N04"
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	fake := &fakeTrendRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", CategoryCode: &cat, TradeDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), AvgPrice: floatPtr(40), VolumeKg: floatPtr(100), Source: "MOA_WHOLESALE_DAILY", SourceDigest: "abc", AvailableAt: now, RevisionNo: 1},
			}, nil
		},
		restDays: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return nil, nil
		},
		restDayCoverage: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) { return nil, nil },
	}
	uc := NewTrendSummary(wrapFakeTrendRepo(fake), nil, testConfig(defaultTestInference()), zap.NewNop()) // forecastUC unused on the crop path
	result, err := uc.Get(context.Background(), "A1", "", "109", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("want 1 point, got %d", len(result.Points))
	}
	if result.Points[0].Value == nil || *result.Points[0].Value != 40 {
		t.Errorf("Points[0].Value = %v, want 40", result.Points[0].Value)
	}
	if result.SelectionKind != "crop" {
		t.Errorf("SelectionKind = %q, want crop", result.SelectionKind)
	}
	if result.Granularity != forecast.GranularityDay {
		t.Errorf("Granularity = %q, want %q (Get must echo the requested granularity onto the result, not leave it zero-valued)", result.Granularity, forecast.GranularityDay)
	}
}

func TestTrendSummaryRejectsBothCropAndGroup(t *testing.T) {
	uc := NewTrendSummary(wrapFakeTrendRepo(&fakeTrendRepo{}), nil, testConfig(defaultTestInference()), zap.NewNop())
	_, err := uc.Get(context.Background(), "A1", "N04", "109", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, time.Now())
	if err == nil {
		t.Fatal("want error when both cropCode and groupCode are set")
	}
}

func TestTrendSummaryWoWReadsBaselineWindow(t *testing.T) {
	cat := "N04"
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	var lastFrom time.Time
	fake := &fakeTrendRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			lastFrom = from
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", CategoryCode: &cat, TradeDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), AvgPrice: floatPtr(42), VolumeKg: floatPtr(100), Source: "MOA_WHOLESALE_DAILY", AvailableAt: now, RevisionNo: 1},
				{MarketCode: "109", CropCode: "A1", CategoryCode: &cat, TradeDate: time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC), AvgPrice: floatPtr(40), VolumeKg: floatPtr(100), Source: "MOA_WHOLESALE_DAILY", AvailableAt: now, RevisionNo: 1}, // 7 days before, the WoW baseline
			}, nil
		},
		restDays: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return nil, nil
		},
		restDayCoverage: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) { return nil, nil },
	}
	uc := NewTrendSummary(wrapFakeTrendRepo(fake), nil, testConfig(defaultTestInference()), zap.NewNop())
	result, err := uc.Get(context.Background(), "A1", "", "109", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, []forecast.TrendComparisonKind{forecast.ComparisonWoW}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wow, ok := result.Comparisons[forecast.ComparisonWoW]
	if !ok {
		t.Fatal("want a wow comparison entry")
	}
	if wow.ChangeAmount == nil || *wow.ChangeAmount != 2 {
		t.Errorf("wow.ChangeAmount = %v, want 2 (42 - 40)", wow.ChangeAmount)
	}
	// The repo's From span must have widened to include 7-25, not just the
	// primary [8-1, 8-1] window.
	if lastFrom.After(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("repo query From = %s did not widen to cover the WoW baseline date", lastFrom)
	}
}

// TestTrendSummaryCalendarForUsesRealWindowNotZeroTime pins the fix over the
// task brief's original illustrative calendarFor (which called
// u.repo.RestDays(ctx, marketCode, time.Time{}, time.Time{})): pgxutil's
// Date() maps a zero time.Time to pgtype.Date{Valid: false} (SQL NULL), and
// RestDaysForMarket's `rest_date BETWEEN sqlc.arg(from_date) AND
// sqlc.arg(to_date)` is unsatisfiable when either bound is NULL — a
// zero-time call would silently always return zero rest days, permanently
// disabling the calendar_closed / calendar_unverified distinction. This
// test asserts RestDays is actually called with the real (non-zero)
// wideFrom/wideTo window derived from the request.
func TestTrendSummaryCalendarForUsesRealWindowNotZeroTime(t *testing.T) {
	cat := "N04"
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	var gotFrom, gotTo time.Time
	fake := &fakeTrendRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", CategoryCode: &cat, TradeDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), AvgPrice: floatPtr(40), VolumeKg: floatPtr(100), AvailableAt: now, RevisionNo: 1},
			}, nil
		},
		restDays: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			gotFrom, gotTo = from, to
			return nil, nil
		},
		restDayCoverage: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) { return nil, nil },
	}
	uc := NewTrendSummary(wrapFakeTrendRepo(fake), nil, testConfig(defaultTestInference()), zap.NewNop())
	_, err := uc.Get(context.Background(), "A1", "", "109", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFrom.IsZero() || gotTo.IsZero() {
		t.Fatalf("RestDays called with from=%v to=%v, want both non-zero (a zero time.Time silently becomes SQL NULL and RestDaysForMarket's BETWEEN clause would then always return 0 rows)", gotFrom, gotTo)
	}
}

func TestTrendSummaryGroupPathAggregatesAndDataMetaExcludesInactiveCrop(t *testing.T) {
	activeDigest := repeatHexTest('4')
	activeAvailableAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	inactiveAvailableAt := time.Date(2026, 8, 1, 23, 0, 0, 0, time.UTC) // later, but must not leak in
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	d1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}, {CropCode: "crop2", Active: false}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			// crop2's row deliberately uses a DISTINCT MarketCode ("B", not
			// "A") from crop1's — if the contributingRows filter were ever
			// accidentally bypassed, ObservedMarketCount would come back as
			// 2 (both A and B), not 1, so that assertion below actually
			// discriminates the filter instead of trivially passing because
			// both rows happened to share one market code.
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1, AvgPrice: floatPtr(30), VolumeKg: floatPtr(1), SourceDigest: activeDigest, AvailableAt: activeAvailableAt},
				{MarketCode: "B", CropCode: "crop2", TradeDate: d1, AvgPrice: floatPtr(9999), VolumeKg: floatPtr(1), SourceDigest: repeatHexTest('9'), AvailableAt: inactiveAvailableAt},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "A", CropCode: "crop1", Volume: 1}}, nil
		},
		restDaysFn: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return nil, nil
		},
		restDayCoverageFn: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) { return nil, nil },
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	uc := NewTrendSummary(repo, forecastUC, testConfig(defaultTestInference()), zap.NewNop())

	result, err := uc.Get(context.Background(), "", "group1", "", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SelectionKind != "group" {
		t.Errorf("SelectionKind = %q, want group", result.SelectionKind)
	}
	if len(result.Points) != 1 || result.Points[0].Value == nil || *result.Points[0].Value != 30 {
		t.Fatalf("Points = %+v, want 1 point with Value=30 (inactive crop2's 9999 price must not contribute)", result.Points)
	}
	if result.Points[0].ObservedMarketCount != 1 {
		t.Errorf("ObservedMarketCount = %d, want 1 (only market A's active-crop row counts)", result.Points[0].ObservedMarketCount)
	}

	wantDigest := "sha256:" + activeDigest
	if result.DataMeta.DatasetDigest == nil || *result.DataMeta.DatasetDigest != wantDigest {
		t.Errorf("DataMeta.DatasetDigest = %v, want %q (only the active crop's contributing row, inactive crop2 must not leak in)", result.DataMeta.DatasetDigest, wantDigest)
	}
	if result.DataMeta.MaxFeatureAvailableAt == nil || !result.DataMeta.MaxFeatureAvailableAt.Equal(activeAvailableAt) {
		t.Errorf("DataMeta.MaxFeatureAvailableAt = %v, want %v (inactive crop2's later available_at must not leak in)", result.DataMeta.MaxFeatureAvailableAt, activeAvailableAt)
	}
}

// TestTrendSummaryGroupPathCategoryCodePrefersActiveCrop covers the fix
// review found: crop.category_code has no DB constraint tying it to its
// crop_group (schema.sql has no FK/check for this — the "one group, one
// category" invariant only holds because price_ingest.go sets both crop and
// crop_group category_code from the same upstream TcType in the same
// upsert, an application convention, not a guarantee). groupPath must
// therefore prefer an ACTIVE crop's CategoryCode over an inactive one it
// happened to see first in CropsInGroup's (unordered) result — picking the
// wrong category feeds ExpectedTradingDay/PeriodStatus with the wrong
// closure key, which can misreport a genuinely-closed period as ready or
// vice versa.
//
// Setup: CropsInGroup returns the INACTIVE crop2 (category N05) FIRST,
// followed by the ACTIVE crop1 (category N04) — proving the fix isn't
// "prefer whichever comes first" but actually keys off Active. Market A is
// declared closed via RestDays for market_type "V" (== category N04) only,
// not "F" (== category N05); RestDayCoverage verifies both categories so
// neither is calendar_unverified. crop1 (the only crop with observed data)
// has a price on 8/1 despite market A being closed that day for N04 — a
// data anomaly on purpose, so the assertion is driven entirely by which
// category the closure check uses, not by whether data exists.
//
//   - Correct categoryCode (N04, from active crop1): market A closed for N04
//     on 8/1 -> Status == market_closed.
//   - Wrong categoryCode (N05, from inactive crop2, no closure recorded for
//     N05): market A reads as open -> Status == ready instead.
func TestTrendSummaryGroupPathCategoryCodePrefersActiveCrop(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	d1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	inactiveCategory := "N05" // crop2, Active=false, listed FIRST
	activeCategory := "N04"   // crop1, Active=true, listed SECOND

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: "crop2", Active: false, CategoryCode: &inactiveCategory},
				{CropCode: "crop1", Active: true, CategoryCode: &activeCategory},
			}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1, AvgPrice: floatPtr(30), VolumeKg: floatPtr(1), AvailableAt: now},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "A", CropCode: "crop1", Volume: 1}}, nil
		},
		restDaysFn: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return []domain.RestDay{{MarketCode: "A", MarketType: "V", RestDate: d1}}, nil // "V" -> N04 only
		},
		restDayCoverageFn: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{
				{MarketCode: "A", MarketType: "V", Year: 2026}, // verifies N04
				{MarketCode: "A", MarketType: "F", Year: 2026}, // verifies N05 too, so a wrong pick isn't masked by calendar_unverified
			}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	uc := NewTrendSummary(repo, forecastUC, testConfig(defaultTestInference()), zap.NewNop())

	result, err := uc.Get(context.Background(), "", "group1", "", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("want 1 point, got %d", len(result.Points))
	}
	if result.Points[0].Status != forecast.PredictionStatusMarketClosed {
		t.Errorf("Points[0].Status = %q, want %q (categoryCode must resolve to active crop1's N04 — market A is closed on 8/1 under N04; picking inactive crop2's N05 instead would find no closure and misreport this period as ready)",
			result.Points[0].Status, forecast.PredictionStatusMarketClosed)
	}
}

// TestTrendSummaryCropPathFallsBackToCropByCodeWhenRowsCarryNoCategory pins
// the fix for the bug this task's integration test
// (TestTrendSummary_MarketClosedWeekVsInsufficientData_Integration,
// trend_summary_integration_test.go) caught against real Postgres: a fully
// closed market produces ZERO ObservedPricesByCrop rows (no closed-market
// sentinel rows exist — see NewForecastRepository's doc comment), so
// cropPath cannot derive categoryCode from `rows` the way it normally does.
// Without the CropByCode fallback, categoryCode stays nil,
// verified.IsVerified(..., nil, ...) is unconditionally false
// (calendar.go), every date reads calendarUnverified, and PeriodStatus can
// only ever return insufficient_data — market_closed becomes structurally
// unreachable. This is the same shape as the integration test but exercised
// via the fast fake-repo path (milliseconds, no Docker) so a regression in
// just this branch is caught by `go test ./internal/usecase/...` without
// waiting on the Docker-gated integration suite.
//
// Two things are asserted, not just "no error": (1) CropByCode is actually
// invoked with cropPath's own cropCode — proves the fallback fires, not
// just that it's wired into the interface; (2) the resulting Status is
// market_closed, not insufficient_data — proves the fallback's returned
// category is actually THREADED THROUGH into the closure/verified-calendar
// match (RestDays/RestDayCoverage below are seeded for category N04, the
// exact category CropByCode returns — a wrong or ignored category would
// find no matching closure and misreport ready/insufficient_data instead).
func TestTrendSummaryCropPathFallsBackToCropByCodeWhenRowsCarryNoCategory(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	closedDate := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	fallbackCategory := "N04"
	var gotCropCode string
	fake := &fakeTrendRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return nil, nil // zero rows — the real closed-market shape; nothing to derive categoryCode from
		},
		restDays: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return []domain.RestDay{{MarketCode: "109", MarketType: "V", RestDate: closedDate}}, nil // "V" -> N04
		},
		restDayCoverage: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: "109", MarketType: "V", Year: 2026}}, nil
		},
		cropByCode: func(ctx context.Context, cropCode string) (domain.CropRef, error) {
			gotCropCode = cropCode
			return domain.CropRef{CropCode: cropCode, CategoryCode: &fallbackCategory}, nil
		},
	}
	uc := NewTrendSummary(wrapFakeTrendRepo(fake), nil, testConfig(defaultTestInference()), zap.NewNop())
	result, err := uc.Get(context.Background(), "A1", "", "109",
		forecast.NewDate(2026, 8, 3), forecast.NewDate(2026, 8, 3), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCropCode != "A1" {
		t.Errorf("CropByCode called with cropCode=%q, want %q — fallback must fire with cropPath's own cropCode, not be skipped", gotCropCode, "A1")
	}
	if len(result.Points) != 1 {
		t.Fatalf("want 1 point, got %d", len(result.Points))
	}
	if result.Points[0].Status != forecast.PredictionStatusMarketClosed {
		t.Errorf("Points[0].Status = %q, want %q (CropByCode's returned category must actually be used to match the closure calendar, not just fetched and discarded)",
			result.Points[0].Status, forecast.PredictionStatusMarketClosed)
	}
}

// TestTrendSummaryDataMetaExcludesBaselineWindowRows pins the I2 fix (final
// review, 2026-08-08): dataMeta must describe only the primary [from,to]
// window, not the wider wideFrom/wideTo range cropPath/groupPath actually
// read the repository with (that wider range also carries WoW/YoY364
// baseline rows — see the design spec's dataMeta section: "computed from the
// raw rows that actually contributed to points[]... not the baseline
// windows"). Before the fix, requesting a WoW comparison pulled the 7-25
// baseline row's distinct SourceDigest into buildDataMeta's input, changing
// DatasetDigest for an otherwise byte-identical primary response — breaking
// any client-side "did the data change" check purely by toggling
// `comparisons`.
//
// The fake's byCrop deliberately filters by the from/to it's called with
// (unlike this file's other fakes, which ignore from/to) so the test
// actually distinguishes "repo was asked for a wider range" from "repo was
// asked for just the primary window" — without that, both calls would see
// identical `rows` regardless of the bug, and the test would pass whether or
// not the fix is applied.
func TestTrendSummaryDataMetaExcludesBaselineWindowRows(t *testing.T) {
	cat := "N04"
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	primaryDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	baselineDate := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC) // WoW baseline, 7 days earlier
	primaryDigest := repeatHexTest('1')
	baselineDigest := repeatHexTest('2')
	allRows := []domain.ObservedPrice{
		{MarketCode: "109", CropCode: "A1", CategoryCode: &cat, TradeDate: primaryDate, AvgPrice: floatPtr(42), VolumeKg: floatPtr(100), SourceDigest: primaryDigest, AvailableAt: now, RevisionNo: 1},
		{MarketCode: "109", CropCode: "A1", CategoryCode: &cat, TradeDate: baselineDate, AvgPrice: floatPtr(40), VolumeKg: floatPtr(100), SourceDigest: baselineDigest, AvailableAt: now, RevisionNo: 1},
	}
	newFake := func() *fakeTrendRepo {
		return &fakeTrendRepo{
			byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
				var out []domain.ObservedPrice
				for _, r := range allRows {
					if r.TradeDate.Before(from) || r.TradeDate.After(to) {
						continue
					}
					out = append(out, r)
				}
				return out, nil
			},
			restDays: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
				return nil, nil
			},
			restDayCoverage: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) { return nil, nil },
		}
	}

	ucWithComparisons := NewTrendSummary(wrapFakeTrendRepo(newFake()), nil, testConfig(defaultTestInference()), zap.NewNop())
	withComparisons, err := ucWithComparisons.Get(context.Background(), "A1", "", "109",
		forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay,
		[]forecast.TrendComparisonKind{forecast.ComparisonWoW}, now)
	if err != nil {
		t.Fatalf("unexpected error (with comparisons): %v", err)
	}

	ucNoComparisons := NewTrendSummary(wrapFakeTrendRepo(newFake()), nil, testConfig(defaultTestInference()), zap.NewNop())
	noComparisons, err := ucNoComparisons.Get(context.Background(), "A1", "", "109",
		forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay,
		nil, now)
	if err != nil {
		t.Fatalf("unexpected error (no comparisons): %v", err)
	}

	if withComparisons.DataMeta.DatasetDigest == nil || noComparisons.DataMeta.DatasetDigest == nil {
		t.Fatalf("want non-nil DatasetDigest on both responses, got with=%v without=%v",
			withComparisons.DataMeta.DatasetDigest, noComparisons.DataMeta.DatasetDigest)
	}
	if *withComparisons.DataMeta.DatasetDigest != *noComparisons.DataMeta.DatasetDigest {
		t.Errorf("DatasetDigest differs by whether `comparisons` was requested: with=%q without=%q — dataMeta must describe only the primary window, not baseline comparison data",
			*withComparisons.DataMeta.DatasetDigest, *noComparisons.DataMeta.DatasetDigest)
	}
	wantDigest := "sha256:" + primaryDigest
	if *noComparisons.DataMeta.DatasetDigest != wantDigest {
		t.Errorf("DatasetDigest = %q, want %q (only the primary-window row's digest, the baseline row's digest %q must not leak in)",
			*noComparisons.DataMeta.DatasetDigest, wantDigest, baselineDigest)
	}
}

// TestTrendSummaryCalendarForWarnsOnUnknownMarketType pins the I4 fix (final
// review, 2026-08-08): calendarFor must route through the SAME
// addRestDaysToClosures/appendVerifiedCalendarKeys helpers forecast.go
// already uses (SeriesForecast/GroupForecast), which Warn on an unrecognized
// market_type instead of silently skipping it. Before the fix, calendarFor
// hand-rolled an equivalent loop that dropped this Warn entirely — an
// upstream market_type code change would then read as "this market has no
// closures" with zero observable signal.
func TestTrendSummaryCalendarForWarnsOnUnknownMarketType(t *testing.T) {
	cat := "N04"
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	fake := &fakeTrendRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", CategoryCode: &cat, TradeDate: tradeDate, AvgPrice: floatPtr(40), VolumeKg: floatPtr(100), AvailableAt: now, RevisionNo: 1},
			}, nil
		},
		restDays: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return []domain.RestDay{{MarketCode: "109", MarketType: "X", RestDate: tradeDate}}, nil // "X" has no CategoryCodeForMarketType mapping
		},
		restDayCoverage: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: "109", MarketType: "X", Year: 2026}}, nil // also unrecognized
		},
	}
	logger, logs := testLogger()
	uc := NewTrendSummary(wrapFakeTrendRepo(fake), nil, testConfig(defaultTestInference()), logger)
	_, err := uc.Get(context.Background(), "A1", "", "109", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "unknown market_type") {
			found = true
		}
	}
	if !found {
		t.Error("calendarFor must route through addRestDaysToClosures/appendVerifiedCalendarKeys and Warn on unknown market_type, not silently skip it")
	}
}

// TestTrendSummaryGroupPathCategoryCodeIgnoresActiveCropWithNilCategory pins
// the M2 fix (final review, 2026-08-08): the active-crop categoryCode
// preference must also require the active crop's CategoryCode to be
// non-nil before overwriting an already-picked, perfectly usable category
// from an earlier inactive crop. Before the fix,
// [{inactive crop2, CategoryCode: N05}, {active crop1, CategoryCode: nil}]
// picked nil over N05, because the condition only checked
// `!categoryCodeFromActive && c.Active`, never whether the active crop's own
// category was itself usable.
//
// Setup mirrors TestTrendSummaryGroupPathCategoryCodePrefersActiveCrop but
// swaps which crop carries the real category: crop2 (inactive, listed
// FIRST) has category N05; crop1 (active, listed SECOND, the only crop with
// observed data) has a nil category. Market A is declared closed via
// RestDays for market_type "F" (== category N05) only; RestDayCoverage
// verifies both N04 and N05 so neither reads as calendar_unverified.
//
//   - Correct categoryCode (N05, from inactive crop2 — the fix keeps it
//     since active crop1's category is nil, not usable): market A closed
//     for N05 on 8/1 -> Status == market_closed.
//   - Buggy categoryCode (nil, overwritten by active crop1's unusable
//     category): nil categoryCode makes IsVerified unconditionally false
//     (calendar.go) -> every date calendar_unverified -> Status ==
//     insufficient_data instead.
func TestTrendSummaryGroupPathCategoryCodeIgnoresActiveCropWithNilCategory(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	d1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	realCategory := "N05" // crop2, Active=false, listed FIRST — the usable category

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: "crop2", Active: false, CategoryCode: &realCategory},
				{CropCode: "crop1", Active: true, CategoryCode: nil}, // active, but no usable category
			}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1, AvgPrice: floatPtr(30), VolumeKg: floatPtr(1), AvailableAt: now},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "A", CropCode: "crop1", Volume: 1}}, nil
		},
		restDaysFn: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return []domain.RestDay{{MarketCode: "A", MarketType: "F", RestDate: d1}}, nil // "F" -> N05
		},
		restDayCoverageFn: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{
				{MarketCode: "A", MarketType: "V", Year: 2026}, // verifies N04
				{MarketCode: "A", MarketType: "F", Year: 2026}, // verifies N05 too, so a wrong pick isn't masked by calendar_unverified
			}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	uc := NewTrendSummary(repo, forecastUC, testConfig(defaultTestInference()), zap.NewNop())

	result, err := uc.Get(context.Background(), "", "group1", "", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("want 1 point, got %d", len(result.Points))
	}
	if result.Points[0].Status != forecast.PredictionStatusMarketClosed {
		t.Errorf("Points[0].Status = %q, want %q (categoryCode must stay N05 from inactive crop2 — active crop1's nil category must not overwrite an already-usable pick)",
			result.Points[0].Status, forecast.PredictionStatusMarketClosed)
	}
}

// TestTrendSummaryGroupPathVolumeNilNotFabricatedZero pins the I3 fix (final
// review, 2026-08-08) end to end at the usecase layer: a group-path
// contributing row whose VolumeKg is a genuine SQL NULL must produce a nil
// TrendSummaryPoint.Volume, not a fabricated &0.0 — reaching all the way
// from domain.ObservedPrice through DailyVariantPrice/GroupObservedPoint to
// the usecase's own response DTO (the last hop before the gRPC handler maps
// it onto the wire's optional volume_kg field).
func TestTrendSummaryGroupPathVolumeNilNotFabricatedZero(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	d1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				// VolumeKg deliberately left nil — a genuine SQL NULL, not a
				// real zero.
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1, AvgPrice: floatPtr(30), VolumeKg: nil, AvailableAt: now},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return nil, nil // no recent-volume rows either — irrelevant to this row's own nil Volume
		},
		restDaysFn: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			return nil, nil
		},
		restDayCoverageFn: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) { return nil, nil },
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	uc := NewTrendSummary(repo, forecastUC, testConfig(defaultTestInference()), zap.NewNop())

	result, err := uc.Get(context.Background(), "", "group1", "", forecast.NewDate(2026, 8, 1), forecast.NewDate(2026, 8, 1), forecast.GranularityDay, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("want 1 point, got %d", len(result.Points))
	}
	if result.Points[0].Value == nil || *result.Points[0].Value != 30 {
		t.Fatalf("Points[0].Value = %v, want 30 (the row's real price)", result.Points[0].Value)
	}
	if result.Points[0].Volume != nil {
		t.Errorf("Points[0].Volume = %v, want nil (the contributing row's volume_kg was a genuine SQL NULL — must not be fabricated into 0)", *result.Points[0].Volume)
	}
}
