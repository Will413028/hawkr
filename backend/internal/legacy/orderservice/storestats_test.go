package orderservice

import (
	"context"
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
)

// fakeStats 依序回傳 pages，並記下每次請求。
type fakeStats struct {
	orderconnect.OrderServiceClient
	pages []*orderpb.ListStoreOrderStatsResponse
	err   error
	seen  []*orderpb.ListStoreOrderStatsRequest
}

func (f *fakeStats) ListStoreOrderStats(ctx context.Context, req *connect.Request[orderpb.ListStoreOrderStatsRequest]) (*connect.Response[orderpb.ListStoreOrderStatsResponse], error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("missing deadline")
	}
	f.seen = append(f.seen, req.Msg)
	if f.err != nil {
		return nil, f.err
	}
	i := len(f.seen) - 1
	if i >= len(f.pages) {
		i = len(f.pages) - 1 // 最後一頁重複回，用來測保險
	}
	return connect.NewResponse(f.pages[i]), nil
}

func stat(id string, buyers, qualifying, cancelled int32) *orderpb.StoreOrderStats {
	return &orderpb.StoreOrderStats{StoreId: id, DistinctBuyers: buyers, QualifyingOrders: qualifying, SellerCancelledOrders: cancelled}
}

func TestStoreOrderStatsWalksAllPages(t *testing.T) {
	f := &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{
		{Stats: []*orderpb.StoreOrderStats{stat("10", 2, 3, 1), stat("20", 0, 0, 2)}, NextAfterStoreId: "20"},
		{Stats: []*orderpb.StoreOrderStats{stat("9", 1, 1, 0)}},
	}}
	got, err := (&Client{rpc: f}).StoreOrderStats(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	want := []StoreOrderStats{{10, 2, 3, 1}, {20, 0, 0, 2}, {9, 1, 1, 0}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if len(f.seen) != 2 || f.seen[0].GetAfterStoreId() != "" || f.seen[1].GetAfterStoreId() != "20" || f.seen[0].GetDays() != 30 || f.seen[0].GetPageSize() != 1000 {
		t.Fatalf("requests=%v", f.seen)
	}
}

func TestStoreOrderStatsEmpty(t *testing.T) {
	got, err := (&Client{rpc: &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{{}}}}).StoreOrderStats(context.Background(), 7)
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestStoreOrderStatsRejectsBadInput(t *testing.T) {
	page := func(s ...*orderpb.StoreOrderStats) *orderpb.ListStoreOrderStatsResponse {
		return &orderpb.ListStoreOrderStatsResponse{Stats: s}
	}
	for _, tc := range []struct {
		name string
		days int
		f    *fakeStats
	}{
		{"days zero", 0, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page()}}},
		{"days too large", 91, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page()}}},
		{"transport error", 30, &fakeStats{err: errors.New("down")}},
		{"business code", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{{Code: 400}}}},
		{"bad store id", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page(stat("x", 1, 1, 0))}}},
		{"zero store id", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page(stat("0", 1, 1, 0))}}},
		{"negative count", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page(stat("1", 1, -1, 0))}}},
		{"buyers exceed orders", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page(stat("1", 3, 2, 0))}}},
		{"orders without buyers", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page(stat("1", 0, 2, 0))}}},
		{"empty store row", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{page(stat("1", 0, 0, 0))}}},
		{"cursor without rows", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{{NextAfterStoreId: "5"}}}},
		{"cursor not last row", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{{Stats: []*orderpb.StoreOrderStats{stat("1", 1, 1, 0)}, NextAfterStoreId: "7"}}}},
		{"duplicate store across pages", 30, &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{
			{Stats: []*orderpb.StoreOrderStats{stat("1", 1, 1, 0)}, NextAfterStoreId: "1"},
			{Stats: []*orderpb.StoreOrderStats{stat("1", 1, 1, 0)}},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := (&Client{rpc: tc.f}).StoreOrderStats(context.Background(), tc.days); err == nil {
				t.Fatalf("accepted: %+v", got)
			}
		})
	}
}

// 舊服務游標不前進時，不會無限翻頁：重複的店先擋下。
func TestStoreOrderStatsStuckCursorTerminates(t *testing.T) {
	f := &fakeStats{pages: []*orderpb.ListStoreOrderStatsResponse{
		{Stats: []*orderpb.StoreOrderStats{stat("1", 1, 1, 0)}, NextAfterStoreId: "1"},
	}}
	if _, err := (&Client{rpc: f}).StoreOrderStats(context.Background(), 30); err == nil {
		t.Fatal("stuck cursor accepted")
	}
	if len(f.seen) > 3 {
		t.Fatalf("requests=%d", len(f.seen))
	}
}
