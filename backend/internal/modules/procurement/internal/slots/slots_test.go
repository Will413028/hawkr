package slots

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
)

// 對照測試的「舊」指舊 ai-service：backend/services/ai-service/internal/procurement/domain/。
// 輸入與預期都取自舊測試與 testdata/slots_apply/*.json，註解標出來源 file:line。

func ptr[T any](v T) *T { return &v }

// refNow 與舊測試的 refNow（slots_test.go:13）同一時刻：2026-08-07 00:00 UTC，即台北 08:00，同一個日曆日。
var refNow = time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestApplyParity(t *testing.T) {
	long := strings.Repeat("梨", MaxTextLen+50)
	truncated := strings.Repeat("梨", MaxTextLen)
	before80, _ := Set{}.Apply(Patch{Item: ptr("寶島梨禮盒"), Quantity: ptr(int64(50))}, refNow)
	beforeKG, _ := Set{}.Apply(Patch{Unit: ptr("KG")}, refNow)

	cases := []struct {
		name   string
		src    string
		before Set
		patch  Patch
		want   Set
		reject []Rejection
	}{
		{"accepts valid patch", "slots_test.go:15; testdata/slots_apply/accepts_valid_patch.json", Set{},
			Patch{Item: ptr("寶島梨禮盒"), Quantity: ptr(int64(50)), Unit: ptr("KG"), BudgetPerUnit: ptr(int64(800)), DeliveryCity: ptr("桃園市"), DeliveryDate: ptr("2026-08-20")},
			Set{Item: ptr("寶島梨禮盒"), Quantity: ptr(int32(50)), Unit: ptr("KG"), BudgetPerUnit: ptr(int64(800)), DeliveryCity: ptr("桃園市"), DeliveryDate: day(2026, 8, 20), ExactMatchRequired: ptr(true)},
			nil},
		{"drops invalid fields, keeps valid ones", "slots_test.go:35; testdata/slots_apply/drops_invalid_fields_but_keeps_valid_ones.json", Set{},
			Patch{Item: ptr("寶島梨禮盒"), Quantity: ptr(int64(0)), Unit: ptr("箱子"), DeliveryCity: ptr("不存在市"), DeliveryDate: ptr("2026-08-01")},
			Set{Item: ptr("寶島梨禮盒"), ExactMatchRequired: ptr(true)},
			[]Rejection{{Quantity, ReasonQuantityRange}, {Unit, ReasonUnitNotAllowed}, {DeliveryCity, ReasonCityUnrecognized}, {DeliveryDate, ReasonDatePast}}},
		{"truncates overlong text by runes", "slots_test.go:62; testdata/slots_apply/truncates_overlong_text.json", Set{},
			Patch{Item: &long},
			Set{Item: &truncated, ExactMatchRequired: ptr(true)},
			[]Rejection{{Item, ReasonTruncated}}},
		{"rejects date too far out", "slots_test.go:75; testdata/slots_apply/rejects_date_too_far_out.json", Set{},
			Patch{DeliveryDate: ptr("2028-01-01")},
			Set{ExactMatchRequired: ptr(true)},
			[]Rejection{{DeliveryDate, ReasonDateTooFar}}},
		{"preserves untouched fields", "slots_test.go:81; testdata/slots_apply/preserves_untouched_fields.json", before80,
			Patch{Quantity: ptr(int64(80))},
			Set{Item: ptr("寶島梨禮盒"), Quantity: ptr(int32(80)), ExactMatchRequired: ptr(true)},
			nil},
		{"normalizes city alias", "slots_test.go:128; testdata/slots_apply/normalizes_city_alias.json", Set{},
			Patch{DeliveryCity: ptr("桃園")},
			Set{DeliveryCity: ptr("桃園市"), ExactMatchRequired: ptr(true)}, nil},
		{"ambiguous city stays rejected", "slots_test.go:134; testdata/slots_apply/ambiguous_city_stays_rejected.json", Set{},
			Patch{DeliveryCity: ptr("新竹")},
			Set{ExactMatchRequired: ptr(true)},
			[]Rejection{{DeliveryCity, ReasonCityUnrecognized}}},
		{"normalizes delivery date with year", "slots_test.go:142; testdata/slots_apply/normalizes_delivery_date_with_year.json", Set{},
			Patch{DeliveryDate: ptr("2026/8/20")},
			Set{DeliveryDate: day(2026, 8, 20), ExactMatchRequired: ptr(true)}, nil},
		{"missing year is rejected with its own reason", "slots_test.go:148; testdata/slots_apply/rejects_delivery_date_missing_year_with_clear_reason.json", Set{},
			Patch{DeliveryDate: ptr("8/20")},
			Set{ExactMatchRequired: ptr(true)},
			[]Rejection{{DeliveryDate, ReasonDateMissingYear}}},
		{"golden request from flow 19", "slots_test.go:178; testdata/slots_apply/golden_request_from_flow_19.json", Set{},
			Patch{Item: ptr("寶島梨禮盒"), Quantity: ptr(int64(50)), BudgetPerUnit: ptr(int64(800)), DeliveryCity: ptr("桃園"), DeliveryDate: ptr("8/20")},
			Set{Item: ptr("寶島梨禮盒"), Quantity: ptr(int32(50)), BudgetPerUnit: ptr(int64(800)), DeliveryCity: ptr("桃園市"), ExactMatchRequired: ptr(true)},
			[]Rejection{{DeliveryDate, ReasonDateMissingYear}}},
		{"unit outside the closed set is rejected", "slots_test.go:222; testdata/slots_apply/rejects_unit_not_in_allowed_units.json", Set{},
			Patch{Unit: ptr("BOX")},
			Set{ExactMatchRequired: ptr(true)},
			[]Rejection{{Unit, ReasonUnitNotAllowed}}},
		{"invalid new unit keeps the confirmed one", "slots_test.go:244; testdata/slots_apply/preserves_unit_on_rejection_when_new_unit_invalid.json", beforeKG,
			Patch{Unit: ptr("BOX")},
			Set{Unit: ptr("KG"), ExactMatchRequired: ptr(true)},
			[]Rejection{{Unit, ReasonUnitNotAllowed}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, rejected := c.before.Apply(c.patch, refNow)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("(%s) Apply = %+v, want %+v", c.src, got, c.want)
			}
			if !reflect.DeepEqual(rejected, c.reject) {
				t.Errorf("(%s) rejected = %v, want %v", c.src, rejected, c.reject)
			}
		})
	}
}

// 舊 slots_test.go:35 之外的欄位值邊界：數量與預算的上下限、單位大小寫與空白。
func TestApplyBounds(t *testing.T) {
	cases := []struct {
		name   string
		patch  Patch
		reject []Rejection
	}{
		{"quantity at lower bound", Patch{Quantity: ptr(int64(1))}, nil},
		{"quantity at upper bound", Patch{Quantity: ptr(int64(MaxQuantity))}, nil},
		{"quantity above upper bound", Patch{Quantity: ptr(int64(MaxQuantity + 1))}, []Rejection{{Quantity, ReasonQuantityRange}}},
		{"quantity beyond int32 is rejected, not wrapped", Patch{Quantity: ptr(int64(1) << 40)}, []Rejection{{Quantity, ReasonQuantityRange}}},
		{"budget zero", Patch{BudgetPerUnit: ptr(int64(0))}, []Rejection{{BudgetPerUnit, ReasonBudgetRange}}},
		{"budget at upper bound", Patch{BudgetPerUnit: ptr(int64(MaxBudgetPerUnit))}, nil},
		{"budget above upper bound", Patch{BudgetPerUnit: ptr(int64(MaxBudgetPerUnit + 1))}, []Rejection{{BudgetPerUnit, ReasonBudgetRange}}},
		{"unit is trimmed and upper-cased", Patch{Unit: ptr("  kg ")}, nil},
		{"empty item is dropped", Patch{Item: ptr("   ")}, []Rejection{{Item, ReasonEmpty}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, rejected := Set{}.Apply(c.patch, refNow)
			if !reflect.DeepEqual(rejected, c.reject) {
				t.Errorf("rejected = %v, want %v", rejected, c.reject)
			}
		})
	}
	got, _ := Set{}.Apply(Patch{Unit: ptr("  kg ")}, refNow)
	if got.Unit == nil || *got.Unit != "KG" {
		t.Errorf("unit = %v, want KG", got.Unit)
	}
}

// 舊 Apply 以 UTC 判斷「今天」，台北凌晨會把昨天當成今天；新規則以台北日曆日為界（platform/twtime.Day），這是刻意的差異。
func TestApplyDateBoundariesUseTaipeiCalendarDay(t *testing.T) {
	// 2026-08-06 18:00 UTC 是台北 08-07 02:00。
	now := time.Date(2026, 8, 6, 18, 0, 0, 0, time.UTC)
	cases := []struct {
		date   string
		reject []Rejection
	}{
		{"2026-08-06", []Rejection{{DeliveryDate, ReasonDatePast}}}, // 台北的昨天（舊規則會收）
		{"2026-08-07", nil}, // 台北的今天
		{"2027-08-07", nil}, // 今天加 365 天
		{"2027-08-08", []Rejection{{DeliveryDate, ReasonDateTooFar}}},
	}
	for _, c := range cases {
		_, rejected := Set{}.Apply(Patch{DeliveryDate: ptr(c.date)}, now)
		if !reflect.DeepEqual(rejected, c.reject) {
			t.Errorf("date %s: rejected = %v, want %v", c.date, rejected, c.reject)
		}
	}
}

// 截斷點落在空白上時，截斷後的值不得有尾端空白：資料表的 CHECK 不收（舊實作沒有這一步）。
func TestApplyTruncationLeavesNoTrailingSpace(t *testing.T) {
	in := strings.Repeat("a", MaxTextLen-1) + " b"
	got, rejected := Set{}.Apply(Patch{Item: &in}, refNow)
	if len(rejected) != 1 || rejected[0].Reason != ReasonTruncated {
		t.Fatalf("rejected = %v, want one TRUNCATED", rejected)
	}
	if want := strings.Repeat("a", MaxTextLen-1); *got.Item != want {
		t.Errorf("item = %q, want %q", *got.Item, want)
	}
}

// 品項一變動就清掉舊解析；品項被驗證退回時不清（舊 slots.go:139-153 的不變量）。
func TestApplyItemChangeClearsResolution(t *testing.T) {
	res := Resolution{Kind: BuyerDescribed, Label: "小白菜"}
	base, rejected := Set{}.Apply(Patch{Item: ptr("青菜"), Resolution: &res}, refNow)
	if len(rejected) != 0 || base.Resolution == nil {
		t.Fatalf("setup: resolution = %v, rejected = %v", base.Resolution, rejected)
	}

	same, _ := base.Apply(Patch{Item: ptr("青菜")}, refNow)
	if same.Resolution == nil {
		t.Error("same item must keep the resolution")
	}
	bad, _ := base.Apply(Patch{Item: ptr("  ")}, refNow)
	if bad.Resolution == nil || *bad.Item != "青菜" {
		t.Error("a rejected item patch must not clear the resolution")
	}
	changed, _ := base.Apply(Patch{Item: ptr("高麗菜")}, refNow)
	if changed.Resolution != nil {
		t.Error("a changed item must clear the old resolution")
	}
	both, _ := base.Apply(Patch{Item: ptr("高麗菜"), Resolution: &Resolution{Kind: CategoryConfirmed, Label: "高麗菜"}}, refNow)
	if both.Resolution == nil || both.Resolution.Label != "高麗菜" {
		t.Errorf("item and resolution in one patch: resolution = %v", both.Resolution)
	}
}

func TestApplyResolutionRules(t *testing.T) {
	cases := []struct {
		name   string
		before Set
		res    Resolution
		reject []Rejection
	}{
		{"no item to resolve", Set{}, Resolution{Kind: BuyerDescribed, Label: "x"}, []Rejection{{ItemResolution, ReasonResolutionNoItem}}},
		{"unknown kind", Set{Item: ptr("a")}, Resolution{Kind: "OTHER", Label: "x"}, []Rejection{{ItemResolution, ReasonResolutionUnknownKind}}},
		{"empty label", Set{Item: ptr("a")}, Resolution{Kind: CategoryConfirmed, Label: " "}, []Rejection{{ItemResolution, ReasonResolutionNoLabel}}},
		{"valid", Set{Item: ptr("a")}, Resolution{Kind: CategoryConfirmed, Label: "x"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, rejected := c.before.Apply(Patch{Resolution: &c.res}, refNow)
			if !reflect.DeepEqual(rejected, c.reject) {
				t.Errorf("rejected = %v, want %v", rejected, c.reject)
			}
		})
	}
}

// 舊 slots_test.go:200-217：必填是品項、配送城市、單位，順序固定，填齊時是空 slice 而不是 nil。
func TestMissingRequired(t *testing.T) {
	cases := []struct {
		name  string
		patch Patch
		want  []model.SlotKey
	}{
		{"nothing filled", Patch{}, []model.SlotKey{Item, DeliveryCity, Unit}},
		{"city missing (slots_test.go:200)", Patch{Item: ptr("梨"), Quantity: ptr(int64(50)), Unit: ptr("KG")}, []model.SlotKey{DeliveryCity}},
		{"unit missing (slots_test.go:213)", Patch{Item: ptr("梨"), DeliveryCity: ptr("桃園市")}, []model.SlotKey{Unit}},
		{"flow 19: only unit missing (slots_test.go:190)", Patch{Item: ptr("梨"), Quantity: ptr(int64(50)), BudgetPerUnit: ptr(int64(800)), DeliveryCity: ptr("桃園")}, []model.SlotKey{Unit}},
		{"complete (slots_test.go:102)", Patch{Item: ptr("梨"), DeliveryCity: ptr("桃園市"), Unit: ptr("KG")}, []model.SlotKey{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := Set{}.Apply(c.patch, refNow)
			got := s.MissingRequired()
			if got == nil || !reflect.DeepEqual(got, c.want) {
				t.Errorf("MissingRequired = %#v, want %#v", got, c.want)
			}
		})
	}
}

func TestIsRequired(t *testing.T) {
	for _, k := range []model.SlotKey{Item, DeliveryCity, Unit} {
		if !IsRequired(k) {
			t.Errorf("%s should be required", k)
		}
	}
	for _, k := range []model.SlotKey{Quantity, BudgetPerUnit, DeliveryDate, Packaging, ExactMatchRequired, ItemResolution} {
		if IsRequired(k) {
			t.Errorf("%s should not be required", k)
		}
	}
}

// 舊 normalize_test.go:5-86。
func TestNormalizeCity(t *testing.T) {
	aliases := map[string]string{ // normalize_test.go:6-21
		"桃園": "桃園市", "桃園縣": "桃園市", "台北": "台北市", "臺北": "臺北市", "新北": "新北市", "台北縣": "新北市", "臺北縣": "新北市",
		"台中縣": "台中市", "臺中縣": "臺中市", "高雄": "高雄市", "高雄縣": "高雄市", "苗栗": "苗栗縣", "台東": "台東縣", "臺東": "臺東縣",
	}
	for raw, want := range aliases {
		if got, ok := normalizeCity(raw); !ok || got != want {
			t.Errorf("normalizeCity(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"新竹", "嘉義", "桃園市"} { // normalize_test.go:34-48：有歧義或已是正式值，不命中
		if _, ok := normalizeCity(raw); ok {
			t.Errorf("normalizeCity(%q) must not match", raw)
		}
	}
	for alias, canonical := range cityAliases {
		if !contains(AllowedCities, canonical) {
			t.Errorf("alias %q points at %q which is not an allowed city", alias, canonical)
		}
	}
}

func TestNormalizeDeliveryDate(t *testing.T) {
	for raw, want := range map[string]string{ // normalize_test.go:51-57
		"2026/8/20": "2026-08-20", "2026/08/20": "2026-08-20", "2026年8月20日": "2026-08-20", "2026.8.20": "2026-08-20", "2026-08-20": "2026-08-20",
	} {
		if got, ok := normalizeDeliveryDate(raw); !ok || got != want {
			t.Errorf("normalizeDeliveryDate(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"8/20", "8月20日", "8-20"} { // normalize_test.go:72，缺年份不補
		if _, ok := normalizeDeliveryDate(raw); ok {
			t.Errorf("normalizeDeliveryDate(%q) must not resolve", raw)
		}
	}
	if got := dateFormatReason("8/20"); got != ReasonDateMissingYear { // normalize_test.go:79-86
		t.Errorf("missing-year reason = %s", got)
	}
	if got := dateFormatReason("不是日期"); got != ReasonDateFormat {
		t.Errorf("format reason = %s", got)
	}
}

// 舊 recorded_test.go:10-138。
func TestRecordedThisTurn(t *testing.T) {
	apply := func(before Set, p Patch) Set {
		next, _ := before.Apply(p, refNow)
		return next
	}
	t.Run("empty patch (recorded_test.go:10)", func(t *testing.T) {
		if got := RecordedThisTurn(Patch{}, Set{}, apply(Set{}, Patch{})); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("system default is not recorded (recorded_test.go:16)", func(t *testing.T) {
		p := Patch{Item: ptr("寶島梨禮盒")}
		got := RecordedThisTurn(p, Set{}, apply(Set{}, p))
		want := []Recorded{{Item, "寶島梨禮盒"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("fixed order, string, number, bool and date (recorded_test.go:28,40)", func(t *testing.T) {
		p := Patch{DeliveryCity: ptr("桃園市"), Item: ptr("寶島梨禮盒"), Quantity: ptr(int64(50)), BudgetPerUnit: ptr(int64(800)), ExactMatchRequired: ptr(false), DeliveryDate: ptr("2026-08-20")}
		got := RecordedThisTurn(p, Set{}, apply(Set{}, p))
		want := []Recorded{{Item, "寶島梨禮盒"}, {Quantity, "50"}, {BudgetPerUnit, "800"}, {DeliveryCity, "桃園市"}, {DeliveryDate, "2026-08-20"}, {ExactMatchRequired, "false"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("rejected field excluded (recorded_test.go:55)", func(t *testing.T) {
		p := Patch{Item: ptr("寶島梨禮盒"), Quantity: ptr(int64(0))}
		got := RecordedThisTurn(p, Set{}, apply(Set{}, p))
		if want := []Recorded{{Item, "寶島梨禮盒"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("unchanged earlier fields are not repeated (recorded_test.go:66)", func(t *testing.T) {
		first := apply(Set{}, Patch{Item: ptr("寶島梨禮盒")})
		p := Patch{DeliveryCity: ptr("桃園市")}
		got := RecordedThisTurn(p, first, apply(first, p))
		if want := []Recorded{{DeliveryCity, "桃園市"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("correction reappears (recorded_test.go:77)", func(t *testing.T) {
		first := apply(Set{}, Patch{DeliveryCity: ptr("台北市")})
		p := Patch{DeliveryCity: ptr("桃園市")}
		got := RecordedThisTurn(p, first, apply(first, p))
		if want := []Recorded{{DeliveryCity, "桃園市"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("truncated but adopted field is recorded with the truncated value (recorded_test.go:88)", func(t *testing.T) {
		p := Patch{Item: ptr(strings.Repeat("寶", 205))}
		got := RecordedThisTurn(p, Set{}, apply(Set{}, p))
		if want := []Recorded{{Item, strings.Repeat("寶", 200)}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v", got)
		}
	})
	t.Run("fully discarded field is not recorded (recorded_test.go:102)", func(t *testing.T) {
		p := Patch{Item: ptr("")}
		if got := RecordedThisTurn(p, Set{}, apply(Set{}, p)); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("nil next value does not panic (recorded_test.go:112)", func(t *testing.T) {
		cur := Set{Quantity: ptr(int32(50)), BudgetPerUnit: ptr(int64(800)), ExactMatchRequired: ptr(true), DeliveryDate: day(2026, 9, 1)}
		p := Patch{Quantity: ptr(int64(60)), BudgetPerUnit: ptr(int64(900)), ExactMatchRequired: ptr(false), DeliveryDate: ptr("2026-09-02")}
		if got := RecordedThisTurn(p, cur, Set{}); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("item resolution is recorded when the server sets it", func(t *testing.T) {
		first := apply(Set{}, Patch{Item: ptr("青菜")})
		p := Patch{Resolution: &Resolution{Kind: CategoryConfirmed, Label: "小白菜"}}
		got := RecordedThisTurn(p, first, apply(first, p))
		if want := []Recorded{{ItemResolution, "小白菜"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
