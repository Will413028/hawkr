package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
)

func TestRegionalComparison_Get_SelectionValidation_ReturnsError(t *testing.T) {
	uc := NewRegionalComparison(forecast.ModelArtifact{}, testGeography(t), &fakeSeriesForecaster{}, nil)
	for name, in := range map[string]RegionalComparisonInput{
		"neither crop nor group": {HorizonDays: 1, Now: time.Now()},
		"both crop and group":    {CropCode: strPtr("A1"), GroupCode: strPtr("G1"), HorizonDays: 1, Now: time.Now()},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := uc.Get(context.Background(), in)
			require.ErrorIs(t, err, ErrRegionalComparisonExactlyOneSelection)
		})
	}
}

func TestRegionalComparison_Get_HorizonOutOfRange_ReturnsError(t *testing.T) {
	uc := NewRegionalComparison(forecast.ModelArtifact{}, testGeography(t), &fakeSeriesForecaster{}, nil)
	for _, horizon := range []int{0, -1, 8, 100} {
		_, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("A1"), HorizonDays: horizon, Now: time.Now()})
		require.ErrorIsf(t, err, ErrRegionalComparisonHorizonOutOfRange, "horizon %d", horizon)
	}
}

func TestRegionalComparison_Get_UnknownCrop_EveryRegionUnavailableWithReason(t *testing.T) {
	// A crop with zero series in the artifact must surface as a business
	// "unavailable" outcome on every region — issue #2248/#2249:
	// syntactically valid but unmodeled is 200 + status, never an error.
	artifact := forecast.ModelArtifact{Series: map[string]forecast.SeriesModel{}}
	uc := NewRegionalComparison(artifact, testGeography(t), &fakeSeriesForecaster{}, nil)
	out, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("ZZZ"), HorizonDays: 1, Now: time.Now()})
	require.NoError(t, err)
	require.NotEmpty(t, out.Regions)
	for _, region := range out.Regions {
		require.Equal(t, "unavailable", region.Status)
		require.Nil(t, region.P10)
		require.Nil(t, region.P50)
		require.Nil(t, region.P90)
		require.Contains(t, region.QualityReasonCodes, spatialReasonNoScoreableTradeRows)
	}
}

// Real fixture data (services/pricebook-service/reference/market_registry.v1.csv,
// active N04 rows): market 104 = 臺北市第二果菜批發市場, market 109 =
// 臺北市第一果菜批發市場 — both resolve to county_code 63000 (台北市).
// Market 800 = 高雄市果菜批發市場 → county_code 64000 (高雄市).

func TestRegionalComparison_Get_SingleMarketCounty_ReadyNotInsufficientData(t *testing.T) {
	// Design doc D2: 1 supporting market is enough for status=ready — the
	// quantile-averaging semantics (D1) never degrade with N=1, unlike a
	// cross-market percentile-of-P50s would.
	byKey := map[string]SeriesForecastOutput{
		"800:A1": {MarketCode: "800", CropCode: "A1", Freshness: forecast.FreshnessFresh, Forecast: []ForecastPoint{
			{HorizonDays: 1, Status: forecast.PredictionStatusReady, P10: floatPtr(20), P50: floatPtr(25), P90: floatPtr(35)},
		}},
	}
	artifact := spatialTestArtifact("A1", "N04", "800")
	uc := NewRegionalComparison(artifact, testGeography(t), &fakeSeriesForecaster{byKey: byKey}, nil)

	out, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)

	kaohsiung := findRegion(t, out.Regions, "64000")
	require.Equal(t, "ready", kaohsiung.Status)
	require.Equal(t, 1, kaohsiung.SupportingMarketCount)
	require.Equal(t, regionalComparisonAggregationMethod, kaohsiung.AggregationMethod)
	require.InDelta(t, 20.0, *kaohsiung.P10, 0.001)
	require.InDelta(t, 25.0, *kaohsiung.P50, 0.001)
	require.InDelta(t, 35.0, *kaohsiung.P90, 0.001)
	require.Equal(t, "fresh", kaohsiung.FreshnessStatus)
}

func TestRegionalComparison_Get_TwoMarketsSameCounty_EqualWeightAverage(t *testing.T) {
	// Crop path never queries recent volume (marketVolumesForStatistics
	// returns a nil map on the crop path — see that function's own doc
	// comment), so both markets fall back to equal weight 1: the region's
	// P50 must be the plain arithmetic mean of the two markets' own P50.
	byKey := map[string]SeriesForecastOutput{
		"104:A1": {MarketCode: "104", CropCode: "A1", Freshness: forecast.FreshnessFresh, Forecast: []ForecastPoint{
			{HorizonDays: 1, Status: forecast.PredictionStatusReady, P10: floatPtr(30), P50: floatPtr(40), P90: floatPtr(50)},
		}},
		"109:A1": {MarketCode: "109", CropCode: "A1", Freshness: forecast.FreshnessFresh, Forecast: []ForecastPoint{
			{HorizonDays: 1, Status: forecast.PredictionStatusReady, P10: floatPtr(50), P50: floatPtr(60), P90: floatPtr(70)},
		}},
	}
	artifact := spatialTestArtifact("A1", "N04", "104", "109")
	uc := NewRegionalComparison(artifact, testGeography(t), &fakeSeriesForecaster{byKey: byKey}, nil)

	out, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)

	taipei := findRegion(t, out.Regions, "63000")
	require.Equal(t, "ready", taipei.Status)
	require.Equal(t, 2, taipei.SupportingMarketCount)
	require.Empty(t, taipei.ExcludedMarketCodes)
	require.InDelta(t, 40.0, *taipei.P10, 0.001) // (30+50)/2
	require.InDelta(t, 50.0, *taipei.P50, 0.001) // (40+60)/2
	require.InDelta(t, 60.0, *taipei.P90, 0.001) // (50+70)/2
	require.Equal(t, "fresh", taipei.FreshnessStatus)
}

func TestRegionalComparison_Get_MarketWithNoPrice_ExcludedFromItsOwnCountyOnly(t *testing.T) {
	// market 104 forecasts nothing usable (no entry in byKey ->
	// fakeSeriesForecaster returns a zero-value SeriesForecastOutput, i.e.
	// no Forecast points) — must appear in county 63000's
	// excludedMarketCodes, and must not make any other county's list.
	artifact := spatialTestArtifact("A1", "N04", "104")
	uc := NewRegionalComparison(artifact, testGeography(t), &fakeSeriesForecaster{}, nil)

	out, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)

	for _, region := range out.Regions {
		if region.RegionCode == "63000" {
			require.Equal(t, "unavailable", region.Status)
			require.Equal(t, 0, region.SupportingMarketCount)
			require.Contains(t, region.ExcludedMarketCodes, "104")
			require.Contains(t, region.QualityReasonCodes, regionalComparisonReasonNoMarketsInRegion)
			require.Equal(t, "unavailable", region.FreshnessStatus)
		} else {
			require.NotContains(t, region.ExcludedMarketCodes, "104", "market 104 must not appear outside its own county (design doc D6)")
		}
	}
}

func TestRegionalComparison_Get_UnequalVolumes_WeightsTheAverage(t *testing.T) {
	// Directly exercises the true branch of regionalComparisonRegion's
	// weight lookup (volumes[m.marketCode], ok && v > 0) — every other
	// test in this file only exercises the equal-weight-1 fallback
	// because the crop path never populates a volumes map. This test
	// calls regionalComparisonRegion directly with a hand-built
	// []spatialMarket + volumes map, bypassing Get()/the resolver, since
	// there is no supported request path that produces group-path
	// per-market volumes in a single-crop, single-horizon, two-market
	// fixture without a full ComputeGroupVolumes setup — the aggregation
	// function itself is what's under test here, not the resolution
	// pipeline (already covered by the other tests and by Task 1's
	// regression suite).
	var taipei geography.CountyRegion
	for _, c := range testGeography(t).RegionsInScope() {
		if c.CountyCode == "63000" {
			taipei = c
			break
		}
	}
	require.Equal(t, "63000", taipei.CountyCode, "county_code 63000 (台北市) not found in the test geography registry")

	markets := []spatialMarket{
		{marketCode: "104", hasCounty: true, county: taipei, value: floatPtr(40), p10: floatPtr(30), p90: floatPtr(50), freshness: forecast.FreshnessFresh},
		{marketCode: "109", hasCounty: true, county: taipei, value: floatPtr(60), p10: floatPtr(50), p90: floatPtr(70), freshness: forecast.FreshnessFresh},
	}
	volumes := map[string]float64{"104": 3, "109": 1} // 3:1 weight, not equal

	region := regionalComparisonRegion(taipei, markets, volumes, "")

	require.Equal(t, "ready", region.Status)
	require.Equal(t, 2, region.SupportingMarketCount)
	// Weighted mean of (40,60) with weights (3,1) = (40*3+60*1)/4 = 45,
	// NOT the equal-weight arithmetic mean of 50 — this is the assertion
	// that would fail if the weight lookup used the wrong key or fell
	// back to equal weight incorrectly.
	require.InDelta(t, 45.0, *region.P50, 0.001)
	require.InDelta(t, 35.0, *region.P10, 0.001) // (30*3+50*1)/4
	require.InDelta(t, 55.0, *region.P90, 0.001) // (50*3+70*1)/4
	require.Equal(t, "fresh", region.FreshnessStatus)
}

func TestRegionalComparison_Get_SetsMappingVersion(t *testing.T) {
	// mapping_version identifies the crop/group/region mapping policy
	// version (issue #2403 V.7.2) — a distinct concept from
	// Lineage.ModelVersion (the forecast model's own version), and must be
	// set on every successful Get() regardless of whether the selection
	// itself resolves to any markets.
	uc := NewRegionalComparison(forecast.ModelArtifact{}, testGeography(t), &fakeSeriesForecaster{}, nil)
	out, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("ZZZ"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, CurrentMappingPolicyVersion, out.MappingVersion)
}

func findRegion(t *testing.T, regions []RegionalComparisonRegion, code string) RegionalComparisonRegion {
	t.Helper()
	for _, r := range regions {
		if r.RegionCode == code {
			return r
		}
	}
	t.Fatalf("region %q not found in Regions — every county in geography scope must always appear", code)
	return RegionalComparisonRegion{}
}

func TestRegionalComparison_Get_ClosedMarketWithConditionalPrice_StillSupportsItsCounty(t *testing.T) {
	byKey := map[string]SeriesForecastOutput{
		"800:A1": {MarketCode: "800", CropCode: "A1", Freshness: forecast.FreshnessFresh, Forecast: []ForecastPoint{
			{
				HorizonDays: 1,
				Status:      forecast.PredictionStatusMarketClosed,
				P10:         floatPtr(20), P50: floatPtr(25), P90: floatPtr(35),
			},
		}},
	}
	artifact := spatialTestArtifact("A1", "N04", "800")
	uc := NewRegionalComparison(artifact, testGeography(t), &fakeSeriesForecaster{byKey: byKey}, nil)

	out, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)

	kaohsiung := findRegion(t, out.Regions, "64000")
	require.Equal(t, "ready", kaohsiung.Status)
	require.Equal(t, 1, kaohsiung.SupportingMarketCount)
	require.Empty(t, kaohsiung.ExcludedMarketCodes)
	require.InDelta(t, 25.0, *kaohsiung.P50, 0.001)
}

func TestRegionalComparison_Get_ClosedMarketWithoutPrice_StillExcluded(t *testing.T) {
	byKey := map[string]SeriesForecastOutput{
		"800:A1": {MarketCode: "800", CropCode: "A1", Freshness: forecast.FreshnessFresh, Forecast: []ForecastPoint{
			{HorizonDays: 1, Status: forecast.PredictionStatusMarketClosed},
		}},
	}
	artifact := spatialTestArtifact("A1", "N04", "800")
	uc := NewRegionalComparison(artifact, testGeography(t), &fakeSeriesForecaster{byKey: byKey}, nil)

	out, err := uc.Get(context.Background(), RegionalComparisonInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)

	kaohsiung := findRegion(t, out.Regions, "64000")
	require.Equal(t, "unavailable", kaohsiung.Status)
	require.Contains(t, kaohsiung.ExcludedMarketCodes, "800")
}
