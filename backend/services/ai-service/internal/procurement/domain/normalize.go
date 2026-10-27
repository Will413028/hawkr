package domain

import (
	"fmt"
	"regexp"
)

// cityAliases 把買家常用的簡稱／舊制縣市名對應到 AllowedCities 裡的正式值。
// 只收「唯一解」的別名——像新竹、嘉義同時有市與縣兩個實體，刻意不收進來，
// 讓 Apply 走既有的 reject 路徑逼買家講清楚是市還是縣（confirm-not-assume，
// 設計書 §4.5），而不是幫他猜一個。
//
// 桃園縣／台北縣／台中縣／台南縣／高雄縣這幾個是 2010／2014 縣市合併前的舊制
// 名稱，買家口語仍常用；台北縣併入後改名新北市（不是「台北市」），其餘四個
// 維持同名只是拿掉「縣」字，個別列出以求清楚。
var cityAliases = map[string]string{
	"台北": "台北市", "臺北": "臺北市",
	"台北縣": "新北市", "臺北縣": "新北市", "新北": "新北市",
	"桃園": "桃園市", "桃園縣": "桃園市",
	"台中": "台中市", "臺中": "臺中市",
	"台中縣": "台中市", "臺中縣": "臺中市",
	"台南": "台南市", "臺南": "臺南市",
	"台南縣": "台南市", "臺南縣": "臺南市",
	"高雄": "高雄市", "高雄縣": "高雄市",
	"基隆": "基隆市",
	"苗栗": "苗栗縣",
	"彰化": "彰化縣",
	"南投": "南投縣",
	"雲林": "雲林縣",
	"屏東": "屏東縣",
	"宜蘭": "宜蘭縣",
	"花蓮": "花蓮縣",
	"台東": "台東縣", "臺東": "臺東縣",
	"澎湖": "澎湖縣",
	"金門": "金門縣",
	"連江": "連江縣",
}

// normalizeCity 把買家原話的縣市簡稱／舊制名稱轉成 AllowedCities 的正式值。
// 查不到別名時原樣傳回，讓呼叫端沿用既有的 contains 檢查與 reject 語意。
func normalizeCity(raw string) (string, bool) {
	canonical, ok := cityAliases[raw]
	return canonical, ok
}

// fullDatePattern 比對已經包含四位數西元年的日期文字，年月日之間可以是
// - / . 年 月 其中之一（可混用），月／日不需補零，「日」字尾可有可無。
var fullDatePattern = regexp.MustCompile(`^(\d{4})[-/.年](\d{1,2})[-/.月](\d{1,2})日?$`)

// monthDayOnlyPattern 比對缺西元年的日期文字（例如「8/20」「8月20日」），
// 用來在 reject 時給出比「格式不對」更精確的原因。
var monthDayOnlyPattern = regexp.MustCompile(`^(\d{1,2})[-/.月](\d{1,2})日?$`)

// normalizeDeliveryDate 把含完整西元年的日期文字轉成 DateLayout（YYYY-MM-DD）。
//
// 缺年份的輸入（如「8/20」）刻意不補年——設計書 §5.3 要求缺年份要問而非猜，
// 這裡直接讓它轉換失敗，Apply 既有的 reject／再問流程就會自然生效，不需要
// 另外設計「猜測後再確認」的 UI 機制。
func normalizeDeliveryDate(raw string) (string, bool) {
	m := fullDatePattern.FindStringSubmatch(raw)
	if m == nil {
		return raw, false
	}
	year, month, day := m[1], m[2], m[3]
	if len(month) == 1 {
		month = "0" + month
	}
	if len(day) == 1 {
		day = "0" + day
	}
	return fmt.Sprintf("%s-%s-%s", year, month, day), true
}

// deliveryDateRejectReason 在日期解析失敗時挑一個對買家更好懂的原因：
// 看起來像日期但缺年份的，明講缺年份；其餘沿用原本的格式錯誤說法。
func deliveryDateRejectReason(v string) string {
	if monthDayOnlyPattern.MatchString(v) {
		return "缺少西元年份，請補上年份（例如 2026-08-20）"
	}
	return "日期格式需為 YYYY-MM-DD"
}
