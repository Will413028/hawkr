//go:build integration

package crm_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// seedKeysetStore 在店 A 建 n 位客戶：買家 id 從 base 起；total_spent 只有三個值（同值很多）、order_count 只有兩個值、first_order_at 依建立順序遞增。
func (f *fixture) seedKeysetStore(base int64, n int) {
	f.t.Helper()
	for i := 0; i < n; i++ {
		buyer := base + int64(i)
		f.mustOrder(fmt.Sprintf("evt-%d-a", buyer), fmt.Sprintf("K%d-a", buyer), storeA, buyer, int64(100*(i%3+1)))
		if i%2 == 0 {
			f.mustOrder(fmt.Sprintf("evt-%d-b", buyer), fmt.Sprintf("K%d-b", buyer), storeA, buyer, 0)
		}
	}
}

func (f *fixture) allCustomers(in crm.CustomerListInput) []crm.Customer {
	f.t.Helper()
	in.StoreID = storeA
	in.Size = 4
	var out []crm.Customer
	for pages := 0; ; pages++ {
		if pages > 100 {
			f.t.Fatal("分頁沒有結束")
		}
		page, err := f.m.ListCustomers(context.Background(), in)
		if err != nil {
			f.t.Fatal(err)
		}
		if len(page.Customers) > 4 {
			f.t.Fatalf("一頁 %d 筆，超過要求的 4", len(page.Customers))
		}
		out = append(out, page.Customers...)
		if page.Next == nil {
			return out
		}
		in.After = page.Next
	}
}

// CD9：同值的列跨頁無重複無遺漏（排序鍵加 buyer_id 次序鍵），三個排序欄位 × 兩個方向。
func TestKeysetPaginationWithTies(t *testing.T) {
	t.Parallel()
	f := setup(t)
	const n = 25
	f.seedKeysetStore(buyer1, n)
	for _, sort := range []crm.SortBy{crm.SortTotalSpent, crm.SortOrderCount, crm.SortFirstOrderAt} {
		for _, desc := range []bool{true, false} {
			t.Run(fmt.Sprintf("sort=%d desc=%v", sort, desc), func(t *testing.T) {
				got := f.allCustomers(crm.CustomerListInput{Sort: sort, Desc: desc})
				if len(got) != n {
					t.Fatalf("共 %d 筆，預期 %d", len(got), n)
				}
				seen := map[int64]bool{}
				for _, c := range got {
					if seen[c.BuyerID] {
						t.Fatalf("買家 %d 重複出現", c.BuyerID)
					}
					seen[c.BuyerID] = true
				}
				key := func(c crm.Customer) [2]int64 {
					switch sort {
					case crm.SortTotalSpent:
						return [2]int64{c.TotalSpent, c.BuyerID}
					case crm.SortOrderCount:
						return [2]int64{int64(c.OrderCount), c.BuyerID}
					case crm.SortFirstOrderAt:
						return [2]int64{c.FirstOrderAt.UnixNano(), c.BuyerID}
					}
					panic("unknown sort")
				}
				ok := slices.IsSortedFunc(got, func(a, b crm.Customer) int {
					ka, kb := key(a), key(b)
					c := 0
					switch {
					case ka[0] != kb[0]:
						c = cmpInt(ka[0], kb[0])
					default:
						c = cmpInt(ka[1], kb[1])
					}
					if desc {
						return -c
					}
					return c
				})
				if !ok {
					t.Fatalf("排序不是 (排序鍵, buyer_id) %v 序", map[bool]string{true: "遞減", false: "遞增"}[desc])
				}
			})
		}
	}
}

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func TestListCustomersFilters(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.seedKeysetStore(buyer1, 10)
	for _, b := range []int64{buyer1, buyer1 + 3, buyer1 + 7} {
		if err := f.m.SetCustomerType(ctx, storeA, b, crm.TypeVIP); err != nil {
			t.Fatal(err)
		}
	}
	f.mustAddTags(storeA, buyer1+3, "常客")
	f.mustAddTags(storeA, buyer1+4, "常客", "團購")
	tags, _ := f.m.ListTags(ctx, storeA)
	var regular crm.Tag
	for _, tg := range tags {
		if tg.Name == "常客" {
			regular = tg
		}
	}
	vip := crm.TypeVIP
	got := f.allCustomers(crm.CustomerListInput{Type: &vip, Sort: crm.SortFirstOrderAt})
	if ids := buyerIDs(got); !slices.Equal(ids, []int64{buyer1, buyer1 + 3, buyer1 + 7}) {
		t.Fatalf("VIP 篩選 %v", ids)
	}
	got = f.allCustomers(crm.CustomerListInput{TagID: &regular.ID, Sort: crm.SortFirstOrderAt})
	if ids := buyerIDs(got); !slices.Equal(ids, []int64{buyer1 + 3, buyer1 + 4}) {
		t.Fatalf("標籤篩選 %v", ids)
	}
	got = f.allCustomers(crm.CustomerListInput{TagID: &regular.ID, Type: &vip, Sort: crm.SortFirstOrderAt})
	if ids := buyerIDs(got); !slices.Equal(ids, []int64{buyer1 + 3}) {
		t.Fatalf("標籤加 VIP 篩選 %v", ids)
	}
	// 列表帶標籤（名稱升序），沒有標籤的客戶 Tags 是空 slice 不是 nil。
	all := f.allCustomers(crm.CustomerListInput{Sort: crm.SortFirstOrderAt})
	for _, c := range all {
		if c.Tags == nil {
			t.Fatalf("買家 %d 的 Tags 是 nil", c.BuyerID)
		}
		if c.BuyerID == buyer1+4 && (len(c.Tags) != 2 || c.Tags[0].Name != "團購" && c.Tags[0].Name != "常客") {
			t.Fatalf("買家 %d 的標籤 %+v", c.BuyerID, c.Tags)
		}
	}
	if _, err := f.m.ListCustomers(ctx, crm.CustomerListInput{StoreID: storeA, Type: ptr(crm.CustomerType("BLOCKLIST"))}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("未知分類：err=%v，預期 InvalidInput", err)
	}
	// 別家店看不到。
	other, err := f.m.ListCustomers(ctx, crm.CustomerListInput{StoreID: storeB})
	if err != nil || len(other.Customers) != 0 || other.Next != nil {
		t.Fatalf("店 B：%+v err=%v", other, err)
	}
}

func ptr[T any](v T) *T { return &v }

func buyerIDs(cs []crm.Customer) []int64 {
	out := make([]int64, len(cs))
	for i, c := range cs {
		out[i] = c.BuyerID
	}
	slices.Sort(out)
	return out
}

// CD2：SetCustomerType 對非客戶回 NotFound（0 列），值域外的字串被拒絕且不改任何東西。
func TestSetCustomerType(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	wantKind(t, f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeVIP), apperr.KindNotFound, crm.ReasonCustomerNotFound)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	for _, bad := range []crm.CustomerType{"BLOCKLIST", "blocklist", "", "vip", "ADMIN"} {
		if err := f.m.SetCustomerType(ctx, storeA, buyer1, bad); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%q：err=%v，預期 InvalidInput", bad, err)
		}
	}
	// DB 的 CHECK 是第二道防線：繞過 Go 直接寫值域外的字串也被拒絕。
	if _, err := f.db.Pool().Exec(ctx, "UPDATE crm.customers SET customer_type = 'BLOCKLIST'"); err == nil {
		t.Fatal("DB 接受了 BLOCKLIST")
	}
	if err := f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeVIP); err != nil {
		t.Fatal(err)
	}
	c, err := f.m.GetCustomer(ctx, storeA, buyer1)
	if err != nil || c.Type != crm.TypeVIP {
		t.Fatalf("customer=%+v err=%v", c, err)
	}
	// 其他店的同一位買家不受影響（店家對買家的分類只屬於那家店）。
	wantKind(t, f.m.SetCustomerType(ctx, storeB, buyer1, crm.TypeVIP), apperr.KindNotFound, crm.ReasonCustomerNotFound)
}

func TestGetCustomer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, err := f.m.GetCustomer(ctx, storeA, buyer1)
	wantKind(t, err, apperr.KindNotFound, crm.ReasonCustomerNotFound)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 800)
	f.mustOrder("evt-2", "O2", storeA, buyer1, 200)
	f.mustAddTags(storeA, buyer1, "乙", "甲")
	c, err := f.m.GetCustomer(ctx, storeA, buyer1)
	if err != nil {
		t.Fatal(err)
	}
	if c.StoreID != storeA || c.BuyerID != buyer1 || c.Type != crm.TypeNormal || c.TotalSpent != 1000 || c.OrderCount != 2 || c.FirstOrderAt.IsZero() {
		t.Fatalf("customer = %+v", c)
	}
	if len(c.Tags) != 2 || c.Tags[0].Name != "乙" && c.Tags[0].Name != "甲" {
		t.Fatalf("tags = %+v", c.Tags)
	}
	names := []string{c.Tags[0].Name, c.Tags[1].Name}
	if !slices.IsSorted(names) {
		t.Fatalf("標籤不是名稱升序：%v", names)
	}
}

// VIPStatuses：混合有無客戶列的店；每個請求的店家都有一筆。
func TestVIPStatuses(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	f.mustOrder("evt-2", "O2", storeB, buyer1, 100)
	f.mustOrder("evt-3", "O3", storeA, buyer2, 100)
	if err := f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeVIP); err != nil {
		t.Fatal(err)
	}
	if err := f.m.SetCustomerType(ctx, storeA, buyer2, crm.TypeVIP); err != nil {
		t.Fatal(err)
	}
	got, err := f.m.VIPStatuses(ctx, buyer1, []int64{storeA, storeB, storeC, storeA})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]bool{storeA: true, storeB: false, storeC: false}
	if len(got) != len(want) || got[storeA] != true || got[storeB] != false || got[storeC] != false {
		t.Fatalf("got %v，預期 %v", got, want)
	}
	if got, err := f.m.VIPStatuses(ctx, buyer1, nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("空清單：%v %v，預期空 map", got, err)
	}
	var tooMany []int64
	for i := int64(1); i <= crm.MaxStoreIDs+1; i++ {
		tooMany = append(tooMany, i)
	}
	if _, err := f.m.VIPStatuses(ctx, buyer1, tooMany); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("超過上限：err=%v", err)
	}
	if _, err := f.m.VIPStatuses(ctx, buyer1, []int64{storeA, 0}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("非正整數：err=%v", err)
	}
}
