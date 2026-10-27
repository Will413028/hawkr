package twaddr

import "strings"

// TaiwanCities is the canonical 22-city/county display order used by public filters.
var TaiwanCities = []string{
	"基隆市", "台北市", "新北市", "桃園市", "新竹市", "新竹縣", "苗栗縣",
	"台中市", "彰化縣", "南投縣", "雲林縣", "嘉義市", "嘉義縣", "台南市",
	"高雄市", "屏東縣", "宜蘭縣", "花蓮縣", "台東縣", "澎湖縣", "金門縣", "連江縣",
}

var cityCodes = map[string]string{
	"基隆市": "TW-KEE", "台北市": "TW-TPE", "新北市": "TW-NWT", "桃園市": "TW-TAO",
	"新竹市": "TW-HSZ", "新竹縣": "TW-HSQ", "苗栗縣": "TW-MIA", "台中市": "TW-TXG",
	"彰化縣": "TW-CHA", "南投縣": "TW-NAN", "雲林縣": "TW-YUN", "嘉義市": "TW-CYI",
	"嘉義縣": "TW-CYQ", "台南市": "TW-TNN", "高雄市": "TW-KHH", "屏東縣": "TW-PIF",
	"宜蘭縣": "TW-ILA", "花蓮縣": "TW-HUA", "台東縣": "TW-TTT", "澎湖縣": "TW-PEN",
	"金門縣": "TW-KIN", "連江縣": "TW-LIE",
}

var citiesByCode = func() map[string]string {
	m := make(map[string]string, len(cityCodes))
	for city, code := range cityCodes {
		m[code] = city
	}
	return m
}()

var canonicalCities = func() map[string]string {
	m := make(map[string]string, len(TaiwanCities)+4)
	for _, city := range TaiwanCities {
		m[city] = city
	}
	m["臺北市"] = "台北市"
	m["臺中市"] = "台中市"
	m["臺南市"] = "台南市"
	m["臺東縣"] = "台東縣"
	return m
}()

// CanonicalCity normalizes supported 台/臺 aliases and rejects non-Taiwan values.
func CanonicalCity(city string) (string, bool) {
	canonical, ok := canonicalCities[strings.TrimSpace(city)]
	return canonical, ok
}

// CityCode returns the stable ISO 3166-2:TW subdivision code for a city alias.
func CityCode(city string) (string, bool) {
	canonical, ok := CanonicalCity(city)
	if !ok {
		return "", false
	}
	code, ok := cityCodes[canonical]
	return code, ok
}

// CityFromCode resolves an ISO 3166-2:TW code case-insensitively.
func CityFromCode(code string) (string, bool) {
	city, ok := citiesByCode[strings.ToUpper(strings.TrimSpace(code))]
	return city, ok
}

// CityAliases returns persisted spellings accepted for a canonical city during legacy backfill rollout.
func CityAliases(canonical string) []string {
	switch canonical {
	case "台北市":
		return []string{"台北市", "臺北市"}
	case "台中市":
		return []string{"台中市", "臺中市"}
	case "台南市":
		return []string{"台南市", "臺南市"}
	case "台東縣":
		return []string{"台東縣", "臺東縣"}
	default:
		return []string{canonical}
	}
}
