package buyeragent

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// StateKeyBuyer 是 session state 裡買家 ID 的 key；值以十進位字串存，數字型的 ID 經 JSONB 往返會被 float64 改寫。
const StateKeyBuyer = "buyer_id"

// ErrBuyerScope 是 tool 發現 session 的買家與這一輪的買家不一致（或缺值）時的錯誤。
var ErrBuyerScope = errors.New("buyeragent：session 的買家與這一輪不一致")

// ToolCall 是一次 tool 執行的結果，經 Deps.OnTool 回報。
type ToolCall struct {
	Name string
	// Args 是模型呼叫 tool 時實際送出的參數（沒有參數時是 nil）。
	Args map[string]any
	// Failed 是 tool 回了錯誤；錯誤文字會回給 LLM，助理仍會接著回覆。
	Failed bool
}

type orderQueryInput struct {
	DateRange string `json:"date_range" jsonschema:"日期範圍：today, last_7_days, all"`
	Status    string `json:"status,omitempty" jsonschema:"訂單狀態碼（0/10/20/30/40），空字串表示全部"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"回傳筆數（預設10，最大20）"`
}

type orderItem struct {
	OrderNo      string `json:"order_no"`
	Status       string `json:"status"`
	SellerName   string `json:"seller_name"`
	ItemCount    int64  `json:"item_count"`
	AmountTWD    int64  `json:"amount_twd"`
	CreateTimeMs int64  `json:"create_time_ms"`
}

type orderQueryOutput struct {
	Orders []orderItem `json:"orders"`
	Total  int         `json:"total"`
}

type orderDetailInput struct {
	OrderNo string `json:"order_no" jsonschema:"訂單編號"`
}

// orderDetailOutput 的 Result 是訂單詳情；明寫 result 外層，LLM 看到的形狀與舊服務一致。
type orderDetailOutput struct {
	Result map[string]any `json:"result"`
}

type trackingInput struct{}

type trackingItem struct {
	OrderNo            string `json:"order_no"`
	Status             string `json:"status"`
	ShippingMethod     string `json:"shipping_method"`
	ShippingStatus     string `json:"shipping_status"`
	ShippingStatusDesc string `json:"shipping_status_desc"`
	SellerName         string `json:"seller_name"`
	ShipTime           string `json:"ship_time"`
	Amount             int64  `json:"amount_twd"`
}

type trackingOutput struct {
	Message  string         `json:"message,omitempty"`
	Packages []trackingItem `json:"packages"`
}

// orderStatusLabels 把訂單服務的狀態碼轉成給 LLM 看的中文。
// 疑似缺陷（照搬，不修）：這張表的 20＝「待出貨」、30＝「配送中」，與 coordinator prompt 與 get_my_orders 說明寫的
// 「20＝已出貨/配送中」不一致，track_my_package 又以狀態 20 當「配送中」查詢。
var orderStatusLabels = map[string]string{
	"0":  "待付款",
	"10": "已付款（待建立物流單）",
	"20": "待出貨（物流單已建立，待出貨）",
	"30": "配送中",
	"32": "待取貨",
	"50": "已完成",
	"80": "已取消",
}

func statusLabel(code string) string {
	if label, ok := orderStatusLabels[code]; ok {
		return label
	}
	return code
}

// scopedBuyer 回這一輪的買家 ID，前提是 session state 記的買家與它一致。ADK v2 的 callback 與 tool 一律從 ctx.State() 讀。
func (d Deps) scopedBuyer(ctx agent.Context) (int64, error) {
	raw, err := ctx.State().Get(StateKeyBuyer)
	if err != nil {
		return 0, ErrBuyerScope
	}
	s, ok := raw.(string)
	if !ok {
		return 0, ErrBuyerScope
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 || id != d.BuyerID {
		return 0, ErrBuyerScope
	}
	return id, nil
}

// afterTool 是掛在 specialist 上的 AfterToolCallback：以模型實際送出的原始參數（args）與執行結果回報給 OnTool。
// ADK 會把 tool 內的 panic 轉成 err，所以 panic 也會回報為失敗。
func (d Deps) afterTool(_ agent.Context, t tool.Tool, args, _ map[string]any, err error) (map[string]any, error) {
	if d.OnTool == nil {
		return nil, nil
	}
	if len(args) == 0 {
		args = nil
	}
	d.OnTool(ToolCall{Name: t.Name(), Args: args, Failed: err != nil})
	return nil, nil
}

// newTool 建一支 tool：先驗身分範圍，再執行；結果由 afterTool 回報。
func newTool[In, Out any](d Deps, name, description string, run func(ctx context.Context, buyerID int64, in In) (Out, error)) (tool.Tool, error) {
	t, err := functiontool.New(functiontool.Config{Name: name, Description: description}, func(ctx agent.Context, in In) (Out, error) {
		var zero Out
		buyerID, err := d.scopedBuyer(ctx)
		if err != nil {
			return zero, err
		}
		return run(ctx, buyerID, in)
	})
	if err != nil {
		return nil, fmt.Errorf("buyeragent：建立 tool %s：%w", name, err)
	}
	return t, nil
}

func (d Deps) myOrdersTool() (tool.Tool, error) {
	return newTool(d, "get_my_orders", "查詢買家的訂單清單，可依日期範圍和狀態篩選",
		func(ctx context.Context, buyerID int64, in orderQueryInput) (orderQueryOutput, error) {
			pageSize := in.PageSize
			if pageSize <= 0 || pageSize > 20 {
				pageSize = 10
			}
			orders, err := d.Orders.MyOrders(ctx, buyerID, in.Status, 1, pageSize)
			if err != nil {
				return orderQueryOutput{}, fmt.Errorf("查詢訂單失敗: %w", err)
			}
			orders = filterByDateRange(orders, in.DateRange, d.Now())
			items := make([]orderItem, 0, len(orders))
			for _, o := range orders {
				items = append(items, orderItem{
					OrderNo: o.OrderNo, Status: statusLabel(o.Status), SellerName: o.SellerName,
					ItemCount: o.ItemCount, AmountTWD: o.Amount, CreateTimeMs: o.CreateTime.UnixMilli(),
				})
			}
			return orderQueryOutput{Orders: items, Total: len(items)}, nil
		})
}

func (d Deps) orderDetailTool() (tool.Tool, error) {
	return newTool(d, "get_buyer_order_detail", "查詢買家單筆訂單的詳細資訊",
		func(ctx context.Context, buyerID int64, in orderDetailInput) (orderDetailOutput, error) {
			detail, err := d.Orders.OrderDetail(ctx, buyerID, in.OrderNo)
			if err != nil {
				return orderDetailOutput{}, fmt.Errorf("查詢訂單 %s 失敗: %w", in.OrderNo, err)
			}
			return orderDetailOutput{Result: map[string]any{
				"order_no":        detail.OrderNo,
				"status":          statusLabel(detail.Status),
				"amount_twd":      detail.Amount,
				"seller_name":     detail.SellerName,
				"shipping_method": detail.ShippingMethod,
				"shipping_status": detail.ShippingStatus,
				"shipping_fee":    detail.ShippingFee,
				"recipient_name":  detail.RecipientName,
				"payment_method":  detail.PaymentMethod,
			}}, nil
		})
}

func (d Deps) trackingTool() (tool.Tool, error) {
	return newTool(d, "track_my_package", "查詢買家配送中的包裹物流狀態",
		func(ctx context.Context, buyerID int64, _ trackingInput) (trackingOutput, error) {
			orders, err := d.Orders.MyOrders(ctx, buyerID, "20", 1, 10)
			if err != nil {
				return trackingOutput{}, fmt.Errorf("查詢配送中訂單失敗: %w", err)
			}
			if len(orders) == 0 {
				return trackingOutput{Message: "目前沒有配送中的包裹"}, nil
			}
			packages := make([]trackingItem, 0, len(orders))
			for _, o := range orders {
				detail, err := d.Orders.OrderDetail(ctx, buyerID, o.OrderNo)
				if err != nil {
					// 照搬：單筆物流詳情查失敗就略過，不通知 LLM（疑似缺陷：買家看到的清單少一筆卻沒有說明）。
					continue
				}
				packages = append(packages, trackingItem{
					OrderNo: detail.OrderNo, Status: statusLabel(detail.Status), ShippingMethod: detail.ShippingMethod,
					ShippingStatus: detail.ShippingStatus, SellerName: detail.SellerName, Amount: detail.Amount,
				})
			}
			return trackingOutput{Packages: packages}, nil
		})
}

func filterByDateRange(orders []Order, dateRange string, now time.Time) []Order {
	var cutoff int64
	switch dateRange {
	case "today":
		cutoff = twtime.Midnight(now).UnixMilli()
	case "last_7_days":
		cutoff = now.AddDate(0, 0, -7).UnixMilli()
	default:
		return orders
	}
	filtered := make([]Order, 0, len(orders))
	for _, o := range orders {
		if o.CreateTime.UnixMilli() >= cutoff {
			filtered = append(filtered, o)
		}
	}
	return filtered
}
