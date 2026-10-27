package agent

import (
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	adktool "google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
)

const productAutofillInstruction = `你是商品表單預填助手，負責從賣家提供的文字（或圖片描述）中抽取商品資訊。

規則：
- 只抽取文字中明確出現的資訊，沒有就留空，禁止自行推測或補充
- categoryKeyword 填商品的核心品項關鍵字（例如「芭樂」「高麗菜」「蝦仁」），不要填完整分類路徑
- unit 只能填 KG（公斤）、CATTY（台斤）、GRAM（公克）或 NONE（非重量類規格）
- price 為新台幣元，必須是整數
- 若同時有多個規格或包裝，每個都放一筆 sku
- 輸出語言：繁體中文`

// productDraftSchema 定義 LLM structured output 的 JSON schema
var productDraftSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"name": {
			Type:      genai.TypeString,
			MaxLength: genai.Ptr(int64(60)),
		},
		"origin": {
			Type:      genai.TypeString,
			MaxLength: genai.Ptr(int64(30)),
		},
		"categoryKeyword": {
			Type: genai.TypeString,
		},
		"description": {
			Type:      genai.TypeString,
			MaxLength: genai.Ptr(int64(2000)),
		},
		"skus": {
			Type:     genai.TypeArray,
			MaxItems: genai.Ptr(int64(10)),
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Required: []string{"skuName", "price"},
				Properties: map[string]*genai.Schema{
					"skuName": {
						Type:      genai.TypeString,
						MaxLength: genai.Ptr(int64(40)),
					},
					"price": {
						Type:    genai.TypeInteger,
						Minimum: genai.Ptr(float64(0)),
					},
					"unit": {
						Type: genai.TypeString,
						Enum: []string{"KG", "CATTY", "GRAM", "NONE"},
					},
				},
			},
		},
		"tags": {
			Type:     genai.TypeArray,
			MaxItems: genai.Ptr(int64(8)),
			Items:    &genai.Schema{Type: genai.TypeString},
		},
	},
	PropertyOrdering: []string{"name", "origin", "categoryKeyword", "description", "skus", "tags"},
}

const orderPresenterInstruction = `你是「生鮮賣家小幫手」，負責整理並呈現待出貨訂單資料。

你會收到一段訂單資料（JSON 格式），請：
1. 若沒有訂單，友善告知賣家目前沒有待出貨訂單
2. 若有訂單，條列顯示每筆訂單的編號、買家、商品、金額
3. 語氣友善，使用繁體中文，適度使用 emoji
4. 不需要呼叫任何工具，直接根據提供的資料回覆`

const productListingInstruction = `你是「上架商品 AI 助手」，協助生鮮賣家快速完成商品上架。

## 核心策略：先批量提取，再補充缺漏

### 第一輪（開場問題）
直接問：「您好！請用一段話描述您的商品，例如：『有機香蕉，台灣南投，每台斤 100 元，通常備貨 50 斤，無農藥栽培』。描述越詳細，我幫您填入越多欄位 😊」

### 收到描述後（批量處理，本輪只做一次）
1. 從描述中抽取**所有可推斷的欄位**
2. 若能推斷商品類別，**先呼叫 search_category** 取得 catalogId
3. 將所有確定的欄位用**一次** update_draft 送出（不可多次呼叫）
4. 呼叫後，條列說明已填入哪些欄位（✓），並將**所有缺漏欄位一次列出詢問**，例如：
   「已填入：名稱✓ 產地✓ 規格（台斤/100元）✓。請告訴我：①分類：候選『水果 > 香蕉』確認嗎？②庫存數量？③有什麼特色標籤（例如有機、無毒）？」

### 後續輪次（補充缺漏）
每輪可同時問多個未填欄位，賣家回覆後再次一次呼叫 update_draft 補齊

## 欄位清單
- 商品名稱（name，max 60 字元）
- 分類（categoryId，**必須**先呼叫 search_category 拿候選，讓賣家確認後才填）
- 產地（origin）
- 商品規格（skus：每筆含 skuName、price、unit、weight、stockQty）
- 商品描述（description，max 2000 字元）
- 商品標籤（tags，最多 8 個）
- 農業認證（certifications，選填）

## 工具使用規則
1. categoryId **必須**來自 search_category 回傳的 catalogId，禁止自行猜測
2. update_draft 每輪只呼叫**一次**，送出本輪所有新確定的欄位（未變動者省略）
3. skus 是覆蓋式更新，有變動時請送出完整 SKU 清單
4. search_category 若回傳空結果，最多換一個相近關鍵字再試一次；第二次仍空，告知賣家手動選擇，update_draft 略過 categoryId

## 完成條件
主要欄位齊全（name + 至少 1 筆 SKU）時，提示「資料已齊全，請關閉對話、按『下一步』檢視並送出」；若 categoryId 未設定，補充說明「分類請在下一步手動選擇」

## 注意事項
- 語氣友善，使用繁體中文，適度使用 emoji
- 價格為新台幣整數元
- unit 只能填：KG / CATTY / GRAM / NONE（非重量類規格）`

// Agents 包含 coordinator、specialist 與保留的獨立 agent。
type Agents struct {
	// key = "seller:<specKey>" / "buyer:<specKey>"
	Specialists map[string]agent.Agent
	// key = "seller" / "buyer"
	Coordinators map[string]agent.Agent
	// Phase 1 保留不動
	OrderPresenter  agent.Agent // /seller/assistant/orders 特例流程用
	ProductAutofill agent.Agent
	ProductListing  agent.Agent
}

// buyerCoordinatorExcluded 列出「只走按鈕路徑、不進 buyer coordinator 委派清單」的
// buyer specialist key（MVP 決策：見 review finding I3）。
// seller_match 的按鈕 handler（internal/handler/buyer_assistant.go 的 SellerMatch）直接用
// Specialists["buyer:seller_match"]，不透過 coordinator；一般 /buyer/assistant/chat 尚未支援
// 渲染 type:"seller_candidates" 的 tool_result，若讓 coordinator 也能觸發它，買家可能在
// 一般對話中誤觸發、拿到前端無法渲染的回應。
//
// 這個 map 本身即測試點：agents_test.go 的 TestBuyerCoordinatorFallbackExcluded_OnlySellerMatch
// 不需要 GEMINI_API_KEY 就能驗證排除清單只含 seller_match，不含其他 key。
var buyerCoordinatorExcluded = map[string]bool{
	"seller_match": true,
}

// NewAgents 建立所有 agent。
//
// 🔴 **永遠不回因設定缺席而起的 error**：這支是 fx.Provide 的直接入口，而
// agent.Module 無條件掛在 app.go ⇒ 回 error 就是 fx abort ⇒ 58 支 RPC 全滅
// （現行五個已註冊 service 的總和：AIImageService 2＋AIAdminService 7＋
// RecommendationService 1＋AIAssistantService 15＋AIProcurementService 33。
// 之前是 59——#3519 刪掉 legacy AIService 的 10 支死 RPC，但 AIProcurementService
// 同期也持續在長，不是簡單的 59−10。查證：`grep -cE "^\s+(MethodName|StreamName):"
// pkg/proto/ai/ai_grpc.pb.go`，不要用減法推算）。
// backend#3329 盤點時漏掉這條路徑（它只列了 internal/client/gemini 與
// procurement 的三個建構子），所以在那之後「ai-service 不設 GEMINI_API_KEY
// 也能啟動」仍然是假的——2026-09-11 實測坐實。model 現在由 internal/llmprovider
// 提供，未開通時是 fail-closed 的停用實作，agent 照常建得起來、呼叫時回 9703。
func NewAgents(a llmprovider.Assistant, deps *ToolDeps) (*Agents, error) {
	model := a.LLM()

	specialists := make(map[string]agent.Agent)
	var sellerAgents, buyerAgents []agent.Agent

	for _, spec := range sellerSpecialistSpecs() {
		tools := spec.buildTools(deps)
		a, err := buildSpecialist(spec, tools, model)
		if err != nil {
			return nil, err
		}
		specialists["seller:"+spec.Key] = a
		sellerAgents = append(sellerAgents, a)
	}
	for _, spec := range buyerSpecialistSpecs() {
		tools := spec.buildTools(deps)
		a, err := buildSpecialist(spec, tools, model)
		if err != nil {
			return nil, err
		}
		specialists["buyer:"+spec.Key] = a
		if buyerCoordinatorExcluded[spec.Key] {
			// 僅供按鈕路徑（見 buyerCoordinatorExcluded 註解），不進 coordinator 委派清單，
			// 不進 fallback all-tools chat。
			continue
		}
		buyerAgents = append(buyerAgents, a)
	}

	sellerCoord, err := buildCoordinator("seller_coordinator", sellerCoordinatorInstruction, 0.3, sellerAgents, model)
	if err != nil {
		return nil, err
	}
	buyerCoord, err := buildCoordinator("buyer_coordinator", buyerCoordinatorInstruction, 0.3, buyerAgents, model)
	if err != nil {
		return nil, err
	}

	// --- 以下保留原樣（Phase 1 不動）：orderPresenter / productAutofill / productListing ---
	orderPresenter, err := llmagent.New(llmagent.Config{
		Name:                  "order_presenter",
		Model:                 model,
		Description:           "待出貨訂單呈現 agent，接收預查詢的訂單資料並生成自然語言回覆",
		Instruction:           orderPresenterInstruction,
		GenerateContentConfig: &genai.GenerateContentConfig{Temperature: genai.Ptr(float32(0.3))},
	})
	if err != nil {
		return nil, err
	}
	productAutofill, err := llmagent.New(llmagent.Config{
		Name:        "product_autofill_agent",
		Model:       model,
		Description: "從賣家自由文字（或圖片）抽取商品資訊，回傳 structured JSON ProductDraft",
		Instruction: productAutofillInstruction,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema:   productDraftSchema,
			Temperature:      genai.Ptr(float32(0.2)),
		},
	})
	if err != nil {
		return nil, err
	}
	productListing, err := llmagent.New(llmagent.Config{
		Name:                  "product_listing_agent",
		Model:                 model,
		Description:           "對話式上架助手，引導賣家逐步填寫商品資訊並透過 tool 寫入草稿",
		Instruction:           productListingInstruction,
		Tools:                 []adktool.Tool{NewSearchCategoryTool(deps), NewUpdateDraftTool(deps)},
		GenerateContentConfig: &genai.GenerateContentConfig{Temperature: genai.Ptr(float32(0.3))},
	})
	if err != nil {
		return nil, err
	}

	return &Agents{
		Specialists:     specialists,
		Coordinators:    map[string]agent.Agent{"seller": sellerCoord, "buyer": buyerCoord},
		OrderPresenter:  orderPresenter,
		ProductAutofill: productAutofill,
		ProductListing:  productListing,
	}, nil
}
