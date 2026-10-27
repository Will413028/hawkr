package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// orderStatusMap translates numeric status codes to human-readable labels.
// 對應 CodeMeta OrderStatus 定義。
var orderStatusMap = map[string]string{
	"0":  "待付款",
	"10": "已付款（待建立物流單）",
	"20": "待出貨（物流單已建立，待出貨）",
	"30": "配送中",
	"32": "待取貨",
	"50": "已完成",
	"80": "已取消",
}

// StatusLabel 將數字狀態碼轉換為中文標籤。
func StatusLabel(code string) string {
	if label, ok := orderStatusMap[code]; ok {
		return label
	}
	return code
}

// BuildItemsSummary 將訂單品項組成摘要字串，如 "A5 和牛 x1, 和牛組 x2"。
func BuildItemsSummary(items []usecase.SellerOrderItem) string {
	return buildItemsSummary(items)
}

// --- Tool Input/Output Structs ---

// OrderQueryInput 訂單查詢
type OrderQueryInput struct {
	DateRange string `json:"date_range" jsonschema:"日期範圍：today, last_7_days, all"`
	Status    string `json:"status,omitempty" jsonschema:"訂單狀態碼，空字串表示全部。dashboard待出貨對應20。可用值：0(待付款) 10(已付款/待建立物流單) 20(待出貨/物流單已建立) 30(配送中) 32(待取貨) 50(已完成) 80(已取消)"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"回傳筆數（預設10，最大20）"`
}

// OrderItem 訂單摘要
type OrderItem struct {
	OrderNo      string `json:"order_no"`
	Status       string `json:"status"`
	BuyerName    string `json:"buyer_name"`
	ItemsSummary string `json:"items_summary"`
	AmountTWD    int64  `json:"amount_twd"`
	CreateTimeMs int64  `json:"create_time_ms"`
}

// OrderQueryOutput 訂單查詢結果
type OrderQueryOutput struct {
	Orders []OrderItem `json:"orders"`
	Total  int         `json:"total"`
}

// OrderDetailInput 訂單詳情查詢
type OrderDetailInput struct {
	OrderNo string `json:"order_no" jsonschema:"訂單編號"`
}

// OrderDetailOutput 訂單詳情
type OrderDetailOutput struct {
	OrderNo        string `json:"order_no"`
	Status         string `json:"status"`
	BuyerID        string `json:"buyer_id"`
	ShipmentNo     string `json:"shipment_no"`
	ShippingStatus string `json:"shipping_status"`
	Amount         int64  `json:"amount_twd"`
}

// InventoryCheckInput 庫存檢查（無參數）
type InventoryCheckInput struct{}

// ShortageItem 庫存不足品項
type ShortageItem struct {
	ProductName  string `json:"product_name"`
	CurrentStock int64  `json:"current_stock"`
	Required     int    `json:"required"`
	Shortage     int64  `json:"shortage"`
}

// InventoryCheckOutput 庫存檢查結果
type InventoryCheckOutput struct {
	Message           string         `json:"message,omitempty"`
	ShortageItems     []ShortageItem `json:"shortage_items"`
	SufficientCount   int            `json:"sufficient_count"`
	TotalProductCount int            `json:"total_product_count"`
}

// PaymentSummaryInput 收款對帳
type PaymentSummaryInput struct {
	Period string `json:"period,omitempty" jsonschema:"查詢週期：this_week, last_week, this_month"`
}

// PendingDetail 待收款明細
type PendingDetail struct {
	OrderNo   string `json:"order_no"`
	BuyerName string `json:"buyer_name"`
	Amount    int64  `json:"amount"`
}

// PaymentSummaryOutput 收款對帳結果
type PaymentSummaryOutput struct {
	Period          string          `json:"period"`
	PeriodLabel     string          `json:"period_label"`
	PendingDetails  []PendingDetail `json:"pending_details"`
	CollectedAmount int64           `json:"collected_amount"`
	PendingAmount   int64           `json:"pending_amount"`
	TotalAmount     int64           `json:"total_amount"`
}

// SeasonalPriceInput 農產品行情
type SeasonalPriceInput struct {
	ProductName string `json:"product_name,omitempty" jsonschema:"指定查詢的農產品名稱，不填則自動比對賣家所有商品"`
}

// CropResult 單一品項行情
type CropResult struct {
	CropName string                  `json:"crop_name"`
	Error    string                  `json:"error,omitempty"`
	Prices   []usecase.ObservedPrice `json:"prices,omitempty"`
}

// SeasonalPriceOutput 農產品行情結果
type SeasonalPriceOutput struct {
	Crops []CropResult `json:"crops"`
}

// BuyerOrderQueryInput 買家訂單查詢
type BuyerOrderQueryInput struct {
	DateRange string `json:"date_range" jsonschema:"日期範圍：today, last_7_days, all"`
	Status    string `json:"status,omitempty" jsonschema:"訂單狀態碼（0/10/20/30/40），空字串表示全部"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"回傳筆數（預設10，最大20）"`
}

// BuyerOrderItem 買家訂單摘要
type BuyerOrderItem struct {
	OrderNo      string `json:"order_no"`
	Status       string `json:"status"`
	SellerName   string `json:"seller_name"`
	ItemCount    int    `json:"item_count"`
	AmountTWD    int64  `json:"amount_twd"`
	CreateTimeMs int64  `json:"create_time_ms"`
}

// BuyerOrderQueryOutput 買家訂單查詢結果
type BuyerOrderQueryOutput struct {
	Orders []BuyerOrderItem `json:"orders"`
	Total  int              `json:"total"`
}

// --- Tool Deps (injected via fx) ---

// ToolDeps 所有 tool 共用的依賴
type ToolDeps struct {
	OrderClient           usecase.OrderClient
	ProductClient         usecase.ProductClient
	PricebookClient       usecase.PricebookClient
	CategoryIndex         *cache.CategoryIndex
	CandidateLeadSearcher usecase.CandidateLeadSearcher
}

// --- Tool Builders ---

// NewGetSellerOrdersTool 建立訂單查詢 tool
func NewGetSellerOrdersTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "get_seller_orders",
		Description: "查詢賣家的訂單清單，可依日期範圍和狀態篩選",
	}, func(ctx agent.Context, input OrderQueryInput) (OrderQueryOutput, error) {
		storeID := getStoreID(ctx)
		pageSize := input.PageSize
		if pageSize <= 0 || pageSize > 20 {
			pageSize = 10
		}

		orders, err := deps.OrderClient.GetSellerOrders(ctx, storeID, input.Status, 1, pageSize)
		if err != nil {
			return OrderQueryOutput{}, fmt.Errorf("查詢訂單失敗: %w", err)
		}
		orders = filterByDateRange(orders, input.DateRange)

		items := make([]OrderItem, 0, len(orders))
		for _, o := range orders {
			items = append(items, OrderItem{
				OrderNo:      o.OrderNo,
				Status:       StatusLabel(o.Status),
				BuyerName:    o.BuyerName,
				ItemsSummary: buildItemsSummary(o.Items),
				AmountTWD:    o.Amount,
				CreateTimeMs: o.CreateTimeMs,
			})
		}
		return OrderQueryOutput{Orders: items, Total: len(items)}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 get_seller_orders tool 失敗: %v", err)
	}
	return t
}

// NewGetOrderDetailTool 建立訂單詳情 tool
func NewGetOrderDetailTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "get_order_detail",
		Description: "查詢單筆訂單的詳細資訊，包括金額、出貨編號、配送狀態",
	}, func(ctx agent.Context, input OrderDetailInput) (OrderDetailOutput, error) {
		storeID := getStoreID(ctx)
		detail, err := deps.OrderClient.GetSellerOrderDetail(ctx, storeID, input.OrderNo)
		if err != nil {
			return OrderDetailOutput{}, fmt.Errorf("查詢訂單 %s 失敗: %w", input.OrderNo, err)
		}
		return OrderDetailOutput{
			OrderNo:        detail.OrderNo,
			Status:         StatusLabel(detail.Status),
			Amount:         detail.Amount,
			BuyerID:        detail.BuyerID,
			ShipmentNo:     detail.ShipmentNo,
			ShippingStatus: detail.ShippingStatus,
		}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 get_order_detail tool 失敗: %v", err)
	}
	return t
}

// NewCheckInventoryShortageTool 建立庫存檢查 tool
func NewCheckInventoryShortageTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "check_inventory_shortage",
		Description: "檢查賣家所有商品的庫存是否足以滿足未出貨訂單需求，列出庫存不足的品項",
	}, func(ctx agent.Context, _ InventoryCheckInput) (InventoryCheckOutput, error) {
		storeID := getStoreID(ctx)

		products, err := deps.ProductClient.ListSellerProducts(ctx, storeID, 1, 100)
		if err != nil {
			return InventoryCheckOutput{}, fmt.Errorf("查詢商品失敗: %w", err)
		}
		if len(products) == 0 {
			return InventoryCheckOutput{Message: "目前沒有上架商品"}, nil
		}

		orders, err := deps.OrderClient.GetSellerOrders(ctx, storeID, "10", 1, 100)
		if err != nil {
			return InventoryCheckOutput{}, fmt.Errorf("查詢待出貨訂單失敗: %w", err)
		}

		demandBySku := make(map[string]int)
		skuToProduct := make(map[string]string)
		for _, o := range orders {
			for _, item := range o.Items {
				demandBySku[item.SkuID] += item.Quantity
				if _, ok := skuToProduct[item.SkuID]; !ok {
					skuToProduct[item.SkuID] = item.ProductName
				}
			}
		}

		if len(demandBySku) == 0 {
			return InventoryCheckOutput{Message: "目前沒有待出貨訂單，無庫存需求"}, nil
		}

		skuIDs := make([]string, 0, len(demandBySku))
		for skuID := range demandBySku {
			skuIDs = append(skuIDs, skuID)
		}
		stockMap, err := deps.ProductClient.GetStockBatch(ctx, skuIDs)
		if err != nil {
			return InventoryCheckOutput{}, fmt.Errorf("查詢庫存失敗: %w", err)
		}

		var shortages []ShortageItem
		for skuID, demand := range demandBySku {
			stock := stockMap[skuID]
			if int64(demand) > stock {
				shortages = append(shortages, ShortageItem{
					ProductName:  skuToProduct[skuID],
					CurrentStock: stock,
					Required:     demand,
					Shortage:     int64(demand) - stock,
				})
			}
		}

		return InventoryCheckOutput{
			ShortageItems:     shortages,
			SufficientCount:   len(demandBySku) - len(shortages),
			TotalProductCount: len(demandBySku),
		}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 check_inventory_shortage tool 失敗: %v", err)
	}
	return t
}

// NewQueryPaymentSummaryTool 建立收款對帳 tool
func NewQueryPaymentSummaryTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "query_payment_summary",
		Description: "查詢賣家的收款統計（已收款/待收款金額）與待收款訂單明細",
	}, func(ctx agent.Context, input PaymentSummaryInput) (PaymentSummaryOutput, error) {
		storeID := getStoreID(ctx)
		period := input.Period
		if period == "" {
			period = "this_week"
		}
		startMs, endMs, periodLabel := periodToRange(period)

		completedOrders, err := deps.OrderClient.GetSellerOrders(ctx, storeID, "30", 1, 100)
		if err != nil {
			return PaymentSummaryOutput{}, fmt.Errorf("查詢已完成訂單失敗: %w", err)
		}
		var collectedAmount int64
		for _, o := range completedOrders {
			if o.CreateTimeMs >= startMs && o.CreateTimeMs <= endMs {
				collectedAmount += o.Amount
			}
		}

		pendingOrders, err := deps.OrderClient.GetSellerOrders(ctx, storeID, "10,20", 1, 100)
		if err != nil {
			return PaymentSummaryOutput{}, fmt.Errorf("查詢待收款訂單失敗: %w", err)
		}
		var pendingAmount int64
		var details []PendingDetail
		for _, o := range pendingOrders {
			if o.CreateTimeMs >= startMs && o.CreateTimeMs <= endMs {
				pendingAmount += o.Amount
				if len(details) < 5 {
					details = append(details, PendingDetail{
						OrderNo:   o.OrderNo,
						BuyerName: o.BuyerName,
						Amount:    o.Amount,
					})
				}
			}
		}

		return PaymentSummaryOutput{
			Period:          period,
			PeriodLabel:     periodLabel,
			CollectedAmount: collectedAmount,
			PendingAmount:   pendingAmount,
			TotalAmount:     collectedAmount + pendingAmount,
			PendingDetails:  details,
		}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 query_payment_summary tool 失敗: %v", err)
	}
	return t
}

// NewGetSeasonalPriceSuggestionTool 建立農產品行情 tool
func NewGetSeasonalPriceSuggestionTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "get_seasonal_price_suggestion",
		Description: "查詢農委會公開市場的當季農產品批發行情，結合賣家商品給出定價建議",
	}, func(ctx agent.Context, input SeasonalPriceInput) (SeasonalPriceOutput, error) {
		storeID := getStoreID(ctx)
		var cropNames []string
		if input.ProductName != "" {
			cropNames = []string{input.ProductName}
		} else {
			products, err := deps.ProductClient.ListSellerProducts(ctx, storeID, 1, 50)
			if err != nil {
				return SeasonalPriceOutput{}, fmt.Errorf("查詢賣家商品失敗: %w", err)
			}
			for _, p := range products {
				cropNames = append(cropNames, p.Name)
			}
		}
		if len(cropNames) == 0 {
			return SeasonalPriceOutput{Crops: []CropResult{{CropName: "", Error: "目前沒有上架商品"}}}, nil
		}

		results := make([]CropResult, 0, len(cropNames))
		for _, name := range cropNames {
			observations, err := deps.PricebookClient.GetObservedPrice(ctx, name)
			if err != nil {
				pkglogger.Warn(ctx, "[Agent] pricebook-service 查詢失敗", zap.String("crop", name), zap.Error(err))
				results = append(results, CropResult{CropName: name, Error: "行情資料查詢失敗，請稍後再試"})
				continue
			}
			if len(observations) == 0 {
				// pricebook-service 的 GetObservedPrice 不區分「作物名無法解析」與
				// 「已解析但近期無成交行情」，兩者皆回空 observations、無 error。
				results = append(results, CropResult{CropName: name, Error: "查無行情資料，可能是商品名稱無法對應到官方作物名，或近期無成交紀錄"})
				continue
			}
			results = append(results, CropResult{CropName: name, Prices: observations})
		}
		return SeasonalPriceOutput{Crops: results}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 get_seasonal_price_suggestion tool 失敗: %v", err)
	}
	return t
}

// NewGetMyOrdersTool 建立買家訂單查詢 tool
func NewGetMyOrdersTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "get_my_orders",
		Description: "查詢買家的訂單清單，可依日期範圍和狀態篩選",
	}, func(ctx agent.Context, input BuyerOrderQueryInput) (BuyerOrderQueryOutput, error) {
		buyerID := getBuyerID(ctx)
		pageSize := input.PageSize
		if pageSize <= 0 || pageSize > 20 {
			pageSize = 10
		}

		orders, err := deps.OrderClient.GetMyOrders(ctx, buyerID, input.Status, 1, pageSize)
		if err != nil {
			return BuyerOrderQueryOutput{}, fmt.Errorf("查詢訂單失敗: %w", err)
		}
		orders = filterBuyerByDateRange(orders, input.DateRange)

		items := make([]BuyerOrderItem, 0, len(orders))
		for _, o := range orders {
			items = append(items, BuyerOrderItem{
				OrderNo:      o.OrderNo,
				Status:       StatusLabel(o.Status),
				SellerName:   o.SellerName,
				ItemCount:    o.ItemCount,
				AmountTWD:    o.Amount,
				CreateTimeMs: o.CreateTimeMs,
			})
		}
		return BuyerOrderQueryOutput{Orders: items, Total: len(items)}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 get_my_orders tool 失敗: %v", err)
	}
	return t
}

// NewGetBuyerOrderDetailTool 建立買家訂單詳情 tool
func NewGetBuyerOrderDetailTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "get_buyer_order_detail",
		Description: "查詢買家單筆訂單的詳細資訊",
	}, func(ctx agent.Context, input OrderDetailInput) (json.RawMessage, error) {
		buyerID := getBuyerID(ctx)
		detail, err := deps.OrderClient.GetBuyerOrderDetail(ctx, buyerID, input.OrderNo)
		if err != nil {
			return nil, fmt.Errorf("查詢訂單 %s 失敗: %w", input.OrderNo, err)
		}
		result := map[string]interface{}{
			"order_no":        detail.OrderNo,
			"status":          StatusLabel(detail.Status),
			"amount_twd":      detail.Amount,
			"seller_name":     detail.SellerName,
			"shipping_method": detail.ShippingMethod,
			"shipping_status": detail.ShippingStatus,
			"shipping_fee":    detail.ShippingFee,
			"recipient_name":  detail.RecipientName,
			"payment_method":  detail.PaymentMethod,
		}
		b, _ := json.Marshal(result)
		return b, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 get_buyer_order_detail tool 失敗: %v", err)
	}
	return t
}

// --- Buyer: 包裹追蹤 ---

// TrackingInput 包裹追蹤（無參數，自動查配送中訂單）
type TrackingInput struct{}

// TrackingItem 單筆物流資訊
type TrackingItem struct {
	OrderNo            string `json:"order_no"`
	Status             string `json:"status"`
	ShippingMethod     string `json:"shipping_method"`
	ShippingStatus     string `json:"shipping_status"`
	ShippingStatusDesc string `json:"shipping_status_desc"`
	SellerName         string `json:"seller_name"`
	ShipTime           string `json:"ship_time"`
	Amount             int64  `json:"amount_twd"`
}

// TrackingOutput 包裹追蹤結果
type TrackingOutput struct {
	Message  string         `json:"message,omitempty"`
	Packages []TrackingItem `json:"packages"`
}

// NewTrackMyPackageTool 建立包裹追蹤 tool
func NewTrackMyPackageTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "track_my_package",
		Description: "查詢買家配送中的包裹物流狀態",
	}, func(ctx agent.Context, _ TrackingInput) (TrackingOutput, error) {
		buyerID := getBuyerID(ctx)

		// 查配送中訂單 (status=20)
		orders, err := deps.OrderClient.GetMyOrders(ctx, buyerID, "20", 1, 10)
		if err != nil {
			return TrackingOutput{}, fmt.Errorf("查詢配送中訂單失敗: %w", err)
		}
		if len(orders) == 0 {
			return TrackingOutput{Message: "目前沒有配送中的包裹"}, nil
		}

		// 取得每筆訂單的物流詳情
		packages := make([]TrackingItem, 0, len(orders))
		for _, o := range orders {
			detail, err := deps.OrderClient.GetBuyerOrderDetail(ctx, buyerID, o.OrderNo)
			if err != nil {
				continue
			}
			packages = append(packages, TrackingItem{
				OrderNo:            detail.OrderNo,
				Status:             StatusLabel(detail.Status),
				ShippingMethod:     detail.ShippingMethod,
				ShippingStatus:     detail.ShippingStatus,
				ShippingStatusDesc: "", // proto 未直接暴露，由 LLM 從 ShippingStatus 描述
				SellerName:         detail.SellerName,
				Amount:             detail.Amount,
			})
		}

		return TrackingOutput{Packages: packages}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 track_my_package tool 失敗: %v", err)
	}
	return t
}

// --- Buyer: 當季水果推薦 ---

// SeasonalRecommendInput 當季水果推薦（無參數）
type SeasonalRecommendInput struct{}

// SeasonalProduct 推薦商品
type SeasonalProduct struct {
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
	PriceMin int64  `json:"price_min"`
	PriceMax int64  `json:"price_max"`
}

// SeasonalRecommendOutput 推薦結果
type SeasonalRecommendOutput struct {
	Message  string            `json:"message,omitempty"`
	Products []SeasonalProduct `json:"products"`
}

// NewRecommendSeasonalFruitsTool 建立當季水果推薦 tool
func NewRecommendSeasonalFruitsTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "recommend_seasonal_fruits",
		Description: "從平台上架的水果類商品中推薦當季品項，回傳商品名稱與價格",
	}, func(ctx agent.Context, _ SeasonalRecommendInput) (SeasonalRecommendOutput, error) {
		// 查詢水果類商品（status=4004 上架中）
		products, _, err := deps.ProductClient.GetProductList(ctx, 1, 20)
		if err != nil {
			return SeasonalRecommendOutput{}, fmt.Errorf("查詢商品失敗: %w", err)
		}
		if len(products) == 0 {
			return SeasonalRecommendOutput{Message: "目前沒有上架的水果商品"}, nil
		}

		items := make([]SeasonalProduct, 0, len(products))
		for _, p := range products {
			items = append(items, SeasonalProduct{
				Name:     p.Name,
				ImageURL: p.ImageURL,
				PriceMin: p.PriceMin,
				PriceMax: p.PriceMax,
			})
		}

		return SeasonalRecommendOutput{Products: items}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 recommend_seasonal_fruits tool 失敗: %v", err)
	}
	return t
}

// --- Buyer: AI 找貨（開發名單候選廠商配對） ---

// SearchCandidateSellersInput 候選廠商查詢條件，皆可選，由 LLM 讀買家描述後判斷。
type SearchCandidateSellersInput struct {
	Category string `json:"category,omitempty" jsonschema:"商品分類，例如：水果、蔬菜米穀、海鮮、加工"`
	Keywords string `json:"keywords,omitempty" jsonschema:"品項或特色關鍵字，用於比對商品描述與品牌定位文字"`
	Region   string `json:"region,omitempty" jsonschema:"產地或地區，例如：台南、南投"`
}

// CandidateSellerResult 候選廠商（僅含公開資訊，不含 BD 私有聯絡窗口）。
type CandidateSellerResult struct {
	// ID 刻意用 string（不是 int64 + json:",string"）：seller_prospects.id 是 64-bit TSID
	// （~18-19 位數，bigint PK，同 candidate_leads.id 舊制），超過 float64 53-bit
	// mantissa 可精確表示的範圍。google-adk 的
	// functiontool.Run 內部會把 tool 輸出經 typeutil.ConvertToWithJSONSchema 做
	// json.Unmarshal 轉成 map[string]any，JSON 裸數字在此一律變成 float64，精度會在這一步就被
	// 無聲捨去。
	//
	// 這個 struct 沒有設定 functiontool.Config.OutputSchema，ADK 會用
	// jsonschema-go（jsonschema.For[T]）以 reflect.Kind 自動推導 output schema——而它只看 Go
	// 型別本身，完全不理會 json tag。若型別是 int64 只加 json:"id,string" 這種 tag 技巧，
	// encoding/json 序列化出來的值會是加引號的字串，但 ADK 推導出的 schema 仍宣告 "integer"；
	// functiontool.Run 內部接著會用該 schema 對序列化結果做 resolvedSchema.Validate，schema
	// 與實際值型別對不上，導致每一次呼叫都直接失敗（不是只有大 TSID 才炸，是結構性、每次都炸）。
	// 因此必須讓 Go 型別本身就是 string，schema 推導與序列化值才會一致。
	ID              string `json:"id"`
	BrandName       string `json:"brand_name"`
	Category        string `json:"category"`
	ProductSummary  string `json:"product_summary"`
	Region          string `json:"region"`
	Positioning     string `json:"positioning"`
	ResearchSummary string `json:"research_summary"`
	// 只回公開官網；社群帳號與 Email／電話刻意不出現在 tool output——這層會進
	// LLM context、ADK events 表與 SSE payload（設計書 55 §8.1）。之後外部邀價
	// （Phase ⑤）要寄信時由 server 端直接讀 seller-service 的 seller_prospects
	// 主檔（仍保有 public_email／public_phone／public_social，見 api/proto/seller/
	// seller.proto 的 SellerProspectItem），不經此路徑、也不經 usecase.ProspectHit
	// （這個 RPC 契約本身就沒有攜帶那三個欄位）。
	//
	// ⚠️ 欄位層下架不等於這一層沒有聯絡管道，兩個已知缺口：
	//   1. public_website 不保證是品牌官網——CRM1 名單的「連結」欄就是映射到這個
	//      欄位，而該分頁的候選多是個人社群經營者，值可能是社群網址。（原本指向
	//      scripts/import_candidate_leads.go 的 crm1ColumnMap，該腳本已隨資料源
	//      遷移刪除；seller-service 的 domain.BuildProspectMatchKeys 也因此刻意
	//      把社群網域排除在 match_domain 之外，見該檔 prospectSocialHosts。）
	//   2. research_summary／positioning／product_summary 是原樣透傳的自由文字，
	//      內容取決於開發名單怎麼填。
	// 真正要關掉這兩條需要值層過濾或改欄位語意，屬另一顆票（設計書 55 §8.1）。
	PublicWebsite string `json:"public_website"`
	// MatchedFacets／MissedFacets 是可驗證的配對理由（category／region／keywords）。
	// 型別必須是 []string 而非自訂型別：ADK 依 reflect.Kind 推導 output schema。
	MatchedFacets []string `json:"matched_facets"`
	MissedFacets  []string `json:"missed_facets"`
}

// SearchCandidateSellersOutput 候選廠商查詢結果。
type SearchCandidateSellersOutput struct {
	Message    string                  `json:"message,omitempty"`
	Candidates []CandidateSellerResult `json:"candidates"`
}

// NewSearchCandidateSellersTool 建立候選廠商查詢 tool
func NewSearchCandidateSellersTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "search_candidate_sellers",
		Description: "從開發名單候選廠商中查詢符合分類/關鍵字/地區的候選，回傳可能符合買家需求的廠商清單（僅公開資訊）",
	}, func(ctx agent.Context, input SearchCandidateSellersInput) (SearchCandidateSellersOutput, error) {
		return searchCandidateSellers(ctx, deps, input)
	})
	if err != nil {
		zap.S().Fatalf("建立 search_candidate_sellers tool 失敗: %v", err)
	}
	return t
}

// nilToEmpty 讓 nil slice 序列化成 []，而非 null——ADK FunctionResponse 轉 JSON 後
// null 會讓消費端多一層判斷（同 SearchCandidateSellersOutput.Candidates 的既有理由）。
func nilToEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// searchCandidateSellers 是 search_candidate_sellers tool 的核心邏輯，抽成獨立函式方便單元測試——
// agent.Context 內嵌 context.Context，呼叫端可以直接傳 context.Background()，不需要建構 ADK 內部型別。
func searchCandidateSellers(ctx context.Context, deps *ToolDeps, input SearchCandidateSellersInput) (SearchCandidateSellersOutput, error) {
	matches, err := deps.CandidateLeadSearcher.Search(ctx, usecase.CandidateSearchParams{
		Category: input.Category,
		Keywords: input.Keywords,
		Region:   input.Region,
	})
	if err != nil {
		return SearchCandidateSellersOutput{}, fmt.Errorf("查詢候選廠商失敗: %w", err)
	}
	if len(matches) == 0 {
		// Candidates 明確給非 nil 空 slice：nil slice 經 ADK FunctionResponse 轉 JSON 會變成
		// "candidates":null，前端多一層 null 判斷；空陣列則直接可迭代。
		return SearchCandidateSellersOutput{
			Message:    "目前開發名單中沒有符合條件的候選廠商",
			Candidates: []CandidateSellerResult{},
		}, nil
	}
	items := make([]CandidateSellerResult, 0, len(matches))
	for _, m := range matches {
		l := m.Vendor
		items = append(items, CandidateSellerResult{
			// l.ID 是 usecase.ProspectVendor 的 int64 id；CandidateSellerResult.ID 是 string，
			// 在這裡用 strconv.FormatInt 顯式轉換（見上方欄位註解：ADK 依 Go 型別推導 output
			// schema，不能靠 json tag 取巧）。
			ID:              strconv.FormatInt(l.ID, 10),
			BrandName:       l.DisplayName,
			Category:        l.Category,
			ProductSummary:  l.ProductSummary,
			Region:          l.Region,
			Positioning:     l.Positioning,
			ResearchSummary: l.ResearchSummary,
			PublicWebsite:   l.PublicWebsite,
			MatchedFacets:   nilToEmpty(m.MatchedFacets),
			MissedFacets:    nilToEmpty(m.MissedFacets),
		})
	}
	return SearchCandidateSellersOutput{Candidates: items}, nil
}

// --- Product Listing Assistant Tools ---

// SearchCategoryInput 分類搜尋輸入
type SearchCategoryInput struct {
	Keyword string `json:"keyword" jsonschema:"商品核心品項關鍵字（例如「芭樂」「奇異果」「高麗菜」），不要填分類路徑"`
}

// CategoryHit 分類候選
type CategoryHit struct {
	CatalogID  string `json:"catalogId"`
	Path       string `json:"path"`
	Confidence string `json:"confidence"`
}

// SearchCategoryOutput 分類搜尋結果
type SearchCategoryOutput struct {
	Candidates []CategoryHit `json:"candidates"`
}

// SkuPatch 單一 SKU 草稿
type SkuPatch struct {
	SkuName  string `json:"skuName" jsonschema:"規格名稱，例如「1 公斤裝」「禮盒 12 顆」"`
	Unit     string `json:"unit,omitempty" jsonschema:"重量單位：KG / CATTY / GRAM / NONE（非重量類規格）"`
	Price    int64  `json:"price" jsonschema:"價格，新台幣整數元"`
	Weight   int64  `json:"weight,omitempty" jsonschema:"重量數值，搭配 unit 使用"`
	StockQty int64  `json:"stockQty,omitempty" jsonschema:"庫存數量"`
}

// CertificationPatch 認證編號
type CertificationPatch struct {
	Name   string `json:"name" jsonschema:"認證名稱，例如「有機農產品標章」"`
	CertNo string `json:"certNo" jsonschema:"認證編號"`
}

// ProductDraftPatch 商品草稿的部分更新（本輪新得到的欄位）
type ProductDraftPatch struct {
	Name           string               `json:"name,omitempty" jsonschema:"商品名稱，最多 60 字元"`
	Origin         string               `json:"origin,omitempty" jsonschema:"產地，例如 台灣 / 紐西蘭"`
	CategoryID     string               `json:"categoryId,omitempty" jsonschema:"L3 分類 ID，必須從 search_category 工具回傳的 catalogId 取得，禁止自行猜測"`
	Description    string               `json:"description,omitempty" jsonschema:"商品描述，最多 2000 字元"`
	Tags           []string             `json:"tags,omitempty" jsonschema:"商品標籤，最多 8 個"`
	Skus           []SkuPatch           `json:"skus,omitempty" jsonschema:"商品規格清單（覆蓋式更新，有變更時請送出完整清單）"`
	Certifications []CertificationPatch `json:"certifications,omitempty" jsonschema:"農業認證編號清單"`
}

// UpdateDraftInput 更新草稿輸入
type UpdateDraftInput struct {
	Patch ProductDraftPatch `json:"patch" jsonschema:"要更新的商品欄位，只塞本輪確定的新欄位，未變動者請省略"`
}

// UpdateDraftOutput 更新草稿結果
type UpdateDraftOutput struct {
	Patch    ProductDraftPatch `json:"patch"`
	Accepted bool              `json:"accepted"`
}

// NewSearchCategoryTool 建立分類搜尋 tool（對話式上架助手用）
func NewSearchCategoryTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "search_category",
		Description: "依品項關鍵字查詢 top 3 分類候選。提到新商品類別時必須先呼叫此工具確認分類，禁止自行猜測 catalogId。",
	}, func(ctx agent.Context, input SearchCategoryInput) (SearchCategoryOutput, error) {
		if deps.CategoryIndex == nil || !deps.CategoryIndex.Ready() {
			return SearchCategoryOutput{Candidates: []CategoryHit{}}, nil
		}
		matches := deps.CategoryIndex.Match(input.Keyword, 3)
		hits := make([]CategoryHit, 0, len(matches))
		for _, m := range matches {
			hits = append(hits, CategoryHit{
				CatalogID:  m.CatalogID,
				Path:       m.Path,
				Confidence: m.Confidence,
			})
		}
		return SearchCategoryOutput{Candidates: hits}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 search_category tool 失敗: %v", err)
	}
	return t
}

// NewUpdateDraftTool 建立草稿更新 tool（對話式上架助手用）
// Tool 本身僅回傳 patch；handler 層攔截此 tool 的 FunctionResponse，額外推 draft_patch SSE event 給前端。
// 草稿實際狀態由前端累積；LLM 透過對話 history（ADK session 自動儲存）知道已填過哪些欄位。
func NewUpdateDraftTool(deps *ToolDeps) tool.Tool {
	t, err := functiontool.New(functiontool.Config{
		Name:        "update_draft",
		Description: "將本輪對話取得的商品欄位套到草稿。前端會累積所有 patch，你只需送出本輪新得到的欄位，已確認過的欄位請省略。",
	}, func(ctx agent.Context, input UpdateDraftInput) (UpdateDraftOutput, error) {
		return UpdateDraftOutput{Accepted: true, Patch: input.Patch}, nil
	})
	if err != nil {
		zap.S().Fatalf("建立 update_draft tool 失敗: %v", err)
	}
	return t
}

// --- Helpers ---

// StateKeyStore / StateKeyBuyer 是 ADK session state（map[string]any）的分區
// key。這是型別化這條鏈（HTTP header → handler → ADK session state →
// agent/tools.go → usecase → client → proto）上唯一無法靠型別守住的一跳：
// ADK session state 本身就是 map[string]any，key 只能是字串。
//
// 寫端在 internal/actor/actor.go 的 Actor.StateKey（寫入值見該檔 StateVal
// 註解），讀端在這裡的 getStoreID/getBuyerID，兩邊**必須**是同一組常數——
// handler 已經 import 本 package（agentpkg），方向是 handler → agent，因此
// 常數定義在這裡、handler 端引用，不會造成反向 import。
const (
	StateKeyStore = "store_id"
	StateKeyBuyer = "buyer_id"
)

// getStoreID 從 adk-go session state 取得現任 store_id（賣家工具專用；由
// SellerAssistantHandler 建立 session 時以 X-Store-Id 已解析值寫入，非原始
// 登入 account_id）。
func getStoreID(ctx agent.Context) identity.StoreID {
	return identity.StoreID(getInt64State(ctx, StateKeyStore))
}

// getBuyerID 從 adk-go session state 取得現任 buyer_id（買家工具專用；由
// BuyerAssistantHandler 以 X-Buyer-Id 已解析值寫入，非原始登入 account_id）。
func getBuyerID(ctx agent.Context) identity.BuyerID {
	return identity.BuyerID(getInt64State(ctx, StateKeyBuyer))
}

// getInt64State 從 session state 取得 int64 值。寫入端
// （SellerAssistantHandler.streamRunner）一律用 strconv.FormatInt 寫入字串，
// 避免值經過 adk-go database session service 的 JSONB 往返後失真——19 位 TSID
// 遠超 float64 的 53-bit 精確整數上限（2^53），裸 int64 寫入會在 Postgres
// 讀回時被 encoding/json.Unmarshal 靜默且不可逆地改寫成鄰近值（int64/float64
// 兩個 case 只用來相容 fix 部署前、還在 24 小時保留窗內建立的舊 session，新
// session 一律走 string case）。
func getInt64State(ctx agent.Context, key string) int64 {
	v, err := ctx.State().Get(key)
	if err != nil {
		return 0
	}
	switch id := v.(type) {
	case string:
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	case int64:
		return id
	case float64:
		return int64(id)
	}
	return 0
}

func buildItemsSummary(items []usecase.SellerOrderItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		name := item.ProductName
		if item.SkuName != "" && item.SkuName != item.ProductName {
			name = item.SkuName
		}
		parts = append(parts, fmt.Sprintf("%s x%d", name, item.Quantity))
	}
	return strings.Join(parts, ", ")
}

func filterByDateRange(orders []*usecase.SellerOrder, dateRange string) []*usecase.SellerOrder {
	if dateRange == "all" || dateRange == "" {
		return orders
	}
	now := time.Now()
	var cutoffMs int64
	switch dateRange {
	case "today":
		y, m, d := now.Date()
		cutoffMs = time.Date(y, m, d, 0, 0, 0, 0, now.Location()).UnixMilli()
	case "last_7_days":
		cutoffMs = now.AddDate(0, 0, -7).UnixMilli()
	default:
		return orders
	}
	filtered := make([]*usecase.SellerOrder, 0, len(orders))
	for _, o := range orders {
		if o.CreateTimeMs >= cutoffMs {
			filtered = append(filtered, o)
		}
	}
	return filtered
}

func filterBuyerByDateRange(orders []*usecase.BuyerOrder, dateRange string) []*usecase.BuyerOrder {
	if dateRange == "all" || dateRange == "" {
		return orders
	}
	now := time.Now()
	var cutoffMs int64
	switch dateRange {
	case "today":
		y, m, d := now.Date()
		cutoffMs = time.Date(y, m, d, 0, 0, 0, 0, now.Location()).UnixMilli()
	case "last_7_days":
		cutoffMs = now.AddDate(0, 0, -7).UnixMilli()
	default:
		return orders
	}
	filtered := make([]*usecase.BuyerOrder, 0, len(orders))
	for _, o := range orders {
		if o.CreateTimeMs >= cutoffMs {
			filtered = append(filtered, o)
		}
	}
	return filtered
}

func periodToRange(period string) (startMs, endMs int64, label string) {
	now := time.Now()
	var start, end time.Time
	switch period {
	case "last_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		thisMonday := now.AddDate(0, 0, -(weekday - 1))
		lastMonday := thisMonday.AddDate(0, 0, -7)
		start = time.Date(lastMonday.Year(), lastMonday.Month(), lastMonday.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 7).Add(-time.Millisecond)
	case "this_month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0).Add(-time.Millisecond)
	default:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		start = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 7).Add(-time.Millisecond)
	}
	label = fmt.Sprintf("%d/%d-%d/%d", start.Month(), start.Day(), end.Month(), end.Day())
	return start.UnixMilli(), end.UnixMilli(), label
}
