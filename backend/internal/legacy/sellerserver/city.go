package sellerserver

import "strings"

type city struct{ ISO, Name, CountyCode string }

var cities = [...]city{
	{"TW-KEE", "基隆市", "10017"}, {"TW-TPE", "台北市", "63000"}, {"TW-NWT", "新北市", "65000"}, {"TW-TAO", "桃園市", "68000"},
	{"TW-HSZ", "新竹市", "10018"}, {"TW-HSQ", "新竹縣", "10004"}, {"TW-MIA", "苗栗縣", "10005"}, {"TW-TXG", "台中市", "66000"},
	{"TW-CHA", "彰化縣", "10007"}, {"TW-NAN", "南投縣", "10008"}, {"TW-YUN", "雲林縣", "10009"}, {"TW-CYI", "嘉義市", "10020"},
	{"TW-CYQ", "嘉義縣", "10010"}, {"TW-TNN", "台南市", "67000"}, {"TW-KHH", "高雄市", "64000"}, {"TW-PIF", "屏東縣", "10013"},
	{"TW-ILA", "宜蘭縣", "10002"}, {"TW-HUA", "花蓮縣", "10015"}, {"TW-TTT", "台東縣", "10014"}, {"TW-PEN", "澎湖縣", "10016"},
	{"TW-KIN", "金門縣", "09020"}, {"TW-LIE", "連江縣", "09007"},
}

func cityOf(input string) (city, bool) {
	input = strings.TrimSpace(input)
	name := strings.ReplaceAll(input, "臺", "台")
	for _, c := range cities {
		if c.Name == name || strings.EqualFold(c.ISO, input) {
			return c, true
		}
	}
	return city{}, false
}

func countyCodes() []string {
	out := make([]string, len(cities))
	for i, c := range cities {
		out[i] = c.CountyCode
	}
	return out
}
