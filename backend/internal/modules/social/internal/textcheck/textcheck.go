// Package textcheck 是內容文字（貼文、留言、討論區留言）寫入前的檢查：去頭尾空白、必填、長度、髒字詞典。所有寫入路徑（建立與編輯）
// 都經過同一個入口（SD6：舊碼只擋建立，編輯會繞過）。
package textcheck

import (
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/textfilter"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Kind 是被檢查的文字種類，決定錯誤訊息裡怎麼稱呼它（句子在 social/internal/text）。
type Kind int

const (
	// PostBody 是貼文內文。
	PostBody Kind = iota
	// Comment 是留言與討論區留言。
	Comment
)

// Body 回傳去掉頭尾空白後的 s。required 為真時不得是空的；長度以字元計，不得超過 maxRunes；不得命中髒字詞典。field 是模組輸入的
// 欄位名，kind 決定錯誤訊息的名稱，l 是訊息的語系。
func Body(l uitext.Locale, field string, kind Kind, s string, maxRunes int, required bool) (string, error) {
	if !utf8.ValidString(s) {
		return "", apperr.InvalidInputOn([]string{field}, "%s", invalidText(l, kind))
	}
	s = trimtext.Trim(s)
	if required && s == "" {
		return "", apperr.InvalidInputOn([]string{field}, "%s", requiredText(l, kind))
	}
	if utf8.RuneCountInString(s) > maxRunes {
		return "", apperr.InvalidInputOn([]string{field}, "%s", tooLong(l, kind, maxRunes))
	}
	if hit, _ := textfilter.Check(s); hit {
		return "", apperr.InvalidInputOn([]string{field}, "%s", communityRules(l, kind))
	}
	return s, nil
}

func invalidText(l uitext.Locale, k Kind) uitext.Message {
	if k == PostBody {
		return text.PostBodyInvalidText(l)
	}
	return text.CommentBodyInvalidText(l)
}

func requiredText(l uitext.Locale, k Kind) uitext.Message {
	if k == PostBody {
		return text.PostBodyRequired(l)
	}
	return text.CommentBodyRequired(l)
}

func tooLong(l uitext.Locale, k Kind, max int) uitext.Message {
	if k == PostBody {
		return text.PostBodyTooLong(l, text.PostBodyTooLongArgs{Max: max})
	}
	return text.CommentBodyTooLong(l, text.CommentBodyTooLongArgs{Max: max})
}

func communityRules(l uitext.Locale, k Kind) uitext.Message {
	if k == PostBody {
		return text.PostBodyCommunityRules(l)
	}
	return text.CommentBodyCommunityRules(l)
}
