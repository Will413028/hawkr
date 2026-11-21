package slots

import (
	"fmt"
	"regexp"
)

// cityAliases 把常用簡稱與合併前的舊縣市名對到 AllowedCities 的正式值。只收唯一解的別名：新竹、嘉義同時有市與縣，刻意不收，
// 讓 Apply 拒絕並請買家講清楚，不替他猜。
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

func normalizeCity(raw string) (string, bool) {
	c, ok := cityAliases[raw]
	return c, ok
}

// 含四位數西元年的日期：年月日之間可以是 - / . 年 月（可混用），月日不需補零，「日」字尾可有可無。
var fullDatePattern = regexp.MustCompile(`^(\d{4})[-/.年](\d{1,2})[-/.月](\d{1,2})日?$`)

// 缺西元年的日期（「8/20」「8月20日」），用來給出比「格式不對」更精確的原因。
var monthDayOnlyPattern = regexp.MustCompile(`^(\d{1,2})[-/.月](\d{1,2})日?$`)

// normalizeDeliveryDate 把含完整西元年的日期文字轉成 DateLayout。缺年份的刻意不補：缺年份要問，不猜。
func normalizeDeliveryDate(raw string) (string, bool) {
	m := fullDatePattern.FindStringSubmatch(raw)
	if m == nil {
		return raw, false
	}
	month, day := m[2], m[3]
	if len(month) == 1 {
		month = "0" + month
	}
	if len(day) == 1 {
		day = "0" + day
	}
	return fmt.Sprintf("%s-%s-%s", m[1], month, day), true
}

func dateFormatReason(v string) Reason {
	if monthDayOnlyPattern.MatchString(v) {
		return ReasonDateMissingYear
	}
	return ReasonDateFormat
}
