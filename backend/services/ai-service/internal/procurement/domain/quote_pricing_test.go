package domain_test

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
)

func f64p(f float64) *float64 { return &f }
func int64p(i int64) *int64   { return &i }

func TestMatchRegionPrice_FreshReadyRegion_ReturnsMarket(t *testing.T) {
	regions := []usecase.RegionalComparisonRegion{
		{RegionName: "臺北市", Status: "ready", P50: f64p(50.0), FreshnessStatus: "fresh"},
		{RegionName: "桃園市", Status: "ready", P50: f64p(42.5), FreshnessStatus: "fresh"},
	}
	result := domain.MatchRegionPrice(regions, "桃園市")
	if result.Availability != domain.PriceAvailabilityMarket {
		t.Errorf("Availability = %v, want MARKET", result.Availability)
	}
	if result.PricePerKg == nil || *result.PricePerKg != 42.5 {
		t.Errorf("PricePerKg = %v, want 42.5", result.PricePerKg)
	}
}

func TestMatchRegionPrice_StaleReadyRegion_ReturnsMarketStale(t *testing.T) {
	regions := []usecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P50: f64p(42.5), FreshnessStatus: "stale"},
	}
	result := domain.MatchRegionPrice(regions, "桃園市")
	if result.Availability != domain.PriceAvailabilityMarketStale {
		t.Errorf("Availability = %v, want MARKET_STALE", result.Availability)
	}
	if result.PricePerKg == nil || *result.PricePerKg != 42.5 {
		t.Errorf("PricePerKg = %v, want 42.5（stale 仍要帶著舊資料的價格，不是 nil）", result.PricePerKg)
	}
}

func TestMatchRegionPrice_NormalizesTaiCharacterVariant(t *testing.T) {
	regions := []usecase.RegionalComparisonRegion{
		{RegionName: "臺南市", Status: "ready", P50: f64p(30.0), FreshnessStatus: "fresh"},
	}
	result := domain.MatchRegionPrice(regions, "台南市")
	if result.Availability != domain.PriceAvailabilityMarket || result.PricePerKg == nil || *result.PricePerKg != 30.0 {
		t.Errorf("result = %+v, want MARKET/30.0", result)
	}
}

func TestMatchRegionPrice_UnavailableRegion_ReturnsUnavailableNilPrice(t *testing.T) {
	regions := []usecase.RegionalComparisonRegion{
		{RegionName: "澎湖縣", Status: "unavailable", P50: nil},
	}
	result := domain.MatchRegionPrice(regions, "澎湖縣")
	if result.Availability != domain.PriceAvailabilityUnavailable {
		t.Errorf("Availability = %v, want UNAVAILABLE", result.Availability)
	}
	if result.PricePerKg != nil {
		t.Errorf("PricePerKg = %v, want nil", result.PricePerKg)
	}
}

func TestMatchRegionPrice_CityNotInResponse_ReturnsUnavailable(t *testing.T) {
	regions := []usecase.RegionalComparisonRegion{{RegionName: "臺北市", Status: "ready", P50: f64p(50.0), FreshnessStatus: "fresh"}}
	result := domain.MatchRegionPrice(regions, "高雄市")
	if result.Availability != domain.PriceAvailabilityUnavailable {
		t.Errorf("Availability = %v, want UNAVAILABLE", result.Availability)
	}
}

func TestConvertToUnitPrice_ReturnsPricePerKgCopy(t *testing.T) {
	base := int64p(40)
	price := domain.ConvertToUnitPrice(base)
	if price == nil || *price != 40 {
		t.Errorf("price = %v, want 40", price)
	}
	if price == base {
		t.Error("ConvertToUnitPrice 必須回傳新指標，不能跟 basePricePerKg alias 同一顆——否則同批多個候選會共用同一顆指標")
	}
}

func TestConvertToUnitPrice_NilBasePrice_ReturnsNil(t *testing.T) {
	price := domain.ConvertToUnitPrice(nil)
	if price != nil {
		t.Errorf("price = %v, want nil（沒有市場行情就不該算出任何單價）", price)
	}
}
