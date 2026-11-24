//go:build integration

package coupon_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
)

func actor(action string) adminaudit.Actor { return adminaudit.Actor{AdminID: adminID, Action: action} }

func (f *fixture) auditRows(action string) int64 {
	f.t.Helper()
	return f.scalar("SELECT count(*) FROM adminaudit.records WHERE action = $1", action)
}

func (f *fixture) adminDraft(mod ...func(*coupon.CreateCampaignInput)) coupon.Campaign {
	f.t.Helper()
	in := coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "後台活動", DiscountType: coupon.DiscountCash, DiscountAmount: 100, StartAt: base.Add(-time.Hour), EndAt: base.Add(windowLen)},
		TargetType:   coupon.TargetAll,
	}
	for _, m := range mod {
		m(&in)
	}
	c, err := f.m.AdminCreateCampaign(context.Background(), actor("create"), in)
	if err != nil {
		f.t.Fatalf("後台建立活動：%v", err)
	}
	return c
}

// 六支後台寫入各留一筆後台動作紀錄，對象、管理員與動作都對；發行者是呼叫的管理員，輸入帶的發行者被忽略。
func TestAdminWritesRecordOneAuditRowEach(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c := f.adminDraft(func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer1}
		in.Issuer = coupon.Issuer{StoreID: storeA}
	})
	if c.Status != coupon.StatusDraft || c.IssuerAdminID == nil || *c.IssuerAdminID != adminID || c.IssuerStoreID != nil {
		t.Fatalf("後台建立：status=%s issuerAdmin=%v issuerStore=%v，預期 DRAFT、發行者是呼叫的管理員", c.Status, c.IssuerAdminID, c.IssuerStoreID)
	}
	if err := f.m.AdminPublishCampaign(ctx, actor("publish"), c.ID); err != nil {
		t.Fatal(err)
	}
	granted, err := f.m.AdminIssue(ctx, actor("grant"), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: []int64{buyer2}})
	if err != nil || granted.Issued != 1 {
		t.Fatalf("指定發放：%+v %v", granted, err)
	}
	vs, err := f.m.ListVouchers(ctx, coupon.VoucherListInput{BuyerID: buyer2})
	if err != nil || len(vs.Vouchers) != 1 {
		t.Fatalf("買家的券：%v %d", err, len(vs.Vouchers))
	}
	if err := f.m.AdminVoidVoucher(ctx, actor("void"), coupon.VoidVoucherInput{VoucherID: vs.Vouchers[0].Voucher.ID, Reason: "測試"}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.AdminSetSellerQuota(ctx, actor("quota"), storeA, 9); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.AdminEndCampaign(ctx, actor("end"), coupon.EndCampaignInput{CampaignID: c.ID, StoreID: 99999, Reason: "收尾"}); err != nil {
		t.Fatalf("後台結束不受店家範圍限制：%v", err)
	}
	for action, target := range map[string]string{
		"create": c.ID.String(), "publish": c.ID.String(), "grant": c.ID.String(), "end": c.ID.String(),
		"void": vs.Vouchers[0].Voucher.ID.String(), "quota": "501",
	} {
		if n := f.auditRows(action); n != 1 {
			t.Errorf("動作 %s 的後台紀錄 = %d 筆，預期 1", action, n)
		}
		if n := f.scalar("SELECT count(*) FROM adminaudit.records WHERE action = $1 AND target_id = $2 AND admin_id = $3", action, target, adminID); n != 1 {
			t.Errorf("動作 %s 的紀錄對象或管理員不對（預期對象 %s、管理員 %d）", action, target, adminID)
		}
	}
}

// 紀錄與領域變更同一個 transaction：領域失敗不留紀錄；紀錄寫不進去（沒有管理員身分）領域變更跟著回滾。
func TestAdminAuditAndDomainChangeCommitOrRollBackTogether(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c := f.adminDraft()
	if err := f.m.AdminPublishCampaign(ctx, actor("publish"), c.ID); err != nil {
		t.Fatal(err)
	}
	wantReason(t, f.m.AdminPublishCampaign(ctx, actor("publish"), c.ID), coupon.ReasonInvalidStatus)
	if n := f.auditRows("publish"); n != 1 {
		t.Fatalf("發布失敗後紀錄 = %d 筆，預期仍是 1", n)
	}

	draft := f.adminDraft()
	if err := f.m.AdminPublishCampaign(ctx, adminaudit.Actor{AdminID: 0, Action: "publish-broken"}, draft.ID); err == nil {
		t.Fatal("沒有管理員身分，紀錄寫不進去，發布要失敗")
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_campaigns WHERE id = $1 AND status = 'DRAFT'", draft.ID); got != 1 {
		t.Fatal("紀錄失敗，發布要回滾，活動仍是 DRAFT")
	}

	if _, err := f.m.AdminCreateCampaign(ctx, adminaudit.Actor{AdminID: 0, Action: "create-broken"}, coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "x", DiscountType: coupon.DiscountCash, DiscountAmount: 1, StartAt: base, EndAt: base.Add(windowLen)}, TargetType: coupon.TargetAll,
	}); err == nil {
		t.Fatal("沒有管理員身分，建立要失敗")
	}
	if n := f.scalar("SELECT count(*) FROM coupon.coupon_campaigns WHERE title = 'x'"); n != 0 {
		t.Fatalf("紀錄失敗，建立的活動要回滾（殘留 %d 筆）", n)
	}
}

// 唯讀方法不寫後台紀錄。
func TestAdminReadsWriteNoAuditRow(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c := f.campaign()
	f.claim(c, buyer1)
	before := f.scalar("SELECT count(*) FROM adminaudit.records")
	if _, err := f.m.AdminListCampaigns(ctx, coupon.AdminListInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.AdminGetCampaign(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.ListVouchers(ctx, coupon.VoucherListInput{BuyerID: buyer1}); err != nil {
		t.Fatal(err)
	}
	if after := f.scalar("SELECT count(*) FROM adminaudit.records"); after != before {
		t.Fatalf("唯讀方法留下了 %d 筆後台紀錄", after-before)
	}
}

func campaignIDs(p coupon.AdminCampaignPage) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Campaigns))
	for i, c := range p.Campaigns {
		out[i] = c.Campaign.ID
	}
	return out
}

// 後台列表看得到平台與店家的活動，依狀態與發行者篩選，不含補償活動；翻頁不重不漏。
func TestAdminListCampaignsFiltersAndPages(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.advance(time.Second)
	platform := f.campaign()
	f.advance(time.Second)
	shopA := f.sellerCampaign(storeA)
	f.advance(time.Second)
	shopB := f.sellerCampaign(storeA + 1)
	f.advance(time.Second)
	draft := f.adminDraft()
	if _, err := f.m.AdminEndCampaign(ctx, actor("end"), coupon.EndCampaignInput{CampaignID: shopB.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.IssueCompensation(ctx, coupon.CompensationInput{
		SourceType: coupon.SourceAftersalesCase, SourceID: "AS-9", BuyerID: buyer1, StoreID: storeA, Amount: 100, MinOrderAmount: 500,
		EndAt: base.Add(30 * windowLen), Title: "補償", Issuer: coupon.Issuer{StoreID: storeA},
	}); err != nil {
		t.Fatal(err)
	}

	list := func(in coupon.AdminListInput) []uuid.UUID {
		t.Helper()
		p, err := f.m.AdminListCampaigns(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		return campaignIDs(p)
	}
	if got, want := list(coupon.AdminListInput{}), []uuid.UUID{draft.ID, shopB.ID, shopA.ID, platform.ID}; !slices.Equal(got, want) {
		t.Fatalf("全部（新到舊、不含補償）= %v，預期 %v", got, want)
	}
	if got, want := list(coupon.AdminListInput{PlatformOnly: true}), []uuid.UUID{draft.ID, platform.ID}; !slices.Equal(got, want) {
		t.Fatalf("平台 = %v，預期 %v", got, want)
	}
	if got, want := list(coupon.AdminListInput{StoreID: storeA}), []uuid.UUID{shopA.ID}; !slices.Equal(got, want) {
		t.Fatalf("店家 A = %v，預期 %v", got, want)
	}
	if got, want := list(coupon.AdminListInput{Status: coupon.StatusEnded}), []uuid.UUID{shopB.ID}; !slices.Equal(got, want) {
		t.Fatalf("ENDED = %v，預期 %v", got, want)
	}
	if got, want := list(coupon.AdminListInput{Status: coupon.StatusDraft, PlatformOnly: true}), []uuid.UUID{draft.ID}; !slices.Equal(got, want) {
		t.Fatalf("DRAFT 加平台 = %v，預期 %v", got, want)
	}
	if _, err := f.m.AdminListCampaigns(ctx, coupon.AdminListInput{PlatformOnly: true, StoreID: storeA}); err == nil {
		t.Fatal("平台與店家同時篩選要被拒絕")
	}
	var got []uuid.UUID
	var after *coupon.Cursor
	for pages := 0; ; pages++ {
		if pages > 4 {
			t.Fatal("翻頁沒有結束")
		}
		p, err := f.m.AdminListCampaigns(ctx, coupon.AdminListInput{Size: 3, After: after})
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, campaignIDs(p)...)
		if p.Next == nil {
			break
		}
		after = p.Next
	}
	if want := []uuid.UUID{draft.ID, shopB.ID, shopA.ID, platform.ID}; !slices.Equal(got, want) {
		t.Fatalf("分頁累積 = %v，預期 %v", got, want)
	}
}

// 明細含上限（未設定與有上限分得開）、已發數量與各狀態券數；補償活動也讀得到。
func TestAdminGetCampaignShowsLimitIssuedAndCounts(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	unlimited := f.campaign()
	limited := f.campaign(func(in *coupon.CreateCampaignInput) { in.TotalLimit = 3 })
	f.claim(limited, buyer1)
	v2 := f.claim(limited, buyer2)
	f.mustLock(v2, "ORD-ADM-1")
	u, err := f.m.AdminGetCampaign(ctx, unlimited.ID)
	if err != nil || u.Campaign.TotalLimit != nil {
		t.Fatalf("無上限的活動 TotalLimit = %v（%v），預期 nil", u.Campaign.TotalLimit, err)
	}
	l, err := f.m.AdminGetCampaign(ctx, limited.ID)
	if err != nil || l.Campaign.TotalLimit == nil || *l.Campaign.TotalLimit != 3 || l.Campaign.IssuedCount != 2 || l.Available != 1 || l.Locked != 1 {
		t.Fatalf("明細 = %+v（%v），預期上限 3、已發 2、AVAILABLE 1、LOCKED 1", l, err)
	}
	if _, err := f.m.AdminGetCampaign(ctx, uuid.UUID{9}); coupon.ReasonOf(err) != coupon.ReasonCampaignNotFound {
		t.Fatalf("不存在的活動：%v", err)
	}
}

func (f *fixture) platformDesignated(buyers ...int64) coupon.Campaign {
	f.t.Helper()
	c := f.adminDraft(func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, buyers
	})
	if err := f.m.AdminPublishCampaign(context.Background(), actor("publish"), c.ID); err != nil {
		f.t.Fatal(err)
	}
	return c
}

// 指定發放只能用在平台的活動：店家發的 DESIGNATED 活動回 NOT_FOUND，不建券、不留紀錄。
func TestAdminIssueRejectsAStoreCampaign(t *testing.T) {
	f := setup(t)
	shop := f.sellerCampaign(storeA, func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer1}
	})
	_, err := f.m.AdminIssue(context.Background(), actor("grant"), coupon.IssueInput{CampaignID: shop.ID, BuyerIDs: []int64{buyer2}})
	wantReason(t, err, coupon.ReasonCampaignNotFound)
	if n := f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1", shop.ID); n != 0 {
		t.Fatalf("店家活動不該被發出券，有 %d 張", n)
	}
	if n := f.auditRows("grant"); n != 0 {
		t.Fatalf("失敗的發放不該留紀錄，有 %d 筆", n)
	}
}

// 後台只能結束平台發的非補償活動：店家活動與補償活動都回 NOT_FOUND，活動、券與紀錄都不變。
func TestAdminEndCampaignRejectsStoreAndCompensationCampaigns(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	shop := f.sellerCampaign(storeA)
	shopVoucher := f.claim(shop, buyer1)
	comp, err := f.m.IssueCompensation(ctx, coupon.CompensationInput{
		SourceType: coupon.SourceAdminCompensation, SourceID: "ADM-1", BuyerID: buyer1, StoreID: storeA, Amount: 100, MinOrderAmount: 500,
		EndAt: base.Add(30 * windowLen), Title: "補償", Issuer: coupon.Issuer{AdminID: adminID},
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]uuid.UUID{"店家活動": shop.ID, "補償活動": comp.Campaign.ID} {
		_, err := f.m.AdminEndCampaign(ctx, actor("end"), coupon.EndCampaignInput{CampaignID: id, Reason: "x"})
		if coupon.ReasonOf(err) != coupon.ReasonCampaignNotFound {
			t.Errorf("%s：原因 = %q（%v），預期 %s", name, coupon.ReasonOf(err), err, coupon.ReasonCampaignNotFound)
		}
	}
	if n := f.scalar("SELECT count(*) FROM coupon.coupon_campaigns WHERE id = ANY($1) AND status = 'ACTIVE'", []uuid.UUID{shop.ID, comp.Campaign.ID}); n != 2 {
		t.Fatalf("兩個活動都不該被結束，ACTIVE 的有 %d 個", n)
	}
	if got := f.voucherStatus(shopVoucher.ID); got != "AVAILABLE" {
		t.Fatalf("店家活動的券 = %s，預期 AVAILABLE", got)
	}
	if n := f.auditRows("end"); n != 0 {
		t.Fatalf("失敗的結束不該留紀錄，有 %d 筆", n)
	}
}

// 發放先去重再檢查 1 到 500 位：500 個不重複加 1 個重複成功且只發 500 張；501 個不重複被拒。
func TestAdminIssueDeduplicatesBeforeTheBatchLimit(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c := f.platformDesignated(buyer1)
	ids := make([]int64, 0, coupon.MaxIssueBatch+1)
	for i := range coupon.MaxIssueBatch {
		ids = append(ids, int64(10000+i))
	}
	res, err := f.m.AdminIssue(ctx, actor("grant"), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: append(slices.Clone(ids), ids[0])})
	if err != nil || res.Issued != coupon.MaxIssueBatch {
		t.Fatalf("500 不重複加 1 重複：%+v %v，預期發出 %d", res, err, coupon.MaxIssueBatch)
	}
	if n := f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1", c.ID); n != coupon.MaxIssueBatch {
		t.Fatalf("券 %d 張，預期 %d", n, coupon.MaxIssueBatch)
	}
	_, err = f.m.AdminIssue(ctx, actor("grant"), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: append(slices.Clone(ids), 99999)})
	if err == nil {
		t.Fatal("501 個不重複要被拒")
	}
}

// 結束活動的稽核紀錄與活動操作紀錄用同一個去空白後的 reason：全空白的理由兩邊都是空。
func TestAdminEndCampaignAuditUsesTheNormalizedReason(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for action, reason := range map[string]string{"end-blank": " \t ", "end-padded": "  收尾  "} {
		c := f.platformDesignated(buyer1)
		if _, err := f.m.AdminEndCampaign(ctx, actor(action), coupon.EndCampaignInput{CampaignID: c.ID, Reason: reason}); err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSpace(reason)
		var got string
		if err := f.db.Pool().QueryRow(ctx, "SELECT detail->>'reason' FROM adminaudit.records WHERE action = $1", action).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s：稽核 reason = %q，預期 %q", action, got, want)
		}
		var logged *string
		if err := f.db.Pool().QueryRow(ctx, "SELECT reason FROM coupon.coupon_redemption_logs WHERE campaign_id = $1 AND action = 'END_CAMPAIGN'", c.ID).Scan(&logged); err != nil {
			t.Fatal(err)
		}
		if (want == "") != (logged == nil) || (logged != nil && *logged != want) {
			t.Errorf("%s：活動操作紀錄 reason = %v，預期 %q", action, logged, want)
		}
	}
}

// 指定發放對補償活動（無論發行者）回 NOT_FOUND，不建券：補償券只由系統依來源冪等發放。
func TestAdminIssueRejectsACompensationCampaignWhoeverIssuedIt(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	comp, err := f.m.IssueCompensation(ctx, coupon.CompensationInput{
		SourceType: coupon.SourceAdminCompensation, SourceID: "ADM-2", BuyerID: buyer1, StoreID: storeA, Amount: 100, MinOrderAmount: 500,
		EndAt: base.Add(30 * windowLen), Title: "補償", Issuer: coupon.Issuer{AdminID: adminID},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.m.AdminIssue(ctx, actor("grant"), coupon.IssueInput{CampaignID: comp.Campaign.ID, BuyerIDs: []int64{buyer2}})
	wantReason(t, err, coupon.ReasonCampaignNotFound)
	if n := f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1", comp.Campaign.ID); n != 1 {
		t.Fatalf("補償活動的券 %d 張，預期仍是原本的 1 張", n)
	}
	if n := f.auditRows("grant"); n != 0 {
		t.Fatalf("失敗的發放不該留紀錄，有 %d 筆", n)
	}
}

// 稽核紀錄的 granted_buyers 是去重後的人數，不是請求的原始個數。
func TestAdminIssueAuditRecordsTheDeduplicatedBuyerCount(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c := f.platformDesignated(buyer1)
	if _, err := f.m.AdminIssue(ctx, actor("grant"), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: []int64{buyer2, buyer3, buyer2, buyer3, buyer2}}); err != nil {
		t.Fatal(err)
	}
	if n := f.scalar("SELECT (detail->>'granted_buyers')::bigint FROM adminaudit.records WHERE action = 'grant'"); n != 2 {
		t.Fatalf("granted_buyers = %d，預期去重後的 2", n)
	}
}

// 後台發布只限平台發的非補償活動：店家發的 DRAFT 活動回 NOT_FOUND 且狀態不變、不留紀錄；平台的照常發布。
func TestAdminPublishRejectsAStoreCampaign(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	shop := f.sellerCampaign(storeA)
	if err := f.exec("UPDATE coupon.coupon_campaigns SET status = 'DRAFT' WHERE id = $1", shop.ID); err != nil {
		t.Fatal(err)
	}
	wantReason(t, f.m.AdminPublishCampaign(ctx, actor("publish"), shop.ID), coupon.ReasonCampaignNotFound)
	if n := f.scalar("SELECT count(*) FROM coupon.coupon_campaigns WHERE id = $1 AND status = 'DRAFT'", shop.ID); n != 1 {
		t.Fatal("店家活動不該被後台發布")
	}
	if n := f.auditRows("publish"); n != 0 {
		t.Fatalf("失敗的發布不該留紀錄，有 %d 筆", n)
	}
	wantReason(t, f.m.AdminPublishCampaign(ctx, actor("publish"), uuid.New()), coupon.ReasonCampaignNotFound)
	f.platformDesignated(buyer1)
}

// 限店的平台活動：店家不存在或已關閉回 InvalidInput 指出 StoreID 且不建活動；存在的（含停權的）照常建立。
func TestAdminCreateCampaignChecksTheStoreBeforeWriting(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.dir.missing[storeB+1] = true
	f.dir.closed[storeB] = true
	for name, store := range map[string]int64{"不存在": storeB + 1, "已關閉": storeB} {
		_, err := f.m.AdminCreateCampaign(ctx, actor("create"), coupon.CreateCampaignInput{
			CampaignSpec: coupon.CampaignSpec{Title: "限店", DiscountType: coupon.DiscountCash, DiscountAmount: 100, StartAt: base.Add(-time.Hour), EndAt: base.Add(windowLen)},
			TargetType:   coupon.TargetAll, StoreID: store,
		})
		if err == nil || !strings.Contains(err.Error(), "店家") {
			t.Fatalf("店家%s：err=%v，預期店家的欄位錯誤", name, err)
		}
	}
	if n := f.scalar("SELECT count(*) FROM coupon.coupon_campaigns"); n != 0 {
		t.Fatalf("被拒的建立不該寫入活動，有 %d 個", n)
	}
	f.stores.set(storeA, true)
	f.adminDraft(func(in *coupon.CreateCampaignInput) { in.StoreID = storeA })
}

// 非 DESIGNATED 的活動帶名單：不查買家，維持「名單只給 DESIGNATED」的錯誤。
func TestAdminCreateCampaignLooksUpBuyersOnlyForDesignated(t *testing.T) {
	f := setup(t)
	f.buyers.missing[buyer1] = true
	_, err := f.m.AdminCreateCampaign(context.Background(), actor("create"), coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "全平台", DiscountType: coupon.DiscountCash, DiscountAmount: 100, StartAt: base.Add(-time.Hour), EndAt: base.Add(windowLen)},
		TargetType:   coupon.TargetAll, TargetBuyers: []int64{buyer1},
	})
	if err == nil || strings.Contains(err.Error(), "買家不存在") || len(f.buyers.calls) != 0 {
		t.Fatalf("err=%v 買家查詢=%d 次，預期不查買家且不是買家不存在", err, len(f.buyers.calls))
	}
	_, err = f.m.AdminCreateCampaign(context.Background(), actor("create"), coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "指定", DiscountType: coupon.DiscountCash, DiscountAmount: 100, StartAt: base.Add(-time.Hour), EndAt: base.Add(windowLen)},
		TargetType:   coupon.TargetDesignated, TargetBuyers: []int64{buyer2, buyer1},
	})
	if err == nil || !strings.Contains(err.Error(), "買家不存在") {
		t.Fatalf("DESIGNATED 的未知買家：err=%v", err)
	}
}
