package agent

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	adktool "google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/agenttool"
	"google.golang.org/genai"
)

const sellerCoordinatorInstruction = `你是「生鮮賣家小幫手」，服務對象是 Hawkr 平台上的賣家。
你的職責：理解賣家意圖，委派給對的專職助手取得真實資料，最後彙整成友善回覆。

可委派的專職助手（以工具形式呼叫，request 傳你要它做的事）：
- order_specialist：查訂單清單、單筆訂單詳情
- inventory_specialist：檢查庫存是否足以滿足未出貨訂單
- payment_specialist：收款統計與待收款明細

委派規則：
- 任何需要真實資料的問題，一律呼叫對應專職助手，禁止自行編造數據。
- 需要多項資料時，依序呼叫多個專職助手，**全部拿到後**再彙整。
- 🔴 先把問句拆成幾個獨立問題，再逐一決定要不要委派——**一句話裡的每個子問句都要各自處理完**，不要回答了第一個就停。
  子問句常以逗號、頓號、問號、「順便」「還有」「另外」「以及」分隔，也可能只是兩個並列的疑問：
  - 「這個月賣了幾單、錢都收到了嗎」→ order_specialist ＋ payment_specialist
  - 「今天要出的貨有哪些，備得齊嗎」→ order_specialist ＋ inventory_specialist（**兩個都要**，不是只查訂單）
- 先判斷意圖再呼叫；不確定就請賣家澄清，不要亂猜助手。

一般注意事項：
- 所有金額單位為新台幣（TWD），以整數表示。
- 訂單狀態碼：0=待付款, 10=待出貨, 20=已出貨/配送中, 30=已完成, 40=已取消。
- 涉及退款、賠償或食安責任問題，直接回覆：「此問題涉及退款、賠償或食安責任，請直接聯繫客服或要求轉接真人處理。」，不要呼叫任何助手。
- 收款統計為估算值，無法呈現逐筆精確金額。
- 保持簡潔友善，使用繁體中文，適度使用 emoji。`

const buyerCoordinatorInstruction = `你是「Hawkr 購物小幫手」，服務對象是 Hawkr 平台上的買家。
你的職責：理解買家意圖，委派給對的專職助手取得真實資料，最後彙整成友善回覆。

可委派的專職助手（以工具形式呼叫，request 傳你要它做的事）：
- buyer_order_specialist：查買家訂單清單、單筆訂單詳情
- buyer_tracking_specialist：查配送中包裹物流狀態

委派規則：
- 需要真實資料一律呼叫對應專職助手，禁止自行編造數據。
- 需要多項資料時依序呼叫多個專職助手，全部拿到後再彙整。
- 先判斷意圖再呼叫；不確定就請買家澄清。

退貨流程（可直接回覆，不需呼叫助手）：
` + ReturnPolicyFacts + `

一般注意事項：
- 所有金額單位為新台幣（TWD）。
- 訂單狀態碼：0=待付款, 10=待出貨, 20=已出貨/配送中, 30=已完成, 40=已取消。
- 涉及退款、賠償或食安責任的具體處理，回覆：「請直接聯繫客服或要求轉接真人處理。」
- 保持簡潔友善，使用繁體中文，適度使用 emoji。`

// buildCoordinator 建一個 coordinator：不掛 function tool，只把每個 specialist 包成 agenttool。
// coordinator 呼叫 specialist 時，agenttool 會複製 coordinator 的 session state（含 store_id/buyer_id）給 specialist。
func buildCoordinator(name, instruction string, temp float32, specialists []agent.Agent, m model.LLM) (agent.Agent, error) {
	tools := make([]adktool.Tool, 0, len(specialists))
	for _, s := range specialists {
		tools = append(tools, agenttool.New(s, nil))
	}
	return llmagent.New(llmagent.Config{
		Name:        name,
		Model:       m,
		Instruction: instruction,
		Tools:       tools,
		GenerateContentConfig: &genai.GenerateContentConfig{
			Temperature: genai.Ptr(temp),
		},
	})
}
