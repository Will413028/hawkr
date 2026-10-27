// services/pricebook-service/internal/usecase/regional_comparison.go
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
)

// ErrRegionalComparisonExactlyOneSelection / ErrRegionalComparisonHorizonOutOfRange /
// ErrRegionalComparisonZeroNow mirror SpatialSurface's own three request-shape
// errors (see that file's doc comments for the reasoning behind each) —
// defined separately rather than shared, since the two-way "exactly one
// of crop_code/group_code" check and the 1-7 horizon check are each a few
// lines of trivial, unlikely-to-diverge validation logic, unlike the
// market-resolution pipeline Task 1 shares (design doc: only the
// genuinely complex, must-not-diverge logic is worth sharing).
var (
	ErrRegionalComparisonExactlyOneSelection = errors.New("usecase: exactly one of crop_code or group_code must be set")
	ErrRegionalComparisonHorizonOutOfRange   = errors.New("usecase: horizon_days must be between 1 and 7")
	ErrRegionalComparisonZeroNow             = errors.New("usecase: now must not be zero (a zero time.Time silently becomes SQL NULL and every as-of query returns 0 rows with err=nil)")
)

// regionalComparisonAggregationMethod names the actual algorithm — design
// doc D1/D3: volume-weighted average of each market's own P10/P50/P90
// (quantile averaging), reusing weightedValue(), never IDW. Deliberately
// distinct from spatialAggregationMethod ("surface_cell_area_weighted/v1"
// in internal/grpc/handler.go) so callers cannot assume the two numbers
// come from the same algorithm.
const regionalComparisonAggregationMethod = "market_volume_weighted_quantile_average/v1"

// regionalComparisonReasonNoMarketsInRegion is this feature's one new
// quality-reason code (design doc): a county with zero supporting markets
// even though the overall selection resolved to markets elsewhere.
const regionalComparisonReasonNoMarketsInRegion = "no_markets_in_region"

// CurrentMappingPolicyVersion 是目前唯一一套 crop/group/region 映射邏輯的
// 版本標籤。系統只有一種映射邏輯時版本化的意義是未來變更時能區分新舊，
// 不是現在就要多版本並存，因此先寫死常數而非接資料庫/設定檔。
const CurrentMappingPolicyVersion = "region-mapping.v1"

// RegionalComparisonInput mirrors SpatialSurfaceInput's shape — same
// two-way selection + 1-7 horizon caller contract (issue #2249 Slice C).
type RegionalComparisonInput struct {
	CropCode    *string
	GroupCode   *string
	HorizonDays int
	Now         time.Time
}

// RegionalComparisonRegion is one county's row. P10/P50/P90 are nil
// together with SupportingMarketCount == 0 and Status == "unavailable" —
// never 0, never a national average (issue #2249: "不得補全國平均").
type RegionalComparisonRegion struct {
	RegionCode string
	RegionName string
	Status     string // ready | unavailable
	// FreshnessStatus is the worst forecast.FreshnessLabel among this
	// county's contributing markets — "unavailable" (never empty) when
	// SupportingMarketCount == 0.
	FreshnessStatus       string
	P10, P50, P90         *float64
	SupportingMarketCount int
	ExcludedMarketCodes   []string
	// AggregationMethod is set only when Status == "ready".
	AggregationMethod  string
	QualityReasonCodes []string
}

// RegionalComparisonOutput is the usecase-level DTO the gRPC handler maps
// to the proto response.
type RegionalComparisonOutput struct {
	SelectionKind string // crop | group
	SelectionCode string
	HorizonDays   int
	Currency      string
	Unit          string
	// One entry per county in geography scope, always (issue #2249: a
	// county with no supporting market still appears, status
	// "unavailable" — never omitted, which would be indistinguishable
	// from "this county doesn't exist").
	Regions []RegionalComparisonRegion
	Lineage SpatialLineage
	// MappingVersion identifies the crop/group/region mapping policy
	// version (issue #2403 V.7.2) — distinct from Lineage.ModelVersion,
	// which is the forecast model's own version.
	MappingVersion string
}

// RegionalComparison computes issue #2249 Slice C's regional-comparison
// table entirely from the same two inputs SpatialSurface uses (the local
// model artifact and geography registry) via the shared marketResolver —
// see design doc docs/superpowers/specs/2026-08-08-pricebook-regional-comparison-design.md.
type RegionalComparison struct {
	resolver *marketResolver
}

// NewRegionalComparison mirrors NewSpatialSurface's constructor shape,
// minus the price-scale dependency — this usecase draws no map, so it
// has no legend to look up (design doc D3).
func NewRegionalComparison(artifact forecast.ModelArtifact, geo geography.Registry, forecaster seriesForecaster, groupUC *Forecast) *RegionalComparison {
	return &RegionalComparison{resolver: &marketResolver{artifact: artifact, geography: geo, forecast: forecaster, group: groupUC}}
}

// Get computes the comparison table. Its error contract mirrors
// SpatialSurface.Get exactly: only the three request-shape violations
// above return an error; every business outcome (unmodelled crop, a
// county with no supporting market) is Status/QualityReasonCodes on a
// successful call.
func (u *RegionalComparison) Get(ctx context.Context, in RegionalComparisonInput) (RegionalComparisonOutput, error) {
	if (in.CropCode == nil) == (in.GroupCode == nil) {
		return RegionalComparisonOutput{}, ErrRegionalComparisonExactlyOneSelection
	}
	if in.HorizonDays < 1 || in.HorizonDays > 7 {
		return RegionalComparisonOutput{}, ErrRegionalComparisonHorizonOutOfRange
	}
	if in.Now.IsZero() {
		return RegionalComparisonOutput{}, ErrRegionalComparisonZeroNow
	}

	res, err := u.resolver.resolveAndForecastMarkets(ctx, marketSelectionInput(in))
	if err != nil {
		return RegionalComparisonOutput{}, err
	}

	out := RegionalComparisonOutput{
		SelectionKind: res.SelectionKind, SelectionCode: res.SelectionCode,
		HorizonDays: in.HorizonDays, Currency: dataMetaCurrency, Unit: dataMetaUnit,
		Lineage:        res.Lineage,
		MappingVersion: CurrentMappingPolicyVersion,
	}
	globalReason := ""
	if len(res.MarketCodes) == 0 {
		globalReason = spatialReasonNoScoreableTradeRows
	}
	counties := u.resolver.geography.RegionsInScope()
	out.Regions = make([]RegionalComparisonRegion, 0, len(counties))
	for _, county := range counties {
		out.Regions = append(out.Regions, regionalComparisonRegion(county, res.Markets, res.MarketVolumes, globalReason))
	}
	return out, nil
}

// regionalComparisonRegion aggregates one county's rows out of the full
// resolved market list. Design doc D1: p10/p50/p90 are each market's own
// forecast uncertainty band, volume-weighted-averaged separately per
// quantile (quantile averaging via weightedValue — the same primitive
// aggregateMarkets already uses to blend a group's sub-variants within
// one market), NOT a spatial IDW interpolation and NOT a cross-market
// percentile of P50 point estimates. Design doc D2: 1 supporting market
// is enough for status=ready. Design doc D6: a market with no county at
// all (missing facility/coordinate) is filtered out by the caller before
// this function ever sees it going into ANY county's list — see how
// markets is passed in unfiltered here and this function itself checks
// m.hasCounty, so a market with no county contributes to no region's
// ExcludedMarketCodes either.
func regionalComparisonRegion(county geography.CountyRegion, markets []spatialMarket, volumes map[string]float64, globalReason string) RegionalComparisonRegion {
	region := RegionalComparisonRegion{RegionCode: county.CountyCode, RegionName: county.CountyName, Status: spatialStatusUnavailable}

	var p10s, p50s, p90s, weights []float64
	var regionFreshness forecast.FreshnessLabel
	for _, m := range markets {
		if !m.hasCounty || m.county.CountyCode != county.CountyCode {
			continue
		}
		if m.value == nil {
			region.ExcludedMarketCodes = append(region.ExcludedMarketCodes, m.marketCode)
			continue
		}
		// Same volume-weight fallback rule as weightedMedianP50
		// (spatial_surface.go): a market missing from volumes, or with a
		// non-positive entry, falls back to equal weight 1 rather than
		// being dropped.
		weight := 1.0
		if v, ok := volumes[m.marketCode]; ok && v > 0 {
			weight = v
		}
		p10s = append(p10s, *m.p10)
		p50s = append(p50s, *m.value)
		p90s = append(p90s, *m.p90)
		weights = append(weights, weight)
		regionFreshness = worstFreshness(regionFreshness, m.freshness)
	}

	region.SupportingMarketCount = len(p50s)
	if region.SupportingMarketCount == 0 {
		region.FreshnessStatus = string(forecast.FreshnessUnavailable)
		if globalReason != "" {
			region.QualityReasonCodes = []string{globalReason}
		} else {
			region.QualityReasonCodes = []string{regionalComparisonReasonNoMarketsInRegion}
		}
		return region
	}

	region.Status = spatialStatusReady
	region.P10 = weightedValue(p10s, weights)
	region.P50 = weightedValue(p50s, weights)
	region.P90 = weightedValue(p90s, weights)
	region.AggregationMethod = regionalComparisonAggregationMethod
	region.FreshnessStatus = string(regionFreshness)
	return region
}
