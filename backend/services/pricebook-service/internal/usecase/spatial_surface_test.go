package usecase

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
)

// fakeSeriesForecaster follows this package's existing hand-rolled-fake
// convention (see forecast_test.go, handler_test.go) — no mockgen/
// testify-mock in this codebase. Keyed by market_code + crop_code so the
// group path (several crops in one market) is expressible too.
type fakeSeriesForecaster struct {
	byKey map[string]SeriesForecastOutput
	err   error
	calls []string
}

func (f *fakeSeriesForecaster) SeriesForecast(ctx context.Context, in SeriesForecastInput) (*SeriesForecastOutput, error) {
	f.calls = append(f.calls, in.MarketCode+":"+in.CropCode)
	if f.err != nil {
		return nil, f.err
	}
	out, ok := f.byKey[in.MarketCode+":"+in.CropCode]
	if !ok {
		return &SeriesForecastOutput{MarketCode: in.MarketCode, CropCode: in.CropCode}, nil
	}
	return &out, nil
}

func testGeography(t *testing.T) geography.Registry {
	t.Helper()
	registry, err := geography.Load(
		"../../reference/market_registry.v1.csv", "../../reference/market_registry.v1.manifest.json",
		"../../reference/taiwan_boundary.taiwan-atlas-2021.9.20.geojson", "../../reference/taiwan_boundary.v1.manifest.json",
	)
	require.NoError(t, err)
	return registry
}

func testPriceScales(t *testing.T) map[string]spatial.PriceScale {
	t.Helper()
	scales, err := spatial.LoadPriceScales("../../reference/price_scale.v1.csv", "../../reference/price_scale.v1.manifest.json")
	require.NoError(t, err)
	return scales
}

func TestSpatialSurface_Get_UnknownCrop_ReturnsUnavailableStatus(t *testing.T) {
	// A crop with zero series in the artifact must surface as a business
	// "unavailable" outcome (issue #2248: syntactically valid but
	// unmodeled is 200 + status, never an error).
	artifact := forecast.ModelArtifact{Series: map[string]forecast.SeriesModel{}}
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, nil /* forecastUC, unused on this path */)
	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("ZZZ"), HorizonDays: 1, Now: time.Now()})
	require.NoError(t, err)
	require.Equal(t, "unavailable", out.Status)
	require.Contains(t, out.QualityReasonCodes, "no_scoreable_trade_rows")
}

// The two request-shape violations below are the ONLY error returns on this
// path (issue #2248 / design doc Error Handling): everything else — unmodeled
// crop, closed markets, too few points, missing boundary — is a business
// outcome carried by Status/QualityReasonCodes, asserted throughout the rest
// of this file.
func TestSpatialSurface_Get_SelectionValidation_ReturnsError(t *testing.T) {
	uc := NewSpatialSurface(forecast.ModelArtifact{}, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, nil)
	for name, in := range map[string]SpatialSurfaceInput{
		"neither crop nor group": {HorizonDays: 1, Now: time.Now()},
		"both crop and group":    {CropCode: strPtr("A1"), GroupCode: strPtr("G1"), HorizonDays: 1, Now: time.Now()},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := uc.Get(context.Background(), in)
			require.ErrorIs(t, err, ErrSpatialExactlyOneSelection)
		})
	}
}

func TestSpatialSurface_Get_HorizonOutOfRange_ReturnsError(t *testing.T) {
	uc := NewSpatialSurface(forecast.ModelArtifact{}, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, nil)
	for _, horizon := range []int{0, -1, 8, 100} {
		_, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: horizon, Now: time.Now()})
		require.ErrorIsf(t, err, ErrSpatialHorizonOutOfRange, "horizon %d", horizon)
	}
}

// ==================== 空間曲面 happy path ====================

const (
	spatialTestModelVersion  = "direct-ridge-v2-testfixture"
	spatialTestDatasetDigest = "02b9fd8cef27a5bff8a4be4ebf846f8e7d4e7f12617cda2ba6556ad7c740fffe"
)

var spatialTestOrigin = time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)

// spatialTestArtifact builds an artifact whose Series map covers exactly
// (marketCode, cropCode) for every marketCode given, all under one
// category. Only the identity fields the spatial pipeline reads
// (MarketID/CropID/CategoryCode) are populated — SeriesForecast itself is
// faked, so no ridge coefficients are needed.
func spatialTestArtifact(cropCode, categoryCode string, marketCodes ...string) forecast.ModelArtifact {
	series := make(map[string]forecast.SeriesModel, len(marketCodes))
	for _, mc := range marketCodes {
		series["moa:"+mc+":"+cropCode] = forecast.SeriesModel{
			SeriesID:     "moa:" + mc + ":" + cropCode,
			MarketID:     "moa:" + mc,
			CropID:       "moa:" + cropCode,
			CategoryCode: strPtr(categoryCode),
		}
	}
	return forecast.ModelArtifact{
		ModelVersion:  spatialTestModelVersion,
		DatasetDigest: spatialTestDatasetDigest,
		Series:        series,
		Metadata:      map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
}

// spatialReadyOutput is one market's faked 7-day forecast, all points ready,
// P50 = price (P10/P90 bracket it).
func spatialReadyOutput(marketCode, cropCode string, price float64) SeriesForecastOutput {
	points := make([]ForecastPoint, 0, 7)
	for h := 1; h <= 7; h++ {
		p10, p50, p90 := price*0.9, price, price*1.1
		points = append(points, ForecastPoint{
			HorizonDays: h, TargetDate: forecast.NewDate(2026, 8, 8).AddDays(h),
			Status: forecast.PredictionStatusReady, ModelKind: "direct_ridge",
			P10: &p10, P50: &p50, P90: &p90,
		})
	}
	through := forecast.NewDate(2026, 8, 7)
	availableAt := time.Date(2026, 8, 8, 1, 30, 0, 0, time.UTC)
	return SeriesForecastOutput{
		SeriesID: "moa:" + marketCode + ":" + cropCode, MarketCode: marketCode, CropCode: cropCode,
		Forecast: points, Freshness: forecast.FreshnessFresh, DataThroughDate: &through,
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		MaxFeatureAvailableAt: &availableAt,
	}
}

// spatialVegetableMarkets is a real, well-distributed subset of the N04
// (vegetable) facilities in reference/market_registry.v1.csv — Taipei
// through Pingtung plus the two east-coast markets — with hand-picked
// prices spanning a wide enough range (33 to 145) to exercise multiple
// adaptive breaks (issue #3102's DeriveAdaptiveScale), rather than any
// fixed CSV scale's bands.
var spatialVegetableMarkets = map[string]float64{
	"104": 42.0, "220": 38.0, "338": 55.0, "400": 61.0,
	"512": 74.0, "540": 88.0, "648": 66.0, "800": 120.0,
	"900": 145.0, "930": 96.0, "950": 33.0,
}

func spatialCropFixture(t *testing.T, cropCode string) (forecast.ModelArtifact, *fakeSeriesForecaster) {
	t.Helper()
	codes := make([]string, 0, len(spatialVegetableMarkets))
	byKey := make(map[string]SeriesForecastOutput, len(spatialVegetableMarkets))
	for mc, price := range spatialVegetableMarkets {
		codes = append(codes, mc)
		byKey[mc+":"+cropCode] = spatialReadyOutput(mc, cropCode, price)
	}
	return spatialTestArtifact(cropCode, "N04", codes...), &fakeSeriesForecaster{byKey: byKey}
}

func TestSpatialSurface_Get_CropWithMarkets_ReturnsReadySurface(t *testing.T) {
	artifact, forecaster := spatialCropFixture(t, "A1")
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)

	require.Equal(t, "ready", out.Status)
	require.Empty(t, out.QualityReasonCodes)
	require.Equal(t, "crop", out.SelectionKind)
	require.Equal(t, "A1", out.SelectionCode)
	require.Equal(t, "N04", out.CategoryCode)
	require.Equal(t, 1, out.HorizonDays)
	require.Equal(t, "2026-08-09", out.TargetDate.String())
	require.Equal(t, "TWD", out.Currency)
	require.Equal(t, "kg", out.Unit)

	// issue #3102: out.Scale on a successful build is no longer the
	// reviewed N04-price-scale-v1 policy's fixed breaks verbatim — it is
	// derived per-request from this call's own P50 domain. The ScaleID
	// still traces back to the reviewed base policy (adaptiveScaleID),
	// which is what #2248's approval gate continues to guarantee.
	require.True(t, strings.HasPrefix(out.Scale.ScaleID, "N04-price-scale-v1-adaptive-"), out.Scale.ScaleID)
	require.Equal(t, spatial.AdaptivePolicyV1, out.Scale.PolicyVersion)
	require.GreaterOrEqual(t, len(out.Scale.Breaks), 2)
	require.LessOrEqual(t, len(out.Scale.Breaks), 5)
	for i := 1; i < len(out.Scale.Breaks); i++ {
		require.Greater(t, out.Scale.Breaks[i], out.Scale.Breaks[i-1])
	}

	// Surface + contours are real geometry over the real Taiwan boundary.
	require.NotEmpty(t, out.SurfaceCells)
	require.NotEmpty(t, out.ContourSegments)
	require.Equal(t, "contours_and_points", out.DisplayMode)
	for _, cell := range out.SurfaceCells {
		require.Len(t, cell.PolygonWGS84, 5, "surface cell ring must be closed")
		require.Equal(t, cell.PolygonWGS84[0], cell.PolygonWGS84[4])
		require.GreaterOrEqual(t, cell.BandIndex, 0)
		require.LessOrEqual(t, cell.BandIndex, len(out.Scale.Breaks))
		require.InDelta(t, 120.9, cell.PolygonWGS84[0][0], 2.0, "cell longitude inside Taiwan")
		require.InDelta(t, 23.7, cell.PolygonWGS84[0][1], 2.5, "cell latitude inside Taiwan")
		// IDW output can never leave its inputs' range.
		require.GreaterOrEqual(t, cell.PriceTwdPerKg, 33.0)
		require.LessOrEqual(t, cell.PriceTwdPerKg, 145.0)
	}
	for _, seg := range out.ContourSegments {
		require.Contains(t, out.Scale.Breaks, seg.LevelValue)
		require.InDelta(t, 120.9, seg.Left[0], 2.0)
		require.InDelta(t, 23.7, seg.Left[1], 2.5)
	}

	// Every county in scope appears; the outlying islands have no markets,
	// so they come back with a null price rather than a fabricated 0 or a
	// national average (issue #2248's explicit rule).
	require.Len(t, out.CountyRollups, len(testGeography(t).RegionsInScope()))
	priced, unpriced := 0, 0
	for _, r := range out.CountyRollups {
		if r.EstimatedP50TwdPerKg == nil {
			unpriced++
			require.Equal(t, "unavailable", r.Status)
			require.Zero(t, r.SupportingCellCount)
			continue
		}
		priced++
		require.Equal(t, "ready", r.Status)
		require.Positive(t, r.SupportingCellCount)
		require.NotNil(t, r.Geometry)
	}
	require.Positive(t, priced)
	require.Positive(t, unpriced, "kinmen/matsu/penghu have no vegetable markets")

	// Per-market table: sorted, complete, and every market fed the surface.
	require.Len(t, out.MarketPrices, len(spatialVegetableMarkets))
	require.True(t, sort.SliceIsSorted(out.MarketPrices, func(i, j int) bool {
		return out.MarketPrices[i].MarketCode < out.MarketPrices[j].MarketCode
	}))
	for _, mp := range out.MarketPrices {
		require.Equal(t, "N04", mp.CategoryCode)
		require.Equal(t, string(forecast.PredictionStatusReady), mp.PredictionStatus)
		require.Equal(t, "open", mp.MarketStatus)
		require.True(t, mp.UsedForSurface, mp.MarketCode)
		require.Empty(t, mp.ExclusionReason)
		require.NotNil(t, mp.P50)
		require.InDelta(t, spatialVegetableMarkets[mp.MarketCode], *mp.P50, 1e-9)
		require.NotNil(t, mp.Longitude)
		require.NotNil(t, mp.Latitude)
		require.NotEmpty(t, mp.MarketName)
		require.NotEmpty(t, mp.CountyCode)
	}
	require.Empty(t, out.ExcludedMarketCodes)

	require.Equal(t, len(spatialVegetableMarkets), out.Statistics.TotalMarketCount)
	require.Equal(t, len(spatialVegetableMarkets), out.Statistics.PricedMarketCount)
	require.Zero(t, out.Statistics.ClosedMarketCount)
	require.Equal(t, len(out.SurfaceCells), out.Statistics.SurfaceCellCount)
	require.Equal(t, len(out.ContourSegments), out.Statistics.ContourSegmentCount)

	require.Equal(t, []string{
		"wholesale_market_location_not_production_origin",
		"not_quote_or_procurement_price_advice",
		"interpolated_surface_not_observed_between_markets",
	}, out.Disclosures)

	require.Equal(t, spatialTestModelVersion, out.Lineage.ModelVersion)
	require.Equal(t, spatialTestModelVersion, out.Lineage.ArtifactID)
	require.Equal(t, "sha256:"+spatialTestDatasetDigest, out.Lineage.DatasetDigest)
	require.NotEmpty(t, out.Lineage.BoundaryDigest)
	require.Equal(t, dataMetaSchemaVersion, out.Lineage.SchemaVersion)
	require.Equal(t, spatialTestOrigin, out.Lineage.GeneratedAt)
	require.NotNil(t, out.Lineage.SourceDataAsOf)
	require.NotNil(t, out.Lineage.MaxFeatureAvailableAt)

	// New in PR C: raw boundary geometry (separate from CountyRollups, which
	// carries the price-annotated choropleth layer) and the land-area
	// coverage ratio the quality gate already computed but used to discard.
	geo := testGeography(t)
	require.Equal(t, geo.BoundaryVersion(), out.Boundary.Version)
	require.Equal(t, geo.BoundaryDigest(), out.Boundary.Digest)
	require.Equal(t, geo.BBoxWGS84(), out.Boundary.BBoxWGS84)
	require.Len(t, out.Boundary.Regions, len(geo.RegionsInScope()))
	require.NotNil(t, out.SupportedLandAreaRatio, "a real grid was built for this fixture, ratio must be reported")
	require.GreaterOrEqual(t, *out.SupportedLandAreaRatio, 0.0)
	require.LessOrEqual(t, *out.SupportedLandAreaRatio, 1.0)
}

// TestSpatialSurface_Get_UnknownCrop_StillReportsBoundary pins that Boundary
// is populated on every finish() path, not just the happy path — it is a
// property of the mirrored geography data, independent of whether this
// request's selection produced a grid.
func TestSpatialSurface_Get_UnknownCrop_StillReportsBoundary(t *testing.T) {
	artifact := forecast.ModelArtifact{Series: map[string]forecast.SeriesModel{}}
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, nil)
	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("ZZZ"), HorizonDays: 1, Now: time.Now()})
	require.NoError(t, err)
	require.Equal(t, "unavailable", out.Status)

	require.NotEmpty(t, out.Boundary.Version)
	require.NotEmpty(t, out.Boundary.Digest)
	require.NotEmpty(t, out.Boundary.Regions)
	require.Nil(t, out.SupportedLandAreaRatio, "no grid was ever attempted on this path")
}

// TestSpatialSurface_Get_TooFewPointsToInterpolate_LandRatioStaysNil covers
// the buildSurface path that runs but never calls interpolateGrid for any
// region (every region has too few points) — landNodes stays 0, so the
// ratio must be nil, not a divide-by-zero or a fabricated 0.
func TestSpatialSurface_Get_TooFewPointsToInterpolate_LandRatioStaysNil(t *testing.T) {
	artifact, forecaster := spatialCropFixture(t, "A1")
	// Keep exactly one market's forecast usable — buildSurface's
	// spatialMinPoints gate (4) rejects every region before any grid builds.
	single := &fakeSeriesForecaster{byKey: map[string]SeriesForecastOutput{
		"104:A1": forecaster.byKey["104:A1"],
	}}
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), single, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Nil(t, out.SupportedLandAreaRatio)
}

// The same request must produce byte-identical geometry every time — Go's
// randomized map iteration over artifact.Series would otherwise reorder
// the IDW input locations and, through float addition, perturb the
// interpolated values (issue #2248: "同 selection/date/horizon 重取必須穩定").
func TestSpatialSurface_Get_IsDeterministicAcrossRuns(t *testing.T) {
	registry, scales := testGeography(t), testPriceScales(t)
	var first SpatialSurfaceOutput
	for run := 0; run < 3; run++ {
		artifact, forecaster := spatialCropFixture(t, "A1")
		uc := NewSpatialSurface(artifact, registry, scales, forecaster, nil)
		out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
		require.NoError(t, err)
		if run == 0 {
			first = out
			continue
		}
		require.Equal(t, first.SurfaceCells, out.SurfaceCells)
		require.Equal(t, first.ContourSegments, out.ContourSegments)
		require.Equal(t, first.CountyRollups, out.CountyRollups)
		require.Equal(t, first.MarketPrices, out.MarketPrices)
		require.Equal(t, first.Scale, out.Scale)
	}
}

// ==================== 大類（group）路徑 ====================

// spatialGroupForecast wires a real *Forecast around a fake repository —
// the group path only reaches Forecast for its CropsInGroup query (the
// per-series inference itself still goes through seriesForecaster), which
// is exactly the seam GroupForecast uses too.
func spatialGroupForecast(t *testing.T, crops []domain.CropRef, volumes ...domain.CropMarketVolume) *Forecast {
	t.Helper()
	logger, _ := testLogger()
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return crops, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return volumes, nil
		},
	}
	return NewForecast(repo, testConfig(defaultTestInference()), logger)
}

func TestSpatialSurface_Get_GroupPath_AveragesActiveCropsPerMarket(t *testing.T) {
	// Two active sub-variants plus one retired one. Each active variant is
	// modelled in a different (overlapping) set of markets, so the group's
	// market list must be the union and each market's value the mean of the
	// variants it actually carries.
	series := map[string]forecast.SeriesModel{}
	byKey := map[string]SeriesForecastOutput{}
	for mc, price := range spatialVegetableMarkets {
		for _, crop := range []string{"A1", "A2"} {
			if crop == "A2" && mc == "950" {
				continue // A2 is not sold in Hualien
			}
			series["moa:"+mc+":"+crop] = forecast.SeriesModel{
				SeriesID: "moa:" + mc + ":" + crop, MarketID: "moa:" + mc,
				CropID: "moa:" + crop, CategoryCode: strPtr("N04"),
			}
			offset := 0.0
			if crop == "A2" {
				offset = 20.0
			}
			byKey[mc+":"+crop] = spatialReadyOutput(mc, crop, price+offset)
		}
	}
	// A retired sub-variant that IS in the artifact must never be forecast.
	series["moa:104:A9"] = forecast.SeriesModel{
		SeriesID: "moa:104:A9", MarketID: "moa:104", CropID: "moa:A9", CategoryCode: strPtr("N04"),
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		Series: series, Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
	forecaster := &fakeSeriesForecaster{byKey: byKey}
	groupUC := spatialGroupForecast(t, []domain.CropRef{
		{CropCode: "A1", Active: true}, {CropCode: "A2", Active: true}, {CropCode: "A9", Active: false},
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)

	require.Equal(t, "group", out.SelectionKind)
	require.Equal(t, "G-N04", out.SelectionCode)
	require.Equal(t, "ready", out.Status)
	require.Empty(t, out.QualityReasonCodes)
	require.NotEmpty(t, out.SurfaceCells)
	// The group code has no scale row of its own; the category-level scale
	// the reference file actually ships is the eligibility fallback (issue
	// #2248), and issue #3102's adaptive ScaleID still traces back to it.
	require.True(t, strings.HasPrefix(out.Scale.ScaleID, "N04-price-scale-v1-adaptive-"), out.Scale.ScaleID)
	require.Equal(t, spatial.AdaptivePolicyV1, out.Scale.PolicyVersion)

	require.Len(t, out.MarketPrices, len(spatialVegetableMarkets))
	byMarket := map[string]SpatialMarketPrice{}
	for _, mp := range out.MarketPrices {
		byMarket[mp.MarketCode] = mp
	}
	// 104 carries both variants → mean of price and price+20.
	require.InDelta(t, spatialVegetableMarkets["104"]+10, *byMarket["104"].P50, 1e-9)
	// 950 carries only A1 → that variant's own price, not a hole.
	require.InDelta(t, spatialVegetableMarkets["950"], *byMarket["950"].P50, 1e-9)

	require.NotContains(t, forecaster.calls, "104:A9", "retired sub-variants must not be forecast")
	require.Contains(t, forecaster.calls, "104:A1")
	require.Contains(t, forecaster.calls, "104:A2")
	require.NotContains(t, forecaster.calls, "950:A2", "SeriesForecast must not be called for a (market, crop) the artifact does not cover")
}

// A group whose sub-variants are all retired (or which does not exist) is
// a business outcome, NOT the error GroupForecast returns for the same
// situation — issue #2248 requires 200 + status for a syntactically valid
// selection the model simply does not cover.
func TestSpatialSurface_Get_GroupWithNoActiveCrops_ReturnsUnavailableStatus(t *testing.T) {
	artifact, _ := spatialCropFixture(t, "A1")
	groupUC := spatialGroupForecast(t, []domain.CropRef{{CropCode: "A1", Active: false}})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-EMPTY"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "unavailable", out.Status)
	require.Equal(t, []string{"no_scoreable_trade_rows"}, out.QualityReasonCodes)
	require.Equal(t, "table_only", out.DisplayMode)
	require.Empty(t, out.SurfaceCells)
}

// The group dependency is genuinely required on the group path; a nil one
// is a wiring bug, not a business outcome, so it must not masquerade as an
// empty surface.
func TestSpatialSurface_Get_GroupPathWithoutForecastDependency_ReturnsError(t *testing.T) {
	uc := NewSpatialSurface(forecast.ModelArtifact{}, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, nil)
	_, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.Error(t, err)
	require.Contains(t, err.Error(), "group dependency is nil")
}

// ==================== qualityReasonCodes 業務結果 ====================

// spatialStatusOutput is one market's faked forecast where every point
// carries a non-ready status and no numbers (the shape SeriesForecast
// produces for a closed market or an unusable series).
func spatialStatusOutput(marketCode, cropCode string, status forecast.PredictionStatus, reason forecast.MissingReason) SeriesForecastOutput {
	points := make([]ForecastPoint, 0, 7)
	for h := 1; h <= 7; h++ {
		points = append(points, ForecastPoint{
			HorizonDays: h, TargetDate: forecast.NewDate(2026, 8, 8).AddDays(h),
			Status: status, MissingReason: reason,
		})
	}
	return SeriesForecastOutput{
		SeriesID: "moa:" + marketCode + ":" + cropCode, MarketCode: marketCode, CropCode: cropCode,
		Forecast: points, Freshness: forecast.FreshnessFresh,
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
	}
}

// spatialFixtureFor builds an artifact + forecaster over the given market
// codes, using outputFor to decide each market's faked forecast.
func spatialFixtureFor(cropCode, categoryCode string, marketCodes []string, outputFor func(marketCode string) SeriesForecastOutput) (forecast.ModelArtifact, *fakeSeriesForecaster) {
	byKey := map[string]SeriesForecastOutput{}
	for _, mc := range marketCodes {
		byKey[mc+":"+cropCode] = outputFor(mc)
	}
	return spatialTestArtifact(cropCode, categoryCode, marketCodes...), &fakeSeriesForecaster{byKey: byKey}
}

// issue #3102: on the happy path, out.Scale is no longer the reviewed
// fixed CSV policy verbatim — it is derived per-request from this same
// call's own P50 domain (minimum/median/maximum), banded via
// spatial.DeriveAdaptiveScale.
func TestSpatialSurface_Get_CropPath_ReturnsAdaptiveScale(t *testing.T) {
	codes := []string{"104", "220", "400", "512", "800", "900"}
	prices := map[string]float64{
		"104": 20, "220": 25, "400": 30, "512": 35, "800": 40, "900": 45,
	}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialReadyOutput(mc, "A1", prices[mc])
	})
	priceScales := testPriceScales(t)
	uc := NewSpatialSurface(artifact, testGeography(t), priceScales, forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, spatial.AdaptivePolicyV1, out.Scale.PolicyVersion)
	require.GreaterOrEqual(t, len(out.Scale.Breaks), 2)
	require.LessOrEqual(t, len(out.Scale.Breaks), 5)
	require.NotNil(t, out.Statistics.MinimumP50)
	require.NotNil(t, out.Statistics.MaximumP50)
	for _, b := range out.Scale.Breaks {
		require.Greater(t, b, *out.Statistics.MinimumP50)
		require.Less(t, b, *out.Statistics.MaximumP50)
	}
	for i := 1; i < len(out.Scale.Breaks); i++ {
		require.Greater(t, out.Scale.Breaks[i], out.Scale.Breaks[i-1])
	}

	// issue #3102 final review Fix 7: bind out.Scale to out.Statistics
	// exactly, so a Minimum/Median swap bug (or any wrong-field mapping) in
	// Get()'s wiring would fail this test even though it would still pass
	// the range/monotonicity-only assertions above. Recomputes
	// DeriveAdaptiveScale from the same fields Get() actually feeds it —
	// this call's own out.Statistics, plus the same identity fields:
	// basePolicy.ScaleID from the CSV-loaded fixture (no crop-specific row
	// for A1, so this selection falls back to the group:N04 row) and
	// artifact.ModelVersion straight from the fixture, not a hardcoded
	// guess.
	basePolicy := priceScales["group:N04"]
	expectedScale := spatial.DeriveAdaptiveScale(
		spatial.AdaptiveScaleDomain{
			Minimum: *out.Statistics.MinimumP50,
			Median:  *out.Statistics.P50,
			Maximum: *out.Statistics.MaximumP50,
		},
		spatial.AdaptiveScaleIdentity{
			SelectionKind: out.SelectionKind,
			SelectionCode: out.SelectionCode,
			TargetDate:    out.TargetDate.String(),
			HorizonDays:   1,
			ModelVersion:  artifact.ModelVersion,
			BasePolicyID:  basePolicy.ScaleID,
		},
	)
	require.Equal(t, expectedScale, out.Scale)
}

// issue #3102 acceptance criteria: two different selections must be able to
// come back with different adaptive breaks — the point of moving off a
// single category-wide fixed scale. Reuses the crop-path fixture from
// TestSpatialSurface_Get_CropPath_ReturnsAdaptiveScale (prices 20..45 across
// 6 markets) and the group-path fixture from
// TestSpatialSurface_Get_GroupPath_AveragesActiveCropsPerMarket
// (spatialVegetableMarkets prices, 33..145, averaged across A1/A2
// sub-variants) — materially different price distributions, so their
// derived breaks must differ too.
func TestSpatialSurface_Get_DifferentSelections_ProduceDifferentAdaptiveBreaks(t *testing.T) {
	codes := []string{"104", "220", "400", "512", "800", "900"}
	prices := map[string]float64{
		"104": 20, "220": 25, "400": 30, "512": 35, "800": 40, "900": 45,
	}
	artifact1, forecaster1 := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialReadyOutput(mc, "A1", prices[mc])
	})
	uc1 := NewSpatialSurface(artifact1, testGeography(t), testPriceScales(t), forecaster1, nil)
	out1, err := uc1.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, spatial.AdaptivePolicyV1, out1.Scale.PolicyVersion)

	series := map[string]forecast.SeriesModel{}
	byKey := map[string]SeriesForecastOutput{}
	for mc, price := range spatialVegetableMarkets {
		for _, crop := range []string{"A1", "A2"} {
			if crop == "A2" && mc == "950" {
				continue // A2 is not sold in Hualien
			}
			series["moa:"+mc+":"+crop] = forecast.SeriesModel{
				SeriesID: "moa:" + mc + ":" + crop, MarketID: "moa:" + mc,
				CropID: "moa:" + crop, CategoryCode: strPtr("N04"),
			}
			offset := 0.0
			if crop == "A2" {
				offset = 20.0
			}
			byKey[mc+":"+crop] = spatialReadyOutput(mc, crop, price+offset)
		}
	}
	artifact2 := forecast.ModelArtifact{
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		Series: series, Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
	forecaster2 := &fakeSeriesForecaster{byKey: byKey}
	groupUC := spatialGroupForecast(t, []domain.CropRef{
		{CropCode: "A1", Active: true}, {CropCode: "A2", Active: true},
	})
	uc2 := NewSpatialSurface(artifact2, testGeography(t), testPriceScales(t), forecaster2, groupUC)
	out2, err := uc2.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, spatial.AdaptivePolicyV1, out2.Scale.PolicyVersion)

	require.NotEqual(t, out1.Scale.Breaks, out2.Scale.Breaks)
}

// issue #3102: every market in scope reporting the exact same P50 makes the
// domain flat (Minimum == Median == Maximum) — DeriveAdaptiveScale then has
// no candidate break strictly between them, so it falls back to
// AdaptivePolicyV1FlatFallback with an empty Breaks slice rather than
// inventing a fake band boundary.
func TestSpatialSurface_Get_FlatDomain_ReturnsFlatFallbackScale(t *testing.T) {
	codes := []string{"104", "220", "400", "512", "800", "900"}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialReadyOutput(mc, "A1", 30) // 每個市場同一個價格 → domain 完全平坦
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, spatial.AdaptivePolicyV1FlatFallback, out.Scale.PolicyVersion)
	require.Empty(t, out.Scale.Breaks)
	// flat-fallback is still a legitimate "computed" outcome, not
	// unavailable — cells still get built (just all in one implicit band).
	// But every market sharing one exact price also makes buildSurface's
	// own interpolated field flat within each region, which is a
	// pre-existing, unrelated quality gate (spatial_surface.go's
	// flatValues/spatialReasonFlatPriceField, predates issue #3102): a
	// completely flat price field always degrades, regardless of which
	// scale banded it. Confirmed by an actual run rather than assumed —
	// see task-6-report.md.
	require.Equal(t, "degraded", out.Status)
	require.Equal(t, []string{"flat_price_field"}, out.QualityReasonCodes)
	require.NotEmpty(t, out.SurfaceCells)
}

func TestSpatialSurface_Get_AllMarketsClosed_ReturnsUnavailableNotError(t *testing.T) {
	codes := []string{"104", "220", "400", "512", "800"}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialStatusOutput(mc, "A1", forecast.PredictionStatusMarketClosed, forecast.MissingReasonMarketClosed)
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err, "a closed market day is a business outcome, never an error")
	require.Equal(t, "unavailable", out.Status)
	require.Equal(t, []string{"all_scoped_markets_closed", "insufficient_spatial_coverage"}, out.QualityReasonCodes)
	// The markets themselves are still reported — the seller sees "these
	// markets are closed", not an empty screen.
	require.Equal(t, "points_only", out.DisplayMode)
	require.Len(t, out.MarketPrices, len(codes))
	require.Equal(t, len(codes), out.Statistics.ClosedMarketCount)
	require.Zero(t, out.Statistics.PricedMarketCount)
	for _, mp := range out.MarketPrices {
		require.Equal(t, "closed", mp.MarketStatus)
		require.Nil(t, mp.P50)
		require.False(t, mp.UsedForSurface)
		// A closed market is geocoded and in scope — it simply has no
		// number today. Saying "outside_or_unsupported_domain" here would
		// tell the seller their market is off the map.
		require.Equal(t, "no_usable_price", mp.ExclusionReason)
	}
	require.ElementsMatch(t, codes, out.ExcludedMarketCodes)
	// Every county is still listed, each with a null price rather than a
	// fabricated 0 (issue #2248's explicit rule) — the choropleth renders
	// as "no data everywhere", not as a free nationwide price of zero.
	require.Len(t, out.CountyRollups, len(testGeography(t).RegionsInScope()))
	for _, r := range out.CountyRollups {
		require.Nil(t, r.EstimatedP50TwdPerKg)
		require.Equal(t, "unavailable", r.Status)
	}

	// issue #3102 final review Fix 8: pin the invariant a downstream
	// consumer (the website's lib/pricebook/repo.ts) relies on —
	// flat-fallback with genuinely no P50 data (weightedMedianP50 returns
	// nil, nil, nil together when every market is closed) must always
	// coincide with status=unavailable, never surface as a
	// misleadingly-successful flat-fallback response with no underlying
	// statistics.
	require.Equal(t, spatial.AdaptivePolicyV1FlatFallback, out.Scale.PolicyVersion)
	require.Empty(t, out.Scale.Breaks)
	require.Nil(t, out.Statistics.MinimumP50)
	require.Nil(t, out.Statistics.P50)
	require.Nil(t, out.Statistics.MaximumP50)
}

func TestSpatialSurface_Get_TooFewPointsToInterpolate_ReturnsUnavailable(t *testing.T) {
	// spatialMinPoints is 4; three geocoded markets can never produce a
	// surface no matter how good their numbers are.
	codes := []string{"104", "400", "800"}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialReadyOutput(mc, "A1", 50)
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "unavailable", out.Status)
	require.Equal(t, []string{
		"insufficient_spatial_coverage", "insufficient_spatial_points", "unsupported_boundary_regions",
	}, out.QualityReasonCodes)
	require.Empty(t, out.SurfaceCells)
	require.Equal(t, "points_only", out.DisplayMode)
	// The prices are still real and still shown; only the map is missing.
	require.Equal(t, 3, out.Statistics.PricedMarketCount)
	// No grid was ever built, so no market fed a surface. Claiming
	// usedForSurface on a response with zero cells would be a lie the
	// client has no way to detect.
	require.ElementsMatch(t, codes, out.ExcludedMarketCodes)
	for _, mp := range out.MarketPrices {
		require.False(t, mp.UsedForSurface, mp.MarketCode)
		require.Equal(t, "outside_or_unsupported_domain", mp.ExclusionReason, mp.MarketCode)
	}
}

func TestSpatialSurface_Get_NoApprovedPriceScale_ReturnsUnavailable(t *testing.T) {
	// A selection with no approved scale. Banding against invented breaks
	// is exactly the per-request scale issue #2248 forbids, so the surface
	// is withheld rather than guessed.
	//
	// The markets here are real, geocoded, in scope and priced — only the
	// scale is missing. That is deliberate: it is the case where
	// pre-build eligibility and actual grid participation disagree, so it
	// pins usedForSurface to the latter.
	codes := []string{"104", "220", "400", "512", "800", "900"}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialReadyOutput(mc, "A1", 50)
	})
	uc := NewSpatialSurface(artifact, testGeography(t), map[string]spatial.PriceScale{}, forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "unavailable", out.Status)
	require.Equal(t, []string{"price_scale_unavailable"}, out.QualityReasonCodes)
	require.Empty(t, out.Scale.ScaleID)
	require.Empty(t, out.Scale.Breaks)
	require.Len(t, out.MarketPrices, len(codes))
	// Same rule as the too-few-points path: the surface step never ran, so
	// nothing is used — even though every one of these markets WOULD have
	// been eligible had there been a scale to band against.
	require.ElementsMatch(t, codes, out.ExcludedMarketCodes)
	for _, mp := range out.MarketPrices {
		require.NotNil(t, mp.P50, mp.MarketCode)
		require.NotNil(t, mp.Longitude, mp.MarketCode)
		require.NotEmpty(t, mp.CountyCode, mp.MarketCode)
		require.False(t, mp.UsedForSurface, mp.MarketCode)
		require.Equal(t, "outside_or_unsupported_domain", mp.ExclusionReason, mp.MarketCode)
	}
}

func TestSpatialSurface_Get_BoundaryUnavailable_ReturnsUnavailable(t *testing.T) {
	// A zero-value Registry is what a service would hold if the mirrored
	// boundary file were ever missing at startup — no counties in scope, so
	// there is nothing to interpolate over or roll up onto.
	codes := []string{"104", "220", "400", "512", "800"}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialReadyOutput(mc, "A1", 50)
	})
	uc := NewSpatialSurface(artifact, geography.Registry{}, testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "unavailable", out.Status)
	require.Contains(t, out.QualityReasonCodes, "boundary_unavailable")
	require.Empty(t, out.SurfaceCells)
	require.Empty(t, out.CountyRollups)
	// Without geography there are no coordinates either, so there is not
	// even a point layer left to draw.
	require.Equal(t, "table_only", out.DisplayMode)
}

func TestSpatialSurface_Get_MarketWithoutGeography_DegradesButStillDraws(t *testing.T) {
	// A market the model covers but the geography registry does not (an
	// upstream mirror that has not caught up). The surface is still drawn
	// from the markets that DO resolve, and the gap is declared rather than
	// hidden.
	codes := make([]string, 0, len(spatialVegetableMarkets)+1)
	for mc := range spatialVegetableMarkets {
		codes = append(codes, mc)
	}
	codes = append(codes, "999")
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		price, ok := spatialVegetableMarkets[mc]
		if !ok {
			price = 70
		}
		return spatialReadyOutput(mc, "A1", price)
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "degraded", out.Status)
	require.Equal(t, []string{"missing_market_geocodes"}, out.QualityReasonCodes)
	require.NotEmpty(t, out.SurfaceCells)
	require.Equal(t, "contours_and_points", out.DisplayMode)
	require.Equal(t, []string{"999"}, out.ExcludedMarketCodes)
	for _, mp := range out.MarketPrices {
		if mp.MarketCode != "999" {
			continue
		}
		require.False(t, mp.UsedForSurface)
		require.Equal(t, "missing_coordinate", mp.ExclusionReason)
		require.Nil(t, mp.Longitude)
		require.NotNil(t, mp.P50, "an ungeocoded market still has a real price to show in the table")
	}
}

func TestSpatialSurface_Get_EveryMarketSamePrice_FlagsFlatField(t *testing.T) {
	codes := make([]string, 0, len(spatialVegetableMarkets))
	for mc := range spatialVegetableMarkets {
		codes = append(codes, mc)
	}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		return spatialReadyOutput(mc, "A1", 88)
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "degraded", out.Status)
	require.Contains(t, out.QualityReasonCodes, "flat_price_field")
	require.NotEmpty(t, out.SurfaceCells)
	// issue #3102: every market sharing one exact price also makes the
	// adaptive P50 domain flat, so DeriveAdaptiveScale falls back to
	// AdaptivePolicyV1FlatFallback with empty Breaks — band()'s Task 5 fix
	// for that case returns band index 0 uniformly, not a band derived from
	// the old fixed CSV breaks.
	require.Empty(t, out.Scale.Breaks)
	for _, cell := range out.SurfaceCells {
		require.InDelta(t, 88.0, cell.PriceTwdPerKg, 1e-9, "a flat input field must interpolate to a flat surface")
		require.Equal(t, 0, cell.BandIndex)
	}
	// No isoline can cross a constant field, so the surface is drawn
	// without contours.
	require.Empty(t, out.ContourSegments)
	require.Equal(t, "surface_and_points", out.DisplayMode)
}

// Requesting a later horizon must read that horizon's point, not silently
// fall back to D+1 (the bug pointAtHorizon exists to prevent — its
// hardcoded ancestor pointAtHorizon1 is still used elsewhere).
func TestSpatialSurface_Get_HonoursRequestedHorizon(t *testing.T) {
	codes := make([]string, 0, len(spatialVegetableMarkets))
	for mc := range spatialVegetableMarkets {
		codes = append(codes, mc)
	}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		out := spatialReadyOutput(mc, "A1", spatialVegetableMarkets[mc])
		// Only horizon 5 carries a usable number.
		for i := range out.Forecast {
			if out.Forecast[i].HorizonDays != 5 {
				out.Forecast[i].Status = forecast.PredictionStatusInsufficientData
				out.Forecast[i].P10, out.Forecast[i].P50, out.Forecast[i].P90 = nil, nil, nil
			}
		}
		return out
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 5, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "ready", out.Status)
	require.Equal(t, 5, out.HorizonDays)
	require.Equal(t, "2026-08-13", out.TargetDate.String())

	degraded, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "unavailable", degraded.Status, "horizon 1 has no usable point in this fixture")
}

// A market whose contributing points are `partial` still feeds the surface
// (same rule priceResultFromPoint applies), but the per-market row must
// keep saying `partial` — collapsing it to `ready` would overstate the
// confidence of a number the seller is looking at.
func TestSpatialSurface_Get_PartialPoints_KeepTheirStatus(t *testing.T) {
	codes := make([]string, 0, len(spatialVegetableMarkets))
	for mc := range spatialVegetableMarkets {
		codes = append(codes, mc)
	}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		out := spatialReadyOutput(mc, "A1", spatialVegetableMarkets[mc])
		for i := range out.Forecast {
			out.Forecast[i].Status = forecast.PredictionStatusPartial
		}
		return out
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "ready", out.Status)
	require.NotEmpty(t, out.SurfaceCells)
	for _, mp := range out.MarketPrices {
		require.Equal(t, string(forecast.PredictionStatusPartial), mp.PredictionStatus)
		require.True(t, mp.UsedForSurface)
	}
}

// targetDate comes from the forecast's own calendar-aware point, not from
// a local origin+horizon recomputation — and the first market in sorted
// order may be one that produced no point at all, so it must not decide.
func TestSpatialSurface_Get_TargetDateComesFromAProducedPoint(t *testing.T) {
	codes := make([]string, 0, len(spatialVegetableMarkets))
	for mc := range spatialVegetableMarkets {
		codes = append(codes, mc)
	}
	artifact, forecaster := spatialFixtureFor("A1", "N04", codes, func(mc string) SeriesForecastOutput {
		out := spatialReadyOutput(mc, "A1", spatialVegetableMarkets[mc])
		if mc == "104" { // lowest market code — sorted first
			out.Forecast = nil
			return out
		}
		for i := range out.Forecast {
			out.Forecast[i].TargetDate = out.Forecast[i].TargetDate.AddDays(1)
		}
		return out
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "2026-08-10", out.TargetDate.String())
	require.Equal(t, "no_usable_price", out.MarketPrices[0].ExclusionReason)
	require.Equal(t, "104", out.MarketPrices[0].MarketCode)
}

// ==================== Review round 2: Important 1-4 ====================

// The group path must weight a market's sub-variants by recent volume, the
// same way GroupForecast/GroupObservedSeries/TrendSummary already do —
// otherwise this RPC and the pages beside it would answer "what is this
// group worth at market X" differently. Under the old unweighted mean the
// answer here would be base+10 for every market.
func TestSpatialSurface_Get_GroupPath_WeightsSubVariantsByVolume(t *testing.T) {
	series := map[string]forecast.SeriesModel{}
	byKey := map[string]SeriesForecastOutput{}
	volumes := make([]domain.CropMarketVolume, 0, 2*len(spatialVegetableMarkets))
	for mc, price := range spatialVegetableMarkets {
		for _, crop := range []string{"A1", "A2"} {
			series["moa:"+mc+":"+crop] = forecast.SeriesModel{
				SeriesID: "moa:" + mc + ":" + crop, MarketID: "moa:" + mc,
				CropID: "moa:" + crop, CategoryCode: strPtr("N04"),
			}
			offset, volume := 0.0, 300.0
			if crop == "A2" {
				offset, volume = 20.0, 100.0
			}
			byKey[mc+":"+crop] = spatialReadyOutput(mc, crop, price+offset)
			volumes = append(volumes, domain.CropMarketVolume{MarketCode: mc, CropCode: crop, Volume: volume})
		}
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		Series: series, Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
	groupUC := spatialGroupForecast(t,
		[]domain.CropRef{{CropCode: "A1", Active: true}, {CropCode: "A2", Active: true}},
		volumes...)
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{byKey: byKey}, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "ready", out.Status)
	for _, mp := range out.MarketPrices {
		base := spatialVegetableMarkets[mp.MarketCode]
		// (base*300 + (base+20)*100) / 400 == base + 5, not the base + 10 a
		// simple mean would give.
		require.InDeltaf(t, base+5, *mp.P50, 1e-9, "market %s must be volume-weighted", mp.MarketCode)
		require.InDelta(t, (base+5)*0.9, *mp.P10, 1e-9)
		require.InDelta(t, (base+5)*1.1, *mp.P90, 1e-9)
	}
}

// With no recent volume at all, WeightedAverage's own documented
// equal-weight fallback applies — this must not become a second rule
// invented here, and must not produce a zero or a nil.
func TestSpatialSurface_Get_GroupPath_NoVolumeFallsBackToEqualWeight(t *testing.T) {
	series := map[string]forecast.SeriesModel{}
	byKey := map[string]SeriesForecastOutput{}
	for mc, price := range spatialVegetableMarkets {
		for _, crop := range []string{"A1", "A2"} {
			series["moa:"+mc+":"+crop] = forecast.SeriesModel{
				SeriesID: "moa:" + mc + ":" + crop, MarketID: "moa:" + mc,
				CropID: "moa:" + crop, CategoryCode: strPtr("N04"),
			}
			offset := 0.0
			if crop == "A2" {
				offset = 20.0
			}
			byKey[mc+":"+crop] = spatialReadyOutput(mc, crop, price+offset)
		}
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		Series: series, Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
	groupUC := spatialGroupForecast(t, []domain.CropRef{{CropCode: "A1", Active: true}, {CropCode: "A2", Active: true}})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{byKey: byKey}, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	for _, mp := range out.MarketPrices {
		require.InDelta(t, spatialVegetableMarkets[mp.MarketCode]+10, *mp.P50, 1e-9)
	}
}

// Severity ordering, not "whichever sub-variant sorted first". A1 sorts
// before A2, so under the old fold every one of these markets would report
// `ready` while half its contributing variants were only `partial`.
func TestSpatialSurface_Get_MixedSubVariantSeverity_ReportsTheWorst(t *testing.T) {
	series := map[string]forecast.SeriesModel{}
	byKey := map[string]SeriesForecastOutput{}
	for mc, price := range spatialVegetableMarkets {
		for _, crop := range []string{"A1", "A2", "A3"} {
			series["moa:"+mc+":"+crop] = forecast.SeriesModel{
				SeriesID: "moa:" + mc + ":" + crop, MarketID: "moa:" + mc,
				CropID: "moa:" + crop, CategoryCode: strPtr("N04"),
			}
			out := spatialReadyOutput(mc, crop, price)
			switch crop {
			case "A2": // usable but only partial
				for i := range out.Forecast {
					out.Forecast[i].Status = forecast.PredictionStatusPartial
				}
			case "A3": // closed — the worst of the three
				out = spatialStatusOutput(mc, crop, forecast.PredictionStatusMarketClosed, forecast.MissingReasonMarketClosed)
			}
			byKey[mc+":"+crop] = out
		}
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		Series: series, Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
	groupUC := spatialGroupForecast(t, []domain.CropRef{
		{CropCode: "A1", Active: true}, {CropCode: "A2", Active: true}, {CropCode: "A3", Active: true},
	})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{byKey: byKey}, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "ready", out.Status)
	require.NotEmpty(t, out.SurfaceCells)
	for _, mp := range out.MarketPrices {
		require.Equalf(t, string(forecast.PredictionStatusPartial), mp.PredictionStatus,
			"market %s mixes ready+partial+closed sub-variants; the row must report the worst contributing one", mp.MarketCode)
		require.True(t, mp.UsedForSurface)
	}
}

// A market where the model covers three sub-variants but only one produced
// a usable point is `partial` — the number is real, just built from fewer
// variants than the market carries (AggregateGroup's own semantics).
func TestSpatialSurface_Get_SomeSubVariantsUnusable_ReportsPartial(t *testing.T) {
	series := map[string]forecast.SeriesModel{}
	byKey := map[string]SeriesForecastOutput{}
	for mc, price := range spatialVegetableMarkets {
		for _, crop := range []string{"A1", "A2"} {
			series["moa:"+mc+":"+crop] = forecast.SeriesModel{
				SeriesID: "moa:" + mc + ":" + crop, MarketID: "moa:" + mc,
				CropID: "moa:" + crop, CategoryCode: strPtr("N04"),
			}
			if crop == "A2" {
				byKey[mc+":"+crop] = spatialStatusOutput(mc, crop, forecast.PredictionStatusInsufficientData, forecast.MissingReasonInsufficientRecentData)
				continue
			}
			byKey[mc+":"+crop] = spatialReadyOutput(mc, crop, price)
		}
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		Series: series, Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
	groupUC := spatialGroupForecast(t, []domain.CropRef{{CropCode: "A1", Active: true}, {CropCode: "A2", Active: true}})
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{byKey: byKey}, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	for _, mp := range out.MarketPrices {
		require.Equal(t, string(forecast.PredictionStatusPartial), mp.PredictionStatus, mp.MarketCode)
		require.InDelta(t, spatialVegetableMarkets[mp.MarketCode], *mp.P50, 1e-9)
	}
}

func TestSpatialSurface_Get_ZeroNow_ReturnsError(t *testing.T) {
	uc := NewSpatialSurface(forecast.ModelArtifact{}, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, nil)
	_, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1})
	require.ErrorIs(t, err, ErrSpatialZeroNow)
}

// Even a completely unmodelled selection reports which date it was asked
// about — a response whose targetDate is the zero date is unreadable.
func TestSpatialSurface_Get_UnknownCrop_StillReportsTargetDate(t *testing.T) {
	uc := NewSpatialSurface(forecast.ModelArtifact{Series: map[string]forecast.SeriesModel{}},
		testGeography(t), testPriceScales(t), &fakeSeriesForecaster{}, nil)
	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("ZZZ"), HorizonDays: 3, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "unavailable", out.Status)
	require.Equal(t, "2026-08-11", out.TargetDate.String())
	require.Equal(t, 3, out.HorizonDays)
}

// spatialQueryCounter counts the repository reads SeriesForecast issues, so
// the request-path DB fan-out can be measured rather than asserted about.
//
// restDaysFor / coverageFor / observationsFor, when set, make the fake
// answer depend on its arguments instead of returning the same empty result
// to everyone — see spatialClosedOn and
// TestSpatialSurface_Get_CalendarMemoDoesNotChangeResults for why a fake
// that ignores its arguments cannot tell a correct memo from a broken one.
// All three default to nil, which keeps the argument-blind behaviour the
// query-counting test above wants.
type spatialQueryCounter struct {
	seriesObservations int
	restDays           int
	restDayCoverage    int

	restDaysFor     func(marketCode string, from, to time.Time) []domain.RestDay
	coverageFor     func(marketCode string) []domain.RestDayCoverage
	observationsFor func(marketCode, cropCode string) []domain.Observation
}

func (c *spatialQueryCounter) repo(crops []domain.CropRef) *fakeForecastRepo {
	return &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) { return crops, nil },
		seriesObservationsFn: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			c.seriesObservations++
			if c.observationsFor == nil {
				return nil, nil
			}
			return c.observationsFor(marketCode, cropCode), nil
		},
		restDaysFn: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			c.restDays++
			if c.restDaysFor == nil {
				return nil, nil
			}
			return c.restDaysFor(marketCode, from, to), nil
		},
		restDayCoverageFn: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
			c.restDayCoverage++
			if c.coverageFor == nil {
				return nil, nil
			}
			return c.coverageFor(marketCode), nil
		},
	}
}

// spatialCoverageExcept says "every market has a verified N04 calendar for
// year, except these". A market without coverage cannot be told apart from
// one whose calendar simply has no rest day, so SeriesForecast reports
// insufficient_data (calendar_unverified) — a third per-market outcome, and
// the one that makes the coverage memo's own key (market_code) observable.
func spatialCoverageExcept(year int, without ...string) func(string) []domain.RestDayCoverage {
	skip := make(map[string]bool, len(without))
	for _, mc := range without {
		skip[mc] = true
	}
	return func(marketCode string) []domain.RestDayCoverage {
		if skip[marketCode] {
			return nil
		}
		return []domain.RestDayCoverage{{MarketCode: marketCode, MarketType: "V", Year: year}}
	}
}

// spatialClosedOn builds a restDaysFor that closes the named markets on
// restDate — and only when the caller's [from, to] window actually contains
// restDate, exactly as a real `WHERE rest_date BETWEEN $2 AND $3` would.
//
// The market half is what makes the usecase-level test discriminating: a
// memo key that lost its market_code answers one market's calendar for
// every other, and the per-market statuses diverge (verified by mutation).
// The window half cannot be exercised from here — every SeriesForecast run
// inside one SpatialSurface.Get derives the same window from the same
// origin date, so one request only ever asks one window per market. It is
// honoured anyway so the fake tells the truth about its own arguments; the
// key's date component is pinned directly in
// TestCalendarMemoRepo_KeysOnEveryArgumentThatChangesTheAnswer instead.
//
// "V" is the MOA calendar's code for the N04 vegetable category the
// fixture's series carry (forecast.CategoryCodeForMarketType).
func spatialClosedOn(restDate forecast.Date, marketCodes ...string) func(string, time.Time, time.Time) []domain.RestDay {
	closed := make(map[string]bool, len(marketCodes))
	for _, mc := range marketCodes {
		closed[mc] = true
	}
	return func(marketCode string, from, to time.Time) []domain.RestDay {
		if !closed[marketCode] {
			return nil
		}
		at := restDate.ToTime()
		if at.Before(from) || at.After(to) {
			return nil
		}
		return []domain.RestDay{{MarketCode: marketCode, MarketType: "V", RestDate: at}}
	}
}

// The group path drives SeriesForecast once per (market, sub-variant), and
// each run issues one crop-DEPENDENT read plus two crop-INDEPENDENT calendar
// reads. Without the per-request memo the calendar reads multiply by the
// sub-variant count for no reason; GroupForecast already avoids exactly this
// by looping markets rather than series.
//
// This test uses the real production wiring (the same *Forecast injected as
// both the forecaster and the group dependency, as di.go's
// provideSpatialSurface does) and, for the "before" number, the same
// *Forecast reading the same repository but injected as a distinct
// forecaster so the memo does not engage.
func TestSpatialSurface_Get_GroupPath_DoesNotRefetchCalendarPerSubVariant(t *testing.T) {
	marketCodes := []string{"104", "220", "338", "400", "512", "540", "648", "800", "900", "930", "950"}
	cropCodes := []string{"A1", "A2", "A3", "A4", "A5"}
	crops := make([]domain.CropRef, 0, len(cropCodes))
	models := make([]forecast.SeriesModel, 0, len(marketCodes)*len(cropCodes))
	for _, crop := range cropCodes {
		crops = append(crops, domain.CropRef{CropCode: crop, Active: true})
		for _, mc := range marketCodes {
			models = append(models, testSeriesModel("moa:"+mc+":"+crop, "moa:"+mc, "moa:"+crop, "N04", 200, 50))
		}
	}
	artifact := testArtifact(models...)
	logger, _ := testLogger()
	registry, scales := testGeography(t), testPriceScales(t)
	in := SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin}

	// Before: forecaster and group dependency are different *Forecast values
	// over the same repository, so nothing is memoised.
	before := &spatialQueryCounter{}
	beforeRepo := before.repo(crops)
	beforeGroup := NewForecast(beforeRepo, testConfig(defaultTestInference()), logger)
	unmemoed := NewSpatialSurface(artifact, registry, scales,
		NewForecast(beforeRepo, testConfig(defaultTestInference()), logger), beforeGroup)
	_, err := unmemoed.Get(context.Background(), in)
	require.NoError(t, err)

	// After: production wiring.
	after := &spatialQueryCounter{}
	afterGroup := NewForecast(after.repo(crops), testConfig(defaultTestInference()), logger)
	memoed := NewSpatialSurface(artifact, registry, scales, afterGroup, afterGroup)
	_, err = memoed.Get(context.Background(), in)
	require.NoError(t, err)

	markets, variants := len(marketCodes), len(cropCodes)
	t.Logf("group path DB reads for %d markets x %d sub-variants: before=%d (obs=%d restDays=%d coverage=%d) after=%d (obs=%d restDays=%d coverage=%d)",
		markets, variants,
		before.seriesObservations+before.restDays+before.restDayCoverage,
		before.seriesObservations, before.restDays, before.restDayCoverage,
		after.seriesObservations+after.restDays+after.restDayCoverage,
		after.seriesObservations, after.restDays, after.restDayCoverage)

	// Before: every read multiplied by the sub-variant count.
	require.Equal(t, markets*variants, before.seriesObservations)
	require.Equal(t, markets*variants, before.restDays)
	require.Equal(t, markets*variants, before.restDayCoverage)

	// After: the crop-dependent read is unchanged (it genuinely differs per
	// sub-variant); the two calendar reads collapse to once per market.
	require.Equal(t, markets*variants, after.seriesObservations)
	require.Equal(t, markets, after.restDays)
	require.Equal(t, markets, after.restDayCoverage)
}

// The memo must not change any answer — only how often it is fetched. Same
// request, both wirings, identical output.
//
// The calendar fake here deliberately answers differently per market, so
// the eleven markets land on three different outcomes: two closed (a rest
// day on the target date), one missing (no calendar coverage at all), eight
// open. Both memoised reads therefore matter — RestDays decides the first
// bucket, RestDayCoverage the second.
//
// Before that, every market's calendar read returned the same empty result,
// which made the assertions below hold no matter what the memo did with its
// key: a key that dropped market_code and served one market's calendar to
// every other would have sailed straight through. The per-market status
// map is the part that discriminates; the whole-output equality is the
// backstop. (The key's date-window component is not reachable from here —
// see spatialClosedOn.)
func TestSpatialSurface_Get_CalendarMemoDoesNotChangeResults(t *testing.T) {
	marketCodes := []string{"104", "220", "338", "400", "512", "540", "648", "800", "900", "930", "950"}
	cropCodes := []string{"A1", "A2"}
	crops := make([]domain.CropRef, 0, len(cropCodes))
	models := make([]forecast.SeriesModel, 0, len(marketCodes)*len(cropCodes))
	for _, crop := range cropCodes {
		crops = append(crops, domain.CropRef{CropCode: crop, Active: true})
		for _, mc := range marketCodes {
			models = append(models, testSeriesModel("moa:"+mc+":"+crop, "moa:"+mc, "moa:"+crop, "N04", 200, 50))
		}
	}
	artifact := testArtifact(models...)
	logger, _ := testLogger()
	registry, scales := testGeography(t), testPriceScales(t)
	in := SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin}

	// spatialTestOrigin is 2026-08-08 and the artifact's calendar timezone
	// is UTC, so horizon 1 targets 2026-08-09.
	originDate := forecast.NewDate(2026, 8, 8)
	closedMarkets := []string{"540", "900"}
	uncoveredMarkets := []string{"950"}
	targetDate := originDate.AddDays(1)

	// Real observations are load-bearing, not scenery: with none, every
	// series' maxAvailableAt is nil, SeriesForecast's freshness step
	// overwrites every point's status with "unavailable", and the closed
	// markets below would be indistinguishable from the open ones — the
	// same tautology in a different disguise.
	counter := &spatialQueryCounter{
		restDaysFor: spatialClosedOn(targetDate, closedMarkets...),
		coverageFor: spatialCoverageExcept(targetDate.Year, uncoveredMarkets...),
		observationsFor: func(marketCode, cropCode string) []domain.Observation {
			return dailyObservations(marketCode, cropCode, strPtr("N04"),
				originDate.AddDays(-40), originDate,
				spatialVegetableMarkets[marketCode], 1000, spatialTestOrigin.Add(-time.Hour))
		},
	}
	repo := counter.repo(crops)
	group := NewForecast(repo, testConfig(defaultTestInference()), logger)

	plain, err := NewSpatialSurface(artifact, registry, scales,
		NewForecast(repo, testConfig(defaultTestInference()), logger), group).Get(context.Background(), in)
	require.NoError(t, err)
	memoed, err := NewSpatialSurface(artifact, registry, scales, group, group).Get(context.Background(), in)
	require.NoError(t, err)

	// The fixture must actually produce two different per-market outcomes,
	// or the comparison below is vacuous again.
	statusOf := func(out SpatialSurfaceOutput) map[string]string {
		byMarket := make(map[string]string, len(out.MarketPrices))
		for _, mp := range out.MarketPrices {
			byMarket[mp.MarketCode] = mp.MarketStatus
		}
		return byMarket
	}
	plainStatus := statusOf(plain)
	require.Len(t, plainStatus, len(marketCodes))
	for _, mc := range closedMarkets {
		require.Equal(t, "closed", plainStatus[mc],
			"market %s must come back closed (rest-day read), otherwise this test cannot detect a broken RestDays memo key", mc)
	}
	for _, mc := range uncoveredMarkets {
		require.Equal(t, "missing", plainStatus[mc],
			"market %s must come back missing (no calendar coverage), otherwise this test cannot detect a broken RestDayCoverage memo key", mc)
	}
	require.Equal(t, len(marketCodes)-len(closedMarkets)-len(uncoveredMarkets), countStatus(plainStatus, "open"),
		"every market that is neither closed nor uncovered must be open — three distinct outcomes, not one")

	// Same request, memo engaged: every market must land on the same status.
	require.Equal(t, plainStatus, statusOf(memoed))

	require.Equal(t, plain.Status, memoed.Status)
	require.Equal(t, plain.QualityReasonCodes, memoed.QualityReasonCodes)
	require.Equal(t, plain.SurfaceCells, memoed.SurfaceCells)
	require.Equal(t, plain.ContourSegments, memoed.ContourSegments)
	require.Equal(t, plain.CountyRollups, memoed.CountyRollups)
	require.Equal(t, plain.MarketPrices, memoed.MarketPrices)
}

func countStatus(byMarket map[string]string, want string) int {
	n := 0
	for _, got := range byMarket {
		if got == want {
			n++
		}
	}
	return n
}

func TestWeightedMedianP50(t *testing.T) {
	p50 := func(v float64) *float64 { return &v }

	t.Run("single market", func(t *testing.T) {
		prices := []SpatialMarketPrice{{MarketCode: "104", P50: p50(42)}}
		median, minimum, maximum := weightedMedianP50(prices, nil)
		require.NotNil(t, median)
		require.InDelta(t, 42.0, *median, 1e-9)
		require.InDelta(t, 42.0, *minimum, 1e-9)
		require.InDelta(t, 42.0, *maximum, 1e-9)
	})

	t.Run("no volumes falls back to equal-weight median, not a mean", func(t *testing.T) {
		prices := []SpatialMarketPrice{
			{MarketCode: "A", P50: p50(10)},
			{MarketCode: "B", P50: p50(20)},
			{MarketCode: "C", P50: p50(30)},
		}
		median, minimum, maximum := weightedMedianP50(prices, nil)
		require.InDelta(t, 20.0, *median, 1e-9, "odd count, equal weights: the true middle value")
		require.InDelta(t, 10.0, *minimum, 1e-9)
		require.InDelta(t, 30.0, *maximum, 1e-9)
	})

	t.Run("dominant volume pulls the median toward it", func(t *testing.T) {
		prices := []SpatialMarketPrice{
			{MarketCode: "A", P50: p50(10)},
			{MarketCode: "B", P50: p50(20)},
			{MarketCode: "C", P50: p50(30)},
		}
		volumes := map[string]float64{"A": 1, "B": 1, "C": 100}
		median, minimum, maximum := weightedMedianP50(prices, volumes)
		require.InDelta(t, 30.0, *median, 1e-9, "C's overwhelming volume must dominate the weighted median")
		require.InDelta(t, 10.0, *minimum, 1e-9, "min/max never depend on weight")
		require.InDelta(t, 30.0, *maximum, 1e-9)
	})

	t.Run("market missing from volumes falls back to weight 1, not dropped", func(t *testing.T) {
		prices := []SpatialMarketPrice{
			{MarketCode: "A", P50: p50(10)}, // volume 5
			{MarketCode: "B", P50: p50(20)}, // no entry -> weight 1
			{MarketCode: "C", P50: p50(30)}, // volume 5
		}
		volumes := map[string]float64{"A": 5, "C": 5}
		median, minimum, maximum := weightedMedianP50(prices, volumes)
		// total weight 11, target 5.5; cumulative 10(w5,cum5) then 20(w1,cum6>=5.5) -> median 20.
		require.InDelta(t, 20.0, *median, 1e-9)
		require.InDelta(t, 10.0, *minimum, 1e-9)
		require.InDelta(t, 30.0, *maximum, 1e-9)
	})

	t.Run("zero or negative volume falls back to weight 1, never to zero", func(t *testing.T) {
		prices := []SpatialMarketPrice{
			{MarketCode: "A", P50: p50(10)},
			{MarketCode: "B", P50: p50(20)},
		}
		volumes := map[string]float64{"A": 0, "B": -5}
		median, _, _ := weightedMedianP50(prices, volumes)
		// Both fall back to weight 1: total 2, target 1; cumulative 10(cum1>=1) -> median 10.
		require.InDelta(t, 10.0, *median, 1e-9)
	})

	t.Run("markets without a usable P50 are excluded entirely", func(t *testing.T) {
		prices := []SpatialMarketPrice{
			{MarketCode: "A", P50: nil},
			{MarketCode: "B", P50: p50(20)},
		}
		median, minimum, maximum := weightedMedianP50(prices, nil)
		require.InDelta(t, 20.0, *median, 1e-9)
		require.InDelta(t, 20.0, *minimum, 1e-9)
		require.InDelta(t, 20.0, *maximum, 1e-9)
	})

	t.Run("no market has a P50 returns a nil triple", func(t *testing.T) {
		prices := []SpatialMarketPrice{{MarketCode: "A", P50: nil}}
		median, minimum, maximum := weightedMedianP50(prices, nil)
		require.Nil(t, median)
		require.Nil(t, minimum)
		require.Nil(t, maximum)
	})
}

// TestSpatialSurface_Get_CropPath_StatisticsWeightedByCropVolume covers the
// crop path's new ComputeCropVolumes wiring — before this, RecentVolumeForCrop
// did not exist and the crop path had no market-level volume at all, so
// statistics.p50/minimum_p50/maximum_p50 could only ever be equal-weighted.
// Market 950 (price 33.0, spatialVegetableMarkets' minimum) gets an
// overwhelming volume; the plain unweighted median across all eleven markets
// is 66.0 (market 648), so a weighted median landing on 33.0 proves the new
// per-market volume actually reached weightedMedianP50, not just that some
// median got computed.
func TestSpatialSurface_Get_CropPath_StatisticsWeightedByCropVolume(t *testing.T) {
	artifact, forecaster := spatialCropFixture(t, "A1")
	logger, _ := testLogger()
	repo := &fakeForecastRepo{
		recentVolumeForCropFn: func(ctx context.Context, cropCode string, from, to, cutoff time.Time) ([]domain.MarketVolume, error) {
			require.Equal(t, "A1", cropCode)
			return []domain.MarketVolume{{MarketCode: "950", Volume: 1000}}, nil
		},
	}
	groupUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, "ready", out.Status)
	require.NotNil(t, out.Statistics.P50)
	require.InDelta(t, 33.0, *out.Statistics.P50, 1e-9, "market 950's overwhelming volume must dominate the crop-path median")
	require.NotNil(t, out.Statistics.MinimumP50)
	require.InDelta(t, 33.0, *out.Statistics.MinimumP50, 1e-9)
	require.NotNil(t, out.Statistics.MaximumP50)
	require.InDelta(t, 145.0, *out.Statistics.MaximumP50, 1e-9)
}

// TestSpatialSurface_Get_CropPath_NoGroupDependency_StatisticsFallBackToEqualWeight
// covers the pre-existing "group is nil" crop-path shape (every other
// crop-path test in this file passes nil): marketVolumesForStatistics must
// not error or panic just because there is no Forecast usecase to query
// crop volume from — it degrades to the plain equal-weighted median.
func TestSpatialSurface_Get_CropPath_NoGroupDependency_StatisticsFallBackToEqualWeight(t *testing.T) {
	artifact, forecaster := spatialCropFixture(t, "A1")
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), forecaster, nil)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.NotNil(t, out.Statistics.P50)
	require.InDelta(t, 66.0, *out.Statistics.P50, 1e-9, "eleven equally-weighted markets: true median is market 648's 66.0")
}

// TestSpatialSurface_Get_GroupPath_StatisticsReuseSameVolumeQuery pins down
// the design property marketVolumesForStatistics's doc comment claims: the
// group path must reuse variantWeights' own already-fetched
// GroupVolumes.ByMarket rather than calling RecentVolume a second time.
func TestSpatialSurface_Get_GroupPath_StatisticsReuseSameVolumeQuery(t *testing.T) {
	series := map[string]forecast.SeriesModel{}
	byKey := map[string]SeriesForecastOutput{}
	for mc, price := range spatialVegetableMarkets {
		series["moa:"+mc+":A1"] = forecast.SeriesModel{
			SeriesID: "moa:" + mc + ":A1", MarketID: "moa:" + mc,
			CropID: "moa:A1", CategoryCode: strPtr("N04"),
		}
		byKey[mc+":A1"] = spatialReadyOutput(mc, "A1", price)
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: spatialTestModelVersion, DatasetDigest: spatialTestDatasetDigest,
		Series: series, Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}

	callCount := 0
	logger, _ := testLogger()
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "A1", Active: true}}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			callCount++
			return []domain.CropMarketVolume{{MarketCode: "950", CropCode: "A1", Volume: 1000}}, nil
		},
	}
	groupUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	uc := NewSpatialSurface(artifact, testGeography(t), testPriceScales(t), &fakeSeriesForecaster{byKey: byKey}, groupUC)

	out, err := uc.Get(context.Background(), SpatialSurfaceInput{GroupCode: strPtr("G-N04"), HorizonDays: 1, Now: spatialTestOrigin})
	require.NoError(t, err)
	require.Equal(t, 1, callCount, "group path must reuse variantWeights' GroupVolumes.ByMarket, not re-query RecentVolume")
	require.NotNil(t, out.Statistics.P50)
	require.InDelta(t, 33.0, *out.Statistics.P50, 1e-9, "market 950's overwhelming volume must dominate the group-path median")
}

// BenchmarkSpatialSurface_Get_CropWithMarkets inlines spatialCropFixture's
// two lines instead of calling it — that helper takes a *testing.T only to
// call t.Helper() (never used for assertions), and passing a benchmark a
// stand-in *testing.T for that is a needless fragility for a measurement
// task; b.Fatal below is this function's real, correctly-typed failure path.
func BenchmarkSpatialSurface_Get_CropWithMarkets(b *testing.B) {
	codes := make([]string, 0, len(spatialVegetableMarkets))
	byKey := make(map[string]SeriesForecastOutput, len(spatialVegetableMarkets))
	for mc, price := range spatialVegetableMarkets {
		codes = append(codes, mc)
		byKey[mc+":A1"] = spatialReadyOutput(mc, "A1", price)
	}
	artifact := spatialTestArtifact("A1", "N04", codes...)
	forecaster := &fakeSeriesForecaster{byKey: byKey}

	geo, err := geography.Load(
		"../../reference/market_registry.v1.csv", "../../reference/market_registry.v1.manifest.json",
		"../../reference/taiwan_boundary.taiwan-atlas-2021.9.20.geojson", "../../reference/taiwan_boundary.v1.manifest.json",
	)
	if err != nil {
		b.Fatal(err)
	}
	scales, err := spatial.LoadPriceScales("../../reference/price_scale.v1.csv", "../../reference/price_scale.v1.manifest.json")
	if err != nil {
		b.Fatal(err)
	}
	uc := NewSpatialSurface(artifact, geo, scales, forecaster, nil)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := uc.Get(ctx, SpatialSurfaceInput{CropCode: strPtr("A1"), HorizonDays: 1, Now: spatialTestOrigin}); err != nil {
			b.Fatal(err)
		}
	}
}
