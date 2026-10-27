package validation

import (
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// RequireVersion 要求修改時帶讀取時拿到的版本（ADR 2026-09-25-optimistic-concurrency-etag D2）。
func RequireVersion(what string, v int64) error {
	if v <= 0 {
		return apperr.InvalidInput("修改%s必須帶讀取時的版本（etag）", what)
	}
	return nil
}

// RequireText 要求必填的文字欄位不是空白。field 是模組輸入的欄位名，label 是給使用者看的名稱。
//
// 文字欄位是 text，不設長度上限：只有產品規格訂了上限的欄位另外檢查（banner 的按鈕文字），
// request 的大小由傳輸層限制。
func RequireText(field, label, v string) error {
	if strings.TrimSpace(v) == "" {
		return apperr.InvalidInputOn([]string{field}, "%s 必填", label)
	}
	return nil
}
