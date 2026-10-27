package usecase

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
)

// marshalKeys 回傳 v 序列化後的頂層鍵，排序後方便比對。
func marshalKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() error = %v，序列化結果：%s", err, b)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestPersistedPayloadTypes_UseCamelCaseJSONKeys 釘住這五個型別序列化出來的鍵名。
//
// 它們不是普通的記憶體結構——每一個都被 json.Marshal 直接寫進一個 JSONB 欄位，
// 所以「Go 欄位叫什麼名字」就是「DB 與下游讀到的鍵叫什麼名字」。沒有 json tag
// 時鍵是 Go 欄位名的 PascalCase，跟同一份 payload 裡其餘 camelCase 欄位分家
// （confirmedRfqPayload 的頂層鍵、SellerFacingPayload、proto JSON 全是 camelCase）。
//
// 斷言鍵集完全相同（不是「包含」）：新增欄位卻忘了 tag 會讓鍵集多出一個
// PascalCase 名字，這裡就要紅。
func TestPersistedPayloadTypes_UseCamelCaseJSONKeys(t *testing.T) {
	cases := []struct {
		name   string
		column string
		value  any
		want   []string
	}{
		{
			name:   "BuyerRequestedRange",
			column: "procurement_price_previews.buyer_requested_range_json ＋ confirmed snapshot 的 buyerRequestedRange",
			value:  pricing.BuyerRequestedRange{},
			want:   []string{"basisQuantity", "basisUnit", "currency", "lowerBoundAmount", "priceComponent", "upperBoundAmount"},
		},
		{
			name:   "ReferencePriceRange",
			column: "procurement_price_previews.reference_payload_json ＋ confirmed snapshot 的 referencePriceRange",
			value:  pricing.ReferencePriceRange{},
			want:   []string{"basisQuantity", "basisUnit", "lower", "midpoint", "upper"},
		},
		{
			name:   "MarketPriceBasis",
			column: "procurement_price_previews.market_price_basis_json",
			value:  pricing.MarketPriceBasis{},
			want: []string{
				"freshnessStatus", "priceLayer", "regionName", "scope",
				"sourceDataAsOf", "supportingMarketCount",
			},
		},
		{
			name:   "QuoteTotalRange",
			column: "procurement_price_previews.quote_total_range_json",
			value:  pricing.QuoteTotalRange{},
			want:   []string{"currency", "lowerBoundAmount", "quantity", "quantityUnit", "upperBoundAmount"},
		},
		{
			name:   "DealTemperature",
			column: "procurement_price_previews.deal_temperature_json",
			value:  pricing.DealTemperature{},
			want:   []string{"reasonCode", "status"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := marshalKeys(t, tc.value)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("%s 的 JSON 鍵 = %v，want %v（這組鍵就是 %s 的內容）", tc.name, got, tc.want, tc.column)
			}
		})
	}
}

// TestToPricePreviewResult_ReadsBackRowsWrittenBeforeTheJSONTags 是這次改動的
// 零遷移論證本身：改 tag 之前寫進去的列，鍵是 PascalCase；改完之後
// pricing.ToPricePreviewResult 仍然要能把它們完整還原。
//
// 成立的理由是 encoding/json 的 Unmarshal 對鍵名做 case-insensitive fallback
// （精確匹配優先，找不到才退回不分大小寫），所以 "Lower" 仍然填得進
// `json:"lower"` 的欄位。這條性質沒有被任何測試釘住過，而整個「不寫 data
// migration」的決定完全靠它——所以在這裡釘住，不是靠語言文件的記憶。
func TestToPricePreviewResult_ReadsBackRowsWrittenBeforeTheJSONTags(t *testing.T) {
	legacy := repository.PricePreview{
		ID: 7, Version: 2, PricingMode: "PRICEBOOK_GUIDED", ReferenceAvailability: "AVAILABLE",
		ReferencePayloadJSON:    json.RawMessage(`{"Lower":30.25,"Midpoint":36,"Upper":41.75,"BasisQuantity":"1000","BasisUnit":"GRAM"}`),
		BuyerRequestedRangeJSON: json.RawMessage(`{"Currency":"TWD","PriceComponent":"GOODS_ONLY","BasisQuantity":"1000","BasisUnit":"GRAM","LowerBoundAmount":"30","UpperBoundAmount":"42"}`),
		MarketPriceBasisJSON:    json.RawMessage(`{"RegionName":"臺中市","SupportingMarketCount":3,"SourceDataAsOf":"2026-09-01T00:00:00Z","FreshnessStatus":"fresh"}`),
		QuoteTotalRangeJSON:     json.RawMessage(`{"Currency":"TWD","LowerBoundAmount":"3000","UpperBoundAmount":"4200","Quantity":"100","QuantityUnit":"KG"}`),
		DealTemperatureJSON:     json.RawMessage(`{"Status":"WARM","ReasonCode":"WITHIN_MARKET_RANGE"}`),
	}

	got := pricing.ToPricePreviewResult(legacy)

	if got.ReferencePriceRange == nil {
		t.Fatal("ReferencePriceRange = nil，舊列的參考價區間整個讀不回來")
	}
	if got.ReferencePriceRange.Lower == nil || got.ReferencePriceRange.Upper == nil {
		t.Fatalf("舊列的上下界讀成 nil：%+v", *got.ReferencePriceRange)
	}
	if *got.ReferencePriceRange.Lower != 30.25 || got.ReferencePriceRange.Midpoint != 36 ||
		*got.ReferencePriceRange.Upper != 41.75 {
		t.Errorf("ReferencePriceRange 金額 = %+v，want {30.25 36 41.75}", *got.ReferencePriceRange)
	}
	if got.ReferencePriceRange.BasisQuantity != "1000" || got.ReferencePriceRange.BasisUnit != "GRAM" {
		t.Errorf("ReferencePriceRange 計價基準 = %q/%q，want 1000/GRAM", got.ReferencePriceRange.BasisQuantity, got.ReferencePriceRange.BasisUnit)
	}
	if got.BuyerRequestedRange.Currency != "TWD" || got.BuyerRequestedRange.LowerBoundAmount != "30" || got.BuyerRequestedRange.UpperBoundAmount != "42" {
		t.Errorf("BuyerRequestedRange = %+v，want TWD 30~42", got.BuyerRequestedRange)
	}
	if got.BuyerRequestedRange.BasisQuantity != "1000" || got.BuyerRequestedRange.BasisUnit != "GRAM" {
		t.Errorf("BuyerRequestedRange 計價基準 = %q/%q，want 1000/GRAM", got.BuyerRequestedRange.BasisQuantity, got.BuyerRequestedRange.BasisUnit)
	}
	if got.MarketPriceBasis == nil || got.MarketPriceBasis.RegionName != "臺中市" || got.MarketPriceBasis.SupportingMarketCount != 3 {
		t.Errorf("MarketPriceBasis = %+v，want 臺中市/3", got.MarketPriceBasis)
	}
	if got.QuoteTotalRange == nil || got.QuoteTotalRange.LowerBoundAmount != "3000" || got.QuoteTotalRange.QuantityUnit != "KG" {
		t.Errorf("QuoteTotalRange = %+v，want 3000.../KG", got.QuoteTotalRange)
	}
	if got.DealTemperature.Status != "WARM" || got.DealTemperature.ReasonCode != "WITHIN_MARKET_RANGE" {
		t.Errorf("DealTemperature = %+v，want WARM/WITHIN_MARKET_RANGE", got.DealTemperature)
	}
}
