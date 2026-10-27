// Package trimtext 是單體裡「頭尾沒有空白的文字」唯一的定義。空白是 Unicode White_Space，也就是 Go 的 unicode.IsSpace（25 個碼位）。
//
// 寫入端以 Trim 正規化；DB 以 Check(欄位) 展開的 CHECK 當後備，兩者是同一個集合。一參數的 btrim 只去 U+0020，tab、NBSP、U+3000
// 都會通過，所以模組的 schema 與 query 不用 btrim/ltrim/rtrim/trim。規則 2 不允許跨 schema 的 domain 或函式，每個模組的 schema.sql
// 各自寫一份 Check 的展開結果；cmd/hawkr 的 TestTrimmedTextChecksUseTheCanonicalPattern 對 migration 套用後的每個 CHECK 與索引
// 檢查只用這個寫法，並以 SQL 直接寫入每個欄位的空白值確認被拒絕；internal/arch 的 TestModuleSQLDoesNotTrim 擋 schema 與 query 的
// trim 函式。
package trimtext

import (
	"fmt"
	"strings"
	"unicode"
)

// SpaceClass 是 Postgres 正則的方括號表示式，逐一列出 Unicode White_Space 的碼位（反斜線 u 加四位小寫十六進位，不用範圍、字元類別或
// 不分大小寫，比對逐碼位相等，與 collation、locale 無關；需要 standard_conforming_strings=on，Postgres 的預設）。由 unicode.White_Space
// 展開而不是手抄，TestSpaceClassIsUnicodeIsSpace 鎖住它與 unicode.IsSpace（strings.TrimSpace 的集合）相同。
var SpaceClass = spaceClass()

// Pattern 比對開頭或結尾是空白的文字。
var Pattern = `^` + SpaceClass + `|` + SpaceClass + `$`

// Check 回傳 column 頭尾沒有空白的 CHECK 條件（schema.sql 的寫法）。空字串會通過，非空白另以 char_length 檢查：非空且頭尾不是空白就不是空白。
func Check(column string) string { return column + " !~ '" + Pattern + "'" }

// Trim 去掉頭尾的空白（與 SpaceClass 同一個集合：strings.TrimSpace 就是以 unicode.IsSpace 去頭尾）。寫入受 Check 保護的欄位前一律經過它。
func Trim(s string) string { return strings.TrimSpace(s) }

func spaceClass() string {
	var b strings.Builder
	b.WriteByte('[')
	for _, r := range unicode.White_Space.R16 {
		for c := r.Lo; c <= r.Hi; c += r.Stride {
			fmt.Fprintf(&b, "\\u%04x", c)
		}
	}
	for _, r := range unicode.White_Space.R32 {
		for c := r.Lo; c <= r.Hi; c += r.Stride {
			fmt.Fprintf(&b, "\\u%04x", c)
		}
	}
	b.WriteByte(']')
	return b.String()
}
