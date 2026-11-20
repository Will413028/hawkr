package buyeragent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool/functiontool"
)

// StateKeyBuyer 是 session state 裡買家 ID 的 key；值以十進位字串存，數字型的 ID 經 JSONB 往返會被 float64 改寫。
const StateKeyBuyer = "buyer_id"

// ErrBuyerScope 是 tool 發現 session 的買家與這一輪的買家不一致（或缺值）時的錯誤。
var ErrBuyerScope = errors.New("buyeragent：session 的買家與這一輪不一致")

type orderQueryInput struct {
	DateRange string `json:"date_range" jsonschema:"日期範圍：today, last_7_days, all"`
	Status    string `json:"status,omitempty" jsonschema:"訂單狀態碼（見系統提示的狀態碼表），可用逗號分隔多個，空字串表示全部"`
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
	// Note 是不帶語言的代碼（noteNoPackages、noteSomePackagesUnavailable、noteMorePackagesNotShown），多個以逗號分隔，模型依買家的語言轉述。
	Note     string         `json:"note,omitempty"`
	Packages []trackingItem `json:"packages"`
}

// trackingStatuses 是「追蹤中的包裹」的訂單狀態：20 待出貨（已建物流單）、30 配送中、32 待取貨、35 物流異常、40 已送達、
// 54 配送失敗未退款。集合由 ordering 擁有（CodeMeta〈業務狀態碼〉的 OrderStatus；order-service 的 status filter 以逗號分隔多個值，
// 見 OrderService.parseStatusFilter）：ordering 搬進單體後改由它提供，這裡不另行決定；每筆包裹帶自己的狀態碼，由模型回答。
const trackingStatuses = "20,30,32,35,40,54"

// upstream 把訂單服務的錯誤換成給模型的代碼；錯誤原因由 guardedTool 記一次 log。port 約定「不存在或不屬於該買家」回 ErrOrderNotFound。
func upstream(err error) error {
	if errors.Is(err, ErrOrderNotFound) {
		return toolFail(codeOrderNotFound, err)
	}
	return toolFail(codeUpstreamUnavailable, err)
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

// newTool 建一支 tool：handler 先驗身分範圍，再執行；外面包一層 guardedTool（唯一能掛上 agent 的型別）。
func newTool[In, Out any](d Deps, id model.Tool, name, description string, run func(ctx context.Context, buyerID int64, in In) (Out, error)) (*guardedTool, error) {
	t, err := functiontool.New(functiontool.Config{Name: name, Description: description}, func(ctx agent.Context, in In) (Out, error) {
		var zero Out
		buyerID, err := d.scopedBuyer(ctx)
		if err != nil {
			return zero, toolFail(codeForbidden, err)
		}
		return run(ctx, buyerID, in)
	})
	if err != nil {
		return nil, fmt.Errorf("buyeragent：建立 tool %s：%w", name, err)
	}
	c, ok := t.(callable)
	if !ok {
		return nil, fmt.Errorf("buyeragent：tool %s 不能執行（%T）", name, t)
	}
	return &guardedTool{inner: c, id: id, out: d.OnTool}, nil
}

func (d Deps) myOrdersTool() (*guardedTool, error) {
	return newTool(d, model.ToolGetMyOrders, "get_my_orders", "查詢買家的訂單清單，可依日期範圍和狀態篩選",
		func(ctx context.Context, buyerID int64, in orderQueryInput) (orderQueryOutput, error) {
			pageSize := in.PageSize
			if pageSize <= 0 || pageSize > 20 {
				pageSize = 10
			}
			orders, err := d.Orders.MyOrders(ctx, buyerID, in.Status, 1, pageSize)
			if err != nil {
				return orderQueryOutput{}, upstream(err)
			}
			orders = filterByDateRange(orders, in.DateRange, d.Now())
			items := make([]orderItem, 0, len(orders))
			for _, o := range orders {
				items = append(items, orderItem{
					OrderNo: o.OrderNo, Status: o.Status, SellerName: o.SellerName,
					ItemCount: o.ItemCount, AmountTWD: o.Amount, CreateTimeMs: o.CreateTime.UnixMilli(),
				})
			}
			return orderQueryOutput{Orders: items, Total: len(items)}, nil
		})
}

func (d Deps) orderDetailTool() (*guardedTool, error) {
	return newTool(d, model.ToolGetBuyerOrderDetail, "get_buyer_order_detail", "查詢買家單筆訂單的詳細資訊",
		func(ctx context.Context, buyerID int64, in orderDetailInput) (orderDetailOutput, error) {
			detail, err := d.Orders.OrderDetail(ctx, buyerID, in.OrderNo)
			if err != nil {
				return orderDetailOutput{}, upstream(err)
			}
			return orderDetailOutput{Result: map[string]any{
				"order_no":        detail.OrderNo,
				"status":          detail.Status,
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

// trackingPageSize 是追蹤 tool 一次查的包裹數。舊 order-service 的列表只有 status filter 與分頁，沒有「排除已送達」之類的條件
// （legacy/orderservice/buyerorders.go MyOrders：只有 statusFilter、page、pageSize），每多一頁就多 N 次詳情查詢；
// 所以只查第一頁，滿頁時以 noteMorePackagesNotShown 告訴模型清單可能不完整，不在這裡翻頁。
const trackingPageSize = 10

func (d Deps) trackingTool() (*guardedTool, error) {
	return newTool(d, model.ToolTrackMyPackage, "track_my_package", "查詢買家包裹的物流狀態",
		func(ctx context.Context, buyerID int64, _ trackingInput) (trackingOutput, error) {
			orders, err := d.Orders.MyOrders(ctx, buyerID, trackingStatuses, 1, trackingPageSize)
			if err != nil {
				return trackingOutput{}, upstream(err)
			}
			if len(orders) == 0 {
				return trackingOutput{Note: noteNoPackages}, nil
			}
			packages := make([]trackingItem, 0, len(orders))
			// 詳情失敗的原因：訂單已不存在（ErrOrderNotFound）靜默略過；其餘是上游問題。
			var upstreamFailures []error
			gone := 0
			for _, o := range orders {
				detail, err := d.Orders.OrderDetail(ctx, buyerID, o.OrderNo)
				if err != nil {
					if errors.Is(err, ErrOrderNotFound) {
						gone++
					} else {
						upstreamFailures = append(upstreamFailures, fmt.Errorf("訂單 %s：%w", o.OrderNo, err))
					}
					continue
				}
				packages = append(packages, trackingItem{
					OrderNo: detail.OrderNo, Status: detail.Status, ShippingMethod: detail.ShippingMethod,
					ShippingStatus: detail.ShippingStatus, SellerName: detail.SellerName, Amount: detail.Amount,
				})
			}
			if len(packages) == 0 {
				// 全部失敗一律 upstream_unavailable，不看最後一筆是什麼錯誤；原因合併成一個 error，由 guardedTool 記一次 log
				// （這條路徑不另外逐筆記 log，避免同一批原因記兩次）。
				if gone > 0 {
					upstreamFailures = append(upstreamFailures, fmt.Errorf("另有 %d 筆訂單已不存在：%w", gone, ErrOrderNotFound))
				}
				return trackingOutput{}, toolFail(codeUpstreamUnavailable, errors.Join(upstreamFailures...))
			}
			// 部分失敗：逐筆記 log（含訂單號與 tool 名稱），因為 guardedTool 只會在整體失敗時記。
			for _, err := range upstreamFailures {
				slog.WarnContext(ctx, "assistant：包裹詳情查詢失敗", "tool", "track_my_package", "error", err)
			}
			var notes []string
			if len(upstreamFailures) > 0 {
				notes = append(notes, noteSomePackagesUnavailable)
			}
			if len(orders) == trackingPageSize {
				notes = append(notes, noteMorePackagesNotShown)
			}
			return trackingOutput{Note: strings.Join(notes, ","), Packages: packages}, nil
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
