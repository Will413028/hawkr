package slots

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// mergedCounties 是升格合併前的舊縣名對到現行縣市（代碼表只有現行縣市，這幾個不能從名稱推導）。
var mergedCounties = map[string]string{
	"臺北縣": "新北市", "桃園縣": "桃園市", "臺中縣": "臺中市", "臺南縣": "臺南市", "高雄縣": "高雄市",
}

// cityNames 把縣市的正式名稱與去掉「市／縣」的簡稱對到代碼表（platform/twaddr）的縣市名；同一個簡稱對到兩個縣市
// （新竹、嘉義）的不收，讓 Apply 拒絕並請買家講清楚，不替他猜。
var cityNames = func() map[string]string {
	m := map[string]string{}
	short := map[string][]string{}
	for _, c := range twaddr.Counties() {
		m[c.Name] = c.Name
		r := []rune(c.Name)
		base := string(r[:len(r)-1])
		short[base] = append(short[base], c.Name)
	}
	for base, names := range short {
		if len(names) == 1 {
			m[base] = names[0]
		}
	}
	return m
}()

// normalizeCity 先把「台」統一成代碼表的「臺」，再對縣市；回傳代碼表的正式名稱，台與臺只有這一種拼法。
func normalizeCity(raw string) (string, bool) {
	v := strings.ReplaceAll(raw, "台", "臺")
	if c, ok := mergedCounties[v]; ok {
		return c, true
	}
	c, ok := cityNames[v]
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
	if m := monthDayOnlyPattern.FindStringSubmatch(v); m != nil {
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return ReasonDateMissingYear
		}
	}
	return ReasonDateFormat
}
