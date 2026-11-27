// Package hashtag 是貼文標籤的正規化與驗證：規則在 platform/hashtag（商品共用），這裡只把違規的種類換成 social 的使用者面文字。
package hashtag

import (
	"errors"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/hashtag"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	// MaxLength 是每個標籤的字元數上限（含）。
	MaxLength = hashtag.MaxLength
	// MaxCount 是每篇貼文的標籤數上限（含）。
	MaxCount = hashtag.MaxCount
)

// Tag 是正規化後的標籤：Name 是 ASCII 小寫的名稱（比對與去重用），Display 是 NFC 的樣子（顯示用）。
type Tag = hashtag.Tag

// Normalize 對一個輸入做正規化，見 platform/hashtag。
func Normalize(raw string) (name, display string, ok bool) { return hashtag.Normalize(raw) }

func invalid(l uitext.Locale, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, hashtag.ErrEmpty):
		return apperr.InvalidInput("%s", text.HashtagEmpty(l))
	case errors.Is(err, hashtag.ErrInvalidChars):
		return apperr.InvalidInput("%s", text.HashtagInvalidChars(l))
	case errors.Is(err, hashtag.ErrLength):
		return apperr.InvalidInput("%s", text.HashtagLength(l, text.HashtagLengthArgs{Max: MaxLength}))
	case errors.Is(err, hashtag.ErrTooMany):
		return apperr.InvalidInput("%s", text.HashtagTooMany(l, text.HashtagTooManyArgs{Max: MaxCount}))
	}
	return err
}

// ValidName 檢查一個已正規化的名稱：字元集與長度。
func ValidName(l uitext.Locale, name string) error { return invalid(l, hashtag.ValidName(name)) }

// RequireSet 對一組輸入逐一正規化與驗證，依名稱去重；任何一個無效、或去重後超過 MaxCount 個都回 InvalidInput。輸入是空的回 nil。
func RequireSet(l uitext.Locale, raw []string) ([]Tag, error) {
	tags, err := hashtag.RequireSet(raw)
	if err != nil {
		return nil, invalid(l, err)
	}
	return tags, nil
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
