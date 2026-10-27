package model

import (
	"net/mail"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

const emailMaskFill = "***"

// IsDeliverableEmail 回傳 s 是不是收得到信的位址：去掉前後空白後是單純的 addr-spec（不接受「名稱 <位址>」），網域含點。
// prospect 的 public_email 是自由文字，會有「待確認」「N/A」「0912…」這類值；對它們發邀請等於靜默失敗。
func IsDeliverableEmail(s string) bool {
	s = trimtext.Trim(s)
	if s == "" {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 {
		return false
	}
	domain := s[at+1:]
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1
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
