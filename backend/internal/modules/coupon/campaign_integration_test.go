//go:build integration

package coupon_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
)

// D5、BD8：結束活動只擋新的領取與鎖定、作廢 AVAILABLE；LOCKED 的券照常核銷。重送結束是成功。
// 結束時連 LOCKED 一起作廢，這個測試變紅。
func TestEndCampaignKeepsLockedVouchersRedeemable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign()
	locked := f.claim(c, buyer1)
	free := f.claim(c, buyer2)
	f.mustLock(locked, "O1")

	res, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: c.ID, AdminID: adminID, Reason: "預算用完"})
	if err != nil || res.Voided != 1 {
		t.Fatalf("結束 = %+v, %v", res, err)
	}
	if got := f.voucherStatus(free.ID); got != "VOIDED" {
		t.Fatalf("AVAILABLE 的券狀態 = %s，要 VOIDED", got)
	}
	if got := f.voucherStatus(locked.ID); got != "LOCKED" {
		t.Fatalf("LOCKED 的券狀態 = %s，要維持 LOCKED", got)
	}

	_, err = f.m.Claim(context.Background(), c.ID, buyer3)
	wantReason(t, err, coupon.ReasonCampaignNotActive)
	_, err = f.lock(free, "O2")
	wantReason(t, err, coupon.ReasonCampaignNotActive)

	again, err := f.lock(locked, "O1")
	if err != nil || !again.Replayed {
		t.Fatalf("結束後同一筆訂單的重送要冪等成功：%+v, %v", again, err)
	}
	if _, err := f.m.Redeem(context.Background(), "O1"); err != nil {
		t.Fatalf("結束後 LOCKED 的券要能核銷：%v", err)
	}
	if got := f.voucherStatus(locked.ID); got != "REDEEMED" {
		t.Fatalf("核銷後狀態 = %s", got)
	}
	res, err = f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: c.ID, AdminID: adminID})
	if err != nil || res.Voided != 0 {
		t.Fatalf("重送結束 = %+v, %v", res, err)
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_redemption_logs WHERE campaign_id = $1 AND action = 'END_CAMPAIGN'", c.ID); got != 1 {
		t.Fatalf("END_CAMPAIGN 稽核 %d 筆，要 1", got)
	}
}

// D5：結束後釋放 LOCKED 的券，券作廢而不是回到可用（結束之後沒有 AVAILABLE）。
func TestReleaseAfterEndVoidsInsteadOfRestoring(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign()
	v := f.claim(c, buyer1)
	f.mustLock(v, "O1")
	if _, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: c.ID, AdminID: adminID}); err != nil {
		t.Fatal(err)
	}
	rel, err := f.m.Release(context.Background(), "O1")
	if err != nil || !rel.Released || !rel.Voided {
		t.Fatalf("釋放 = %+v, %v", rel, err)
	}
	if got := f.voucherStatus(v.ID); got != "VOIDED" {
		t.Fatalf("狀態 = %s，要 VOIDED", got)
	}
}

// 結束與領券並發：結束提交之後沒有任何新的領取成功，也沒有 AVAILABLE 的券留在已結束的活動。
func TestEndCampaignRacingWithClaims(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign()
	errs := parallel(12, func(i int) error {
		if i == 6 {
			_, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: c.ID, AdminID: adminID})
			return err
		}
		_, err := f.m.Claim(context.Background(), c.ID, 4000+int64(i))
		return err
	})
	for i, err := range errs {
		if err != nil && (i == 6 || coupon.ReasonOf(err) != coupon.ReasonCampaignNotActive) {
			t.Fatalf("第 %d 個：%v", i, err)
		}
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1 AND status = 'AVAILABLE'", c.ID); got != 0 {
		t.Fatalf("已結束的活動留下 %d 張 AVAILABLE 的券", got)
	}
}

// 賣家只能結束自己店的活動：別人的活動回 NOT_FOUND，不洩漏存在。
func TestEndCampaignIsScopedToTheIssuingStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) { in.Issuer = coupon.Issuer{StoreID: storeA} })
	_, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: c.ID, StoreID: storeB})
	wantReason(t, err, coupon.ReasonCampaignNotFound)
	if _, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: c.ID, StoreID: storeA}); err != nil {
		t.Fatal(err)
	}
	_, err = f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: coupon.Campaign{}.ID})
	wantReason(t, err, coupon.ReasonCampaignNotFound)
}

// BD32：建立時折抵金額不得超過門檻；門檻 0（無門檻）不套用。DB 的 CHECK 是後備：直接寫入同樣被擋下。
func TestCreateCampaignRejectsDiscountAboveThreshold(t *testing.T) {
	t.Parallel()
	f := setup(t)
	spec := coupon.CampaignSpec{Title: "活動", DiscountType: coupon.DiscountCash, DiscountAmount: 500, ThresholdAmount: 300, StartAt: base, EndAt: base.Add(windowLen)}
	in := coupon.CreateCampaignInput{CampaignSpec: spec, TargetType: coupon.TargetAll, Issuer: coupon.Issuer{AdminID: adminID}}
	if _, err := f.m.CreateCampaign(context.Background(), in); err == nil {
		t.Fatal("折抵 500 超過門檻 300，要被擋下")
	}
	in.ThresholdAmount = 500
	if _, err := f.m.CreateCampaign(context.Background(), in); err != nil {
		t.Fatalf("折抵等於門檻要通過：%v", err)
	}
	in.ThresholdAmount = 0
	if _, err := f.m.CreateCampaign(context.Background(), in); err != nil {
		t.Fatalf("門檻 0 是無門檻，不套用：%v", err)
	}
	err := f.exec(insertCampaignSQL, "00000000-0000-7000-8000-000000000001", 500, 300, "ADMIN", 1, nil, nil)
	wantConstraint(t, err, "campaigns_discount_within_threshold_check")
}

func TestCreateCampaignValidatesTheSpec(t *testing.T) {
	t.Parallel()
	f := setup(t)
	good := coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: " 活動 ", DiscountType: coupon.DiscountCash, DiscountAmount: 100, StartAt: base, EndAt: base.Add(windowLen)},
		TargetType:   coupon.TargetAll, Issuer: coupon.Issuer{AdminID: adminID},
	}
	for name, mod := range map[string]func(*coupon.CreateCampaignInput){
		"名稱空白":     func(in *coupon.CreateCampaignInput) { in.Title = "  " },
		"名稱太長":     func(in *coupon.CreateCampaignInput) { in.Title = string(make([]rune, 101)) + "x" },
		"現金折抵 0":   func(in *coupon.CreateCampaignInput) { in.DiscountAmount = 0 },
		"現金折抵帶百分比": func(in *coupon.CreateCampaignInput) { in.DiscountPercent = 10 },
		"百分比 100": func(in *coupon.CreateCampaignInput) {
			in.DiscountType, in.DiscountAmount, in.DiscountPercent, in.MaxDiscountAmount = coupon.DiscountPercent, 0, 100, 10
		},
		"百分比沒有上限": func(in *coupon.CreateCampaignInput) {
			in.DiscountType, in.DiscountAmount, in.DiscountPercent = coupon.DiscountPercent, 0, 10
		},
		"門檻為負":        func(in *coupon.CreateCampaignInput) { in.ThresholdAmount = -1 },
		"起日晚於迄日":      func(in *coupon.CreateCampaignInput) { in.StartAt = in.EndAt.Add(time.Second) },
		"未知的折抵方式":     func(in *coupon.CreateCampaignInput) { in.DiscountType = "FREE_SHIPPING" },
		"補償券不能直接建立":   func(in *coupon.CreateCampaignInput) { in.TargetType = coupon.TargetCompensation },
		"MEMBER 沒有店家": func(in *coupon.CreateCampaignInput) { in.TargetType = coupon.TargetMember },
		"ALL 帶名單":     func(in *coupon.CreateCampaignInput) { in.TargetBuyers = []int64{buyer1} },
		"名單含非正數": func(in *coupon.CreateCampaignInput) {
			in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{0}
		},
		"總量為負":          func(in *coupon.CreateCampaignInput) { in.TotalLimit = -1 },
		"沒有發行者":         func(in *coupon.CreateCampaignInput) { in.Issuer = coupon.Issuer{} },
		"賣家與 admin 同時有": func(in *coupon.CreateCampaignInput) { in.Issuer = coupon.Issuer{AdminID: 1, StoreID: 1} },
		"賣家的 DESIGNATED 沒有名單": func(in *coupon.CreateCampaignInput) {
			in.Issuer, in.TargetType = coupon.Issuer{StoreID: storeA}, coupon.TargetDesignated
		},
		"賣家發給別家店": func(in *coupon.CreateCampaignInput) { in.Issuer, in.StoreID = coupon.Issuer{StoreID: storeA}, storeB },
	} {
		in := good
		mod(&in)
		if _, err := f.m.CreateCampaign(context.Background(), in); err == nil {
			t.Errorf("%s：要被擋下", name)
		}
	}
	c, err := f.m.CreateCampaign(context.Background(), good)
	if err != nil || c.Title != "活動" || c.Status != coupon.StatusDraft {
		t.Fatalf("合法的活動 = %+v, %v", c, err)
	}
	if err := f.m.PublishCampaign(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	wantReason(t, f.m.PublishCampaign(context.Background(), c.ID), coupon.ReasonInvalidStatus)
	wantReason(t, f.m.PublishCampaign(context.Background(), coupon.Campaign{}.ID), coupon.ReasonCampaignNotFound)
}

// BD15：賣家的活動數配額在鎖住配額列之後檢查：配額 2，同時建 6 個，恰好 2 個成功；admin 調高配額後可以再建。
// 結束的活動不占配額；補償券不算。
func TestSellerQuotaIsEnforcedAtomically(t *testing.T) {
	t.Parallel()
	f := setup(t)
	if err := f.m.SetSellerQuota(context.Background(), storeA, 2); err != nil {
		t.Fatal(err)
	}
	create := func() (coupon.Campaign, error) {
		return f.m.CreateCampaign(context.Background(), coupon.CreateCampaignInput{
			CampaignSpec: coupon.CampaignSpec{Title: "賣家券", DiscountType: coupon.DiscountCash, DiscountAmount: 50, StartAt: base, EndAt: base.Add(windowLen)},
			TargetType:   coupon.TargetAll, Issuer: coupon.Issuer{StoreID: storeA},
		})
	}
	var made []coupon.Campaign
	results := make(chan coupon.Campaign, 6)
	errs := parallel(6, func(int) error {
		c, err := create()
		if err == nil {
			results <- c
		}
		return err
	})
	close(results)
	for c := range results {
		made = append(made, c)
	}
	for _, err := range errs {
		if err != nil {
			wantReason(t, err, coupon.ReasonQuotaExceeded)
		}
	}
	if len(made) != 2 {
		t.Fatalf("成功 %d 個，要 2", len(made))
	}
	if made[0].Status != coupon.StatusActive {
		t.Fatalf("賣家的活動要直接 ACTIVE：%+v", made[0])
	}
	_, err := create()
	wantReason(t, err, coupon.ReasonQuotaExceeded)

	if _, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: made[0].ID, StoreID: storeA}); err != nil {
		t.Fatal(err)
	}
	if _, err := create(); err != nil {
		t.Fatalf("結束的活動不占配額：%v", err)
	}
	if _, err := f.m.IssueCompensation(context.Background(), coupon.CompensationInput{
		SourceType: coupon.SourceAftersalesCase, SourceID: "AS-quota", BuyerID: buyer1, StoreID: storeA,
		Amount: 100, MinOrderAmount: 500, EndAt: base.Add(windowLen), Title: "補償", Issuer: coupon.Issuer{StoreID: storeA},
	}); err != nil {
		t.Fatalf("補償券不受配額限制：%v", err)
	}
	if err := f.m.SetSellerQuota(context.Background(), storeA, 0); err != nil {
		t.Fatal(err)
	}
	_, err = create()
	wantReason(t, err, coupon.ReasonQuotaExceeded)
	if err := f.m.SetSellerQuota(context.Background(), 0, 1); err == nil {
		t.Fatal("店家 0 要被擋下")
	}
}

// 沒有配額列的店家用預設配額。
func TestSellerWithoutQuotaRowGetsTheDefault(t *testing.T) {
	t.Parallel()
	f := setup(t)
	for i := range coupon.DefaultSellerQuota {
		if _, err := f.m.CreateCampaign(context.Background(), coupon.CreateCampaignInput{
			CampaignSpec: coupon.CampaignSpec{Title: fmt.Sprint("券", i), DiscountType: coupon.DiscountCash, DiscountAmount: 50, StartAt: base, EndAt: base.Add(windowLen)},
			TargetType:   coupon.TargetAll, Issuer: coupon.Issuer{StoreID: storeB},
		}); err != nil {
			t.Fatalf("第 %d 個：%v", i, err)
		}
	}
	_, err := f.m.CreateCampaign(context.Background(), coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "超過", DiscountType: coupon.DiscountCash, DiscountAmount: 50, StartAt: base, EndAt: base.Add(windowLen)},
		TargetType:   coupon.TargetAll, Issuer: coupon.Issuer{StoreID: storeB},
	})
	wantReason(t, err, coupon.ReasonQuotaExceeded)
}
