// Package orderservice 呼叫遷移期的 order-service。
package orderservice

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
)

const timeout = 5 * time.Second

// Client 讀取買家的訂單資料。
type Client struct {
	rpc orderconnect.OrderServiceClient
}

// MerchantSummary 是單店消費金額的已解析投影。
type MerchantSummary struct {
	StoreID, Spent, Refunded int64
}

// SpendingRecord 是單筆訂單的已解析顯示資料。
type SpendingRecord struct {
	OrderNo       string
	OrderTime     time.Time
	Amount        int64
	Refunded      int64
	CouponApplied bool
}

// SpendingPage 是舊服務的一頁資料。
type SpendingPage struct {
	Records []SpendingRecord
	Total   int64
}

// New 建立以 h2c 呼叫並簽章的 client。
func New(addr, secret string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("orderservice：缺少位址")
	}
	return &Client{rpc: orderconnect.NewOrderServiceClient(grpcclient.HTTPClient(), "http://"+addr, connect.WithGRPC(), connect.WithInterceptors(grpcclient.Signer(secret, nil)))}, nil
}

// HasOpenOrders 查詢會阻擋刪帳的訂單狀態。
func (c *Client) HasOpenOrders(ctx context.Context, buyerID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetMyOrderStatusCounts(ctx, connect.NewRequest(&orderpb.GetMyOrderStatusCountsRequest{BuyerId: buyerID}))
	if err != nil {
		return false, fmt.Errorf("orderservice：GetMyOrderStatusCounts：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return false, fmt.Errorf("orderservice：GetMyOrderStatusCounts code=%d", resp.Msg.GetCode())
	}
	open := false
	for _, item := range resp.Msg.GetCounts() {
		count, err := strconv.ParseInt(item.GetCount(), 10, 64)
		if err != nil || count < 0 {
			return false, fmt.Errorf("orderservice：訂單筆數不合法")
		}
		isOpen, known := openStatuses[item.GetStatus()]
		if !known {
			return false, fmt.Errorf("orderservice：未知訂單狀態 %q", item.GetStatus())
		}
		if isOpen && count > 0 {
			open = true
		}
	}
	return open, nil
}

// openStatuses 是舊契約的訂單狀態（CodeMeta〈業務狀態碼〉的 OrderStatus，擁有者是 ordering）是否算未結：
// 付款前到送達都算，完成、退款、取消與配送失敗結案不算。不在表內的狀態一律當成錯誤，不推定為已結。買家與店家兩側共用
// 這張表。來源：docs/代碼管理（CodeMeta）/業務狀態碼.md 的 OrderStatus（Java 的 OrderStatus.java 同值）；
// TestOpenStatusesCoverCodeMetaOrderStatus 比對兩邊。
//
//	未結：0 待付款、10 已付款、20 待出貨、30 配送中、32 待取貨、35 物流異常、40 已送達（買家還沒確認收貨）
//	已結：50 已完成、54 配送失敗未退款、60 部分退款、70 全額退款、80 已取消
var openStatuses = map[string]bool{
	"0": true, "10": true, "20": true, "30": true, "32": true, "35": true, "40": true,
	"50": false, "54": false, "60": false, "70": false, "80": false,
}

// MerchantSummary 回傳買家在各店的消費摘要。
func (c *Client) MerchantSummary(ctx context.Context, buyerID int64) ([]MerchantSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetBuyerMerchantSummary(ctx, connect.NewRequest(&orderpb.GetBuyerMerchantSummaryRequest{BuyerId: buyerID}))
	if err != nil {
		return nil, fmt.Errorf("orderservice：GetBuyerMerchantSummary：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("orderservice：GetBuyerMerchantSummary code=%d", resp.Msg.GetCode())
	}
	out := make([]MerchantSummary, 0, len(resp.Msg.GetData()))
	for _, item := range resp.Msg.GetData() {
		storeID, err := positiveID(item.GetStoreId())
		if err != nil {
			return nil, err
		}
		spent, err := nonnegativeAmount(item.GetTotalSpending())
		if err != nil {
			return nil, err
		}
		refunded, err := nonnegativeAmount(item.GetTotalRefund())
		if err != nil {
			return nil, err
		}
		out = append(out, MerchantSummary{StoreID: storeID, Spent: spent, Refunded: refunded})
	}
	return out, nil
}

// SpendingRecords 回傳舊 RPC 的單頁紀錄與總數。
func (c *Client) SpendingRecords(ctx context.Context, buyerID, storeID int64, page, size int) (SpendingPage, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetBuyerOrdersForSeller(ctx, connect.NewRequest(&orderpb.GetBuyerOrdersForSellerRequest{BuyerId: buyerID, StoreId: storeID, Page: strconv.Itoa(page), Size: strconv.Itoa(size)}))
	if err != nil {
		return SpendingPage{}, fmt.Errorf("orderservice：GetBuyerOrdersForSeller：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return SpendingPage{}, fmt.Errorf("orderservice：GetBuyerOrdersForSeller code=%d", resp.Msg.GetCode())
	}
	total, err := nonnegativeAmount(resp.Msg.GetTotal())
	if err != nil {
		return SpendingPage{}, err
	}
	if resp.Msg.GetPage() != strconv.Itoa(page) || resp.Msg.GetPageSize() != strconv.Itoa(size) {
		return SpendingPage{}, fmt.Errorf("orderservice：訂單頁碼不一致")
	}
	out := SpendingPage{Total: total, Records: make([]SpendingRecord, 0, len(resp.Msg.GetRecords()))}
	for _, record := range resp.Msg.GetRecords() {
		if record.GetOrderNo() == "" {
			return SpendingPage{}, fmt.Errorf("orderservice：訂單號缺失")
		}
		milliseconds, err := nonnegativeAmount(record.GetCreateTime())
		if err != nil {
			return SpendingPage{}, fmt.Errorf("orderservice：訂單時間不合法: %w", err)
		}
		amount, err := nonnegativeAmount(record.GetActualAmount())
		if err != nil {
			return SpendingPage{}, err
		}
		refund, err := nonnegativeAmount(record.GetRefundAmount())
		if err != nil {
			return SpendingPage{}, err
		}
		discount, err := nonnegativeAmount(record.GetDiscountAmount())
		if err != nil {
			return SpendingPage{}, err
		}
		out.Records = append(out.Records, SpendingRecord{OrderNo: record.GetOrderNo(), OrderTime: time.UnixMilli(milliseconds).UTC(), Amount: amount, Refunded: refund, CouponApplied: discount > 0})
	}
	return out, nil
}

func positiveID(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("orderservice：店家 ID 不合法: %q", raw)
	}
	return v, nil
}

func nonnegativeAmount(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("orderservice：金額不合法: %q", raw)
	}
	return v, nil
}

// openStatusFilter 是 GetSellerOrders 的 status_filter：全部未結的狀態，逗號分隔（含 0 待付款：買家還能付款的訂單也會讓
// 店家關店後出問題，所以 GetSellerOrderStatusCounts 的四桶不夠，它不計 0，見 Java 的 getSellerOrderStatusCounts）。
var openStatusFilter = func() string {
	var codes []int
	for status, open := range openStatuses {
		if open {
			code, err := strconv.Atoi(status)
			if err != nil {
				panic("orderservice：openStatuses 的狀態碼不是數字：" + status)
			}
			codes = append(codes, code)
		}
	}
	slices.Sort(codes)
	parts := make([]string, len(codes))
	for i, code := range codes {
		parts[i] = strconv.Itoa(code)
	}
	return strings.Join(parts, ",")
}()

// HasOpenStoreOrders 查詢店家有沒有會阻擋關店或刪帳的訂單：任何未結狀態（含 0 待付款）的訂單，或還在處理中的售後。
// 兩個 RPC 都要查：GetSellerOrders 以未結狀態篩選、只取總筆數（meta.total_items，Java 的 countSellerOrders 與列表同一個
// query）；售後處理中的訂單狀態可能已是終態（例如已完成後才申請售後），只有 GetSellerOrderStatusCounts 的
// aftersales_count 看得到。任何錯誤、不認得的狀態或不合法的筆數都回錯誤（fail-closed），不推定為沒有訂單。
func (c *Client) HasOpenStoreOrders(ctx context.Context, storeID int64) (bool, error) {
	if storeID <= 0 {
		return false, fmt.Errorf("orderservice：店家 ID 不合法: %d", storeID)
	}
	open, err := c.sellerOrdersOpen(ctx, storeID)
	if err != nil || open {
		return open, err
	}
	return c.sellerAftersalesOpen(ctx, storeID)
}

func (c *Client) sellerOrdersOpen(ctx context.Context, storeID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetSellerOrders(ctx, connect.NewRequest(&orderpb.GetSellerOrdersRequest{
		StoreId: storeID, StatusFilter: openStatusFilter, Page: &orderpb.PageRequest{Page: "1", Size: "1"},
	}))
	if err != nil {
		return false, fmt.Errorf("orderservice：GetSellerOrders：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return false, fmt.Errorf("orderservice：GetSellerOrders code=%d", resp.Msg.GetCode())
	}
	// 篩選之後回來的每一筆都該是未結的；不認得或已結的狀態表示篩選沒有照預期運作，總數不可信。
	for _, order := range resp.Msg.GetOrders() {
		if open, known := openStatuses[order.GetStatus()]; !known {
			return false, fmt.Errorf("orderservice：未知訂單狀態 %q", order.GetStatus())
		} else if !open {
			return false, fmt.Errorf("orderservice：未結篩選回了已結的狀態 %q", order.GetStatus())
		}
	}
	total, err := nonnegativeAmount(resp.Msg.GetMeta().GetTotalItems())
	if err != nil {
		return false, fmt.Errorf("orderservice：訂單筆數不合法: %w", err)
	}
	return total > 0, nil
}

func (c *Client) sellerAftersalesOpen(ctx context.Context, storeID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetSellerOrderStatusCounts(ctx, connect.NewRequest(&orderpb.GetSellerOrderStatusCountsRequest{StoreId: storeID}))
	if err != nil {
		return false, fmt.Errorf("orderservice：GetSellerOrderStatusCounts：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return false, fmt.Errorf("orderservice：GetSellerOrderStatusCounts code=%d", resp.Msg.GetCode())
	}
	if resp.Msg.GetAftersalesCount() < 0 {
		return false, fmt.Errorf("orderservice：售後筆數不合法")
	}
	return resp.Msg.GetAftersalesCount() > 0, nil
}

// StoreOrderStats 是單店近期訂單統計的已解析投影（ListStoreOrderStats）。
type StoreOrderStats struct {
	StoreID               int64
	DistinctBuyers        int64 // 成交訂單的不重複買家數
	QualifyingOrders      int64 // 成交訂單數（營收口徑的狀態）
	SellerCancelledOrders int64 // 賣家取消的訂單數
}

const (
	statsPageSize = 1000 // 舊服務的上限
	// statsMaxPages 是翻頁的保險：一千頁等於一百萬家店，遠超過平台規模；超過代表游標沒有前進。
	statsMaxPages = 1000
)

// StoreOrderStats 回傳近 days 天（1 到 90）每家有成交或賣家取消訂單的店的統計，翻完所有頁。
// 舊服務以 store_id 做 keyset 分頁；任何錯誤、不合法的筆數、重複的店或沒有前進的游標都回錯誤，不回部分結果。
func (c *Client) StoreOrderStats(ctx context.Context, days int) ([]StoreOrderStats, error) {
	if days < 1 || days > 90 {
		return nil, fmt.Errorf("orderservice：days 必須在 1 到 90：%d", days)
	}
	var out []StoreOrderStats
	seen := map[int64]bool{}
	after := ""
	for range statsMaxPages {
		page, next, err := c.storeOrderStatsPage(ctx, days, after)
		if err != nil {
			return nil, err
		}
		for _, stat := range page {
			if seen[stat.StoreID] {
				return nil, fmt.Errorf("orderservice：ListStoreOrderStats 店家 %d 重複出現", stat.StoreID)
			}
			seen[stat.StoreID] = true
			out = append(out, stat)
		}
		if next == "" {
			return out, nil
		}
		if len(page) == 0 || next == after {
			return nil, fmt.Errorf("orderservice：ListStoreOrderStats 游標沒有前進：%q", next)
		}
		after = next
	}
	return nil, fmt.Errorf("orderservice：ListStoreOrderStats 超過 %d 頁仍未結束", statsMaxPages)
}

func (c *Client) storeOrderStatsPage(ctx context.Context, days int, after string) ([]StoreOrderStats, string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.ListStoreOrderStats(ctx, connect.NewRequest(&orderpb.ListStoreOrderStatsRequest{Days: int32(days), AfterStoreId: after, PageSize: statsPageSize})) //nolint:gosec // days 已在 StoreOrderStats 驗證為 1 到 90
	if err != nil {
		return nil, "", fmt.Errorf("orderservice：ListStoreOrderStats：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, "", fmt.Errorf("orderservice：ListStoreOrderStats code=%d", resp.Msg.GetCode())
	}
	if len(resp.Msg.GetStats()) > statsPageSize {
		return nil, "", fmt.Errorf("orderservice：ListStoreOrderStats 一頁超過 %d 筆", statsPageSize)
	}
	out := make([]StoreOrderStats, 0, len(resp.Msg.GetStats()))
	for _, item := range resp.Msg.GetStats() {
		storeID, err := positiveID(item.GetStoreId())
		if err != nil {
			return nil, "", err
		}
		buyers, qualifying, cancelled := int64(item.GetDistinctBuyers()), int64(item.GetQualifyingOrders()), int64(item.GetSellerCancelledOrders())
		if buyers < 0 || qualifying < 0 || cancelled < 0 || buyers > qualifying || (qualifying > 0 && buyers == 0) || qualifying+cancelled == 0 {
			return nil, "", fmt.Errorf("orderservice：店家 %d 的訂單統計不合法（買家 %d、成交 %d、賣家取消 %d）", storeID, buyers, qualifying, cancelled)
		}
		out = append(out, StoreOrderStats{StoreID: storeID, DistinctBuyers: buyers, QualifyingOrders: qualifying, SellerCancelledOrders: cancelled})
	}
	next := resp.Msg.GetNextAfterStoreId()
	if next != "" {
		if len(out) == 0 || next != resp.Msg.GetStats()[len(out)-1].GetStoreId() {
			return nil, "", fmt.Errorf("orderservice：ListStoreOrderStats 游標與本頁最後一家不符：%q", next)
		}
	}
	return out, next, nil
}

// Window 是統計的時間窗 [Start, End)，以付款時間比對；兩端都要是整毫秒（線上以 Unix ms 傳送，不整的值會被悄悄截斷）。
type Window struct {
	Start, End time.Time
}

// PeriodOrderStats 是單店在一個時間窗內的成立訂單統計（GetStorePeriodOrderStats）。成立訂單的定義與 ListStoreOrderStats
// 相同：已付款、未取消、未全額退款，自買單不排除。
type PeriodOrderStats struct {
	QualifyingOrders int64 // 成立訂單數
	DistinctBuyers   int64 // 成立訂單的不重複買家數
	Revenue          int64 // 營收（新台幣元）：實付金額扣掉已完成的退款金額
}

const (
	// periodStatsMaxWindows 與 periodStatsMaxSpan 是舊服務的上限（GetStorePeriodOrderStats 超過回 INVALID_PARAM）。
	periodStatsMaxWindows = 4
	periodStatsMaxSpan    = 90 * 24 * time.Hour
)

// StorePeriodOrderStats 回傳店家在每個時間窗內的成立訂單統計，依 windows 的順序，窗內沒有訂單時各值為 0。
// 窗不合法、任何錯誤、回應筆數與請求不符或數值不合法都回錯誤，不回部分結果。
func (c *Client) StorePeriodOrderStats(ctx context.Context, storeID int64, windows []Window) ([]PeriodOrderStats, error) {
	if storeID <= 0 {
		return nil, fmt.Errorf("orderservice：店家 ID 不合法: %d", storeID)
	}
	if len(windows) == 0 || len(windows) > periodStatsMaxWindows {
		return nil, fmt.Errorf("orderservice：時間窗必須是 1 到 %d 個：%d", periodStatsMaxWindows, len(windows))
	}
	req := &orderpb.GetStorePeriodOrderStatsRequest{StoreId: storeID, Windows: make([]*orderpb.StatsWindow, len(windows))}
	for i, w := range windows {
		if !w.Start.Before(w.End) || w.End.Sub(w.Start) > periodStatsMaxSpan || w.Start.UnixMilli() <= 0 ||
			w.Start.Nanosecond()%int(time.Millisecond) != 0 || w.End.Nanosecond()%int(time.Millisecond) != 0 {
			return nil, fmt.Errorf("orderservice：時間窗不合法：[%s, %s)", w.Start.Format(time.RFC3339Nano), w.End.Format(time.RFC3339Nano))
		}
		req.Windows[i] = &orderpb.StatsWindow{StartTime: strconv.FormatInt(w.Start.UnixMilli(), 10), EndTime: strconv.FormatInt(w.End.UnixMilli(), 10)}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetStorePeriodOrderStats(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("orderservice：GetStorePeriodOrderStats：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("orderservice：GetStorePeriodOrderStats code=%d", resp.Msg.GetCode())
	}
	if len(resp.Msg.GetWindows()) != len(windows) {
		return nil, fmt.Errorf("orderservice：GetStorePeriodOrderStats 回了 %d 個時間窗，請求 %d 個", len(resp.Msg.GetWindows()), len(windows))
	}
	out := make([]PeriodOrderStats, len(windows))
	for i, item := range resp.Msg.GetWindows() {
		orders, err := nonnegativeAmount(item.GetQualifyingOrders())
		if err != nil {
			return nil, fmt.Errorf("orderservice：成立訂單數不合法: %w", err)
		}
		buyers, err := nonnegativeAmount(item.GetDistinctBuyers())
		if err != nil {
			return nil, fmt.Errorf("orderservice：買家數不合法: %w", err)
		}
		revenue, err := nonnegativeAmount(item.GetRevenue())
		if err != nil {
			return nil, fmt.Errorf("orderservice：營收不合法: %w", err)
		}
		if buyers > orders || (orders > 0 && buyers == 0) || (orders == 0 && revenue != 0) {
			return nil, fmt.Errorf("orderservice：時間窗 %d 的統計不合法（成立訂單 %d、買家 %d、營收 %d）", i, orders, buyers, revenue)
		}
		out[i] = PeriodOrderStats{QualifyingOrders: orders, DistinctBuyers: buyers, Revenue: revenue}
	}
	return out, nil
}

// SellerOrderStatusCounts 是店家訂單狀態的目前快照（GetSellerOrderStatusCounts，賣家工作台四桶，定義見 order.proto 與設計書 03 §2.1）。
type SellerOrderStatusCounts struct {
	PendingShipment int64
	InProgress      int64
	Closed          int64 // 已結束，含已取消與全額退款，不是成交數
	Aftersales      int64
}

// SellerOrderStatusCounts 回傳店家的四桶訂單計數；任何錯誤或負數都回錯誤，不回部分結果。
func (c *Client) SellerOrderStatusCounts(ctx context.Context, storeID int64) (SellerOrderStatusCounts, error) {
	if storeID <= 0 {
		return SellerOrderStatusCounts{}, fmt.Errorf("orderservice：店家 ID 不合法: %d", storeID)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetSellerOrderStatusCounts(ctx, connect.NewRequest(&orderpb.GetSellerOrderStatusCountsRequest{StoreId: storeID}))
	if err != nil {
		return SellerOrderStatusCounts{}, fmt.Errorf("orderservice：GetSellerOrderStatusCounts：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return SellerOrderStatusCounts{}, fmt.Errorf("orderservice：GetSellerOrderStatusCounts code=%d", resp.Msg.GetCode())
	}
	out := SellerOrderStatusCounts{
		PendingShipment: resp.Msg.GetPendingShipmentCount(), InProgress: resp.Msg.GetInProgressCount(),
		Closed: resp.Msg.GetCompletedCount(), Aftersales: resp.Msg.GetAftersalesCount(),
	}
	if out.PendingShipment < 0 || out.InProgress < 0 || out.Closed < 0 || out.Aftersales < 0 {
		return SellerOrderStatusCounts{}, fmt.Errorf("orderservice：訂單狀態計數不合法: %+v", out)
	}
	return out, nil
}
