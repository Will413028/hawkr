// services/pricebook-service/internal/usecase/spatial_surface.go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/paulmach/orb"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
)

type seriesForecaster interface {
	SeriesForecast(ctx context.Context, in SeriesForecastInput) (*SeriesForecastOutput, error)
}

// ErrSpatialExactlyOneSelection / ErrSpatialHorizonOutOfRange are the two
// request-shape violations of GetSpatialSurface's caller contract (issue
// #2248: zero-or-both selections and an out-of-range horizon are
// InvalidArgument; every *business* "no surface" outcome is a Status +
// QualityReasonCodes, never an error — see Get's doc comment). Exported
// for the same reason ErrConflictingResolutionInputs is: internal/grpc's
// handler maps them to codes.InvalidArgument with errors.Is, and it lives
// in a different package.
var (
	ErrSpatialExactlyOneSelection = errors.New("usecase: exactly one of crop_code or group_code must be set")
	ErrSpatialHorizonOutOfRange   = errors.New("usecase: horizon_days must be between 1 and 7")
	// ErrSpatialZeroNow mirrors SeriesForecast/GroupForecast's own zero-Origin
	// guard: a zero time.Time becomes SQL NULL through pgxutil, every as-of
	// query then returns 0 rows with err=nil, and the caller gets a
	// plausible-looking empty surface instead of a complaint.
	ErrSpatialZeroNow = errors.New("usecase: now must not be zero (a zero time.Time silently becomes SQL NULL and every as-of query returns 0 rows with err=nil)")
)

// Spatial engine parameters. These mirror Transformer's api/catalog.py
// live-API SpatialConfig exactly (design doc Components) — deliberately
// compile-time constants, not env-tunable config: issue #2248 requires the
// same selection/date/horizon to reproduce the same surface, and a
// deployment-time knob on grid resolution or IDW support distance would
// silently break that guarantee between two pods of the same release.
const (
	spatialMinPoints              = 4
	spatialGridResolutionM        = 7500.0
	spatialIDWPower               = 2.0
	spatialMaxSupportDistanceM    = 140_000.0
	spatialMinSupportingNeighbors = 3
	spatialMinSupportedLandRatio  = 0.12
	spatialMaxGridCellsPerRegion  = 12000
)

// Business-outcome vocabulary. Every code below except
// spatialReasonPriceScaleUnavailable is lifted verbatim from Transformer's
// own spatial engine (interpolation.py's build_spatial_surface/_region_grid
// and reporting/daily.py) so the Go port and the Python producer describe
// the same situations with the same words. The one addition exists because
// this port replaces Transformer's per-request _nice_contour_breaks with a
// versioned fixed scale (issue #2248's stability rule), which introduces a
// failure mode Python does not have: a selection with no approved scale.
const (
	spatialReasonNoScoreableTradeRows       = "no_scoreable_trade_rows"
	spatialReasonAllScopedMarketsClosed     = "all_scoped_markets_closed"
	spatialReasonMissingMarketGeocodes      = "missing_market_geocodes"
	spatialReasonOutsideBoundaryDomain      = "outside_boundary_domain"
	spatialReasonBoundaryUnavailable        = "boundary_unavailable"
	spatialReasonUnsupportedBoundaryRegions = "unsupported_boundary_regions"
	spatialReasonInsufficientSpatialPoints  = "insufficient_spatial_points"
	spatialReasonCollinearSpatialPoints     = "collinear_spatial_points"
	spatialReasonUnsupportedLandArea        = "unsupported_land_area"
	spatialReasonFlatPriceField             = "flat_price_field"
	spatialReasonInsufficientSpatialCover   = "insufficient_spatial_coverage"
	spatialReasonPriceScaleUnavailable      = "price_scale_unavailable"
)

// Per-market exclusion reasons (interpolation.py's _point_features).
const (
	spatialExclusionMissingCoordinate = "missing_coordinate"
	spatialExclusionOutsideDomain     = "outside_or_unsupported_domain"
	// spatialExclusionNoUsablePrice has no Python counterpart: Transformer
	// excludes *observed trades*, which always carry a price, whereas this
	// pipeline excludes *forecast* markets, which can be geocoded and in
	// scope and still have nothing to plot (closed market, insufficient
	// data). Reusing outside_or_unsupported_domain for those would tell the
	// seller their market is off the map when it is merely shut today.
	spatialExclusionNoUsablePrice = "no_usable_price"
)

// Statuses and display modes (interpolation.py's SpatialSurface.status /
// .display_mode). "closed"/"unavailable" display modes from Transformer's
// *report* layer are deliberately not reproduced — this RPC has a separate
// top-level status field carrying that information.
const (
	spatialStatusReady       = "ready"
	spatialStatusDegraded    = "degraded"
	spatialStatusUnavailable = "unavailable"

	spatialDisplayContoursAndPoints = "contours_and_points"
	spatialDisplaySurfaceAndPoints  = "surface_and_points"
	spatialDisplayPointsOnly        = "points_only"
	spatialDisplayTableOnly         = "table_only"
)

// Disclosure codes, verbatim from Transformer reporting/daily.py's
// DailyPriceReportDraft.ui — the first two are unconditional, the third
// only when an interpolated surface is actually drawn.
const (
	spatialDisclosureMarketLocation = "wholesale_market_location_not_production_origin"
	spatialDisclosureNotAdvice      = "not_quote_or_procurement_price_advice"
	spatialDisclosureInterpolated   = "interpolated_surface_not_observed_between_markets"
)

// knownCategoryCodes is the deterministic probe order used when a series
// carries no category_code (SeriesModel.CategoryCode is a pointer and the
// upstream artifact may leave it null): the geography registry is keyed by
// (market_code, category_code), so some category must be chosen to resolve
// a facility at all. Order matches forecast.marketTypeToCategoryCode's
// V/F/L declaration order.
var knownCategoryCodes = []string{"N04", "N05", "N06"}

// marketResolver is the shared "resolve which markets belong to this
// crop/group selection, forecast each at the requested horizon, and
// resolve each to its geography facility/county" pipeline. SpatialSurface
// (issue #2248) and RegionalComparison (issue #2249 Slice C) both need
// exactly this and must not diverge — issue #2249: "兩者應讀同一 spatial
// snapshot 與 aggregation 結果，不能各算一套". It deliberately does not
// know about IDW/grid/contour building (spatial.BuildRegionGrid, IDWAt,
// spatial.BuildSurfaceCells, spatial.BuildContours) — only SpatialSurface
// needs those, and RegionalComparison must not pay for them (design doc
// D3).
type marketResolver struct {
	artifact  forecast.ModelArtifact
	geography geography.Registry
	forecast  seriesForecaster
	group     *Forecast // nil is valid for crop-path-only tests, same as SpatialSurface today
}

// marketSelectionInput is the shared subset of SpatialSurfaceInput /
// RegionalComparisonInput that resolveAndForecastMarkets needs — both
// usecases build one from their own request type before calling in.
type marketSelectionInput struct {
	CropCode    *string
	GroupCode   *string
	HorizonDays int
	Now         time.Time
}

// resolvedMarkets is resolveAndForecastMarkets' output — the shared
// midpoint both usecases build their own response shape from.
type resolvedMarkets struct {
	SelectionKind string // "crop" | "group"
	SelectionCode string
	CropCodes     []string
	MarketCodes   []string
	// Markets/MarketVolumes/Lineage stay zero-valued when MarketCodes is
	// empty (selection resolved to nothing modelled) — callers detect
	// this via len(MarketCodes) == 0, mirroring SpatialSurface.Get's
	// existing early-exit rather than probing Markets itself.
	Markets       []spatialMarket
	MarketVolumes map[string]float64
	Lineage       SpatialLineage
}

// resolveAndForecastMarkets is SpatialSurface.Get's former inline
// selection-resolution block (moved verbatim, then given a second
// caller). Only genuine infrastructure failures return an error; an
// unmodelled crop/group is reported via an empty MarketCodes, not an
// error (issue #2248/#2249: "syntactically valid but unmodeled is 200 +
// status, never an error" — each caller decides what that means for its
// own output shape).
func (r *marketResolver) resolveAndForecastMarkets(ctx context.Context, in marketSelectionInput) (resolvedMarkets, error) {
	originDate, cutoff, err := forecast.OriginDateAndCutoff(in.Now, calendarTimezoneOf(r.artifact))
	if err != nil {
		return resolvedMarkets{}, fmt.Errorf("usecase: %w", err)
	}

	var out resolvedMarkets
	out.SelectionKind = "crop"
	if in.CropCode != nil {
		out.SelectionCode = *in.CropCode
		out.CropCodes = []string{out.SelectionCode}
		out.MarketCodes = r.marketsForCrop(out.SelectionCode)
	} else {
		out.SelectionKind, out.SelectionCode = "group", *in.GroupCode
		cropCodes, err := r.cropsInGroup(ctx, out.SelectionCode)
		if err != nil {
			return resolvedMarkets{}, err
		}
		out.CropCodes = cropCodes
		out.MarketCodes = r.marketsForCrops(cropCodes)
	}
	if len(out.MarketCodes) == 0 {
		return out, nil
	}

	weights, err := r.variantWeights(ctx, out.SelectionKind, out.SelectionCode, originDate, cutoff)
	if err != nil {
		return resolvedMarkets{}, err
	}
	out.MarketVolumes, err = r.marketVolumesForStatistics(ctx, out.SelectionKind, out.SelectionCode, weights, originDate, cutoff)
	if err != nil {
		return resolvedMarkets{}, err
	}
	out.Markets, out.Lineage, err = r.aggregateMarkets(ctx, out.MarketCodes, out.CropCodes, weights, in)
	if err != nil {
		return resolvedMarkets{}, err
	}
	return out, nil
}

// marketsForCrop mirrors forecast_curve.go's marketComparison pattern
// (in-memory filter of r.artifact.Series, no DB query). Sorted for the
// same reason marketComparison sorts: Go's map iteration order over
// r.artifact.Series is randomized per process, and every downstream step
// here (which markets get forecast, which locations feed IDW, the order
// of marketPrices) must be reproducible for the same request.
func (r *marketResolver) marketsForCrop(cropCode string) []string {
	cropID := "moa:" + cropCode
	seen := map[string]bool{}
	var codes []string
	for _, series := range r.artifact.Series {
		if series.CropID == cropID {
			mc := strings.TrimPrefix(series.MarketID, "moa:")
			if !seen[mc] {
				seen[mc] = true
				codes = append(codes, mc)
			}
		}
	}
	sort.Strings(codes)
	return codes
}

// marketsForCrops is the group-path union of marketsForCrop, sorted and
// deduped for the same determinism reason.
func (r *marketResolver) marketsForCrops(cropCodes []string) []string {
	seen := map[string]bool{}
	var codes []string
	for _, cropCode := range cropCodes {
		for _, mc := range r.marketsForCrop(cropCode) {
			if !seen[mc] {
				seen[mc] = true
				codes = append(codes, mc)
			}
		}
	}
	sort.Strings(codes)
	return codes
}

// cropsInGroup enumerates the group's active sub-variants via the same
// repository call GroupForecast uses (forecast.go's CropsInGroup +
// Active filter). It deliberately diverges from GroupForecast on the empty
// cases: GroupForecast errors on "group has no crops"/"no active crops",
// whereas issue #2248 requires a syntactically valid but unmodelled
// selection to be a 200 + status, so those come back as an empty crop list
// which Get turns into unavailable + no_scoreable_trade_rows.
func (r *marketResolver) cropsInGroup(ctx context.Context, groupCode string) ([]string, error) {
	if r.group == nil {
		return nil, fmt.Errorf("usecase: spatial surface group path requires the Forecast usecase (group dependency is nil)")
	}
	crops, err := r.group.repo.CropsInGroup(ctx, groupCode)
	if err != nil {
		return nil, fmt.Errorf("usecase: CropsInGroup(%s): %w", groupCode, err)
	}
	codes := make([]string, 0, len(crops))
	for _, c := range crops {
		if c.Active {
			codes = append(codes, c.CropCode)
		}
	}
	sort.Strings(codes)
	return codes, nil
}

// variantWeights fetches the group path's volume weights. It reuses
// Forecast.ComputeGroupVolumes — the single implementation GroupForecast,
// GroupObservedSeries and TrendSummary already share (forecast.go's
// groupVolumes doc comment: "Must stay the single implementation of this
// conversion; do not inline a second copy") — so this RPC cannot end up
// with a different definition of "this group's price at market X" than the
// pages next to it. The crop path has one series per market, where any
// weighting is arithmetically a no-op, so it does not pay for the query.
func (r *marketResolver) variantWeights(ctx context.Context, selectionKind, selectionCode string, originDate forecast.Date, cutoff time.Time) (forecast.GroupVolumes, error) {
	if selectionKind != "group" || r.group == nil {
		return forecast.GroupVolumes{}, nil
	}
	vols, err := r.group.ComputeGroupVolumes(ctx, selectionCode, originDate, cutoff)
	if err != nil {
		return forecast.GroupVolumes{}, err
	}
	return vols, nil
}

// resolveFacility looks the market up in the geography registry. When the
// series carried no category_code there is nothing to key the registry
// with, so the known categories are probed in a fixed order — a market
// code alone is ambiguous in the real registry (400 and 800 each have both
// a produce and a flower facility, at different coordinates).
func (r *marketResolver) resolveFacility(marketCode, categoryCode string, now time.Time) (geography.Facility, bool) {
	on := forecast.DateFromTime(now)
	if categoryCode != "" {
		return r.geography.ResolveFacility(marketCode, categoryCode, on)
	}
	for _, code := range knownCategoryCodes {
		if f, ok := r.geography.ResolveFacility(marketCode, code, on); ok {
			return f, true
		}
	}
	return geography.Facility{}, false
}

// perRequestForecaster returns the forecaster aggregateMarkets should use.
//
// In production the same *Forecast is injected as both the forecaster and
// the group dependency (di.go's provideSpatialSurface). On the group path
// that means SeriesForecast runs once per (market, sub-variant), and each
// run re-issues the market's rest-day calendar and coverage queries —
// which do not depend on the crop at all. GroupForecast solved the same
// problem by querying the calendar once per market rather than once per
// series (forecast.go's marketCodes loop); this wraps the one repository
// in a per-request memo to reach the same query count without duplicating
// SeriesForecast's calendar/quality-gate logic here, which
// estimated_item_price.go's doc comment explicitly forbids.
//
// The memo is per request and covers only crop-independent, argument-keyed
// reads, so it cannot change any result — only how many times the same
// answer is fetched. When the forecaster is not the group dependency
// (every test injecting a fake), nothing is wrapped.
func (r *marketResolver) perRequestForecaster() seriesForecaster {
	if r.group == nil || r.forecast != seriesForecaster(r.group) {
		return r.forecast
	}
	return NewForecast(newCalendarMemoRepo(r.group.repo), r.group.cfg, r.group.logger)
}

// marketVolumesForStatistics resolves each contributing market's total
// recent-volume weight for weightedMedianP50 — a different axis from
// variantWeights, which blends *within* one market across a group's
// sub-variants (see that function's own doc comment). Group path reuses
// weights.ByMarket — already the stage-2 per-market total groupVolumes
// computed for this same request, no extra query — after stripping the
// "moa:" market-ID prefix that convention uses (spatialMarket.marketCode
// and SpatialMarketPrice.MarketCode are always bare). Crop path has exactly
// one series per market so aggregateMarkets never queries volume for it,
// but a genuine cross-market weight still needs one, so this queries it
// directly via ComputeCropVolumes.
func (r *marketResolver) marketVolumesForStatistics(ctx context.Context, selectionKind, selectionCode string, weights forecast.GroupVolumes, originDate forecast.Date, cutoff time.Time) (map[string]float64, error) {
	if selectionKind == "group" {
		out := make(map[string]float64, len(weights.ByMarket))
		for marketID, volume := range weights.ByMarket {
			out[strings.TrimPrefix(marketID, "moa:")] = volume
		}
		return out, nil
	}
	if r.group == nil {
		return nil, nil
	}
	vols, err := r.group.ComputeCropVolumes(ctx, selectionCode, originDate, cutoff)
	if err != nil {
		return nil, fmt.Errorf("usecase: ComputeCropVolumes(%s): %w", selectionCode, err)
	}
	return vols, nil
}

// aggregateMarkets runs SeriesForecast once per (market, crop) pair the
// artifact actually covers and folds the results into one row per market.
//
// The per-market value is the VOLUME-WEIGHTED average of the contributing
// sub-variants' quantiles, via weightedValue → forecast.WeightedAverage,
// over the weights variantWeights fetched once per request from
// Forecast.ComputeGroupVolumes. Those are exactly GroupForecast's stage-1
// weights (GroupVolumes.ByVariantMarket, "this sub-variant's recent volume
// at this market"), so the surface cannot disagree with the curve or the
// trend page about what a group is worth at one market. Only stage 1
// applies here: GroupForecast's stage 2 collapses markets into one national
// number, which is the opposite of what a surface needs — every market
// must survive as its own interpolation point.
//
// The extra DB round trip ComputeGroupVolumes costs is paid deliberately
// and only on the group path. The crop path has exactly one series per
// market, where any weighting is arithmetically a no-op, so variantWeights
// skips the query and WeightedAverage's documented equal-weight fallback
// takes over — the same fallback that covers a group whose recent volume
// is genuinely all zero.
//
// P10/P50/P90 are each weighted from their own values, and a sub-variant
// missing any of the three is dropped from all three averages, mirroring
// group.go's aggregateOneHorizon — a quantile must never be derived from
// its neighbour.
//
// Do not confuse this step with consolidateLocations further down this
// file, which merges markets that project to the same metre and is
// deliberately still UNWEIGHTED. That is a different question at a
// different stage: by then each market is a single number (its volumes
// already folded in here), spatialMarket carries no volume field for the
// merge to weight by, and Python's counterpart is merging raw observations
// that still carry their own volume_kg.
//
// SpatialMarketPrice.RecentVolumeKg stays nil either way — see its own doc
// comment for why fetching these weights did not make it fillable.
//
// Unlike forecast_curve.go's marketComparison, a failing SeriesForecast is
// fatal here rather than skipped: comparison data is supplementary, but
// these markets *are* the surface's inputs, and this usecase has no logger
// to make a silent drop observable.
func (r *marketResolver) aggregateMarkets(ctx context.Context, marketCodes, cropCodes []string, weights forecast.GroupVolumes, in marketSelectionInput) ([]spatialMarket, SpatialLineage, error) {
	forecaster := r.perRequestForecaster()
	lineage := SpatialLineage{
		ArtifactID:    r.artifact.ModelVersion,
		ModelVersion:  r.artifact.ModelVersion,
		GeneratedAt:   in.Now,
		SchemaVersion: dataMetaSchemaVersion,
	}
	if r.artifact.DatasetDigest != "" {
		lineage.DatasetDigest = "sha256:" + r.artifact.DatasetDigest
	}
	lineage.BoundaryDigest = r.geography.BoundaryDigest()

	countyByCode := make(map[string]geography.CountyRegion)
	for _, c := range r.geography.RegionsInScope() {
		countyByCode[c.CountyCode] = c
	}

	var dataThrough *forecast.Date
	var maxAvailable *time.Time
	markets := make([]spatialMarket, 0, len(marketCodes))
	for _, marketCode := range marketCodes {
		m := spatialMarket{marketCode: marketCode}
		var p50s, p10s, p90s, variantWeights []float64
		var contributingStatus forecast.PredictionStatus
		var contributingFreshness forecast.FreshnessLabel
		coveredSeries := 0
		for _, cropCode := range cropCodes {
			if !r.artifact.HasSeries(marketCode, cropCode) {
				continue
			}
			coveredSeries++
			out, err := forecaster.SeriesForecast(ctx, SeriesForecastInput{
				Artifact: r.artifact, MarketCode: marketCode, CropCode: cropCode, Origin: in.Now,
			})
			if err != nil {
				return nil, SpatialLineage{}, fmt.Errorf("usecase: SeriesForecast(%s, %s): %w", marketCode, cropCode, err)
			}
			if out == nil {
				continue
			}
			if m.categoryCode == "" && out.CategoryCode != nil {
				m.categoryCode = *out.CategoryCode
			}
			if m.categoryCode == "" {
				if series, ok := r.artifact.Series["moa:"+marketCode+":"+cropCode]; ok && series.CategoryCode != nil {
					m.categoryCode = *series.CategoryCode
				}
			}
			if out.DataThroughDate != nil && (dataThrough == nil || dataThrough.Before(*out.DataThroughDate)) {
				d := *out.DataThroughDate
				dataThrough = &d
			}
			if out.MaxFeatureAvailableAt != nil && (maxAvailable == nil || maxAvailable.Before(*out.MaxFeatureAvailableAt)) {
				t := *out.MaxFeatureAvailableAt
				maxAvailable = &t
			}
			point, ok := pointAtHorizon(out.Forecast, in.HorizonDays)
			if !ok {
				continue
			}
			m.targetDate = point.TargetDate
			m.predictionStatus = worstPredictionStatus(m.predictionStatus, point.Status)
			// A sub-variant missing ANY of its three quantiles is dropped
			// from all three averages rather than having the gap filled
			// from a neighbouring quantile. This mirrors group.go's
			// aggregateOneHorizon, which skips such a participant outright:
			// P10/P50/P90 must each be weighted from real P10/P50/P90s,
			// never derived from one another, or the band silently narrows
			// toward the median. Structurally unreachable today —
			// forecast.OrderedNonNegative produces all three or the point
			// carries none — so this is a guard, not a live branch.
			if !usablePredictionStatus(point.Status) || point.P10 == nil || point.P50 == nil || point.P90 == nil {
				continue
			}
			contributingStatus = worstPredictionStatus(contributingStatus, point.Status)
			contributingFreshness = worstFreshness(contributingFreshness, out.Freshness)
			p50s = append(p50s, *point.P50)
			p10s = append(p10s, *point.P10)
			p90s = append(p90s, *point.P90)
			variantWeights = append(variantWeights, weights.ByVariantMarket[forecast.VariantMarketKey{
				CropID: "moa:" + cropCode, MarketID: "moa:" + marketCode,
			}])
		}
		if len(p50s) > 0 {
			// Report the status the usable points actually carried —
			// forcing "ready" here would hide a group whose contributing
			// sub-variants were only partial.
			m.predictionStatus = contributingStatus
			if len(p50s) < coveredSeries {
				// Some sub-variant the model covers at this market produced
				// nothing usable. That is exactly AggregateGroup's "partial"
				// (group.go): the number is real but built from fewer
				// variants than the market actually carries.
				m.predictionStatus = worstPredictionStatus(m.predictionStatus, forecast.PredictionStatusPartial)
			}
			m.value = weightedValue(p50s, variantWeights)
			m.p10, m.p90 = weightedValue(p10s, variantWeights), weightedValue(p90s, variantWeights)
			m.freshness = contributingFreshness
		}
		if f, ok := r.resolveFacility(marketCode, m.categoryCode, in.Now); ok {
			m.facility, m.hasFacility = f, true
			if f.CountyCode != "" {
				m.county, m.hasCounty = countyByCode[f.CountyCode]
			}
			if m.categoryCode == "" && len(f.CategoryCodes) > 0 {
				m.categoryCode = f.CategoryCodes[0]
			}
		}
		markets = append(markets, m)
	}
	if dataThrough != nil {
		t := forecast.SourceDataAsOfUTC(*dataThrough)
		lineage.SourceDataAsOf = &t
	}
	lineage.MaxFeatureAvailableAt = maxAvailable
	return markets, lineage, nil
}

// SpatialSurface computes the nationwide crop/group price surface entirely
// from pricebook-service's own already-loaded model artifact and geography
// registry — see docs/superpowers/specs/2026-08-08-pricebook-spatial-surface-go-native-design.md.
type SpatialSurface struct {
	resolver *marketResolver
	scales   map[string]spatial.PriceScale
}

func NewSpatialSurface(artifact forecast.ModelArtifact, geo geography.Registry, scales map[string]spatial.PriceScale, forecaster seriesForecaster, groupUC *Forecast) *SpatialSurface {
	return &SpatialSurface{
		resolver: &marketResolver{artifact: artifact, geography: geo, forecast: forecaster, group: groupUC},
		scales:   scales,
	}
}

type SpatialSurfaceInput struct {
	CropCode    *string
	GroupCode   *string
	HorizonDays int
	Now         time.Time
}

// SpatialContourSegment is one isoline segment already projected to WGS84
// — spatial.BuildContours deliberately leaves its output in TM2 meters
// (see its doc comment: projection is the caller's job), and this usecase
// is that caller. Left/Right are [lon, lat], rounded to 6 decimals to
// match spatial.BuildSurfaceCells' own ring rounding.
type SpatialContourSegment struct {
	Left          [2]float64
	Right         [2]float64
	LevelValue    float64
	SurfaceRegion string
}

// SpatialCountyRollup is one county's entry in boundaryPriceGeojson.
// EstimatedP50TwdPerKg is nil — never 0, never a national average — for a
// county no surface cell fell inside (issue #2248: "缺資料行政區回 null +
// status/reason，不得補 0 或全國平均"). Geometry is the county's own
// boundary geometry straight from geography.CountyRegion, so the handler
// can emit a renderable choropleth without needing its own geography
// dependency.
type SpatialCountyRollup struct {
	CountyCode           string
	CountyName           string
	CountyNameEN         string
	Status               string // ready | unavailable
	EstimatedP50TwdPerKg *float64
	BandLower            *float64
	BandUpper            *float64
	SupportingCellCount  int
	Geometry             orb.Geometry
}

// SpatialMarketPrice is one market's row in both marketPrices and
// marketPointsGeojson — the two wire fields carry the same set of markets
// (the GeoJSON one adds geometry), so they share one Go type rather than
// two lists that could drift apart.
type SpatialMarketPrice struct {
	MarketCode       string
	MarketName       string
	CategoryCode     string
	PredictionStatus string // forecast.PredictionStatus of the requested horizon
	// MarketStatus describes the *target* date, reusing ObservedPoint's
	// vocabulary: "closed" when the horizon point is market_closed, "open"
	// when it produced a usable P50, "missing" otherwise.
	MarketStatus    string
	UsedForSurface  bool
	ExclusionReason string // "" when UsedForSurface
	P10, P50, P90   *float64
	// RecentVolumeKg is never assigned by this usecase — the wire field
	// exists, and the handler maps it, but nothing in the pipeline fills
	// it. Not for want of volume data: aggregateMarkets does fetch recent
	// volumes on the group path (variantWeights → ComputeGroupVolumes) and
	// weights the quantiles with them. Those weights are group-scoped and
	// keyed by (crop, market) though, so on the group path this row has no
	// single honest number to report, and on the crop path the query is
	// skipped entirely — a field that were populated for groups and nil for
	// crops would be worse than one that is consistently nil. Populating it
	// means deciding what it should mean for both paths first.
	RecentVolumeKg      *float64
	Longitude, Latitude *float64
	SurfaceRegion       string
	CountyCode          string
	CountyName          string
}

// SpatialStatistics mirrors the wire contract's statistics object.
type SpatialStatistics struct {
	PricedMarketCount   int
	ClosedMarketCount   int
	TotalMarketCount    int
	SurfaceCellCount    int
	ContourSegmentCount int
	// P50/MinimumP50/MaximumP50: cross-market summary over every market
	// with a non-nil P50 (weightedMedianP50's doc comment has the full
	// weighting rule). nil together, only when PricedMarketCount is 0.
	P50, MinimumP50, MaximumP50 *float64
}

// SpatialLineage is the provenance block. Everything in it derives from
// this service's own two immutable inputs — the baked model artifact and
// the mirrored boundary file — so there is no cross-service snapshot to
// fall out of sync (design doc Architecture).
type SpatialLineage struct {
	// ArtifactID equals ModelVersion: this service loads exactly one
	// model.json and its model_version is already content-addressed
	// (e.g. "direct-ridge-v2-aa66e30432a2729a"), so there is no separate
	// artifact identifier to report — inventing one would be a second,
	// unverifiable identity axis.
	ArtifactID            string
	ModelVersion          string
	DatasetDigest         string // "sha256:"-prefixed, same convention as DataMeta
	BoundaryDigest        string
	GeneratedAt           time.Time
	SourceDataAsOf        *time.Time
	MaxFeatureAvailableAt *time.Time
	SchemaVersion         string
}

// SpatialBoundaryOutput carries the raw county boundary geometry the
// handler serializes into the wire boundary field — separate from
// CountyRollups, which carries the price-annotated choropleth layer.
// Geometry stays Go-native here for the same reason SpatialSurfaceOutput's
// own doc comment gives for CountyRollups: the proto dependency belongs to
// the handler, not this layer.
type SpatialBoundaryOutput struct {
	Version   string
	Digest    string
	BBoxWGS84 [4]float64
	Regions   []geography.CountyRegion
}

// SpatialSurfaceOutput is the usecase-level DTO internal/grpc's
// GetSpatialSurface handler maps to the proto response (never the other
// way around). Geometry stays Go-native here — the four GeoJSON
// FeatureCollections the wire contract names (boundaryPriceGeojson /
// marketPointsGeojson / surfaceCellsGeojson / contourLinesGeojson) are
// assembled from CountyRollups / MarketPrices / SurfaceCells /
// ContourSegments respectively by the handler, so this layer stays
// testable without a proto dependency.
type SpatialSurfaceOutput struct {
	Status              string
	DisplayMode         string
	SelectionKind       string // crop | group
	SelectionCode       string
	CategoryCode        string
	TargetDate          forecast.Date
	HorizonDays         int
	Currency            string
	Unit                string
	Scale               spatial.PriceScale
	SurfaceCells        []spatial.SurfaceCell
	ContourSegments     []SpatialContourSegment
	CountyRollups       []SpatialCountyRollup
	MarketPrices        []SpatialMarketPrice
	Statistics          SpatialStatistics
	QualityReasonCodes  []string
	Disclosures         []string
	ExcludedMarketCodes []string
	Lineage             SpatialLineage
	Boundary            SpatialBoundaryOutput

	// SupportedLandAreaRatio is the quality gate's own
	// spatialReasonUnsupportedLandArea denominator, surfaced instead of
	// discarded — nil when no grid was ever attempted (base.Lineage or the
	// scale/counties short-circuit paths in Get), so callers can tell "no
	// data" apart from "0% supported".
	SupportedLandAreaRatio *float64
}

func pointAtHorizon(points []ForecastPoint, horizonDays int) (ForecastPoint, bool) {
	for _, p := range points {
		if p.HorizonDays == horizonDays {
			return p, true
		}
	}
	return ForecastPoint{}, false
}

// Get computes the surface. Its error contract is the design doc's Error
// Handling section: only the two request-shape violations above, plus
// genuine infrastructure failures (the group path's CropsInGroup query,
// the per-market SeriesForecast run, an unusable calendar timezone),
// return a non-nil error. Everything a seller could legitimately hit —
// an unmodeled crop, every market closed, too few geocoded markets to
// interpolate, no approved price scale — comes back as
// Status/QualityReasonCodes on a successful call.
func (u *SpatialSurface) Get(ctx context.Context, in SpatialSurfaceInput) (SpatialSurfaceOutput, error) {
	if (in.CropCode == nil) == (in.GroupCode == nil) {
		return SpatialSurfaceOutput{}, ErrSpatialExactlyOneSelection
	}
	if in.HorizonDays < 1 || in.HorizonDays > 7 {
		return SpatialSurfaceOutput{}, ErrSpatialHorizonOutOfRange
	}
	if in.Now.IsZero() {
		return SpatialSurfaceOutput{}, ErrSpatialZeroNow
	}
	originDate, _, err := forecast.OriginDateAndCutoff(in.Now, calendarTimezoneOf(u.resolver.artifact))
	if err != nil {
		return SpatialSurfaceOutput{}, fmt.Errorf("usecase: spatial surface: %w", err)
	}

	res, err := u.resolver.resolveAndForecastMarkets(ctx, marketSelectionInput(in))
	if err != nil {
		return SpatialSurfaceOutput{}, err
	}
	selectionKind, selectionCode := res.SelectionKind, res.SelectionCode
	marketCodes := res.MarketCodes
	base := SpatialSurfaceOutput{
		SelectionKind: selectionKind, SelectionCode: selectionCode,
		HorizonDays: in.HorizonDays, TargetDate: originDate.AddDays(in.HorizonDays),
		Currency: dataMetaCurrency, Unit: dataMetaUnit,
	}
	if len(marketCodes) == 0 {
		// The selection is syntactically valid but nothing in the artifact
		// covers it (unknown crop, group with no modelled sub-variants).
		return u.finish(base, []string{spatialReasonNoScoreableTradeRows}, nil, nil, nil, nil, in.Now, nil, nil), nil
	}
	markets, lineage := res.Markets, res.Lineage
	marketVolumes := res.MarketVolumes
	// Prefer the calendar-aware target date the forecast itself produced;
	// markets[0] may be one that produced no point at all, so scan for the
	// first that did rather than trusting the first row.
	for _, m := range markets {
		if m.targetDate != (forecast.Date{}) {
			base.TargetDate = m.targetDate
			break
		}
	}
	base.Lineage = lineage

	var reasons []string
	closedCount := 0
	pricedCount := 0
	for _, m := range markets {
		if m.predictionStatus == forecast.PredictionStatusMarketClosed {
			closedCount++
		}
		if m.value != nil {
			pricedCount++
		}
	}
	if pricedCount == 0 && closedCount == len(markets) {
		reasons = append(reasons, spatialReasonAllScopedMarketsClosed)
	}

	base.CategoryCode = dominantCategoryCode(markets)
	basePolicy, scaleOK := u.priceScaleFor(selectionKind, selectionCode, base.CategoryCode)
	base.Scale = basePolicy

	counties := u.resolver.geography.RegionsInScope()
	if len(counties) == 0 {
		reasons = append(reasons, spatialReasonBoundaryUnavailable)
	}
	if !scaleOK {
		reasons = append(reasons, spatialReasonPriceScaleUnavailable)
	}

	if !scaleOK || len(counties) == 0 {
		// No grid was even attempted, so no market fed a surface — the rows
		// must say so rather than reporting pre-build eligibility. base.Scale
		// stays the raw #2248 reviewed policy here (never adaptive) since no
		// surface exists to band in the first place.
		return u.finish(base, reasons, nil, nil, nil, u.marketPrices(markets, nil), in.Now, nil, marketVolumes), nil
	}

	// issue #3102: breaks are no longer the reviewed policy's fixed values —
	// they are derived per selection from this same call's own P50 domain.
	// weightedMedianP50 doesn't depend on usedMarketCodes, so computing it
	// here and again in finish() (for out.Statistics) is intentional
	// duplication, not a bug — both calls see the same prices/volumes and
	// must agree.
	median, minimum, maximum := weightedMedianP50(u.marketPrices(markets, nil), marketVolumes)
	var domain spatial.AdaptiveScaleDomain
	if median != nil {
		domain = spatial.AdaptiveScaleDomain{Minimum: *minimum, Median: *median, Maximum: *maximum}
	}
	base.Scale = spatial.DeriveAdaptiveScale(domain, spatial.AdaptiveScaleIdentity{
		SelectionKind: selectionKind,
		SelectionCode: selectionCode,
		TargetDate:    base.TargetDate.String(),
		HorizonDays:   in.HorizonDays,
		ModelVersion:  u.resolver.artifact.ModelVersion,
		BasePolicyID:  basePolicy.ScaleID,
	})

	cells, contours, usedMarketCodes, surfaceReasons, landAreaRatio, err := u.buildSurface(markets, counties, base.Scale.Breaks)
	if err != nil {
		return SpatialSurfaceOutput{}, err
	}
	reasons = append(reasons, surfaceReasons...)
	rollups := u.countyRollups(cells, counties, base.Scale.Breaks)
	return u.finish(base, reasons, cells, contours, rollups, u.marketPrices(markets, usedMarketCodes), in.Now, landAreaRatio, marketVolumes), nil
}

// spatialMarket is one market's consolidated view of the selection at the
// requested horizon.
type spatialMarket struct {
	marketCode       string
	categoryCode     string
	predictionStatus forecast.PredictionStatus
	freshness        forecast.FreshnessLabel
	targetDate       forecast.Date
	value            *float64 // volume-weighted P50 over contributing series (see aggregateMarkets), nil when unusable
	p10, p90         *float64
	facility         geography.Facility
	hasFacility      bool
	county           geography.CountyRegion
	hasCounty        bool
}

func usablePredictionStatus(s forecast.PredictionStatus) bool {
	return s == forecast.PredictionStatusReady ||
		s == forecast.PredictionStatusPartial ||
		s == forecast.PredictionStatusMarketClosed
}

// worstPredictionStatus folds several sub-variants' statuses at one market
// into the one a caller should trust least, ranked by
// forecast.PredictionStatusSeverity — the ordering ComputeComparison
// already uses, deliberately not a second hand-rolled table. "First
// non-empty wins" would let a group whose A1 is ready and A2 is partial
// report ready purely because crop codes sort A1 first, overstating how
// solid the number on screen is.
func worstPredictionStatus(current, candidate forecast.PredictionStatus) forecast.PredictionStatus {
	if current == "" {
		return candidate
	}
	if candidate == "" {
		return current
	}
	if forecast.PredictionStatusSeverity(candidate) > forecast.PredictionStatusSeverity(current) {
		return candidate
	}
	return current
}

// worstFreshness is worstPredictionStatus's counterpart for
// forecast.FreshnessLabel — same reasoning: folding several sub-variants'
// freshness at one market (or several markets' freshness at one county)
// must report the one a caller should trust least, not whichever happened
// to be seen first.
func worstFreshness(current, candidate forecast.FreshnessLabel) forecast.FreshnessLabel {
	if current == "" {
		return candidate
	}
	if candidate == "" {
		return current
	}
	if forecast.FreshnessSeverity(candidate) > forecast.FreshnessSeverity(current) {
		return candidate
	}
	return current
}

// weightedValue folds one market's contributing sub-variants into a single
// number using forecast.WeightedAverage — the same volume weighting
// GroupForecast/GroupObservedSeries apply, so the surface, the curve and
// the trend page cannot disagree about a group's price at a market. With
// no positive weights (crop path, or a group with no recent volume) it
// degrades to the equal-weight mean, which is WeightedAverage's own
// documented fallback, not a second rule invented here.
func weightedValue(values, weights []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	v, _ := forecast.WeightedAverage(values, weights)
	return &v
}

func meanOf(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	return &mean
}

// dominantCategoryCode picks the category the selection mostly belongs to,
// breaking ties lexicographically so the choice — and therefore the price
// scale — is reproducible.
func dominantCategoryCode(markets []spatialMarket) string {
	counts := map[string]int{}
	for _, m := range markets {
		if m.categoryCode != "" {
			counts[m.categoryCode]++
		}
	}
	best, bestCount := "", 0
	codes := make([]string, 0, len(counts))
	for code := range counts {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		if counts[code] > bestCount {
			best, bestCount = code, counts[code]
		}
	}
	return best
}

// priceScaleFor resolves the versioned #2248 fixed scale for this
// selection — as of issue #3102 this result (basePolicy in Get()) is used
// only as an eligibility/provenance gate, not as the breaks returned to the
// client (see DeriveAdaptiveScale). An explicit per-selection row wins;
// otherwise the category-level scale applies — which is what
// reference/price_scale.v1.csv actually ships today (its rows are keyed
// group:N04/N05/N06, i.e. MOA category codes, see that file's manifest
// sources). No approved row at all is a business outcome, not an error:
// building a surface without an approved base policy would invent
// eligibility issue #2248 never granted.
func (u *SpatialSurface) priceScaleFor(selectionKind, selectionCode, categoryCode string) (spatial.PriceScale, bool) {
	if scale, ok := u.scales[selectionKind+":"+selectionCode]; ok {
		return scale, true
	}
	if categoryCode != "" {
		if scale, ok := u.scales["group:"+categoryCode]; ok {
			return scale, true
		}
	}
	return spatial.PriceScale{}, false
}

// marketPrices renders the per-market table. usedMarketCodes is the set
// buildSurface reports as having actually reached a built grid — NOT
// "looked eligible before the grid step", which would claim
// usedForSurface: true on a response carrying no surface at all (too few
// points to interpolate, no approved price scale, no boundary).
//
// Exclusion reasons follow interpolation.py's _point_features, plus
// no_usable_price for the forecast-only case: no number is a different
// story from no coordinate, and both differ from "geocoded and priced, but
// its region never produced a grid" (outside_or_unsupported_domain — the
// same bucket Python puts an unsupported region's markets in).
func (u *SpatialSurface) marketPrices(markets []spatialMarket, usedMarketCodes map[string]bool) []SpatialMarketPrice {
	out := make([]SpatialMarketPrice, 0, len(markets))
	for _, m := range markets {
		row := SpatialMarketPrice{
			MarketCode: m.marketCode, MarketName: m.marketCode, CategoryCode: m.categoryCode,
			PredictionStatus: string(m.predictionStatus), MarketStatus: marketStatusOf(m),
			P50: m.value, P10: m.p10, P90: m.p90,
		}
		if m.hasFacility {
			if m.facility.FacilityName != "" {
				row.MarketName = m.facility.FacilityName
			} else if m.facility.SourceMarketName != "" {
				row.MarketName = m.facility.SourceMarketName
			}
			row.Longitude, row.Latitude = m.facility.Longitude, m.facility.Latitude
			row.CountyCode, row.CountyName = m.facility.CountyCode, m.facility.CountyName
		}
		if m.hasCounty {
			row.SurfaceRegion = m.county.SurfaceRegion
		}
		switch {
		case usedMarketCodes[m.marketCode]:
			row.UsedForSurface = true
		case m.value == nil:
			row.ExclusionReason = spatialExclusionNoUsablePrice
		case !m.hasFacility || m.facility.Longitude == nil || m.facility.Latitude == nil:
			row.ExclusionReason = spatialExclusionMissingCoordinate
		default:
			row.ExclusionReason = spatialExclusionOutsideDomain
		}
		out = append(out, row)
	}
	return out
}

func marketStatusOf(m spatialMarket) string {
	switch {
	case m.predictionStatus == forecast.PredictionStatusMarketClosed:
		return "closed"
	case m.value != nil:
		return "open"
	default:
		return "missing"
	}
}

// buildSurface is the geometric core: group the mirrored counties into
// their coarse interpolation regions, build one grid per region, IDW every
// land node from that region's markets, then band the result into cells
// and isolines.
//
// The 22 mirrored counties resolve to 4 surface regions
// (taiwan_main_island on EPSG 3826; kinmen, matsu and penghu on EPSG
// 3825), not the 3 the design doc's prose mentions — penghu was added to
// the boundary data afterwards and shares the outlying islands' CRS, so
// nothing in the projection layer needed to change.
func (u *SpatialSurface) buildSurface(markets []spatialMarket, counties []geography.CountyRegion, breaks []float64) ([]spatial.SurfaceCell, []SpatialContourSegment, map[string]bool, []string, *float64, error) {
	usedMarketCodes := map[string]bool{}
	countiesByRegion := map[string][]geography.CountyRegion{}
	var regionNames []string
	for _, c := range counties {
		if _, seen := countiesByRegion[c.SurfaceRegion]; !seen {
			regionNames = append(regionNames, c.SurfaceRegion)
		}
		countiesByRegion[c.SurfaceRegion] = append(countiesByRegion[c.SurfaceRegion], c)
	}
	sort.Strings(regionNames)

	marketsByRegion := map[string][]spatialMarket{}
	reasonSet := map[string]bool{}
	for _, m := range markets {
		if m.value == nil {
			continue
		}
		switch {
		case !m.hasFacility || m.facility.Longitude == nil || m.facility.Latitude == nil:
			reasonSet[spatialReasonMissingMarketGeocodes] = true
		case !m.hasCounty:
			reasonSet[spatialReasonOutsideBoundaryDomain] = true
		default:
			marketsByRegion[m.county.SurfaceRegion] = append(marketsByRegion[m.county.SurfaceRegion], m)
		}
	}

	var cells []spatial.SurfaceCell
	var contours []SpatialContourSegment
	landNodes, supportedNodes := 0, 0
	unsupportedRegions := 0
	allFlat := true
	anyGrid := false

	for _, regionName := range regionNames {
		regionMarkets := marketsByRegion[regionName]
		if len(regionMarkets) == 0 {
			// A region with no markets at all is simply out of this
			// selection's scope, not "unsupported". Transformer counts it
			// as unsupported, but with the real mirrored data kinmen/matsu/
			// penghu have no wholesale markets in any category, which would
			// make "degraded" permanent and strip the status field of any
			// signal — the same trap ADR D1 (forecast.go's
			// totalVariantCount) already fixed once for group readiness.
			continue
		}
		regionCounties := countiesByRegion[regionName]
		epsg := regionCounties[0].InterpolationEPSG
		for _, c := range regionCounties {
			if c.InterpolationEPSG != epsg {
				return nil, nil, nil, nil, nil, fmt.Errorf("usecase: surface region %q mixes interpolation EPSG %d and %d", regionName, epsg, c.InterpolationEPSG)
			}
		}
		polygons, err := projectCounties(regionCounties, epsg)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		locations, err := consolidateLocations(regionMarkets, epsg)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if len(locations) < spatialMinPoints {
			reasonSet[spatialReasonInsufficientSpatialPoints] = true
			unsupportedRegions++
			continue
		}
		if !hasNonCollinearPoints(locations) {
			reasonSet[spatialReasonCollinearSpatialPoints] = true
			unsupportedRegions++
			continue
		}
		grid, err := spatial.BuildRegionGrid(polygons, spatialGridResolutionM, spatialMaxGridCellsPerRegion)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("usecase: building grid for surface region %q: %w", regionName, err)
		}
		// The grid exists, so every market consolidated into this region's
		// locations really did feed the interpolation — mirrors Python's
		// used_market_ids, which it likewise derives from the grids that
		// got built, not from pre-build eligibility.
		for _, m := range regionMarkets {
			usedMarketCodes[m.marketCode] = true
		}
		values, land, supported := interpolateGrid(grid, locations)
		landNodes += land
		supportedNodes += supported
		anyGrid = true
		if !flatValues(locations) {
			allFlat = false
		}
		regionCells, err := spatial.BuildSurfaceCells(grid, values, breaks, epsg)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		cells = append(cells, regionCells...)
		segments, err := spatial.BuildContours(grid, values, breaks)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		projected, err := projectContours(segments, epsg, regionName)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		contours = append(contours, projected...)
	}

	if unsupportedRegions > 0 {
		reasonSet[spatialReasonUnsupportedBoundaryRegions] = true
	}
	if anyGrid && landNodes > 0 && float64(supportedNodes)/float64(landNodes) < spatialMinSupportedLandRatio {
		reasonSet[spatialReasonUnsupportedLandArea] = true
	}
	if anyGrid && allFlat {
		reasonSet[spatialReasonFlatPriceField] = true
	}
	if len(cells) == 0 {
		reasonSet[spatialReasonInsufficientSpatialCover] = true
	}
	reasons := make([]string, 0, len(reasonSet))
	for reason := range reasonSet {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)

	// landAreaRatio reuses the same supportedNodes/landNodes fraction the
	// spatialReasonUnsupportedLandArea gate above already computed, instead
	// of silently discarding it — nil (not 0) when no region ever built a
	// grid, so "no attempt" and "0% supported" stay distinguishable.
	var landAreaRatio *float64
	if anyGrid && landNodes > 0 {
		ratio := float64(supportedNodes) / float64(landNodes)
		landAreaRatio = &ratio
	}
	return cells, contours, usedMarketCodes, reasons, landAreaRatio, nil
}

// interpolateGrid runs IDWAt once per land node, leaving unsupported nodes
// NaN exactly as Transformer's numpy path does (np.nan is what
// BuildSurfaceCells/BuildContours already skip on).
func interpolateGrid(grid spatial.RegionGrid, locations []spatial.Location) (values [][]float64, landNodes, supportedNodes int) {
	cfg := spatial.IDWConfig{
		Power:                  spatialIDWPower,
		MaxSupportDistanceM:    spatialMaxSupportDistanceM,
		MinSupportingNeighbors: spatialMinSupportingNeighbors,
	}
	values = make([][]float64, len(grid.YAxis))
	for yi, y := range grid.YAxis {
		row := make([]float64, len(grid.XAxis))
		for xi, x := range grid.XAxis {
			row[xi] = math.NaN()
			if !grid.LandMask[yi][xi] {
				continue
			}
			landNodes++
			if v, _, _, ok := spatial.IDWAt(x, y, locations, cfg); ok {
				row[xi] = v
				supportedNodes++
			}
		}
		values[yi] = row
	}
	return values, landNodes, supportedNodes
}

// consolidateLocations mirrors interpolation.py's _consolidate_locations:
// markets whose projected coordinates round to the same metre share one
// interpolation point, so a facility serving several markets does not get
// extra weight. Ordering is by rounded (x, y), matching Python's
// `for key in sorted(grouped)`.
func consolidateLocations(markets []spatialMarket, epsg int) ([]spatial.Location, error) {
	type key struct{ x, y int64 }
	grouped := map[key][]float64{}
	exact := map[key][2]float64{}
	var keys []key
	for _, m := range markets {
		x, y, err := spatial.WGS84ToTWD97TM2(*m.facility.Longitude, *m.facility.Latitude, epsg)
		if err != nil {
			return nil, fmt.Errorf("usecase: projecting market %s: %w", m.marketCode, err)
		}
		k := key{x: int64(math.Round(x)), y: int64(math.Round(y))}
		if _, seen := grouped[k]; !seen {
			keys = append(keys, k)
			exact[k] = [2]float64{x, y}
		}
		grouped[k] = append(grouped[k], *m.value)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].x != keys[j].x {
			return keys[i].x < keys[j].x
		}
		return keys[i].y < keys[j].y
	})
	locations := make([]spatial.Location, 0, len(keys))
	for _, k := range keys {
		locations = append(locations, spatial.Location{X: exact[k][0], Y: exact[k][1], Value: *meanOf(grouped[k])})
	}
	return locations, nil
}

// hasNonCollinearPoints mirrors interpolation.py's
// _has_non_collinear_points — IDW over points on a single line produces a
// surface with no lateral information, which Transformer refuses to draw.
func hasNonCollinearPoints(locations []spatial.Location) bool {
	if len(locations) < 3 {
		return false
	}
	origin := locations[0]
	for left := 1; left < len(locations); left++ {
		lx, ly := locations[left].X-origin.X, locations[left].Y-origin.Y
		for right := left + 1; right < len(locations); right++ {
			rx, ry := locations[right].X-origin.X, locations[right].Y-origin.Y
			if math.Abs(lx*ry-ly*rx) > 1.0 {
				return true
			}
		}
	}
	return false
}

// flatValues mirrors interpolation.py's
// `len({round(location.value, 10) for location in grid.locations}) == 1`
// — the 1e-10 tolerance is that rounding, not an arbitrary epsilon.
func flatValues(locations []spatial.Location) bool {
	for _, l := range locations[1:] {
		if math.Abs(l.Value-locations[0].Value) > 1e-10 {
			return false
		}
	}
	return true
}

// projectCounties turns a surface region's counties into the planar
// polygons BuildRegionGrid expects. orb geometry is lon/lat; the grid is
// TM2 metres.
func projectCounties(counties []geography.CountyRegion, epsg int) ([]spatial.Polygon, error) {
	var polygons []spatial.Polygon
	for _, county := range counties {
		switch g := county.Geometry.(type) {
		case orb.Polygon:
			p, err := projectPolygon(g, epsg)
			if err != nil {
				return nil, err
			}
			polygons = append(polygons, p)
		case orb.MultiPolygon:
			for _, member := range g {
				p, err := projectPolygon(member, epsg)
				if err != nil {
					return nil, err
				}
				polygons = append(polygons, p)
			}
		default:
			return nil, fmt.Errorf("usecase: county %s has unsupported geometry %T", county.CountyCode, county.Geometry)
		}
	}
	return polygons, nil
}

func projectPolygon(p orb.Polygon, epsg int) (spatial.Polygon, error) {
	out := make(spatial.Polygon, 0, len(p))
	for _, ring := range p {
		projected := make(spatial.Ring, 0, len(ring))
		for _, pt := range ring {
			x, y, err := spatial.WGS84ToTWD97TM2(pt[0], pt[1], epsg)
			if err != nil {
				return nil, fmt.Errorf("usecase: projecting boundary ring: %w", err)
			}
			projected = append(projected, spatial.Point{X: x, Y: y})
		}
		out = append(out, projected)
	}
	return out, nil
}

// projectContours converts BuildContours' TM2 output to WGS84 and drops
// segments that collapse to a point once rounded — the same guard
// interpolation.py applies (`if left_wgs84 == right_wgs84: continue`).
func projectContours(segments []spatial.ContourSegment, epsg int, regionName string) ([]SpatialContourSegment, error) {
	out := make([]SpatialContourSegment, 0, len(segments))
	for _, seg := range segments {
		left, err := toWGS84(seg.Left, epsg)
		if err != nil {
			return nil, err
		}
		right, err := toWGS84(seg.Right, epsg)
		if err != nil {
			return nil, err
		}
		if left == right {
			continue
		}
		out = append(out, SpatialContourSegment{Left: left, Right: right, LevelValue: seg.LevelValue, SurfaceRegion: regionName})
	}
	return out, nil
}

func toWGS84(p spatial.Point, epsg int) ([2]float64, error) {
	lon, lat, err := spatial.TWD97TM2ToWGS84(p.X, p.Y, epsg)
	if err != nil {
		return [2]float64{}, fmt.Errorf("usecase: unprojecting contour vertex: %w", err)
	}
	// spatial.RoundTo, not a local copy: the surface and contour layers must
	// agree vertex for vertex where they touch, which only holds if both go
	// through the same half-to-even rounding.
	return [2]float64{spatial.RoundTo(lon, 6), spatial.RoundTo(lat, 6)}, nil
}

// countyRollups merges spatial.RollupByCounty's area-weighted result back
// onto the full county list, so counties the surface never reached appear
// with a null price instead of being silently absent.
func (u *SpatialSurface) countyRollups(cells []spatial.SurfaceCell, counties []geography.CountyRegion, breaks []float64) []SpatialCountyRollup {
	byCode := map[string]spatial.CountyRollup{}
	for _, r := range spatial.RollupByCounty(cells, counties, breaks) {
		byCode[r.CountyCode] = r
	}
	out := make([]SpatialCountyRollup, 0, len(counties))
	for _, county := range counties {
		row := SpatialCountyRollup{
			CountyCode: county.CountyCode, CountyName: county.CountyName, CountyNameEN: county.CountyNameEN,
			Status: spatialStatusUnavailable, Geometry: county.Geometry,
		}
		if r, ok := byCode[county.CountyCode]; ok {
			value := r.EstimatedP50TwdPerKg
			row.Status = spatialStatusReady
			row.EstimatedP50TwdPerKg = &value
			row.BandLower, row.BandUpper = r.BandLower, r.BandUpper
			row.SupportingCellCount = r.SupportingCellCount
		}
		out = append(out, row)
	}
	return out
}

// finish assembles the status, display mode, disclosures and statistics
// that every return path shares. Status follows interpolation.py's rule:
// a surface with cells is ready when nothing was flagged and degraded
// otherwise; no cells at all is unavailable.
func (u *SpatialSurface) finish(base SpatialSurfaceOutput, reasons []string, cells []spatial.SurfaceCell, contours []SpatialContourSegment, rollups []SpatialCountyRollup, prices []SpatialMarketPrice, now time.Time, landAreaRatio *float64, volumes map[string]float64) SpatialSurfaceOutput {
	out := base
	out.SurfaceCells, out.ContourSegments, out.CountyRollups, out.MarketPrices = cells, contours, rollups, prices
	// Boundary is a property of the mirrored geography data, not of this
	// request's outcome, so it is populated unconditionally — every finish()
	// call site gets it, including the early-exit paths that never attempt
	// a grid. SupportedLandAreaRatio stays whatever buildSurface (or an
	// early-exit call site) decided: nil means no grid was ever attempted.
	out.Boundary = SpatialBoundaryOutput{
		Version:   u.resolver.geography.BoundaryVersion(),
		Digest:    u.resolver.geography.BoundaryDigest(),
		BBoxWGS84: u.resolver.geography.BBoxWGS84(),
		Regions:   u.resolver.geography.RegionsInScope(),
	}
	out.SupportedLandAreaRatio = landAreaRatio
	seen := map[string]bool{}
	out.QualityReasonCodes = nil
	for _, r := range reasons {
		if !seen[r] {
			seen[r] = true
			out.QualityReasonCodes = append(out.QualityReasonCodes, r)
		}
	}
	sort.Strings(out.QualityReasonCodes)

	switch {
	case len(cells) > 0 && len(out.QualityReasonCodes) == 0:
		out.Status = spatialStatusReady
	case len(cells) > 0:
		out.Status = spatialStatusDegraded
	default:
		out.Status = spatialStatusUnavailable
	}

	anyGeocodedMarket := false
	for _, p := range prices {
		if p.Longitude != nil && p.Latitude != nil {
			anyGeocodedMarket = true
			break
		}
	}
	switch {
	case out.Status != spatialStatusUnavailable && len(cells) > 0 && len(contours) > 0:
		out.DisplayMode = spatialDisplayContoursAndPoints
	case out.Status != spatialStatusUnavailable && len(cells) > 0:
		out.DisplayMode = spatialDisplaySurfaceAndPoints
	case anyGeocodedMarket:
		out.DisplayMode = spatialDisplayPointsOnly
	default:
		out.DisplayMode = spatialDisplayTableOnly
	}

	out.Disclosures = []string{spatialDisclosureMarketLocation, spatialDisclosureNotAdvice}
	if out.DisplayMode == spatialDisplayContoursAndPoints || out.DisplayMode == spatialDisplaySurfaceAndPoints {
		out.Disclosures = append(out.Disclosures, spatialDisclosureInterpolated)
	}

	out.Statistics = SpatialStatistics{
		TotalMarketCount:    len(prices),
		SurfaceCellCount:    len(cells),
		ContourSegmentCount: len(contours),
	}
	for _, p := range prices {
		if p.P50 != nil {
			out.Statistics.PricedMarketCount++
		}
		if p.MarketStatus == "closed" {
			out.Statistics.ClosedMarketCount++
		}
		if !p.UsedForSurface {
			out.ExcludedMarketCodes = append(out.ExcludedMarketCodes, p.MarketCode)
		}
	}
	out.Statistics.P50, out.Statistics.MinimumP50, out.Statistics.MaximumP50 = weightedMedianP50(prices, volumes)
	if out.Lineage.GeneratedAt.IsZero() {
		out.Lineage.GeneratedAt = now
		out.Lineage.ArtifactID = u.resolver.artifact.ModelVersion
		out.Lineage.ModelVersion = u.resolver.artifact.ModelVersion
		out.Lineage.SchemaVersion = dataMetaSchemaVersion
		out.Lineage.BoundaryDigest = u.resolver.geography.BoundaryDigest()
		if u.resolver.artifact.DatasetDigest != "" {
			out.Lineage.DatasetDigest = "sha256:" + u.resolver.artifact.DatasetDigest
		}
	}
	return out
}

// weightedMedianP50 computes statistics.p50/minimum_p50/maximum_p50: a
// cross-market summary over every market with a non-nil P50 (the same set
// PricedMarketCount counts) — a different axis from aggregateMarkets'
// per-market weighting, which blends a group's sub-variants *within* one
// market. This blends *between* markets.
//
// Weighted by volumes[market_code] (the caller resolves this from
// GroupVolumes.ByMarket on the group path, RecentVolumeForCrop on the crop
// path — see marketVolumesForStatistics). A market missing from volumes, or
// with a non-positive entry, falls back to an equal weight of 1 rather than
// being dropped — recent_volume_kg genuinely being unavailable for one
// market must not silently exclude that market's price from the summary,
// it only means that market can't out-weigh the others. This mirrors
// forecast.WeightedAverage's own documented equal-weight fallback for a
// group whose recent volume is all zero (aggregateMarkets' doc comment).
//
// The median is the standard weighted-median definition: sort by value,
// walk the cumulative weight, return the first value at which the running
// total reaches half the total weight. Returns (nil, nil, nil) when no
// market in prices has a P50 — nothing to summarize.
func weightedMedianP50(prices []SpatialMarketPrice, volumes map[string]float64) (median, minimum, maximum *float64) {
	type weighted struct {
		value  float64
		weight float64
	}
	entries := make([]weighted, 0, len(prices))
	for _, p := range prices {
		if p.P50 == nil {
			continue
		}
		weight := 1.0
		if v, ok := volumes[p.MarketCode]; ok && v > 0 {
			weight = v
		}
		entries = append(entries, weighted{value: *p.P50, weight: weight})
	}
	if len(entries) == 0 {
		return nil, nil, nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].value < entries[j].value })

	minV, maxV := entries[0].value, entries[len(entries)-1].value
	totalWeight := 0.0
	for _, e := range entries {
		totalWeight += e.weight
	}
	target := totalWeight / 2
	cumulative := 0.0
	medV := entries[len(entries)-1].value
	for _, e := range entries {
		cumulative += e.weight
		if cumulative >= target {
			medV = e.value
			break
		}
	}
	return &medV, &minV, &maxV
}
