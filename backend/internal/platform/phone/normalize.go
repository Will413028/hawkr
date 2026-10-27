// Package phone 提供台灣聯絡電話處理。
package phone

import "strings"

// NormalizeToLocal 移除常見符號，將台灣國碼轉為本地格式。
func NormalizeToLocal(value string) string {
	value = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(value)
	if strings.HasPrefix(value, "+886") {
		return "0" + strings.TrimPrefix(value[4:], "0")
	}
	if strings.HasPrefix(value, "886") {
		return "0" + strings.TrimPrefix(value[3:], "0")
	}
	return value
}

// Valid 檢查正規化後的台灣聯絡電話格式。
func Valid(value string) bool {
	if len(value) < 9 || len(value) > 10 || value[0] != '0' {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// areaCodes 是台灣的區碼（含開頭的 0，中華電信的區碼表），依長度由長到短比對：烏坵與馬祖是 4 碼、金門 3 碼，其餘縣市 1 至 3 碼。
// 0800、0809 是免付費與語音服務的 4 碼開頭（最長的前綴優先，不會被當成 08 區碼）；09 是行動電話的開頭，格式同市話（開頭 2 碼加號碼）。
var areaCodes = []string{"0800", "0809", "0826", "0836", "082", "037", "049", "089", "02", "03", "04", "05", "06", "07", "08", "09"}

// FormatWithAreaCode 把正規化後的台灣電話（NormalizeToLocal 的結果，可帶「#分機」）轉成「區碼-電話號碼」，有分機時接「#分機」，
// 例：0298887777#123 → 02-98887777#123。區碼不在表裡、號碼不是 5 到 8 碼的數字、或分機不是 1 到 6 碼的數字時 ok 為 false。
func FormatWithAreaCode(value string) (formatted string, ok bool) {
	number, extension, hasExtension := strings.Cut(value, "#")
	if !digits(number) || (hasExtension && (len(extension) < 1 || len(extension) > 6 || !digits(extension))) {
		return "", false
	}
	for _, code := range areaCodes {
		if !strings.HasPrefix(number, code) {
			continue
		}
		subscriber := number[len(code):]
		if len(subscriber) < 5 || len(subscriber) > 8 {
			return "", false
		}
		formatted = code + "-" + subscriber
		if hasExtension {
			formatted += "#" + extension
		}
		return formatted, true
	}
	return "", false
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
