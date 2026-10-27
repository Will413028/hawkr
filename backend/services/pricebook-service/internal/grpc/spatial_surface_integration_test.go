package grpc

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// Everything else in handler_test.go feeds GetSpatialSurface a hand-built
// SpatialSurfaceOutput: a handful of counties, one or two market rows, a
// couple of cells. That pins the mapping rules but proves nothing about
// the shape production actually produces — ~1000 surface cells, several
// hundred contour segments, and 22 counties whose geometry is real
// orb.MultiPolygon read off reference/taiwan_boundary.*.geojson, some of
// them with dozens of rings. structpb.NewStruct rejects values it cannot
// represent and orb geometry has more concrete types than the four this
// handler knows about, so "the synthetic DTO converts" and "the real
// surface converts" are genuinely different claims.
//
// This test makes the second one. Only the per-series inference is faked
// (a real one would need a DB and a 154MB model.json); the geography
// registry, the price scales, the interpolation, the banding, the contour
// tracing, the county rollup and the whole handler conversion are real.
//
// It also reports the serialized response size, which is otherwise an
// unmeasured quantity for the largest response this service can return.

// realForecasterFake stands in for *usecase.Forecast. Its method set is
// exported, so it satisfies the usecase package's unexported
// seriesForecaster interface from out here.
type realForecasterFake struct {
	byKey map[string]usecase.SeriesForecastOutput
}

func (f *realForecasterFake) SeriesForecast(ctx context.Context, in usecase.SeriesForecastInput) (*usecase.SeriesForecastOutput, error) {
	out, ok := f.byKey[in.MarketCode+":"+in.CropCode]
	if !ok {
		return &usecase.SeriesForecastOutput{MarketCode: in.MarketCode, CropCode: in.CropCode}, nil
	}
	return &out, nil
}

// spatialIntegrationMarkets is a real, well-distributed subset of the N04
// (vegetable) facilities in reference/market_registry.v1.csv, with
// hand-picked prices spanning a wide enough range (33 to 145) to exercise
// multiple adaptive breaks (issue #3102's DeriveAdaptiveScale) — the same
// fixture internal/usecase/spatial_surface_test.go's ready-path test uses,
// so the two layers are asserted over the same surface.
var spatialIntegrationMarkets = map[string]float64{
	"104": 42.0, "220": 38.0, "338": 55.0, "400": 61.0,
	"512": 74.0, "540": 88.0, "648": 66.0, "800": 120.0,
	"900": 145.0, "930": 96.0, "950": 33.0,
}

func realSpatialSurfaceUC(t *testing.T, cropCode string) *usecase.SpatialSurface {
	t.Helper()

	registry, err := geography.Load(
		"../../reference/market_registry.v1.csv", "../../reference/market_registry.v1.manifest.json",
		"../../reference/taiwan_boundary.taiwan-atlas-2021.9.20.geojson", "../../reference/taiwan_boundary.v1.manifest.json",
	)
	if err != nil {
		t.Fatalf("load geography registry: %v", err)
	}
	scales, err := spatial.LoadPriceScales("../../reference/price_scale.v1.csv", "../../reference/price_scale.v1.manifest.json")
	if err != nil {
		t.Fatalf("load price scales: %v", err)
	}

	categoryCode := "N04"
	series := make(map[string]forecast.SeriesModel, len(spatialIntegrationMarkets))
	byKey := make(map[string]usecase.SeriesForecastOutput, len(spatialIntegrationMarkets))
	for marketCode, price := range spatialIntegrationMarkets {
		seriesID := "moa:" + marketCode + ":" + cropCode
		series[seriesID] = forecast.SeriesModel{
			SeriesID: seriesID, MarketID: "moa:" + marketCode, CropID: "moa:" + cropCode,
			CategoryCode: &categoryCode,
		}
		byKey[marketCode+":"+cropCode] = spatialIntegrationForecast(marketCode, cropCode, price)
	}
	artifact := forecast.ModelArtifact{
		ModelVersion:  "direct-ridge-v2-integrationfixture",
		DatasetDigest: "02b9fd8cef27a5bff8a4be4ebf846f8e7d4e7f12617cda2ba6556ad7c740fffe",
		Series:        series,
		Metadata:      map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
	// groupUC is nil: this is the crop path, which never reaches Forecast
	// (see NewSpatialSurface's parameter doc).
	return usecase.NewSpatialSurface(artifact, registry, scales, &realForecasterFake{byKey: byKey}, nil)
}

// spatialIntegrationForecast is one market's faked 7-day forecast, every
// point ready. The horizon offsets are relative to the request's own origin
// date, which the handler takes from time.Now() and does not expose — the
// usecase only matches points by HorizonDays, so absolute dates here need
// only be self-consistent.
func spatialIntegrationForecast(marketCode, cropCode string, price float64) usecase.SeriesForecastOutput {
	origin := forecast.DateFromTime(time.Now())
	points := make([]usecase.ForecastPoint, 0, 7)
	for h := 1; h <= 7; h++ {
		p10, p50, p90 := price*0.9, price, price*1.1
		points = append(points, usecase.ForecastPoint{
			HorizonDays: h, TargetDate: origin.AddDays(h),
			Status: forecast.PredictionStatusReady, ModelKind: "direct_ridge",
			P10: &p10, P50: &p50, P90: &p90,
		})
	}
	through := origin.AddDays(-1)
	availableAt := time.Now().Add(-8 * time.Hour)
	return usecase.SeriesForecastOutput{
		SeriesID: "moa:" + marketCode + ":" + cropCode, MarketCode: marketCode, CropCode: cropCode,
		Forecast: points, Freshness: forecast.FreshnessFresh, DataThroughDate: &through,
		MaxFeatureAvailableAt: &availableAt,
	}
}

func TestGetSpatialSurface_RealUsecase_ProducesAWellFormedResponse(t *testing.T) {
	uc := realSpatialSurfaceUC(t, "A1")
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{},
		&fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	resp, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 1})
	if err != nil {
		t.Fatalf("GetSpatialSurface over the real usecase: %v", err)
	}

	if resp.GetStatus() != "ready" {
		t.Fatalf("Status = %q, want ready (this fixture prices 11 well-distributed markets)", resp.GetStatus())
	}
	if resp.GetSelectionKind() != "crop" || resp.GetHorizonDays() != 1 {
		t.Errorf("SelectionKind/HorizonDays = %q/%d, want crop/1", resp.GetSelectionKind(), resp.GetHorizonDays())
	}
	if resp.GetCurrency() != "TWD" || resp.GetUnit() != "kg" {
		t.Errorf("Currency/Unit = %q/%q, want TWD/kg", resp.GetCurrency(), resp.GetUnit())
	}
	// issue #3102: the response's scale is no longer the reviewed
	// N04-price-scale-v1 policy's fixed breaks verbatim — it is adaptively
	// derived per-request from this same call's own P50 domain. The
	// ScaleID still traces back to the reviewed base policy (see
	// internal/usecase/spatial_surface_test.go's matching assertions).
	if resp.Scale == nil || !strings.HasPrefix(resp.Scale.GetScaleId(), "N04-price-scale-v1-adaptive-") || resp.Scale.GetPolicyVersion() != spatial.AdaptivePolicyV1 {
		t.Errorf("Scale = %+v, want an adaptive scale derived from N04-price-scale-v1", resp.Scale)
	}

	// The four GeoJSON layers must all survive structpb conversion as real,
	// non-empty FeatureCollections. A nil Struct or an empty feature list
	// here is the silent-drop failure this test exists to catch.
	for _, layer := range []struct {
		name    string
		value   *structpb.Struct
		minimum int
	}{
		{"BoundaryPriceGeojson", resp.BoundaryPriceGeojson, 22},
		{"MarketPointsGeojson", resp.MarketPointsGeojson, len(spatialIntegrationMarkets)},
		{"SurfaceCellsGeojson", resp.SurfaceCellsGeojson, 100},
		{"ContourLinesGeojson", resp.ContourLinesGeojson, 1},
	} {
		if layer.value == nil {
			t.Fatalf("%s = nil, want a FeatureCollection", layer.name)
		}
		asMap := layer.value.AsMap()
		if asMap["type"] != "FeatureCollection" {
			t.Fatalf("%s.type = %v, want FeatureCollection", layer.name, asMap["type"])
		}
		features, ok := asMap["features"].([]any)
		if !ok {
			t.Fatalf("%s.features is %T, want a list", layer.name, asMap["features"])
		}
		if len(features) < layer.minimum {
			t.Fatalf("%s has %d features, want at least %d", layer.name, len(features), layer.minimum)
		}
		// Real geometry, really serializable: every feature must round-trip
		// to JSON with finite coordinates. structpb rejects NaN/Inf at
		// construction, so reaching this point already proves a lot; the
		// marshal proves the client sees valid JSON rather than a Struct
		// that merely constructed.
		blob, err := json.Marshal(asMap)
		if err != nil {
			t.Fatalf("marshal %s: %v", layer.name, err)
		}
		if len(blob) == 0 {
			t.Fatalf("%s marshalled to nothing", layer.name)
		}
		first, ok := features[0].(map[string]any)
		if !ok {
			t.Fatalf("%s feature 0 is %T, want an object", layer.name, features[0])
		}
		if first["type"] != "Feature" {
			t.Errorf("%s feature 0 type = %v, want Feature", layer.name, first["type"])
		}
		geometry, ok := first["geometry"].(map[string]any)
		if !ok || geometry["type"] == nil || geometry["coordinates"] == nil {
			t.Errorf("%s feature 0 geometry = %v, want a typed geometry with coordinates", layer.name, first["geometry"])
		}
	}

	// Market prices survived with real numbers attached.
	if len(resp.MarketPrices) != len(spatialIntegrationMarkets) {
		t.Fatalf("MarketPrices = %d rows, want %d", len(resp.MarketPrices), len(spatialIntegrationMarkets))
	}
	priced := 0
	for _, mp := range resp.MarketPrices {
		want, known := spatialIntegrationMarkets[mp.GetMarketCode()]
		if !known {
			t.Errorf("MarketPrices carries unknown market %q", mp.GetMarketCode())
			continue
		}
		if mp.P50 == nil {
			continue
		}
		priced++
		if math.Abs(mp.GetP50()-want) > 1e-9 {
			t.Errorf("market %s P50 = %v, want %v", mp.GetMarketCode(), mp.GetP50(), want)
		}
		if mp.GetPredictionStatus() != "ready" || mp.GetMarketStatus() != "open" || !mp.GetUsedForSurface() {
			t.Errorf("market %s = %+v, want ready/open/used", mp.GetMarketCode(), mp)
		}
		// The registry-resolved facility name must have reached the wire —
		// SpatialMarketPrice falls back to the market code when geography
		// resolution fails, so "name == code" means the lookup silently
		// missed.
		if mp.GetMarketName() == "" || mp.GetMarketName() == mp.GetMarketCode() {
			t.Errorf("market %s lost its registry identity: name=%q",
				mp.GetMarketCode(), mp.GetMarketName())
		}
		if mp.GetCategoryCode() != "N04" {
			t.Errorf("market %s categoryCode = %q, want N04", mp.GetMarketCode(), mp.GetCategoryCode())
		}
	}
	if priced != len(spatialIntegrationMarkets) {
		t.Fatalf("priced markets = %d, want all %d", priced, len(spatialIntegrationMarkets))
	}

	// Statistics must describe the payload actually sent, not the DTO.
	cells := len(resp.SurfaceCellsGeojson.AsMap()["features"].([]any))
	contours := len(resp.ContourLinesGeojson.AsMap()["features"].([]any))
	if int(resp.Statistics.GetSurfaceCellCount()) != cells {
		t.Errorf("statistics.surfaceCellCount = %d, but %d cell features were sent",
			resp.Statistics.GetSurfaceCellCount(), cells)
	}
	if int(resp.Statistics.GetContourSegmentCount()) != contours {
		t.Errorf("statistics.contourSegmentCount = %d, but %d contour features were sent",
			resp.Statistics.GetContourSegmentCount(), contours)
	}

	if resp.Lineage == nil || resp.Lineage.GetModelVersion() == "" || resp.Lineage.GetBoundaryDigest() == "" {
		t.Errorf("Lineage = %+v, want model version and boundary digest", resp.Lineage)
	}

	// The one number nothing else in this repo measures: how big the
	// largest response this service can produce actually is on the wire,
	// and which layer is responsible for it.
	size := proto.Size(resp)
	counties := len(resp.BoundaryPriceGeojson.AsMap()["features"].([]any))
	t.Logf("serialized GetSpatialSurfaceResponse: %d bytes (%.2f MiB) for %d surface cells, %d contour segments, %d counties, %d markets",
		size, float64(size)/(1024*1024), cells, contours, counties, len(resp.MarketPrices))
	// boundaryPriceGeojson (the per-county price rollup) and boundary (the
	// raw county geometry, resp.Boundary) are two distinct response fields
	// that happen to share a name fragment — labeled separately here so
	// this breakdown doesn't quietly conflate them.
	t.Logf("  per layer: boundaryPriceGeojson=%d boundary=%d surfaceCells=%d contours=%d marketPoints=%d bytes",
		proto.Size(resp.BoundaryPriceGeojson), proto.Size(resp.Boundary), proto.Size(resp.SurfaceCellsGeojson),
		proto.Size(resp.ContourLinesGeojson), proto.Size(resp.MarketPointsGeojson))
	if size <= 0 {
		t.Fatal("proto.Size = 0, want a non-empty response")
	}
	// gRPC's default max receive size is 4MiB. Crossing it does not fail
	// this test — it is not this test's call to make — but it must not
	// happen silently, because the client-side symptom is an opaque
	// ResourceExhausted with no hint that a payload grew.
	if size > 4*1024*1024 {
		t.Errorf("serialized response is %d bytes, over gRPC's 4MiB default receive limit — "+
			"clients need an explicit MaxCallRecvMsgSize, or this payload needs trimming", size)
	}
}
