package httputil

import (
	"fmt"
	"strconv"
	"strings"
)

// ContentDisposition 依 RFC 6266 組出 Content-Disposition 的值，同時輸出兩個參數：
// {@code filename=} 給不認得 RFC 5987 的舊 client（非 ASCII 換成底線），
// {@code filename*=} 才是真正的檔名。
//
// 🔴 **為什麼不能只給 filename=**：那個參數是 ISO-8859-1 quoted-string，非 ASCII 的
// 唯一合規表達是 filename*。只給前者的話中文檔名會被瀏覽器亂碼或截斷——而會走到這裡的
// 檔名，多半正是使用者自己上傳、自己要認得的那一個（backend#3149）。
//
// ⚠️ **呼叫端要先剝掉會破壞 header 的字元**（引號、反斜線、CR/LF）。本函式不做那件事：
// 各服務對「剝掉之後變成空字串」有不同的預設名，而那是業務決定不是編碼問題。
//
// disposition 傳 "attachment" 或 "inline"；fallback 是 ASCII 退化名整個沒有資訊時
// （例如純中文檔名）要用的替代名。
func ContentDisposition(disposition, fileName, fallback string) string {
	return disposition + "; filename=" + strconv.Quote(asciiFallbackName(fileName, fallback)) +
		"; filename*=UTF-8''" + rfc5987Encode(fileName)
}

// asciiFallbackName 產生 filename= 用的退化名：非 ASCII 與控制字元一律換底線。
// 認得 filename* 的 client 不會用到它。
func asciiFallbackName(s, fallback string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x7F {
			b.WriteByte('_')
		} else {
			b.WriteByte(c)
		}
	}
	out := b.String()
	if strings.Trim(out, "_") == "" {
		// 整個檔名都是非 ASCII（純中文檔名很常見）⇒ 退化名沒有任何資訊。
		return fallback
	}
	return out
}

// rfc5987Encode 依 RFC 5987 §3.2 把字串編成 ext-value：attr-char 以外的位元組全部
// percent-encode。
//
// ⚠️ **不能用 url.PathEscape 代替**——它放行 $ & + , : ; = ? @ 等字元，而其中
// , : ; = ? @ 在 attr-char 之外。
func rfc5987Encode(s string) string {
	const attrChar = "!#$&+-.^_`|~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			strings.IndexByte(attrChar, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
