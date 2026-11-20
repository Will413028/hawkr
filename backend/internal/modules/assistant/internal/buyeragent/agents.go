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

const orderSpecialistInstruction = `你是買家訂單查詢助手。依 request 使用 get_my_orders 查訂單、get_buyer_order_detail 查單筆詳情，不需要確認。使用繁體中文回覆，適度使用 emoji。`

const trackingSpecialistInstruction = `你是買家包裹追蹤助手。收到訊息後直接使用 track_my_package 工具查詢配送中包裹的物流狀態，不需要確認。使用繁體中文回覆，適度使用 emoji。`

// coordinatorInstruction 是買家 coordinator 的系統提示。退貨流程取 zh-TW 的共用文案，與固定回覆同一份來源。
// 疑似缺陷（照搬）：「訂單狀態碼：0／10／20／30／40」與 tool 實際回的狀態標籤不是同一張表。
func coordinatorInstruction() string {
	return `你是「Hawkr 購物小幫手」，服務對象是 Hawkr 平台上的買家。
你的職責：理解買家意圖，委派給對的專職助手取得真實資料，最後彙整成友善回覆。

可委派的專職助手（以工具形式呼叫，request 傳你要它做的事）：
- buyer_order_specialist：查買家訂單清單、單筆訂單詳情
- buyer_tracking_specialist：查配送中包裹物流狀態

委派規則：
- 需要真實資料一律呼叫對應專職助手，禁止自行編造數據。
- 需要多項資料時依序呼叫多個專職助手，全部拿到後再彙整。
- 先判斷意圖再呼叫；不確定就請買家澄清。

退貨流程（可直接回覆，不需呼叫助手）：
` + text.ReturnPolicyFacts(uitext.ZhTW).Text + `

一般注意事項：
- 所有金額單位為新台幣（TWD）。
- 訂單狀態碼：0=待付款, 10=待出貨, 20=已出貨/配送中, 30=已完成, 40=已取消。
- 涉及退款、賠償或食安責任的具體處理，回覆：「請直接聯繫客服或要求轉接真人處理。」
- 保持簡潔友善，使用繁體中文，適度使用 emoji。`
}

// Deps 是為一輪建構 agent 所需的依賴。
type Deps struct {
	Orders Orders
	Model  model.LLM
	// BuyerID 是這一輪的買家；tool 只服務這個買家。
	BuyerID int64
	// Now 回現在時間（必填）。
	Now func() time.Time
	// OnTool 在每支 tool 執行結束時被呼叫；可能從多個 goroutine 同時呼叫。
	OnTool func(ToolCall)
}

func (d Deps) specialist(name, description, instruction string, tools ...adktool.Tool) (agent.Agent, error) {
	a, err := llmagent.New(llmagent.Config{
		Name:                  name,
		Model:                 d.Model,
		Description:           description,
		Instruction:           instruction,
		Tools:                 tools,
		GenerateContentConfig: &genai.GenerateContentConfig{Temperature: genai.Ptr(temperature)},
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
	return d.specialist("buyer_order_specialist", "查詢買家訂單清單與單筆訂單詳情", orderSpecialistInstruction, list, detail)
}

// TrackingSpecialist 是「物流」入口直接使用的 agent。
func (d Deps) TrackingSpecialist() (agent.Agent, error) {
	track, err := d.trackingTool()
	if err != nil {
		return nil, err
	}
	return d.specialist("buyer_tracking_specialist", "查詢買家配送中包裹的物流狀態", trackingSpecialistInstruction, track)
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
	a, err := llmagent.New(llmagent.Config{
		Name:                  "buyer_coordinator",
		Model:                 d.Model,
		Instruction:           coordinatorInstruction(),
		Tools:                 []adktool.Tool{agenttool.New(orders, nil), agenttool.New(tracking, nil)},
		GenerateContentConfig: &genai.GenerateContentConfig{Temperature: genai.Ptr(temperature)},
	})
	if err != nil {
		return nil, fmt.Errorf("buyeragent：建立 buyer_coordinator：%w", err)
	}
	return a, nil
}
