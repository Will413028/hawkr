package buyer

import (
	"context"
	"fmt"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"sort"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeOrders struct {
	summaries   []MerchantSummary
	records     map[int64][]SpendingRecord
	calls       int
	changeTotal bool
}

func (f *fakeOrders) MerchantSummary(context.Context, int64) ([]MerchantSummary, error) {
	return append([]MerchantSummary(nil), f.summaries...), nil
}

func (f *fakeOrders) SpendingRecords(_ context.Context, _, storeID int64, page, size int) (SpendingPage, error) {
	f.calls++
	all := f.records[storeID]
	start := (page - 1) * size
	if start > len(all) {
		start = len(all)
	}
	end := start + size
	if end > len(all) {
		end = len(all)
	}
	total := int64(len(all))
	if f.changeTotal && page > 1 {
		total++
	}
	return SpendingPage{Records: all[start:end], Total: total}, nil
}

type fakeCRM struct{}

func (fakeCRM) VIPStatuses(context.Context, int64, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

func membershipFixture(t *testing.T, orders *fakeOrders, stores []seller.StoreCard) (*App, context.Context) {
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	return mustApp(t, fakeModule{profile: model.Profile{BuyerID: 7, AccountID: 1}}, rpc.Images{}, Ports{Orders: orders, Stores: fakeStores{cards: stores}, CRM: fakeCRM{}}, testStats(&countedRatings{})), ctx
}

func TestMemberCardsSpentOrderAndEqualTotalsPaging(t *testing.T) {
	orders := &fakeOrders{summaries: []MerchantSummary{{StoreID: 1, Spent: 30}, {StoreID: 3, Spent: 10}, {StoreID: 2, Spent: 50, Refunded: 20}, {StoreID: 4, Spent: 40, Refunded: 10}}}
	h, ctx := membershipFixture(t, orders, []seller.StoreCard{openStoreCard(1, "a", ""), openStoreCard(2, "b", ""), openStoreCard(3, "c", ""), openStoreCard(4, "d", "")})
	want := []string{"4", "2", "1", "3"}
	var got []string
	var token string
	for range want {
		resp, err := h.ListMemberCards(ctx, connect.NewRequest(&pb.ListMemberCardsRequest{PageSize: 1, PageToken: token}))
		if err != nil || len(resp.Msg.GetMemberCards()) != 1 {
			t.Fatalf("page=%v err=%v", resp, err)
		}
		got = append(got, resp.Msg.GetMemberCards()[0].GetStore().GetId())
		token = resp.Msg.GetNextPageToken()
	}
	if token != "" || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got=%v token=%q want=%v", got, token, want)
	}
}

func TestSpendingRecordsWalkAndKeyset(t *testing.T) {
	base := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	all := make([]SpendingRecord, 201)
	for i := range all {
		all[i] = SpendingRecord{OrderNo: fmt.Sprintf("O%03d", i), OrderTime: base.Add(time.Duration(i%3) * time.Hour), Amount: 10, Refunded: 2, CouponApplied: i%2 == 0}
	}
	orders := &fakeOrders{records: map[int64][]SpendingRecord{3: all, 4: all[:1]}}
	h, ctx := membershipFixture(t, orders, nil)
	var token string
	var got []string
	for range 3 {
		resp, err := h.ListSpendingRecords(ctx, connect.NewRequest(&pb.ListSpendingRecordsRequest{StoreId: "3", PageSize: 100, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		if resp.Msg.GetTotalSpent() != 2010 || resp.Msg.GetTotalRefunded() != 402 {
			t.Fatalf("totals=%d/%d", resp.Msg.GetTotalSpent(), resp.Msg.GetTotalRefunded())
		}
		for _, row := range resp.Msg.GetRecords() {
			got = append(got, row.GetOrderNumber())
		}
		token = resp.Msg.GetNextPageToken()
	}
	if orders.calls != 9 || token != "" || len(got) != 201 {
		t.Fatalf("calls=%d count=%d token=%q", orders.calls, len(got), token)
	}
	seen := map[string]bool{}
	for _, id := range got {
		if seen[id] {
			t.Fatalf("repeated %s", id)
		}
		seen[id] = true
	}
	want := append([]SpendingRecord(nil), all...)
	sort.Slice(want, func(i, j int) bool { return recordLess(want[i], want[j]) })
	for i, row := range want {
		if got[i] != row.OrderNo {
			t.Fatalf("order at %d=%q want=%q", i, got[i], row.OrderNo)
		}
	}
	first, err := h.ListSpendingRecords(ctx, connect.NewRequest(&pb.ListSpendingRecordsRequest{StoreId: "3", PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.ListSpendingRecords(ctx, connect.NewRequest(&pb.ListSpendingRecordsRequest{StoreId: "4", PageToken: first.Msg.GetNextPageToken()}))
	if err == nil || !strings.Contains(err.Error(), "page_token") {
		t.Fatalf("cross-store token: %v", err)
	}
}

func TestSpendingRecordsRejectsChangedTotal(t *testing.T) {
	all := make([]SpendingRecord, 101)
	for i := range all {
		all[i] = SpendingRecord{OrderNo: fmt.Sprintf("O%d", i), OrderTime: time.Now(), Amount: 1}
	}
	h, ctx := membershipFixture(t, &fakeOrders{records: map[int64][]SpendingRecord{3: all}, changeTotal: true}, nil)
	_, err := h.ListSpendingRecords(ctx, connect.NewRequest(&pb.ListSpendingRecordsRequest{StoreId: "3"}))
	if apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("changed total: %v", err)
	}
}
