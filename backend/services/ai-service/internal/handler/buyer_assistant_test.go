package handler

import (
	"strings"
	"testing"

	agentpkg "github.com/Will413028/hawkr/ai-service/internal/agent"
)

// TestReturnPolicyText_UnchangedAfterConvergingOnSharedSource locks in that
// composing ReturnPolicyText from agentpkg.ReturnPolicyFacts (task #3519
// slice B, Task 7 item 5 — converging three independently hand-maintained
// return-policy copies onto one source) produced byte-for-byte the same
// text buyers already see today. If this ever fails, either the shared
// source changed (update this test deliberately) or the composition here
// silently drifted from the original wording.
func TestReturnPolicyText_UnchangedAfterConvergingOnSharedSource(t *testing.T) {
	want := `🔄 退貨申請流程說明：

📋 退貨條件：
• 收到商品後 24hr 內可申請退貨
• 商品需保持完整包裝及標籤
• 生鮮商品需確保冷藏/冷凍狀態

📝 申請步驟：
1. 已完成訂單點選商品評分，進到商品評分頁的右上角，進入售後服務
2. 上傳商品照片，填寫退貨原因，按下送出申請按鈕
3. 等待賣家回覆審核
4. 審核通過後安排退款

⏰ 退款時間：
• 信用卡：7-14 個工作天
• ATM轉帳：3-5 個工作天

需要我協助您申請退貨嗎？`

	if ReturnPolicyText != want {
		t.Fatalf("ReturnPolicyText drifted from its original wording:\nwant=%q\ngot =%q", want, ReturnPolicyText)
	}
}

// TestReturnPolicyText_EmbedsSharedFacts guards the actual point of this
// convergence: ReturnPolicyText must literally contain
// agentpkg.ReturnPolicyFacts, not a separately hand-copied paraphrase of it.
func TestReturnPolicyText_EmbedsSharedFacts(t *testing.T) {
	if !strings.Contains(ReturnPolicyText, agentpkg.ReturnPolicyFacts) {
		t.Fatal("ReturnPolicyText no longer embeds agentpkg.ReturnPolicyFacts verbatim")
	}
}
