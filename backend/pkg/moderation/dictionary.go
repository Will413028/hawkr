// Package moderation provides write-path content filtering for chat, post,
// and review services. Phase 1 uses a hardcoded profanity dictionary;
// Phase 2 will switch to a DB-driven word list with admin CRUD.
package moderation

import "strings"

// Words is the active profanity / hate-speech dictionary used by Check.
// Phase 1 baseline：工程端先頂著用的高信心詞，正式清單仍待 Will／法遵
// 覆核擴充（B-135）。Modify this slice and redeploy to update.
//
// 「殺你」「廢物」「刷單」已於 2026-07-25 移除：substring match 在食品電商情境
// 下會誤傷正常文案（"秒殺你的荷包"、"廢物利用環保餐盒"、"不刷單"），詞庫做
// token-boundary 或更精準比對前先拿掉，避免正常內容被擋。
var Words = []string{
	// 髒話／人身攻擊
	"白痴", "智障", "垃圾人", "去死", "賤人", "婊子", "王八蛋",
	// 色情／援交
	"約砲", "一夜情", "援交", "色情片",
	// 詐騙／招攬（食品電商聊天情境常見）
	"境外匯款", "保證獲利", "私訊下單免上架",
	// 暴力威脅
	"砍你", "找人打你",
}

// CheckAgainst reports whether text contains any word from the supplied list.
// Comparison is case-insensitive substring match. Returns the matching words
// (in their original dictionary form) for audit logging.
// Empty words in the list are skipped to avoid universal false positives.
func CheckAgainst(text string, words []string) (bool, []string) {
	lowerText := strings.ToLower(text)
	var hits []string
	for _, w := range words {
		if w == "" {
			continue
		}
		if strings.Contains(lowerText, strings.ToLower(w)) {
			hits = append(hits, w)
		}
	}
	return len(hits) > 0, hits
}

// Check uses the package-level Words dictionary.
func Check(text string) (bool, []string) {
	return CheckAgainst(text, Words)
}
