package slots

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

var mergedCounties = map[string]string{
	"臺北縣": "新北市", "桃園縣": "桃園市", "臺中縣": "臺中市", "臺南縣": "臺南市", "高雄縣": "高雄市",
}

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

func normalizeCity(raw string) (string, bool) {
	v := strings.ReplaceAll(raw, "台", "臺")
	if c, ok := mergedCounties[v]; ok {
		return c, true
	}
	c, ok := cityNames[v]
	return c, ok
}

var fullDatePattern = regexp.MustCompile(`^(\d{4})[-/.年](\d{1,2})[-/.月](\d{1,2})日?$`)

var monthDayOnlyPattern = regexp.MustCompile(`^(\d{1,2})[-/.月](\d{1,2})日?$`)

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
