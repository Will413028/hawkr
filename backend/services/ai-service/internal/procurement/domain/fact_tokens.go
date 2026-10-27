package domain

import (
	"regexp"
	"slices"
	"strings"
)

// 這三個抽取器只服務 usecase 層的 fact-echo validator（#2549 Phase 3）：
// 把一段文字裡「可能是已 commit 事實」的 token 抽成正規化形式，讓 validator
// 用集合比對判斷模型有沒有講出事實清單以外的值。
//
// 放在 domain 而不是 usecase：AllowedCities／AllowedUnits／cityAliases 都在
// 這一層，normalizeCity 未匯出，validator 需要的是「跟 Slots.Apply 同一份
// 值域認知」，不是第二套各自維護的城市／單位表。

var arabicNumberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

// NumberTokensIn 抽出文字裡的阿拉伯數字，依出現順序回傳，並去掉前導零。
//
// 去前導零是必要的正規化：到貨日期存成 ISO 的 "2026-09-30"，模型改寫成
// 「9月30日」是完全正當的敘述，兩者必須抽出同一組 token，否則每一輪帶日期的
// 敘述都會被 fact-echo 誤擋。去零不會讓 50／500 互相涵蓋——validator 用的是
// 逐 token 相等，不是子字串。
func NumberTokensIn(text string) []string {
	raw := arabicNumberPattern.FindAllString(text, -1)
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, tok := range raw {
		out = append(out, stripLeadingZeros(tok))
	}
	return out
}

func stripLeadingZeros(tok string) string {
	intPart, frac, hasFrac := strings.Cut(tok, ".")
	trimmed := strings.TrimLeft(intPart, "0")
	if trimmed == "" {
		trimmed = "0"
	}
	if hasFrac {
		return trimmed + "." + frac
	}
	return trimmed
}

// unitAliases 把買家與模型口語的重量單位對應到 AllowedUnits 的正式值。
// 小寫形式一併收：模型自由文字寫 "kg" 的機率不低，而 AllowedUnits 是大寫。
var unitAliases = map[string]string{
	"公斤": "KG", "kg": "KG", "KG": "KG",
	"台斤": "CATTY", "臺斤": "CATTY", "catty": "CATTY", "CATTY": "CATTY",
	// 刻意不收單字母 "g"、中文單字「克」與小寫 "gram"——三者是同一類漏洞
	// （子字串比對命中更長的無關字詞）：
	// - "g" 會命中任何含小寫 g 的文字（"logo"、"kg" 的大小寫變體等）。
	// - 「克」會命中「克力」（巧克力禮盒、蘭姆葡萄乾巧克力等），這在生鮮採購
	//   域是常態品項名，把整段敘述誤判成講重量單位，導致 fact-echo 誤擋訊息。
	// - 小寫 "gram" 會命中 program／Instagram／telegram／diagram，實測
	//   CanonicalUnitsIn("已排入 program 流程") 會回 [GRAM]。大寫 "GRAM" 保留
	//   （它就是 AllowedUnits 的正式值，不會出現在英文單字中間）。
	// 少收這三個的代價是模型寫 "50g"、"500克"、"500 gram" 時不檢查單位——但那句
	// 話裡的數字仍然會被數字檢查涵蓋。
	"公克": "GRAM", "GRAM": "GRAM",
}

// foldCityVariant 把異體字「臺」摺成「台」。
//
// 這是 fact-echo 專用的正規化，不是 Slots.Apply 的值域改動：cityAliases 把
// 「台北」→「台北市」、「臺北」→「臺北市」映成兩個不同的 canonical 值，
// AllowedCities 也同時收了兩種寫法，兩者都是欄位驗證的既有語意、不動。
// 但 fact-echo 用集合精確相等比對，不摺的話 committed「臺北市」而模型寫
// 「台北」就會算成不同 token → fact_mismatch → 整輪退回模板。受影響的是
// 臺北／臺中／臺南／臺東四個主要都會，而 delivery_city 是必填欄位，記錄它
// 的那一輪必定進到這個檢查。摺向「台」只是擇一，反向等價，但兩端必須一致。
func foldCityVariant(s string) string {
	return strings.ReplaceAll(s, "臺", "台")
}

// CanonicalCitiesIn 回傳文字裡出現過的縣市，正規化成 AllowedCities 的正式值
// （再經 foldCityVariant 收斂異體字），去重後依 AllowedCities 順序回傳。
//
// 別名優先於直接匹配，且優先長別名：先按別名長度（長→短）排序，掃描文字中出現的別名，
// 當已匹配某個別名時跳過所有是其子字符的別名（例如「台北縣」優先於「台北」）。
// 接著掃描 AllowedCities 的正式值，已有的不再加入。這樣「台北縣」→新北市、「台北」→台北市
// 這組別名與正式值不會同時出現。
func CanonicalCitiesIn(text string) []string {
	text = foldCityVariant(text)
	seen := map[string]bool{}
	var out []string
	add := func(city string) {
		city = foldCityVariant(city)
		if !seen[city] {
			seen[city] = true
			out = append(out, city)
		}
	}
	// 先掃別名，按長度從長到短排序，優先匹配更具體的別名。
	// 例如「台北縣」會優先於「台北」被匹配。
	// 當已經匹配到某個別名時，跳過所有是它子字符的別名。
	aliases := make([]string, 0, len(cityAliases))
	for alias := range cityAliases {
		aliases = append(aliases, alias)
	}
	slices.SortFunc(aliases, func(a, b string) int {
		// 按長度降序排列。Invariant：map key 不重複、相異等長字串不可互為子字符，
		// 故不穩定排序在這裡不影響 skip 判斷結果。「臺」與「台」摺疊後長度不變
		// （兩者都是 3 bytes），排序結果不受影響；摺疊後同形的兩個別名（「臺北」
		// 與「台北」）由 matchedAliases 的子字符檢查與 seen 各擋一次，不會重複加入。
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return 0
	})
	matchedAliases := make(map[string]bool)
	for _, alias := range aliases {
		folded := foldCityVariant(alias)
		if !strings.Contains(text, folded) {
			continue
		}
		// 檢查是否已經匹配過包含此別名的更長別名
		skip := false
		for matched := range matchedAliases {
			if strings.Contains(matched, folded) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		matchedAliases[folded] = true
		if canonical, ok := normalizeCity(alias); ok {
			add(canonical)
		}
	}
	// 再掃 AllowedCities，保持順序
	for _, city := range AllowedCities {
		if strings.Contains(text, foldCityVariant(city)) {
			add(city)
		}
	}
	return out
}

// CanonicalUnitsIn 回傳文字裡出現過的重量單位，正規化成 AllowedUnits 的正式值。
func CanonicalUnitsIn(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, unit := range AllowedUnits {
		for alias, canonical := range unitAliases {
			if canonical != unit {
				continue
			}
			if strings.Contains(text, alias) && !seen[unit] {
				seen[unit] = true
				out = append(out, unit)
			}
		}
	}
	return out
}
