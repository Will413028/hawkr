//go:build integration

package payment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const kindChargeClose = "payment.charge_close"

func (f *chargeFixture) closeOrder(t *testing.T, orderNo string) bool {
	t.Helper()
	queued, err := f.module.CloseOrderPayment(context.Background(), orderNo)
	must(t, err)
	return queued
}

// 訂單取消或逾期時關閉等待中的付款：排關閉 job，job 在 transaction 外向 provider 取消，結果照一般路徑寫成 CANCELED 並發 PaymentChanged。
func TestCloseOrderPaymentCancelsAnAwaitingIntentThroughAJob(t *testing.T) {
	f := setupCharge(t)
	f.order("O-CLOSE", 11, 7, 100)
	p := f.create(t, 11, "O-CLOSE")
	if !f.closeOrder(t, "O-CLOSE") || len(f.paymentIDs(t, kindChargeClose)) != 1 {
		t.Fatal("an awaiting payment must queue exactly one close job")
	}
	if f.statusOf(t, "O-CLOSE") != PaymentAwaitingPayment {
		t.Fatal("queueing must not change the payment: the provider is called by the job")
	}
	must(t, f.module.CloseOrderPaymentAttempt(context.Background(), p.ID))
	if f.statusOf(t, "O-CLOSE") != PaymentCanceled || f.changesTo(PaymentCanceled) != 1 {
		t.Fatalf("status=%s canceled changes=%d", f.statusOf(t, "O-CLOSE"), f.changesTo(PaymentCanceled))
	}
	if again := f.closeOrder(t, "O-CLOSE"); again {
		t.Fatal("a canceled payment needs no close")
	}
	f.noViolations(t)
}

func TestCloseOrderPaymentQueuesNothingWhenThereIsNothingToCancel(t *testing.T) {
	f := setupCharge(t)
	f.paidPayment(t, "O-PAID", 100)
	if f.closeOrder(t, "O-PAID") || f.closeOrder(t, "O-NO-PAYMENT") || len(f.paymentIDs(t, kindChargeClose)) != 0 {
		t.Fatal("a paid payment or an order without a payment must queue no close job")
	}
	if _, err := f.module.CloseOrderPayment(context.Background(), "  "); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("blank order number: %v", err)
	}
}

// 取消與買家付款競態：provider 已入帳時取消回入帳的快照，付款記成入帳（入帳勝出），訂單一方依訂單已取消發起自動退款。
func TestCloseJobThatLosesTheRaceAgainstAPaymentRecordsTheSuccess(t *testing.T) {
	f := setupCharge(t)
	f.order("O-RACE", 11, 7, 100)
	p := f.create(t, 11, "O-RACE")
	must(t, f.fake.Settle(f.externalOf(t, "O-RACE"), PaymentStateSucceeded, f.now))
	must(t, f.module.CloseOrderPaymentAttempt(context.Background(), p.ID))
	if f.statusOf(t, "O-RACE") != PaymentSucceeded || f.changesTo(PaymentSucceeded) != 1 {
		t.Fatalf("status=%s", f.statusOf(t, "O-RACE"))
	}
}

// 付款還在建立中（沒有 PaymentIntent）時關閉 job 等建立收斂：建立還沒成功就回錯誤（job snooze 重試、不放棄），收斂成等待付款之後同一次呼叫接著取消，不留下沒人取消的 PaymentIntent（D-G）。
// mutation：Close 在建立中直接放棄（回 nil）→ 付款留在 CREATING，這個測試失敗。
func TestCloseJobWaitsForACreatingPaymentToConvergeThenCancelsItWithoutAnOrphanIntent(t *testing.T) {
	f := setupCharge(t)
	f.order("O-WAIT", 11, 7, 100)
	f.fake.FailCreate(errors.New("provider 暫時不可用"))
	p := f.create(t, 11, "O-WAIT")
	if p.Status != PaymentCreating || !f.closeOrder(t, "O-WAIT") {
		t.Fatalf("status=%s", p.Status)
	}
	f.fake.FailCreate(errors.New("provider 還是不可用"))
	if err := f.module.CloseOrderPaymentAttempt(context.Background(), p.ID); err == nil {
		t.Fatal("a creating payment must make the job wait, not finish")
	}
	if f.statusOf(t, "O-WAIT") != PaymentCreating {
		t.Fatal("waiting must not change the payment")
	}
	must(t, f.module.CloseOrderPaymentAttempt(context.Background(), p.ID))
	if f.statusOf(t, "O-WAIT") != PaymentCanceled || f.fake.Intents() != 1 {
		t.Fatalf("status=%s intents=%d, want the created intent canceled", f.statusOf(t, "O-WAIT"), f.fake.Intents())
	}
	f.noViolations(t)
}

func TestLegacyReadsByOrderNoAndRequestKey(t *testing.T) {
	f := setupCharge(t)
	ctx := context.Background()
	f.paidPayment(t, "O-A", 500)
	f.paidPayment(t, "O-B", 300)
	f.order("O-C", 12, 7, 100)
	f.create(t, 12, "O-C")

	one, err := f.module.PaymentByOrderNo(ctx, " O-A ")
	must(t, err)
	if one.Order.OrderNo != "O-A" || one.Order.Amount != 500 || one.Amount != 500 || one.Status != PaymentSucceeded || one.ClientSecret != "" {
		t.Fatalf("payment = %+v", one)
	}
	if _, err := f.module.PaymentByOrderNo(ctx, "O-NONE"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("missing payment: %v", err)
	}
	many, err := f.module.PaymentsByOrderNos(ctx, []string{"O-A", "O-C", "O-NONE", "O-A"})
	must(t, err)
	if len(many) != 2 {
		t.Fatalf("batch = %d payments, want the 2 that exist (missing and duplicate keys are not rows)", len(many))
	}
	if _, err := f.module.PaymentsByOrderNos(ctx, []string{"O-A", " "}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("blank key: %v", err)
	}

	first := f.createRefund(t, "O-A", "C1-0", 100, false)
	if first.OrderNo != "O-A" {
		t.Fatalf("CreatePaymentRefund must return the order number: %+v", first)
	}
	f.now = f.now.Add(time.Minute)
	second := f.createRefund(t, "O-A", "C2-0", 50, true)
	f.createRefund(t, "O-B", "C1-0", 30, false)

	got, err := f.module.RefundByRequestKey(ctx, "C2-0")
	must(t, err)
	if got.ID != second.ID || got.OrderNo != "O-A" || !got.Auto {
		t.Fatalf("refund by key = %+v", got)
	}
	if _, err := f.module.RefundByRequestKey(ctx, "nope"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("missing refund: %v", err)
	}
	// 鍵只在同一筆付款內唯一：同一把鍵對到兩筆付款的退款時不猜是哪一筆。
	if _, err := f.module.RefundByRequestKey(ctx, "C1-0"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("ambiguous key: %v", err)
	}
	byKeys, err := f.module.RefundsByRequestKeys(ctx, []string{"C1-0", "C2-0", "C9-0"})
	must(t, err)
	if len(byKeys) != 3 {
		t.Fatalf("by keys = %d rows, want 3 (C1-0 on two payments plus C2-0)", len(byKeys))
	}
	latest, err := f.module.LatestRefundsByOrderNos(ctx, []string{"O-A", "O-B", "O-C"})
	must(t, err)
	byOrder := map[string]PaymentRefund{}
	for _, r := range latest {
		byOrder[r.OrderNo] = r
	}
	if len(byOrder) != 2 || byOrder["O-A"].ID != second.ID || byOrder["O-B"].RequestKey != "C1-0" {
		t.Fatalf("latest by order = %+v", byOrder)
	}
}
