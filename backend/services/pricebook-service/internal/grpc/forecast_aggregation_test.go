package grpc

import (
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
	"google.golang.org/protobuf/encoding/protojson"
	"strings"
)

func TestForecastAggregationMappingRetainsDailyCoverage(t *testing.T) {
	day1 := &usecase.ForecastAggregationMeta{Method: "volume_weighted/v1", WeightWindowDays: 28, ParticipatingMarketCodes: []string{"109"}, ExcludedMarkets: []usecase.ForecastMarketExclusion{{MarketCode: "104", Reason: "market_closed"}}}
	day2 := &usecase.ForecastAggregationMeta{Method: "equal_weight_fallback", WeightWindowDays: 28, ParticipatingMarketCodes: []string{"104", "109"}}
	result := toForecastCurveResponse(usecase.ForecastCurveResult{
		Status: forecast.PredictionStatusPartial, Aggregation: day1,
		Points:           []usecase.CurvePoint{{HorizonDays: 1, Aggregation: day1}, {HorizonDays: 2, Aggregation: day2}},
		MarketComparison: []usecase.MarketComparisonEntry{{MarketCode: "104", Status: forecast.PredictionStatusUnavailable}},
	})
	if result.Aggregation.GetWeightWindowDays() != "28" || result.Points[1].Aggregation.Method != "equal_weight_fallback" {
		t.Fatalf("lost metadata: %v", result)
	}
	if result.Aggregation.ExcludedMarkets[0].Reason != "market_closed" || len(result.Points[1].Aggregation.ParticipatingMarketCodes) != 2 {
		t.Fatalf("lost daily coverage: %v", result)
	}
	if result.MarketComparison[0].Status != "unavailable" || result.MarketComparison[0].P50 != nil {
		t.Fatalf("lost unavailable state: %v", result)
	}
	data, err := protojson.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	compact := strings.ReplaceAll(string(data), " ", "")
	if !strings.Contains(compact, `"weightWindowDays":"28"`) {
		t.Fatalf("new numeric must be string: %s", data)
	}
	if toForecastAggregationMeta(nil) != nil {
		t.Fatal("legacy path must retain absent metadata")
	}
}
