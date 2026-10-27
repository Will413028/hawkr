package domain

import "testing"

func TestNormalizeCity_ResolvesKnownAliases(t *testing.T) {
	cases := map[string]string{
		"桃園":  "桃園市",
		"桃園縣": "桃園市",
		"台北":  "台北市",
		"臺北":  "臺北市",
		"新北":  "新北市",
		"台北縣": "新北市",
		"臺北縣": "新北市",
		"台中縣": "台中市",
		"臺中縣": "臺中市",
		"高雄":  "高雄市",
		"高雄縣": "高雄市",
		"苗栗":  "苗栗縣",
		"台東":  "台東縣",
		"臺東":  "臺東縣",
	}
	for raw, want := range cases {
		got, ok := normalizeCity(raw)
		if !ok {
			t.Errorf("normalizeCity(%q) 應該要有別名，實際 ok=false", raw)
			continue
		}
		if got != want {
			t.Errorf("normalizeCity(%q) = %q，want %q", raw, got, want)
		}
	}
}

func TestNormalizeCity_LeavesAmbiguousNamesUnresolved(t *testing.T) {
	// 新竹／嘉義同時有市與縣，不該幫買家猜是哪一個。
	for _, raw := range []string{"新竹", "嘉義"} {
		if _, ok := normalizeCity(raw); ok {
			t.Errorf("normalizeCity(%q) 不該有別名（市／縣有歧義，應交給 Apply 的 reject 逼買家講清楚）", raw)
		}
	}
}

func TestNormalizeCity_LeavesCanonicalNamesUnresolved(t *testing.T) {
	// 已經是正式值時不需要（也不應該）改寫。
	if _, ok := normalizeCity("桃園市"); ok {
		t.Error("normalizeCity(\"桃園市\") 已是正式值，不該命中別名表")
	}
}

func TestNormalizeDeliveryDate_ConvertsFullDatesWithYear(t *testing.T) {
	cases := map[string]string{
		"2026/8/20":  "2026-08-20",
		"2026/08/20": "2026-08-20",
		"2026年8月20日": "2026-08-20",
		"2026.8.20":  "2026-08-20",
		"2026-08-20": "2026-08-20",
	}
	for raw, want := range cases {
		got, ok := normalizeDeliveryDate(raw)
		if !ok {
			t.Errorf("normalizeDeliveryDate(%q) 應可解析，實際 ok=false", raw)
			continue
		}
		if got != want {
			t.Errorf("normalizeDeliveryDate(%q) = %q，want %q", raw, got, want)
		}
	}
}

func TestNormalizeDeliveryDate_LeavesMissingYearUnresolved(t *testing.T) {
	// 設計書 §5.3：缺年份要問而非猜——不得在這裡幫買家補上最近年份。
	for _, raw := range []string{"8/20", "8月20日", "8-20"} {
		if _, ok := normalizeDeliveryDate(raw); ok {
			t.Errorf("normalizeDeliveryDate(%q) 缺年份不該被解析成功", raw)
		}
	}
}

func TestDeliveryDateRejectReason_DistinguishesMissingYear(t *testing.T) {
	if got := deliveryDateRejectReason("8/20"); got == "日期格式需為 YYYY-MM-DD" {
		t.Errorf("缺年份的輸入應給更精確的原因，實際仍是通用格式錯誤：%q", got)
	}
	if got := deliveryDateRejectReason("不是日期"); got != "日期格式需為 YYYY-MM-DD" {
		t.Errorf("非日期形狀的輸入應維持通用格式錯誤，實際 %q", got)
	}
}
