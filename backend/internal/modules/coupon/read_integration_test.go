//go:build integration

package coupon_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
)

func (f *fixture) sellerCampaign(store int64, mod ...func(*coupon.CreateCampaignInput)) coupon.Campaign {
	f.t.Helper()
	f.advance(time.Second)
	return f.campaign(append([]func(*coupon.CreateCampaignInput){func(in *coupon.CreateCampaignInput) {
		in.Issuer = coupon.Issuer{StoreID: store}
	}}, mod...)...)
}

func idsOf(vs []coupon.VoucherView) []uuid.UUID {
	out := make([]uuid.UUID, len(vs))
	for i, v := range vs {
		out[i] = v.Voucher.ID
	}
	return out
}

// 列表在 SQL 內分頁：新到舊、keyset 翻頁不重不漏，Next 為 nil 才是最後一頁（BD12）。
func TestListVouchersPagesNewestFirstWithoutGapsOrRepeats(t *testing.T) {
	f := setup(t)
	var want []uuid.UUID
	for range 5 {
		f.advance(time.Second)
		want = append([]uuid.UUID{f.claim(f.campaign(), buyer1).ID}, want...)
	}
	var got []uuid.UUID
	var after *coupon.Cursor
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("翻頁沒有結束")
		}
		p, err := f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer1, Size: 2, After: after})
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, idsOf(p.Vouchers)...)
		if p.Next == nil {
			break
		}
		after = p.Next
	}
	if !slices.Equal(got, want) {
		t.Fatalf("券的順序 %v，預期 %v", got, want)
	}
}

func TestListVouchersFiltersByStatusAndKeepsExpiredAsAvailable(t *testing.T) {
	f := setup(t)
	c1, c2 := f.campaign(), f.campaign()
	v1, v2 := f.claim(c1, buyer1), f.claim(c2, buyer1)
	f.mustLock(v1, "ORD-1")
	avail, err := f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer1, Statuses: []coupon.VoucherStatus{coupon.VoucherAvailable}})
	if err != nil || len(avail.Vouchers) != 1 || avail.Vouchers[0].Voucher.ID != v2.ID {
		t.Fatalf("AVAILABLE 篩選：%v %+v", err, idsOf(avail.Vouchers))
	}
	f.advance(2 * windowLen)
	all, err := f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer1})
	if err != nil || len(all.Vouchers) != 2 {
		t.Fatalf("全部：%v %d", err, len(all.Vouchers))
	}
	for _, v := range all.Vouchers {
		if !v.Expired {
			t.Errorf("時間窗已過，%s 應標 expired", v.Voucher.ID)
		}
	}
	if other, err := f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer2}); err != nil || len(other.Vouchers) != 0 {
		t.Fatalf("別的買家看到我的券：%v %d", err, len(other.Vouchers))
	}
}

// 結帳情境與鎖券同一組檢查：門檻、店家範圍、停權、到期的券都不出現，並算預估折抵。
func TestListVouchersCheckoutUsesTheLockChecks(t *testing.T) {
	f := setup(t)
	ok := f.claim(f.campaign(func(in *coupon.CreateCampaignInput) { in.ThresholdAmount = 200 }), buyer1)
	f.claim(f.campaign(func(in *coupon.CreateCampaignInput) { in.ThresholdAmount = 5000 }), buyer1)
	f.claim(f.campaign(func(in *coupon.CreateCampaignInput) { in.StoreID = storeB }), buyer1)
	co := &coupon.Checkout{StoreID: storeA, OrderAmount: 1000}
	p, err := f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer1, Checkout: co})
	if err != nil || len(p.Vouchers) != 1 || p.Vouchers[0].Voucher.ID != ok.ID || p.Vouchers[0].EstimatedDiscount != 100 {
		t.Fatalf("結帳列表：err=%v %+v", err, p.Vouchers)
	}
	sold := f.claim(f.sellerCampaign(storeA), buyer2)
	f.stores.set(storeA, true)
	p, err = f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer2, Checkout: co})
	if err != nil || len(p.Vouchers) != 0 {
		t.Fatalf("停權店家的券不該出現在結帳列表（%s）：err=%v %d", sold.ID, err, len(p.Vouchers))
	}
	f.stores.set(storeA, false)
	f.advance(2 * windowLen)
	if p, err = f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer1, Checkout: co}); err != nil || len(p.Vouchers) != 0 {
		t.Fatalf("到期的券不該出現：err=%v %d", err, len(p.Vouchers))
	}
}

func TestListClaimableUsesTheClaimQualification(t *testing.T) {
	f := setup(t)
	open := f.campaign()
	held := f.campaign()
	f.claim(held, buyer1)
	f.campaign(func(in *coupon.CreateCampaignInput) { in.TotalLimit = 1 })
	soldOut := f.campaign(func(in *coupon.CreateCampaignInput) { in.TotalLimit = 1 })
	f.claim(soldOut, buyer2)
	f.campaign(func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer2}
	})
	f.campaign(func(in *coupon.CreateCampaignInput) {
		in.StartAt, in.EndAt = base.Add(-3*windowLen), base.Add(-2*windowLen)
	})
	f.sellerCampaign(storeB)
	f.stores.set(storeB, true)
	p, err := f.m.ListClaimable(context.Background(), coupon.ClaimableInput{BuyerID: buyer1})
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]bool{}
	for _, c := range p.Campaigns {
		got[c.ID] = true
		if _, err := f.m.Claim(context.Background(), c.ID, buyer1); err != nil && c.ID != held.ID {
			t.Errorf("列表裡的活動 %s 領不到：%v（列表與領券要同一個判定）", c.ID, err)
		}
	}
	if !got[open.ID] || got[held.ID] || got[soldOut.ID] || len(p.Campaigns) != 2 {
		t.Fatalf("可領的活動 = %v，預期只有 open 與一個還有名額的活動（停權店家的活動不在內）", got)
	}
	f.advance(2 * windowLen)
	if p, err = f.m.ListClaimable(context.Background(), coupon.ClaimableInput{BuyerID: buyer3}); err != nil || len(p.Campaigns) != 0 {
		t.Fatalf("時間窗過了就不能領：err=%v %d", err, len(p.Campaigns))
	}
}

func TestListClaimableScopesToTheStoreAndFailsWhenAPortFails(t *testing.T) {
	f := setup(t)
	platform := f.campaign()
	mine := f.sellerCampaign(storeA)
	f.sellerCampaign(storeB)
	p, err := f.m.ListClaimable(context.Background(), coupon.ClaimableInput{BuyerID: buyer1, StoreID: storeA})
	if err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{}
	for _, c := range p.Campaigns {
		ids = append(ids, c.ID)
	}
	if len(ids) != 2 || !slices.Contains(ids, platform.ID) || !slices.Contains(ids, mine.ID) {
		t.Fatalf("store_id=A 應是全平台與 A 店的活動：%v", ids)
	}
	f.campaign(func(in *coupon.CreateCampaignInput) { in.TargetType, in.StoreID = coupon.TargetMember, storeA })
	f.members.err = errors.New("projection down")
	if _, err := f.m.ListClaimable(context.Background(), coupon.ClaimableInput{BuyerID: buyer1}); err == nil {
		t.Fatal("MEMBER 資格的 port 失敗要讓整個列表失敗，不能降級成不是會員（BD28）")
	}
}

func TestSellerCampaignsListAndDetailAreScopedToTheStore(t *testing.T) {
	f := setup(t)
	a1 := f.sellerCampaign(storeA, func(in *coupon.CreateCampaignInput) { in.TotalLimit = 3 })
	a2 := f.sellerCampaign(storeA, func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer2, buyer3}
	})
	b1 := f.sellerCampaign(storeB)
	if _, err := f.m.IssueCompensation(context.Background(), coupon.CompensationInput{
		SourceType: coupon.SourceAdminCompensation, SourceID: "c1", BuyerID: buyer1, StoreID: storeA, Title: "補償", Amount: 100, MinOrderAmount: 500,
		EndAt: base.Add(windowLen), Issuer: coupon.Issuer{AdminID: adminID},
	}); err != nil {
		t.Fatalf("補償券：%v", err)
	}
	page, err := f.m.ListSellerCampaigns(context.Background(), coupon.SellerListInput{StoreID: storeA})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Campaigns) != 2 || page.Campaigns[0].Campaign.ID != a2.ID || page.Campaigns[1].Campaign.ID != a1.ID {
		t.Fatalf("A 店的列表（不含 B 店與補償）：%+v", page.Campaigns)
	}
	if page.MaxActive != coupon.DefaultSellerQuota || page.CurrentActive != 2 {
		t.Fatalf("配額 %d、進行中 %d，預期 %d、2", page.MaxActive, page.CurrentActive, coupon.DefaultSellerQuota)
	}
	if page.Campaigns[1].Campaign.TotalLimit == nil || *page.Campaigns[1].Campaign.TotalLimit != 3 || page.Campaigns[0].Campaign.TotalLimit != nil {
		t.Fatalf("上限 3 與無上限（nil）要分得開：%v / %v", page.Campaigns[1].Campaign.TotalLimit, page.Campaigns[0].Campaign.TotalLimit)
	}
	v := f.claim(a1, buyer1)
	f.mustLock(v, "ORD-9")
	d, err := f.m.GetSellerCampaign(context.Background(), storeA, a1.ID)
	if err != nil || d.Locked != 1 || d.Available != 0 {
		t.Fatalf("明細：%v %+v", err, d)
	}
	d2, err := f.m.GetSellerCampaign(context.Background(), storeA, a2.ID)
	if err != nil || !slices.Equal(d2.TargetBuyers, []int64{buyer2, buyer3}) {
		t.Fatalf("名單：%v %v", err, d2.TargetBuyers)
	}
	_, errOther := f.m.GetSellerCampaign(context.Background(), storeA, b1.ID)
	_, errMissing := f.m.GetSellerCampaign(context.Background(), storeA, uuid.UUID{9})
	if coupon.ReasonOf(errOther) != coupon.ReasonCampaignNotFound || coupon.ReasonOf(errMissing) != coupon.ReasonCampaignNotFound {
		t.Fatalf("別家與不存在要回同一個 NotFound：%v / %v", errOther, errMissing)
	}
	ended, err := f.m.ListSellerCampaigns(context.Background(), coupon.SellerListInput{StoreID: storeA, Status: coupon.StatusEnded})
	if err != nil || len(ended.Campaigns) != 0 {
		t.Fatalf("ENDED 篩選：%v %d", err, len(ended.Campaigns))
	}
}

// 可領列表的 SQL 先排除別人的 DESIGNATED 活動：它們不該讓列表對 Stores port 發出查詢。
func TestListClaimableSkipsDesignatedCampaignsOfOtherBuyersBeforeAskingStores(t *testing.T) {
	f := setup(t)
	f.sellerCampaign(storeA, func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer2}
	})
	mine := f.sellerCampaign(storeB, func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer1}
	})
	p, err := f.m.ListClaimable(context.Background(), coupon.ClaimableInput{BuyerID: buyer1})
	if err != nil || len(p.Campaigns) != 1 || p.Campaigns[0].ID != mine.ID {
		t.Fatalf("可領的活動：err=%v %+v", err, p.Campaigns)
	}
	if got := f.stores.calls.Load(); got != 1 {
		t.Fatalf("Stores 被問了 %d 次，只有 buyer1 在名單內的那一個活動需要問（1 次）", got)
	}
}

// 同一頁多張券來自同一家店時，停權狀態只問 Stores 一次。
func TestListVouchersCheckoutAsksStoresOncePerStore(t *testing.T) {
	f := setup(t)
	for range 3 {
		f.claim(f.sellerCampaign(storeA), buyer1)
	}
	f.stores.calls.Store(0)
	p, err := f.m.ListVouchers(context.Background(), coupon.VoucherListInput{BuyerID: buyer1, Checkout: &coupon.Checkout{StoreID: storeA, OrderAmount: 1000}})
	if err != nil || len(p.Vouchers) != 3 {
		t.Fatalf("結帳列表：err=%v %d", err, len(p.Vouchers))
	}
	if got := f.stores.calls.Load(); got != 1 {
		t.Fatalf("Stores 被問了 %d 次，同一家店只該問 1 次", got)
	}
	f.stores.calls.Store(0)
	if _, err := f.m.ListClaimable(context.Background(), coupon.ClaimableInput{BuyerID: buyer2}); err != nil {
		t.Fatal(err)
	}
	if got := f.stores.calls.Load(); got != 1 {
		t.Fatalf("可領列表的 3 個同店活動問了 Stores %d 次，應是 1 次", got)
	}
}

// expired 由模組的時鐘判斷：時間窗過了，列表與明細都是 true，之前是 false。
func TestSellerCampaignExpiredFollowsTheModuleClock(t *testing.T) {
	f := setup(t)
	c := f.sellerCampaign(storeA)
	list := func() (bool, bool) {
		p, err := f.m.ListSellerCampaigns(context.Background(), coupon.SellerListInput{StoreID: storeA})
		if err != nil || len(p.Campaigns) != 1 {
			t.Fatalf("列表：%v %d", err, len(p.Campaigns))
		}
		d, err := f.m.GetSellerCampaign(context.Background(), storeA, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		return p.Campaigns[0].Expired, d.Expired
	}
	if l, d := list(); l || d {
		t.Fatalf("時間窗內：列表 %v、明細 %v，都該是 false", l, d)
	}
	f.advance(2 * windowLen)
	if l, d := list(); !l || !d {
		t.Fatalf("時間窗過了：列表 %v、明細 %v，都該是 true", l, d)
	}
}
