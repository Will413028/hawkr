package model

import (
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/emailaddr"
)

const emailMaskFill = "***"

// DeliverableEmail 回傳 s 正規化後的位址（去前後空白、小寫、同 emailaddr.Normalize），s 不是收得到信的位址時 ok 為 false。
// prospect 的 public_email 是自由文字，會有「待確認」「N/A」「0912…」這類值；對它們發邀請等於靜默失敗。
func DeliverableEmail(s string) (string, bool) {
	return emailaddr.Normalize(s)
}

// MaskEmail 遮住 email 的本地部分，只留第一個字元與網域：給預覽用，讓拿到連結的人認得出寄到哪、又不洩漏完整位址。
// 沒有 @ 或本地部分是空的，整個遮掉。
func MaskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return emailMaskFill
	}
	if at == 1 {
		return emailMaskFill + email[at:]
	}
	return email[:1] + emailMaskFill + email[at:]
}
