//go:build integration

package payment_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 已算進轉帳金額的退款之後失敗，賣家少收了那筆錢，補一筆轉帳給賣家（D11 補充）：補差額的狀態記在退款列（topup_*），不是轉帳列，
// 所以不佔 transfers_active_payment、不動轉帳的 amount；結果依 D68 三分收斂。

const kindTransferTopup = "payment.transfer_topup"

var errRefundDeclined = fmt.Errorf("%w：卡片已失效", ErrChargePermanent)

// topupOf 讀退款列的補差額狀態。
func (f *chargeFixture) topupOf(t *testing.T, refundID interface{ String() string }) string {
	t.Helper()
	return f.text(t, "SELECT topup_status FROM payment.refunds WHERE id = $1::uuid", refundID.String())
}

// topupSending 建立一筆已入帳 100 的付款：轉帳送出前已有一筆 30 的退款（算進轉帳金額，賣家收 70），轉帳成立之後這筆退款被 provider 拒絕，補差額進 SENDING。
func (f *chargeFixture) topupSending(t *testing.T, orderNo string) (PaymentTransfer, PaymentRefund) {
	t.Helper()
	f.paidPayment(t, orderNo, 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, orderNo)
	r := f.createRefund(t, orderNo, "k", 30, false)
	must(t, f.runTransfer(tr.ID, false))
	f.fake.FailRefund(errRefundDeclined)
	f.clearJobs()
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "FAILED" || f.topupOf(t, r.ID) != "SENDING" {
		t.Fatalf("前置：退款 %s，補差額 %s", f.refundText(t, "status", r.ID), f.topupOf(t, r.ID))
	}
	return tr, r
}

// 已算進轉帳金額的退款失敗：賣家少收 30，補一筆 30 的轉帳（新的冪等鍵、同一個 charge），恰好一次；轉帳列不變（amount 70、沒有第二筆轉帳列、沒有撤回），
// 重跑 job 不再呼叫 provider。
// mutation：退款失敗的 SQL 不進補差額 WAITING（FailRefundCall 拿掉 topup_status 的寫入）→ 沒有補差額、賣家少收，失敗。
// mutation：補差額呼叫的金額改成 called.Amount + 1 → provider 收到的補差額金額不是 30，失敗。
func TestADeductedRefundThatFailsGetsExactlyOneTopUpTransferToTheSeller(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	tr, r := f.topupSending(t, "TU-1")
	if f.refundText(t, "transfer_deducted", r.ID) != "true" {
		t.Fatalf("退款沒有標成已算進轉帳金額：%s", f.refundText(t, "transfer_deducted", r.ID))
	}
	if ids := f.jobIDs(t, kindTransferTopup, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("補差額 job=%v", ids)
	}
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	if got := f.fake.TransferAmounts(); len(got) != 2 || got[0] != 70 || got[1] != 30 {
		t.Fatalf("provider 收到的轉帳金額=%v", got)
	}
	if keys := f.fake.TransferKeys(); len(keys) != 2 || keys[0] == keys[1] {
		t.Fatalf("補差額沒有用新的冪等鍵：%v", keys)
	}
	ext := f.refundText(t, "topup_external_id", r.ID)
	if f.topupOf(t, r.ID) != "DONE" || ext == "" || ext == f.transferText(t, "external_transfer_id", tr.ID) || !slices.Contains(f.fake.TransferIDs(), ext) || f.fake.Transfers() != 2 {
		t.Fatalf("補差額之後：%s，外部 ID %q，provider 的轉帳 %v", f.topupOf(t, r.ID), ext, f.fake.TransferIDs())
	}
	if f.count(t, "SELECT count(*) FROM payment.transfers WHERE payment_id = $1", tr.PaymentID) != 1 || f.transferText(t, "amount", tr.ID) != "70" || f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.reversedOf(t, tr.ID) != 0 {
		t.Fatalf("轉帳列被補差額改動：%s 金額 %s", f.transferText(t, "status", tr.ID), f.transferText(t, "amount", tr.ID))
	}
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	must(t, f.runRefund(r.ID, false))
	if f.fake.TransferCalls() != 2 || len(f.jobIDs(t, kindTransferTopup, "refund_id")) != 1 || len(f.anomalies) != 0 {
		t.Fatalf("重跑之後：provider 呼叫 %d 次，job %d 個，事件 %+v", f.fake.TransferCalls(), len(f.jobIDs(t, kindTransferTopup, "refund_id")), f.anomalies)
	}
	f.noViolations(t)
}

// 沒有算進轉帳金額的退款失敗不補：轉帳成立之後才建立的退款（錢本來就留在平台，賣家的轉帳金額沒扣它），失敗時沒有補差額、沒有 job、不呼叫 provider。
// mutation：退款失敗的 SQL 不看 transfer_deducted（FailRefundCall 的 WHEN transfer_deducted 改成 WHEN true）→ 補了 40 給賣家，失敗。
func TestARefundThatWasNotDeductedFromTheTransferNeedsNoTopUpWhenItFails(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	_, tr := f.transferred(t, "TU-2")
	r := f.createRefund(t, "TU-2", "k", 40, false)
	if f.refundText(t, "transfer_deducted", r.ID) != "false" {
		t.Fatalf("轉帳成立之後的退款被標成已算進轉帳金額")
	}
	f.fake.FailRefund(errRefundDeclined)
	f.clearJobs()
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "FAILED" || f.topupOf(t, r.ID) != "NONE" || len(f.jobIDs(t, kindTransferTopup, "refund_id")) != 0 {
		t.Fatalf("退款 %s，補差額 %s", f.refundText(t, "status", r.ID), f.topupOf(t, r.ID))
	}
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	if f.fake.TransferCalls() != 1 || f.transferText(t, "amount", tr.ID) != "100" || len(f.anomalies) != 0 {
		t.Fatalf("不該補差額：provider 呼叫 %d 次，事件 %+v", f.fake.TransferCalls(), f.anomalies)
	}
	f.noViolations(t)
}

// 轉帳結果不明時退款失敗：補差額等在 WAITING（還不知道賣家收到的是不是扣過的金額），轉帳成立的 transaction 才開始補差額並排 job。
// mutation：acceptTransfer 成功後不呼叫 startTopups → 轉帳成立後補差額仍在 WAITING、沒有 job，失敗。
func TestATopUpWaitsForTheTransferToSucceedBeforeItStarts(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.paidPayment(t, "TU-3", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "TU-3")
	r := f.createRefund(t, "TU-3", "k", 20, false)
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.runTransfer(tr.ID, false); err == nil || f.transferText(t, "status", tr.ID) != "PENDING" {
		t.Fatalf("前置：%v，轉帳 %s", err, f.transferText(t, "status", tr.ID))
	}
	f.fake.FailRefund(errRefundDeclined)
	f.clearJobs()
	must(t, f.runRefund(r.ID, false))
	if f.topupOf(t, r.ID) != "WAITING" || len(f.jobIDs(t, kindTransferTopup, "refund_id")) != 0 {
		t.Fatalf("轉帳還沒成立就開始補差額：%s", f.topupOf(t, r.ID))
	}
	if err := f.module.TopUpPaymentTransfer(ctx, r.ID, false); err != nil || f.fake.TransferCalls() != 1 {
		t.Fatalf("WAITING 的補差額被執行：%v，provider 呼叫 %d 次", err, f.fake.TransferCalls())
	}
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.topupOf(t, r.ID) != "SENDING" {
		t.Fatalf("轉帳成立之後：%s %s", f.transferText(t, "status", tr.ID), f.topupOf(t, r.ID))
	}
	if ids := f.jobIDs(t, kindTransferTopup, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("轉帳成立的 transaction 沒有排補差額 job：%v", ids)
	}
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	if got := f.fake.TransferAmounts(); len(got) != 3 || got[0] != 80 || got[1] != 80 || got[2] != 20 || f.fake.Transfers() != 2 || f.topupOf(t, r.ID) != "DONE" {
		t.Fatalf("補差額之後：金額 %v，provider 的轉帳 %d 筆，狀態 %s", got, f.fake.Transfers(), f.topupOf(t, r.ID))
	}
	f.noViolations(t)
}

// 轉帳換新鍵重新算定金額，已失敗的退款不在新的金額裡，賣家直接收到：補差額釋出（counted_in_new_amount），不補第二次（否則賣家多收 30）。
// mutation：claimTransfer 的重算分支不釋出 WAITING 的補差額 → 轉帳成立時補差額被排出去，賣家多收，失敗。
// mutation：claimTransfer 的重算分支不重寫 transfer_deducted → 失敗的退款仍標成已算進轉帳金額，下面的欄位檢查失敗。
func TestAKeyRotationReleasesTheTopUpOfARefundTheNewAmountNoLongerCounts(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.paidPayment(t, "TU-4", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "TU-4")
	r := f.createRefund(t, "TU-4", "k", 30, false)
	f.fake.FailTransfer(errInsufficientFunds)
	if err := f.runTransfer(tr.ID, false); err == nil || f.transferText(t, "called_at", tr.ID) != "" {
		t.Fatalf("前置：%v", err)
	}
	f.fake.FailRefund(errRefundDeclined)
	must(t, f.runRefund(r.ID, false))
	if f.topupOf(t, r.ID) != "WAITING" {
		t.Fatalf("換鍵之後退款失敗：%s", f.topupOf(t, r.ID))
	}
	f.clearJobs()
	must(t, f.runTransfer(tr.ID, false))
	if got := f.fake.TransferAmounts(); len(got) != 2 || got[0] != 70 || got[1] != 100 {
		t.Fatalf("provider 收到的金額=%v", got)
	}
	if f.topupOf(t, r.ID) != "RELEASED" || f.refundText(t, "topup_reason", r.ID) != "counted_in_new_amount" || f.refundText(t, "transfer_deducted", r.ID) != "false" || len(f.jobIDs(t, kindTransferTopup, "refund_id")) != 0 {
		t.Fatalf("補差額沒有釋出：%s %s deducted=%s", f.topupOf(t, r.ID), f.refundText(t, "topup_reason", r.ID), f.refundText(t, "transfer_deducted", r.ID))
	}
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	if f.fake.Transfers() != 1 || f.transferText(t, "amount", tr.ID) != "100" {
		t.Fatalf("賣家多收：provider 的轉帳 %d 筆", f.fake.Transfers())
	}
	f.noViolations(t)
}

// 轉帳終局失敗（沒有搬錢）：等待中的補差額釋出（transfer_failed）並清掉「已算進轉帳金額」，之後新的轉帳以當時的退款重算，失敗的退款不在其中。
// mutation：failTransfer 拿掉 settleTopupsWithoutTransfer → 補差額留在 WAITING，失敗。
func TestAPermanentlyFailedTransferReleasesTheWaitingTopUp(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "TU-5", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "TU-5")
	r := f.createRefund(t, "TU-5", "k", 30, false)
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.runTransfer(tr.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	f.fake.FailRefund(errRefundDeclined)
	must(t, f.runRefund(r.ID, false))
	if f.topupOf(t, r.ID) != "WAITING" {
		t.Fatalf("前置：%s", f.topupOf(t, r.ID))
	}
	f.fake.FailTransfer(fmt.Errorf("%w：目的帳戶不能收款", ErrChargePermanent))
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "FAILED" || f.topupOf(t, r.ID) != "RELEASED" || f.refundText(t, "topup_reason", r.ID) != "transfer_failed" || f.refundText(t, "transfer_deducted", r.ID) != "false" {
		t.Fatalf("轉帳失敗之後：%s %s %s", f.transferText(t, "status", tr.ID), f.topupOf(t, r.ID), f.refundText(t, "topup_reason", r.ID))
	}
	f.noViolations(t)
}

// 補差額被可重試拒絕（平台餘額不足、沒有執行）：換新冪等鍵重試，不計期限；重送被拒絕保持原鍵。
// mutation：RotateRefundTopupKey 不換鍵（拿掉 topup_idempotency_key = gen_random_uuid()）→ 重試用同一把鍵，失敗。
func TestARetryableRefusalOfTheTopUpRotatesItsIdempotencyKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	_, r := f.topupSending(t, "TU-6")
	before := f.refundText(t, "topup_idempotency_key", r.ID)
	f.fake.FailTransfer(errInsufficientFunds)
	if err := f.module.TopUpPaymentTransfer(ctx, r.ID, false); err == nil {
		t.Fatal("平台餘額不足沒有回錯誤")
	}
	if after := f.refundText(t, "topup_idempotency_key", r.ID); after == before || f.refundText(t, "topup_called_at", r.ID) != "" || f.topupOf(t, r.ID) != "SENDING" {
		t.Fatalf("換鍵之後：鍵 %s → %s，called_at=%q，狀態 %s", before, after, f.refundText(t, "topup_called_at", r.ID), f.topupOf(t, r.ID))
	}
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	keys := f.fake.TransferKeys()
	if len(keys) != 3 || keys[1] == keys[2] || keys[0] == keys[2] || f.topupOf(t, r.ID) != "DONE" || f.fake.Transfers() != 2 {
		t.Fatalf("補差額的鍵=%v，狀態 %s", keys, f.topupOf(t, r.ID))
	}
	f.noViolations(t)
}

// 補差額結果不明：回應遺失（provider 那邊已有一筆）過了 ResendWindow 只查回，找到就收下，不建第二筆、不再呼叫 CreateTransfer、鍵不變。
// mutation：topUp() 在過了視窗之後不走 lookupTopup 而直接重送 → 呼叫次數增加或出現第二筆補差額，失敗。
func TestAnUnknownTopUpFoundByLookupCreatesNoSecondTransfer(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	_, r := f.topupSending(t, "TU-7")
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.module.TopUpPaymentTransfer(ctx, r.ID, false); err == nil || f.topupOf(t, r.ID) != "SENDING" || f.refundText(t, "topup_called_at", r.ID) == "" {
		t.Fatalf("前置：%v", err)
	}
	key, calls, lookups := f.refundText(t, "topup_idempotency_key", r.ID), f.fake.TransferCalls(), f.fake.LookupCalls()
	f.pastResendWindow()
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	if f.topupOf(t, r.ID) != "DONE" || f.fake.TransferCalls() != calls || f.fake.Transfers() != 2 || f.fake.LookupCalls() != lookups+1 || f.refundText(t, "topup_idempotency_key", r.ID) != key {
		t.Fatalf("查回之後：%s，呼叫 %d → %d，轉帳 %d 筆，查回 %d 次（之前 %d）", f.topupOf(t, r.ID), calls, f.fake.TransferCalls(), f.fake.Transfers(), f.fake.LookupCalls(), lookups)
	}
	f.noViolations(t)
}

// 補差額結果不明且 provider 那邊沒有：查無要滿 NotFoundAfter 才算證明上一次不存在，這時才換新鍵重送（錢不能留在平台）；沒滿就稍後再查、鍵不變。
// mutation：lookupTopup 查無就換鍵、不等 NotFoundAfter → 未滿時鍵已被換，失敗。
// mutation：lookupTopup 換鍵之後不清 topup_called_at（RotateRefundTopupKey 拿掉 topup_called_at = NULL）→ 新鍵不重送，失敗。
func TestAnUnknownTopUpThatLookupProvesAbsentIsResentWithANewKeyOnlyAfterTheWait(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	_, r := f.topupSending(t, "TU-8")
	f.fake.FailTransfer(errLost)
	if err := f.module.TopUpPaymentTransfer(ctx, r.ID, false); err == nil {
		t.Fatal("呼叫沒有到達 provider 卻沒有回錯誤")
	}
	lost := f.refundText(t, "topup_idempotency_key", r.ID)
	f.pastResendWindow()
	if err := f.module.TopUpPaymentTransfer(ctx, r.ID, false); err == nil || f.refundText(t, "topup_idempotency_key", r.ID) != lost {
		t.Fatalf("未滿 NotFoundAfter 就換鍵或沒有回錯誤：%v", err)
	}
	f.now = f.now.Add(PaymentNotFoundAfter)
	if err := f.module.TopUpPaymentTransfer(ctx, r.ID, false); err == nil || f.refundText(t, "topup_idempotency_key", r.ID) == lost || f.refundText(t, "topup_called_at", r.ID) != "" {
		t.Fatalf("查無滿 NotFoundAfter 之後：%v", err)
	}
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	keys := f.fake.TransferKeys()
	if len(keys) != 3 || keys[1] != lost || keys[2] == lost || f.topupOf(t, r.ID) != "DONE" || f.fake.Transfers() != 2 {
		t.Fatalf("補差額的鍵=%v，狀態 %s，轉帳 %d 筆", keys, f.topupOf(t, r.ID), f.fake.Transfers())
	}
	f.noViolations(t)
}

// 補差額被 provider 永久拒絕：進 RELEASED（賣家少收，錢留在平台）並發 MoneyAnomaly 交後台，不重試。
// mutation：releaseTopup 不發 MoneyAnomaly → 沒有告警事件，失敗。
func TestAPermanentRefusalOfTheTopUpReleasesItAndAlertsTheMoneyAnomaly(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	_, r := f.topupSending(t, "TU-9")
	f.fake.FailTransfer(fmt.Errorf("%w：目的帳戶不能收款", ErrChargePermanent))
	must(t, f.module.TopUpPaymentTransfer(context.Background(), r.ID, false))
	if f.topupOf(t, r.ID) != "RELEASED" || f.refundText(t, "topup_reason", r.ID) != "provider_refused" || len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyTopup || f.anomalies[0].ObjectID != r.ID {
		t.Fatalf("永久拒絕之後：%s %s，事件 %+v", f.topupOf(t, r.ID), f.refundText(t, "topup_reason", r.ID), f.anomalies)
	}
	must(t, f.module.TopUpPaymentTransfer(context.Background(), r.ID, false))
	if f.fake.TransferCalls() != 2 {
		t.Fatalf("RELEASED 之後又呼叫了 provider：%d 次", f.fake.TransferCalls())
	}
	f.noViolations(t)
}

// 補差額查回連續失敗進 ANOMALY（補掃不再重試、發 MoneyAnomaly）；後台結案成成功要核對 provider 證據（依列上的嘗試 ID 查回），成功進 DONE、失敗進 RELEASED，各留一筆稽核。
// mutation：ResolveAnomaly 的 AnomalyTopup 不核對證據（verifyEvidence 直接回空的 proof）→ 錯的證據 ID 被收下，失敗。
func TestATopUpAnomalyIsResolvedOnlyAgainstTheProvidersEvidence(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	tr, r := f.topupSending(t, "TU-10")
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.module.TopUpPaymentTransfer(ctx, r.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	f.pastResendWindow()
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f.fake.FailLookup(errLost)
		_ = f.module.TopUpPaymentTransfer(ctx, r.ID, false)
	}
	if f.topupOf(t, r.ID) != "ANOMALY" || f.refundText(t, "topup_reason", r.ID) != "lookup_failed" || len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyTopup {
		t.Fatalf("查回連續失敗之後：%s，事件 %+v", f.topupOf(t, r.ID), f.anomalies)
	}
	calls := f.fake.TransferCalls()
	must(t, f.module.TopUpPaymentTransfer(ctx, r.ID, false))
	if f.fake.TransferCalls() != calls {
		t.Fatal("ANOMALY 的補差額被重送")
	}
	wantKind(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTopup, r.ID, AnomalySucceeded, f.transferText(t, "external_transfer_id", tr.ID)), apperr.KindFailedPrecondition, "證據是轉帳而不是補差額")
	var ext string
	for _, id := range f.fake.TransferIDs() {
		if id != f.transferText(t, "external_transfer_id", tr.ID) {
			ext = id
		}
	}
	audits := f.auditCount(t)
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTopup, r.ID, AnomalySucceeded, ext))
	if f.topupOf(t, r.ID) != "DONE" || f.refundText(t, "topup_external_id", r.ID) != ext || f.auditCount(t) != audits+1 {
		t.Fatalf("結案成功之後：%s %s", f.topupOf(t, r.ID), f.refundText(t, "topup_external_id", r.ID))
	}

	f2 := setupCharge(t)
	_, r2 := f2.topupSending(t, "TU-10b")
	f2.fake.FailTransferAfterCreate(errLost)
	_ = f2.module.TopUpPaymentTransfer(ctx, r2.ID, false)
	f2.pastResendWindow()
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f2.fake.FailLookup(errLost)
		_ = f2.module.TopUpPaymentTransfer(ctx, r2.ID, false)
	}
	must(t, f2.module.ResolveAnomaly(ctx, f2.admin(), AnomalyTopup, r2.ID, AnomalyFailed, ""))
	if f2.topupOf(t, r2.ID) != "RELEASED" || f2.refundText(t, "topup_reason", r2.ID) != "admin_resolved" {
		t.Fatalf("結案成失敗之後：%s", f2.topupOf(t, r2.ID))
	}
	f.noViolations(t)
	f2.noViolations(t)
}

// 併發：兩筆已算進轉帳金額的退款失敗與轉帳成立同時進來（轉帳先回應遺失、列 PENDING），不管誰先誰後，每筆退款恰好排一個補差額 job、provider 那邊恰好多兩筆轉帳；
// 同一筆補差額被兩個 job 同時執行也只補一次（同鍵冪等，後到的看到 DONE）。
// mutation：startTopups 不鎖也不看 WAITING（WaitingTopups 改成不過濾 topup_status）→ 補差額被排到不該補的退款或重複排，失敗。
func TestConcurrentRefundFailuresAndTransferSuccessNeverTopUpTwice(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.paidPayment(t, "TU-11", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "TU-11")
	a := f.createRefund(t, "TU-11", "a", 20, false)
	b := f.createRefund(t, "TU-11", "b", 10, false)
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.runTransfer(tr.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	f.fake.FailRefund(errRefundDeclined, errRefundDeclined)
	f.clearJobs()
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
		func() error { return f.runTransfer(tr.ID, false) },
	)
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.topupOf(t, a.ID) != "SENDING" || f.topupOf(t, b.ID) != "SENDING" {
		t.Fatalf("併發之後：轉帳 %s，補差額 %s %s", f.transferText(t, "status", tr.ID), f.topupOf(t, a.ID), f.topupOf(t, b.ID))
	}
	ids := f.jobIDs(t, kindTransferTopup, "refund_id")
	want := []string{a.ID.String(), b.ID.String()}
	got := []string{}
	for _, id := range ids {
		got = append(got, id.String())
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("補差額 job=%v，期望每筆退款一個 %v", got, want)
	}
	run(
		func() error { return f.module.TopUpPaymentTransfer(ctx, a.ID, false) },
		func() error { return f.module.TopUpPaymentTransfer(ctx, a.ID, false) },
		func() error { return f.module.TopUpPaymentTransfer(ctx, b.ID, false) },
		func() error { return f.module.TopUpPaymentTransfer(ctx, b.ID, false) },
	)
	extA, extB := f.refundText(t, "topup_external_id", a.ID), f.refundText(t, "topup_external_id", b.ID)
	if f.topupOf(t, a.ID) != "DONE" || f.topupOf(t, b.ID) != "DONE" || extA == "" || extB == "" || extA == extB || f.fake.Transfers() != 3 || len(f.anomalies) != 0 {
		t.Fatalf("補差額之後：%s %s，轉帳 %d 筆，事件 %+v", f.topupOf(t, a.ID), f.topupOf(t, b.ID), f.fake.Transfers(), f.anomalies)
	}
	f.noViolations(t)
}
