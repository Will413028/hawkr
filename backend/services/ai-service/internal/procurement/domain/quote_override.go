package domain

import (
	"fmt"
	"strings"
	"time"
)

// MaxBuyerNoteLen 是 quote-batches 覆蓋欄位 buyerNote 的長度上限（以 rune 計），
// 沿用 MaxTextLen 同一個上限、同一組理由（見 slots.go 的 item/packaging）。
const MaxBuyerNoteLen = MaxTextLen

// ValidateQuoteDeadline 驗證 quote-batches 覆蓋欄位 quoteDeadline：格式必須是
// DateLayout（YYYY-MM-DD），且不早於今天。刻意不跟 deliveryDate 做交叉驗證
// （設計書 2026-08-15 procurement-quote-batches-buyer-overrides §5）。
func ValidateQuoteDeadline(v string, now time.Time) (string, error) {
	v = strings.TrimSpace(v)
	d, err := time.ParseInLocation(DateLayout, v, time.UTC)
	if err != nil {
		return "", fmt.Errorf("格式錯誤，需為 %s", DateLayout)
	}
	if !d.After(now.AddDate(0, 0, -1)) {
		return "", fmt.Errorf("不可早於今天")
	}
	return v, nil
}

// ValidateBuyerNote 驗證 quote-batches 覆蓋欄位 buyerNote：trim 後以 rune 計長，
// 超過 MaxBuyerNoteLen 直接拒絕，不像 domain.Slots.Apply 的自由文字欄位會靜默
// 截斷——quote-batches 是買家明確送出的結構化表單，不該讓他以為送出的內容跟畫面
// 上看到的一致，實際上後端偷偷截斷了一段。trim 後的空字串視為合法（等同沒填）。
func ValidateBuyerNote(v string) (string, error) {
	v = strings.TrimSpace(v)
	if r := []rune(v); len(r) > MaxBuyerNoteLen {
		return "", fmt.Errorf("超過 %d 字", MaxBuyerNoteLen)
	}
	return v, nil
}
