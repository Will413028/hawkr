package buyeragent

import (
	"fmt"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	adktool "google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/agenttool"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// AppName 是 ADK 的 app 名稱，session 以 (AppName, 買家 ID, session ID) 為鍵。
const AppName = "hawkr_ai"

// temperature 是所有買家 agent 的取樣溫度。
const temperature = float32(0.3)

// 系統提示的共用段落。提示本身是給模型的指令（繁體中文），買家看到的語言由 replyRule 決定。

// statusGuide 是訂單狀態碼表，來源是 CodeMeta〈業務狀態碼〉的 OrderStatus（擁有者是 ordering）；
// tool 回給模型的 status 是這些代碼，不是任何語言的標籤。
const statusGuide = `訂單狀態碼（tool 回傳的 status 是代碼）：0=待付款, 10=已付款, 20=待出貨, 30=配送中, 32=待取貨, 35=物流異常, 40=已送達（待買家確認收貨）, 50=已完成, 54=配送失敗未退款, 60=部分退款, 70=全額退款, 80=已取消。回覆時把代碼說成買家語言的狀態名稱，不要直接顯示代碼。`

// toolErrorGuide 說明 tool 失敗時回的代碼；下游的錯誤文字不會給模型。
const toolErrorGuide = `tool 失敗時回傳 {"error": 代碼}：upstream_unavailable=訂單服務暫時不可用、order_not_found=查無這筆訂單、forbidden=無權查詢、tool_failed 與 delegate_failed=查詢失敗、unknown_tool=呼叫了不存在的工具。遇到錯誤時用買家的語言簡短說明查不到或請稍後再試，不要顯示代碼。`

// replyRule 是回覆語言規則（A9-D15）：以買家這則訊息使用的語言回覆，判斷不了才用請求的語系。
func replyRule(loc uitext.Locale) string {
	fallback := "繁體中文"
	if loc == uitext.EN {
		fallback = "English"
	}
	return "以買家這則訊息使用的語言回覆；無法判斷時使用" + fallback + "。適度使用 emoji。"
}

func orderSpecialistInstruction(loc uitext.Locale) string {
	return `你是買家訂單查詢助手。依 request 使用 get_my_orders 查訂單、get_buyer_order_detail 查單筆詳情，不需要確認。
` + statusGuide + `
` + toolErrorGuide + `
` + replyRule(loc)
}

func trackingSpecialistInstruction(loc uitext.Locale) string {
	return `你是買家包裹追蹤助手。收到訊息後直接使用 track_my_package 工具查詢包裹（已建物流單到配送結束前後的訂單）的物流狀態，不需要確認。每個包裹帶自己的 status 代碼，依它回答買家，不要把所有包裹都說成配送中。tool 回傳 note=no_packages_in_transit 表示目前沒有可追蹤的包裹；note=some_packages_unavailable 表示有部分包裹的詳情暫時查不到，清單不完整，要告訴買家並建議稍後再查。
` + statusGuide + `
` + toolErrorGuide + `
` + replyRule(loc)
}

// coordinatorInstruction 是買家 coordinator 的系統提示。退貨流程取請求語系的共用文案，與固定回覆同一份來源。
func coordinatorInstruction(loc uitext.Locale) string {
	return `你是「Hawkr 購物小幫手」，服務對象是 Hawkr 平台上的買家。
你的職責：理解買家意圖，委派給對的專職助手取得真實資料，最後彙整成友善回覆。

可委派的專職助手（以工具形式呼叫，request 傳你要它做的事）：
- buyer_order_specialist：查買家訂單清單、單筆訂單詳情
- buyer_tracking_specialist：查包裹物流狀態

委派規則：
- 需要真實資料一律呼叫對應專職助手，禁止自行編造數據。
- 需要多項資料時依序呼叫多個專職助手，全部拿到後再彙整。
- 先判斷意圖再呼叫；不確定就請買家澄清。

退貨流程（可直接回覆，不需呼叫助手）：
` + text.ReturnPolicyFacts(loc).Text + `

一般注意事項：
- 所有金額單位為新台幣（TWD）。
- ` + statusGuide + `
- ` + toolErrorGuide + `
- 涉及退款、賠償或食安責任的具體處理，請買家直接聯繫客服或要求轉接真人處理。
- 保持簡潔友善。` + replyRule(loc)
}

// Deps 是為一輪建構 agent 所需的依賴。
type Deps struct {
	Orders Orders
	Model  model.LLM
	// BuyerID 是這一輪的買家；tool 只服務這個買家。
	BuyerID int64
	// Now 回現在時間（必填）。
	Now func() time.Time
	// Locale 是請求的語系：買家訊息的語言判斷不了時，agent 用它回覆。
	Locale uitext.Locale
	// OnTool 在每次 tool 呼叫結束時被呼叫一次（成功、失敗、參數驗證失敗、panic 都算）；可能從多個 goroutine 同時呼叫。
	OnTool func(ToolCall)
}

// toolNames 是這個 agent 掛的 tool 名稱（agenttool 的名稱是被包的 agent 名稱）；已知名單與解碼端限制都由它導出，只有這一份。
func toolNames(tools []adktool.Tool) map[string]bool {
	names := make(map[string]bool, len(tools))
	for _, t := range tools {
		names[t.Name()] = true
	}
	return names
}

// generateConfig 是每個 agent 的生成設定（A9-D15 第 3 點修訂）：有 tool 的 agent 以 Gemini 的 VALIDATED 模式
// 把可呼叫的函式限制在自己的 tool 名稱，解碼端就不會產生未知的 tool 名稱；事後防線仍在 onToolError。
func generateConfig(tools []adktool.Tool) *genai.GenerateContentConfig {
	cfg := &genai.GenerateContentConfig{Temperature: genai.Ptr(temperature)}
	if len(tools) == 0 {
		return cfg
	}
	allowed := make([]string, 0, len(tools))
	for _, t := range tools {
		allowed = append(allowed, t.Name())
	}
	cfg.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
		Mode:                 genai.FunctionCallingConfigModeValidated,
		AllowedFunctionNames: allowed,
	}}
	return cfg
}

// specialist 只接受 *guardedTool：沒有包過的 tool 掛不上 agent。
func (d Deps) specialist(name, description, instruction string, tools ...*guardedTool) (agent.Agent, error) {
	list := make([]adktool.Tool, 0, len(tools))
	for _, t := range tools {
		list = append(list, t)
	}
	a, err := llmagent.New(llmagent.Config{
		Name:                  name,
		Model:                 d.Model,
		Description:           description,
		Instruction:           instruction,
		Tools:                 list,
		OnToolErrorCallbacks:  []llmagent.OnToolErrorCallback{onToolError(toolNames(list))},
		GenerateContentConfig: generateConfig(list),
	})
	if err != nil {
		return nil, fmt.Errorf("buyeragent：建立 %s：%w", name, err)
	}
	return a, nil
}

// OrdersSpecialist 是「訂單」入口直接使用的 agent。
func (d Deps) OrdersSpecialist() (agent.Agent, error) {
	list, err := d.myOrdersTool()
	if err != nil {
		return nil, err
	}
	detail, err := d.orderDetailTool()
	if err != nil {
		return nil, err
	}
	return d.specialist("buyer_order_specialist", "查詢買家訂單清單與單筆訂單詳情", orderSpecialistInstruction(d.Locale), list, detail)
}

// TrackingSpecialist 是「物流」入口直接使用的 agent。
func (d Deps) TrackingSpecialist() (agent.Agent, error) {
	track, err := d.trackingTool()
	if err != nil {
		return nil, err
	}
	return d.specialist("buyer_tracking_specialist", "查詢買家包裹的物流狀態", trackingSpecialistInstruction(d.Locale), track)
}

// Coordinator 是一般問答使用的 agent：不掛 function tool，兩個 specialist 以 agenttool 包起來；
// agenttool 會把 coordinator 的 session state（含 buyer_id）複製給 specialist。
func (d Deps) Coordinator() (agent.Agent, error) {
	orders, err := d.OrdersSpecialist()
	if err != nil {
		return nil, err
	}
	tracking, err := d.TrackingSpecialist()
	if err != nil {
		return nil, err
	}
	delegates := []adktool.Tool{agenttool.New(orders, nil), agenttool.New(tracking, nil)}
	a, err := llmagent.New(llmagent.Config{
		Name:                  "buyer_coordinator",
		Model:                 d.Model,
		Instruction:           coordinatorInstruction(d.Locale),
		Tools:                 delegates,
		OnToolErrorCallbacks:  []llmagent.OnToolErrorCallback{onToolError(toolNames(delegates))},
		GenerateContentConfig: generateConfig(delegates),
	})
	if err != nil {
		return nil, fmt.Errorf("buyeragent：建立 buyer_coordinator：%w", err)
	}
	return a, nil
}
