package handler

import (
	agentpkg "github.com/Will413028/hawkr/ai-service/internal/agent"
)

// 退貨流程固定文案。BuyerAssistantHandler.ReturnPolicy（HTTP SSE）已隨 Phase C
// （移除 ai-service HTTP business surface）刪除，現在唯一的消費端是
// internal/grpc/assistant_handler.go 的 ProxyBuyerReturnPolicySSE 對應 RPC。
//
// 政策內容（條件／步驟／退款時間）從 agentpkg.ReturnPolicyFacts 來，跟買家
// coordinator／chat fallback 的 LLM 系統提示共用同一份，不再各自維護一份文案
// （見 internal/agent/return_policy.go 的說明）。這裡只加這個消費端專屬的
// 標題與結尾引導句。
const ReturnPolicyText = `🔄 退貨申請流程說明：

` + agentpkg.ReturnPolicyFacts + `

需要我協助您申請退貨嗎？`

// 買家按鈕端點預設訊息。init() 合併進 DefaultMessages（seller_assistant.go），
// 這個檔案原本的 BuyerAssistantHandler（HTTP handler 本體）已隨 Phase C 刪除。
var buyerDefaultMessages = map[string]string{
	"orders":       "查詢我的訂單",
	"tracking":     "查詢包裹狀態",
	"seller-match": "幫我找適合的供應商",
}

func init() {
	// 將買家預設訊息合併到全域 DefaultMessages
	for k, v := range buyerDefaultMessages {
		if _, exists := DefaultMessages[k]; !exists {
			DefaultMessages[k] = v
		}
	}
}
