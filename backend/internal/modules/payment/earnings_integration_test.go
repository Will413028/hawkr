//go:build integration

package payment_test

import (
	"context"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)


func (f *chargeFixture) paidTransferred(t *testing.T, orderNo string, storeID, amount int64) PaymentTransfer {
	t.Helper()
	f.order(orderNo, 11, storeID, amount)
	p := f.create(t, 11, orderNo)
	must(t, f.fake.Settle(f.externalOf(t, orderNo), PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	f.enableAccount(t, storeID, "acct_seller_"+orderNo, true)
	tr := f.requestTransfer(t, orderNo)
	must(t, f.runTransfer(tr.ID, false))
	return tr
}

func earningsByType(entries []EarningsEntry) map[EarningsType]EarningsEntry {
	out := map[EarningsType]EarningsEntry{}
	for _, e := range entries {
		out[e.Type] = e
	}
	return out
}

// 帳本算術：轉入為正、收回為負、補差額為正，淨額只加總已完成的列。
// mutation：收回列的金額不取負（SellerListEarnings 的 (-v.amount) 改成 v.amount）→ 收回是 +30、淨額 130，失敗。
// mutation：補差額列從聯集拿掉 → 沒有 TOP_UP 列、淨額少 30，失敗。
func TestTheEarningsLedgerSignsTransfersReversalsAndTopUps(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()

	p, _ := f.transferred(t, "E-1")
	f.insertReversal(t, p.ID, "REFUNDING", "REVERSED", 50, 30)
	got, err := f.module.OrderEarnings(ctx, 7, "E-1")
	must(t, err)
	byType := earningsByType(got.Entries)
	if len(got.Entries) != 2 || byType[EarningsTransfer].Amount != 100 || byType[EarningsReversal].Amount != -30 ||
		byType[EarningsTransfer].Status != EarningsCompleted || byType[EarningsReversal].Status != EarningsCompleted || got.NetAmount != 70 {
		t.Fatalf("E-1：%+v 淨額 %d", got.Entries, got.NetAmount)
	}

	_, r := f.topupSending(t, "E-2")
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	got, err = f.module.OrderEarnings(ctx, 7, "E-2")
	must(t, err)
	byType = earningsByType(got.Entries)
	if len(got.Entries) != 2 || byType[EarningsTransfer].Amount != 70 || byType[EarningsTopUp].Amount != 30 || byType[EarningsTopUp].ID != r.ID ||
		byType[EarningsTopUp].Status != EarningsCompleted || got.NetAmount != 100 {
		t.Fatalf("E-2：%+v 淨額 %d", got.Entries, got.NetAmount)
	}
	f.noViolations(t)
}

// 進行中與失敗的列不計入淨額；沒有收回的 RELEASED 與沒有補差額的退款不出現。
func TestTheEarningsNetCountsOnlyCompletedEntries(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, _ := f.transferred(t, "E-3")
	f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 40, 40)
	f.insertReversal(t, p.ID, "FAILED", "RELEASED", 10, 10)
	got, err := f.module.OrderEarnings(ctx, 7, "E-3")
	must(t, err)
	if len(got.Entries) != 2 || got.NetAmount != 100 || earningsByType(got.Entries)[EarningsReversal].Status != EarningsPending {
		t.Fatalf("E-3：%+v 淨額 %d", got.Entries, got.NetAmount)
	}
}

// 授權：只回呼叫的店家的列。別家店讀這張訂單回 NotFound（與不存在的訂單不分），列表也看不到；有付款但還沒有轉帳的訂單是空列、淨額 0。
// mutation：SellerListEarnings 拿掉轉帳那一支的 store_id 條件 → 店家 8 看得到店家 7 的轉入，失敗。
// mutation：SellerHasOrderPayment 拿掉 store_id 條件 → 店家 8 讀 E-4 不是 NotFound，失敗。
func TestTheEarningsLedgerIsScopedToTheCallingStore(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.paidTransferred(t, "E-4", 7, 100)
	f.paidTransferred(t, "E-5", 8, 50)

	if _, err := f.module.OrderEarnings(ctx, 8, "E-4"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("別家店讀 E-4：%v", err)
	}
	if _, err := f.module.OrderEarnings(ctx, 7, "no-such-order"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的訂單：%v", err)
	}
	for storeID, want := range map[int64]string{7: "E-4", 8: "E-5"} {
		page, err := f.module.ListEarnings(ctx, storeID, EarningsFilter{}, nil, 100)
		must(t, err)
		if len(page.Entries) != 1 || page.Entries[0].OrderNo != want || page.Entries[0].Type != EarningsTransfer {
			t.Fatalf("店家 %d 的帳本：%+v", storeID, page.Entries)
		}
	}
	page, err := f.module.ListEarnings(ctx, 8, EarningsFilter{OrderNo: "E-4"}, nil, 100)
	must(t, err)
	if len(page.Entries) != 0 {
		t.Fatalf("別家店以訂單編號篩出了列：%+v", page.Entries)
	}

	f.paidPayment(t, "E-6", 100)
	got, err := f.module.OrderEarnings(ctx, 7, "E-6")
	must(t, err)
	if len(got.Entries) != 0 || got.NetAmount != 0 {
		t.Fatalf("還沒有轉帳的訂單：%+v", got)
	}
}

// 分頁與篩選：新的在前、每頁不重複不漏，時間篩選含起點不含終點，起點不早於終點回 InvalidInput。
func TestTheEarningsLedgerPagesNewestFirstAndFiltersByTime(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, _ := f.transferred(t, "E-7")
	f.insertReversal(t, p.ID, "REFUNDING", "REVERSED", 20, 20)
	f.insertReversal(t, p.ID, "REFUNDING", "REVERSED", 10, 10)

	var seen []EarningsEntry
	var after *RecordCursor
	for range 5 {
		page, err := f.module.ListEarnings(ctx, 7, EarningsFilter{}, after, 1)
		must(t, err)
		seen = append(seen, page.Entries...)
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	if len(seen) != 3 || seen[0].ID.String() <= seen[1].ID.String() || seen[1].ID.String() <= seen[2].ID.String() {
		t.Fatalf("分頁結果：%+v", seen)
	}

	from, to := f.now.Add(-time.Hour), f.now.Add(time.Hour)
	if page, err := f.module.ListEarnings(ctx, 7, EarningsFilter{From: &from, To: &to}, nil, 100); err != nil || len(page.Entries) == 0 {
		t.Fatalf("涵蓋現在的時間窗：%v %+v", err, page.Entries)
	}
	if page, err := f.module.ListEarnings(ctx, 7, EarningsFilter{To: &from}, nil, 100); err != nil || len(page.Entries) != 0 {
		t.Fatalf("終點在過去：%v %+v", err, page.Entries)
	}
	if _, err := f.module.ListEarnings(ctx, 7, EarningsFilter{From: &to, To: &from}, nil, 100); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("起點晚於終點：%v", err)
	}
}
