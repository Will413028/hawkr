package agent

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	adktool "google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
)

// specialistSpec 定義一個專職 agent：只掛 1-2 個 tool，讓 tool 選擇決策單純。
// 被按鈕端點（自己的 runner 直接跑）與 chat（coordinator 以 agenttool 呼叫）共用。
type specialistSpec struct {
	Key         string // role 內唯一路由鍵，如 "order"/"inventory"
	Name        string // agent name，會成為 coordinator agenttool 的函式名，如 "order_specialist"
	Description string // 供 coordinator 路由判斷（agenttool 的函式 description）
	Instruction string
	ButtonKey   string // 對應按鈕端點 key；"" = 僅供 chat 委派、不獨立開按鈕
	Temperature float32
	buildTools  func(*ToolDeps) []adktool.Tool
}

const sellerOrderSpecialistInstruction = `你是賣家訂單查詢助手。依使用者的 request，使用 get_seller_orders 查訂單清單、get_order_detail 查單筆詳情。只回傳整理過的查詢結果，不做退款/賠償判斷。使用繁體中文，金額為新台幣整數。`

const inventorySpecialistInstruction = `你是賣家庫存檢查助手。收到訊息後直接使用 check_inventory_shortage 工具檢查庫存，不需要確認。使用繁體中文回覆，適度使用 emoji。`

const paymentSpecialistInstruction = `你是賣家收款對帳助手。收到訊息後直接使用 query_payment_summary 工具查詢收款狀況，不需要確認。收款統計為估算值（基於訂單狀態推算）。使用繁體中文回覆，適度使用 emoji。`

const buyerOrderSpecialistInstruction = `你是買家訂單查詢助手。依 request 使用 get_my_orders 查訂單、get_buyer_order_detail 查單筆詳情，不需要確認。使用繁體中文回覆，適度使用 emoji。`

const buyerTrackingSpecialistInstruction = `你是買家包裹追蹤助手。收到訊息後直接使用 track_my_package 工具查詢配送中包裹的物流狀態，不需要確認。使用繁體中文回覆，適度使用 emoji。`

const buyerSellerMatchSpecialistInstruction = `你是幫買家找供應商的採購助手。讀買家描述的需求，判斷分類（category）、關鍵字（keywords）、地區（region），呼叫 search_candidate_sellers 查詢候選廠商。
若買家的描述完全無法判斷分類或關鍵字，直接反問「想找哪一類商品？」，不要用空條件查詢。
拿到查詢結果後，針對每個候選廠商用 1-2 句話說明為什麼符合買家的需求（依據 positioning、research_summary 的內容），不要只列清單不解釋原因。找不到符合的候選時，誠實告知，並建議買家換個說法再試一次。使用繁體中文回覆。`

func sellerSpecialistSpecs() []specialistSpec {
	return []specialistSpec{
		{
			Key: "order", Name: "order_specialist",
			Description: "查詢賣家訂單清單與單筆訂單詳情（依日期/狀態）",
			Instruction: sellerOrderSpecialistInstruction,
			ButtonKey:   "", Temperature: 0.3,
			buildTools: func(d *ToolDeps) []adktool.Tool {
				return []adktool.Tool{NewGetSellerOrdersTool(d), NewGetOrderDetailTool(d)}
			},
		},
		{
			Key: "inventory", Name: "inventory_specialist",
			Description: "檢查賣家庫存是否足以滿足未出貨訂單，列出不足品項",
			Instruction: inventorySpecialistInstruction,
			ButtonKey:   "inventory", Temperature: 0.3,
			buildTools: func(d *ToolDeps) []adktool.Tool {
				return []adktool.Tool{NewCheckInventoryShortageTool(d)}
			},
		},
		{
			Key: "payment", Name: "payment_specialist",
			Description: "查詢賣家收款統計（已收/待收）與待收款訂單明細",
			Instruction: paymentSpecialistInstruction,
			ButtonKey:   "payment", Temperature: 0.3,
			buildTools: func(d *ToolDeps) []adktool.Tool {
				return []adktool.Tool{NewQueryPaymentSummaryTool(d)}
			},
		},
	}
}

func buyerSpecialistSpecs() []specialistSpec {
	return []specialistSpec{
		{
			Key: "order", Name: "buyer_order_specialist",
			Description: "查詢買家訂單清單與單筆訂單詳情",
			Instruction: buyerOrderSpecialistInstruction,
			ButtonKey:   "orders", Temperature: 0.3,
			buildTools: func(d *ToolDeps) []adktool.Tool {
				return []adktool.Tool{NewGetMyOrdersTool(d), NewGetBuyerOrderDetailTool(d)}
			},
		},
		{
			Key: "tracking", Name: "buyer_tracking_specialist",
			Description: "查詢買家配送中包裹的物流狀態",
			Instruction: buyerTrackingSpecialistInstruction,
			ButtonKey:   "tracking", Temperature: 0.3,
			buildTools: func(d *ToolDeps) []adktool.Tool {
				return []adktool.Tool{NewTrackMyPackageTool(d)}
			},
		},
		{
			Key: "seller_match", Name: "buyer_seller_match_specialist",
			Description: "從開發名單候選廠商中，依買家描述的需求找出可能符合的供應商",
			Instruction: buyerSellerMatchSpecialistInstruction,
			ButtonKey:   "seller-match", Temperature: 0.4,
			buildTools: func(d *ToolDeps) []adktool.Tool {
				return []adktool.Tool{NewSearchCandidateSellersTool(d)}
			},
		},
	}
}

// buildSpecialist 用給定的 tools 建 specialist agent。tools 由呼叫端事先以 spec.buildTools(deps)
// 產出（讓 caller 能同時累積成 fallback all-tools chat 的 tool 集，避免重複建）。
func buildSpecialist(spec specialistSpec, tools []adktool.Tool, m model.LLM) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:        spec.Name,
		Model:       m,
		Description: spec.Description,
		Instruction: spec.Instruction,
		Tools:       tools,
		GenerateContentConfig: &genai.GenerateContentConfig{
			Temperature: genai.Ptr(spec.Temperature),
		},
	})
}
