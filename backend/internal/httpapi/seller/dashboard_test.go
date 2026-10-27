package seller

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// dashSources 同時是儀表板的三個來源。每個來源可以注入錯誤、卡住直到逾時，或回傳指定的值；
// 每個 goroutine 只寫自己那一組欄位（銷售、訂單狀態、粉絲、瀏覽數各一組），所以不需要鎖。
type dashSources struct {
	// 銷售
	sales       []SalesStats
	salesErr    error
	salesHangs  bool
	salesWindow []SalesWindow
	salesStore  int64
	// 訂單狀態
	counts      OrderStatusCounts
	countsErr   error
	countsHangs bool
	// 粉絲
	followers       buyer.SubscriberStats
	followersErr    error
	followersHangs  bool
	followerWindows []buyer.TimeWindow
	followerStore   int64
	followerCalls   int
	// 瀏覽數
	viewsCurrent int64
	viewsPrev    int64
	viewsErr     error
	viewsHangs   bool
	viewRanges   [][2]time.Time
}

func hang(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (f *dashSources) SalesStats(ctx context.Context, storeID int64, windows []SalesWindow) ([]SalesStats, error) {
	f.salesStore, f.salesWindow = storeID, windows
	if f.salesHangs {
		return nil, hang(ctx)
	}
	return f.sales, f.salesErr
}

func (f *dashSources) StatusCounts(ctx context.Context, _ int64) (OrderStatusCounts, error) {
	if f.countsHangs {
		return OrderStatusCounts{}, hang(ctx)
	}
	return f.counts, f.countsErr
}

func (f *dashSources) SubscriberStats(ctx context.Context, storeID int64, windows []buyer.TimeWindow) (buyer.SubscriberStats, error) {
	f.followerCalls++
	f.followerStore, f.followerWindows = storeID, windows
	if f.followersHangs {
		return buyer.SubscriberStats{}, hang(ctx)
	}
	return f.followers, f.followersErr
}

func (f *dashSources) ProductViewsBetween(ctx context.Context, _ int64, start, end time.Time) (int64, error) {
	f.viewRanges = append(f.viewRanges, [2]time.Time{start, end})
	if f.viewsHangs {
		return 0, hang(ctx)
	}
	if f.viewsErr != nil {
		return 0, f.viewsErr
	}
	if len(f.viewRanges) == 1 {
		return f.viewsCurrent, nil
	}
	return f.viewsPrev, nil
}

// healthy 回傳每個欄位都不同的來源，欄位對調時測試會看出來。
func healthy() *dashSources {
	return &dashSources{
		sales:        []SalesStats{{Orders: 12, Customers: 7, Revenue: 1000}, {Orders: 10, Customers: 6, Revenue: 900}},
		counts:       OrderStatusCounts{PendingShipment: 3, InProgress: 4, Closed: 5, Aftersales: 6},
		followers:    buyer.SubscriberStats{Total: 55, New: []int64{5, 4}},
		viewsCurrent: 300,
		viewsPrev:    250,
	}
}

var clockAtTaipeiDayBoundary = time.Date(2026, 10, 4, 0, 30, 0, 0, twtime.Taipei) // = 2026-10-03 16:30 UTC

func dashboardHandler(t *testing.T, src *dashSources, now time.Time) *App {
	return mustApp(t, &fakeModule{}, nil, rpc.Images{}, Dashboard{
		Orders: src, Views: src, Followers: src,
		Now: func() time.Time { return now }, Budget: 50 * time.Millisecond,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, testStats(fakeFollowerCounts{}, fakeRatings{}))
}

func getDashboard(t *testing.T, h *App) *pb.Dashboard {
	t.Helper()
	resp, err := h.GetMyDashboard(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyDashboardRequest{}))
	if err != nil {
		t.Fatalf("GetMyDashboard 不該因為區塊失敗而失敗：%v", err)
	}
	return resp.Msg.GetDashboard()
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, twtime.Taipei) }

func TestGetMyDashboardMapsEverySectionFromTheRightSource(t *testing.T) {
	src := healthy()
	d := getDashboard(t, dashboardHandler(t, src, clockAtTaipeiDayBoundary))

	want := &pb.Dashboard{
		CurrentPeriod:  &pb.DashboardPeriod{FirstDate: protoDate(day(2026, 9, 27)), LastDate: protoDate(day(2026, 10, 3))},
		PreviousPeriod: &pb.DashboardPeriod{FirstDate: protoDate(day(2026, 9, 20)), LastDate: protoDate(day(2026, 9, 26))},
		Sales: &pb.DashboardSales{
			Revenue:   &pb.PeriodAmount{Current: 1000, Previous: 900},
			Orders:    &pb.PeriodCount{Current: 12, Previous: 10},
			Customers: &pb.PeriodCount{Current: 7, Previous: 6},
		},
		Followers:         &pb.DashboardFollowers{Total: 55, NewFollowers: &pb.PeriodCount{Current: 5, Previous: 4}},
		ProductViews:      &pb.PeriodCount{Current: 300, Previous: 250},
		OrderStatusCounts: &pb.OrderStatusCounts{PendingShipmentCount: 3, InProgressCount: 4, ClosedCount: 5, AftersalesCount: 6},
	}
	if !proto.Equal(d, want) {
		t.Fatalf("dashboard=%v\nwant=%v", d, want)
	}
	if d.GetCurrentPeriod().GetFirstDate().GetDay() != 27 || d.GetPreviousPeriod().GetLastDate().GetDay() != 26 {
		t.Fatalf("期間的日期不對：%v %v", d.GetCurrentPeriod(), d.GetPreviousPeriod())
	}
}

// 台北日的界線：00:30（UTC 還是前一天）以台北的今天為準，期間不含今天。
func TestDashboardPeriodsFollowTheTaipeiDayAndPassTheSameWindowsToEveryPort(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		now                          time.Time
		curFirst, curLast, prevFirst time.Time
	}{
		{"台北 00:30（UTC 是前一天 16:30）", time.Date(2026, 10, 3, 16, 30, 0, 0, time.UTC), day(2026, 9, 27), day(2026, 10, 3), day(2026, 9, 20)},
		{"台北 00:00 整", day(2026, 10, 4), day(2026, 9, 27), day(2026, 10, 3), day(2026, 9, 20)},
		{"台北 23:59:59 還是同一天", time.Date(2026, 10, 4, 23, 59, 59, 0, twtime.Taipei), day(2026, 9, 27), day(2026, 10, 3), day(2026, 9, 20)},
		{"前一秒屬於前一天", time.Date(2026, 10, 3, 15, 59, 59, 0, time.UTC), day(2026, 9, 26), day(2026, 10, 2), day(2026, 9, 19)},
		{"跨年", time.Date(2027, 1, 3, 12, 0, 0, 0, twtime.Taipei), day(2026, 12, 27), day(2027, 1, 2), day(2026, 12, 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := healthy()
			d := getDashboard(t, dashboardHandler(t, src, tc.now))

			curEnd := tc.curLast.AddDate(0, 0, 1)
			wantSales := []SalesWindow{{tc.curFirst, curEnd}, {tc.prevFirst, tc.curFirst}}
			if len(src.salesWindow) != 2 || !src.salesWindow[0].Start.Equal(wantSales[0].Start) || !src.salesWindow[0].End.Equal(wantSales[0].End) ||
				!src.salesWindow[1].Start.Equal(wantSales[1].Start) || !src.salesWindow[1].End.Equal(wantSales[1].End) {
				t.Fatalf("訂單 port 收到的時間窗 %v，應為 %v", src.salesWindow, wantSales)
			}
			if src.followerCalls != 1 || len(src.followerWindows) != 2 ||
				!src.followerWindows[0].Start.Equal(tc.curFirst) || !src.followerWindows[0].End.Equal(curEnd) ||
				!src.followerWindows[1].Start.Equal(tc.prevFirst) || !src.followerWindows[1].End.Equal(tc.curFirst) {
				t.Fatalf("粉絲 port 應只被呼叫一次，收到的區間 %v（%d 次）", src.followerWindows, src.followerCalls)
			}
			if len(src.viewRanges) != 2 ||
				!src.viewRanges[0][0].Equal(tc.curFirst) || !src.viewRanges[0][1].Equal(curEnd) ||
				!src.viewRanges[1][0].Equal(tc.prevFirst) || !src.viewRanges[1][1].Equal(tc.curFirst) {
				t.Fatalf("瀏覽數 port 收到的區間 %v", src.viewRanges)
			}
			for _, r := range [][2]time.Time{src.viewRanges[0], src.viewRanges[1]} {
				if r[0].Hour() != 0 || r[1].Hour() != 0 {
					t.Fatalf("瀏覽數的期間兩端應是台北 00:00：%v", r)
				}
			}
			gotFirst := d.GetCurrentPeriod().GetFirstDate()
			if int(gotFirst.GetYear()) != tc.curFirst.Year() || time.Month(gotFirst.GetMonth()) != tc.curFirst.Month() || int(gotFirst.GetDay()) != tc.curFirst.Day() {
				t.Fatalf("current_period.first_date=%v", gotFirst)
			}
			if src.salesStore != 11 || src.followerStore != 11 {
				t.Fatalf("來源收到的店家不是 ctx 裡的店家：sales=%d followers=%d", src.salesStore, src.followerStore)
			}
		})
	}
}

// 每一個區塊獨立：它的來源失敗、逾時、回傳不合法或放不進 int32 的值時，只有它不設，其他區塊照樣有值，方法本身仍然成功。
func TestEachDashboardSectionIsIndependentlyUnsetWhenItsSourceIsUnusable(t *testing.T) {
	boom := errors.New("down")
	tooBig := int64(math.MaxInt32) + 1
	type section string
	const (
		sales     section = "sales"
		followers section = "followers"
		views     section = "product_views"
		status    section = "order_status_counts"
	)
	for _, tc := range []struct {
		name    string
		broken  section
		corrupt func(*dashSources)
	}{
		{"sales: 訂單服務錯誤", sales, func(s *dashSources) { s.salesErr = boom }},
		{"sales: 逾時", sales, func(s *dashSources) { s.salesHangs = true }},
		{"sales: 回應的時間窗數量不符", sales, func(s *dashSources) { s.sales = s.sales[:1] }},
		{"sales: 空的回應", sales, func(s *dashSources) { s.sales = nil }},
		{"sales: 訂單數超出 int32", sales, func(s *dashSources) { s.sales[1].Orders = tooBig }},
		{"sales: 顧客數超出 int32", sales, func(s *dashSources) { s.sales[0].Customers = tooBig }},
		{"sales: 營收是負數", sales, func(s *dashSources) { s.sales[0].Revenue = -1 }},
		{"followers: 讀取錯誤", followers, func(s *dashSources) { s.followersErr = boom }},
		{"followers: 逾時", followers, func(s *dashSources) { s.followersHangs = true }},
		{"followers: 時間窗數量不符", followers, func(s *dashSources) { s.followers.New = []int64{5} }},
		{"followers: 沒有時間窗", followers, func(s *dashSources) { s.followers.New = nil }},
		{"followers: 總數超出 int32", followers, func(s *dashSources) { s.followers.Total = tooBig }},
		{"followers: 新增數超出 int32", followers, func(s *dashSources) { s.followers.New[1] = tooBig }},
		{"views: 讀取錯誤", views, func(s *dashSources) { s.viewsErr = boom }},
		{"views: 逾時", views, func(s *dashSources) { s.viewsHangs = true }},
		{"views: 超出 int32", views, func(s *dashSources) { s.viewsCurrent = tooBig }},
		{"views: 負數", views, func(s *dashSources) { s.viewsPrev = -1 }},
		{"status: 訂單服務錯誤", status, func(s *dashSources) { s.countsErr = boom }},
		{"status: 逾時", status, func(s *dashSources) { s.countsHangs = true }},
		{"status: 超出 int32", status, func(s *dashSources) { s.counts.Closed = tooBig }},
		{"status: 負數", status, func(s *dashSources) { s.counts.Aftersales = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := healthy()
			tc.corrupt(src)
			d := getDashboard(t, dashboardHandler(t, src, clockAtTaipeiDayBoundary))

			set := map[section]bool{
				sales: d.Sales != nil, followers: d.Followers != nil, views: d.ProductViews != nil, status: d.OrderStatusCounts != nil,
			}
			for name, isSet := range set {
				if name == tc.broken && isSet {
					t.Errorf("%s 的來源壞了，該區塊應該不設，卻是 %v", name, d)
				}
				if name != tc.broken && !isSet {
					t.Errorf("%s 的來源是好的，卻被 %s 連累而不設", name, tc.broken)
				}
			}
			if d.GetCurrentPeriod() == nil || d.GetPreviousPeriod() == nil {
				t.Errorf("期間不依賴任何來源，必須一直有值：%v", d)
			}
		})
	}
}

// 四個來源全部壞掉：方法仍然成功，只剩期間。
func TestDashboardWithEveryDownstreamFailingStillSucceedsWithOnlyThePeriods(t *testing.T) {
	boom := errors.New("down")
	src := &dashSources{salesErr: boom, countsErr: boom, followersErr: boom, viewsErr: boom}
	d := getDashboard(t, dashboardHandler(t, src, clockAtTaipeiDayBoundary))
	if d.Sales != nil || d.Followers != nil || d.ProductViews != nil || d.OrderStatusCounts != nil {
		t.Fatalf("壞掉的區塊必須不設：%v", d)
	}
	if d.CurrentPeriod == nil || d.PreviousPeriod == nil {
		t.Fatal("期間不見了")
	}
}

// 真的是 0 就是 0：成立訂單、粉絲與瀏覽都是 0 的店家，區塊有設、數字是 0。
func TestDashboardReportsARealZeroAsZero(t *testing.T) {
	src := &dashSources{sales: []SalesStats{{}, {}}, followers: buyer.SubscriberStats{New: []int64{0, 0}}}
	d := getDashboard(t, dashboardHandler(t, src, clockAtTaipeiDayBoundary))
	if d.Sales == nil || d.Followers == nil || d.ProductViews == nil || d.OrderStatusCounts == nil {
		t.Fatalf("零是有效值，區塊必須設：%v", d)
	}
	if d.Sales.GetRevenue().GetCurrent() != 0 || d.Followers.GetTotal() != 0 || d.ProductViews.GetCurrent() != 0 {
		t.Fatalf("%v", d)
	}
}

// int32 的邊界：剛好放得進的值照樣回傳。
func TestDashboardAcceptsTheLargestInt32(t *testing.T) {
	src := healthy()
	src.viewsCurrent = math.MaxInt32
	d := getDashboard(t, dashboardHandler(t, src, clockAtTaipeiDayBoundary))
	if d.GetProductViews().GetCurrent() != math.MaxInt32 {
		t.Fatalf("%v", d.GetProductViews())
	}
}

// handler 沒有宣告能力時 ctx 裡沒有店家，回沒有分類的錯誤，不去讀任何來源。
func TestGetMyDashboardWithoutAStoreInTheContextIsAnInternalError(t *testing.T) {
	src := healthy()
	_, err := dashboardHandler(t, src, clockAtTaipeiDayBoundary).GetMyDashboard(context.Background(), connect.NewRequest(&pb.GetMyDashboardRequest{}))
	if err == nil || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("err=%v", err)
	}
	if src.salesStore != 0 || len(src.viewRanges) != 0 || src.followerCalls != 0 {
		t.Fatal("沒有店家卻讀了來源")
	}
}

// 儀表板要 CapMarketing（行銷經營）：停權與等待刪除的店家沒有這個能力，營運中的有。停權的拒絕帶 SELLER_SUSPENDED 由
// seller_integration_test 的 TestSuspendedSellerCanReadAndCloseButNotWrite 經完整的 interceptor 驗證。
func TestDashboardNeedsMarketingWhichSuspendedAndPendingDeletionStoresLack(t *testing.T) {
	d, ok := declarations[sellerv1alpha1connect.SellerAppServiceGetMyDashboardProcedure]
	if !ok || d.capability != model.CapMarketing {
		t.Fatalf("GetMyDashboard 的宣告：%+v ok=%v", d, ok)
	}
	for name, tc := range map[string]struct {
		status    model.Status
		suspended bool
		want      bool
	}{
		"營運中":  {model.Active, false, true},
		"停權":   {model.Active, true, false},
		"等待刪除": {model.PendingDeletion, false, false},
		"已關閉":  {model.Closed, false, false},
	} {
		store := model.Store{Status: tc.status, Sanctions: sanctionsOf(tc.suspended), Conditions: model.Conditions{
			Derived: true, Active: tc.status != model.Closed, NotPendingDeletion: tc.status != model.PendingDeletion, KYCApproved: true,
			StoreName: true, Logo: true, TownCode: true,
		}}
		if got := slices.Contains(model.ActiveCapabilities(model.CapabilitiesOf(store)), d.capability); got != tc.want {
			t.Errorf("%s：有 %s = %v，應為 %v", name, d.capability, got, tc.want)
		}
	}
}
