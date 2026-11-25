// Package hashtag 是標籤的正規化與驗證，貼文（social）與商品（catalog）共用同一份規則：client 明確送標籤，伺服器正規化（去頭尾空白與
// 開頭的 #、NFC、ASCII 小寫）並驗證字元集（a-z、0-9 與漢字）、每個長度（1 到 40 個字元）與每個內容的數量（最多 10 個）。
// 這個套件只回違規的種類（sentinel 錯誤）；使用者面的文字由呼叫的模組依自己的文案目錄產生。
package hashtag

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

const (
	// MaxLength 是每個標籤的字元數上限（含）。
	MaxLength = 40
	// MaxCount 是每個內容（貼文或商品）的標籤數上限（含）。
	MaxCount = 10
)

// 違規的種類；用 errors.Is 判斷。
var (
	// ErrEmpty 表示標籤正規化後是空的。
	ErrEmpty = errors.New("hashtag: empty")
	// ErrInvalidChars 表示標籤含有不接受的字元。
	ErrInvalidChars = errors.New("hashtag: invalid characters")
	// ErrLength 表示標籤長度不在 1 到 MaxLength 之間。
	ErrLength = errors.New("hashtag: invalid length")
	// ErrTooMany 表示去重後的標籤數超過 MaxCount。
	ErrTooMany = errors.New("hashtag: too many")
)

// Tag 是正規化後的標籤：Name 是 ASCII 小寫的名稱（比對與去重用），Display 是 NFC 的樣子（顯示用，保留使用者輸入的大小寫）。
type Tag struct {
	Name    string
	Display string
}

// Normalize 對一個輸入做正規化：去頭尾空白、去開頭的 #、NFC；名稱再把 ASCII 大寫轉小寫（漢字與全形不動）。輸入是空的或只剩 # 時 ok 為假。
func Normalize(raw string) (name, display string, ok bool) {
	trimmed := trimtext.Trim(raw)
	stripped := strings.TrimLeft(trimmed, "#")
	if stripped == "" {
		return "", "", false
	}
	display = norm.NFC.String(stripped)
	return lowerASCII(display), display, true
}

func lowerASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// allowed 是名稱接受的字元：ASCII 小寫字母、數字與漢字（zh-TW 與 en 都夠用）。
func allowed(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return true
	}
	return unicode.Is(unicode.Han, r)
}

// ValidName 檢查一個已正規化的名稱：字元集（ErrInvalidChars）與長度（ErrLength）。
func ValidName(name string) error {
	for _, r := range name {
		if !allowed(r) {
			return ErrInvalidChars
		}
	}
	if n := utf8.RuneCountInString(name); n < 1 || n > MaxLength {
		return ErrLength
	}
	return nil
}

// RequireSet 對一組輸入逐一正規化與驗證，依名稱去重（保留第一次出現的樣子與順序）。輸入是空的回 nil；任何一個無效回
// ErrEmpty、ErrInvalidChars 或 ErrLength；去重後超過 MaxCount 個回 ErrTooMany。
func RequireSet(raw []string) ([]Tag, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]Tag, 0, len(raw))
	for _, r := range raw {
		name, display, ok := Normalize(r)
		if !ok {
			return nil, ErrEmpty
		}
		if err := ValidName(name); err != nil {
			return nil, err
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, Tag{Name: name, Display: display})
	}
	if len(out) > MaxCount {
		return nil, ErrTooMany
	}
	return out, nil
}
