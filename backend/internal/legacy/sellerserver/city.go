package sellerserver

import "strings"

// city 是舊契約的縣市：ISO 3166-2:TW 代碼、舊的正規名稱（用「台」）與內政部的 5 碼縣市代碼（單體的資料用它，platform/twaddr）。
// 舊 seller-service 的對照表在 pkg/twaddr，單體 import 不到，所以對照放在這個 adapter；舊契約退場時一起刪除。
type city struct{ ISO, Name, CountyCode string }

// cities 的順序是舊契約的固定顯示順序（ListPublicStoresByCity 一律回 22 筆，含沒有店家的縣市）。
var cities = [...]city{
	{"TW-KEE", "基隆市", "10017"}, {"TW-TPE", "台北市", "63000"}, {"TW-NWT", "新北市", "65000"}, {"TW-TAO", "桃園市", "68000"},
	{"TW-HSZ", "新竹市", "10018"}, {"TW-HSQ", "新竹縣", "10004"}, {"TW-MIA", "苗栗縣", "10005"}, {"TW-TXG", "台中市", "66000"},
	{"TW-CHA", "彰化縣", "10007"}, {"TW-NAN", "南投縣", "10008"}, {"TW-YUN", "雲林縣", "10009"}, {"TW-CYI", "嘉義市", "10020"},
	{"TW-CYQ", "嘉義縣", "10010"}, {"TW-TNN", "台南市", "67000"}, {"TW-KHH", "高雄市", "64000"}, {"TW-PIF", "屏東縣", "10013"},
	{"TW-ILA", "宜蘭縣", "10002"}, {"TW-HUA", "花蓮縣", "10015"}, {"TW-TTT", "台東縣", "10014"}, {"TW-PEN", "澎湖縣", "10016"},
	{"TW-KIN", "金門縣", "09020"}, {"TW-LIE", "連江縣", "09007"},
}

// cityOf 以縣市名稱（台、臺視為相同）或 ISO 代碼（不分大小寫）找縣市。
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
