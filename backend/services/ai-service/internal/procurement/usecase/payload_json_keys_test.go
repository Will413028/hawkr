package usecase

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
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

// TestPersistedPayloadTypes_UseCamelCaseJSONKeys 釘住這兩個型別序列化出來的鍵名。
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
			name:   "QuoteTotalRange",
			column: "procurement_price_previews.quote_total_range_json",
			value:  pricing.QuoteTotalRange{},
			want:   []string{"currency", "lowerBoundAmount", "quantity", "quantityUnit", "upperBoundAmount"},
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
