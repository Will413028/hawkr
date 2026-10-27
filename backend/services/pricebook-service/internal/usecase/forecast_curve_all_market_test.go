package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"go.uber.org/zap"
)

func TestAggregateForecastMarkets_QuantilesShareWeightsAndCoverage(t *testing.T) {
	date := forecast.NewDate(2026, 7, 26)
	makeOut := func(a, b, c float64) *SeriesForecastOutput {
		return &SeriesForecastOutput{Forecast: []ForecastPoint{{HorizonDays: 1, TargetDate: date, Status: forecast.PredictionStatusReady, P10: &a, P50: &b, P90: &c}}}
	}
	for _, tt := range []struct {
		name   string
		vol    map[string]float64
		bad    bool
		want   []float64
		method string
		status forecast.PredictionStatus
	}{
		{"weighted", map[string]float64{"109": 1, "110": 3}, false, []float64{25, 35, 45}, forecast.TrendAggregationVolumeWeightedV1, forecast.PredictionStatusReady},
		{"zero fallback", map[string]float64{"109": 0, "110": 0}, false, []float64{20, 30, 40}, "equal_weight_fallback", forecast.PredictionStatusReady},
		{"missing fallback", nil, false, []float64{20, 30, 40}, "equal_weight_fallback", forecast.PredictionStatusReady},
		{"zero excluded", map[string]float64{"109": 1}, false, []float64{10, 20, 30}, forecast.TrendAggregationVolumeWeightedV1, forecast.PredictionStatusPartial},
		{"missing quantile excludes whole market", map[string]float64{"109": 1, "110": 3}, true, []float64{10, 20, 30}, forecast.TrendAggregationVolumeWeightedV1, forecast.PredictionStatusPartial},
	} {
		t.Run(tt.name, func(t *testing.T) {
			outputs := map[string]*SeriesForecastOutput{"109": makeOut(10, 20, 30), "110": makeOut(30, 40, 50)}
			if tt.bad {
				outputs["110"].Forecast[0].P10 = nil
			}
			p := aggregateForecastMarkets([]string{"109", "110"}, outputs, tt.vol, date, 1, 28)
			for i, v := range []*float64{p.P10, p.P50, p.P90} {
				if v == nil || math.Abs(*v-tt.want[i]) > 1e-12 {
					t.Fatalf("quantile %d=%v want %v", i, v, tt.want[i])
				}
			}
			if p.Status != tt.status || p.Aggregation.Method != tt.method || p.Aggregation.WeightWindowDays != 28 {
				t.Fatalf("%+v %+v", p, p.Aggregation)
			}
		})
	}
	for _, status := range []forecast.PredictionStatus{forecast.PredictionStatusMarketClosed, forecast.PredictionStatusInsufficientData} {
		out := map[string]*SeriesForecastOutput{"109": {Forecast: []ForecastPoint{{HorizonDays: 1, Status: status}}}}
		p := aggregateForecastMarkets([]string{"109"}, out, nil, date, 1, 28)
		if p.Status != status || p.P50 != nil || len(p.Aggregation.ExcludedMarkets) != 1 {
			t.Fatalf("%+v", p)
		}
	}
}

func TestForecastCurve_AllMarketUsesSameSnapshotAndPrimaryErrorsAreFatal(t *testing.T) {
	now := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	date := forecast.NewDate(2026, 7, 25)
	artifact := testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", "N05", 118, 60), testSeriesModel("moa:110:A1", "moa:110", "moa:A1", "N05", 118, 60))
	_, cutoff, err := forecast.OriginDateAndCutoff(now, calendarTimezoneOf(artifact))
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(defaultTestInference())
	sentinel := errors.New("primary database unavailable")
	fail := false
	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, gotCutoff time.Time) ([]domain.Observation, error) {
			if !gotCutoff.Equal(cutoff) {
				t.Fatalf("cutoff=%v want %v", gotCutoff, cutoff)
			}
			if fail && mc == "110" {
				return nil, sentinel
			}
			return dailyObservations(mc, cc, strPtr("N05"), date.AddDays(-60), date, 55, 900, now.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeForCropFn: func(ctx context.Context, cc string, from, to, gotCutoff time.Time) ([]domain.MarketVolume, error) {
			if cc != "A1" || !gotCutoff.Equal(cutoff) || !from.Equal(date.AddDays(-cfg.Inference.GroupVolumeWindowDays).ToTime()) || !to.Equal(date.ToTime()) {
				t.Fatalf("volume window/cutoff mismatch %s %v %v %v", cc, from, to, gotCutoff)
			}
			return []domain.MarketVolume{{MarketCode: "109", Volume: 1}, {MarketCode: "110", Volume: 3}}, nil
		},
	}
	uc := NewForecastCurve(NewForecast(repo, cfg, zap.NewNop()), repo, mapping.Seed{}, artifact, cfg, zap.NewNop())
	result, err := uc.Get(context.Background(), "", "", "A1", "", now)
	if err != nil || result.Status != forecast.PredictionStatusReady || len(result.Points) != 7 || result.MarketCode != nil || len(result.Aggregation.ParticipatingMarketCodes) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	fail = true
	_, err = uc.Get(context.Background(), "", "", "A1", "", now)
	if !errors.Is(err, sentinel) {
		t.Fatalf("primary failure swallowed: %v", err)
	}
}

func TestForecastCurve_GroupComparisonFailureRetainsMarketIdentity(t *testing.T) {
	now := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	artifact := testArtifact(testSeriesModel("moa:110:A1", "moa:110", "moa:A1", "N05", 118, 60))
	repo := &fakeForecastRepo{seriesObservationsFn: func(context.Context, string, string, time.Time, time.Time, time.Time) ([]domain.Observation, error) {
		return nil, errors.New("database failure")
	}}
	cfg := testConfig(defaultTestInference())
	uc := NewForecastCurve(NewForecast(repo, cfg, zap.NewNop()), repo, mapping.Seed{}, artifact, cfg, zap.NewNop())
	result, err := uc.groupMarketComparison(context.Background(), []domain.CropRef{{CropCode: "A1", Active: true}}, "109", now)
	if err != nil || len(result) != 1 || result[0].MarketCode != "110" || result[0].P50 != nil || result[0].Status != forecast.PredictionStatusUnavailable || !result[0].IsApproximate {
		t.Fatalf("comparison=%+v err=%v", result, err)
	}
}

// TestForecastCurve_ComparisonRejectsUnusableRetainedQuantile 守的不變量是
// 「逐點狀態不可用、但點上仍留著 P50 的那種點，其數值不得外露進
// marketComparison」——三條呼叫路徑（單一市場、全市場、大類）都要成立。
//
// 2026-09-14：本測試原本靠新鮮度硬覆寫製造那個狀態（市場 110 的 available_at
// 給 96h 前，>72h 就被覆寫成 unavailable 但數值仍留在點上）。該覆寫已移除
// （見 ADR docs/adr/2026-09-14-pricebook-freshness-describes-age-not-usability.md），
// 那條產生途徑不復存在。改用 below_ridge_sample_threshold 這道逐點閘門：
// EvaluatePointGates 在 SampleCount < MinRidgeSampleCount 時 gateFailed，而
// applyGatesAndOuterStatus 無條件把 P10/P50/P90 抄到 ForecastPoint 上，因此
// 產生的正是同一種「狀態不可用、數值仍在」的點。不變量與三條路徑都沒變。
func TestForecastCurve_ComparisonRejectsUnusableRetainedQuantile(t *testing.T) {
	now := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	date := forecast.NewDate(2026, 7, 25)
	cfg := testConfig(defaultTestInference())
	// 市場 110 的 SampleCount 50 低於 defaultTestInference 的 MinRidgeSampleCount
	// 100，逐點閘門因此失敗；市場 109 的 118 過關，當對照組。
	artifact := testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", "N05", 118, 60), testSeriesModel("moa:110:A1", "moa:110", "moa:A1", "N05", 50, 60))
	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			// 兩個市場的新鮮度刻意相同——本測試與新鮮度無關，不讓它成為變因。
			return dailyObservations(mc, cc, strPtr("N05"), date.AddDays(-60), date, 55, 900, now.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
	}
	fc := NewForecast(repo, cfg, zap.NewNop())
	gated, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{Artifact: artifact, MarketCode: "110", CropCode: "A1", Origin: now})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := pointAtHorizon1(gated.Forecast)
	if !ok || p.Status != forecast.PredictionStatusInsufficientData || p.P50 == nil {
		t.Fatalf("fixture must retain the quantile on an unusable point: %+v", p)
	}
	if p.MissingReason != forecast.MissingReasonBelowRidgeSampleThreshold {
		t.Fatalf("fixture 應由 below_ridge_sample_threshold 造成，實得 %q", p.MissingReason)
	}
	uc := NewForecastCurve(fc, repo, mapping.Seed{}, artifact, cfg, zap.NewNop())
	assertUnusable := func(entries []MarketComparisonEntry) {
		t.Helper()
		for _, e := range entries {
			if e.MarketCode == "110" {
				if e.P50 != nil {
					t.Fatalf("unusable market exposed a price: %+v", e)
				}
				if e.Status == forecast.PredictionStatusReady || e.Status == forecast.PredictionStatusPartial {
					t.Fatalf("unusable market reported as usable: %+v", e)
				}
				return
			}
		}
		t.Fatal("unusable market identity missing")
	}
	single, err := uc.Get(context.Background(), "", "109", "A1", "", now)
	if err != nil {
		t.Fatal(err)
	}
	assertUnusable(single.MarketComparison)
	all, err := uc.Get(context.Background(), "", "", "A1", "", now)
	if err != nil {
		t.Fatal(err)
	}
	assertUnusable(all.MarketComparison)
	group, err := uc.groupMarketComparison(context.Background(), []domain.CropRef{{CropCode: "A1", Active: true}}, "109", now)
	if err != nil {
		t.Fatal(err)
	}
	assertUnusable(group)
}

func TestUsableComparisonPrice_StatusAndFiniteValue(t *testing.T) {
	for _, tt := range []struct {
		status forecast.PredictionStatus
		value  float64
		valid  bool
	}{
		{forecast.PredictionStatusReady, 0, true},
		{forecast.PredictionStatusPartial, 25, true},
		{forecast.PredictionStatusUnavailable, 25, false},
		{forecast.PredictionStatusMarketClosed, 25, false},
		{forecast.PredictionStatusInsufficientData, 25, false},
		{forecast.PredictionStatusReady, math.NaN(), false},
		{forecast.PredictionStatusPartial, math.Inf(1), false},
	} {
		got := usableComparisonPrice(ForecastPoint{Status: tt.status, P50: &tt.value})
		if (got != nil) != tt.valid {
			t.Fatalf("status=%s value=%v valid=%v", tt.status, tt.value, got != nil)
		}
	}
	if usableComparisonPrice(ForecastPoint{Status: forecast.PredictionStatusReady}) != nil {
		t.Fatal("nil price must remain nil")
	}
}

func TestForecastCurve_AllMarketRecentUnmodeledMarketMakesCoveragePartial(t *testing.T) {
	now := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	date := forecast.NewDate(2026, 7, 25)
	cfg := testConfig(defaultTestInference())
	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			if mc != "109" {
				t.Fatalf("unmodeled market forecast invoked: %s", mc)
			}
			return dailyObservations(mc, cc, strPtr("N05"), date.AddDays(-60), date, 55, 900, now.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeForCropFn: func(context.Context, string, time.Time, time.Time, time.Time) ([]domain.MarketVolume, error) {
			return []domain.MarketVolume{{MarketCode: "109", Volume: 1}, {MarketCode: "110", Volume: 3}}, nil
		},
	}
	artifact := testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", "N05", 118, 60))
	uc := NewForecastCurve(NewForecast(repo, cfg, zap.NewNop()), repo, mapping.Seed{}, artifact, cfg, zap.NewNop())
	result, err := uc.Get(context.Background(), "", "", "A1", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != forecast.PredictionStatusPartial || len(result.Points) != 7 {
		t.Fatalf("result=%+v", result)
	}
	for _, p := range result.Points {
		if p.P50 == nil || p.Status != forecast.PredictionStatusPartial || len(p.Aggregation.ExcludedMarkets) != 1 || p.Aggregation.ExcludedMarkets[0] != (ForecastMarketExclusion{MarketCode: "110", Reason: "unavailable"}) {
			t.Fatalf("point=%+v aggregation=%+v", p, p.Aggregation)
		}
	}
	if len(result.MarketComparison) != 2 || result.MarketComparison[1].MarketCode != "110" || result.MarketComparison[1].P50 != nil || result.MarketComparison[1].Status != forecast.PredictionStatusUnavailable {
		t.Fatalf("comparison=%+v", result.MarketComparison)
	}
	// The same observed universe with no model at all remains a business
	// unavailable result and does not invoke a nonexistent primary series.
	uc.artifact = forecast.ModelArtifact{}
	result, err = uc.Get(context.Background(), "", "", "A1", "", now)
	if err != nil || result.Status != forecast.PredictionStatusUnavailable || len(result.Points) != 7 {
		t.Fatalf("all unmodeled=%+v err=%v", result, err)
	}
	for _, p := range result.Points {
		if p.P50 != nil || p.Status != forecast.PredictionStatusUnavailable {
			t.Fatalf("all unmodeled point=%+v", p)
		}
	}
}
