package orderservice

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
)

type fakePeriod struct {
	orderconnect.OrderServiceClient
	resp   *orderpb.GetStorePeriodOrderStatsResponse
	counts *orderpb.GetSellerOrderStatusCountsResponse
	err    error
	seen   []*orderpb.GetStorePeriodOrderStatsRequest
	seenSC []*orderpb.GetSellerOrderStatusCountsRequest
}

func (f *fakePeriod) GetStorePeriodOrderStats(ctx context.Context, req *connect.Request[orderpb.GetStorePeriodOrderStatsRequest]) (*connect.Response[orderpb.GetStorePeriodOrderStatsResponse], error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("missing deadline")
	}
	f.seen = append(f.seen, req.Msg)
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func (f *fakePeriod) GetSellerOrderStatusCounts(ctx context.Context, req *connect.Request[orderpb.GetSellerOrderStatusCountsRequest]) (*connect.Response[orderpb.GetSellerOrderStatusCountsResponse], error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("missing deadline")
	}
	f.seenSC = append(f.seenSC, req.Msg)
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.counts), nil
}

func periodRow(orders, buyers, revenue string) *orderpb.StorePeriodOrderStats {
	return &orderpb.StorePeriodOrderStats{QualifyingOrders: orders, DistinctBuyers: buyers, Revenue: revenue}
}

var (
	day0 = time.Date(2026, 9, 27, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	win1 = Window{Start: day0, End: day0.AddDate(0, 0, 7)}
	win0 = Window{Start: day0.AddDate(0, 0, -7), End: day0}
)

func TestStorePeriodOrderStatsSendsWindowsAsUnixMillisAndKeepsOrder(t *testing.T) {
	f := &fakePeriod{resp: &orderpb.GetStorePeriodOrderStatsResponse{Windows: []*orderpb.StorePeriodOrderStats{periodRow("3", "2", "5000"), periodRow("0", "0", "0")}}}
	got, err := (&Client{rpc: f}).StorePeriodOrderStats(context.Background(), 42, []Window{win1, win0})
	if err != nil {
		t.Fatal(err)
	}
	want := []PeriodOrderStats{{QualifyingOrders: 3, DistinctBuyers: 2, Revenue: 5000}, {}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	req := f.seen[0]
	if req.GetStoreId() != 42 || len(req.GetWindows()) != 2 ||
		req.GetWindows()[0].GetStartTime() != "1790438400000" || req.GetWindows()[0].GetEndTime() != "1791043200000" ||
		req.GetWindows()[1].GetStartTime() != "1789833600000" || req.GetWindows()[1].GetEndTime() != "1790438400000" {
		t.Fatalf("request=%v", req)
	}
}

func TestStorePeriodOrderStatsRejectsBadInputAndResponses(t *testing.T) {
	ok := func(rows ...*orderpb.StorePeriodOrderStats) *fakePeriod {
		return &fakePeriod{resp: &orderpb.GetStorePeriodOrderStatsResponse{Windows: rows}}
	}
	subMilli := Window{Start: day0.Add(time.Microsecond), End: day0.AddDate(0, 0, 1)}
	tooLong := Window{Start: day0, End: day0.Add(90*24*time.Hour + time.Millisecond)}
	for _, tc := range []struct {
		name    string
		storeID int64
		windows []Window
		f       *fakePeriod
	}{
		{"store zero", 0, []Window{win1}, ok(periodRow("0", "0", "0"))},
		{"no windows", 1, nil, ok()},
		{"five windows", 1, []Window{win1, win1, win1, win1, win1}, ok()},
		{"empty window", 1, []Window{{Start: day0, End: day0}}, ok(periodRow("0", "0", "0"))},
		{"reversed window", 1, []Window{{Start: day0.AddDate(0, 0, 1), End: day0}}, ok(periodRow("0", "0", "0"))},
		{"window longer than 90 days", 1, []Window{tooLong}, ok(periodRow("0", "0", "0"))},
		{"sub-millisecond start", 1, []Window{subMilli}, ok(periodRow("0", "0", "0"))},
		{"before the epoch", 1, []Window{{Start: time.UnixMilli(-5), End: time.UnixMilli(5)}}, ok(periodRow("0", "0", "0"))},
		{"transport error", 1, []Window{win1}, &fakePeriod{err: errors.New("down")}},
		{"business code", 1, []Window{win1}, &fakePeriod{resp: &orderpb.GetStorePeriodOrderStatsResponse{Code: 400}}},
		{"too few rows", 1, []Window{win1, win0}, ok(periodRow("0", "0", "0"))},
		{"too many rows", 1, []Window{win1}, ok(periodRow("0", "0", "0"), periodRow("0", "0", "0"))},
		{"empty string count", 1, []Window{win1}, ok(periodRow("", "0", "0"))},
		{"negative revenue", 1, []Window{win1}, ok(periodRow("1", "1", "-5"))},
		{"non-numeric buyers", 1, []Window{win1}, ok(periodRow("1", "x", "5"))},
		{"buyers exceed orders", 1, []Window{win1}, ok(periodRow("1", "2", "5"))},
		{"orders without buyers", 1, []Window{win1}, ok(periodRow("2", "0", "5"))},
		{"revenue without orders", 1, []Window{win1}, ok(periodRow("0", "0", "5"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := (&Client{rpc: tc.f}).StorePeriodOrderStats(context.Background(), tc.storeID, tc.windows); err == nil {
				t.Fatalf("accepted: %+v", got)
			}
		})
	}
}

func TestSellerOrderStatusCounts(t *testing.T) {
	f := &fakePeriod{counts: &orderpb.GetSellerOrderStatusCountsResponse{PendingShipmentCount: 3, InProgressCount: 4, CompletedCount: 5, AftersalesCount: 6}}
	got, err := (&Client{rpc: f}).SellerOrderStatusCounts(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if want := (SellerOrderStatusCounts{PendingShipment: 3, InProgress: 4, Closed: 5, Aftersales: 6}); got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if len(f.seenSC) != 1 || f.seenSC[0].GetStoreId() != 42 {
		t.Fatalf("requests=%v", f.seenSC)
	}
}

func TestSellerOrderStatusCountsRejectsBadInputAndResponses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		storeID int64
		f       *fakePeriod
	}{
		{"store zero", 0, &fakePeriod{counts: &orderpb.GetSellerOrderStatusCountsResponse{}}},
		{"transport error", 1, &fakePeriod{err: errors.New("down")}},
		{"business code", 1, &fakePeriod{counts: &orderpb.GetSellerOrderStatusCountsResponse{Code: 500}}},
		{"negative pending", 1, &fakePeriod{counts: &orderpb.GetSellerOrderStatusCountsResponse{PendingShipmentCount: -1}}},
		{"negative in progress", 1, &fakePeriod{counts: &orderpb.GetSellerOrderStatusCountsResponse{InProgressCount: -1}}},
		{"negative closed", 1, &fakePeriod{counts: &orderpb.GetSellerOrderStatusCountsResponse{CompletedCount: -1}}},
		{"negative aftersales", 1, &fakePeriod{counts: &orderpb.GetSellerOrderStatusCountsResponse{AftersalesCount: -1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := (&Client{rpc: tc.f}).SellerOrderStatusCounts(context.Background(), tc.storeID); err == nil {
				t.Fatalf("accepted: %+v", got)
			}
		})
	}
}
