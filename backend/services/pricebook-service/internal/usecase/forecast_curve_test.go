// services/pricebook-service/internal/usecase/forecast_curve_test.go
package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

func TestForecastCurve_UnresolvedCropName_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	uc := NewForecastCurve(nil, &fakeForecastRepo{}, seed, forecast.ModelArtifact{}, testConfig(defaultTestInference()), zap.NewNop())

	result, err := uc.Get(context.Background(), "火龍果", "", "", "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable {
		t.Errorf("Status = %q, want unavailable", result.Status)
	}
	if len(result.Points) != 0 {
		t.Errorf("expected no points for unresolved crop, got %+v", result.Points)
	}

	// dataMeta 規則：完全沒有東西可以描述時，optional 欄位一律 nil，不是假的
	// 0/空字串（issue #2249「缺值使用 null/省略，永不補 0」）。
	meta := result.DataMeta
	if meta.UsabilityStatus != forecast.UsabilityUnusable {
		t.Errorf("DataMeta.UsabilityStatus = %q, want unusable", meta.UsabilityStatus)
	}
	if meta.FreshnessStatus != forecast.FreshnessUnavailable {
		t.Errorf("DataMeta.FreshnessStatus = %q, want unavailable", meta.FreshnessStatus)
	}
	if meta.ModelVersion != nil {
		t.Errorf("DataMeta.ModelVersion = %v, want nil (no model was even resolved)", meta.ModelVersion)
	}
	if meta.DatasetDigest != nil {
		t.Errorf("DataMeta.DatasetDigest = %v, want nil", meta.DatasetDigest)
	}
	if meta.SnapshotID != nil {
		t.Errorf("DataMeta.SnapshotID = %v, want nil", meta.SnapshotID)
	}
	if meta.SourceDataAsOf != nil {
		t.Errorf("DataMeta.SourceDataAsOf = %v, want nil", meta.SourceDataAsOf)
	}
	if meta.MaxFeatureAvailableAt != nil {
		t.Errorf("DataMeta.MaxFeatureAvailableAt = %v, want nil", meta.MaxFeatureAvailableAt)
	}
	if len(meta.QualityReasonCodes) != 0 {
		t.Errorf("DataMeta.QualityReasonCodes = %v, want empty", meta.QualityReasonCodes)
	}
	if meta.Source != "MOA_WHOLESALE_DAILY" || meta.SchemaVersion != "hawkr.pricebook/v1" || meta.Currency != "TWD" || meta.Unit != "kg" {
		t.Errorf("DataMeta constant fields wrong: %+v", meta)
	}
}

func TestForecastCurve_CropResolved_WithoutMarketCode_NoModelReturnsUnavailable(t *testing.T) {
	repo := &fakeForecastRepo{}
	cfg := testConfig(defaultTestInference())
	uc := NewForecastCurve(NewForecast(repo, cfg, zap.NewNop()), repo, seedWithCrop("甘藍-初秋", "A1"), forecast.ModelArtifact{}, cfg, zap.NewNop())
	result, err := uc.Get(context.Background(), "甘藍-初秋", "", "", "", time.Now())
	if err != nil || result.Status != forecast.PredictionStatusUnavailable || result.ResolvedCropCode == nil || *result.ResolvedCropCode != "A1" || result.MarketCode != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestForecastCurve_CropResolved_ModelDoesNotCoverSeries_ReturnsUnavailable(t *testing.T) {
	seed := seedWithCrop("甘藍-初秋", "A1")
	uc := NewForecastCurve(nil, &fakeForecastRepo{}, seed, forecast.ModelArtifact{}, testConfig(defaultTestInference()), zap.NewNop())

	result, err := uc.Get(context.Background(), "甘藍-初秋", "109", "", "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable || result.ResolvedCropCode == nil || *result.ResolvedCropCode != "A1" {
		t.Errorf("result = %+v, want Unavailable with ResolvedCropCode=A1", result)
	}
}

func TestForecastCurve_GroupResolved_ZeroMatches_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
		return nil, nil
	}}
	uc := NewForecastCurve(nil, repo, seed, forecast.ModelArtifact{}, testConfig(defaultTestInference()), zap.NewNop())

	result, err := uc.Get(context.Background(), "甘藍菜", "", "", "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != forecast.PredictionStatusUnavailable {
		t.Errorf("Status = %q, want unavailable", result.Status)
	}
}

func TestForecastCurve_GroupResolved_AmbiguousMatches_ReturnsError(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
		return []domain.CropGroupMatch{{GroupCode: "G1"}, {GroupCode: "G2"}}, nil
	}}
	uc := NewForecastCurve(nil, repo, seed, forecast.ModelArtifact{}, testConfig(defaultTestInference()), zap.NewNop())

	_, err := uc.Get(context.Background(), "甘藍菜", "", "", "", time.Now())
	if !errors.Is(err, ErrAmbiguousCropGroup) {
		t.Errorf("err = %v, want ErrAmbiguousCropGroup", err)
	}
}

// TestForecastCurve_CropResolved_HappyPath 跟 estimated_item_price_test.go 的
// TestEstimatedItemPrice_CropResolved_HappyPath 用同一套 fixture 模式（真的
// SeriesForecast 接線，不是 fake），驗證 Get 正確把全部 7 個點原封不動端出來
// （不做任何價格換算——這是它跟 EstimatedItemPrice 的關鍵差異）。
func TestForecastCurve_CropResolved_HappyPath(t *testing.T) {
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
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍-初秋", marketCode, "", "", origin)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready", result.Status)
	}
	if len(result.Points) != 7 {
		t.Fatalf("len(Points) = %d, want 7", len(result.Points))
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
	// 沒有任何價格換算——Points 的 P10/P50/P90 應與底層 SeriesForecast 給的
	// 完全相同（不像 EstimatedItemPrice 會乘 net_weight_kg）。
	first := result.Points[0]
	if first.HorizonDays != 1 {
		t.Errorf("Points[0].HorizonDays = %d, want 1", first.HorizonDays)
	}
}

// TestForecastCurve_GroupResolved_HappyPath 鏡射
// estimated_item_price_test.go 目前唯一沒有的 group happy path（P5 final
// review 才發現 EstimatedItemPrice 也缺這個，group 是唯一真的可達的正式路
// 徑），跟 forecast_test.go 的 TestGroupForecast_InactiveCropDoesNotBlockReady
// 用同一套 fixture 形狀。
func TestForecastCurve_GroupResolved_HappyPath(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, category := "GRP-A", "A1", "N05"

	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: groupCode}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations("109", cropCode, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: cropCode, Volume: 500}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithGroup("甘藍菜", groupCode)
	artifact := testArtifact(testSeriesModel("moa:109:"+cropCode, "moa:109", "moa:"+cropCode, category, 118, 60))
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍菜", "", "", "", origin)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready", result.Status)
	}
	if len(result.Points) != 7 {
		t.Fatalf("len(Points) = %d, want 7", len(result.Points))
	}
	if result.ResolvedCropGroupCode == nil || *result.ResolvedCropGroupCode != groupCode {
		t.Errorf("ResolvedCropGroupCode = %v, want %q", result.ResolvedCropGroupCode, groupCode)
	}
}

// --- crop_code / group_code direct-lookup bypass (Task 7) ---
//
// Constructed with a ZERO-VALUE mapping.Seed and an empty cropName, so a
// regression that still routed through seed.Resolve would come back
// Unavailable-with-no-resolved-codes instead of producing a curve.

func TestForecastCurve_CropCodeBypass_HappyPath(t *testing.T) {
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
	artifact := testArtifact(testSeriesModel("moa:"+marketCode+":"+cropCode, "moa:"+marketCode, "moa:"+cropCode, category, 118, 60))
	uc := NewForecastCurve(fc, repo, mapping.Seed{}, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "", marketCode, cropCode, "", origin)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready", result.Status)
	}
	if len(result.Points) != 7 {
		t.Fatalf("len(Points) = %d, want 7", len(result.Points))
	}
	if result.ResolvedCropCode == nil || *result.ResolvedCropCode != cropCode {
		t.Errorf("ResolvedCropCode = %v, want %q", result.ResolvedCropCode, cropCode)
	}
}

// TestForecastCurve_GroupCodeBypass_WithMarketCode_Succeeds replaces
// TestForecastCurve_GroupCodeBypassForbidsMarketCode — the direct
// group_code bypass path (no crop_name) is the actual production entry
// point the frontend uses for an L3 group selection with a chosen market
// (design spec §1c/§2c), and it must now accept marketCode, not reject it.
func TestForecastCurve_GroupCodeBypass_WithMarketCode_Succeeds(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, category := "GRP-C", "N05"
	viewedMarket := "109"

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: "crop1", Name: "作物一", Active: true, CategoryCode: strPtr(category)},
			}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(viewedMarket, "crop1", strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: viewedMarket, CropCode: "crop1", Volume: 500}}, nil
		},
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(mc, cc, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		marketByCodeFn: func(ctx context.Context, mc string) (domain.MarketRef, error) {
			return domain.MarketRef{MarketCode: mc, Name: "市場-" + mc}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	artifact := testArtifact(
		testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", category, 118, 60),
		testSeriesModel("moa:110:crop1", "moa:110", "moa:crop1", category, 118, 60),
	)
	uc := NewForecastCurve(fc, repo, mapping.Seed{}, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "", viewedMarket, "", groupCode, origin)
	if err != nil {
		t.Fatalf("Get: unexpected error %v (group_code bypass + market_code must now be accepted)", err)
	}
	if result.ResolvedCropGroupCode == nil || *result.ResolvedCropGroupCode != groupCode {
		t.Fatalf("ResolvedCropGroupCode = %v, want %q", result.ResolvedCropGroupCode, groupCode)
	}
	found109 := false
	for _, e := range result.MarketComparison {
		if e.MarketCode == viewedMarket {
			found109 = true
		}
	}
	if found109 {
		t.Errorf("MarketComparison = %+v, viewed market %s must be excluded", result.MarketComparison, viewedMarket)
	}
}

func TestForecastCurve_ConflictingResolutionInputsRejected(t *testing.T) {
	repo := &fakeForecastRepo{}
	cfg := testConfig(defaultTestInference())
	uc := NewForecastCurve(NewForecast(repo, cfg, zap.NewNop()), repo, seedWithCrop("甘藍-初秋", "A1"), forecast.ModelArtifact{}, cfg, zap.NewNop())

	_, err := uc.Get(context.Background(), "甘藍-初秋", "109", "A1", "", time.Now())
	if !errors.Is(err, ErrConflictingResolutionInputs) {
		t.Errorf("err = %v, want ErrConflictingResolutionInputs", err)
	}
}

func TestForecastCurve_NoResolutionInputRejected(t *testing.T) {
	uc := NewForecastCurve(nil, &fakeForecastRepo{}, mapping.Seed{}, forecast.ModelArtifact{}, testConfig(defaultTestInference()), zap.NewNop())

	_, err := uc.Get(context.Background(), "", "109", "", "", time.Now())
	if !errors.Is(err, ErrNoResolutionInput) {
		t.Errorf("err = %v, want ErrNoResolutionInput", err)
	}
}

// --- MarketComparison (Task 8) ---
//
// The first two tests reuse TestForecastCurve_CropResolved_HappyPath's and
// TestForecastCurve_GroupResolved_HappyPath's exact fixture shapes (same
// origin/originDate/category/testSeriesModel/testArtifact wiring), only
// adding what's needed for the sibling-market comparison itself: a second
// trained series (for the crop test) and marketByCodeFn.

func TestForecastCurve_Get_CropResolutionIncludesMarketComparison(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, siblingMarketCode, cropCode, category := "109", "110", "A1", "N05"

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(mc, cropCode, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		marketByCodeFn: func(ctx context.Context, mc string) (domain.MarketRef, error) {
			return domain.MarketRef{MarketCode: mc, Name: "市場-" + mc}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithCrop("甘藍-初秋", cropCode)
	artifact := testArtifact(
		testSeriesModel("moa:"+marketCode+":"+cropCode, "moa:"+marketCode, "moa:"+cropCode, category, 118, 60),
		testSeriesModel("moa:"+siblingMarketCode+":"+cropCode, "moa:"+siblingMarketCode, "moa:"+cropCode, category, 118, 60),
	)
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍-初秋", marketCode, "", "", origin)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(result.MarketComparison) != 1 || result.MarketComparison[0].MarketCode != siblingMarketCode {
		t.Fatalf("MarketComparison = %+v, want exactly 1 sibling entry for market %s (%s itself excluded)", result.MarketComparison, siblingMarketCode, marketCode)
	}
	if result.MarketComparison[0].MarketName != "市場-"+siblingMarketCode {
		t.Errorf("MarketComparison[0].MarketName = %q, want %q", result.MarketComparison[0].MarketName, "市場-"+siblingMarketCode)
	}
	if result.MarketComparison[0].P50 == nil {
		t.Error("MarketComparison[0].P50 should not be nil — the sibling market's own forecast should succeed")
	}
	if result.MarketComparison[0].IsApproximate {
		t.Error("MarketComparison[0].IsApproximate should be false — the crop path's P50 is a single model's direct output, never a group average")
	}
}

// TestForecastCurve_Get_GroupResolutionIncludesMarketComparison replaces
// TestForecastCurve_Get_GroupResolutionHasNoMarketComparison — a group
// resolution now DOES get a MarketComparison, unioned across the group's
// crops' sibling markets, with each entry's P50 the arithmetic mean of
// whichever of the group's crops have a usable P50 at that market, and
// IsApproximate always true. Two crops in the group: crop1 trained in
// markets 109 (viewed, excluded) and 110; crop2 trained in markets 109 and
// 111. Sibling market 110 gets only crop1's P50 (crop2 has no trained
// series there); market 111 gets only crop2's P50.
func TestForecastCurve_Get_GroupResolutionIncludesMarketComparison(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, category := "GRP-A", "N05"
	viewedMarket := "109"

	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: groupCode}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: "crop1", Name: "作物一", Active: true, CategoryCode: strPtr(category)},
				{CropCode: "crop2", Name: "作物二", Active: true, CategoryCode: strPtr(category)},
			}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(viewedMarket, "crop1", strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: viewedMarket, CropCode: "crop1", Volume: 500}}, nil
		},
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(mc, cc, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		marketByCodeFn: func(ctx context.Context, mc string) (domain.MarketRef, error) {
			return domain.MarketRef{MarketCode: mc, Name: "市場-" + mc}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithGroup("甘藍菜", groupCode)
	artifact := testArtifact(
		testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", category, 118, 60),
		testSeriesModel("moa:110:crop1", "moa:110", "moa:crop1", category, 118, 60),
		testSeriesModel("moa:109:crop2", "moa:109", "moa:crop2", category, 118, 60),
		testSeriesModel("moa:111:crop2", "moa:111", "moa:crop2", category, 118, 60),
	)
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍菜", viewedMarket, "", "", origin)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	byMarket := map[string]MarketComparisonEntry{}
	for _, e := range result.MarketComparison {
		byMarket[e.MarketCode] = e
	}
	if len(byMarket) != 2 {
		t.Fatalf("MarketComparison = %+v, want exactly markets 110 and 111 (viewed market %s excluded)", result.MarketComparison, viewedMarket)
	}
	for _, code := range []string{"110", "111"} {
		e, ok := byMarket[code]
		if !ok {
			t.Fatalf("MarketComparison missing market %s", code)
		}
		if !e.IsApproximate {
			t.Errorf("MarketComparison[%s].IsApproximate = false, want true (group resolution always approximate)", code)
		}
		if e.P50 == nil {
			t.Errorf("MarketComparison[%s].P50 = nil, want a value", code)
		}
	}
}

// TestForecastCurve_Get_GroupMarketComparison_AveragesAcrossCropsSharingAMarket
// proves the arithmetic-mean step itself: two crops both trained in the
// same sibling market get their P50s averaged into one entry, not two
// entries and not the last-one-wins.
func TestForecastCurve_Get_GroupMarketComparison_AveragesAcrossCropsSharingAMarket(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, category := "GRP-B", "N05"
	viewedMarket, siblingMarket := "109", "110"

	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: groupCode}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: "crop1", Name: "作物一", Active: true, CategoryCode: strPtr(category)},
				{CropCode: "crop2", Name: "作物二", Active: true, CategoryCode: strPtr(category)},
			}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(viewedMarket, "crop1", strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: viewedMarket, CropCode: "crop1", Volume: 500}}, nil
		},
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			// Both crops share the same flat/linear synthetic price curve
			// shape (dailyObservations' own generator), so both crops'
			// resulting P50 at the sibling market is deterministic and equal
			// — the test only needs to prove averaging ran (a fixed known
			// P50, not two different ones), not exercise a specific spread.
			return dailyObservations(mc, cc, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		marketByCodeFn: func(ctx context.Context, mc string) (domain.MarketRef, error) {
			return domain.MarketRef{MarketCode: mc, Name: "市場-" + mc}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithGroup("甘藍菜", groupCode)
	artifact := testArtifact(
		testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", category, 118, 60),
		testSeriesModel("moa:110:crop1", "moa:110", "moa:crop1", category, 118, 60),
		testSeriesModel("moa:109:crop2", "moa:109", "moa:crop2", category, 118, 60),
		testSeriesModel("moa:110:crop2", "moa:110", "moa:crop2", category, 118, 60),
	)
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍菜", viewedMarket, "", "", origin)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(result.MarketComparison) != 1 || result.MarketComparison[0].MarketCode != siblingMarket {
		t.Fatalf("MarketComparison = %+v, want exactly one entry for market %s", result.MarketComparison, siblingMarket)
	}
	if result.MarketComparison[0].P50 == nil {
		t.Fatal("MarketComparison[0].P50 = nil, want a value (both crop1 and crop2 have trained series at the sibling market)")
	}
}

// Supplementary failure retains market identity and a nullable price.
func TestForecastCurve_Get_SiblingMarketForecastFailureRetainsIdentityNotFatal(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, siblingMarketCode, cropCode, category := "109", "110", "A1", "N05"
	siblingErr := errors.New("sibling market observations unavailable")

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			if mc == siblingMarketCode {
				return nil, siblingErr
			}
			return dailyObservations(mc, cropCode, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		marketByCodeFn: func(ctx context.Context, mc string) (domain.MarketRef, error) {
			return domain.MarketRef{MarketCode: mc, Name: "市場-" + mc}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithCrop("甘藍-初秋", cropCode)
	artifact := testArtifact(
		testSeriesModel("moa:"+marketCode+":"+cropCode, "moa:"+marketCode, "moa:"+cropCode, category, 118, 60),
		testSeriesModel("moa:"+siblingMarketCode+":"+cropCode, "moa:"+siblingMarketCode, "moa:"+cropCode, category, 118, 60),
	)
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍-初秋", marketCode, "", "", origin)
	if err != nil {
		t.Fatalf("Get: unexpected error %v (sibling market forecast failure must not be fatal)", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready (the primary market's own forecast is unaffected by the sibling's failure)", result.Status)
	}
	if len(result.MarketComparison) != 1 || result.MarketComparison[0].P50 != nil || result.MarketComparison[0].Status != forecast.PredictionStatusUnavailable {
		t.Errorf("MarketComparison = %+v, want unavailable entry (sibling market %s's forecast errored and must retain identity)", result.MarketComparison, siblingMarketCode)
	}
}

// A failed supplementary catalog lookup has the same unavailable shape.
func TestForecastCurve_Get_SiblingMarketNameLookupFailureRetainsIdentityNotFatal(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, siblingMarketCode, cropCode, category := "109", "110", "A1", "N05"
	marketNameErr := errors.New("market name lookup unavailable")

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(mc, cropCode, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		marketByCodeFn: func(ctx context.Context, mc string) (domain.MarketRef, error) {
			if mc == siblingMarketCode {
				return domain.MarketRef{}, marketNameErr
			}
			return domain.MarketRef{MarketCode: mc, Name: "市場-" + mc}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithCrop("甘藍-初秋", cropCode)
	artifact := testArtifact(
		testSeriesModel("moa:"+marketCode+":"+cropCode, "moa:"+marketCode, "moa:"+cropCode, category, 118, 60),
		testSeriesModel("moa:"+siblingMarketCode+":"+cropCode, "moa:"+siblingMarketCode, "moa:"+cropCode, category, 118, 60),
	)
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍-初秋", marketCode, "", "", origin)
	if err != nil {
		t.Fatalf("Get: unexpected error %v (sibling market name lookup failure must not be fatal)", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready (the primary market's own forecast is unaffected by the sibling's name lookup failure)", result.Status)
	}
	if len(result.MarketComparison) != 1 || result.MarketComparison[0].P50 != nil || result.MarketComparison[0].Status != forecast.PredictionStatusUnavailable {
		t.Errorf("MarketComparison = %+v, want unavailable entry (sibling market %s's MarketByCode errored and must retain identity)", result.MarketComparison, siblingMarketCode)
	}
}

// ==================== dataMeta（issue #2249 Slice A） ====================

// TestForecastCurve_DataMeta_CropResolved_ModelVersionEqualsTopLevelAndUsableWhenReady
// reuses TestForecastCurve_CropResolved_HappyPath's exact fixture shape and
// pins the dataMeta invariants the issue's test matrix calls out explicitly:
// forecast top-level model_version equals dataMeta.modelVersion; official
// fields never collide with dataMeta's own quantile-free shape; missing
// values are nil, not fake zero/empty-string.
func TestForecastCurve_DataMeta_CropResolved_ModelVersionEqualsTopLevelAndUsableWhenReady(t *testing.T) {
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
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍-初秋", marketCode, "", "", origin)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready", result.Status)
	}

	meta := result.DataMeta
	if meta.ModelVersion == nil || *meta.ModelVersion != result.ModelVersion {
		t.Errorf("DataMeta.ModelVersion = %v, want equal to top-level ModelVersion %q (issue rule: the two must match)", meta.ModelVersion, result.ModelVersion)
	}
	if meta.UsabilityStatus != forecast.UsabilityUsable {
		t.Errorf("DataMeta.UsabilityStatus = %q, want usable (top-level status is ready)", meta.UsabilityStatus)
	}
	if meta.FreshnessStatus != forecast.FreshnessFresh {
		t.Errorf("DataMeta.FreshnessStatus = %q, want fresh (latest observation 1h old)", meta.FreshnessStatus)
	}
	if meta.DatasetDigest == nil || len(*meta.DatasetDigest) < len("sha256:") || (*meta.DatasetDigest)[:7] != "sha256:" {
		t.Errorf("DataMeta.DatasetDigest = %v, want sha256:-prefixed", meta.DatasetDigest)
	}
	if meta.SnapshotID == nil || meta.DatasetDigest == nil || *meta.SnapshotID != *meta.DatasetDigest {
		t.Errorf("DataMeta.SnapshotID = %v, want equal to DatasetDigest %v", meta.SnapshotID, meta.DatasetDigest)
	}
	if meta.SourceDataAsOf == nil {
		t.Error("DataMeta.SourceDataAsOf should not be nil — DataThroughDate was set")
	}
	if meta.MaxFeatureAvailableAt == nil {
		t.Error("DataMeta.MaxFeatureAvailableAt should not be nil — observations exist")
	}
	if len(meta.QualityReasonCodes) != 0 {
		t.Errorf("DataMeta.QualityReasonCodes = %v, want empty on the non-group path", meta.QualityReasonCodes)
	}
}

// TestForecastCurve_DataMeta_GroupResolved_DegradedWhenMarketSupportBelowMinimum
// reuses TestForecastCurve_GroupResolved_HappyPath's single-market fixture:
// the top-level status is ready (the crop's own forecast is clean), but the
// group only has 1 supporting market — below GroupMinMarketSupportCount's
// default of 2 (ADR D3) — so dataMeta.usabilityStatus must be degraded with
// INSUFFICIENT_MARKET_SUPPORT, even though the plain PredictionStatus-based
// mapping the non-group path uses would have said usable. This is the test
// that would catch GroupUsability being wired with the wrong GroupPoint or
// the wrong threshold.
func TestForecastCurve_DataMeta_GroupResolved_DegradedWhenMarketSupportBelowMinimum(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, category := "GRP-A", "A1", "N05"

	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: groupCode}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations("109", cropCode, strPtr(category), originDate.AddDays(-60), originDate, 55, 900, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: cropCode, Volume: 500}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
	seed := seedWithGroup("甘藍菜", groupCode)
	artifact := testArtifact(testSeriesModel("moa:109:"+cropCode, "moa:109", "moa:"+cropCode, category, 118, 60))
	uc := NewForecastCurve(fc, repo, seed, artifact, testConfig(defaultTestInference()), logger)

	result, err := uc.Get(context.Background(), "甘藍菜", "", "", "", origin)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if result.Status != forecast.PredictionStatusReady {
		t.Fatalf("Status = %q, want ready (this crop's own forecast is clean; the group support signal is a separate axis)", result.Status)
	}

	meta := result.DataMeta
	if meta.UsabilityStatus != forecast.UsabilityDegraded {
		t.Errorf("DataMeta.UsabilityStatus = %q, want degraded (only 1 supporting market, below GroupMinMarketSupportCount=%d)",
			meta.UsabilityStatus, forecast.DefaultGroupMinMarketSupportCount)
	}
	foundCode := false
	for _, c := range meta.QualityReasonCodes {
		if c == forecast.QualityReasonInsufficientMarketSupport {
			foundCode = true
		}
	}
	if !foundCode {
		t.Errorf("DataMeta.QualityReasonCodes = %v, want to include INSUFFICIENT_MARKET_SUPPORT", meta.QualityReasonCodes)
	}
}
