//go:build integration

package coupon_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

var errRollback = errors.New("rollback the outer transaction")

func (f *fixture) deleteBuyer(buyerID int64, fail bool) error {
	f.t.Helper()
	bus := events.NewBus()
	events.Subscribe(bus, f.m.OnBuyerDeleted)
	return f.db.InTx(context.Background(), func(ctx context.Context) error {
		if err := events.Publish(ctx, bus, buyer.BuyerDeleted{EventID: uuid.New(), BuyerID: buyerID, AccountID: buyerID}); err != nil {
			return err
		}
		if fail {
			return errRollback
		}
		return nil
	})
}

func (f *fixture) count(sql string, args ...any) int64 { return f.scalar(sql, args...) }

// 刪帳訂閱者與外層 transaction 同生死（D8、規則 4）：外層回滾，券、名單、投影、稽核都回到原樣；提交後 AVAILABLE 的券作廢、名單與投影清掉，
// LOCKED 與已核銷的券不動，別的買家不受影響，重放不再寫稽核。訂閱者改用 transaction 外的連線寫入時，回滾後變更仍在，這個測試變紅。
func TestBuyerDeletedSubscriberRunsInTheOuterTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)

	open := f.claim(f.campaign(), buyer1)
	other := f.claim(f.campaign(), buyer2)
	locked := f.claim(f.campaign(), buyer1)
	if _, err := f.m.Lock(context.Background(), f.lockInput(locked, "O-LOCKED")); err != nil {
		t.Fatal(err)
	}
	redeemed := f.claim(f.campaign(), buyer1)
	if _, err := f.m.Lock(context.Background(), f.lockInput(redeemed, "O-REDEEMED")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Redeem(context.Background(), "O-REDEEMED"); err != nil {
		t.Fatal(err)
	}
	f.campaign(func(in *coupon.CreateCampaignInput) {
		in.TargetType, in.TargetBuyers = coupon.TargetDesignated, []int64{buyer1, buyer2}
	})
	f.member(buyer1, storeA)
	f.member(buyer2, storeA)

	const (
		targets = "SELECT count(*) FROM coupon.coupon_target_accounts WHERE buyer_id = $1"
		members = "SELECT count(*) FROM coupon.store_buyer_membership WHERE buyer_id = $1"
		logs    = "SELECT count(*) FROM coupon.coupon_redemption_logs WHERE action = 'VOID' AND reason = 'BUYER_DELETED' AND buyer_id = $1"
	)
	if err := f.deleteBuyer(buyer1, true); !errors.Is(err, errRollback) {
		t.Fatalf("外層回滾：%v", err)
	}
	if got := f.voucherStatus(open.ID); got != "AVAILABLE" {
		t.Fatalf("外層回滾後券是 %s，要 AVAILABLE：訂閱者必須在外層 transaction 內寫入", got)
	}
	if f.count(targets, buyer1) != 1 || f.count(members, buyer1) != 1 || f.count(logs, buyer1) != 0 {
		t.Fatalf("外層回滾後名單 %d、投影 %d、稽核 %d，要 1、1、0", f.count(targets, buyer1), f.count(members, buyer1), f.count(logs, buyer1))
	}

	if err := f.deleteBuyer(buyer1, false); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		id   uuid.UUID
		want string
	}{"AVAILABLE": {open.ID, "VOIDED"}, "LOCKED": {locked.ID, "LOCKED"}, "REDEEMED": {redeemed.ID, "REDEEMED"}, "別人的": {other.ID, "AVAILABLE"}} {
		if got := f.voucherStatus(c.id); got != c.want {
			t.Errorf("%s 的券 = %s，要 %s", name, got, c.want)
		}
	}
	if f.count(targets, buyer1) != 0 || f.count(members, buyer1) != 0 {
		t.Fatalf("提交後名單 %d、投影 %d，要都清掉", f.count(targets, buyer1), f.count(members, buyer1))
	}
	if f.count(targets, buyer2) != 1 || f.count(members, buyer2) != 1 {
		t.Fatal("別的買家的名單與投影不該被動到")
	}
	if got := f.count(logs, buyer1); got != 1 {
		t.Fatalf("作廢稽核 %d 筆，要 1（只有 open 一張 AVAILABLE）", got)
	}

	if err := f.deleteBuyer(buyer1, false); err != nil {
		t.Fatalf("重放：%v", err)
	}
	if got := f.count(logs, buyer1); got != 1 {
		t.Fatalf("重放後稽核 %d 筆，要仍是 1", got)
	}
}

// MEMBER 活動可以由賣家建立；資格只在消費紀錄投影有列之後才成立。重放同一筆完成訂單冪等：只有一列，第二次回 recorded=false。
// 投影不寫入時資格永遠不成立，後半段變紅。
func TestMemberEligibilityFollowsTheProjection(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.sellerCampaign(storeA, func(in *coupon.CreateCampaignInput) { in.TargetType, in.StoreID = coupon.TargetMember, storeA })
	if c.TargetType != coupon.TargetMember {
		t.Fatalf("target = %s", c.TargetType)
	}
	_, err := f.m.Claim(context.Background(), c.ID, buyer1)
	wantReason(t, err, coupon.ReasonNotEligible)

	for i, want := range []bool{true, false} {
		got, err := f.m.RecordOrderCompleted(context.Background(), storeA, buyer1)
		if err != nil || got != want {
			t.Fatalf("第 %d 次記錄：recorded=%v err=%v，要 %v", i+1, got, err, want)
		}
	}
	if n := f.count("SELECT count(*) FROM coupon.store_buyer_membership WHERE store_id = $1 AND buyer_id = $2", storeA, buyer1); n != 1 {
		t.Fatalf("投影 %d 列，要 1", n)
	}
	f.claim(c, buyer1)
	_, err = f.m.Claim(context.Background(), c.ID, buyer2)
	wantReason(t, err, coupon.ReasonNotEligible)

	for name, args := range map[string][2]int64{"店家 0": {0, buyer1}, "買家 0": {storeA, 0}, "買家為負": {storeA, -1}} {
		if _, err := f.m.RecordOrderCompleted(context.Background(), args[0], args[1]); err == nil {
			t.Errorf("%s 要被擋下", name)
		}
	}
}
