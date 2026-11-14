// Package emailaddr 提供信箱位址的正規化與驗證。
package emailaddr

import (
	"net/mail"
	"strings"
	"unicode"
)

// MaxLen 是信箱位址的長度上限（RFC 5321 的路徑上限）。
const MaxLen = 254

// Normalize 去掉前後空白、轉小寫，要求恰好一個 @、兩側非空、沒有空白、至多 254 字，且網域有點（user@localhost 不算）。
// 這些條件與資料庫的信箱 CHECK（contact_email、support_email）一致；mail.ParseAddress 接受的帶引號本地
// 部分（"a@b"@c.com）會多出一個 @，所以要在解析之後再檢查一次。
func Normalize(raw string) (string, bool) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil || addr.Name != "" {
		return "", false
	}
	email := strings.ToLower(addr.Address)
	at := strings.Index(email, "@")
	ok := at > 0 && at < len(email)-1 && strings.Count(email, "@") == 1 &&
		strings.Contains(email[at+1:], ".") && len(email) <= MaxLen &&
		strings.IndexFunc(email, unicode.IsSpace) < 0
	return email, ok
}
