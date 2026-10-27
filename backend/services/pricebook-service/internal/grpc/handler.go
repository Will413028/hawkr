package grpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/paulmach/orb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// observedPriceUC is the narrow interface this handler needs — lets the
// handler test use a fake without importing the full usecase package.
// *usecase.ObservedPrice (Task 4) satisfies this.
type observedPriceUC interface {
	Get(ctx context.Context, cropName, marketCode, cropCode, groupCode string, tradeDate *time.Time, history bool, now time.Time) (resolved bool, observations []domain.ObservedPrice, meta usecase.DataMeta, err error)
}

// estimatedItemPriceUC is the narrow interface this handler needs for
// GetEstimatedItemPrice — same pattern as observedPriceUC.
// *usecase.EstimatedItemPrice (Task 3) satisfies this.
type estimatedItemPriceUC interface {
	Get(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error)
}

// forecastCurveUC is the narrow interface this handler needs for
// GetForecastCurve — same pattern as observedPriceUC/estimatedItemPriceUC.
// *usecase.ForecastCurve (Task 4) satisfies this.
type forecastCurveUC interface {
	Get(ctx context.Context, cropName, marketCode, cropCode, groupCode string, now time.Time) (usecase.ForecastCurveResult, error)
}

type commodityForecastUC interface {
	Handle(ctx context.Context, commodityCode string, cutoff time.Time) (usecase.CommodityForecastResult, error)
}

// searchCropCatalogUC is the narrow interface this handler needs for
// SearchCropCatalog and ListGroupCropOptions — same pattern as
// observedPriceUC. *usecase.SearchCropCatalog (Task 6) satisfies this; both
// RPCs read the same catalog through the same usecase, so they share one
// interface rather than each getting a provider of its own.
type searchCropCatalogUC interface {
	Get(ctx context.Context, query string, limit int32) ([]usecase.CatalogMatch, error)
	ListGroupCropOptions(ctx context.Context, groupCode string, limit int32) ([]usecase.CatalogMatch, error)
}

// searchPriceableEntitiesUC is the narrow interface this handler needs for
// SearchPriceableEntities — same pattern as searchCropCatalogUC.
type searchPriceableEntitiesUC interface {
	Get(ctx context.Context, variants []string, limit int32) ([]usecase.PriceableEntityMatch, error)
}

// trendSummaryUC is the narrow interface this handler needs for
// GetTrendSummary — same pattern as observedPriceUC.
// *usecase.TrendSummary (Task 8) satisfies this.
type trendSummaryUC interface {
	Get(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (usecase.TrendSummaryResult, error)
}

// spatialSurfaceUC is the narrow interface this handler needs for
// GetSpatialSurface — same pattern as observedPriceUC.
// *usecase.SpatialSurface (Task 9) satisfies this.
type spatialSurfaceUC interface {
	Get(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error)
}

// regionalComparisonUC is the narrow interface this handler needs for
// GetRegionalComparison — same pattern as spatialSurfaceUC.
// *usecase.RegionalComparison (issue #2249 Slice C) satisfies this.
type regionalComparisonUC interface {
	Get(ctx context.Context, in usecase.RegionalComparisonInput) (usecase.RegionalComparisonOutput, error)
}

// resolveCategoryMappingUC is the narrow interface this handler needs for
// ResolveCategoryMapping — same pattern as observedPriceUC.
// *usecase.ResolveCategoryMapping (issue #2786) satisfies this.
type resolveCategoryMappingUC interface {
	Get(ctx context.Context, categoryID string) (domain.CategoryMapping, error)
}

// upsertCategoryMappingUC is the narrow interface this handler needs for
// UpsertCategoryMapping — same pattern as observedPriceUC.
// *usecase.UpsertCategoryMapping (issue #2786) satisfies this.
type upsertCategoryMappingUC interface {
	Do(ctx context.Context, in usecase.UpsertCategoryMappingInput) (domain.CategoryMapping, error)
}

// listCategoryMappingWorklistUC is the narrow interface this handler needs
// for ListCategoryMappingWorklist — same pattern as observedPriceUC.
// *usecase.ListCategoryMappingWorklist (issue #2786) satisfies this.
type listCategoryMappingWorklistUC interface {
	Do(ctx context.Context) ([]domain.WorklistItem, error)
}

// getCategoryPriceRollupUC is the narrow interface this handler needs for
// GetCategoryPriceRollup — same pattern as resolveCategoryMappingUC.
// *usecase.GetCategoryPriceRollup (Task 10) satisfies this.
type getCategoryPriceRollupUC interface {
	Get(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error)
}

// PricebookServer implements the PricebookService gRPC service.
type PricebookServer struct {
	pb.UnimplementedPricebookServiceServer
	overviewUC                    marketOverviewUC
	watchUC                       marketWatchUC
	uc                            observedPriceUC
	estimatedUC                   estimatedItemPriceUC
	curveUC                       forecastCurveUC
	commodityForecastUC           commodityForecastUC
	catalogUC                     searchCropCatalogUC
	seed                          mapping.Seed
	priceableUC                   searchPriceableEntitiesUC
	trendUC                       trendSummaryUC
	spatialUC                     spatialSurfaceUC
	regionalUC                    regionalComparisonUC
	resolveCategoryMappingUC      resolveCategoryMappingUC
	upsertCategoryMappingUC       upsertCategoryMappingUC
	listCategoryMappingWorklistUC listCategoryMappingWorklistUC
	getCategoryPriceRollupUC      getCategoryPriceRollupUC
}

// NewPricebookServer creates a new PricebookServer. Accepts narrow
// interfaces for the usecases (so tests can pass fakes directly) and the
// concrete mapping.Seed value directly — a cheap, side-effect-free value
// type doesn't need an interface abstraction the way the DB/model-backed
// usecases do. seed is unused by PricebookServer itself: it's threaded
// through purely so fx has a concrete value to inject; SearchCropCatalog
// (catalogUC) owns and consumes its own copy for alias matching.
func NewPricebookServer(uc observedPriceUC, estimatedUC estimatedItemPriceUC, curveUC forecastCurveUC, catalogUC searchCropCatalogUC, seed mapping.Seed, trendUC trendSummaryUC, spatialUC spatialSurfaceUC, regionalUC regionalComparisonUC, resolveCategoryMappingUC resolveCategoryMappingUC, upsertCategoryMappingUC upsertCategoryMappingUC, listCategoryMappingWorklistUC listCategoryMappingWorklistUC, getCategoryPriceRollupUC getCategoryPriceRollupUC, commodityForecastUC commodityForecastUC, priceableUC searchPriceableEntitiesUC) *PricebookServer {
	return &PricebookServer{
		uc: uc, estimatedUC: estimatedUC, curveUC: curveUC, catalogUC: catalogUC, seed: seed, trendUC: trendUC, spatialUC: spatialUC, regionalUC: regionalUC,
		priceableUC:              priceableUC,
		commodityForecastUC:      commodityForecastUC,
		resolveCategoryMappingUC: resolveCategoryMappingUC, upsertCategoryMappingUC: upsertCategoryMappingUC, listCategoryMappingWorklistUC: listCategoryMappingWorklistUC,
		getCategoryPriceRollupUC: getCategoryPriceRollupUC,
	}
}

// validateCropResolutionRequest enforces the "exactly one of crop_name,
// crop_code, or group_code" caller contract eagerly, before touching any
// usecase (see repo-wide rule: every gRPC-layer caller-contract violation is
// InvalidArgument, checked eagerly). Mirrors usecase.validateResolutionInputs
// (unexported, so this handler can't call it directly) — GetObservedPrice
// and GetForecastCurve both apply it identically.
func validateCropResolutionRequest(cropName, cropCode, groupCode string) error {
	setCount := 0
	for _, v := range []string{cropName, cropCode, groupCode} {
		if v != "" {
			setCount++
		}
	}
	switch {
	case setCount > 1:
		return status.Error(codes.InvalidArgument, "exactly one of crop_name, crop_code, or group_code must be set")
	case setCount == 0:
		return status.Error(codes.InvalidArgument, "one of crop_name, crop_code, or group_code is required")
	default:
		return nil
	}
}

func (s *PricebookServer) GetObservedPrice(ctx context.Context, req *pb.GetObservedPriceRequest) (*pb.GetObservedPriceResponse, error) {
	if err := validateCropResolutionRequest(req.GetCropName(), req.GetCropCode(), req.GetGroupCode()); err != nil {
		return nil, err
	}

	var tradeDate *time.Time
	if req.TradeDate != nil {
		t, err := time.Parse("2006-01-02", req.GetTradeDate())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "trade_date must be YYYY-MM-DD: %v", err)
		}
		tradeDate = &t
	}

	_, observations, meta, err := s.uc.Get(ctx, req.GetCropName(), req.GetMarketCode(), req.GetCropCode(), req.GetGroupCode(), tradeDate, req.GetHistory(), time.Now())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrAmbiguousCropGroup):
			return nil, status.Error(codes.Internal, "crop name resolves to an ambiguous crop group — needs manual data cleanup")
		case errors.Is(err, usecase.ErrConflictingResolutionInputs), errors.Is(err, usecase.ErrNoResolutionInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrMarketCodeNotAllowedForGroup):
			return nil, status.Error(codes.InvalidArgument, "market_code must not be set when resolving to a crop group")
		case errors.Is(err, usecase.ErrMarketCodeRequiredForCrop):
			return nil, status.Error(codes.InvalidArgument, "market_code is required when resolving to a specific crop")
		default:
			return nil, status.Errorf(codes.Internal, "get observed price: %v", err)
		}
	}

	return &pb.GetObservedPriceResponse{Observations: toPriceObservations(observations), DataMeta: toPricebookDataMeta(meta)}, nil
}

func (s *PricebookServer) SearchCropCatalog(ctx context.Context, req *pb.SearchCropCatalogRequest) (*pb.SearchCropCatalogResponse, error) {
	matches, err := s.catalogUC.Get(ctx, req.GetQuery(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search crop catalog: %v", err)
	}
	return &pb.SearchCropCatalogResponse{Matches: toCatalogMatches(matches)}, nil
}

// SearchPriceableEntities: the only caller-contract violation is "no
// non-blank variant" (INVALID_ARGUMENT, checked eagerly by the usecase);
// no hits is an empty list, not an error. tier / rank cross the gRPC
// boundary as decimal strings (Proto 管理規範 §6).
func (s *PricebookServer) SearchPriceableEntities(ctx context.Context, req *pb.SearchPriceableEntitiesRequest) (*pb.SearchPriceableEntitiesResponse, error) {
	matches, err := s.priceableUC.Get(ctx, req.GetVariants(), req.GetLimit())
	if errors.Is(err, usecase.ErrNoQueryVariants) {
		return nil, status.Error(codes.InvalidArgument, "variants must contain at least one non-blank query")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search priceable entities: %v", err)
	}
	out := make([]*pb.PriceableEntityMatch, 0, len(matches))
	for _, m := range matches {
		out = append(out, &pb.PriceableEntityMatch{
			Kind: string(m.Kind), Code: m.Code, Name: m.Name, CategoryIds: m.CategoryIDs,
			MatchedForm: m.MatchedForm, FormKind: string(m.FormKind),
			Tier: strconv.FormatInt(int64(m.Tier), 10), Rank: strconv.FormatInt(int64(m.Rank), 10),
			HasForecast: m.HasForecast, GroupCode: m.GroupCode,
		})
	}
	return &pb.SearchPriceableEntitiesResponse{Matches: out}, nil
}

// ListGroupCropOptions has no INVALID_ARGUMENT branch on purpose: an empty
// or unknown group_code returns an empty list, matching the proto's own
// doc comment and the usecase's "resolution miss is a business outcome"
// stance. The only error surface is a genuine DB failure.
func (s *PricebookServer) ListGroupCropOptions(ctx context.Context, req *pb.ListGroupCropOptionsRequest) (*pb.ListGroupCropOptionsResponse, error) {
	crops, err := s.catalogUC.ListGroupCropOptions(ctx, req.GetGroupCode(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list group crop options: %v", err)
	}
	return &pb.ListGroupCropOptionsResponse{Crops: toCatalogMatches(crops)}, nil
}

func toCatalogMatches(matches []usecase.CatalogMatch) []*pb.CatalogMatch {
	out := make([]*pb.CatalogMatch, 0, len(matches))
	for _, m := range matches {
		out = append(out, &pb.CatalogMatch{
			MatchType: m.MatchType, GroupCode: m.GroupCode, GroupName: m.GroupName,
			CropCode: m.CropCode, CropName: m.CropName, VariantName: m.VariantName,
			MatchedAlias: m.MatchedAlias, HasForecast: m.HasForecast,
			Markets: toCropMarketOptions(m.Markets),
		})
	}
	return out
}

func toCropMarketOptions(markets []usecase.CatalogMarketOption) []*pb.CropMarketOption {
	out := make([]*pb.CropMarketOption, 0, len(markets))
	for _, mkt := range markets {
		out = append(out, &pb.CropMarketOption{MarketCode: mkt.MarketCode, MarketName: mkt.MarketName})
	}
	return out
}

func (s *PricebookServer) GetEstimatedItemPrice(ctx context.Context, req *pb.GetEstimatedItemPriceRequest) (*pb.GetEstimatedItemPriceResponse, error) {
	if req.GetCropName() == "" {
		return nil, status.Error(codes.InvalidArgument, "crop_name is required")
	}
	// !(x > 0) catches NaN (which fails every comparison, including <= 0)
	// and any x <= 0 in one comparison; the explicit IsInf check catches
	// +Inf (which *is* > 0). Without both, NaN/+Inf would reach the usecase
	// and produce a status: ready response with a garbage number
	// (int64(NaN) is implementation-defined; int64(+Inf * price) overflows
	// to math.MaxInt64) — a fabricated price for a request that should have
	// been rejected outright.
	if !(req.GetNetWeightKg() > 0) || math.IsInf(req.GetNetWeightKg(), 0) {
		return nil, status.Error(codes.InvalidArgument, "net_weight_kg must be positive")
	}

	result, err := s.estimatedUC.Get(ctx, req.GetCropName(), req.GetNetWeightKg(), req.GetMarketCode(), time.Now())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrAmbiguousCropGroup):
			return nil, status.Error(codes.Internal, "crop name resolves to an ambiguous crop group — needs manual data cleanup")
		case errors.Is(err, usecase.ErrMarketCodeNotAllowedForGroup):
			return nil, status.Error(codes.InvalidArgument, "market_code must not be set when crop_name resolves to a crop group")
		case errors.Is(err, usecase.ErrMarketCodeRequiredForCrop):
			return nil, status.Error(codes.InvalidArgument, "market_code is required when crop_name resolves to a specific crop")
		default:
			return nil, status.Errorf(codes.Internal, "get estimated item price: %v", err)
		}
	}

	return toEstimatedItemPriceResponse(result), nil
}

func toEstimatedItemPriceResponse(r usecase.EstimatedItemPriceResult) *pb.GetEstimatedItemPriceResponse {
	return &pb.GetEstimatedItemPriceResponse{
		Status:                string(r.Status),
		Low:                   r.Low,
		Median:                r.Median,
		High:                  r.High,
		ResolvedCropGroupCode: r.ResolvedCropGroupCode,
		ResolvedCropCode:      r.ResolvedCropCode,
		MarketCode:            r.MarketCode,
		ModelVersion:          r.ModelVersion,
	}
}

func (s *PricebookServer) GetForecastCurve(ctx context.Context, req *pb.GetForecastCurveRequest) (*pb.GetForecastCurveResponse, error) {
	if req.CommodityCode != nil {
		if req.GetCropName() != "" || req.CropCode != nil || req.GroupCode != nil {
			return nil, status.Error(codes.InvalidArgument,
				"commodity_code must not be combined with crop_name / crop_code / group_code")
		}
		return s.commodityForecastCurve(ctx, req.GetCommodityCode())
	}
	if err := validateCropResolutionRequest(req.GetCropName(), req.GetCropCode(), req.GetGroupCode()); err != nil {
		return nil, err
	}

	result, err := s.curveUC.Get(ctx, req.GetCropName(), req.GetMarketCode(), req.GetCropCode(), req.GetGroupCode(), time.Now())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrAmbiguousCropGroup):
			return nil, status.Error(codes.Internal, "crop name resolves to an ambiguous crop group — needs manual data cleanup")
		case errors.Is(err, usecase.ErrConflictingResolutionInputs), errors.Is(err, usecase.ErrNoResolutionInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrMarketCodeNotAllowedForGroup):
			return nil, status.Error(codes.InvalidArgument, "market_code must not be set when resolving to a crop group")
		case errors.Is(err, usecase.ErrMarketCodeRequiredForCrop):
			return nil, status.Error(codes.InvalidArgument, "market_code is required when resolving to a specific crop")
		default:
			return nil, status.Errorf(codes.Internal, "get forecast curve: %v", err)
		}
	}

	return toForecastCurveResponse(result), nil
}

func (s *PricebookServer) commodityForecastCurve(ctx context.Context, code string) (*pb.GetForecastCurveResponse, error) {
	r, err := s.commodityForecastUC.Handle(ctx, code, time.Now())
	if err != nil {
		if errors.Is(err, usecase.ErrCommodityNotFound) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get commodity forecast: %v", err)
	}

	resp := &pb.GetForecastCurveResponse{
		Status:       string(r.Status),
		Capabilities: commodityCapabilities(r),
	}
	if r.P50 == nil {
		return resp, nil
	}
	derivation := r.Derivation
	resp.Points = []*pb.ForecastCurvePoint{{
		TargetDate:  r.TargetDate.Format("2006-01-02"),
		HorizonDays: 1,
		Status:      string(r.Status),
		P50:         r.P50,
		Derivation:  &derivation,
	}}
	resp.PriceLayer = proto.String(string(r.PriceLayer))
	resp.Unit = proto.String(r.Unit)
	if r.CanonicalUnit != "" {
		resp.CanonicalUnit = proto.String(r.CanonicalUnit)
	}
	if r.OriginalP50 != nil {
		resp.OriginalP50 = proto.String(strconv.FormatFloat(*r.OriginalP50, 'f', 4, 64))
	}
	resp.SourceDataAsOf = proto.String(r.SourceDate.Format("2006-01-02"))
	resp.SupportingMarketCount = proto.String(strconv.Itoa(r.MarketCount))
	if r.TotalQuantity != nil {
		resp.TotalQuantity = proto.String(strconv.FormatFloat(*r.TotalQuantity, 'f', 3, 64))
		resp.QuantityUnit = proto.String(r.QuantityUnit)
	}
	return resp, nil
}

func cropSeriesCapabilities() *pb.SeriesCapabilities {
	return &pb.SeriesCapabilities{MarketComparison: true, Spatial: true}
}

func commodityCapabilities(r usecase.CommodityForecastResult) *pb.SeriesCapabilities {
	caps := &pb.SeriesCapabilities{}
	if r.MarketCount > 1 {
		caps.ReasonCodes = append(caps.ReasonCodes, "SINGLE_SERIES_ONLY")
	} else {
		caps.ReasonCodes = append(caps.ReasonCodes, "NATIONAL_SERIES")
	}
	caps.ReasonCodes = append(caps.ReasonCodes, "NO_APPROVED_GEOGRAPHY")
	return caps
}

func toForecastCurveResponse(r usecase.ForecastCurveResult) *pb.GetForecastCurveResponse {
	return &pb.GetForecastCurveResponse{
		Capabilities:          cropSeriesCapabilities(),
		Status:                string(r.Status),
		ResolvedCropGroupCode: r.ResolvedCropGroupCode,
		ResolvedCropCode:      r.ResolvedCropCode,
		MarketCode:            r.MarketCode,
		ModelVersion:          r.ModelVersion,
		Points:                toForecastCurvePoints(r.Points),
		MarketComparison:      toMarketComparisonEntries(r.MarketComparison),
		DataMeta:              toPricebookDataMeta(r.DataMeta),
		Aggregation:           toForecastAggregationMeta(r.Aggregation),
	}
}

func toForecastAggregationMeta(meta *usecase.ForecastAggregationMeta) *pb.ForecastAggregationMeta {
	if meta == nil {
		return nil
	}
	exclusions := make([]*pb.ForecastMarketExclusion, 0, len(meta.ExcludedMarkets))
	for _, excluded := range meta.ExcludedMarkets {
		exclusions = append(exclusions, &pb.ForecastMarketExclusion{MarketCode: excluded.MarketCode, Reason: excluded.Reason})
	}
	return &pb.ForecastAggregationMeta{
		Method: meta.Method, WeightWindowDays: strconv.Itoa(meta.WeightWindowDays),
		ParticipatingMarketCodes: meta.ParticipatingMarketCodes, ExcludedMarkets: exclusions,
	}
}

// toPricebookDataMeta maps the usecase-layer DataMeta DTO to its proto
// shape — pure field mapping, no business logic (that all lives in
// usecase.DataMeta's builders, see internal/usecase/datameta.go). Times are
// formatted UTC "...Z" (time.RFC3339 on a .UTC()-normalized time.Time),
// per this repo's time-format redline — never the issue's example "+08:00"
// (that was illustrative, not the wire contract).
func toPricebookDataMeta(m usecase.DataMeta) *pb.PricebookDataMeta {
	out := &pb.PricebookDataMeta{
		SnapshotId:         m.SnapshotID,
		Source:             m.Source,
		GeneratedAt:        timeutil.FormatISO8601(m.GeneratedAt),
		FreshnessStatus:    string(m.FreshnessStatus),
		UsabilityStatus:    string(m.UsabilityStatus),
		QualityReasonCodes: toQualityReasonCodeStrings(m.QualityReasonCodes),
		ModelVersion:       m.ModelVersion,
		DatasetDigest:      m.DatasetDigest,
		SchemaVersion:      m.SchemaVersion,
		Currency:           m.Currency,
		Unit:               m.Unit,
	}
	if m.SourceDataAsOf != nil {
		s := timeutil.FormatISO8601(*m.SourceDataAsOf)
		out.SourceDataAsOf = &s
	}
	if m.MaxFeatureAvailableAt != nil {
		s := timeutil.FormatISO8601(*m.MaxFeatureAvailableAt)
		out.MaxFeatureAvailableAt = &s
	}
	return out
}

func toQualityReasonCodeStrings(codes []forecast.QualityReasonCode) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		out = append(out, string(c))
	}
	return out
}

func toMarketComparisonEntries(entries []usecase.MarketComparisonEntry) []*pb.MarketComparisonEntry {
	out := make([]*pb.MarketComparisonEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, &pb.MarketComparisonEntry{MarketCode: e.MarketCode, MarketName: e.MarketName, P50: e.P50, IsApproximate: e.IsApproximate, Status: string(e.Status)})
	}
	return out
}

// validGranularities/validComparisonKinds are the caller-facing string
// vocabularies GetTrendSummary accepts on the wire, mapped to their
// forecast.Granularity/forecast.TrendComparisonKind domain values. Any
// request string not present in these maps is a caller-contract violation
// — InvalidArgument, checked eagerly, same repo-wide rule
// validateCropResolutionRequest documents above. forecast.PeriodsInRange
// panics on any Granularity other than the two constants below, so this
// eager check is also what keeps that call safe.
var validGranularities = map[string]forecast.Granularity{"day": forecast.GranularityDay, "week": forecast.GranularityWeek}
var validComparisonKinds = map[string]forecast.TrendComparisonKind{"wow": forecast.ComparisonWoW, "yoy364": forecast.ComparisonYoY364}

// trendComparisonKindOrder fixes the wire order toTrendSummaryResponse
// emits pb.GetTrendSummaryResponse.Comparisons entries in — Comparisons is
// a repeated (i.e. positionally ordered) proto field, but
// usecase.TrendSummaryResult.Comparisons is a Go map, whose iteration order
// is randomized per-run. Always wow before yoy364, regardless of the order
// the caller listed them in the request — deterministic for any caller
// input, not just deterministic-per-request.
var trendComparisonKindOrder = []forecast.TrendComparisonKind{forecast.ComparisonWoW, forecast.ComparisonYoY364}

// GetTrendSummary resolves a direct crop_code or group_code trend series
// (issue #2249 Slice B). Unlike GetObservedPrice/GetForecastCurve there is
// no crop_name resolution path, so validateCropResolutionRequest is called
// with cropName="" — it still enforces "exactly one of crop_code/group_code"
// via its setCount logic. market_code required-for-crop /
// not-allowed-for-group is deliberately NOT eagerly re-checked here: like
// GetObservedPrice/GetForecastCurve, that's left solely to
// usecase.TrendSummary.Get's own resolution + the error switch below
// (ErrMarketCodeRequiredForCrop/ErrMarketCodeNotAllowedForGroup) — adding a
// second, earlier check here would duplicate that business rule at this
// layer instead of the single source of truth the sibling handlers use.
func (s *PricebookServer) GetTrendSummary(ctx context.Context, req *pb.GetTrendSummaryRequest) (*pb.GetTrendSummaryResponse, error) {
	if err := validateCropResolutionRequest("", req.GetCropCode(), req.GetGroupCode()); err != nil {
		return nil, err
	}

	from, err := time.Parse("2006-01-02", req.GetFrom())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "from must be YYYY-MM-DD: %v", err)
	}
	to, err := time.Parse("2006-01-02", req.GetTo())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "to must be YYYY-MM-DD: %v", err)
	}
	if to.Before(from) {
		return nil, status.Error(codes.InvalidArgument, "to must not be before from")
	}
	if to.Sub(from) > 366*24*time.Hour {
		return nil, status.Error(codes.InvalidArgument, "from..to must not exceed 366 days")
	}

	granularity, ok := validGranularities[req.GetGranularity()]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "granularity must be one of day, week; got %q", req.GetGranularity())
	}

	comparisons := make([]forecast.TrendComparisonKind, 0, len(req.GetComparisons()))
	for _, c := range req.GetComparisons() {
		kind, ok := validComparisonKinds[c]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "comparisons must be a subset of wow, yoy364; got %q", c)
		}
		comparisons = append(comparisons, kind)
	}

	result, err := s.trendUC.Get(ctx, req.GetCropCode(), req.GetGroupCode(), req.GetMarketCode(),
		forecast.DateFromTime(from), forecast.DateFromTime(to), granularity, comparisons, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrConflictingResolutionInputs), errors.Is(err, usecase.ErrNoResolutionInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrMarketCodeNotAllowedForGroup):
			return nil, status.Error(codes.InvalidArgument, "market_code must not be set when resolving to a crop group")
		case errors.Is(err, usecase.ErrMarketCodeRequiredForCrop):
			return nil, status.Error(codes.InvalidArgument, "market_code is required when resolving to a specific crop")
		default:
			return nil, status.Errorf(codes.Internal, "get trend summary: %v", err)
		}
	}

	return toTrendSummaryResponse(result), nil
}

func toTrendSummaryResponse(r usecase.TrendSummaryResult) *pb.GetTrendSummaryResponse {
	points := make([]*pb.TrendPoint, 0, len(r.Points))
	for _, p := range r.Points {
		points = append(points, &pb.TrendPoint{
			PeriodStart: p.Bounds.Start.String(), PeriodEnd: p.Bounds.End.String(),
			Status: string(p.Status), ObservedAverageTwdPerKg: p.Value, VolumeKg: p.Volume,
			SampleCount: int32(p.SampleCount), ObservedMarketCount: int32(p.ObservedMarketCount),
			AggregationMethod: p.AggregationMethod,
		})
	}
	// r.Comparisons is a map — ranging it directly would make Comparisons'
	// wire order (a repeated, i.e. ordered, proto field) nondeterministic
	// across calls whenever both wow and yoy364 are present, which would
	// flake any positional/golden-fixture comparison downstream (Task
	// 11/12). Iterate trendComparisonKindOrder instead: a fixed constant
	// order, deterministic regardless of what order the caller listed
	// comparisons in on the request.
	comparisons := make([]*pb.TrendComparisonEntry, 0, len(r.Comparisons))
	for _, kind := range trendComparisonKindOrder {
		c, ok := r.Comparisons[kind]
		if !ok {
			continue
		}
		comparisons = append(comparisons, &pb.TrendComparisonEntry{
			Kind: string(kind), Status: string(c.Status), CurrentValueTwdPerKg: c.CurrentValue,
			BaselineValueTwdPerKg: c.BaselineValue, ChangeAmountTwdPerKg: c.ChangeAmount,
			ChangePercent: c.ChangePercent, ReasonCode: c.ReasonCode,
		})
	}
	granularity := "day"
	if r.Granularity == forecast.GranularityWeek {
		granularity = "week"
	}
	return &pb.GetTrendSummaryResponse{
		SelectionKind: r.SelectionKind, Granularity: granularity, Timezone: "Asia/Taipei",
		Points: points, Comparisons: comparisons, DataMeta: toPricebookDataMeta(r.DataMeta),
	}
}

func toForecastCurvePoints(points []usecase.CurvePoint) []*pb.ForecastCurvePoint {
	out := make([]*pb.ForecastCurvePoint, 0, len(points))
	for _, p := range points {
		out = append(out, &pb.ForecastCurvePoint{
			TargetDate:  p.TargetDate.String(),
			HorizonDays: int32(p.HorizonDays),
			Status:      string(p.Status),
			P10:         p.P10,
			P50:         p.P50,
			P90:         p.P90,
			Aggregation: toForecastAggregationMeta(p.Aggregation),
		})
	}
	return out
}

// spatialAggregationMethod names the method boundary_price_geojson's
// estimatedP50TwdPerKg is actually computed with. spatial.RollupByCounty
// averages the surface cells whose centroid falls in the county, and those
// cells all come from one uniform grid, so an unweighted mean *is* the
// area-weighted result — issue #2248 requires this label to be the true
// method name, not an aspirational one.
const spatialAggregationMethod = "surface_cell_area_weighted/v1"

// GetSpatialSurface returns the nationwide price surface for a crop or
// crop group (issue #2248). Only the two request-shape violations are
// gRPC errors; everything else the usecase reports as
// status/quality_reason_codes on a successful call, same philosophy as
// every sibling RPC here. Like GetTrendSummary there is no crop_name
// resolution path, so validateCropResolutionRequest is called with
// cropName="" and still enforces "exactly one of crop_code/group_code".
//
// horizon_days IS eagerly re-checked here even though the usecase checks
// it too: unlike the market_code rules (a business rule that must live in
// exactly one place), 1-7 is a caller-contract constant written into the
// proto comment and the REST contract, so the gRPC layer owes callers the
// same eager InvalidArgument every other request-shape violation gets.
// The usecase's own check stays as the last line of defence for
// non-gRPC callers.
func (s *PricebookServer) GetSpatialSurface(ctx context.Context, req *pb.GetSpatialSurfaceRequest) (*pb.GetSpatialSurfaceResponse, error) {
	if err := validateCropResolutionRequest("", req.GetCropCode(), req.GetGroupCode()); err != nil {
		return nil, err
	}
	if req.GetHorizonDays() < 1 || req.GetHorizonDays() > 7 {
		return nil, status.Errorf(codes.InvalidArgument, "horizon_days must be between 1 and 7; got %d", req.GetHorizonDays())
	}

	// Built from the validated non-empty values rather than aliasing req's
	// own pointers: validateCropResolutionRequest treats a set-but-empty
	// crop_code as unset, and passing a non-nil pointer to "" would send
	// the usecase down the crop path with no crop.
	in := usecase.SpatialSurfaceInput{HorizonDays: int(req.GetHorizonDays()), Now: time.Now()}
	if code := req.GetCropCode(); code != "" {
		in.CropCode = &code
	}
	if code := req.GetGroupCode(); code != "" {
		in.GroupCode = &code
	}

	out, err := s.spatialUC.Get(ctx, in)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrSpatialExactlyOneSelection), errors.Is(err, usecase.ErrSpatialHorizonOutOfRange):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "get spatial surface: %v", err)
		}
	}

	resp, err := toSpatialSurfaceResponse(out)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build spatial surface response: %v", err)
	}
	return resp, nil
}

func toSpatialSurfaceResponse(out usecase.SpatialSurfaceOutput) (*pb.GetSpatialSurfaceResponse, error) {
	boundary, err := toBoundaryPriceGeojson(out.CountyRollups)
	if err != nil {
		return nil, err
	}
	rawBoundary, err := toSpatialBoundary(out.Boundary)
	if err != nil {
		return nil, err
	}
	points, err := toMarketPointsGeojson(out.MarketPrices)
	if err != nil {
		return nil, err
	}
	cells, err := toSurfaceCellsGeojson(out.SurfaceCells)
	if err != nil {
		return nil, err
	}
	contours, err := toContourLinesGeojson(out.ContourSegments)
	if err != nil {
		return nil, err
	}
	return &pb.GetSpatialSurfaceResponse{
		Status:                 out.Status,
		DisplayMode:            out.DisplayMode,
		SelectionKind:          out.SelectionKind,
		TargetDate:             out.TargetDate.String(),
		HorizonDays:            int32(out.HorizonDays),
		Currency:               out.Currency,
		Unit:                   out.Unit,
		Scale:                  toPriceScaleInfo(out.Scale),
		BoundaryPriceGeojson:   boundary,
		MarketPointsGeojson:    points,
		SurfaceCellsGeojson:    cells,
		ContourLinesGeojson:    contours,
		MarketPrices:           toSpatialMarketPrices(out.MarketPrices),
		Statistics:             toSpatialStatistics(out.Statistics),
		QualityReasonCodes:     out.QualityReasonCodes,
		Disclosures:            out.Disclosures,
		ExcludedMarketCodes:    out.ExcludedMarketCodes,
		Lineage:                toSpatialLineage(out.Lineage),
		Boundary:               rawBoundary,
		SupportedLandAreaRatio: out.SupportedLandAreaRatio,
	}, nil
}

func toPriceScaleInfo(s spatial.PriceScale) *pb.PriceScaleInfo {
	return &pb.PriceScaleInfo{ScaleId: s.ScaleID, PolicyVersion: s.PolicyVersion, Breaks: s.Breaks}
}

// toSpatialMarketPrices carries every market in scope, including the ones
// excluded from the surface — this is the table the caller falls back to
// when display_mode is table_only, and the audit trail behind
// excluded_market_codes. Price fields stay unset rather than zero-filled
// on a market that produced no forecast.
func toSpatialMarketPrices(prices []usecase.SpatialMarketPrice) []*pb.SpatialMarketPrice {
	out := make([]*pb.SpatialMarketPrice, 0, len(prices))
	for _, p := range prices {
		mp := &pb.SpatialMarketPrice{
			MarketCode: p.MarketCode, MarketName: p.MarketName,
			PredictionStatus: p.PredictionStatus, MarketStatus: p.MarketStatus,
			UsedForSurface: p.UsedForSurface,
			P10:            p.P10, P50: p.P50, P90: p.P90, RecentVolumeKg: p.RecentVolumeKg,
		}
		if p.CategoryCode != "" {
			categoryCode := p.CategoryCode
			mp.CategoryCode = &categoryCode
		}
		if p.ExclusionReason != "" {
			exclusionReason := p.ExclusionReason
			mp.ExclusionReason = &exclusionReason
		}
		out = append(out, mp)
	}
	return out
}

func toSpatialStatistics(s usecase.SpatialStatistics) *pb.SpatialStatistics {
	return &pb.SpatialStatistics{
		PricedMarketCount:   int32(s.PricedMarketCount),
		ClosedMarketCount:   int32(s.ClosedMarketCount),
		TotalMarketCount:    int32(s.TotalMarketCount),
		SurfaceCellCount:    int32(s.SurfaceCellCount),
		ContourSegmentCount: int32(s.ContourSegmentCount),
		P50:                 s.P50,
		MinimumP50:          s.MinimumP50,
		MaximumP50:          s.MaximumP50,
	}
}

// toSpatialLineage formats times as RFC3339 UTC "...Z" on a
// .UTC()-normalized time.Time, per this repo's time-format redline — the
// same rule toPricebookDataMeta follows. Every optional field here —
// generated_at, schema_version, source_data_as_of, max_feature_available_at
// — stays unset when the usecase left it zero-valued; formatting a zero
// time.Time into "0001-01-01T00:00:00Z" or emitting an empty-string
// schema_version would both be a fabricated provenance claim.
func toSpatialLineage(l usecase.SpatialLineage) *pb.SpatialLineage {
	out := &pb.SpatialLineage{
		ArtifactId:     l.ArtifactID,
		ModelVersion:   l.ModelVersion,
		DatasetDigest:  l.DatasetDigest,
		BoundaryDigest: l.BoundaryDigest,
	}
	if !l.GeneratedAt.IsZero() {
		generatedAt := timeutil.FormatISO8601(l.GeneratedAt)
		out.GeneratedAt = &generatedAt
	}
	if l.SchemaVersion != "" {
		schemaVersion := l.SchemaVersion
		out.SchemaVersion = &schemaVersion
	}
	if l.SourceDataAsOf != nil {
		s := timeutil.FormatISO8601(*l.SourceDataAsOf)
		out.SourceDataAsOf = &s
	}
	if l.MaxFeatureAvailableAt != nil {
		s := timeutil.FormatISO8601(*l.MaxFeatureAvailableAt)
		out.MaxFeatureAvailableAt = &s
	}
	return out
}

// GetRegionalComparison returns a per-county price comparison table for a
// crop or crop group at a single horizon (issue #2249 Slice C). Same
// request-shape-only-errors philosophy as GetSpatialSurface.
func (s *PricebookServer) GetRegionalComparison(ctx context.Context, req *pb.GetRegionalComparisonRequest) (*pb.GetRegionalComparisonResponse, error) {
	if err := validateCropResolutionRequest("", req.GetCropCode(), req.GetGroupCode()); err != nil {
		return nil, err
	}
	if req.GetHorizonDays() < 1 || req.GetHorizonDays() > 7 {
		return nil, status.Errorf(codes.InvalidArgument, "horizon_days must be between 1 and 7; got %d", req.GetHorizonDays())
	}

	in := usecase.RegionalComparisonInput{HorizonDays: int(req.GetHorizonDays()), Now: time.Now()}
	if code := req.GetCropCode(); code != "" {
		in.CropCode = &code
	}
	if code := req.GetGroupCode(); code != "" {
		in.GroupCode = &code
	}

	out, err := s.regionalUC.Get(ctx, in)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrRegionalComparisonExactlyOneSelection), errors.Is(err, usecase.ErrRegionalComparisonHorizonOutOfRange):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "get regional comparison: %v", err)
		}
	}
	return toRegionalComparisonResponse(out), nil
}

func toRegionalComparisonResponse(out usecase.RegionalComparisonOutput) *pb.GetRegionalComparisonResponse {
	regions := make([]*pb.RegionalComparisonRegion, 0, len(out.Regions))
	for _, r := range out.Regions {
		region := &pb.RegionalComparisonRegion{
			RegionCode: r.RegionCode, RegionName: r.RegionName, Status: r.Status,
			P10: r.P10, P50: r.P50, P90: r.P90,
			SupportingMarketCount: int32(r.SupportingMarketCount),
			ExcludedMarketCodes:   r.ExcludedMarketCodes,
			QualityReasonCodes:    r.QualityReasonCodes,
			FreshnessStatus:       r.FreshnessStatus,
		}
		if r.AggregationMethod != "" {
			method := r.AggregationMethod
			region.AggregationMethod = &method
		}
		regions = append(regions, region)
	}
	return &pb.GetRegionalComparisonResponse{
		SelectionKind: out.SelectionKind, HorizonDays: int32(out.HorizonDays),
		Currency: out.Currency, Unit: out.Unit, Regions: regions,
		Lineage:        toSpatialLineage(out.Lineage),
		MappingVersion: out.MappingVersion,
	}
}

// ResolveCategoryMapping resolves a single categoryId to its authoritative
// cropCode/groupCode mapping (issue #2786). Every business outcome (mapped,
// unmapped, ambiguous, or the target crop/group no longer active) comes
// back via `status`/fields on a successful response, never as a gRPC
// error — same philosophy as GetObservedPrice/GetForecastCurve. There is no
// caller-contract violation to check eagerly here: category_id is a plain
// required string, no "exactly one of" constraint like the crop-resolution
// RPCs.
func (s *PricebookServer) ResolveCategoryMapping(ctx context.Context, req *pb.ResolveCategoryMappingRequest) (*pb.ResolveCategoryMappingResponse, error) {
	m, err := s.resolveCategoryMappingUC.Get(ctx, req.GetCategoryId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve category mapping: %v", err)
	}
	resp := &pb.ResolveCategoryMappingResponse{
		CategoryId:     m.CategoryID,
		Status:         string(m.Status),
		MappingVersion: m.MappingVersion,
		GroupCode:      m.GroupCode,
		Label:          m.Label,
	}
	if m.CommodityID != nil {
		resp.CommodityCode = proto.String(strconv.FormatInt(*m.CommodityID, 10))
	}
	for _, v := range m.CropVarieties {
		resp.CropVarieties = append(resp.CropVarieties, &pb.CategoryCropVariety{
			CropCode: v.CropCode, Name: v.Name, VariantName: v.VariantName,
		})
	}
	return resp, nil
}

// GetCategoryPriceRollup serves category_price_rollup for any tree level.
// Every business outcome (ready with a price, or unavailable) comes back
// via `status` on a successful response — same philosophy as
// ResolveCategoryMapping and GetObservedPrice. No row yet (batch hasn't
// run for this category_id) is folded into the same unavailable shape.
func (s *PricebookServer) GetCategoryPriceRollup(ctx context.Context, req *pb.GetCategoryPriceRollupRequest) (*pb.GetCategoryPriceRollupResponse, error) {
	row, err := s.getCategoryPriceRollupUC.Get(ctx, req.GetCategoryId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get category price rollup: %v", err)
	}
	if row == nil {
		return &pb.GetCategoryPriceRollupResponse{CategoryId: req.GetCategoryId(), Status: string(domain.RollupStatusUnavailable)}, nil
	}
	resp := &pb.GetCategoryPriceRollupResponse{
		CategoryId:       row.CategoryID,
		Level:            int32(row.Level),
		Status:           string(row.Status),
		SourceGroupCount: int32(row.SourceGroupCount),
		ComputedAt:       row.ComputedAt.UTC().Format(time.RFC3339),
	}
	if row.PriceLayer != nil {
		resp.PriceLayer = proto.String(string(*row.PriceLayer))
	}
	resp.Unit = row.Unit
	if row.LatestPrice != nil {
		v := strconv.FormatFloat(*row.LatestPrice, 'f', 4, 64)
		resp.LatestPrice = &v
	}
	if row.PrevPrice != nil {
		v := strconv.FormatFloat(*row.PrevPrice, 'f', 4, 64)
		resp.PrevPrice = &v
	}
	for _, p := range row.Points {
		resp.Points = append(resp.Points, &pb.CategoryPriceRollupPoint{
			TradeDate: p.TradeDate, AvgPrice: strconv.FormatFloat(p.AvgPrice, 'f', 4, 64),
		})
	}
	for _, p := range row.ForecastPoints {
		fp := &pb.CategoryForecastPoint{
			HorizonDays: int32(p.HorizonDays), TargetDate: p.TargetDate,
		}
		if p.P10 != nil {
			v := strconv.FormatFloat(*p.P10, 'f', 4, 64)
			fp.P10 = &v
		}
		if p.P50 != nil {
			v := strconv.FormatFloat(*p.P50, 'f', 4, 64)
			fp.P50 = &v
		}
		if p.P90 != nil {
			v := strconv.FormatFloat(*p.P90, 'f', 4, 64)
			fp.P90 = &v
		}
		resp.ForecastPoints = append(resp.ForecastPoints, fp)
	}
	return resp, nil
}

// UpsertCategoryMapping writes an admin-curated categoryId→cropCode/groupCode
// mapping (issue #2786). Unlike the read-side RPCs above, the three usecase
// sentinel errors here ARE caller-contract violations (an admin trying to
// map a categoryId that doesn't exist/isn't ACTIVE, or to a target
// crop/group code that isn't active) and so are mapped to distinct gRPC
// status codes rather than folded into a business "status" field on a
// successful response — there is no successful shape for "the write was
// rejected".
func (s *PricebookServer) UpsertCategoryMapping(ctx context.Context, req *pb.UpsertCategoryMappingRequest) (*pb.UpsertCategoryMappingResponse, error) {
	in := usecase.UpsertCategoryMappingInput{
		CategoryID: req.GetCategoryId(), Status: domain.CategoryMappingStatus(req.GetStatus()),
		GroupCode: req.GroupCode, SourceNote: req.SourceNote, UpdatedBy: req.GetUpdatedBy(),
	}
	m, err := s.upsertCategoryMappingUC.Do(ctx, in)
	switch {
	case errors.Is(err, usecase.ErrCategoryNotFound):
		return nil, status.Error(codes.NotFound, "category not found")
	case errors.Is(err, usecase.ErrCategoryInactive):
		return nil, status.Error(codes.FailedPrecondition, "category is not ACTIVE")
	case errors.Is(err, usecase.ErrTargetCodeInactive):
		return nil, status.Error(codes.InvalidArgument, "target crop/group code is not active")
	case errors.Is(err, usecase.ErrIncompleteSelection):
		return nil, status.Error(codes.InvalidArgument, "selection fields incomplete or inconsistent with status")
	case err != nil:
		return nil, status.Errorf(codes.Internal, "upsert category mapping: %v", err)
	}
	return &pb.UpsertCategoryMappingResponse{
		CategoryId: m.CategoryID, Status: string(m.Status),
		MappingVersion: m.MappingVersion, UpdatedAt: m.UpdatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// ListCategoryMappingWorklist returns the admin pending-review list (issue
// #2786): unmapped/ambiguous existing rows plus categoryIds with no row at
// all yet (see usecase.ListCategoryMappingWorklist's own doc comment).
// updated_by is converted from the domain layer's *int64 to the proto
// response's *string here — the proto/Gateway JSON boundary is the only
// place that conversion belongs; domain.WorklistItem.UpdatedBy stays *int64
// internally (see api/proto/pricebook/pricebook.proto's
// CategoryMappingWorklistItem.updated_by comment for why the wire type is
// string).
func (s *PricebookServer) ListCategoryMappingWorklist(ctx context.Context, req *pb.ListCategoryMappingWorklistRequest) (*pb.ListCategoryMappingWorklistResponse, error) {
	// caller-contract 檢查照 repo 慣例 eager 做（見 validateCropResolutionRequest）：
	// proto 已寫明 status 只能是空字串｜unmapped｜ambiguous，其他值（含
	// "resolved"）過去會靜默回空清單，讓呼叫端誤以為「真的沒有待處理項目」。
	switch req.GetStatus() {
	case "", string(domain.CategoryMappingUnmapped), string(domain.CategoryMappingAmbiguous):
	default:
		return nil, status.Error(codes.InvalidArgument, "status must be empty, unmapped, or ambiguous")
	}
	items, err := s.listCategoryMappingWorklistUC.Do(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list category mapping worklist: %v", err)
	}
	filtered := items
	if req.GetStatus() != "" {
		filtered = make([]domain.WorklistItem, 0, len(items))
		for _, it := range items {
			if string(it.Status) == req.GetStatus() {
				filtered = append(filtered, it)
			}
		}
	}
	out := make([]*pb.CategoryMappingWorklistItem, 0, len(filtered))
	for _, it := range filtered {
		pbItem := &pb.CategoryMappingWorklistItem{
			CategoryId: it.CategoryID, CategoryName: it.CategoryName, Status: string(it.Status),
		}
		if it.UpdatedBy != nil {
			v := strconv.FormatInt(*it.UpdatedBy, 10)
			pbItem.UpdatedBy = &v
		}
		if it.UpdatedAt != nil {
			v := it.UpdatedAt.UTC().Format(time.RFC3339)
			pbItem.UpdatedAt = &v
		}
		out = append(out, pbItem)
	}
	return &pb.ListCategoryMappingWorklistResponse{Items: out}, nil
}

// toSpatialBoundary builds the raw county geometry layer — same source
// geometry as toBoundaryPriceGeojson's per-county rollups, without the
// price properties. Every field comes from geography.Registry, so it is
// populated on every response regardless of this request's own outcome
// (usecase.finish sets SpatialSurfaceOutput.Boundary unconditionally).
func toSpatialBoundary(b usecase.SpatialBoundaryOutput) (*pb.SpatialBoundary, error) {
	features := make([]any, 0, len(b.Regions))
	for _, r := range b.Regions {
		geometry, err := orbGeometryToGeoJSON(r.Geometry)
		if err != nil {
			return nil, fmt.Errorf("boundary feature for county %s: %w", r.CountyCode, err)
		}
		features = append(features, geoJSONFeature(geometry, map[string]any{
			"regionCode":   r.CountyCode,
			"regionName":   r.CountyName,
			"regionNameEn": r.CountyNameEN,
		}))
	}
	geojson, err := structpb.NewStruct(geoJSONFeatureCollection(features))
	if err != nil {
		return nil, fmt.Errorf("boundary geojson: %w", err)
	}
	return &pb.SpatialBoundary{
		Version:   b.Version,
		Digest:    b.Digest,
		BboxWgs84: b.BBoxWGS84[:],
		Geojson:   geojson,
	}, nil
}

// toBoundaryPriceGeojson builds the per-county choropleth layer. A county
// the surface never reached keeps its geometry but carries a null price
// and status "unavailable" — issue #2248 forbids substituting 0 or a
// national average for missing data, and dropping the feature entirely
// would leave a hole in the map instead of a visibly greyed-out county.
func toBoundaryPriceGeojson(rollups []usecase.SpatialCountyRollup) (*structpb.Struct, error) {
	features := make([]any, 0, len(rollups))
	for _, r := range rollups {
		geometry, err := orbGeometryToGeoJSON(r.Geometry)
		if err != nil {
			return nil, fmt.Errorf("boundary feature for county %s: %w", r.CountyCode, err)
		}
		features = append(features, geoJSONFeature(geometry, map[string]any{
			"regionCode":           r.CountyCode,
			"regionName":           r.CountyName,
			"regionNameEn":         r.CountyNameEN,
			"status":               r.Status,
			"estimatedP50TwdPerKg": finiteOrNull(r.EstimatedP50TwdPerKg),
			"bandLowerTwdPerKg":    finiteOrNull(r.BandLower),
			"bandUpperTwdPerKg":    finiteOrNull(r.BandUpper),
			"sampleCellCount":      r.SupportingCellCount,
			"aggregationMethod":    spatialAggregationMethod,
		}))
	}
	return structpb.NewStruct(geoJSONFeatureCollection(features))
}

// toMarketPointsGeojson emits only the markets that can actually be drawn:
// issue #2248 requires a finite P50 AND a resolved coordinate. Everything
// else (closed markets, markets whose forecast produced nothing, markets
// with no geocoded facility) stays in market_prices as a table row —
// emitting it here with a null geometry or a 0/0 coordinate would put a
// market in the Pacific Ocean.
func toMarketPointsGeojson(prices []usecase.SpatialMarketPrice) (*structpb.Struct, error) {
	features := make([]any, 0, len(prices))
	for _, p := range prices {
		if !isFinitePtr(p.P50) || !isFinitePtr(p.Longitude) || !isFinitePtr(p.Latitude) {
			continue
		}
		features = append(features, geoJSONFeature(
			map[string]any{"type": "Point", "coordinates": []any{*p.Longitude, *p.Latitude}},
			map[string]any{
				"marketCode":     p.MarketCode,
				"marketName":     p.MarketName,
				"categoryCode":   p.CategoryCode,
				"p50":            *p.P50,
				"marketStatus":   p.MarketStatus,
				"usedForSurface": p.UsedForSurface,
			},
		))
	}
	return structpb.NewStruct(geoJSONFeatureCollection(features))
}

// toSurfaceCellsGeojson emits one Polygon per interpolated grid cell.
// BuildSurfaceCells produces a single closed outer ring per cell, so there
// is never an interior ring to carry here.
func toSurfaceCellsGeojson(cells []spatial.SurfaceCell) (*structpb.Struct, error) {
	features := make([]any, 0, len(cells))
	for _, c := range cells {
		features = append(features, geoJSONFeature(
			map[string]any{"type": "Polygon", "coordinates": []any{ringCoordinates(c.PolygonWGS84)}},
			map[string]any{
				"priceTwdPerKg":     c.PriceTwdPerKg,
				"bandIndex":         c.BandIndex,
				"bandLowerTwdPerKg": finiteOrNull(c.LowerBound),
				"bandUpperTwdPerKg": finiteOrNull(c.UpperBound),
			},
		))
	}
	return structpb.NewStruct(geoJSONFeatureCollection(features))
}

// toContourLinesGeojson emits one two-point LineString per marching-squares
// segment. The feature id is the issue's "穩定 feature id" requirement: the
// usecase output is deterministic for the same selection/date/horizon (see
// SpatialSurface.Get's ordering guarantees), so region + position is a
// stable handle a client can key a rendered feature on across re-fetches.
func toContourLinesGeojson(segments []usecase.SpatialContourSegment) (*structpb.Struct, error) {
	features := make([]any, 0, len(segments))
	for i, s := range segments {
		f := geoJSONFeature(
			map[string]any{"type": "LineString", "coordinates": []any{
				[]any{s.Left[0], s.Left[1]},
				[]any{s.Right[0], s.Right[1]},
			}},
			map[string]any{
				"levelPriceTwdPerKg": s.LevelValue,
				"surfaceRegion":      s.SurfaceRegion,
			},
		)
		f["id"] = fmt.Sprintf("contour-%s-%d", s.SurfaceRegion, i)
		features = append(features, f)
	}
	return structpb.NewStruct(geoJSONFeatureCollection(features))
}

func geoJSONFeatureCollection(features []any) map[string]any {
	return map[string]any{"type": "FeatureCollection", "features": features}
}

func geoJSONFeature(geometry, properties map[string]any) map[string]any {
	return map[string]any{"type": "Feature", "geometry": geometry, "properties": properties}
}

// orbGeometryToGeoJSON converts the county boundary geometry the geography
// registry loaded (orb values, kept Go-native all the way through the
// usecase) into RFC 7946 geometry objects structpb can carry. Only the two
// types the boundary loader accepts are handled; anything else is a
// corrupt registry, and issue #2248 requires invalid geometry to be
// rejected rather than shipped to the browser.
func orbGeometryToGeoJSON(g orb.Geometry) (map[string]any, error) {
	switch geom := g.(type) {
	case orb.Polygon:
		return map[string]any{"type": "Polygon", "coordinates": polygonRings(geom)}, nil
	case orb.MultiPolygon:
		polygons := make([]any, 0, len(geom))
		for _, p := range geom {
			polygons = append(polygons, polygonRings(p))
		}
		return map[string]any{"type": "MultiPolygon", "coordinates": polygons}, nil
	default:
		return nil, fmt.Errorf("unsupported geometry %T (RFC 7946 boundary features must be Polygon or MultiPolygon)", g)
	}
}

func polygonRings(p orb.Polygon) []any {
	rings := make([]any, 0, len(p))
	for _, ring := range p {
		rings = append(rings, lineStringCoordinates(ring))
	}
	return rings
}

func lineStringCoordinates(points []orb.Point) []any {
	out := make([]any, 0, len(points))
	for _, pt := range points {
		out = append(out, []any{pt[0], pt[1]})
	}
	return out
}

// ringCoordinates is lineStringCoordinates for the domain/spatial layer's
// own coordinate type — surface cells never became orb values because
// nothing downstream of BuildSurfaceCells needed orb's planar predicates.
func ringCoordinates(ring [][2]float64) []any {
	out := make([]any, 0, len(ring))
	for _, pt := range ring {
		out = append(out, []any{pt[0], pt[1]})
	}
	return out
}

func isFinitePtr(v *float64) bool {
	return v != nil && !math.IsNaN(*v) && !math.IsInf(*v, 0)
}

// finiteOrNull renders an optional price as a JSON value: the number when
// present and finite, JSON null otherwise. structpb will happily carry a
// NaN or ±Inf float64, but JSON cannot spell one — issue #2248 requires
// those to be rejected before reaching the browser, and silently emitting
// null is strictly better than a response the gateway cannot marshal.
func finiteOrNull(v *float64) any {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	return *v
}

func toPriceObservations(obs []domain.ObservedPrice) []*pb.PriceObservation {
	out := make([]*pb.PriceObservation, 0, len(obs))
	for _, o := range obs {
		out = append(out, &pb.PriceObservation{
			MarketCode:   o.MarketCode,
			CropCode:     o.CropCode,
			TradeDate:    o.TradeDate.Format("2006-01-02"),
			AvgPrice:     o.AvgPrice,
			UpperPrice:   o.UpperPrice,
			MiddlePrice:  o.MiddlePrice,
			LowerPrice:   o.LowerPrice,
			VolumeKg:     o.VolumeKg,
			PriceLayer:   proto.String(string(domain.PriceLayerWholesaleAuction)),
			OriginalUnit: proto.String(domain.UnitKilogram),
		})
	}
	return out
}
