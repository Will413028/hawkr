// services/pricebook-service/internal/usecase/estimated_item_price_test.go
package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

func seedWithGroup(aliasName, groupName string) mapping.Seed {
	return mapping.NewSeedForTest([]mapping.SeedEntry{
		{AliasName: aliasName, MatchKind: mapping.MatchKindConsumerAlias, CropGroupName: groupName},
	})
}

func seedWithCrop(aliasName, cropCode string) mapping.Seed {
	return mapping.NewSeedForTest([]mapping.SeedEntry{
		{AliasName: aliasName, MatchKind: mapping.MatchKindConsumerAlias, CropCode: cropCode},
	})
}

func ptr(f float64) *float64 { return &f }

func TestEstimatedItemPrice_UnresolvedCropName_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	uc := NewEstimatedItemPrice(nil, &fakeForecastRepo{}, seed, forecast.ModelArtifact{})

	result, err := uc.Get(context.Background(), "火龍果", 0.5, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable {
		t.Errorf("Status = %q, want unavailable", result.Status)
	}
	if result.Low != nil || result.Median != nil || result.High != nil {
		t.Errorf("expected no price fields for unresolved crop, got %+v", result)
	}
}

func TestEstimatedItemPrice_GroupResolved_WithMarketCode_ReturnsError(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	uc := NewEstimatedItemPrice(nil, &fakeForecastRepo{}, seed, forecast.ModelArtifact{})

	_, err := uc.Get(context.Background(), "甘藍菜", 0.5, "109", time.Now())
	if !errors.Is(err, ErrMarketCodeNotAllowedForGroup) {
		t.Errorf("err = %v, want ErrMarketCodeNotAllowedForGroup", err)
	}
}

func TestEstimatedItemPrice_CropResolved_WithoutMarketCode_ReturnsError(t *testing.T) {
	seed := seedWithCrop("甘藍-初秋", "A1")
	uc := NewEstimatedItemPrice(nil, &fakeForecastRepo{}, seed, forecast.ModelArtifact{})

	_, err := uc.Get(context.Background(), "甘藍-初秋", 0.5, "", time.Now())
	if !errors.Is(err, ErrMarketCodeRequiredForCrop) {
		t.Errorf("err = %v, want ErrMarketCodeRequiredForCrop", err)
	}
}

func TestEstimatedItemPrice_GroupResolved_ZeroMatches_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
		return nil, nil
	}}
	uc := NewEstimatedItemPrice(nil, repo, seed, forecast.ModelArtifact{})

	result, err := uc.Get(context.Background(), "甘藍菜", 0.5, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable {
		t.Errorf("Status = %q, want unavailable", result.Status)
	}
}

func TestEstimatedItemPrice_GroupResolved_AmbiguousMatches_ReturnsError(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
		return []domain.CropGroupMatch{{GroupCode: "G1"}, {GroupCode: "G2"}}, nil
	}}
	uc := NewEstimatedItemPrice(nil, repo, seed, forecast.ModelArtifact{})

	_, err := uc.Get(context.Background(), "甘藍菜", 0.5, "", time.Now())
	if !errors.Is(err, ErrAmbiguousCropGroup) {
		t.Errorf("err = %v, want ErrAmbiguousCropGroup", err)
	}
}

// TestEstimatedItemPrice_CropResolved_HappyPath exercises the real
// SeriesForecast wiring end-to-end (not a fake forecastUC) — a clean 60-day
// observation history with no gaps should reliably produce a ready horizon-1
// point, proving Get() correctly extracts horizon=1 and converts it to a
// per-item price. Fixture pattern mirrors forecast_test.go's own
// TestSeriesForecast_HappyPath (same repo, same helpers), simplified since
// this test isn't re-verifying SeriesForecast's own internal correctness
// (already covered there) — only that this usecase calls it correctly and
// converts its output correctly.
func TestEstimatedItemPrice_CropResolved_HappyPath(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithCrop("甘藍-初秋", cropCode)
	artifact := testArtifact(testSeriesModel("moa:"+marketCode+":"+cropCode, "moa:"+marketCode, "moa:"+cropCode, category, 118, 60))
	uc := NewEstimatedItemPrice(fc, repo, seed, artifact)

	result, err := uc.Get(context.Background(), "甘藍-初秋", 0.5, marketCode, origin)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready", result.Status)
	}
	if result.Low == nil || result.Median == nil || result.High == nil {
		t.Fatalf("expected all three price fields for ready status, got %+v", result)
	}
	if *result.Low > *result.Median || *result.Median > *result.High {
		t.Errorf("prices not monotonic: low=%v median=%v high=%v", *result.Low, *result.Median, *result.High)
	}
	if result.ResolvedCropCode == nil || *result.ResolvedCropCode != cropCode {
		t.Errorf("ResolvedCropCode = %v, want %q", result.ResolvedCropCode, cropCode)
	}
	if result.MarketCode == nil || *result.MarketCode != marketCode {
		t.Errorf("MarketCode = %v, want %q", result.MarketCode, marketCode)
	}
	if result.ModelVersion == "" {
		t.Error("ModelVersion should not be empty")
	}
}

// TestEstimatedItemPrice_CropResolved_ModelDoesNotCoverSeries_ReturnsUnavailable
// is Finding 1's crop-resolved regression test: the model artifact having no
// trained series for this (market, crop) pair is a business "can't produce a
// price" outcome (models are trained incrementally), not an infra failure —
// it must never reach SeriesForecast's own "no series" error path (which the
// gRPC handler maps to codes.Internal). forecastUC is nil: if the pre-check
// didn't short-circuit before calling SeriesForecast, this would panic on a
// nil *Forecast receiver — same proof pattern as
// TestEstimatedItemPrice_UnresolvedCropName_ReturnsUnavailable.
func TestEstimatedItemPrice_CropResolved_ModelDoesNotCoverSeries_ReturnsUnavailable(t *testing.T) {
	seed := seedWithCrop("甘藍-初秋", "A1")
	// artifact has a series, but for a different crop — HasSeries("109", "A1") is false.
	artifact := testArtifact(testSeriesModel("moa:109:ZZ", "moa:109", "moa:ZZ", "N05", 118, 60))
	uc := NewEstimatedItemPrice(nil, &fakeForecastRepo{}, seed, artifact)

	result, err := uc.Get(context.Background(), "甘藍-初秋", 0.5, "109", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable {
		t.Errorf("Status = %q, want unavailable", result.Status)
	}
	if result.ResolvedCropCode == nil || *result.ResolvedCropCode != "A1" {
		t.Errorf("ResolvedCropCode = %v, want A1", result.ResolvedCropCode)
	}
	if result.MarketCode == nil || *result.MarketCode != "109" {
		t.Errorf("MarketCode = %v, want 109", result.MarketCode)
	}
	if result.Low != nil || result.Median != nil || result.High != nil {
		t.Errorf("expected no price fields, got %+v", result)
	}
}

// TestEstimatedItemPrice_GroupResolved_ZeroActiveCrops_ReturnsUnavailable is
// Finding 1's group-resolved regression test for the "group has zero active
// crops" case — mirrors GroupForecast's own "no active crops" error path,
// but pre-checked here so it surfaces as unavailable instead of
// codes.Internal. forecastUC is nil for the same short-circuit-proof reason.
func TestEstimatedItemPrice_GroupResolved_ZeroActiveCrops_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	groupCode := "G1"
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: groupCode}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "A1", Name: "已退場作物", Active: false}}, nil
		},
	}
	uc := NewEstimatedItemPrice(nil, repo, seed, forecast.ModelArtifact{})

	result, err := uc.Get(context.Background(), "甘藍菜", 0.5, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable {
		t.Errorf("Status = %q, want unavailable", result.Status)
	}
	if result.ResolvedCropGroupCode == nil || *result.ResolvedCropGroupCode != groupCode {
		t.Errorf("ResolvedCropGroupCode = %v, want %q", result.ResolvedCropGroupCode, groupCode)
	}
}

// TestEstimatedItemPrice_GroupResolved_NoMatchingSeries_ReturnsUnavailable is
// Finding 1's group-resolved regression test for the "group has active crops
// but the artifact has no series for any of them" case — mirrors
// GroupForecast's own "artifact has no series for any crop in group" error
// path, pre-checked here. forecastUC is nil for the same short-circuit-proof
// reason.
func TestEstimatedItemPrice_GroupResolved_NoMatchingSeries_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	groupCode := "G1"
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: groupCode}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "A1", Name: "現役作物", Active: true}}, nil
		},
	}
	// artifact has a series, but for a different crop (ZZ, not A1) — HasAnySeriesForCrops([A1]) is false.
	artifact := testArtifact(testSeriesModel("moa:109:ZZ", "moa:109", "moa:ZZ", "N05", 118, 60))
	uc := NewEstimatedItemPrice(nil, repo, seed, artifact)

	result, err := uc.Get(context.Background(), "甘藍菜", 0.5, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable {
		t.Errorf("Status = %q, want unavailable", result.Status)
	}
	if result.ResolvedCropGroupCode == nil || *result.ResolvedCropGroupCode != groupCode {
		t.Errorf("ResolvedCropGroupCode = %v, want %q", result.ResolvedCropGroupCode, groupCode)
	}
}

// TestEstimatedItemPrice_GroupResolved_HappyPath is Finding 2's regression
// test: the group-resolved path (the only branch real seed data reaches
// today — the repo's current seed table has zero crop-code rows, all 27
// rows are crop-group rows) had zero end-to-end test coverage before this.
// Exercises the real *Forecast (via NewForecast), not a fake, so it goes
// through GroupForecast for real — mirrors
// TestEstimatedItemPrice_CropResolved_HappyPath's own pattern (real
// forecastUC, only the repo faked) and this package's
// TestGroupForecast_InactiveCropDoesNotBlockReady (forecast_test.go) for the
// group-fixture shape.
func TestEstimatedItemPrice_GroupResolved_HappyPath(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, marketCode, category := "GRP-A", "A1", "109", "N05"

	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: groupCode}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: marketCode, CropCode: cropCode, Volume: 500}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithGroup("甘藍菜", groupCode)
	artifact := testArtifact(testSeriesModel("moa:"+marketCode+":"+cropCode, "moa:"+marketCode, "moa:"+cropCode, category, 118, 60))
	uc := NewEstimatedItemPrice(fc, repo, seed, artifact)

	result, err := uc.Get(context.Background(), "甘藍菜", 0.5, "", origin)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready", result.Status)
	}
	if result.Low == nil || result.Median == nil || result.High == nil {
		t.Fatalf("expected all three price fields for ready status, got %+v", result)
	}
	if *result.Low > *result.Median || *result.Median > *result.High {
		t.Errorf("prices not monotonic: low=%v median=%v high=%v", *result.Low, *result.Median, *result.High)
	}
	if result.ResolvedCropGroupCode == nil || *result.ResolvedCropGroupCode != groupCode {
		t.Errorf("ResolvedCropGroupCode = %v, want %q", result.ResolvedCropGroupCode, groupCode)
	}
	if result.ModelVersion == "" {
		t.Error("ModelVersion should not be empty")
	}
}

func TestPriceResultFromPoint_ReadyStatus_ComputesRoundedPrices(t *testing.T) {
	result := priceResultFromPoint(forecast.PredictionStatusReady, ptr(20.0), ptr(25.0), ptr(30.0), 0.5, "v1")
	if result.Low == nil || *result.Low != 10 {
		t.Errorf("Low = %v, want 10 (round(20.0 x 0.5))", result.Low)
	}
	if result.Median == nil || *result.Median != 13 {
		t.Errorf("Median = %v, want 13 (round(25.0 x 0.5) = 12.5 -> 13)", result.Median)
	}
	if result.High == nil || *result.High != 15 {
		t.Errorf("High = %v, want 15 (round(30.0 x 0.5))", result.High)
	}
}

func TestPriceResultFromPoint_PartialStatus_StillComputesPrices(t *testing.T) {
	result := priceResultFromPoint(forecast.PredictionStatusPartial, ptr(20.0), ptr(25.0), ptr(30.0), 1.0, "v1")
	if result.Low == nil || result.Median == nil || result.High == nil {
		t.Errorf("expected all three price fields for partial status, got %+v", result)
	}
}

func TestPriceResultFromPoint_NonReadyStatus_ReturnsNoPrices(t *testing.T) {
	for _, s := range []forecast.PredictionStatus{
		forecast.PredictionStatusMarketClosed,
		forecast.PredictionStatusInsufficientData,
		forecast.PredictionStatusNotPredicted,
		forecast.PredictionStatusStale,
		forecast.PredictionStatusUnavailable,
	} {
		result := priceResultFromPoint(s, ptr(20.0), ptr(25.0), ptr(30.0), 0.5, "v1")
		if result.Low != nil || result.Median != nil || result.High != nil {
			t.Errorf("status %q: expected no price fields, got %+v", s, result)
		}
	}
}

func TestPointAtHorizon1_FindsCorrectPoint(t *testing.T) {
	points := []ForecastPoint{
		{HorizonDays: 1, Status: forecast.PredictionStatusReady, P10: ptr(1)},
		{HorizonDays: 2, Status: forecast.PredictionStatusReady, P10: ptr(2)},
	}
	p, ok := pointAtHorizon1(points)
	if !ok || p.HorizonDays != 1 || *p.P10 != 1 {
		t.Errorf("pointAtHorizon1 = %+v, %v, want horizon-1 point", p, ok)
	}
}

func TestPointAtHorizon1_EmptySlice_ReturnsFalse(t *testing.T) {
	_, ok := pointAtHorizon1(nil)
	if ok {
		t.Error("expected ok=false for empty slice")
	}
}
