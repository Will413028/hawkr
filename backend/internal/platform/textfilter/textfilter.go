// Package textfilter 是寫入路徑的髒字詞典檢查：社群的貼文、留言與討論區留言的建立與編輯都呼叫同一個入口 Check（單體規則 4
// 不涉及；platform 不得依賴任何模組）。
//
// 詞典與比對規則照舊的 pkg/moderation：不分大小寫的子字串比對。子字串比對在食品電商情境會誤擋正常文案（「秒殺你的荷包」
// 含「殺你」），舊詞典因此移除過三個詞；下一次要加詞時，先把比對改成 token-boundary 或更精準的比對。詞庫應有擁有者與後台維護，
// 目前只搬現行為（觸發：營運要求新增詞）。
package textfilter

import "strings"

// Words 是髒字詞典。
var Words = []string{
	// 髒話、人身攻擊
	"白痴", "智障", "垃圾人", "去死", "賤人", "婊子", "王八蛋",
	// 色情、援交
	"約砲", "一夜情", "援交", "色情片",
	// 詐騙、招攬
	"境外匯款", "保證獲利", "私訊下單免上架",
	// 暴力威脅
	"砍你", "找人打你",
}

// CheckAgainst 回報 text 是否含 words 裡的任何一個詞（不分大小寫的子字串比對），並依 words 的順序回傳命中的詞（給稽核
// 紀錄）。words 裡的空字串略過，否則每段文字都會命中。
func CheckAgainst(text string, words []string) (bool, []string) {
	lower := strings.ToLower(text)
	var hits []string
	for _, w := range words {
		if w != "" && strings.Contains(lower, strings.ToLower(w)) {
			hits = append(hits, w)
		}
	}
	return len(hits) > 0, hits
}

// Check 以 Words 檢查 text。
func Check(text string) (bool, []string) { return CheckAgainst(text, Words) }
