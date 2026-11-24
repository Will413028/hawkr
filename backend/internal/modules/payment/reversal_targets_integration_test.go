//go:build integration

package payment_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
)

// 一次向賣家收回可以跨多個 provider 轉帳：原轉帳先、成功的補差額轉帳後（依退款 ID 升序），每一次 provider 撤回呼叫一列 refund_reversals，各自收斂。

// topupDone 建立已入帳 100 的付款：轉帳成立 70（送出前有一筆 30 的退款算進去），那筆退款失敗、補差額 30 成功（DONE），賣家共收到 70 + 30 = 100。
func (f *chargeFixture) topupDone(t *testing.T, orderNo string) (PaymentTransfer, PaymentRefund) {
	t.Helper()
	tr, r := f.topupSending(t, orderNo)
	must(t, f.module.TopUpPaymentTransfer(context.Background(), r.ID, false))
	if f.topupOf(t, r.ID) != "DONE" {
		t.Fatalf("前置：補差額 %s", f.topupOf(t, r.ID))
	}
	return tr, r
}

type segRow struct {
	Seq    int
	Target string
	Ext    string
	Amount int64
	Status string
}

func (f *chargeFixture) segments(t *testing.T, refundID interface{ String() string }) []segRow {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT seq, COALESCE(target_refund_id::text, ''), target_external_id, amount, status FROM payment.refund_reversals WHERE refund_id = $1::uuid ORDER BY seq`, refundID.String())
	must(t, err)
	defer rows.Close()
	var out []segRow
	for rows.Next() {
		var r segRow
		must(t, rows.Scan(&r.Seq, &r.Target, &r.Ext, &r.Amount, &r.Status))
		out = append(out, r)
	}
	must(t, rows.Err())
	return out
}

// 補差額成功之後，全額退款收回的是原轉帳加補差額轉帳（賣家淨收 0）：兩列撤回列，目標各自是原轉帳與補差額轉帳，額度 70 與 30；兩個目標的累計撤回額各自累加，彙總進 REVERSED。
// mutation：reservableReversal 不加 DoneTopupCapacity（上限只算原轉帳）→ 預留只有 70，失敗。
// mutation：startReversals 只分給第一個目標（迴圈在第一個有額度的目標之後 break）→ 只有一列、補差額轉帳沒被收回，失敗。
func TestAFullRefundAfterATopUpRecoversTheOriginalTransferAndTheTopUpExactly(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	tr, topup := f.topupDone(t, "RT-1")
	topupExt := f.refundText(t, "topup_external_id", topup.ID)
	r := f.createRefund(t, "RT-1", "full", 100, false)
	if r.ReversalAmount != 100 || r.ReversalStatus != ReversalWaiting {
		t.Fatalf("預留沒有含補差額轉帳：%+v", r)
	}
	f.clearJobs()
	must(t, f.runRefund(r.ID, false))
	want := []segRow{
		{Seq: 0, Target: "", Ext: f.transferText(t, "external_transfer_id", tr.ID), Amount: 70, Status: "REVERSING"},
		{Seq: 1, Target: topup.ID.String(), Ext: topupExt, Amount: 30, Status: "REVERSING"},
	}
	if got := f.segments(t, r.ID); !slices.Equal(got, want) {
		t.Fatalf("撤回列=%+v，期望 %+v", got, want)
	}
	if ids := f.jobIDs(t, kindTransferReversal, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("撤回 job=%v", ids)
	}
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.reversalOf(t, r.ID) != "REVERSED" || f.refundText(t, "reversal_recovered", r.ID) != "100" || f.reversedOf(t, tr.ID) != 70 || f.refundText(t, "topup_reversed_amount", topup.ID) != "30" || f.fake.ReversalCalls() != 2 {
		t.Fatalf("撤回之後：%s，recovered %s，原轉帳 %d，補差額 %s，呼叫 %d", f.reversalOf(t, r.ID), f.refundText(t, "reversal_recovered", r.ID), f.reversedOf(t, tr.ID), f.refundText(t, "topup_reversed_amount", topup.ID), f.fake.ReversalCalls())
	}
	if got := f.fake.ReversalIDs(); len(got) != 2 {
		t.Fatalf("provider 的撤回=%v", got)
	}
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.fake.ReversalCalls() != 2 || len(f.anomalies) != 0 {
		t.Fatalf("重跑之後：呼叫 %d，事件 %+v", f.fake.ReversalCalls(), f.anomalies)
	}
	f.noViolations(t)
}

// 還沒成功的補差額轉帳不算進上限：補差額在 SENDING 時全額退款只預留原轉帳的 70；補差額轉帳只有 DONE 才能被撤回。
// mutation：DoneTopupCapacity 拿掉 topup_status = 'DONE' 的過濾 → 預留 100，失敗。
func TestATopUpThatIsNotDoneIsNotCountedInTheReversalCap(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	_, topup := f.topupSending(t, "RT-2")
	r := f.createRefund(t, "RT-2", "full", 100, false)
	if r.ReversalAmount != 70 || f.topupOf(t, topup.ID) != "SENDING" {
		t.Fatalf("預留=%d，補差額 %s", r.ReversalAmount, f.topupOf(t, topup.ID))
	}
	must(t, f.runRefund(r.ID, false))
	if got := f.segments(t, r.ID); len(got) != 1 || got[0].Seq != 0 || got[0].Amount != 70 || got[0].Target != "" {
		t.Fatalf("撤回列=%+v", got)
	}
}

// 一列撤回結果不明不擋另一列：原轉帳那列回應遺失（provider 已撤回）、補差額那列照常成立；過了視窗只查回，找到就收下，不再呼叫 provider，彙總收斂成 REVERSED。
// mutation：reverse() 在第一列失敗時就 return（不繼續處理其他列）→ 補差額那列沒被呼叫，第一段失敗。
// mutation：reverseSegment 過了視窗仍呼叫 ReverseTransfer → 呼叫次數增加，失敗。
func TestAnUnknownOutcomeOfOneSegmentConvergesByLookupWithoutBlockingTheOther(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	tr, topup := f.topupDone(t, "RT-3")
	r := f.createRefund(t, "RT-3", "full", 100, false)
	must(t, f.runRefund(r.ID, false))
	f.fake.FailReversalAfterCreate(errLost)
	if err := f.module.ReversePaymentTransfer(ctx, r.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	got := f.segments(t, r.ID)
	if len(got) != 2 || got[0].Status != "REVERSING" || got[1].Status != "REVERSED" || f.fake.ReversalCalls() != 2 || f.reversalOf(t, r.ID) != "REVERSING" {
		t.Fatalf("第一輪之後：%+v，呼叫 %d，彙總 %s", got, f.fake.ReversalCalls(), f.reversalOf(t, r.ID))
	}
	f.now = f.now.Add(PaymentResendWindow + time.Minute)
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.reversalOf(t, r.ID) != "REVERSED" || f.fake.ReversalCalls() != 2 || f.reversedOf(t, tr.ID) != 70 || f.refundText(t, "topup_reversed_amount", topup.ID) != "30" || f.fake.LookupCalls() != 1 {
		t.Fatalf("查回之後：%s，呼叫 %d，原轉帳 %d，查回 %d 次", f.reversalOf(t, r.ID), f.fake.ReversalCalls(), f.reversedOf(t, tr.ID), f.fake.LookupCalls())
	}
}

// 撤回列各自進 ANOMALY：補差額那列查回連續失敗進 ANOMALY，彙總是 ANOMALY；原轉帳那列不受影響；
// 人工結案這一列（撤回列 ID）成功要核對 provider 證據，成立之後累加補差額轉帳的撤回額、彙總進 REVERSED。
// mutation：resolveReversal 成功時不累加 AddTopupReversal → 補差額轉帳的撤回額仍是 0，失敗。
func TestASegmentAnomalyHoldsTheReservationAndIsResolvedPerSegment(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	_, topup := f.topupDone(t, "RT-4")
	r := f.createRefund(t, "RT-4", "full", 100, false)
	must(t, f.runRefund(r.ID, false))
	f.fake.FailReversalAfterCreate(nil, errLost)
	if err := f.module.ReversePaymentTransfer(ctx, r.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	f.now = f.now.Add(PaymentResendWindow + time.Minute)
	boom := errors.New("查回逾時")
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f.fake.FailLookup(boom)
		_ = f.module.ReversePaymentTransfer(ctx, r.ID, false)
	}
	got := f.segments(t, r.ID)
	if len(got) != 2 || got[0].Status != "REVERSED" || got[1].Status != "ANOMALY" || f.reversalOf(t, r.ID) != "ANOMALY" || f.refundText(t, "reversal_recovered", r.ID) != "70" || len(f.anomalies) == 0 {
		t.Fatalf("ANOMALY 之後：%+v，彙總 %s，recovered %s", got, f.reversalOf(t, r.ID), f.refundText(t, "reversal_recovered", r.ID))
	}
	segID := f.text(t, "SELECT id::text FROM payment.refund_reversals WHERE refund_id = $1 AND seq = 1", r.ID)
	ids := f.fake.ReversalIDs()
	if len(ids) != 2 {
		t.Fatalf("provider 的撤回=%v", ids)
	}
	evidence := ""
	for _, id := range ids {
		if id != f.text(t, "SELECT external_id FROM payment.refund_reversals WHERE refund_id = $1 AND seq = 0", r.ID) {
			evidence = id
		}
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyReversal, uuid.MustParse(segID), AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", evidence))
	if f.reversalOf(t, r.ID) != "REVERSED" || f.refundText(t, "reversal_recovered", r.ID) != "100" || f.refundText(t, "topup_reversed_amount", topup.ID) != "30" || f.auditCount(t) != 1 {
		t.Fatalf("結案之後：%s，recovered %s，補差額撤回 %s", f.reversalOf(t, r.ID), f.refundText(t, "reversal_recovered", r.ID), f.refundText(t, "topup_reversed_amount", topup.ID))
	}
}

// 併發：兩筆退款合計 100 同時開始收回，不管誰先，原轉帳與補差額轉帳各自的累計撤回額不超過各自的額度（70、30），總額恰好 100，provider 端恰好建立 3 筆撤回（同一個撤回列被兩個 worker 同時處理時以同一把冪等鍵重送，provider 回同一筆，所以數建立的撤回而不是呼叫次數）；
// 目標累計撤回額由 CHECK 守，超過額度的寫入被擋下。
// mutation：startReversals 不扣進行中撤回列佔走的額度（OpenReversalTotalOnTransfer 回 0）→ 兩筆退款都從原轉帳分 70，超過原轉帳額度，失敗。
// mutation：拿掉 refunds_topup_reversed_check → 超過額度的寫入沒被擋下，失敗。
func TestConcurrentRefundsNeverReverseMoreThanEachTransferOrTheTotal(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	tr, topup := f.topupDone(t, "RT-5")
	a := f.createRefund(t, "RT-5", "a", 60, false)
	b := f.createRefund(t, "RT-5", "b", 40, false)
	if a.ReversalAmount != 60 || b.ReversalAmount != 40 {
		t.Fatalf("預留 %d %d", a.ReversalAmount, b.ReversalAmount)
	}
	run := func(fns ...func() error) {
		t.Helper()
		errs := make(chan error, len(fns))
		var wg sync.WaitGroup
		for _, fn := range fns {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- fn()
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			must(t, err)
		}
	}
	run(
		func() error { return f.runRefund(a.ID, false) },
		func() error { return f.runRefund(b.ID, false) },
	)
	run(
		func() error { return f.module.ReversePaymentTransfer(ctx, a.ID, false) },
		func() error { return f.module.ReversePaymentTransfer(ctx, b.ID, false) },
		func() error { return f.module.ReversePaymentTransfer(ctx, a.ID, false) },
		func() error { return f.module.ReversePaymentTransfer(ctx, b.ID, false) },
	)
	if f.reversedOf(t, tr.ID) != 70 || f.refundText(t, "topup_reversed_amount", topup.ID) != "30" || f.reversalOf(t, a.ID) != "REVERSED" || f.reversalOf(t, b.ID) != "REVERSED" || len(f.fake.ReversalIDs()) != 3 {
		t.Fatalf("併發之後：原轉帳 %d，補差額 %s，彙總 %s %s，provider 撤回 %d", f.reversedOf(t, tr.ID), f.refundText(t, "topup_reversed_amount", topup.ID), f.reversalOf(t, a.ID), f.reversalOf(t, b.ID), len(f.fake.ReversalIDs()))
	}
	if f.count(t, "SELECT COALESCE(SUM(amount), 0) FROM payment.refund_reversals WHERE payment_order_id = $1 AND status = 'REVERSED'", tr.PaymentOrderID) != 100 {
		t.Fatal("撤回列的總額不是 100")
	}
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.refunds SET topup_reversed_amount = amount + 1 WHERE id = $1", topup.ID)
	if err == nil {
		t.Fatal("補差額轉帳的累計撤回額超過額度卻沒被擋下")
	}
	f.noViolations(t)
}

// 撤回列的額度超過補差額轉帳的剩餘可撤額度（另一邊已經撤回了一部分）：不呼叫 provider、這一列進 ANOMALY（原因 reversal_exceeds_remaining），原轉帳那列照常成立；
// 彙總是 ANOMALY，預留持續佔住。
// mutation：checkSegmentTarget 的補差額剩餘額度不扣 topup_reversed_amount → 超額的撤回照樣呼叫 provider，失敗。
func TestASegmentBeyondTheTopUpRemainingGoesToAnomalyWithoutCallingTheProvider(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	tr, topup := f.topupDone(t, "RT-6")
	r := f.createRefund(t, "RT-6", "full", 100, false)
	must(t, f.runRefund(r.ID, false))
	f.exec(t, "UPDATE payment.refunds SET topup_reversed_amount = 10 WHERE id = $1", topup.ID)
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	got := f.segments(t, r.ID)
	if len(got) != 2 || got[0].Status != "REVERSED" || got[1].Status != "ANOMALY" || f.fake.ReversalCalls() != 1 || f.reversedOf(t, tr.ID) != 70 {
		t.Fatalf("超額之後：%+v，呼叫 %d，原轉帳 %d", got, f.fake.ReversalCalls(), f.reversedOf(t, tr.ID))
	}
	if f.reversalOf(t, r.ID) != "ANOMALY" || f.refundText(t, "reversal_reason", r.ID) != "reversal_exceeds_remaining" || f.refundText(t, "topup_reversed_amount", topup.ID) != "10" {
		t.Fatalf("彙總 %s，原因 %s", f.reversalOf(t, r.ID), f.refundText(t, "reversal_reason", r.ID))
	}
}

// 彙總以最重的撤回列為準：原轉帳那列因目標剩餘額度不足進 ANOMALY，補差額那列暫時性失敗還在 REVERSING 時，彙總是 ANOMALY（預留持續佔住）；
// 補差額那列重試成立之後彙總仍是 ANOMALY；人工把異常那列結案成失敗（RELEASED）之後，所有列終局，彙總是 RELEASED（原因 admin_resolved），已成立的 30 留在 reversal_recovered。
// mutation：SettleRefundReversal 把 REVERSING 排在 ANOMALY 之前 → 第一段彙總是 REVERSING，失敗。
func TestTheRefundAggregateIsAnomalyWhileAnotherSegmentIsStillReversing(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	tr, topup := f.topupDone(t, "RT-7")
	r := f.createRefund(t, "RT-7", "full", 100, false)
	must(t, f.runRefund(r.ID, false))
	f.exec(t, "UPDATE payment.transfers SET reversed_amount = 50 WHERE id = $1", tr.ID)
	f.fake.FailReversal(errors.New("連線逾時"))
	if err := f.module.ReversePaymentTransfer(ctx, r.ID, false); err == nil {
		t.Fatal("補差額那列暫時性失敗沒有回錯誤")
	}
	got := f.segments(t, r.ID)
	if len(got) != 2 || got[0].Status != "ANOMALY" || got[1].Status != "REVERSING" || f.reversalOf(t, r.ID) != "ANOMALY" || f.refundText(t, "reversal_reason", r.ID) != "reversal_exceeds_remaining" {
		t.Fatalf("第一輪之後：%+v，彙總 %s", got, f.reversalOf(t, r.ID))
	}
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.reversalOf(t, r.ID) != "ANOMALY" || f.refundText(t, "reversal_recovered", r.ID) != "30" || f.refundText(t, "topup_reversed_amount", topup.ID) != "30" {
		t.Fatalf("重試之後：%s，recovered %s", f.reversalOf(t, r.ID), f.refundText(t, "reversal_recovered", r.ID))
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyReversal, f.reversalID(t, r.ID), AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", ""))
	if f.reversalOf(t, r.ID) != "RELEASED" || f.refundText(t, "reversal_reason", r.ID) != "admin_resolved" || f.refundText(t, "reversal_recovered", r.ID) != "30" || f.reversedOf(t, tr.ID) != 50 {
		t.Fatalf("結案之後：%s %s，recovered %s", f.reversalOf(t, r.ID), f.refundText(t, "reversal_reason", r.ID), f.refundText(t, "reversal_recovered", r.ID))
	}
}
