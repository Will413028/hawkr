// Package auditreason 是管理員操作的理由與說明（關店、刪帳號、KYC 審核與顯示全文）的共用規則：去掉頭尾空白（platform/trimtext），必填的是
// 1 到 1000 字，選填的是 0 到 1000 字。規則只有這一處，理由的 DB CHECK（trimtext.Check、長度）與它一致。
package auditreason

import (
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// MaxRunes 是理由的字數上限。
const MaxRunes = 1000

// Required 正規化管理員必須說明的理由：去掉前後空白，空白或超過 MaxRunes 字回 InvalidInput（指向 field）。
func Required(raw, field string) (string, error) {
	reason := trimtext.Trim(raw)
	switch {
	case reason == "":
		return "", apperr.InvalidInputOn([]string{field}, "必須說明理由")
	case utf8.RuneCountInString(reason) > MaxRunes:
		return "", apperr.InvalidInputOn([]string{field}, "理由不可超過 1000 字")
	}
	return reason, nil
}

// Optional 正規化管理員可以不填的說明（例如核准的 comment）：去掉前後空白，空白就是沒有（回空字串），超過 MaxRunes 字回
// InvalidInput（指向 field）。長度的規則與 Required 相同，只差在可以不填。
func Optional(raw, field string) (string, error) {
	text := trimtext.Trim(raw)
	if utf8.RuneCountInString(text) > MaxRunes {
		return "", apperr.InvalidInputOn([]string{field}, "說明不可超過 1000 字")
	}
	return text, nil
}
