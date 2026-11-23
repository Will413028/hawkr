//go:build integration

package coupon_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
)

// 同一個活動上限 5，20 位買家同時領：恰好 5 張，其餘都是 ISSUE_LIMIT_REACHED，發行數不超過上限。
// 拿掉 IncrementIssued 的遞增條件，超出的領取會撞 campaigns_issued_count_check 而不是 ISSUE_LIMIT_REACHED（或超發）。
func TestConcurrentClaimNeverOverIssues(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) { in.TotalLimit = 5 })
	errs := parallel(20, func(i int) error {
		_, err := f.m.Claim(context.Background(), c.ID, 2000+int64(i))
		return err
	})
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		wantReason(t, err, coupon.ReasonIssueLimitReached)
	}
	if ok != 5 {
		t.Fatalf("成功 %d 張，要 5", ok)
	}
	if got := f.issued(c.ID); got != 5 {
		t.Fatalf("issued_count = %d，要 5", got)
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1", c.ID); got != 5 {
		t.Fatalf("券 %d 張，要 5", got)
	}
}

// 領券冪等：同一位買家重送回既有的券，不再遞增；同一位買家並發重送也只有一張。
func TestClaimIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) { in.TotalLimit = 3 })
	first, err := f.m.Claim(context.Background(), c.ID, buyer1)
	if err != nil || !first.Created {
		t.Fatalf("第一次領券 = %+v, %v", first, err)
	}
	again, err := f.m.Claim(context.Background(), c.ID, buyer1)
	if err != nil || again.Created || again.Voucher.ID != first.Voucher.ID {
		t.Fatalf("重送 = %+v, %v", again, err)
	}
	for _, err := range parallel(8, func(int) error { _, err := f.m.Claim(context.Background(), c.ID, buyer2); return err }) {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := f.issued(c.ID); got != 2 {
		t.Fatalf("issued_count = %d，要 2（每位買家一張）", got)
	}
}

// 領券的資格：ALL 給所有買家（含活動建立之後才註冊的），MEMBER 只給在該店有消費紀錄的，DESIGNATED 只給名單內的。
func TestClaimChecksTargetEligibility(t *testing.T) {
	t.Parallel()
	f := setup(t)

	all := f.campaign()
	f.claim(all, buyer1)
	f.claim(all, buyer2)

	member := f.campaign(func(in *coupon.CreateCampaignInput) { in.TargetType, in.StoreID = coupon.TargetMember, storeA })
	_, err := f.m.Claim(context.Background(), member.ID, buyer1)
	wantReason(t, err, coupon.ReasonNotEligible)
	f.members.members[[2]int64{buyer1, storeA}] = true
	f.claim(member, buyer1)

	f.members.err = fmt.Errorf("order 不可用")
	_, err = f.m.Claim(context.Background(), member.ID, buyer3)
	if err == nil || coupon.ReasonOf(err) != "" {
		t.Fatalf("port 讀取失敗要原樣回錯誤（不降級成資格不符）：%v", err)
	}
	f.members.err = nil

	des := f.campaign(func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer1}
	})
	f.claim(des, buyer1)
	_, err = f.m.Claim(context.Background(), des.ID, buyer3)
	wantReason(t, err, coupon.ReasonNotEligible)
}

func TestClaimRejectsInactiveAndOutOfWindowCampaigns(t *testing.T) {
	t.Parallel()
	f := setup(t)
	draft, err := f.m.CreateCampaign(context.Background(), coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "草稿", DiscountType: coupon.DiscountCash, DiscountAmount: 50, StartAt: base, EndAt: base.Add(windowLen)},
		TargetType:   coupon.TargetAll, Issuer: coupon.Issuer{AdminID: adminID},
	})
	if err != nil || draft.Status != coupon.StatusDraft {
		t.Fatalf("admin 建立的活動要是 DRAFT：%+v, %v", draft, err)
	}
	_, err = f.m.Claim(context.Background(), draft.ID, buyer1)
	wantReason(t, err, coupon.ReasonCampaignNotActive)

	c := f.campaign()
	f.advance(2 * windowLen)
	_, err = f.m.Claim(context.Background(), c.ID, buyer1)
	wantReason(t, err, coupon.ReasonOutsideWindow)

	_, err = f.m.Claim(context.Background(), draft.ID, 0)
	if err == nil {
		t.Fatal("買家 0 要被擋下")
	}
}

// 一張券同時只被一筆訂單鎖：10 筆訂單同時鎖同一張券，恰好 1 筆成功，其餘 LOCKED_BY_OTHER_ORDER，券的 order_no 是成功的那筆。
// 拿掉 LockVoucher 的 status = 'AVAILABLE' 條件，後面的訂單會覆寫前一筆的鎖，全部成功。
func TestConcurrentLockSucceedsOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	v := f.claim(f.campaign(), buyer1)
	var winner atomic.Value
	errs := parallel(10, func(i int) error {
		order := fmt.Sprintf("O-%d", i)
		_, err := f.lock(v, order)
		if err == nil {
			winner.Store(order)
		}
		return err
	})
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		wantReason(t, err, coupon.ReasonVoucherLockedByOtherOrder)
	}
	if ok != 1 {
		t.Fatalf("成功 %d 筆，要 1", ok)
	}
	var order string
	if err := f.db.Pool().QueryRow(context.Background(), "SELECT order_no FROM coupon.coupon_vouchers WHERE id = $1", v.ID).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if order != winner.Load() {
		t.Fatalf("券由 %q 持有，成功的是 %v", order, winner.Load())
	}
}

// 同一筆訂單重送鎖券是冪等成功（含並發重送），稽核只記一次；別的訂單失敗；折抵金額依訂單計算。
func TestLockIsIdempotentForTheSameOrder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	v := f.claim(f.campaign(), buyer1)
	first := f.mustLock(v, "O1")
	if first.Replayed || first.Discount != 100 || first.Voucher.Status != coupon.VoucherLocked || first.Voucher.OrderNo != "O1" {
		t.Fatalf("第一次鎖 = %+v", first)
	}
	for _, err := range parallel(6, func(int) error { _, err := f.lock(v, "O1"); return err }) {
		if err != nil {
			t.Fatalf("同一筆訂單重送要冪等成功：%v", err)
		}
	}
	again := f.mustLock(v, "O1")
	if !again.Replayed {
		t.Fatalf("重送要標記 Replayed：%+v", again)
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_redemption_logs WHERE voucher_id = $1 AND action = 'LOCK'", v.ID); got != 1 {
		t.Fatalf("LOCK 稽核 %d 筆，要 1", got)
	}
	_, err := f.lock(v, "O2")
	wantReason(t, err, coupon.ReasonVoucherLockedByOtherOrder)
}

// 一筆訂單同時只持有一張券；別人的券、不存在的券都是 NOT_FOUND。
func TestLockRejectsSecondVoucherForTheSameOrderAndForeignVouchers(t *testing.T) {
	t.Parallel()
	f := setup(t)
	a, b := f.claim(f.campaign(), buyer1), f.claim(f.campaign(), buyer1)
	f.mustLock(a, "O1")
	_, err := f.lock(b, "O1")
	wantReason(t, err, coupon.ReasonOrderHoldsAnotherVoucher)

	in := f.lockInput(b, "O9")
	in.BuyerID = buyer2
	_, err = f.m.Lock(context.Background(), in)
	wantReason(t, err, coupon.ReasonVoucherNotFound)
}

// BD1 回歸：鎖券每次重驗資格。券領到之後買家被移出 DESIGNATED 名單，鎖券要失敗（舊的 reserve 不檢查）。
// 拿掉 Lock 裡的資格判定呼叫，這個測試變紅。
func TestLockRechecksEligibility(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer1}
	})
	v := f.claim(c, buyer1)
	if _, err := f.m.Quote(context.Background(), f.lockInput(v, "O1")); err != nil {
		t.Fatalf("名單內的買家驗券要通過：%v", err)
	}
	if err := f.exec("DELETE FROM coupon.coupon_target_accounts WHERE campaign_id = $1 AND buyer_id = $2", c.ID, buyer1); err != nil {
		t.Fatal(err)
	}
	_, err := f.m.Quote(context.Background(), f.lockInput(v, "O1"))
	wantReason(t, err, coupon.ReasonNotEligible)
	_, err = f.lock(v, "O1")
	wantReason(t, err, coupon.ReasonNotEligible)
	if got := f.voucherStatus(v.ID); got != "AVAILABLE" {
		t.Fatalf("被擋下的券狀態 = %s，要維持 AVAILABLE", got)
	}
}

// BD2：鎖券重驗時間窗、店家範圍與門檻，並依活動算折抵（現金不超過訂單金額，百分比取上限）。
func TestLockChecksWindowScopeThresholdAndComputesDiscount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	cash := f.campaign(func(in *coupon.CreateCampaignInput) {
		in.StoreID, in.ThresholdAmount, in.DiscountAmount = storeA, 300, 200
	})
	v := f.claim(cash, buyer1)

	in := f.lockInput(v, "O1")
	in.StoreID = storeB
	_, err := f.m.Lock(context.Background(), in)
	wantReason(t, err, coupon.ReasonNotApplicableToStore)
	in = f.lockInput(v, "O1")
	in.OrderAmount = 299
	_, err = f.m.Lock(context.Background(), in)
	wantReason(t, err, coupon.ReasonBelowThreshold)
	in.OrderAmount = 0
	_, err = f.m.Lock(context.Background(), in)
	if err == nil {
		t.Fatal("訂單金額 0 要被擋下")
	}
	in.OrderAmount = 300
	if r, err := f.m.Quote(context.Background(), in); err != nil || r.Discount != 200 {
		t.Fatalf("Quote = %+v, %v", r, err)
	}
	if got := f.voucherStatus(v.ID); got != "AVAILABLE" {
		t.Fatalf("Quote 不能鎖券：%s", got)
	}
	if r, err := f.m.Lock(context.Background(), in); err != nil || r.Discount != 200 {
		t.Fatalf("Lock = %+v, %v", r, err)
	}

	pct := f.campaign(func(in *coupon.CreateCampaignInput) {
		in.DiscountType, in.DiscountAmount, in.DiscountPercent, in.MaxDiscountAmount = coupon.DiscountPercent, 0, 20, 150
	})
	pv := f.claim(pct, buyer1)
	if r := f.mustLock(pv, "O2"); r.Discount != 150 {
		t.Fatalf("百分比折抵 = %d，1000 的 20%% 是 200，上限 150", r.Discount)
	}

	f.advance(2 * windowLen)
	late := f.campaign(func(in *coupon.CreateCampaignInput) {
		in.StartAt, in.EndAt = base.Add(-time.Hour), f.now().Add(time.Hour)
	})
	lv := f.claim(late, buyer2)
	f.advance(2 * time.Hour)
	_, err = f.lock(lv, "O3")
	wantReason(t, err, coupon.ReasonOutsideWindow)
}

// Redeem、Release 重送冪等：Redeem 兩次第二次 Replayed；Release 兩次第二次沒有東西可釋放；釋放後券可以被別的訂單鎖。
func TestRedeemAndReleaseReplayAreIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	v := f.claim(f.campaign(), buyer1)
	f.mustLock(v, "O1")

	r1, err := f.m.Redeem(context.Background(), "O1")
	if err != nil || r1.Replayed || r1.VoucherID != v.ID {
		t.Fatalf("第一次核銷 = %+v, %v", r1, err)
	}
	r2, err := f.m.Redeem(context.Background(), "O1")
	if err != nil || !r2.Replayed {
		t.Fatalf("重送核銷 = %+v, %v", r2, err)
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_redemption_logs WHERE voucher_id = $1 AND action = 'REDEEM'", v.ID); got != 1 {
		t.Fatalf("REDEEM 稽核 %d 筆，要 1", got)
	}
	_, err = f.m.Redeem(context.Background(), "NO-SUCH-ORDER")
	wantReason(t, err, coupon.ReasonNoVoucherForOrder)

	// 賣家取消已付款訂單：已核銷的券由 Release 還回。
	rel, err := f.m.Release(context.Background(), "O1")
	if err != nil || !rel.Released || rel.Voided {
		t.Fatalf("釋放已核銷的券 = %+v, %v", rel, err)
	}
	rel, err = f.m.Release(context.Background(), "O1")
	if err != nil || rel.Released {
		t.Fatalf("重送釋放 = %+v, %v", rel, err)
	}
	if got := f.voucherStatus(v.ID); got != "AVAILABLE" {
		t.Fatalf("釋放後狀態 = %s", got)
	}
	f.mustLock(v, "O2")
	if _, err := f.m.Release(context.Background(), "O2"); err != nil {
		t.Fatal(err)
	}
	if got := f.voucherStatus(v.ID); got != "AVAILABLE" {
		t.Fatalf("釋放鎖定中的券後狀態 = %s", got)
	}
	if _, err := f.m.Release(context.Background(), "NEVER-LOCKED"); err != nil {
		t.Fatalf("沒用券的訂單釋放要是無害的成功：%v", err)
	}
}

// 並發的 Redeem 與 Release 竟爭同一筆訂單的券：恰好一個贏，券最終是 REDEEMED 或 AVAILABLE，不會卡在中間。
func TestConcurrentRedeemAndReleaseSettleOnOneOutcome(t *testing.T) {
	t.Parallel()
	f := setup(t)
	v := f.claim(f.campaign(), buyer1)
	f.mustLock(v, "O1")
	errs := parallel(8, func(i int) error {
		if i%2 == 0 {
			_, err := f.m.Redeem(context.Background(), "O1")
			return err
		}
		_, err := f.m.Release(context.Background(), "O1")
		return err
	})
	for _, err := range errs {
		if err != nil && coupon.ReasonOf(err) != coupon.ReasonNoVoucherForOrder {
			t.Fatal(err)
		}
	}
	if got := f.voucherStatus(v.ID); got != "REDEEMED" && got != "AVAILABLE" {
		t.Fatalf("最終狀態 = %s", got)
	}
}

// 所有寫入加入呼叫方的 transaction：外層 rollback，鎖定跟著撤銷；外層 commit 才生效。
func TestWritesJoinTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	v := f.claim(f.campaign(), buyer1)
	rollback := errors.New("回滾")
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		if _, err := f.m.Lock(ctx, f.lockInput(v, "O1")); err != nil {
			return err
		}
		if _, err := f.m.Redeem(ctx, "O1"); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("err = %v", err)
	}
	if got := f.voucherStatus(v.ID); got != "AVAILABLE" {
		t.Fatalf("外層 rollback 之後狀態 = %s，要 AVAILABLE", got)
	}
	if err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		if _, err := f.m.Lock(ctx, f.lockInput(v, "O1")); err != nil {
			return err
		}
		_, err := f.m.Redeem(ctx, "O1")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := f.voucherStatus(v.ID); got != "REDEEMED" {
		t.Fatalf("外層 commit 之後狀態 = %s", got)
	}
}

// 系統補償：同一個來源重送（含並發）只建一張券、一個活動；其他資料與參數以第一次為準。
func TestCompensationResendCreatesOneVoucher(t *testing.T) {
	t.Parallel()
	f := setup(t)
	in := coupon.CompensationInput{
		SourceType: coupon.SourceAftersalesCase, SourceID: "AS-1", BuyerID: buyer1, StoreID: storeA,
		Amount: 100, MinOrderAmount: 500, EndAt: base.Add(30 * windowLen), Title: "售後補償券",
		Issuer: coupon.Issuer{StoreID: storeA},
	}
	var created atomic.Int32
	errs := parallel(6, func(int) error {
		r, err := f.m.IssueCompensation(context.Background(), in)
		if err == nil && r.Created {
			created.Add(1)
		}
		return err
	})
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	again, err := f.m.IssueCompensation(context.Background(), in)
	if err != nil || again.Created {
		t.Fatalf("重送 = %+v, %v", again, err)
	}
	if created.Load() != 1 {
		t.Fatalf("Created = %d 次，要 1", created.Load())
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE source_id = 'AS-1'"); got != 1 {
		t.Fatalf("券 %d 張，要 1", got)
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_campaigns WHERE source_id = 'AS-1'"); got != 1 {
		t.Fatalf("活動 %d 個，要 1", got)
	}
	if again.Voucher.Origin != coupon.OriginCompensation || again.Campaign.TargetType != coupon.TargetCompensation {
		t.Fatalf("補償券 = %+v", again)
	}

	// 補償券能鎖：收件人在資格名單內，店家範圍與門檻照常檢查。
	li := f.lockInput(again.Voucher, "O1")
	li.OrderAmount = 500
	if r, err := f.m.Lock(context.Background(), li); err != nil || r.Discount != 100 {
		t.Fatalf("鎖補償券 = %+v, %v", r, err)
	}

	other := in
	other.BuyerID = buyer2
	if _, err := f.m.IssueCompensation(context.Background(), other); err == nil {
		t.Fatal("同一個來源給另一位買家要被擋下")
	}
}

func TestCompensationRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ok := coupon.CompensationInput{
		SourceType: coupon.SourceAftersalesCase, SourceID: "AS-2", BuyerID: buyer1, StoreID: storeA,
		Amount: 100, MinOrderAmount: 600, EndAt: base.Add(windowLen), Title: "補償", Issuer: coupon.Issuer{StoreID: storeA},
	}
	for name, mod := range map[string]func(*coupon.CompensationInput){
		"門檻不到面額 5 倍":   func(in *coupon.CompensationInput) { in.MinOrderAmount = 499 },
		"門檻與面額差不到 100": func(in *coupon.CompensationInput) { in.Amount, in.MinOrderAmount = 10, 50 },
		"賣家沒有門檻":       func(in *coupon.CompensationInput) { in.MinOrderAmount = 0 },
		"面額 0":         func(in *coupon.CompensationInput) { in.Amount = 0 },
		"店家與發行者不同":     func(in *coupon.CompensationInput) { in.StoreID = storeB },
		"沒有店家":         func(in *coupon.CompensationInput) { in.StoreID = 0 },
		"來源種類未知":       func(in *coupon.CompensationInput) { in.SourceType = "OTHER" },
		"沒有來源 id":      func(in *coupon.CompensationInput) { in.SourceID = "" },
		"已過期":          func(in *coupon.CompensationInput) { in.EndAt = base },
	} {
		in := ok
		mod(&in)
		if _, err := f.m.IssueCompensation(context.Background(), in); err == nil {
			t.Errorf("%s：要被擋下", name)
		}
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_campaigns"); got != 0 {
		t.Fatalf("被擋下的補償不能留下活動：%d", got)
	}
	admin := ok
	admin.SourceType, admin.SourceID, admin.MinOrderAmount, admin.Issuer = coupon.SourceAdminCompensation, "ADM-1", 0, coupon.Issuer{AdminID: adminID}
	if _, err := f.m.IssueCompensation(context.Background(), admin); err != nil {
		t.Fatalf("admin 的補償可以沒有門檻：%v", err)
	}
}

// admin 指定發放：名單進資格表、各建一張券；已持有的不重發；全部或全不發（超過總量整筆失敗，名單也不留）。
func TestIssueIsAllOrNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) { in.TargetType, in.TotalLimit = coupon.TargetDesignated, 3 })
	r, err := f.m.Issue(context.Background(), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: []int64{buyer1, buyer2, buyer1}, AdminID: adminID})
	if err != nil || r.Issued != 2 || r.AlreadyHeld != 0 {
		t.Fatalf("Issue = %+v, %v", r, err)
	}
	r, err = f.m.Issue(context.Background(), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: []int64{buyer2}, AdminID: adminID})
	if err != nil || r.Issued != 0 || r.AlreadyHeld != 1 {
		t.Fatalf("重送 = %+v, %v", r, err)
	}
	_, err = f.m.Issue(context.Background(), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: []int64{buyer3, 1004}, AdminID: adminID})
	wantReason(t, err, coupon.ReasonIssueLimitReached)
	if got := f.issued(c.ID); got != 2 {
		t.Fatalf("issued_count = %d，要 2", got)
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1", c.ID); got != 2 {
		t.Fatalf("券 %d 張，要 2", got)
	}
	if got := f.scalar("SELECT count(*) FROM coupon.coupon_target_accounts WHERE campaign_id = $1", c.ID); got != 2 {
		t.Fatalf("名單 %d 人，要 2（失敗的發放不能留下名單）", got)
	}
	v := f.voucherOf(c, buyer1)
	if r := f.mustLock(v, "O1"); r.Discount != 100 {
		t.Fatalf("指定發放的券要能鎖：%+v", r)
	}

	all := f.campaign()
	_, err = f.m.Issue(context.Background(), coupon.IssueInput{CampaignID: all.ID, BuyerIDs: []int64{buyer1}, AdminID: adminID})
	wantReason(t, err, coupon.ReasonNotDesignated)
}

// 並發的指定發放與領券都經活動列的鎖排隊，總量不被超過。
func TestConcurrentIssueAndClaimRespectTheLimit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) { in.TargetType, in.TotalLimit = coupon.TargetDesignated, 4 })
	errs := parallel(10, func(i int) error {
		b := 3000 + int64(i)
		if i%2 == 0 {
			_, err := f.m.Issue(context.Background(), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: []int64{b}, AdminID: adminID})
			return err
		}
		if _, err := f.m.Issue(context.Background(), coupon.IssueInput{CampaignID: c.ID, BuyerIDs: []int64{b - 1000}, AdminID: adminID}); err != nil {
			return err
		}
		_, err := f.m.Claim(context.Background(), c.ID, b-1000)
		return err
	})
	for _, err := range errs {
		if err != nil {
			wantReason(t, err, coupon.ReasonIssueLimitReached)
		}
	}
	if got := f.issued(c.ID); got > 4 || got != f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1", c.ID) {
		t.Fatalf("issued_count = %d，券數 %d", got, f.scalar("SELECT count(*) FROM coupon.coupon_vouchers WHERE campaign_id = $1", c.ID))
	}
}

// admin 作廢單張券：卡在 LOCKED 的券（Release 漏送）可以作廢，之後那筆訂單沒有券可核銷；已核銷的不能作廢。
func TestVoidVoucherClearsAStuckLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	v := f.claim(f.campaign(), buyer1)
	f.mustLock(v, "O1")
	if err := f.m.VoidVoucher(context.Background(), coupon.VoidVoucherInput{VoucherID: v.ID, AdminID: adminID, Reason: " 漏送釋放 "}); err != nil {
		t.Fatal(err)
	}
	if got := f.voucherStatus(v.ID); got != "VOIDED" {
		t.Fatalf("狀態 = %s", got)
	}
	_, err := f.m.Redeem(context.Background(), "O1")
	wantReason(t, err, coupon.ReasonNoVoucherForOrder)
	_, err = f.lock(v, "O2")
	wantReason(t, err, coupon.ReasonVoucherUnavailable)
	if err := f.m.VoidVoucher(context.Background(), coupon.VoidVoucherInput{VoucherID: v.ID, AdminID: adminID}); coupon.ReasonOf(err) != coupon.ReasonInvalidStatus {
		t.Fatalf("重複作廢 = %v", err)
	}
	var reason string
	if err := f.db.Pool().QueryRow(context.Background(), "SELECT reason FROM coupon.coupon_redemption_logs WHERE voucher_id = $1 AND action = 'VOID'", v.ID).Scan(&reason); err != nil || reason != "漏送釋放" {
		t.Fatalf("稽核理由 = %q, %v", reason, err)
	}

	w := f.claim(f.campaign(), buyer2)
	f.mustLock(w, "O3")
	if _, err := f.m.Redeem(context.Background(), "O3"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.VoidVoucher(context.Background(), coupon.VoidVoucherInput{VoucherID: w.ID, AdminID: adminID}); coupon.ReasonOf(err) != coupon.ReasonInvalidStatus {
		t.Fatalf("作廢已核銷的券 = %v", err)
	}
}

// D8 修訂：賣家發行的活動在發行店家停權期間不能領、不能鎖，解除後恢復；平台活動不受影響；已鎖的券停權期間照常核銷。
func TestSuspendedStoreBlocksClaimAndLockButNotRedeem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	seller := f.campaign(func(in *coupon.CreateCampaignInput) { in.Issuer = coupon.Issuer{StoreID: storeA} })
	platform := f.campaign()
	held := f.claim(seller, buyer3)
	locked := f.mustLock(f.claim(seller, buyer2), "O-LOCKED")

	f.stores.set(storeA, true)
	_, err := f.m.Claim(context.Background(), seller.ID, buyer1)
	wantReason(t, err, coupon.ReasonStoreSuspended)
	_, err = f.lock(held, "O1")
	wantReason(t, err, coupon.ReasonStoreSuspended)
	if got := f.voucherStatus(held.ID); got != "AVAILABLE" {
		t.Fatalf("被擋下的券狀態 = %s，要維持 AVAILABLE", got)
	}
	f.claim(platform, buyer1)

	if r, err := f.m.Redeem(context.Background(), "O-LOCKED"); err != nil || r.VoucherID != locked.Voucher.ID {
		t.Fatalf("停權期間已鎖的券要能核銷：%+v, %v", r, err)
	}

	f.stores.set(storeA, false)
	f.claim(seller, buyer1)
	f.mustLock(held, "O1")
}

// 同一筆訂單在兩個 transaction 各鎖一張不同的券：後到的在 UPDATE 卡在唯一鍵上，先到的 commit 後它撞上 vouchers_order_no_key，
// 要回 ORDER_HOLDS_ANOTHER_VOUCHER（不是 Internal），而且它所在的 transaction 仍可使用、可以 commit（衝突發生在 savepoint 內）。
// 時序由 pg_locks 決定：先到的 commit 之前，等到後到的確實卡在鎖上，兩邊的預先檢查都已通過。
func TestConcurrentLockOfTwoVouchersForOneOrder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign()
	v1, v2 := f.claim(c, buyer1), f.claim(c, buyer2)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const order = "O-SAME"

	// 失敗路徑也要放行先到的 transaction：連線池要等它歸還連線才關得掉，否則測試結束時會卡住。
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFirst := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseFirst()

	firstLocked := make(chan error, 1)
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			_, err := f.m.Lock(ctx, f.lockInput(v1, order))
			firstLocked <- err
			if err != nil {
				return err
			}
			<-release
			return nil
		})
	}()
	select {
	case err := <-firstLocked:
		if err != nil {
			t.Fatalf("先到的鎖券：%v", err)
		}
	case <-ctx.Done():
		t.Fatal("先到的 transaction 在 30 秒內沒有鎖到券")
	}

	var secondLockErr error
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			_, secondLockErr = f.m.Lock(ctx, f.lockInput(v2, order))
			_, err := f.db.Conn(ctx).Exec(ctx, "SELECT 1")
			return err
		})
	}()

	// 只看本測試 database 裡卡在 transactionid 鎖上的連線（每支測試各有自己的 database，pg_locks 卻是整個 cluster 共用）。
	const blocked = `SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a USING (pid)
		WHERE NOT l.granted AND l.locktype = 'transactionid' AND a.datname = current_database()`
	for {
		var n int64
		if err := f.db.Pool().QueryRow(ctx, blocked).Scan(&n); err != nil {
			t.Fatalf("查詢 pg_locks：%v", err)
		}
		if n > 0 {
			break
		}
		select {
		case err := <-secondDone:
			t.Fatalf("後到的 transaction 沒有卡在唯一鍵的鎖上就結束了（預先檢查已先擋下？）：%v、鎖定結果 %v", err, secondLockErr)
		case <-ctx.Done():
			t.Fatal("後到的 transaction 沒有在 30 秒內卡在唯一鍵的鎖上")
		case <-time.After(5 * time.Millisecond):
		}
	}
	releaseFirst()

	for _, w := range []struct {
		name string
		ch   chan error
	}{{"先到的 transaction", firstDone}, {"後到的 transaction 在失敗的鎖定之後要仍可使用並 commit", secondDone}} {
		select {
		case err := <-w.ch:
			if err != nil {
				t.Fatalf("%s：%v", w.name, err)
			}
		case <-ctx.Done():
			t.Fatalf("%s：30 秒內沒有結束", w.name)
		}
	}
	wantReason(t, secondLockErr, coupon.ReasonOrderHoldsAnotherVoucher)
	if f.voucherStatus(v1.ID) != "LOCKED" || f.voucherStatus(v2.ID) != "AVAILABLE" {
		t.Fatalf("v1 = %s、v2 = %s，要 LOCKED 與 AVAILABLE", f.voucherStatus(v1.ID), f.voucherStatus(v2.ID))
	}
}

// 已由這筆訂單持有的券，重送 Lock 與 Quote 不因折抵算出 0 元而被拒絕（零折抵只擋新的鎖定）；券仍由該訂單持有。
func TestReplayOfHeldVoucherIgnoresZeroDiscount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) {
		in.DiscountType, in.DiscountAmount, in.DiscountPercent, in.MaxDiscountAmount = coupon.DiscountPercent, 0, 1, 100
	})
	v := f.claim(c, buyer1)
	if r := f.mustLock(v, "O1"); r.Discount != 10 || r.Replayed {
		t.Fatalf("1000 元的 1%% 首次鎖定 = %+v", r)
	}
	in := f.lockInput(v, "O1")
	in.OrderAmount = 50
	r, err := f.m.Lock(context.Background(), in)
	if err != nil || !r.Replayed || r.Discount != 0 {
		t.Fatalf("重送 Lock = %+v, %v，要 Replayed 且折抵 0", r, err)
	}
	q, err := f.m.Quote(context.Background(), in)
	if err != nil || !q.Replayed || q.Discount != 0 {
		t.Fatalf("Quote = %+v, %v，要 Replayed 且折抵 0", q, err)
	}
	if got := f.voucherStatus(v.ID); got != "LOCKED" {
		t.Fatalf("券狀態 = %s，要維持 LOCKED", got)
	}
	other := f.claim(c, buyer2)
	if _, err := f.m.Quote(context.Background(), coupon.LockInput{VoucherID: other.ID, BuyerID: buyer2, OrderNo: "O2", StoreID: storeA, OrderAmount: 50}); err == nil {
		t.Fatal("沒有持有的券 Quote 出 0 元折抵要被拒絕")
	} else {
		wantReason(t, err, coupon.ReasonNoDiscount)
	}
}

// 百分比折抵算出 0 元時不鎖券（不消耗券換不到折扣）。
func TestLockRejectsAPercentDiscountThatRoundsToZero(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) {
		in.DiscountType, in.DiscountAmount, in.DiscountPercent, in.MaxDiscountAmount = coupon.DiscountPercent, 0, 1, 100
	})
	v := f.claim(c, buyer1)
	in := f.lockInput(v, "O1")
	in.OrderAmount = 50
	_, err := f.m.Lock(context.Background(), in)
	wantReason(t, err, coupon.ReasonNoDiscount)
	if got := f.voucherStatus(v.ID); got != "AVAILABLE" {
		t.Fatalf("券狀態 = %s，要維持 AVAILABLE", got)
	}
	in.OrderAmount = 1000
	if r, err := f.m.Lock(context.Background(), in); err != nil || r.Discount != 10 {
		t.Fatalf("1000 元的 1%% = %+v, %v", r, err)
	}
}

// 長度以字元計（與 DB 的 char_length 一致）：128 個中文字的單號合法，129 個不合法；Issue 失敗時不回半成品。
func TestLengthsCountRunesAndIssueFailureReturnsNoResult(t *testing.T) {
	t.Parallel()
	f := setup(t)
	v := f.claim(f.campaign(), buyer1)
	if _, err := f.lock(v, strings.Repeat("單", coupon.MaxOrderNoLen)); err != nil {
		t.Fatalf("128 個字元的單號要成功：%v", err)
	}
	if _, err := f.lock(f.claim(f.campaign(), buyer2), strings.Repeat("單", coupon.MaxOrderNoLen+1)); err == nil {
		t.Fatal("129 個字元的單號要被擋下")
	}

	d := f.campaign(func(in *coupon.CreateCampaignInput) { in.TargetType, in.TotalLimit = coupon.TargetDesignated, 1 })
	r, err := f.m.Issue(context.Background(), coupon.IssueInput{CampaignID: d.ID, BuyerIDs: []int64{buyer1, buyer2}, AdminID: adminID})
	wantReason(t, err, coupon.ReasonIssueLimitReached)
	if r != (coupon.IssueResult{}) {
		t.Fatalf("失敗的 Issue 要回空結果：%+v", r)
	}
}
