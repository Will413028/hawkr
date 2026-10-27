package bizcode

import "testing"

// 生成表是否真的有內容。generator 抓錯欄位會讓整張表空掉，而空表在下游是靜默的：
// Message() 回空字串、呼叫端 fallback 顯示原碼，畫面上與「這個碼沒登記」一模一樣。
// generator 端已有「Message 欄為空即中止生成」的檢查，這裡守的是另一半 —— 表被生成
// 出來但對不上碼（欄序改了、篩選條件改了）。
func TestMessage_CoversRepresentativeCodes(t *testing.T) {
	for _, code := range []int32{
		InsufficientStock,                        // Order，最小的碼段
		ShipmentNotFound,                         // Shipping
		DelegationResourceVersionConflict,        // 代行營運，稽核面板實際會顯示的那一組
		DelegationUpstreamUnavailable,            // 同上
		DelegationCapabilityDependencyIncomplete, // 代行營運碼段的尾端
	} {
		if Message(code) == "" {
			t.Errorf("Message(%d) 為空，錯誤碼.md 有登記訊息", code)
		}
	}
}

// 未登錄的碼回空字串，不回 fallback 文案：一句「未知錯誤」會蓋掉呼叫端原本就有的
// 碼值，而顯示原碼至少還追得到。0 同理 —— 它是成功，沒有錯誤訊息可說。
func TestMessage_UnknownCodeReturnsEmpty(t *testing.T) {
	for _, code := range []int32{0, 999999, -1} {
		if got := Message(code); got != "" {
			t.Errorf("Message(%d) = %q，預期空字串", code, got)
		}
	}
}
