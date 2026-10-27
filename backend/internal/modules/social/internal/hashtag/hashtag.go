// Package hashtag 是貼文標籤的正規化與驗證（舊 social 的 domain/hashtag）：client 明確送標籤，伺服器正規化（去頭尾空白與開頭的 #、
// NFC、ASCII 小寫）並驗證字元集（a-z、0-9 與漢字）、每個長度（1 到 40 個字元）與每篇的數量（最多 10 個）。
package hashtag

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	// MaxLength 是每個標籤的字元數上限（含）。
	MaxLength = 40
	// MaxCount 是每篇貼文的標籤數上限（含）。
	MaxCount = 10
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

// ValidName 檢查一個已正規化的名稱：字元集與長度。
func ValidName(l uitext.Locale, name string) error {
	for _, r := range name {
		if !allowed(r) {
			return apperr.InvalidInput("%s", text.HashtagInvalidChars(l))
		}
	}
	if n := utf8.RuneCountInString(name); n < 1 || n > MaxLength {
		return apperr.InvalidInput("%s", text.HashtagLength(l, text.HashtagLengthArgs{Max: MaxLength}))
	}
	return nil
}

// RequireSet 對一組輸入逐一正規化與驗證，依名稱去重（保留第一次出現的樣子與順序）。輸入是空的回 nil；任何一個無效、或去重後超過
// MaxCount 個都回 InvalidInput。
func RequireSet(l uitext.Locale, raw []string) ([]Tag, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]Tag, 0, len(raw))
	for _, r := range raw {
		name, display, ok := Normalize(r)
		if !ok {
			return nil, apperr.InvalidInput("%s", text.HashtagEmpty(l))
		}
		if err := ValidName(l, name); err != nil {
			return nil, err
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, Tag{Name: name, Display: display})
	}
	if len(out) > MaxCount {
		return nil, apperr.InvalidInput("%s", text.HashtagTooMany(l, text.HashtagTooManyArgs{Max: MaxCount}))
	}
	return out, nil
}

// SearchPattern 把查詢字串組成 LIKE 的樣式：先正規化並驗證字元集（沒有 LIKE 的萬用字元 % 與 _，SD19），前綴是「名稱%」，
// 包含是「%名稱%」。輸入正規化後是空的回 ok 為假（沒有建議，不是錯誤）。
func SearchPattern(l uitext.Locale, q string, contains bool) (pattern string, ok bool, err error) {
	name, _, nonEmpty := Normalize(q)
	if !nonEmpty {
		return "", false, nil
	}
	if err := ValidName(l, name); err != nil {
		return "", false, err
	}
	if contains {
		return "%" + name + "%", true, nil
	}
	return name + "%", true, nil
}
