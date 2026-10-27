package orderservice

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
)

type fakeOrders struct {
	orderconnect.OrderServiceClient
	response *orderpb.GetMyOrderStatusCountsResponse
	summary  *orderpb.GetBuyerMerchantSummaryResponse
	spending *orderpb.GetBuyerOrdersForSellerResponse
	err      error

	// 店家端：orders、counts 是回應，ordersErr、countsErr 是傳輸錯誤，seen 記下收到的請求。
	orders    *orderpb.GetSellerOrdersResponse
	counts    *orderpb.GetSellerOrderStatusCountsResponse
	ordersErr error
	countsErr error
	seen      *sellerCalls
}

type sellerCalls struct {
	orders []*orderpb.GetSellerOrdersRequest
	counts int
}

func (f fakeOrders) GetSellerOrders(_ context.Context, req *connect.Request[orderpb.GetSellerOrdersRequest]) (*connect.Response[orderpb.GetSellerOrdersResponse], error) {
	f.seen.orders = append(f.seen.orders, req.Msg)
	if f.ordersErr != nil {
		return nil, f.ordersErr
	}
	return connect.NewResponse(f.orders), nil
}

func (f fakeOrders) GetSellerOrderStatusCounts(context.Context, *connect.Request[orderpb.GetSellerOrderStatusCountsRequest]) (*connect.Response[orderpb.GetSellerOrderStatusCountsResponse], error) {
	f.seen.counts++
	if f.countsErr != nil {
		return nil, f.countsErr
	}
	return connect.NewResponse(f.counts), nil
}

func (f fakeOrders) GetBuyerMerchantSummary(context.Context, *connect.Request[orderpb.GetBuyerMerchantSummaryRequest]) (*connect.Response[orderpb.GetBuyerMerchantSummaryResponse], error) {
	return connect.NewResponse(f.summary), nil
}

func (f fakeOrders) GetBuyerOrdersForSeller(context.Context, *connect.Request[orderpb.GetBuyerOrdersForSellerRequest]) (*connect.Response[orderpb.GetBuyerOrdersForSellerResponse], error) {
	return connect.NewResponse(f.spending), nil
}

func (f fakeOrders) GetMyOrderStatusCounts(context.Context, *connect.Request[orderpb.GetMyOrderStatusCountsRequest]) (*connect.Response[orderpb.GetMyOrderStatusCountsResponse], error) {
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.response), nil
}

func TestHasOpenOrders(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    string
		count     string
		want      bool
		wantError bool
	}{
		{"pending payment", "0", "1", true, false},
		{"delivered", "40", "1", true, false},
		{"completed", "50", "1", false, false},
		{"cancelled", "80", "1", false, false},
		{"unknown", "999", "1", false, true},
		{"bad count", "0", "bad", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{rpc: fakeOrders{response: &orderpb.GetMyOrderStatusCountsResponse{Counts: []*orderpb.OrderStatusCount{{Status: tc.status, Count: tc.count}}}}}
			got, err := c.HasOpenOrders(context.Background(), 7)
			if got != tc.want || (err != nil) != tc.wantError {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
	c := &Client{rpc: fakeOrders{err: errors.New("offline")}}
	if _, err := c.HasOpenOrders(context.Background(), 7); err == nil {
		t.Fatal("order error treated as no orders")
	}
	c = &Client{rpc: fakeOrders{response: &orderpb.GetMyOrderStatusCountsResponse{Counts: []*orderpb.OrderStatusCount{{Status: "0", Count: "1"}, {Status: "999", Count: "1"}}}}}
	if _, err := c.HasOpenOrders(context.Background(), 7); err == nil {
		t.Fatal("unknown status after open order accepted")
	}
}

func TestParsedMerchantSummaryAndSpendingRecords(t *testing.T) {
	f := fakeOrders{
		summary:  &orderpb.GetBuyerMerchantSummaryResponse{Data: []*orderpb.BuyerMerchantSummaryItem{{StoreId: "7", TotalSpending: "300", TotalRefund: "20"}}},
		spending: &orderpb.GetBuyerOrdersForSellerResponse{Total: "1", Page: "1", PageSize: "100", Records: []*orderpb.BuyerSellerOrderRecord{{OrderNo: "O1", CreateTime: "1700000000000", ActualAmount: "300", RefundAmount: "20", DiscountAmount: "5"}}},
	}
	c := &Client{rpc: f}
	summary, err := c.MerchantSummary(context.Background(), 4)
	if err != nil || len(summary) != 1 || summary[0] != (MerchantSummary{StoreID: 7, Spent: 300, Refunded: 20}) {
		t.Fatalf("summary=%v err=%v", summary, err)
	}
	page, err := c.SpendingRecords(context.Background(), 4, 7, 1, 100)
	if err != nil || page.Total != 1 || len(page.Records) != 1 || page.Records[0].OrderTime.UnixMilli() != 1700000000000 || !page.Records[0].CouponApplied {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	f.spending.Page = "2"
	if _, err := (&Client{rpc: f}).SpendingRecords(context.Background(), 4, 7, 1, 100); err == nil {
		t.Fatal("page mismatch accepted")
	}
	f.spending.Page = "1"
	f.spending.Records[0].CreateTime = "bad"
	if _, err := (&Client{rpc: f}).SpendingRecords(context.Background(), 4, 7, 1, 100); err == nil {
		t.Fatal("invalid Unix milliseconds accepted")
	}
}

func TestOpenStatusesCoverCodeMetaOrderStatus(t *testing.T) {
	md, err := os.ReadFile("../../../docs/代碼管理（CodeMeta）/業務狀態碼.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(md), "## OrderStatus\n")
	if !ok {
		t.Fatal("CodeMeta〈業務狀態碼〉沒有 OrderStatus")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	values := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\| [A-Z_]+ \| [A-Z_]+ \| (\d+) \|`).FindAllStringSubmatch(section, -1) {
		values[m[1]] = true
	}
	if len(values) == 0 {
		t.Fatal("OrderStatus 解析不到任何值")
	}
	for v := range values {
		if _, ok := openStatuses[v]; !ok {
			t.Errorf("CodeMeta 的 OrderStatus %s 沒有決定算不算未結", v)
		}
	}
	for v := range openStatuses {
		if !values[v] {
			t.Errorf("openStatuses 的 %s 不在 CodeMeta 的 OrderStatus", v)
		}
	}
}

func sellerOrdersResponse(total string, statuses ...string) *orderpb.GetSellerOrdersResponse {
	resp := &orderpb.GetSellerOrdersResponse{Meta: &orderpb.PageMeta{TotalItems: total}}
	for _, status := range statuses {
		resp.Orders = append(resp.Orders, &orderpb.OrderSummaryView{OrderNo: "O1", Status: status})
	}
	return resp
}

func TestHasOpenStoreOrders(t *testing.T) {
	for _, tc := range []struct {
		name         string
		orders       *orderpb.GetSellerOrdersResponse
		counts       *orderpb.GetSellerOrderStatusCountsResponse
		ordersErr    error
		countsErr    error
		want         bool
		wantError    bool
		wantCountsRC int
	}{
		// 0 待付款不在 GetSellerOrderStatusCounts 的四桶裡，只有未結篩選的 GetSellerOrders 看得到。
		{name: "unpaid order blocks", orders: sellerOrdersResponse("1", "0"), want: true},
		{name: "shipped order blocks", orders: sellerOrdersResponse("3", "30"), want: true},
		{name: "delivered not yet confirmed blocks", orders: sellerOrdersResponse("1", "40"), want: true},
		{name: "terminal only: nothing blocks", orders: sellerOrdersResponse("0"), counts: &orderpb.GetSellerOrderStatusCountsResponse{CompletedCount: 9}, wantCountsRC: 1},
		{name: "aftersales on a terminal order blocks", orders: sellerOrdersResponse("0"), counts: &orderpb.GetSellerOrderStatusCountsResponse{AftersalesCount: 2}, want: true, wantCountsRC: 1},
		{name: "unknown status is an error", orders: sellerOrdersResponse("1", "999"), wantError: true},
		{name: "filter ignored: a terminal status in the result is an error", orders: sellerOrdersResponse("5", "50"), wantError: true},
		{name: "bad total is an error", orders: sellerOrdersResponse("bad"), wantError: true},
		{name: "negative total is an error", orders: sellerOrdersResponse("-1"), wantError: true},
		{name: "missing meta is an error", orders: &orderpb.GetSellerOrdersResponse{}, wantError: true},
		{name: "orders code is an error", orders: &orderpb.GetSellerOrdersResponse{Code: 500, Meta: &orderpb.PageMeta{TotalItems: "0"}}, wantError: true},
		{name: "orders transport error is an error", ordersErr: errors.New("offline"), wantError: true},
		{name: "counts transport error is an error", orders: sellerOrdersResponse("0"), countsErr: errors.New("offline"), wantError: true, wantCountsRC: 1},
		{name: "counts code is an error", orders: sellerOrdersResponse("0"), counts: &orderpb.GetSellerOrderStatusCountsResponse{Code: 500}, wantError: true, wantCountsRC: 1},
		{name: "negative aftersales is an error", orders: sellerOrdersResponse("0"), counts: &orderpb.GetSellerOrderStatusCountsResponse{AftersalesCount: -1}, wantError: true, wantCountsRC: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := &sellerCalls{}
			c := &Client{rpc: fakeOrders{orders: tc.orders, counts: tc.counts, ordersErr: tc.ordersErr, countsErr: tc.countsErr, seen: seen}}
			got, err := c.HasOpenStoreOrders(context.Background(), 7)
			if got != tc.want || (err != nil) != tc.wantError {
				t.Fatalf("got=%v err=%v", got, err)
			}
			if tc.wantError && got {
				t.Fatal("an error must not also report open orders")
			}
			if seen.counts != tc.wantCountsRC {
				t.Fatalf("GetSellerOrderStatusCounts calls=%d want %d", seen.counts, tc.wantCountsRC)
			}
			if len(seen.orders) != 1 || seen.orders[0].GetStoreId() != 7 {
				t.Fatalf("requests=%v", seen.orders)
			}
		})
	}
}

// 篩選條件必須含 0（待付款）與每一個未結狀態，且不含任何已結的狀態：它是這個檢查的核心。
func TestStoreOrderFilterIsExactlyTheOpenStatuses(t *testing.T) {
	if openStatusFilter != "0,10,20,30,32,35,40" {
		t.Fatalf("filter=%q", openStatusFilter)
	}
	seen := &sellerCalls{}
	c := &Client{rpc: fakeOrders{orders: sellerOrdersResponse("0"), counts: &orderpb.GetSellerOrderStatusCountsResponse{}, seen: seen}}
	if _, err := c.HasOpenStoreOrders(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	req := seen.orders[0]
	if req.GetStatusFilter() != openStatusFilter || req.GetPage().GetPage() != "1" || req.GetPage().GetSize() != "1" {
		t.Fatalf("request=%v", req)
	}
	if _, err := c.HasOpenStoreOrders(context.Background(), 0); err == nil {
		t.Fatal("invalid store id accepted")
	}
}
