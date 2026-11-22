//go:build integration

package crm_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 賣家能力由模組檢查：handler 只推導店家身分，所以在 handler 檢查之後才發生的停權，寫入與讀取仍然被擋（SELLER_SUSPENDED），
// 被擋的寫入沒有任何副作用；停權解除後恢復。VIPStatuses 是買家端的讀取，不要求店家能力。
func TestSuspendedStoreIsRejectedByTheModuleForWritesAndReads(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	f.mustAddTags(storeA, buyer1, "舊")
	tags, err := f.m.ListTags(ctx, storeA)
	if err != nil || len(tags) != 1 {
		t.Fatalf("tags=%v err=%v", tags, err)
	}

	f.gate.suspended.Store(true)
	wantDenied := func(label string, err error) {
		t.Helper()
		if apperr.KindOf(err) != apperr.KindPermissionDenied {
			t.Errorf("%s：%v，預期 permission_denied", label, err)
		}
	}
	wantDenied("SetCustomerType", errOnly(f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeVIP)))
	wantDenied("AddCustomerTags", errOnly(f.m.AddCustomerTags(ctx, storeA, buyer1, []string{"新"})))
	wantDenied("RemoveCustomerTag", errOnly(f.m.RemoveCustomerTag(ctx, storeA, buyer1, tags[0].ID)))
	wantDenied("CreateTag", errOnly(f.m.CreateTag(ctx, storeA, "另一個")))
	wantDenied("RenameTag", errOnly(f.m.RenameTag(ctx, storeA, tags[0].ID, "改")))
	wantDenied("DeleteTag", f.m.DeleteTag(ctx, storeA, tags[0].ID))
	wantDenied("ListTags", errOnly(f.m.ListTags(ctx, storeA)))
	wantDenied("GetCustomer", errOnly(f.m.GetCustomer(ctx, storeA, buyer1)))
	wantDenied("ListCustomers", errOnly(f.m.ListCustomers(ctx, crm.CustomerListInput{StoreID: storeA, Sort: crm.SortTotalSpent})))
	if got := f.tagNames(storeA); len(got) != 1 || got[0] != "舊" {
		t.Errorf("被擋的寫入改了標籤：%v", got)
	}
	if n := f.scalar("SELECT count(*) FROM crm.customers WHERE store_id = $1 AND customer_type = 'VIP'", storeA); n != 0 {
		t.Error("被擋的寫入改了分類")
	}
	if got, err := f.m.VIPStatuses(ctx, buyer1, []int64{storeA}); err != nil || got[storeA] {
		t.Errorf("買家端的讀取不受店家停權影響：%v %v", got, err)
	}

	f.gate.suspended.Store(false)
	c, err := f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeVIP)
	if err != nil || c.Type != crm.TypeVIP || c.BuyerID != buyer1 || len(c.Tags) != 1 {
		t.Fatalf("解除停權後的寫入：%+v %v", c, err)
	}
}

// 寫入回傳寫入當下的列（同 transaction），不是 commit 之後另外讀。
func TestWritesReturnTheWrittenRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 300)
	c, err := f.m.AddCustomerTags(ctx, storeA, buyer1, []string{"乙", "甲"})
	if err != nil || c.TotalSpent != 300 || c.OrderCount != 1 || len(c.Tags) != 2 || c.Tags[0].Name != "乙" && c.Tags[0].Name != "甲" {
		t.Fatalf("AddCustomerTags：%+v %v", c, err)
	}
	c, err = f.m.RemoveCustomerTag(ctx, storeA, buyer1, c.Tags[0].ID)
	if err != nil || len(c.Tags) != 1 {
		t.Fatalf("RemoveCustomerTag：%+v %v", c, err)
	}
	tag, err := f.m.RenameTag(ctx, storeA, c.Tags[0].ID, " 丙 ")
	if err != nil || tag.ID != c.Tags[0].ID || tag.Name != "丙" {
		t.Fatalf("RenameTag：%+v %v", tag, err)
	}
	c, err = f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeVIP)
	if err != nil || c.Type != crm.TypeVIP || len(c.Tags) != 1 || c.Tags[0].Name != "丙" {
		t.Fatalf("SetCustomerType：%+v %v", c, err)
	}
}
