package uitext

// PluralCategory 是整數 n 在語系 l 的 CLDR plural 類別：zh 只有 other，en 是 one 與 other。
// 生成碼只在訊息有 one 分支時呼叫它。
func PluralCategory(l Locale, n int) string {
	if l == EN && n == 1 {
		return "one"
	}
	return "other"
}
