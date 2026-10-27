package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/paulmach/orb"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

type fakeObservedPriceUC struct {
	get func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, tradeDate *time.Time, history bool, now time.Time) (bool, []domain.ObservedPrice, usecase.DataMeta, error)
}

func (f *fakeObservedPriceUC) Get(ctx context.Context, cropName, marketCode, cropCode, groupCode string, tradeDate *time.Time, history bool, now time.Time) (bool, []domain.ObservedPrice, usecase.DataMeta, error) {
	return f.get(ctx, cropName, marketCode, cropCode, groupCode, tradeDate, history, now)
}

func TestGetObservedPrice_ResolvedWithData_ReturnsObservations(t *testing.T) {
	avg := 25.5
	wantMeta := usecase.DataMeta{
		Source: "MOA_WHOLESALE_DAILY", FreshnessStatus: forecast.FreshnessFresh, UsabilityStatus: forecast.UsabilityUsable,
		SchemaVersion: "hawkr.pricebook/v1", Currency: "TWD", Unit: "kg",
	}
	uc := &fakeObservedPriceUC{get: func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, tradeDate *time.Time, history bool, now time.Time) (bool, []domain.ObservedPrice, usecase.DataMeta, error) {
		return true, []domain.ObservedPrice{{MarketCode: "109", CropCode: "A1", TradeDate: time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC), AvgPrice: &avg}}, wantMeta, nil
	}}
	s := NewPricebookServer(uc, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.GetObservedPrice(context.Background(), &pb.GetObservedPriceRequest{CropName: "甘藍菜"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Observations) != 1 || resp.Observations[0].MarketCode != "109" {
		t.Fatalf("Observations = %+v, want one row for market 109", resp.Observations)
	}
	if resp.Observations[0].GetAvgPrice() != 25.5 {
		t.Errorf("AvgPrice = %v, want 25.5", resp.Observations[0].GetAvgPrice())
	}
	if resp.DataMeta == nil {
		t.Fatal("DataMeta = nil, want populated (issue #2249 Slice A: always present)")
	}
	if resp.DataMeta.GetFreshnessStatus() != "fresh" || resp.DataMeta.GetUsabilityStatus() != "usable" {
		t.Errorf("DataMeta = %+v, want freshnessStatus=fresh usabilityStatus=usable", resp.DataMeta)
	}
	if resp.DataMeta.ModelVersion != nil {
		t.Errorf("DataMeta.ModelVersion = %v, want nil (observed responses never carry a model version)", resp.DataMeta.ModelVersion)
	}
}

func TestGetObservedPrice_Unresolved_ReturnsEmptyObservationsNotError(t *testing.T) {
	uc := &fakeObservedPriceUC{get: func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, tradeDate *time.Time, history bool, now time.Time) (bool, []domain.ObservedPrice, usecase.DataMeta, error) {
		return false, nil, usecase.DataMeta{FreshnessStatus: forecast.FreshnessUnavailable, UsabilityStatus: forecast.UsabilityUnusable}, nil
	}}
	s := NewPricebookServer(uc, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.GetObservedPrice(context.Background(), &pb.GetObservedPriceRequest{CropName: "火龍果"})
	if err != nil {
		t.Fatalf("unexpected error for business not-found: %v", err)
	}
	if len(resp.Observations) != 0 {
		t.Errorf("Observations = %+v, want empty", resp.Observations)
	}
	if resp.DataMeta == nil || resp.DataMeta.GetUsabilityStatus() != "unusable" {
		t.Errorf("DataMeta = %+v, want populated with usabilityStatus=unusable", resp.DataMeta)
	}
}

func TestGetObservedPrice_AmbiguousCropGroup_ReturnsInternalError(t *testing.T) {
	uc := &fakeObservedPriceUC{get: func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, tradeDate *time.Time, history bool, now time.Time) (bool, []domain.ObservedPrice, usecase.DataMeta, error) {
		return true, nil, usecase.DataMeta{}, usecase.ErrAmbiguousCropGroup
	}}
	s := NewPricebookServer(uc, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetObservedPrice(context.Background(), &pb.GetObservedPriceRequest{CropName: "甘藍菜"})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("err = %v, want gRPC Internal (ambiguity is a data-quality problem the caller can't act on, not InvalidArgument)", err)
	}
}

func TestGetObservedPrice_EmptyCropName_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetObservedPrice(context.Background(), &pb.GetObservedPriceRequest{CropName: ""})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

type fakeEstimatedItemPriceUC struct {
	get func(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error)
}

func (f *fakeEstimatedItemPriceUC) Get(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error) {
	return f.get(ctx, cropName, netWeightKg, marketCode, now)
}

func TestGetEstimatedItemPrice_Ready_ReturnsPrices(t *testing.T) {
	low, median, high := int64(10), int64(13), int64(15)
	groupCode := "G1"
	uc := &fakeEstimatedItemPriceUC{get: func(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error) {
		return usecase.EstimatedItemPriceResult{
			Status: forecast.PredictionStatusReady, Low: &low, Median: &median, High: &high,
			ResolvedCropGroupCode: &groupCode, ModelVersion: "v1",
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, uc, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "甘藍菜", NetWeightKg: 0.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetStatus() != "ready" || resp.GetLow() != 10 || resp.GetMedian() != 13 || resp.GetHigh() != 15 {
		t.Errorf("resp = %+v, want ready/10/13/15", resp)
	}
	if resp.GetResolvedCropGroupCode() != "G1" {
		t.Errorf("ResolvedCropGroupCode = %q, want G1", resp.GetResolvedCropGroupCode())
	}
}

func TestGetEstimatedItemPrice_Unavailable_ReturnsStatusNoPrices(t *testing.T) {
	uc := &fakeEstimatedItemPriceUC{get: func(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error) {
		return usecase.EstimatedItemPriceResult{Status: forecast.PredictionStatusUnavailable}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, uc, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "火龍果", NetWeightKg: 0.5})
	if err != nil {
		t.Fatalf("unexpected error for business not-found: %v", err)
	}
	if resp.GetStatus() != "unavailable" {
		t.Errorf("Status = %q, want unavailable", resp.GetStatus())
	}
	if resp.Low != nil || resp.Median != nil || resp.High != nil {
		t.Errorf("expected no price fields, got %+v", resp)
	}
}

func TestGetEstimatedItemPrice_EmptyCropName_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "", NetWeightKg: 0.5})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

func TestGetEstimatedItemPrice_NonPositiveNetWeight_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "甘藍菜", NetWeightKg: 0})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

func TestGetEstimatedItemPrice_NaNNetWeight_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "甘藍菜", NetWeightKg: math.NaN()})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

func TestGetEstimatedItemPrice_InfNetWeight_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "甘藍菜", NetWeightKg: math.Inf(1)})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

func TestGetEstimatedItemPrice_AmbiguousCropGroup_ReturnsInternalError(t *testing.T) {
	uc := &fakeEstimatedItemPriceUC{get: func(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error) {
		return usecase.EstimatedItemPriceResult{}, usecase.ErrAmbiguousCropGroup
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, uc, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "甘藍菜", NetWeightKg: 0.5})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("err = %v, want gRPC Internal", err)
	}
}

func TestGetEstimatedItemPrice_MarketCodeNotAllowedForGroup_ReturnsInvalidArgument(t *testing.T) {
	uc := &fakeEstimatedItemPriceUC{get: func(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error) {
		return usecase.EstimatedItemPriceResult{}, usecase.ErrMarketCodeNotAllowedForGroup
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, uc, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "甘藍菜", NetWeightKg: 0.5, MarketCode: proto.String("109")})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

func TestGetEstimatedItemPrice_MarketCodeRequiredForCrop_ReturnsInvalidArgument(t *testing.T) {
	uc := &fakeEstimatedItemPriceUC{get: func(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (usecase.EstimatedItemPriceResult, error) {
		return usecase.EstimatedItemPriceResult{}, usecase.ErrMarketCodeRequiredForCrop
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, uc, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetEstimatedItemPrice(context.Background(), &pb.GetEstimatedItemPriceRequest{CropName: "甘藍-初秋", NetWeightKg: 0.5})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

type fakeForecastCurveUC struct {
	get func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, now time.Time) (usecase.ForecastCurveResult, error)
}

func (f *fakeForecastCurveUC) Get(ctx context.Context, cropName, marketCode, cropCode, groupCode string, now time.Time) (usecase.ForecastCurveResult, error) {
	return f.get(ctx, cropName, marketCode, cropCode, groupCode, now)
}

func TestGetForecastCurve_Ready_ReturnsPoints(t *testing.T) {
	p10, p50, p90 := 20.0, 25.0, 30.0
	// resolvedGroupCode, not groupCode: the closure now has a groupCode
	// *parameter* (the caller's requested bypass code, empty here), and
	// taking &groupCode inside would silently capture that empty string
	// instead of this fixture value.
	resolvedGroupCode := "G1"
	modelVersion := "v1"
	digest := "sha256:" + repeatHexTestGrpc('c')
	uc := &fakeForecastCurveUC{get: func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, now time.Time) (usecase.ForecastCurveResult, error) {
		return usecase.ForecastCurveResult{
			Status: forecast.PredictionStatusReady, ResolvedCropGroupCode: &resolvedGroupCode, ModelVersion: modelVersion,
			Points: []usecase.CurvePoint{
				{TargetDate: forecast.NewDate(2026, 7, 26), HorizonDays: 1, Status: forecast.PredictionStatusReady, P10: &p10, P50: &p50, P90: &p90},
			},
			DataMeta: usecase.DataMeta{
				Source: "MOA_WHOLESALE_DAILY", FreshnessStatus: forecast.FreshnessFresh, UsabilityStatus: forecast.UsabilityUsable,
				ModelVersion: &modelVersion, DatasetDigest: &digest, SnapshotID: &digest,
				SchemaVersion: "hawkr.pricebook/v1", Currency: "TWD", Unit: "kg",
				GeneratedAt: time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC),
			},
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, uc, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{CropName: "甘藍菜"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetStatus() != "ready" || len(resp.GetPoints()) != 1 {
		t.Fatalf("resp = %+v, want ready/1 point", resp)
	}
	if resp.GetPoints()[0].GetTargetDate() != "2026-07-26" {
		t.Errorf("TargetDate = %q, want 2026-07-26", resp.GetPoints()[0].GetTargetDate())
	}
	if resp.GetResolvedCropGroupCode() != "G1" {
		t.Errorf("ResolvedCropGroupCode = %q, want G1", resp.GetResolvedCropGroupCode())
	}
	if resp.DataMeta == nil {
		t.Fatal("DataMeta = nil, want populated")
	}
	if resp.DataMeta.GetModelVersion() != resp.GetModelVersion() {
		t.Errorf("DataMeta.ModelVersion = %q, want equal to top-level ModelVersion %q", resp.DataMeta.GetModelVersion(), resp.GetModelVersion())
	}
	if resp.DataMeta.GetDatasetDigest() != digest {
		t.Errorf("DataMeta.DatasetDigest = %q, want %q", resp.DataMeta.GetDatasetDigest(), digest)
	}
	if resp.DataMeta.GetGeneratedAt() != "2026-07-25T08:00:00Z" {
		t.Errorf("DataMeta.GeneratedAt = %q, want UTC Z-suffixed RFC3339 (repo time-format redline), not the issue example's +08:00", resp.DataMeta.GetGeneratedAt())
	}
}

func TestGetForecastCurve_Unavailable_ReturnsStatusNoPoints(t *testing.T) {
	uc := &fakeForecastCurveUC{get: func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, now time.Time) (usecase.ForecastCurveResult, error) {
		return usecase.ForecastCurveResult{Status: forecast.PredictionStatusUnavailable}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, uc, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{CropName: "火龍果"})
	if err != nil {
		t.Fatalf("unexpected error for business not-found: %v", err)
	}
	if resp.GetStatus() != "unavailable" || len(resp.GetPoints()) != 0 {
		t.Errorf("resp = %+v, want unavailable/0 points", resp)
	}
}

func TestGetForecastCurve_EmptyCropName_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{CropName: ""})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

func TestGetForecastCurve_MarketCodeNotAllowedForGroup_ReturnsInvalidArgument(t *testing.T) {
	uc := &fakeForecastCurveUC{get: func(ctx context.Context, cropName, marketCode, cropCode, groupCode string, now time.Time) (usecase.ForecastCurveResult, error) {
		return usecase.ForecastCurveResult{}, usecase.ErrMarketCodeNotAllowedForGroup
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, uc, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{CropName: "甘藍菜", MarketCode: proto.String("109")})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

type fakeSearchCropCatalogUC struct {
	get                  func(ctx context.Context, query string, limit int32) ([]usecase.CatalogMatch, error)
	listGroupCropOptions func(ctx context.Context, groupCode string, limit int32) ([]usecase.CatalogMatch, error)
}

func (f *fakeSearchCropCatalogUC) Get(ctx context.Context, query string, limit int32) ([]usecase.CatalogMatch, error) {
	return f.get(ctx, query, limit)
}

func (f *fakeSearchCropCatalogUC) ListGroupCropOptions(ctx context.Context, groupCode string, limit int32) ([]usecase.CatalogMatch, error) {
	return f.listGroupCropOptions(ctx, groupCode, limit)
}

func TestSearchCropCatalog_ReturnsMatches(t *testing.T) {
	cropCode, matchedAlias := "crop1", "小黃瓜"
	uc := &fakeSearchCropCatalogUC{get: func(ctx context.Context, query string, limit int32) ([]usecase.CatalogMatch, error) {
		return []usecase.CatalogMatch{{MatchType: "crop", GroupCode: "g1", GroupName: "花胡瓜", CropCode: &cropCode, MatchedAlias: &matchedAlias, HasForecast: true, Markets: []usecase.CatalogMarketOption{{MarketCode: "109", MarketName: "台北一"}}}}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, uc, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.SearchCropCatalog(context.Background(), &pb.SearchCropCatalogRequest{Query: "小黃瓜"})

	if err != nil {
		t.Fatalf("SearchCropCatalog: unexpected error %v", err)
	}
	if len(resp.Matches) != 1 || resp.Matches[0].GetCropCode() != "crop1" || len(resp.Matches[0].Markets) != 1 {
		t.Fatalf("resp.Matches = %+v, want 1 crop match with 1 market", resp.Matches)
	}
}

func TestSearchCropCatalog_UsecaseErrorMapsToInternal(t *testing.T) {
	uc := &fakeSearchCropCatalogUC{get: func(ctx context.Context, query string, limit int32) ([]usecase.CatalogMatch, error) {
		return nil, errors.New("boom")
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, uc, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.SearchCropCatalog(context.Background(), &pb.SearchCropCatalogRequest{})

	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Errorf("code = %v, want Internal", st.Code())
	}
}

func TestGetObservedPrice_ConflictingCropNameAndCropCodeIsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)
	cropCode := "crop1"
	_, err := s.GetObservedPrice(context.Background(), &pb.GetObservedPriceRequest{CropName: "小黃瓜", CropCode: &cropCode})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", st.Code())
	}
}

type fakeTrendSummaryUC struct {
	get func(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (usecase.TrendSummaryResult, error)
}

func (f *fakeTrendSummaryUC) Get(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (usecase.TrendSummaryResult, error) {
	return f.get(ctx, cropCode, groupCode, marketCode, from, to, granularity, comparisons, now)
}

func TestGetTrendSummaryRejectsBothCropAndGroup(t *testing.T) {
	cropCode, groupCode, marketCode := "A1", "N04", "109"
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)
	_, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		CropCode: &cropCode, GroupCode: &groupCode, MarketCode: &marketCode, From: "2026-08-01", To: "2026-08-01", Granularity: "day",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestGetTrendSummaryRejectsUnknownGranularity(t *testing.T) {
	cropCode, marketCode := "A1", "109"
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)
	_, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		CropCode: &cropCode, MarketCode: &marketCode, From: "2026-08-01", To: "2026-08-01", Granularity: "month",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument for unknown granularity, got %v", err)
	}
}

func TestGetTrendSummaryRejectsUnknownComparison(t *testing.T) {
	cropCode, marketCode := "A1", "109"
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)
	_, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		CropCode: &cropCode, MarketCode: &marketCode, From: "2026-08-01", To: "2026-08-01", Granularity: "day", Comparisons: []string{"mom"},
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument for unknown comparison, got %v", err)
	}
}

func TestGetTrendSummaryRejectsRangeOver366Days(t *testing.T) {
	cropCode, marketCode := "A1", "109"
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)
	_, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		CropCode: &cropCode, MarketCode: &marketCode, From: "2025-01-01", To: "2026-06-01", Granularity: "day",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument for range > 366 days, got %v", err)
	}
}

func TestGetTrendSummaryHappyPath(t *testing.T) {
	cropCode, marketCode := "A1", "109"
	v := 40.0
	trendUC := &fakeTrendSummaryUC{get: func(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (usecase.TrendSummaryResult, error) {
		return usecase.TrendSummaryResult{
			SelectionKind: "crop",
			Points: []usecase.TrendSummaryPoint{{
				Bounds: forecast.PeriodBounds{Start: forecast.NewDate(2026, 8, 1), End: forecast.NewDate(2026, 8, 1)},
				Status: forecast.PredictionStatusReady, Value: &v, SampleCount: 1, ObservedMarketCount: 1, AggregationMethod: forecast.TrendAggregationVolumeWeightedV1,
			}},
			DataMeta: usecase.DataMeta{Source: "MOA_WHOLESALE_DAILY", SchemaVersion: "hawkr.pricebook/v1", Currency: "TWD", Unit: "kg"},
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, trendUC, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)
	resp, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		CropCode: &cropCode, MarketCode: &marketCode, From: "2026-08-01", To: "2026-08-01", Granularity: "day",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SelectionKind != "crop" {
		t.Errorf("SelectionKind = %q, want crop", resp.SelectionKind)
	}
	if len(resp.Points) != 1 || resp.Points[0].GetObservedAverageTwdPerKg() != 40 {
		t.Errorf("Points = %+v", resp.Points)
	}
}

// TestGetTrendSummaryComparisonsWireOrderIsDeterministic guards against
// toTrendSummaryResponse regressing to ranging usecase.TrendSummaryResult's
// Comparisons map directly — Comparisons is a repeated (ordered) proto
// field, but Go map iteration order is randomized per run, so a naive
// `for kind, c := range r.Comparisons` would make this response's wire
// order flake nondeterministically across calls whenever both wow and
// yoy364 are present. Requests yoy364 before wow to prove the response
// order (always wow, then yoy364 — see trendComparisonKindOrder) doesn't
// just happen to mirror request order or map insertion order.
func TestGetTrendSummaryComparisonsWireOrderIsDeterministic(t *testing.T) {
	cropCode, marketCode := "A1", "109"
	trendUC := &fakeTrendSummaryUC{get: func(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (usecase.TrendSummaryResult, error) {
		return usecase.TrendSummaryResult{
			SelectionKind: "crop",
			Comparisons: map[forecast.TrendComparisonKind]forecast.TrendComparison{
				forecast.ComparisonYoY364: {Status: forecast.PredictionStatusReady},
				forecast.ComparisonWoW:    {Status: forecast.PredictionStatusReady},
			},
			DataMeta: usecase.DataMeta{Source: "MOA_WHOLESALE_DAILY", SchemaVersion: "hawkr.pricebook/v1", Currency: "TWD", Unit: "kg"},
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, trendUC, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		CropCode: &cropCode, MarketCode: &marketCode, From: "2026-08-01", To: "2026-08-01", Granularity: "day",
		Comparisons: []string{"yoy364", "wow"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Comparisons) != 2 || resp.Comparisons[0].Kind != "wow" || resp.Comparisons[1].Kind != "yoy364" {
		t.Fatalf("Comparisons = %+v, want [wow, yoy364] in that fixed order", resp.Comparisons)
	}
}

// TestGetTrendSummary_MarketCodeRequiredForCrop_ReturnsInvalidArgument and
// TestGetTrendSummary_MarketCodeNotAllowedForGroup_ReturnsInvalidArgument
// mirror TestGetEstimatedItemPrice_MarketCodeRequiredForCrop_ReturnsInvalidArgument
// / TestGetForecastCurve_MarketCodeNotAllowedForGroup_ReturnsInvalidArgument:
// GetTrendSummary deliberately does not eagerly re-check market_code
// requiredness itself (see the doc comment on GetTrendSummary) — it's the
// switch below s.trendUC.Get that must map these two usecase sentinels to
// InvalidArgument. These tests exist so that mapping can't silently
// regress.
func TestGetTrendSummary_MarketCodeRequiredForCrop_ReturnsInvalidArgument(t *testing.T) {
	uc := &fakeTrendSummaryUC{get: func(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (usecase.TrendSummaryResult, error) {
		return usecase.TrendSummaryResult{}, usecase.ErrMarketCodeRequiredForCrop
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, uc, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	_, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		CropCode: &cropCode, From: "2026-08-01", To: "2026-08-01", Granularity: "day",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

func TestGetTrendSummary_MarketCodeNotAllowedForGroup_ReturnsInvalidArgument(t *testing.T) {
	uc := &fakeTrendSummaryUC{get: func(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (usecase.TrendSummaryResult, error) {
		return usecase.TrendSummaryResult{}, usecase.ErrMarketCodeNotAllowedForGroup
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, uc, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	groupCode, marketCode := "N04", "109"
	_, err := s.GetTrendSummary(context.Background(), &pb.GetTrendSummaryRequest{
		GroupCode: &groupCode, MarketCode: &marketCode, From: "2026-08-01", To: "2026-08-01", Granularity: "day",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want gRPC InvalidArgument", err)
	}
}

type fakeSpatialSurfaceUC struct {
	get func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error)
}

func (f *fakeSpatialSurfaceUC) Get(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
	return f.get(ctx, in)
}

// TestGetSpatialSurface_BoundaryPriceGeojson_RoundTripsFeatureCollection
// pins the one genuinely new thing this handler does: turning Go-native
// geometry (orb.Polygon) plus a usecase DTO into a google.protobuf.Struct
// that is a valid RFC 7946 FeatureCollection. It asserts on the marshalled
// JSON rather than field-by-field because the whole point of using Struct
// instead of an escaped string is that the bytes a MapLibre client
// receives are real JSON — so that is what the test compares. json.Marshal
// sorts map keys, so the expectation below is stable.
func TestGetSpatialSurface_BoundaryPriceGeojson_RoundTripsFeatureCollection(t *testing.T) {
	p50, lower, upper := 42.5, 40.0, 50.0
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		return usecase.SpatialSurfaceOutput{
			Status: "degraded", DisplayMode: "points_only", SelectionKind: "crop",
			CountyRollups: []usecase.SpatialCountyRollup{
				{
					CountyCode: "63000", CountyName: "臺北市", CountyNameEN: "Taipei City",
					Status: "ready", EstimatedP50TwdPerKg: &p50, BandLower: &lower, BandUpper: &upper,
					SupportingCellCount: 18,
					Geometry: orb.Polygon{{
						{121.5, 25.0}, {121.6, 25.0}, {121.6, 25.1}, {121.5, 25.1}, {121.5, 25.0},
					}},
				},
				{
					// A county the surface never reached: null price, never 0,
					// never a national average (issue #2248).
					CountyCode: "09007", CountyName: "連江縣", CountyNameEN: "Lienchiang County",
					Status: "unavailable",
					Geometry: orb.MultiPolygon{{{
						{119.9, 26.1}, {120.0, 26.1}, {120.0, 26.2}, {119.9, 26.2}, {119.9, 26.1},
					}}},
				},
			},
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	resp, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.BoundaryPriceGeojson == nil {
		t.Fatal("BoundaryPriceGeojson = nil, want a FeatureCollection (a degraded response carries empty features, never a missing layer)")
	}

	got, err := json.Marshal(resp.BoundaryPriceGeojson.AsMap())
	if err != nil {
		t.Fatalf("marshal BoundaryPriceGeojson: %v", err)
	}
	want := `{"features":[` +
		`{"geometry":{"coordinates":[[[121.5,25],[121.6,25],[121.6,25.1],[121.5,25.1],[121.5,25]]],"type":"Polygon"},` +
		`"properties":{"aggregationMethod":"surface_cell_area_weighted/v1","bandLowerTwdPerKg":40,"bandUpperTwdPerKg":50,` +
		`"estimatedP50TwdPerKg":42.5,"regionCode":"63000","regionName":"臺北市","regionNameEn":"Taipei City",` +
		`"sampleCellCount":18,"status":"ready"},"type":"Feature"},` +
		`{"geometry":{"coordinates":[[[[119.9,26.1],[120,26.1],[120,26.2],[119.9,26.2],[119.9,26.1]]]],"type":"MultiPolygon"},` +
		`"properties":{"aggregationMethod":"surface_cell_area_weighted/v1","bandLowerTwdPerKg":null,"bandUpperTwdPerKg":null,` +
		`"estimatedP50TwdPerKg":null,"regionCode":"09007","regionName":"連江縣","regionNameEn":"Lienchiang County",` +
		`"sampleCellCount":0,"status":"unavailable"},"type":"Feature"}` +
		`],"type":"FeatureCollection"}`
	if string(got) != want {
		t.Errorf("BoundaryPriceGeojson =\n%s\nwant\n%s", got, want)
	}
}

// TestGetSpatialSurface_PointCellAndContourLayers_RoundTripGeojson pins the
// other three FeatureCollections, including the one filtering rule the
// issue spells out: market_points_geojson carries only markets that have
// BOTH a finite P50 and a resolved coordinate. A closed or ungeocoded
// market stays in market_prices but must never appear here with a null
// geometry or a 0 placeholder.
func TestGetSpatialSurface_PointCellAndContourLayers_RoundTripGeojson(t *testing.T) {
	p50, cellLower, cellUpper := 42.5, 40.0, 50.0
	lon, lat := 121.5, 25.05
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		return usecase.SpatialSurfaceOutput{
			Status: "ready", DisplayMode: "contours_and_points", SelectionKind: "crop",
			MarketPrices: []usecase.SpatialMarketPrice{
				{
					MarketCode: "109", MarketName: "台北一", CategoryCode: "N04",
					PredictionStatus: "ready", MarketStatus: "open", UsedForSurface: true,
					P50: &p50, Longitude: &lon, Latitude: &lat,
				},
				{
					// Closed market: geocoded but no price — table row only.
					MarketCode: "400", MarketName: "台中市", CategoryCode: "N04",
					PredictionStatus: "market_closed", MarketStatus: "closed",
					Longitude: &lon, Latitude: &lat,
				},
				{
					// Priced but never geocoded — table row only, never a
					// 0/0 point.
					MarketCode: "930", MarketName: "屏東市", CategoryCode: "N04",
					PredictionStatus: "ready", MarketStatus: "open", P50: &p50,
				},
			},
			SurfaceCells: []spatial.SurfaceCell{{
				PolygonWGS84: [][2]float64{
					{121.5, 25.0}, {121.55, 25.0}, {121.55, 25.05}, {121.5, 25.05}, {121.5, 25.0},
				},
				PriceTwdPerKg: 42.5, BandIndex: 2, LowerBound: &cellLower, UpperBound: &cellUpper,
			}},
			ContourSegments: []usecase.SpatialContourSegment{{
				Left: [2]float64{121.5, 25.0}, Right: [2]float64{121.55, 25.05},
				LevelValue: 40, SurfaceRegion: "taiwan_main_island",
			}},
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	resp, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tc := range []struct {
		name string
		got  *structpb.Struct
		want string
	}{
		{
			name: "market_points_geojson",
			got:  resp.MarketPointsGeojson,
			want: `{"features":[{"geometry":{"coordinates":[121.5,25.05],"type":"Point"},` +
				`"properties":{"categoryCode":"N04","marketCode":"109","marketName":"台北一",` +
				`"marketStatus":"open","p50":42.5,"usedForSurface":true},"type":"Feature"}],` +
				`"type":"FeatureCollection"}`,
		},
		{
			name: "surface_cells_geojson",
			got:  resp.SurfaceCellsGeojson,
			want: `{"features":[{"geometry":{"coordinates":[[[121.5,25],[121.55,25],[121.55,25.05],[121.5,25.05],[121.5,25]]],"type":"Polygon"},` +
				`"properties":{"bandIndex":2,"bandLowerTwdPerKg":40,"bandUpperTwdPerKg":50,"priceTwdPerKg":42.5},` +
				`"type":"Feature"}],"type":"FeatureCollection"}`,
		},
		{
			name: "contour_lines_geojson",
			got:  resp.ContourLinesGeojson,
			want: `{"features":[{"geometry":{"coordinates":[[121.5,25],[121.55,25.05]],"type":"LineString"},` +
				`"id":"contour-taiwan_main_island-0",` +
				`"properties":{"levelPriceTwdPerKg":40,"surfaceRegion":"taiwan_main_island"},"type":"Feature"}],` +
				`"type":"FeatureCollection"}`,
		},
	} {
		if tc.got == nil {
			t.Errorf("%s = nil, want a FeatureCollection", tc.name)
			continue
		}
		raw, err := json.Marshal(tc.got.AsMap())
		if err != nil {
			t.Errorf("marshal %s: %v", tc.name, err)
			continue
		}
		if string(raw) != tc.want {
			t.Errorf("%s =\n%s\nwant\n%s", tc.name, raw, tc.want)
		}
	}
}

// TestGetSpatialSurface_EmptyOutput_ReturnsFourEmptyFeatureCollections
// pins the issue's "合法降級時可回空 features，不可省略整個欄位" rule: an
// unavailable surface still carries all four layers, each an empty
// FeatureCollection rather than a nil Struct the caller would have to
// special-case.
func TestGetSpatialSurface_EmptyOutput_ReturnsFourEmptyFeatureCollections(t *testing.T) {
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		return usecase.SpatialSurfaceOutput{Status: "unavailable", DisplayMode: "table_only", SelectionKind: "group"}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	groupCode := "N04"
	resp, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{GroupCode: &groupCode, HorizonDays: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantEmpty = `{"features":[],"type":"FeatureCollection"}`
	for name, got := range map[string]*structpb.Struct{
		"boundary_price_geojson": resp.BoundaryPriceGeojson,
		"market_points_geojson":  resp.MarketPointsGeojson,
		"surface_cells_geojson":  resp.SurfaceCellsGeojson,
		"contour_lines_geojson":  resp.ContourLinesGeojson,
	} {
		if got == nil {
			t.Errorf("%s = nil, want an empty FeatureCollection", name)
			continue
		}
		raw, err := json.Marshal(got.AsMap())
		if err != nil {
			t.Errorf("marshal %s: %v", name, err)
			continue
		}
		if string(raw) != wantEmpty {
			t.Errorf("%s = %s, want %s", name, raw, wantEmpty)
		}
	}
}

func TestGetSpatialSurface_Ready_MapsScalarsMarketPricesAndLineage(t *testing.T) {
	p10, p50, p90, volume := 38.0, 42.5, 47.25, 1200.0
	sourceAsOf := time.Date(2026, 8, 7, 16, 0, 0, 0, time.UTC)
	maxFeature := time.Date(2026, 8, 8, 2, 30, 0, 0, time.UTC)
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		return usecase.SpatialSurfaceOutput{
			Status: "degraded", DisplayMode: "surface_and_points", SelectionKind: "crop",
			SelectionCode: "A1", CategoryCode: "N04",
			TargetDate:  forecast.DateFromTime(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)),
			HorizonDays: 3, Currency: "TWD", Unit: "kg",
			Scale: spatial.PriceScale{ScaleID: "N04-price-scale-v1", PolicyVersion: "v1", Breaks: []float64{20, 30, 40, 50}},
			MarketPrices: []usecase.SpatialMarketPrice{
				{
					MarketCode: "109", MarketName: "台北一", CategoryCode: "N04",
					PredictionStatus: "ready", MarketStatus: "open", UsedForSurface: true,
					P10: &p10, P50: &p50, P90: &p90, RecentVolumeKg: &volume,
				},
				{
					MarketCode: "400", MarketName: "台中市", CategoryCode: "N04",
					PredictionStatus: "market_closed", MarketStatus: "closed",
					// no_usable_price, not an invented "market_closed" value:
					// the usecase's spatialExclusion* constants (unexported,
					// see spatial_surface.go) are the only vocabulary this
					// field ever actually carries on the wire.
					ExclusionReason: "no_usable_price",
				},
			},
			Statistics: usecase.SpatialStatistics{
				PricedMarketCount: 1, ClosedMarketCount: 1, TotalMarketCount: 2,
				SurfaceCellCount: 329, ContourSegmentCount: 38,
			},
			QualityReasonCodes:  []string{"partial_market_coverage"},
			Disclosures:         []string{"market_location_approximate", "not_investment_advice"},
			ExcludedMarketCodes: []string{"400"},
			Lineage: usecase.SpatialLineage{
				ArtifactID: "direct-ridge-v2-aa66e30432a2729a", ModelVersion: "direct-ridge-v2-aa66e30432a2729a",
				DatasetDigest: "sha256:" + repeatHexTestGrpc('a'), BoundaryDigest: "sha256:" + repeatHexTestGrpc('b'),
				GeneratedAt:    time.Date(2026, 8, 8, 3, 0, 0, 0, time.UTC),
				SourceDataAsOf: &sourceAsOf, MaxFeatureAvailableAt: &maxFeature,
				SchemaVersion: "hawkr.pricebook/v1",
			},
			Boundary: usecase.SpatialBoundaryOutput{
				Version: "taiwan-atlas-2021.9.20", Digest: "sha256:" + repeatHexTestGrpc('c'),
				BBoxWGS84: [4]float64{118.1, 21.8, 122.0, 26.4},
				Regions: []geography.CountyRegion{
					{CountyCode: "63000", CountyName: "台北市", CountyNameEN: "Taipei City", Geometry: orb.Polygon{orb.Ring{{121.5, 25.0}, {121.6, 25.0}, {121.6, 25.1}, {121.5, 25.1}, {121.5, 25.0}}}},
				},
			},
			SupportedLandAreaRatio: float64Ptr(0.42),
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	resp, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.GetStatus() != "degraded" || resp.GetDisplayMode() != "surface_and_points" || resp.GetSelectionKind() != "crop" {
		t.Errorf("status/display_mode/selection_kind = %q/%q/%q, want degraded/surface_and_points/crop",
			resp.GetStatus(), resp.GetDisplayMode(), resp.GetSelectionKind())
	}
	if resp.GetTargetDate() != "2026-08-09" || resp.GetHorizonDays() != 3 {
		t.Errorf("target_date/horizon_days = %q/%d, want 2026-08-09/3", resp.GetTargetDate(), resp.GetHorizonDays())
	}
	if resp.GetCurrency() != "TWD" || resp.GetUnit() != "kg" {
		t.Errorf("currency/unit = %q/%q, want TWD/kg", resp.GetCurrency(), resp.GetUnit())
	}

	scale := resp.GetScale()
	if scale.GetScaleId() != "N04-price-scale-v1" || scale.GetPolicyVersion() != "v1" {
		t.Errorf("scale = %+v, want N04-price-scale-v1/v1", scale)
	}
	if !reflect.DeepEqual(scale.GetBreaks(), []float64{20, 30, 40, 50}) {
		t.Errorf("scale.breaks = %v, want [20 30 40 50] (the versioned fixed scale, never recomputed per request)", scale.GetBreaks())
	}

	if len(resp.GetMarketPrices()) != 2 {
		t.Fatalf("market_prices = %+v, want both markets including the closed one", resp.GetMarketPrices())
	}
	first := resp.GetMarketPrices()[0]
	if first.GetMarketCode() != "109" || first.GetMarketName() != "台北一" || first.GetCategoryCode() != "N04" ||
		first.GetPredictionStatus() != "ready" || first.GetMarketStatus() != "open" || !first.GetUsedForSurface() {
		t.Errorf("market_prices[0] = %+v, want the priced 台北一 row", first)
	}
	if first.GetP10() != 38 || first.GetP50() != 42.5 || first.GetP90() != 47.25 || first.GetRecentVolumeKg() != 1200 {
		t.Errorf("market_prices[0] bands = %+v, want 38/42.5/47.25/1200", first)
	}
	closed := resp.GetMarketPrices()[1]
	if closed.P10 != nil || closed.P50 != nil || closed.P90 != nil || closed.RecentVolumeKg != nil {
		t.Errorf("market_prices[1] = %+v, want unset price fields on a closed market (never zero-filled)", closed)
	}

	stats := resp.GetStatistics()
	if stats.GetPricedMarketCount() != 1 || stats.GetClosedMarketCount() != 1 || stats.GetTotalMarketCount() != 2 ||
		stats.GetSurfaceCellCount() != 329 || stats.GetContourSegmentCount() != 38 {
		t.Errorf("statistics = %+v, want 1/1/2/329/38", stats)
	}
	if !reflect.DeepEqual(resp.GetQualityReasonCodes(), []string{"partial_market_coverage"}) ||
		!reflect.DeepEqual(resp.GetDisclosures(), []string{"market_location_approximate", "not_investment_advice"}) ||
		!reflect.DeepEqual(resp.GetExcludedMarketCodes(), []string{"400"}) {
		t.Errorf("reason codes/disclosures/excluded = %v/%v/%v, want them carried through verbatim",
			resp.GetQualityReasonCodes(), resp.GetDisclosures(), resp.GetExcludedMarketCodes())
	}

	lineage := resp.GetLineage()
	if lineage.GetArtifactId() != "direct-ridge-v2-aa66e30432a2729a" || lineage.GetModelVersion() != "direct-ridge-v2-aa66e30432a2729a" {
		t.Errorf("lineage artifact/model = %q/%q, want the artifact's own content-addressed version", lineage.GetArtifactId(), lineage.GetModelVersion())
	}
	if lineage.GetDatasetDigest() != "sha256:"+repeatHexTestGrpc('a') || lineage.GetBoundaryDigest() != "sha256:"+repeatHexTestGrpc('b') {
		t.Errorf("lineage digests = %q/%q, want the sha256:-prefixed pair", lineage.GetDatasetDigest(), lineage.GetBoundaryDigest())
	}
	// Times are RFC3339 UTC "...Z", this repo's time-format redline — the
	// same rule toPricebookDataMeta already follows.
	if lineage.GetGeneratedAt() != "2026-08-08T03:00:00Z" ||
		lineage.GetSourceDataAsOf() != "2026-08-07T16:00:00Z" ||
		lineage.GetMaxFeatureAvailableAt() != "2026-08-08T02:30:00Z" {
		t.Errorf("lineage times = %q/%q/%q, want RFC3339 UTC",
			lineage.GetGeneratedAt(), lineage.GetSourceDataAsOf(), lineage.GetMaxFeatureAvailableAt())
	}
	if lineage.GetSchemaVersion() != "hawkr.pricebook/v1" {
		t.Errorf("lineage.schema_version = %q, want hawkr.pricebook/v1", lineage.GetSchemaVersion())
	}

	boundary := resp.GetBoundary()
	if boundary.GetVersion() != "taiwan-atlas-2021.9.20" || boundary.GetDigest() != "sha256:"+repeatHexTestGrpc('c') {
		t.Errorf("boundary version/digest = %q/%q, want taiwan-atlas-2021.9.20/sha256:%s", boundary.GetVersion(), boundary.GetDigest(), repeatHexTestGrpc('c'))
	}
	if !reflect.DeepEqual(boundary.GetBboxWgs84(), []float64{118.1, 21.8, 122.0, 26.4}) {
		t.Errorf("boundary.bbox_wgs84 = %v, want [118.1 21.8 122.0 26.4]", boundary.GetBboxWgs84())
	}
	if boundary.GetGeojson() == nil {
		t.Fatal("boundary.geojson must be set")
	}
	features := boundary.GetGeojson().GetFields()["features"].GetListValue().GetValues()
	if len(features) != 1 {
		t.Fatalf("boundary.geojson features = %d, want 1", len(features))
	}

	if resp.GetSupportedLandAreaRatio() != 0.42 {
		t.Errorf("supported_land_area_ratio = %v, want 0.42", resp.GetSupportedLandAreaRatio())
	}

	if first.GetCategoryCode() != "N04" {
		t.Errorf("market_prices[0].category_code = %q, want N04", first.GetCategoryCode())
	}
	if closed.GetExclusionReason() != "no_usable_price" {
		t.Errorf("market_prices[1].exclusion_reason = %q, want no_usable_price", closed.GetExclusionReason())
	}
}

// TestGetSpatialSurface_NoLineageTimestamps_LeavesOptionalTimesUnset guards
// the one thing a naive time formatter gets wrong: a nil *time.Time must
// stay unset, not become "0001-01-01T00:00:00Z".
func TestGetSpatialSurface_NoLineageTimestamps_LeavesOptionalTimesUnset(t *testing.T) {
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		return usecase.SpatialSurfaceOutput{Status: "unavailable", DisplayMode: "table_only"}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	resp, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetLineage() == nil {
		t.Fatal("lineage = nil, want a present (if mostly empty) provenance block")
	}
	if resp.GetLineage().SourceDataAsOf != nil || resp.GetLineage().MaxFeatureAvailableAt != nil {
		t.Errorf("lineage optional times = %+v, want both unset, never a zero-time string", resp.GetLineage())
	}
	if resp.GetLineage().GeneratedAt != nil || resp.GetLineage().SchemaVersion != nil {
		t.Errorf("lineage generated_at/schema_version = %+v, want both unset on a zero-valued usecase.SpatialLineage, "+
			"never a fabricated \"0001-01-01T00:00:00Z\" or an empty-string schema_version", resp.GetLineage())
	}
}

// TestGetSpatialSurface_EmptyCategoryCode_IsOmittedNotEmptyString
// pins the exact bug PR C's design doc found: before category_code was
// optional, an unset value serialized to "" under EmitUnpopulated, which
// the website's fail-closed optionalStringAt() throws on (it only tolerates
// null/undefined, not ""). After the proto3 `optional` keyword, protojson's
// unpopulatedFieldRanger unconditionally skips any field with a
// ContainingOneof (proto3 `optional` compiles to a synthetic one-field
// oneof) regardless of EmitUnpopulated/EmitDefaultValues — verified against
// the pinned google.golang.org/protobuf v1.36.11 — so an unset field is
// *omitted* from the JSON entirely (key absent), never null. Either shape
// (absent or null) satisfies "must not be the empty string"; this test pins
// the actually-observed shape rather than the one the design doc assumed.
func TestGetSpatialSurface_EmptyCategoryCode_IsOmittedNotEmptyString(t *testing.T) {
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		return usecase.SpatialSurfaceOutput{
			Status: "ready", DisplayMode: "table_only", SelectionKind: "crop", SelectionCode: "A1",
			TargetDate:  forecast.DateFromTime(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)),
			HorizonDays: 1, Currency: "TWD", Unit: "kg",
			Scale: spatial.PriceScale{ScaleID: "x", PolicyVersion: "v1", Breaks: []float64{1, 2}},
			MarketPrices: []usecase.SpatialMarketPrice{
				{MarketCode: "109", MarketName: "台北一", PredictionStatus: "market_closed", MarketStatus: "closed"},
			},
			Lineage: usecase.SpatialLineage{GeneratedAt: time.Date(2026, 8, 8, 3, 0, 0, 0, time.UTC), SchemaVersion: "hawkr.pricebook/v1"},
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	resp, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	marketPrices := decoded["marketPrices"].([]any)
	mp := marketPrices[0].(map[string]any)
	if v, ok := mp["categoryCode"]; ok && v != nil {
		t.Errorf("marketPrices[0].categoryCode = %v (present=%v), want absent or null — never the empty string", v, ok)
	}
	if v, ok := mp["exclusionReason"]; ok && v != nil {
		t.Errorf("marketPrices[0].exclusionReason = %v (present=%v), want absent or null — never the empty string", v, ok)
	}
	lineage := decoded["lineage"].(map[string]any)
	if lineage["schemaVersion"] != "hawkr.pricebook/v1" {
		t.Errorf("lineage.schemaVersion = %v, want hawkr.pricebook/v1 (this one IS set on this fixture)", lineage["schemaVersion"])
	}
}

// TestGetSpatialSurface_SelectionAndHorizon_ReachTheUsecase pins the
// request→input translation, including the reason the handler rebuilds the
// pointers instead of aliasing req's: a set-but-empty crop_code is "unset"
// to validateCropResolutionRequest, so it must not reach the usecase as a
// non-nil pointer to "".
func TestGetSpatialSurface_SelectionAndHorizon_ReachTheUsecase(t *testing.T) {
	var got usecase.SpatialSurfaceInput
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		got = in
		return usecase.SpatialSurfaceOutput{}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	emptyCrop, groupCode := "", "N04"
	if _, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{
		CropCode: &emptyCrop, GroupCode: &groupCode, HorizonDays: 7,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CropCode != nil {
		t.Errorf("CropCode = %q, want nil (an empty crop_code is unset, not a crop named \"\")", *got.CropCode)
	}
	if got.GroupCode == nil || *got.GroupCode != "N04" {
		t.Errorf("GroupCode = %v, want N04", got.GroupCode)
	}
	if got.HorizonDays != 7 {
		t.Errorf("HorizonDays = %d, want 7", got.HorizonDays)
	}
	if got.Now.IsZero() {
		t.Error("Now is zero — the usecase rejects a zero time, the handler must supply time.Now()")
	}
}

func TestGetSpatialSurface_SelectionValidation_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	cropCode, groupCode := "A1", "N04"
	for name, req := range map[string]*pb.GetSpatialSurfaceRequest{
		"neither": {HorizonDays: 1},
		"both":    {CropCode: &cropCode, GroupCode: &groupCode, HorizonDays: 1},
	} {
		_, err := s.GetSpatialSurface(context.Background(), req)
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("%s: err = %v, want gRPC InvalidArgument", name, err)
		}
	}
}

func TestGetSpatialSurface_HorizonOutOfRange_ReturnsInvalidArgument(t *testing.T) {
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	for _, horizon := range []int32{-1, 0, 8, 100} {
		_, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: horizon})
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("horizon %d: err = %v, want gRPC InvalidArgument", horizon, err)
		}
	}
}

// TestGetSpatialSurface_UsecaseErrors_MapToCodes covers both halves of the
// error contract: the usecase's two request-shape sentinels stay
// InvalidArgument even when they surface from the usecase rather than the
// handler's own eager checks, and everything else (an infrastructure
// failure) is Internal — never a fake 200 with an empty surface.
func TestGetSpatialSurface_UsecaseErrors_MapToCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want codes.Code
	}{
		"exactly one selection": {usecase.ErrSpatialExactlyOneSelection, codes.InvalidArgument},
		"horizon out of range":  {usecase.ErrSpatialHorizonOutOfRange, codes.InvalidArgument},
		"infrastructure":        {errors.New("query crops in group: connection refused"), codes.Internal},
		"zero now":              {usecase.ErrSpatialZeroNow, codes.Internal},
	} {
		uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
			return usecase.SpatialSurfaceOutput{}, tc.err
		}}
		s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

		cropCode := "A1"
		_, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 1})
		st, ok := status.FromError(err)
		if !ok || st.Code() != tc.want {
			t.Errorf("%s: err = %v, want gRPC %v", name, err, tc.want)
		}
	}
}

// TestGetSpatialSurface_UnsupportedGeometry_ReturnsInternal pins the
// "invalid geometry must never reach the browser" half of issue #2248's
// geometry contract: a county whose geometry is not Polygon/MultiPolygon
// fails the call rather than shipping a feature the client cannot render.
func TestGetSpatialSurface_UnsupportedGeometry_ReturnsInternal(t *testing.T) {
	uc := &fakeSpatialSurfaceUC{get: func(ctx context.Context, in usecase.SpatialSurfaceInput) (usecase.SpatialSurfaceOutput, error) {
		return usecase.SpatialSurfaceOutput{
			CountyRollups: []usecase.SpatialCountyRollup{{CountyCode: "63000", Geometry: orb.Point{121.5, 25.0}}},
		}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, uc, nil, nil, nil, nil, nil, nil, nil)

	cropCode := "A1"
	_, err := s.GetSpatialSurface(context.Background(), &pb.GetSpatialSurfaceRequest{CropCode: &cropCode, HorizonDays: 1})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("err = %v, want gRPC Internal", err)
	}
}

// float64Ptr is a fixture-building helper for the *float64 optional wire
// fields (supported_land_area_ratio, p10/p50/p90, ...) — Go has no address-
// of-literal syntax, so tests otherwise need a throwaway named variable.
func float64Ptr(v float64) *float64 { return &v }

// repeatHexTestGrpc is this package's own tiny copy of the same "64
// repeated hex chars" fixture builder used in the forecast/usecase test
// packages — each package tests its own layer independently, so this isn't
// reused across package boundaries.
func repeatHexTestGrpc(r byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = r
	}
	return string(b)
}

type fakeRegionalComparisonUC struct {
	get func(ctx context.Context, in usecase.RegionalComparisonInput) (usecase.RegionalComparisonOutput, error)
}

func (f *fakeRegionalComparisonUC) Get(ctx context.Context, in usecase.RegionalComparisonInput) (usecase.RegionalComparisonOutput, error) {
	return f.get(ctx, in)
}

func TestGetRegionalComparison_SelectionValidation_ReturnsInvalidArgument(t *testing.T) {
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, &fakeRegionalComparisonUC{}, nil, nil, nil, nil, nil, nil)
	_, err := server.GetRegionalComparison(context.Background(), &pb.GetRegionalComparisonRequest{HorizonDays: 1})
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetRegionalComparison_HorizonOutOfRange_ReturnsInvalidArgument(t *testing.T) {
	cropCode := "A1"
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, &fakeRegionalComparisonUC{}, nil, nil, nil, nil, nil, nil)
	_, err := server.GetRegionalComparison(context.Background(), &pb.GetRegionalComparisonRequest{CropCode: &cropCode, HorizonDays: 8})
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetRegionalComparison_MapsRegionsAndLineage(t *testing.T) {
	cropCode := "A1"
	fake := &fakeRegionalComparisonUC{get: func(ctx context.Context, in usecase.RegionalComparisonInput) (usecase.RegionalComparisonOutput, error) {
		return usecase.RegionalComparisonOutput{
			SelectionKind: "crop", HorizonDays: 1, Currency: "TWD", Unit: "kg",
			Regions: []usecase.RegionalComparisonRegion{
				{RegionCode: "63000", RegionName: "台北市", Status: "ready",
					P10: float64Ptr(30), P50: float64Ptr(40), P90: float64Ptr(50),
					SupportingMarketCount: 2, AggregationMethod: "market_volume_weighted_quantile_average/v1",
					FreshnessStatus: "fresh"},
				{RegionCode: "64000", RegionName: "高雄市", Status: "unavailable", QualityReasonCodes: []string{"no_markets_in_region"},
					FreshnessStatus: "unavailable"},
			},
			Lineage:        usecase.SpatialLineage{ArtifactID: "test-v1", ModelVersion: "test-v1"},
			MappingVersion: "region-mapping.v1",
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, fake, nil, nil, nil, nil, nil, nil)
	resp, err := server.GetRegionalComparison(context.Background(), &pb.GetRegionalComparisonRequest{CropCode: &cropCode, HorizonDays: 1})
	require.NoError(t, err)
	require.Equal(t, "crop", resp.SelectionKind)
	require.Equal(t, int32(1), resp.HorizonDays)
	require.Equal(t, "TWD", resp.Currency)
	require.Equal(t, "kg", resp.Unit)
	require.Len(t, resp.Regions, 2)

	require.Equal(t, "63000", resp.Regions[0].RegionCode)
	require.Equal(t, "台北市", resp.Regions[0].RegionName)
	require.Equal(t, "ready", resp.Regions[0].Status)
	require.Equal(t, 30.0, resp.Regions[0].GetP10())
	require.Equal(t, 40.0, resp.Regions[0].GetP50())
	require.Equal(t, 50.0, resp.Regions[0].GetP90())
	require.Equal(t, int32(2), resp.Regions[0].SupportingMarketCount)
	require.Equal(t, "market_volume_weighted_quantile_average/v1", resp.Regions[0].GetAggregationMethod())
	require.Empty(t, resp.Regions[0].ExcludedMarketCodes)
	require.Equal(t, "fresh", resp.Regions[0].FreshnessStatus)

	require.Equal(t, "64000", resp.Regions[1].RegionCode)
	require.Equal(t, "高雄市", resp.Regions[1].RegionName)
	require.Equal(t, "unavailable", resp.Regions[1].Status)
	require.Nil(t, resp.Regions[1].P50)
	require.Nil(t, resp.Regions[1].AggregationMethod)
	require.Equal(t, []string{"no_markets_in_region"}, resp.Regions[1].QualityReasonCodes)
	require.Empty(t, resp.Regions[1].ExcludedMarketCodes)
	require.Equal(t, "unavailable", resp.Regions[1].FreshnessStatus)

	require.Equal(t, "test-v1", resp.Lineage.ArtifactId)

	if resp.GetMappingVersion() != "region-mapping.v1" {
		t.Errorf("MappingVersion = %q, want %q", resp.GetMappingVersion(), "region-mapping.v1")
	}
}

// ============================================================================
// categoryId 權威解析三支 RPC（issue #2786）的 handler 層測試。
//
// 這三支的 usecase 各自已有測試，但只存在於 handler 的邏輯——worklist 的
// status 篩選與 caller-contract 檢查、updated_by 的 int64→string 轉換、
// updated_at 的 RFC3339 格式化、sentinel error→gRPC status code 映射——在任何
// 一層都沒有被覆蓋過。
// ============================================================================

type fakeResolveCategoryMappingUC struct {
	get func(ctx context.Context, categoryID string) (domain.CategoryMapping, error)
}

func (f *fakeResolveCategoryMappingUC) Get(ctx context.Context, categoryID string) (domain.CategoryMapping, error) {
	return f.get(ctx, categoryID)
}

type fakeUpsertCategoryMappingUC struct {
	do func(ctx context.Context, in usecase.UpsertCategoryMappingInput) (domain.CategoryMapping, error)
}

func (f *fakeUpsertCategoryMappingUC) Do(ctx context.Context, in usecase.UpsertCategoryMappingInput) (domain.CategoryMapping, error) {
	return f.do(ctx, in)
}

type fakeListCategoryMappingWorklistUC struct {
	do func(ctx context.Context) ([]domain.WorklistItem, error)
}

func (f *fakeListCategoryMappingWorklistUC) Do(ctx context.Context) ([]domain.WorklistItem, error) {
	return f.do(ctx)
}

// TestListCategoryMappingWorklist_UpdatedByInt64ConvertedToString 鎖住
// domain 的 *int64 → proto 的 *string 轉換。用一個超過 IEEE-754 雙精度整數安全
// 範圍（2^53）的 accountId：若哪天有人把它改成走 float64 中轉，字串會失真，
// 這個斷言會抓到。
func TestListCategoryMappingWorklist_UpdatedByInt64ConvertedToString(t *testing.T) {
	const bigAccountID int64 = 9007199254740993 // 2^53 + 1
	updatedAt := time.Date(2026, 8, 26, 4, 5, 6, 0, time.UTC)
	fake := &fakeListCategoryMappingWorklistUC{do: func(ctx context.Context) ([]domain.WorklistItem, error) {
		return []domain.WorklistItem{
			{CategoryID: "cat-1", CategoryName: "有紀錄", Status: domain.CategoryMappingAmbiguous, UpdatedBy: ptrInt64(bigAccountID), UpdatedAt: &updatedAt},
			{CategoryID: "cat-2", CategoryName: "查無列", Status: domain.CategoryMappingUnmapped},
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, fake, nil, nil, nil)

	resp, err := server.ListCategoryMappingWorklist(context.Background(), &pb.ListCategoryMappingWorklistRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	require.Equal(t, "9007199254740993", resp.Items[0].GetUpdatedBy())
	require.Equal(t, "cat-1", resp.Items[0].CategoryId)
	require.Equal(t, "有紀錄", resp.Items[0].CategoryName)
	require.Equal(t, "ambiguous", resp.Items[0].Status)
	require.Nil(t, resp.Items[1].UpdatedBy, "查無列的 categoryId 沒有操作者，必須保持 nil 而不是空字串或 \"0\"")
	require.Nil(t, resp.Items[1].UpdatedAt)
}

// TestListCategoryMappingWorklist_UpdatedAtFormattedAsUTCRFC3339 鎖住時間格式
// 化：API 規範要求 ISO 8601 UTC，非 UTC 的來源時間必須先轉 UTC 再格式化，不能
// 讓 server local timezone 洩漏到對外契約。
func TestListCategoryMappingWorklist_UpdatedAtFormattedAsUTCRFC3339(t *testing.T) {
	taipei := time.FixedZone("CST", 8*3600)
	updatedAt := time.Date(2026, 8, 26, 12, 0, 0, 0, taipei) // = 2026-08-26T04:00:00Z
	fake := &fakeListCategoryMappingWorklistUC{do: func(ctx context.Context) ([]domain.WorklistItem, error) {
		return []domain.WorklistItem{
			{CategoryID: "cat-1", Status: domain.CategoryMappingUnmapped, UpdatedBy: ptrInt64(1), UpdatedAt: &updatedAt},
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, fake, nil, nil, nil)

	resp, err := server.ListCategoryMappingWorklist(context.Background(), &pb.ListCategoryMappingWorklistRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "2026-08-26T04:00:00Z", resp.Items[0].GetUpdatedAt())
}

// TestListCategoryMappingWorklist_StatusFilter_OnlyMatchingItems 證明 status
// 篩選真的過濾（這段邏輯只存在於 handler，usecase 不知道有篩選這回事）。
func TestListCategoryMappingWorklist_StatusFilter_OnlyMatchingItems(t *testing.T) {
	fake := &fakeListCategoryMappingWorklistUC{do: func(ctx context.Context) ([]domain.WorklistItem, error) {
		return []domain.WorklistItem{
			{CategoryID: "cat-1", Status: domain.CategoryMappingUnmapped},
			{CategoryID: "cat-2", Status: domain.CategoryMappingAmbiguous},
			{CategoryID: "cat-3", Status: domain.CategoryMappingUnmapped},
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, fake, nil, nil, nil)

	resp, err := server.ListCategoryMappingWorklist(context.Background(), &pb.ListCategoryMappingWorklistRequest{Status: "ambiguous"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "cat-2", resp.Items[0].CategoryId)

	all, err := server.ListCategoryMappingWorklist(context.Background(), &pb.ListCategoryMappingWorklistRequest{})
	require.NoError(t, err)
	require.Len(t, all.Items, 3, "空字串＝不篩選")
}

// TestListCategoryMappingWorklist_InvalidStatus_ReturnsInvalidArgument：proto
// 契約寫明 status 只能是空字串｜unmapped｜ambiguous。未驗證前，resolved／垃圾字串
// 會靜默回空清單——一個看起來成功的錯誤答案。
func TestListCategoryMappingWorklist_InvalidStatus_ReturnsInvalidArgument(t *testing.T) {
	for _, bad := range []string{"resolved", "xxx", "UNMAPPED"} {
		t.Run(bad, func(t *testing.T) {
			fake := &fakeListCategoryMappingWorklistUC{do: func(ctx context.Context) ([]domain.WorklistItem, error) {
				t.Fatal("caller-contract 違規必須在呼叫 usecase 之前就被擋下")
				return nil, nil
			}}
			server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, fake, nil, nil, nil)

			_, err := server.ListCategoryMappingWorklist(context.Background(), &pb.ListCategoryMappingWorklistRequest{Status: bad})
			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, codes.InvalidArgument, st.Code())
		})
	}
}

// TestUpsertCategoryMapping_SentinelErrors_MappedToStatusCodes 鎖住四個
// sentinel error 各自的 gRPC status code。ErrIncompleteSelection 是這次新增
// 的：沒有它，形狀錯誤的輸入會撞 DB CHECK constraint、以 Internal 回傳，最後
// 在 Gateway 變成 HTTP 503「服務不可用」。
func TestUpsertCategoryMapping_SentinelErrors_MappedToStatusCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"ErrCategoryNotFound", usecase.ErrCategoryNotFound, codes.NotFound},
		{"ErrCategoryInactive", usecase.ErrCategoryInactive, codes.FailedPrecondition},
		{"ErrTargetCodeInactive", usecase.ErrTargetCodeInactive, codes.InvalidArgument},
		{"ErrIncompleteSelection", usecase.ErrIncompleteSelection, codes.InvalidArgument},
		{"unexpected", errors.New("boom"), codes.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeUpsertCategoryMappingUC{do: func(ctx context.Context, in usecase.UpsertCategoryMappingInput) (domain.CategoryMapping, error) {
				return domain.CategoryMapping{}, tc.err
			}}
			server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, fake, nil, nil, nil, nil)

			_, err := server.UpsertCategoryMapping(context.Background(), &pb.UpsertCategoryMappingRequest{CategoryId: "cat-1", Status: "unmapped"})
			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, tc.want, st.Code())
		})
	}
}

// TestUpsertCategoryMapping_Success_FormatsUpdatedAtAsUTCRFC3339 涵蓋成功回應
// 的時間格式化（與 worklist 是各自獨立的一段程式碼）。
func TestUpsertCategoryMapping_Success_FormatsUpdatedAtAsUTCRFC3339(t *testing.T) {
	taipei := time.FixedZone("CST", 8*3600)
	var gotIn usecase.UpsertCategoryMappingInput
	fake := &fakeUpsertCategoryMappingUC{do: func(ctx context.Context, in usecase.UpsertCategoryMappingInput) (domain.CategoryMapping, error) {
		gotIn = in
		return domain.CategoryMapping{
			CategoryID: in.CategoryID, Status: in.Status, MappingVersion: "pricebook-category-v1",
			UpdatedAt: time.Date(2026, 8, 26, 12, 30, 0, 0, taipei),
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, fake, nil, nil, nil, nil)

	groupCode := "G1"
	resp, err := server.UpsertCategoryMapping(context.Background(), &pb.UpsertCategoryMappingRequest{
		CategoryId: "cat-1", Status: "resolved", GroupCode: &groupCode, UpdatedBy: 42,
	})
	require.NoError(t, err)
	require.Equal(t, "cat-1", resp.CategoryId)
	require.Equal(t, "resolved", resp.Status)
	require.Equal(t, "pricebook-category-v1", resp.MappingVersion)
	require.Equal(t, "2026-08-26T04:30:00Z", resp.UpdatedAt)

	require.Equal(t, domain.CategoryMappingResolved, gotIn.Status)
	require.NotNil(t, gotIn.GroupCode)
	require.Equal(t, "G1", *gotIn.GroupCode)
	require.Equal(t, int64(42), gotIn.UpdatedBy)
}

// TestResolveCategoryMapping_PassesThroughOptionalFields 涵蓋 handler 的
// domain→proto 對應（含 optional 欄位的 nil 傳遞）。
func TestResolveCategoryMapping_PassesThroughOptionalFields(t *testing.T) {
	groupCode, label := "G1", "葉菜類"
	fake := &fakeResolveCategoryMappingUC{get: func(ctx context.Context, categoryID string) (domain.CategoryMapping, error) {
		return domain.CategoryMapping{
			CategoryID: categoryID, Status: domain.CategoryMappingResolved,
			GroupCode: &groupCode, Label: &label,
			MappingVersion: "pricebook-category-v1",
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, fake, nil, nil, nil, nil, nil)

	resp, err := server.ResolveCategoryMapping(context.Background(), &pb.ResolveCategoryMappingRequest{CategoryId: "cat-1"})
	require.NoError(t, err)
	require.Equal(t, "cat-1", resp.CategoryId)
	require.Equal(t, "resolved", resp.Status)
	require.Equal(t, "G1", resp.GetGroupCode())
	require.Equal(t, "葉菜類", resp.GetLabel())
	require.Equal(t, "pricebook-category-v1", resp.MappingVersion)
	require.Nil(t, resp.CommodityCode, "group 映射不該帶 commodity_code——兩者互斥")
}

func TestResolveCategoryMapping_CommodityMappingCarriesCommodityCode(t *testing.T) {
	commodityID, label := int64(958000000000000001), "毛豬（規格豬）"
	fake := &fakeResolveCategoryMappingUC{get: func(ctx context.Context, categoryID string) (domain.CategoryMapping, error) {
		return domain.CategoryMapping{
			CategoryID: categoryID, Status: domain.CategoryMappingResolved,
			CommodityID: &commodityID, Label: &label,
			MappingVersion: "pricebook-category-v1",
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, fake, nil, nil, nil, nil, nil)

	resp, err := server.ResolveCategoryMapping(context.Background(), &pb.ResolveCategoryMappingRequest{CategoryId: "2008"})
	require.NoError(t, err)
	require.Equal(t, "resolved", resp.Status)
	require.Equal(t, "958000000000000001", resp.GetCommodityCode())
	require.Nil(t, resp.GroupCode, "commodity 映射不該帶 group_code——兩者互斥")
	require.Equal(t, "毛豬（規格豬）", resp.GetLabel())
}

func ptrInt64(v int64) *int64 { return &v }

// ============================================================================
// GetCategoryPriceRollup（Task 10）：categoryId 分級彙總價格讀端點 handler
// 層測試。usecase 已有自己的測試（純 passthrough），這裡涵蓋只存在於 handler
// 的邏輯——nil row → unavailable 的折疊、LatestPrice/PrevPrice/Points 的
// float64→string 格式化（避免 JS 精度問題）。
// ============================================================================

type fakeGetCategoryPriceRollupUC struct {
	get func(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error)
}

func (f *fakeGetCategoryPriceRollupUC) Get(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
	return f.get(ctx, categoryID)
}

// TestGetCategoryPriceRollup_ReadyRow_FormatsPricesAsRoundTrippableStrings
// 鎖住 float64→string 格式化：LatestPrice/PrevPrice/Points 的 AvgPrice 都必須
// 能用 strconv.ParseFloat 還原成原始數值，不能只是「看起來像數字」。
func TestGetCategoryPriceRollup_ReadyRow_FormatsPricesAsRoundTrippableStrings(t *testing.T) {
	latest, prev := 42.5, 40.25
	computedAt := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	fake := &fakeGetCategoryPriceRollupUC{get: func(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
		return &domain.CategoryPriceRollup{
			CategoryID: categoryID, Level: 2, Status: domain.RollupStatusReady,
			LatestPrice: &latest, PrevPrice: &prev,
			Points:           []domain.RollupPricePoint{{TradeDate: "2026-08-29", AvgPrice: 41.75}},
			SourceGroupCount: 3, ComputedAt: computedAt,
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, nil, fake, nil, nil)

	resp, err := server.GetCategoryPriceRollup(context.Background(), &pb.GetCategoryPriceRollupRequest{CategoryId: "cat-l2"})
	require.NoError(t, err)
	require.Equal(t, "cat-l2", resp.CategoryId)
	require.Equal(t, int32(2), resp.Level)
	require.Equal(t, "ready", resp.Status)
	require.Equal(t, int32(3), resp.SourceGroupCount)
	require.Equal(t, "2026-08-30T01:00:00Z", resp.ComputedAt)

	gotLatest, err := strconv.ParseFloat(resp.GetLatestPrice(), 64)
	require.NoError(t, err)
	require.Equal(t, latest, gotLatest)

	gotPrev, err := strconv.ParseFloat(resp.GetPrevPrice(), 64)
	require.NoError(t, err)
	require.Equal(t, prev, gotPrev)

	require.Len(t, resp.Points, 1)
	require.Equal(t, "2026-08-29", resp.Points[0].TradeDate)
	gotPoint, err := strconv.ParseFloat(resp.Points[0].AvgPrice, 64)
	require.NoError(t, err)
	require.Equal(t, 41.75, gotPoint)
}

// TestGetCategoryPriceRollup_NoRow_FoldsToUnavailable 鎖住 usecase 回傳
// nil, nil（批次還沒跑過這個 category_id）時，handler 必須折疊成跟「已計算但
// unavailable」相同的成功回應形狀，不是 404/error，且不能有任何價格欄位被設值。
func TestGetCategoryPriceRollup_NoRow_FoldsToUnavailable(t *testing.T) {
	fake := &fakeGetCategoryPriceRollupUC{get: func(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
		return nil, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, nil, fake, nil, nil)

	resp, err := server.GetCategoryPriceRollup(context.Background(), &pb.GetCategoryPriceRollupRequest{CategoryId: "cat-missing"})
	require.NoError(t, err)
	require.Equal(t, "cat-missing", resp.CategoryId)
	require.Equal(t, "unavailable", resp.Status)
	require.Nil(t, resp.LatestPrice)
	require.Nil(t, resp.PrevPrice)
	require.Empty(t, resp.Points)
	require.Equal(t, int32(0), resp.SourceGroupCount)
}

// TestGetCategoryPriceRollup_ForecastPoints_MapsToProtoWithOptionalStrings
// 鎖住 forecast_points 的映射：HorizonDays/TargetDate 直接搬，P10/P50/P90 各自
// float64→string 格式化（比照 LatestPrice/PrevPrice 既有慣例），且 nil 值必須
// 讓對應欄位在 wire 上整個 unset（optional string 沒設值），不是空字串。
func TestGetCategoryPriceRollup_ForecastPoints_MapsToProtoWithOptionalStrings(t *testing.T) {
	p50, p90 := 43.125, 45.5
	computedAt := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	fake := &fakeGetCategoryPriceRollupUC{get: func(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
		return &domain.CategoryPriceRollup{
			CategoryID: categoryID, Level: 2, Status: domain.RollupStatusReady,
			ForecastPoints: []domain.RollupForecastPoint{
				{HorizonDays: 1, TargetDate: "2026-09-01", P10: nil, P50: &p50, P90: &p90},
			},
			SourceGroupCount: 3, ComputedAt: computedAt,
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, nil, fake, nil, nil)

	resp, err := server.GetCategoryPriceRollup(context.Background(), &pb.GetCategoryPriceRollupRequest{CategoryId: "cat-l2"})
	require.NoError(t, err)
	require.Len(t, resp.ForecastPoints, 1)

	fp := resp.ForecastPoints[0]
	require.Equal(t, int32(1), fp.HorizonDays)
	require.Equal(t, "2026-09-01", fp.TargetDate)
	require.Nil(t, fp.P10, "P10 為 nil 時整個欄位必須 unset，不是空字串")

	gotP50, err := strconv.ParseFloat(fp.GetP50(), 64)
	require.NoError(t, err)
	require.Equal(t, p50, gotP50)

	gotP90, err := strconv.ParseFloat(fp.GetP90(), 64)
	require.NoError(t, err)
	require.Equal(t, p90, gotP90)
}

func TestListGroupCropOptions_PassesThroughAndMapsCropFields(t *testing.T) {
	cropCode, cropName, variant := "LA1", "甘藍-初秋", "初秋"
	var seenGroup string
	var seenLimit int32
	uc := &fakeSearchCropCatalogUC{listGroupCropOptions: func(ctx context.Context, groupCode string, limit int32) ([]usecase.CatalogMatch, error) {
		seenGroup, seenLimit = groupCode, limit
		return []usecase.CatalogMatch{{
			MatchType: "crop", GroupCode: "g-cabbage", GroupName: "甘藍",
			CropCode: &cropCode, CropName: &cropName, VariantName: &variant, HasForecast: true,
			Markets: []usecase.CatalogMarketOption{{MarketCode: "109", MarketName: "台北一"}},
		}}, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, uc, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.ListGroupCropOptions(context.Background(), &pb.ListGroupCropOptionsRequest{GroupCode: "g-cabbage", Limit: 6})

	if err != nil {
		t.Fatalf("ListGroupCropOptions: unexpected error %v", err)
	}
	if seenGroup != "g-cabbage" || seenLimit != 6 {
		t.Fatalf("usecase got (%q, %d), want (g-cabbage, 6)", seenGroup, seenLimit)
	}
	if len(resp.Crops) != 1 {
		t.Fatalf("len(resp.Crops) = %d, want 1", len(resp.Crops))
	}
	got := resp.Crops[0]
	if got.GetCropCode() != "LA1" || got.GetCropName() != "甘藍-初秋" || got.GetVariantName() != "初秋" {
		t.Errorf("crop fields = %q/%q/%q, want LA1/甘藍-初秋/初秋", got.GetCropCode(), got.GetCropName(), got.GetVariantName())
	}
	if got.MatchType != "crop" || got.GroupCode != "g-cabbage" {
		t.Errorf("match = %q/%q, want crop/g-cabbage", got.MatchType, got.GroupCode)
	}
	if len(got.Markets) != 1 {
		t.Errorf("len(Markets) = %d, want 1——crop 型結果必須帶市場清單", len(got.Markets))
	}
}

// 空 group_code 不是 INVALID_ARGUMENT：proto 與 usecase 都把「查無大類」定為
// 業務結果。少了這支，日後有人「順手」補一道參數檢查就不會被擋下來。
func TestListGroupCropOptions_EmptyGroupCodeIsNotInvalidArgument(t *testing.T) {
	uc := &fakeSearchCropCatalogUC{listGroupCropOptions: func(ctx context.Context, groupCode string, limit int32) ([]usecase.CatalogMatch, error) {
		return nil, nil
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, uc, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.ListGroupCropOptions(context.Background(), &pb.ListGroupCropOptionsRequest{})

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(resp.Crops) != 0 {
		t.Fatalf("len(resp.Crops) = %d, want 0", len(resp.Crops))
	}
}

func TestListGroupCropOptions_UsecaseErrorMapsToInternal(t *testing.T) {
	uc := &fakeSearchCropCatalogUC{listGroupCropOptions: func(ctx context.Context, groupCode string, limit int32) ([]usecase.CatalogMatch, error) {
		return nil, errors.New("boom")
	}}
	s := NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, uc, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, nil)

	_, err := s.ListGroupCropOptions(context.Background(), &pb.ListGroupCropOptionsRequest{GroupCode: "g-cabbage"})

	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("code = %v, want Internal", st.Code())
	}
}

func TestGetCategoryPriceRollup_ReadyRow_CarriesPriceLayerAndUnit(t *testing.T) {
	latest := 105.42
	layer := domain.PriceLayerLiveAnimal
	unit := domain.UnitKilogram
	fake := &fakeGetCategoryPriceRollupUC{get: func(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
		return &domain.CategoryPriceRollup{
			CategoryID: categoryID, Level: 2, Status: domain.RollupStatusReady,
			LatestPrice: &latest, SourceGroupCount: 1,
			PriceLayer: &layer, Unit: &unit,
			ComputedAt: time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC),
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, nil, fake, nil, nil)

	resp, err := server.GetCategoryPriceRollup(context.Background(), &pb.GetCategoryPriceRollupRequest{CategoryId: "198"})
	require.NoError(t, err)
	require.Equal(t, "live_animal", resp.GetPriceLayer())
	require.Equal(t, "kg", resp.GetUnit())
}

func TestGetCategoryPriceRollup_MixedLayer_HasNeitherPriceNorLayer(t *testing.T) {
	fake := &fakeGetCategoryPriceRollupUC{get: func(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
		return &domain.CategoryPriceRollup{
			CategoryID: categoryID, Level: 1, Status: domain.RollupStatusMixedLayer,
			SourceGroupCount: 5,
			ComputedAt:       time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC),
		}, nil
	}}
	server := NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, nil, fake, nil, nil)

	resp, err := server.GetCategoryPriceRollup(context.Background(), &pb.GetCategoryPriceRollupRequest{CategoryId: "4"})
	require.NoError(t, err)
	require.Equal(t, "mixed_layer", resp.Status)
	require.Nil(t, resp.LatestPrice)
	require.Nil(t, resp.PrevPrice)
	require.Nil(t, resp.PriceLayer)
	require.Nil(t, resp.Unit)
	require.Equal(t, int32(5), resp.SourceGroupCount)
}

type fakeCommodityForecastUC struct {
	result usecase.CommodityForecastResult
	err    error
}

func (f *fakeCommodityForecastUC) Handle(context.Context, string, time.Time) (usecase.CommodityForecastResult, error) {
	return f.result, f.err
}

func commodityServer(uc commodityForecastUC) *PricebookServer {
	return NewPricebookServer(nil, nil, nil, nil, mapping.Seed{}, nil, nil, nil, nil, nil, nil, nil, uc, nil)
}

func TestGetForecastCurve_SourcePassthrough_OmitsBandHorizonsAndModel(t *testing.T) {
	p50 := 106.03
	qty := 12189.0
	src := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	s := commodityServer(&fakeCommodityForecastUC{result: usecase.CommodityForecastResult{
		Name: "毛豬（規格豬）", PriceLayer: domain.PriceLayerLiveAnimal, Unit: domain.UnitKilogram,
		Status: forecast.PredictionStatusReady, Derivation: domain.ForecastDerivationSourcePassthrough,
		SourceDate: src, TargetDate: src.AddDate(0, 0, 1), P50: &p50,
		MarketCount: 19, QuantityUnit: "head", TotalQuantity: &qty,
	}})

	resp, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{
		CommodityCode: proto.String("700000000000000001"),
	})
	require.NoError(t, err)
	require.Len(t, resp.Points, 1, "只能有 horizon=1 一個點，不得複製成平線")
	pt := resp.Points[0]
	require.Equal(t, int32(1), pt.HorizonDays)
	require.NotNil(t, pt.P50)
	require.Nil(t, pt.P10, "來源只公布一個均價，不得宣稱區間")
	require.Nil(t, pt.P90)
	require.Equal(t, "source_passthrough", pt.GetDerivation())
	require.Empty(t, resp.ModelVersion, "沒有模型就不該有版本")

	require.Equal(t, "live_animal", resp.GetPriceLayer())
	require.Equal(t, "kg", resp.GetUnit())
	require.Equal(t, "2026-09-12", resp.GetSourceDataAsOf())
	require.Equal(t, "2026-09-13", pt.TargetDate, "來源日的隔日才是它被當成的參考日")
	require.Equal(t, "19", resp.GetSupportingMarketCount())
	require.Equal(t, "head", resp.GetQuantityUnit(), "毛豬的量是頭不是公斤")
}

func TestGetForecastCurve_SourcePassthrough_NoDataGivesNoPoint(t *testing.T) {
	s := commodityServer(&fakeCommodityForecastUC{result: usecase.CommodityForecastResult{
		Status: forecast.PredictionStatusInsufficientData,
	}})
	resp, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{
		CommodityCode: proto.String("700000000000000001"),
	})
	require.NoError(t, err)
	require.Empty(t, resp.Points)
	require.Equal(t, "insufficient_data", resp.Status)
}

func TestGetForecastCurve_CommodityCodeIsExclusiveWithCropTargets(t *testing.T) {
	s := commodityServer(&fakeCommodityForecastUC{})
	_, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{
		CommodityCode: proto.String("1"), CropCode: proto.String("FA1"),
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetForecastCurve_CropSeries_DeclaresBothCapabilities(t *testing.T) {
	s := commodityServer(&fakeCommodityForecastUC{})
	s.curveUC = &fakeForecastCurveUC{get: func(context.Context, string, string, string, string, time.Time) (usecase.ForecastCurveResult, error) {
		return usecase.ForecastCurveResult{Status: "ready"}, nil
	}}

	resp, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{
		CropCode: proto.String("LA1"),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Capabilities)
	require.True(t, resp.Capabilities.MarketComparison, "蔬果序列既有行為必須維持")
	require.True(t, resp.Capabilities.Spatial)
	require.Empty(t, resp.Capabilities.ReasonCodes)
}

func TestGetForecastCurve_NationalSeries_ReasonIsDataShapeNotMissingData(t *testing.T) {
	p50 := 70.83
	src := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	s := commodityServer(&fakeCommodityForecastUC{result: usecase.CommodityForecastResult{
		Status: forecast.PredictionStatusReady, Derivation: domain.ForecastDerivationSourcePassthrough,
		PriceLayer: domain.PriceLayerFarmgate, Unit: "catty", CanonicalUnit: domain.UnitKilogram,
		SourceDate: src, TargetDate: src.AddDate(0, 0, 1), P50: &p50, MarketCount: 1,
	}})

	resp, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{
		CommodityCode: proto.String("1"),
	})
	require.NoError(t, err)
	require.False(t, resp.Capabilities.MarketComparison)
	require.False(t, resp.Capabilities.Spatial)
	require.Contains(t, resp.Capabilities.ReasonCodes, "NATIONAL_SERIES")
	require.Contains(t, resp.Capabilities.ReasonCodes, "NO_APPROVED_GEOGRAPHY")
}

func TestGetForecastCurve_MultiMarketCommodity_UsesDifferentReason(t *testing.T) {
	p50 := 106.03
	src := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	s := commodityServer(&fakeCommodityForecastUC{result: usecase.CommodityForecastResult{
		Status: forecast.PredictionStatusReady, Derivation: domain.ForecastDerivationSourcePassthrough,
		PriceLayer: domain.PriceLayerLiveAnimal, Unit: domain.UnitKilogram,
		CanonicalUnit: domain.UnitKilogram,
		SourceDate:    src, TargetDate: src.AddDate(0, 0, 1), P50: &p50, MarketCount: 19,
	}})

	resp, err := s.GetForecastCurve(context.Background(), &pb.GetForecastCurveRequest{
		CommodityCode: proto.String("1"),
	})
	require.NoError(t, err)
	require.Contains(t, resp.Capabilities.ReasonCodes, "SINGLE_SERIES_ONLY")
	require.NotContains(t, resp.Capabilities.ReasonCodes, "NATIONAL_SERIES")
}

type fakePriceableUC struct {
	matches []usecase.PriceableEntityMatch
	err     error
	got     []string
}

func (f *fakePriceableUC) Get(_ context.Context, variants []string, _ int32) ([]usecase.PriceableEntityMatch, error) {
	f.got = variants
	return f.matches, f.err
}

func newPriceableServer(uc *fakePriceableUC) *PricebookServer {
	return NewPricebookServer(&fakeObservedPriceUC{}, &fakeEstimatedItemPriceUC{}, &fakeForecastCurveUC{}, &fakeSearchCropCatalogUC{}, mapping.Seed{}, &fakeTrendSummaryUC{}, &fakeSpatialSurfaceUC{}, nil, nil, nil, nil, nil, nil, uc)
}

func TestSearchPriceableEntities_EmptyVariantsIsInvalidArgument(t *testing.T) {
	s := newPriceableServer(&fakePriceableUC{err: usecase.ErrNoQueryVariants})

	_, err := s.SearchPriceableEntities(context.Background(), &pb.SearchPriceableEntitiesRequest{Variants: []string{""}})

	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSearchPriceableEntities_MapsFieldsAndStringifiesTierAndRank(t *testing.T) {
	uc := &fakePriceableUC{matches: []usecase.PriceableEntityMatch{{
		Kind: domain.EntityKindCommodity, Code: "7", Name: "白肉雞（2.0 公斤以上）產地價",
		CategoryIDs: []string{"2011", "2258"}, MatchedForm: "白肉雞", FormKind: domain.FormKindSourceTerm, Tier: 0, Rank: 1,
		HasForecast: true, GroupCode: "",
	}}}
	s := newPriceableServer(uc)

	resp, err := s.SearchPriceableEntities(context.Background(), &pb.SearchPriceableEntitiesRequest{Variants: []string{"白肉雞", "雞"}, Limit: 5})

	require.NoError(t, err)
	require.Equal(t, []string{"白肉雞", "雞"}, uc.got)
	require.Len(t, resp.GetMatches(), 1)
	m := resp.GetMatches()[0]
	require.Equal(t, "COMMODITY", m.GetKind())
	require.Equal(t, "7", m.GetCode())
	require.Equal(t, "白肉雞（2.0 公斤以上）產地價", m.GetName())
	require.Equal(t, []string{"2011", "2258"}, m.GetCategoryIds())
	require.Equal(t, "白肉雞", m.GetMatchedForm())
	require.Equal(t, "SOURCE_TERM", m.GetFormKind())
	require.Equal(t, "0", m.GetTier())
	require.Equal(t, "1", m.GetRank())
	require.True(t, m.GetHasForecast())
	require.Equal(t, "", m.GetGroupCode())
}

func TestSearchPriceableEntities_UsecaseErrorMapsToInternal(t *testing.T) {
	s := newPriceableServer(&fakePriceableUC{err: errors.New("boom")})

	_, err := s.SearchPriceableEntities(context.Background(), &pb.SearchPriceableEntitiesRequest{Variants: []string{"x"}})

	require.Equal(t, codes.Internal, status.Code(err))
}
