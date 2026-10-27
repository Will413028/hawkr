package agent

// ReturnPolicyFacts is the single source of truth for hawkr's buyer
// return-policy content: the eligibility window, packaging/refrigeration
// requirements, the application steps, and refund timing.
//
// This used to be three independently hand-maintained copies that had
// already drifted from each other in wording (and, at points in this
// service's history, in substance — see task-7-report.md for the verbatim
// "before" text of each copy this replaces):
//   - buyerChatInstruction（隨 all-tools fallback 於 2026-09-20 一併移除）
//   - coordinator.go's buyerCoordinatorInstruction (further condensed into
//     a single sentence, embedded in the buyer coordinator's LLM system
//     prompt)
//   - internal/handler/buyer_assistant.go's ReturnPolicyText (the literal
//     SSE reply text sent when a buyer taps the return-policy button)
//
// All three now interpolate this constant instead of hand-copying it, so a
// future policy change (e.g. a refund-timing SLA change) only has one place
// to edit. It lives in this package (not internal/handler) because
// internal/handler already imports internal/agent — the reverse would be an
// import cycle.
const ReturnPolicyFacts = `📋 退貨條件：
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
• ATM轉帳：3-5 個工作天`
