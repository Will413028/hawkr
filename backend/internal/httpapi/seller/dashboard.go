package seller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// dashboardBudget 是儀表板讀每一個來源（訂單統計、訂單狀態、粉絲、瀏覽數）的時間上限。儀表板是賣家登入後才看的一頁，
// 不在公開的熱路徑上，所以比店面頁的 statsBudget（300 ms）寬鬆：寧可多等一下，也不要因為來源稍慢就讓區塊消失。
const dashboardBudget = time.Second

// dashboardDays 是儀表板每個期間的天數。
const dashboardDays = 7

// SalesWindow 是訂單統計的時間窗 [Start, End)，以付款時間比對。
type SalesWindow struct {
	Start, End time.Time
}

// SalesStats 是一個時間窗內的成立訂單統計：已付款、未取消、未全額退款的訂單（口徑由訂單的擁有者定義）。
type SalesStats struct {
	Orders    int64 // 成立訂單數
	Customers int64 // 成立訂單的不重複買家數
	Revenue   int64 // 營收（新台幣元）：實付金額扣掉已完成的退款金額
}

// OrderStatusCounts 是店家訂單狀態的目前快照（賣家工作台四桶）。
type OrderStatusCounts struct {
	PendingShipment int64
	InProgress      int64
	Closed          int64 // 已結束，含已取消與全額退款，不是成交數
	Aftersales      int64
}

// DashboardOrders 是儀表板的訂單來源（訂單還在舊服務，由組裝點接上）：失敗回錯誤，不推定為沒有訂單。SalesStats 依 windows
// 的順序逐窗回傳，窗內沒有訂單時各值是 0。
type DashboardOrders interface {
	SalesStats(ctx context.Context, storeID int64, windows []SalesWindow) ([]SalesStats, error)
	StatusCounts(ctx context.Context, storeID int64) (OrderStatusCounts, error)
}

// ProductViews 是儀表板的商品瀏覽數來源（seller 模組）：[start, end) 是左閉右開的期間，兩端必須是台北的 00:00。
type ProductViews interface {
	ProductViewsBetween(ctx context.Context, storeID int64, start, end time.Time) (int64, error)
}

// FollowerStats 是儀表板的粉絲來源（buyer 模組擁有追蹤）：一次回傳粉絲總數與每個期間 [Start, End) 內新增的追蹤數，
// 總數與各期間來自同一個 snapshot。
type FollowerStats interface {
	SubscriberStats(ctx context.Context, storeID int64, windows []buyer.TimeWindow) (buyer.SubscriberStats, error)
}

// Dashboard 是 GetMyDashboard 的來源與時鐘；組裝點（httpapi.NewHandler）必須提供三個來源。
type Dashboard struct {
	Orders    DashboardOrders
	Views     ProductViews
	Followers FollowerStats
	// Now 是目前時間；nil 時用 time.Now。
	Now func() time.Time
	// Budget 是讀每個來源的時間上限；零值時用 dashboardBudget。
	Budget time.Duration
	// Log 記錄讀不到的區塊；nil 時不記錄。
	Log *slog.Logger
}

func (d Dashboard) now() time.Time {
	if d.Now == nil {
		return time.Now()
	}
	return d.Now()
}

func (d Dashboard) budget() time.Duration {
	if d.Budget <= 0 {
		return dashboardBudget
	}
	return d.Budget
}

// dashboardPeriod 是一個期間：[start, end) 的台北 00:00，共 dashboardDays 個日曆日。
type dashboardPeriod struct {
	start, end time.Time
}

func (p dashboardPeriod) window() SalesWindow { return SalesWindow{Start: p.start, End: p.end} }

// proto 回報含頭含尾的台北日曆日。
func (p dashboardPeriod) proto() *pb.DashboardPeriod {
	return &pb.DashboardPeriod{FirstDate: protoDate(p.start), LastDate: protoDate(p.end.AddDate(0, 0, -1))}
}

func protoDate(t time.Time) *date.Date {
	y, m, d := t.Date()
	return &date.Date{Year: int32(y), Month: int32(m), Day: int32(d)} //nolint:gosec // 年月日在 int32 範圍內
}

// dashboardPeriods 回傳 now 當下的兩個期間：current 是最近 7 個完整的台北日曆日（不含今天），previous 是它前面緊鄰的 7 日。
func dashboardPeriods(now time.Time) (current, previous dashboardPeriod) {
	today := twtime.Midnight(now)
	current = dashboardPeriod{start: today.AddDate(0, 0, -dashboardDays), end: today}
	previous = dashboardPeriod{start: today.AddDate(0, 0, -2*dashboardDays), end: current.start}
	return current, previous
}

// GetMyDashboard 平行讀四個區塊（銷售、粉絲、商品瀏覽數、訂單狀態），各自有 dashboardBudget：任何一個失敗、逾時或數值放不進
// 契約的 int32，只讓它的區塊不設（記 warn），不會變成 0，也不讓方法失敗。四個 goroutine 各寫自己的欄位。
func (h *App) GetMyDashboard(ctx context.Context, _ *connect.Request[pb.GetMyDashboardRequest]) (*connect.Response[pb.GetMyDashboardResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	cur, prev := dashboardPeriods(h.dash.now())
	out := &pb.Dashboard{CurrentPeriod: cur.proto(), PreviousPeriod: prev.proto()}
	var wg sync.WaitGroup
	wg.Add(4)
	section := func(read func(ctx context.Context)) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(ctx, h.dash.budget())
		defer cancel()
		read(ctx)
	}
	go section(func(ctx context.Context) { out.Sales = h.dashboardSales(ctx, current.StoreID, cur, prev) })
	go section(func(ctx context.Context) { out.Followers = h.dashboardFollowers(ctx, current.StoreID, cur, prev) })
	go section(func(ctx context.Context) { out.ProductViews = h.dashboardViews(ctx, current.StoreID, cur, prev) })
	go section(func(ctx context.Context) { out.OrderStatusCounts = h.dashboardOrderStatus(ctx, current.StoreID) })
	wg.Wait()
	return connect.NewResponse(&pb.GetMyDashboardResponse{Dashboard: out}), nil
}

func (h *App) dashboardLog() *slog.Logger {
	if h.dash.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return h.dash.Log
}

// periodCount 把兩個期間的計數放進契約的 int32；任何一個放不下（或為負）時整組不設。
func periodCount(current, previous int64) *pb.PeriodCount {
	c, p := count32(current), count32(previous)
	if c == nil || p == nil {
		return nil
	}
	return &pb.PeriodCount{Current: *c, Previous: *p}
}

func (h *App) dashboardSales(ctx context.Context, storeID int64, cur, prev dashboardPeriod) *pb.DashboardSales {
	stats, err := h.dash.Orders.SalesStats(ctx, storeID, []SalesWindow{cur.window(), prev.window()})
	if err != nil {
		h.dashboardLog().WarnContext(ctx, "seller：讀取儀表板的銷售失敗，dashboard.sales 不設", "store_id", storeID, "error", err)
		return nil
	}
	if len(stats) != 2 {
		h.dashboardLog().ErrorContext(ctx, "seller：儀表板的銷售回應的時間窗數量不符，dashboard.sales 不設", "store_id", storeID, "windows", len(stats))
		return nil
	}
	c, p := stats[0], stats[1]
	orders, customers := periodCount(c.Orders, p.Orders), periodCount(c.Customers, p.Customers)
	if orders == nil || customers == nil || c.Revenue < 0 || p.Revenue < 0 {
		h.dashboardLog().ErrorContext(ctx, "seller：儀表板的銷售數值不合法或超出範圍，dashboard.sales 不設", "store_id", storeID)
		return nil
	}
	return &pb.DashboardSales{Revenue: &pb.PeriodAmount{Current: c.Revenue, Previous: p.Revenue}, Orders: orders, Customers: customers}
}

func (h *App) dashboardFollowers(ctx context.Context, storeID int64, cur, prev dashboardPeriod) *pb.DashboardFollowers {
	stats, err := h.dash.Followers.SubscriberStats(ctx, storeID, []buyer.TimeWindow{{Start: cur.start, End: cur.end}, {Start: prev.start, End: prev.end}})
	if err != nil {
		h.dashboardLog().WarnContext(ctx, "seller：讀取儀表板的粉絲失敗，dashboard.followers 不設", "store_id", storeID, "error", err)
		return nil
	}
	if len(stats.New) != 2 {
		h.dashboardLog().ErrorContext(ctx, "seller：儀表板的粉絲回應的時間窗數量不符，dashboard.followers 不設", "store_id", storeID, "windows", len(stats.New))
		return nil
	}
	total, fresh := count32(stats.Total), periodCount(stats.New[0], stats.New[1])
	if total == nil || fresh == nil {
		h.dashboardLog().ErrorContext(ctx, "seller：儀表板的粉絲數值超出範圍，dashboard.followers 不設", "store_id", storeID)
		return nil
	}
	return &pb.DashboardFollowers{Total: *total, NewFollowers: fresh}
}

func (h *App) dashboardViews(ctx context.Context, storeID int64, cur, prev dashboardPeriod) *pb.PeriodCount {
	current, err := h.dash.Views.ProductViewsBetween(ctx, storeID, cur.start, cur.end)
	if err != nil {
		h.dashboardLog().WarnContext(ctx, "seller：讀取儀表板的商品瀏覽數失敗，dashboard.product_views 不設", "store_id", storeID, "error", err)
		return nil
	}
	previous, err := h.dash.Views.ProductViewsBetween(ctx, storeID, prev.start, prev.end)
	if err != nil {
		h.dashboardLog().WarnContext(ctx, "seller：讀取儀表板的商品瀏覽數失敗，dashboard.product_views 不設", "store_id", storeID, "error", err)
		return nil
	}
	views := periodCount(current, previous)
	if views == nil {
		h.dashboardLog().ErrorContext(ctx, "seller：儀表板的商品瀏覽數超出範圍，dashboard.product_views 不設", "store_id", storeID)
	}
	return views
}

func (h *App) dashboardOrderStatus(ctx context.Context, storeID int64) *pb.OrderStatusCounts {
	counts, err := h.dash.Orders.StatusCounts(ctx, storeID)
	if err != nil {
		h.dashboardLog().WarnContext(ctx, "seller：讀取儀表板的訂單狀態失敗，dashboard.order_status_counts 不設", "store_id", storeID, "error", err)
		return nil
	}
	pending, inProgress, closed, aftersales := count32(counts.PendingShipment), count32(counts.InProgress), count32(counts.Closed), count32(counts.Aftersales)
	if pending == nil || inProgress == nil || closed == nil || aftersales == nil {
		h.dashboardLog().ErrorContext(ctx, "seller：儀表板的訂單狀態數值超出範圍，dashboard.order_status_counts 不設", "store_id", storeID)
		return nil
	}
	return &pb.OrderStatusCounts{PendingShipmentCount: *pending, InProgressCount: *inProgress, ClosedCount: *closed, AftersalesCount: *aftersales}
}
