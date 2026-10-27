package phone

import "strings"

// NormalizeToLocal 將台灣手機號碼正規化為本地格式（09XXXXXXXX）。
// 處理 +886、886 前綴，並清除空白、橫線、括號等符號。
// 空字串回傳空字串。
func NormalizeToLocal(p string) string {
	if p == "" {
		return ""
	}

	// 清除常見符號
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	p = strings.ReplaceAll(p, "(", "")
	p = strings.ReplaceAll(p, ")", "")

	// +886912345678 → 0912345678
	if strings.HasPrefix(p, "+886") {
		return "0" + p[4:]
	}
	// 886912345678 → 0912345678
	if strings.HasPrefix(p, "886") {
		return "0" + p[3:]
	}

	return p
}
