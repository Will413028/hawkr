//go:build integration

package payment_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/riverqueue/river"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 退款先退、再向賣家收回（計畫步驟 3c′，D11、ADR D69）的整合測試：轉帳金額在每把鍵第一次送出時鎖付款列算定，退款不看轉帳狀態，
// 撤回只在 provider 接受退款且轉帳成立之後開始，預留額在退款失敗或撤回終局時釋出。

// duringTransfer 包住付款 provider：CreateTransfer 在委派給 provider 之前先執行 during（轉帳已 claim、provider 尚未回應的那一刻）。
type duringTransfer struct {
	ChargeProvider
	during func()
}

func (d *duringTransfer) CreateTransfer(ctx context.Context, in ChargeTransferInput) (ChargeTransferSnapshot, error) {
	d.during()
	return d.ChargeProvider.CreateTransfer(ctx, in)
}

// reversalOf 讀退款列的收回狀態。
func (f *chargeFixture) reversalOf(t *testing.T, refundID interface{ String() string }) string {
	t.Helper()
	return f.text(t, "SELECT reversal_status FROM payment.refunds WHERE id = $1::uuid", refundID.String())
}

// 轉帳送出前的退款：轉帳在第一次送出時才算定金額（付款減非失敗退款），這筆退款已在其中，所以轉帳少付、沒有預留也沒有撤回；全額退款讓轉帳不呼叫 provider 直接終局。
// mutation：claimTransfer 的金額改回寫列時的 locked.Amount（不重算）→ provider 收到 100，失敗。
// mutation：claimTransfer 算出不大於 0 時仍呼叫 provider → 全額退款的轉帳呼叫了 provider，失敗。
// 全額退款還在進行中時轉帳等退款終局，退款成功才進 FAILED（fully_refunded）。
func TestARefundBeforeTheTransferIsSentMakesTheTransferPayLessAndNeedsNoReversal(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RR-1", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RR-1")
	if tr.Amount != 100 {
		t.Fatalf("寫列時的暫定金額=%d", tr.Amount)
	}
	r := f.createRefund(t, "RR-1", "k", 30, false)
	if r.Status != RefundPending || r.ReversalStatus != ReversalNone || r.ReversalAmount != 0 {
		t.Fatalf("轉帳沒有承諾金額時的退款：%+v", r)
	}
	must(t, f.runTransfer(tr.ID, false))
	if got := f.fake.TransferAmounts(); len(got) != 1 || got[0] != 70 {
		t.Fatalf("provider 收到的轉帳金額=%v", got)
	}
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.transferText(t, "amount", tr.ID) != "70" {
		t.Fatalf("轉帳=%s 金額 %s", f.transferText(t, "status", tr.ID), f.transferText(t, "amount", tr.ID))
	}
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "SUCCEEDED" || f.fake.ReversalCalls() != 0 || f.reversedOf(t, tr.ID) != 0 || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 {
		t.Fatalf("不該有撤回：退款 %s，撤回呼叫 %d", f.refundText(t, "status", r.ID), f.fake.ReversalCalls())
	}

	f.paidPayment(t, "RR-1b", 100)
	all := f.requestTransfer(t, "RR-1b")
	allRefund := f.createRefund(t, "RR-1b", "all", 100, false)
	if err := f.runTransfer(all.ID, false); err == nil || f.transferText(t, "status", all.ID) != "PENDING" || f.transferText(t, "called_at", all.ID) != "" {
		t.Fatalf("退款還在進行中，轉帳不該終局：%v，%s", err, f.transferText(t, "status", all.ID))
	}
	must(t, f.runRefund(allRefund.ID, false))
	must(t, f.runTransfer(all.ID, false))
	if f.transferText(t, "status", all.ID) != "FAILED" || f.transferText(t, "failure_reason", all.ID) != "fully_refunded" || f.fake.TransferCalls() != 1 {
		t.Fatalf("全額退款的轉帳：%s %s，provider 呼叫 %d 次", f.transferText(t, "status", all.ID), f.transferText(t, "failure_reason", all.ID), f.fake.TransferCalls())
	}
	_, err := f.module.RequestPaymentTransfer(context.Background(), "RR-1b")
	wantKind(t, err, apperr.KindFailedPrecondition, "全額退款之後再請求轉帳")
	f.noViolations(t)
}

// 換新冪等鍵才重算金額：可重試拒絕（平台餘額不足）之後換新鍵、清掉 called_at，下一次送出重新 claim。
// 拒絕發生在 claim 之後：這段時間進來的退款預留了收回額度（WAITING），換鍵後的重算已把它從金額扣掉，預留釋出、不必撤回；同一把鍵的金額不變（fake 對同鍵不同額回永久失敗）。
// mutation：RotateTransferKey 不清 called_at → 換鍵後不重新 claim，provider 又收到 100，失敗。
// mutation：claimTransfer 的重算分支不釋出 WAITING 預留 → 預留留在列上，失敗。
func TestAKeyRotationRecomputesTheTransferAmountAndReleasesTheReservationsItAlreadyCovers(t *testing.T) {
	t.Parallel()
	var f *chargeFixture
	var armed atomic.Bool
	var during PaymentRefund
	f = setupChargeWith(t, func(fake *ChargeFake) ChargeProvider {
		return &duringTransfer{ChargeProvider: fake, during: func() {
			if armed.CompareAndSwap(true, false) {
				during = f.createRefund(t, "RR-2", "late", 30, false)
			}
		}}
	})
	f.paidPayment(t, "RR-2", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RR-2")
	armed.Store(true)
	f.fake.FailTransfer(errInsufficientFunds)
	if err := f.runTransfer(tr.ID, false); err == nil {
		t.Fatal("平台餘額不足沒有回錯誤")
	}
	if during.ReversalStatus != ReversalWaiting || during.ReversalAmount != 30 || f.reversalOf(t, during.ID) != "WAITING" || f.transferText(t, "called_at", tr.ID) != "" {
		t.Fatalf("claim 之後進來的退款：%+v，called_at=%q", during, f.transferText(t, "called_at", tr.ID))
	}
	must(t, f.runTransfer(tr.ID, false))
	if got := f.fake.TransferAmounts(); len(got) != 2 || got[0] != 100 || got[1] != 70 {
		t.Fatalf("provider 收到的金額=%v", got)
	}
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.transferText(t, "amount", tr.ID) != "70" {
		t.Fatalf("轉帳=%s 金額 %s", f.transferText(t, "status", tr.ID), f.transferText(t, "amount", tr.ID))
	}
	if f.reversalOf(t, during.ID) != "RELEASED" || f.refundText(t, "reversal_reason", during.ID) != "deducted_from_transfer" || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 {
		t.Fatalf("預留沒有釋出：%s %s", f.reversalOf(t, during.ID), f.refundText(t, "reversal_reason", during.ID))
	}
	must(t, f.runRefund(during.ID, false))
	if f.refundText(t, "status", during.ID) != "SUCCEEDED" || f.fake.ReversalCalls() != 0 || f.reversedOf(t, tr.ID) != 0 {
		t.Fatalf("退款之後：%s，撤回呼叫 %d", f.refundText(t, "status", during.ID), f.fake.ReversalCalls())
	}
	f.noViolations(t)
}

// 併發：退款與轉帳送出同時進來，只會排成先後，不會多付。證明競態真的形成：退款停在「已持有付款列鎖、列已寫入、尚未提交」（job 排入時阻塞），
// 轉帳的 claim 必須等在付款列的鎖上（pg_stat_activity 看得到 Lock 等待）；放行之後 claim 看得到這筆退款，轉帳只付 60、沒有撤回。
// 付款列的鎖本身由 TestTheTransferClaimWaitsOnThePaymentRowLockAloneAndSeesTheRefundCommittedUnderIt 守（這裡退款同時持有轉帳列的鎖，拿掉 LockPayment 也會被擋住）。
func TestAConcurrentRefundAndTheTransferSendNeverOverpay(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RR-3", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RR-3")
	reached, release := make(chan struct{}), make(chan struct{})
	unblock := func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}
	defer unblock()
	var first atomic.Bool
	f.jobs.mu.Lock()
	f.jobs.before = func(a river.JobArgs) {
		if a.Kind() == kindRefundCreate && first.CompareAndSwap(false, true) {
			close(reached)
			<-release
		}
	}
	f.jobs.mu.Unlock()
	ctx := context.Background()
	refundDone := make(chan error, 1)
	var refundID PaymentRefund
	go func() {
		r, err := f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RR-3", Amount: 40, RequestKey: "k"})
		refundID = r
		refundDone <- err
	}()
	select {
	case <-reached:
	case <-time.After(30 * time.Second):
		t.Fatal("退款沒有走到排入 job")
	}
	transferDone := make(chan error, 1)
	go func() { transferDone <- f.runTransfer(tr.ID, false) }()
	deadline := time.After(30 * time.Second)
	for blocked := false; !blocked; {
		select {
		case err := <-transferDone:
			t.Fatalf("claim 在退款持有付款列鎖時就結束了：%v", err)
		case <-deadline:
			t.Fatal("轉帳的 claim 一直沒有等在付款列的鎖上")
		case <-time.After(10 * time.Millisecond):
			blocked = f.count(t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%FOR UPDATE%'") > 0
		}
	}
	unblock()
	must(t, <-refundDone)
	must(t, <-transferDone)
	if got := f.fake.TransferAmounts(); len(got) != 1 || got[0] != 60 {
		t.Fatalf("provider 收到的轉帳金額=%v", got)
	}
	if f.reversalOf(t, refundID.ID) != "NONE" || f.fake.ReversalCalls() != 0 || f.transferText(t, "status", tr.ID) != "SUCCEEDED" {
		t.Fatalf("退款先提交時不該有收回：%s", f.reversalOf(t, refundID.ID))
	}
	f.noViolations(t)
}

// 轉帳金額已承諾（PENDING 且呼叫過）時的退款照常受理並預留額度；撤回等「provider 接受退款」與「轉帳成立」兩個事件都到齊，誰後到誰排：
// 退款在轉帳結果不明時先被接受（pending 也算接受）→ 轉帳成立的 transaction 排撤回；轉帳先成立 → 退款被接受的 transaction 排撤回。撤回不等退款成功。
// mutation：acceptRefund 只在 target 是 SUCCEEDED 時開始撤回（pending 不開始）→ 退款 pending 時沒有撤回 job，失敗。
// mutation：acceptTransfer 成功後不呼叫 startReversals → 轉帳成立後沒有撤回 job，失敗。
func TestARefundAcceptedWhileTheTransferIsPendingStartsTheReversalWhenTheTransferSucceeds(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.paidPayment(t, "RR-4", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RR-4")
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.runTransfer(tr.ID, false); err == nil || f.transferText(t, "status", tr.ID) != "PENDING" || f.transferText(t, "called_at", tr.ID) == "" {
		t.Fatalf("前置：%v，狀態 %s", err, f.transferText(t, "status", tr.ID))
	}
	r := f.createRefund(t, "RR-4", "k", 40, false)
	if r.Status != RefundPending || r.ReversalStatus != ReversalWaiting || r.ReversalAmount != 40 {
		t.Fatalf("轉帳 PENDING（已呼叫）時的退款：%+v", r)
	}
	f.fake.NextRefundState(ChargeRefundPending)
	f.clearJobs()
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "REFUNDING" || f.reversalOf(t, r.ID) != "WAITING" || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 || f.fake.ReversalCalls() != 0 {
		t.Fatalf("轉帳還沒成立時就開始撤回：%s %s", f.refundText(t, "status", r.ID), f.reversalOf(t, r.ID))
	}
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.reversalOf(t, r.ID) != "REVERSING" {
		t.Fatalf("轉帳成立之後：%s %s", f.transferText(t, "status", tr.ID), f.reversalOf(t, r.ID))
	}
	if ids := f.jobIDs(t, kindTransferReversal, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("撤回 job=%v", ids)
	}
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.reversalOf(t, r.ID) != "REVERSED" || f.reversedOf(t, tr.ID) != 40 || f.refundText(t, "status", r.ID) != "REFUNDING" || f.fake.ReversalCalls() != 1 {
		t.Fatalf("撤回不該等退款成功：%s，reversed %d，退款 %s", f.reversalOf(t, r.ID), f.reversedOf(t, tr.ID), f.refundText(t, "status", r.ID))
	}

	// 另一個順序：轉帳先成立，退款在 provider 接受（pending）時才排撤回。
	f.paidPayment(t, "RR-4b", 100)
	tr2 := f.requestTransfer(t, "RR-4b")
	must(t, f.runTransfer(tr2.ID, false))
	r2 := f.createRefund(t, "RR-4b", "k", 25, false)
	if r2.ReversalStatus != ReversalWaiting || r2.ReversalAmount != 25 {
		t.Fatalf("轉帳成立之後的退款：%+v", r2)
	}
	f.clearJobs()
	if err := f.module.ReversePaymentTransfer(ctx, r2.ID, false); err != nil || f.fake.ReversalCalls() != 1 || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 {
		t.Fatalf("WAITING 的撤回被執行：%v，撤回呼叫 %d", err, f.fake.ReversalCalls())
	}
	f.fake.NextRefundState(ChargeRefundPending)
	must(t, f.runRefund(r2.ID, false))
	if f.refundText(t, "status", r2.ID) != "REFUNDING" || f.reversalOf(t, r2.ID) != "REVERSING" {
		t.Fatalf("退款被接受之後：%s %s", f.refundText(t, "status", r2.ID), f.reversalOf(t, r2.ID))
	}
	if ids := f.jobIDs(t, kindTransferReversal, "refund_id"); len(ids) != 1 || ids[0] != r2.ID {
		t.Fatalf("退款被接受的 transaction 沒有排撤回 job：%v", ids)
	}

	// 轉帳成立時退款還沒被 provider 接受：預留留在 WAITING、不排撤回 job；退款被接受之後才開始。
	f.paidPayment(t, "RR-4c", 100)
	tr3 := f.requestTransfer(t, "RR-4c")
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.runTransfer(tr3.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	r3 := f.createRefund(t, "RR-4c", "k", 20, false)
	f.clearJobs()
	must(t, f.runTransfer(tr3.ID, false))
	if f.transferText(t, "status", tr3.ID) != "SUCCEEDED" || f.reversalOf(t, r3.ID) != "WAITING" || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 {
		t.Fatalf("退款還沒被接受就開始撤回：%s %s", f.transferText(t, "status", tr3.ID), f.reversalOf(t, r3.ID))
	}
	must(t, f.runRefund(r3.ID, false))
	if f.reversalOf(t, r3.ID) != "REVERSING" || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 1 {
		t.Fatalf("退款被接受之後：%s", f.reversalOf(t, r3.ID))
	}
	f.noViolations(t)
}

// 轉帳在 claim 之後被 provider 永久拒絕：這段時間進來的退款預留（WAITING）同 transaction 釋出（錢本來就在平台手上），退款照常退、不撤回。
// mutation：failTransfer 拿掉 releaseWaitingReversals → 預留留在 WAITING，失敗。
func TestATransferThatFailsPermanentlyReleasesTheReservationsOfRefundsMadeAfterItsClaim(t *testing.T) {
	t.Parallel()
	var f *chargeFixture
	var armed atomic.Bool
	var during PaymentRefund
	f = setupChargeWith(t, func(fake *ChargeFake) ChargeProvider {
		return &duringTransfer{ChargeProvider: fake, during: func() {
			if armed.CompareAndSwap(true, false) {
				during = f.createRefund(t, "RR-9", "late", 30, false)
			}
		}}
	})
	f.paidPayment(t, "RR-9", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RR-9")
	armed.Store(true)
	f.fake.FailTransfer(fmt.Errorf("%w：目的帳戶不能收款", ErrChargePermanent))
	must(t, f.runTransfer(tr.ID, false))
	if during.ReversalStatus != ReversalWaiting || f.transferText(t, "status", tr.ID) != "FAILED" {
		t.Fatalf("前置：%+v，轉帳 %s", during, f.transferText(t, "status", tr.ID))
	}
	if f.reversalOf(t, during.ID) != "RELEASED" || f.refundText(t, "reversal_reason", during.ID) != "transfer_failed" {
		t.Fatalf("預留沒有釋出：%s %s", f.reversalOf(t, during.ID), f.refundText(t, "reversal_reason", during.ID))
	}
	must(t, f.runRefund(during.ID, false))
	if f.refundText(t, "status", during.ID) != "SUCCEEDED" || f.fake.ReversalCalls() != 0 || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 {
		t.Fatalf("退款之後：%s，撤回呼叫 %d", f.refundText(t, "status", during.ID), f.fake.ReversalCalls())
	}
	f.noViolations(t)
}

// 退款在撤回開始之前失敗：預留釋出、不撤回。涵蓋 provider 直接拒絕，以及先被接受（轉帳還沒成立所以撤回仍在等）之後才變成失敗；轉帳之後才成立也不會排撤回。
// mutation：寫 FAILED 的 SQL（FailRefundCall、ApplyRefundSnapshot）不釋出 WAITING → 違反 refunds_failed_releases_waiting_check 或留著預留，失敗。
// mutation：failRefund 之後仍排撤回（startReversals 不看退款狀態）→ 有撤回呼叫，失敗。
func TestARefundThatFailsBeforeTheReversalStartsReleasesTheReservationAndNeverReverses(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	_, tr := f.transferred(t, "RR-5")
	r := f.createRefund(t, "RR-5", "k", 40, false)
	f.fake.FailRefund(fmt.Errorf("%w：卡片已失效", ErrChargePermanent))
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "FAILED" || f.reversalOf(t, r.ID) != "RELEASED" || f.refundText(t, "reversal_reason", r.ID) != "refund_failed" {
		t.Fatalf("provider 拒絕之後：%s %s", f.refundText(t, "status", r.ID), f.reversalOf(t, r.ID))
	}
	if f.fake.ReversalCalls() != 0 || f.reversedOf(t, tr.ID) != 0 || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 || len(f.anomalies) != 0 {
		t.Fatalf("不該有撤回或告警：撤回呼叫 %d，事件 %+v", f.fake.ReversalCalls(), f.anomalies)
	}

	// 先被接受、轉帳還沒成立時變成失敗：預留在 WAITING 釋出，之後轉帳成立也不撤回。
	f.paidPayment(t, "RR-5b", 100)
	tr2 := f.requestTransfer(t, "RR-5b")
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.runTransfer(tr2.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	r2 := f.createRefund(t, "RR-5b", "k", 40, false)
	f.fake.NextRefundState(ChargeRefundPending)
	must(t, f.runRefund(r2.ID, false))
	must(t, f.fake.SetRefundState(f.refundText(t, "external_refund_id", r2.ID), ChargeRefundFailed, "declined"))
	f.syncRefund(t, context.Background(), r2.ID)
	if f.refundText(t, "status", r2.ID) != "FAILED" || f.reversalOf(t, r2.ID) != "RELEASED" {
		t.Fatalf("接受之後才失敗：%s %s", f.refundText(t, "status", r2.ID), f.reversalOf(t, r2.ID))
	}
	f.clearJobs()
	must(t, f.runTransfer(tr2.ID, false))
	if f.transferText(t, "status", tr2.ID) != "SUCCEEDED" || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 || f.fake.ReversalCalls() != 0 {
		t.Fatalf("轉帳成立之後：%s，撤回呼叫 %d", f.transferText(t, "status", tr2.ID), f.fake.ReversalCalls())
	}
	f.noViolations(t)
}

// 退款在撤回開始之後才失敗：撤回照樣收斂到底（不能取消已送出的呼叫），落地時錢留在平台（轉帳的 reversed_amount 不減）並發 MoneyAnomaly；
// 撤回已落地之後才失敗則在退款變失敗的 transaction 發 MoneyAnomaly。
// mutation：acceptReversal 在退款已 FAILED 時不發 MoneyAnomaly → 第一段沒有事件，失敗。
// mutation：afterRefundFailed 拿掉 REVERSED 的 MoneyAnomaly → 第二段沒有事件，失敗。
func TestARefundThatFailsAfterTheReversalStartedKeepsTheReversalAndAlertsTheMoneyAnomaly(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	_, tr := f.transferred(t, "RR-6")
	a := f.createRefund(t, "RR-6", "a", 40, false)
	f.fake.NextRefundState(ChargeRefundPending)
	must(t, f.runRefund(a.ID, false))
	if f.reversalOf(t, a.ID) != "REVERSING" {
		t.Fatalf("前置：%s", f.reversalOf(t, a.ID))
	}
	must(t, f.fake.SetRefundState(f.refundText(t, "external_refund_id", a.ID), ChargeRefundFailed, "declined"))
	f.syncRefund(t, ctx, a.ID)
	if f.refundText(t, "status", a.ID) != "FAILED" || f.reversalOf(t, a.ID) != "REVERSING" || len(f.anomalies) != 0 {
		t.Fatalf("撤回進行中退款失敗：%s %s，事件 %+v", f.refundText(t, "status", a.ID), f.reversalOf(t, a.ID), f.anomalies)
	}
	must(t, f.module.ReversePaymentTransfer(ctx, a.ID, false))
	if f.reversalOf(t, a.ID) != "REVERSED" || f.reversedOf(t, tr.ID) != 40 || len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyRefund || f.anomalies[0].ObjectID != a.ID || !strings.Contains(f.anomalies[0].Reason, "refund_failed_after_reversal") || !f.anomalyInTx[0] {
		t.Fatalf("撤回落地之後：%s，reversed %d，事件 %+v", f.reversalOf(t, a.ID), f.reversedOf(t, tr.ID), f.anomalies)
	}

	b := f.createRefund(t, "RR-6", "b", 30, false)
	if b.ReversalAmount != 30 {
		t.Fatalf("第二筆：%+v", b)
	}
	f.fake.NextRefundState(ChargeRefundPending)
	must(t, f.runRefund(b.ID, false))
	must(t, f.module.ReversePaymentTransfer(ctx, b.ID, false))
	must(t, f.fake.SetRefundState(f.refundText(t, "external_refund_id", b.ID), ChargeRefundFailed, "declined"))
	f.syncRefund(t, ctx, b.ID)
	if f.refundText(t, "status", b.ID) != "FAILED" || f.reversalOf(t, b.ID) != "REVERSED" || f.reversedOf(t, tr.ID) != 70 || len(f.anomalies) != 2 || f.anomalies[1].ObjectID != b.ID || !f.anomalyInTx[1] {
		t.Fatalf("撤回落地之後退款才失敗：%s %s，reversed %d，事件 %+v", f.refundText(t, "status", b.ID), f.reversalOf(t, b.ID), f.reversedOf(t, tr.ID), f.anomalies)
	}
	f.noViolations(t)
}

// 預留額度：撤回的 ANOMALY 持續佔住預留（退款本身已 FAILED、額度已釋出時也一樣），新退款的收回額＝min(退款額, 轉帳金額 − 已撤回 − 其他預留)；
// 撤回永久失敗則釋出預留，退款維持原狀、錢由平台吸收並發 MoneyAnomaly。
// mutation：PendingReversalTotal 不計 ANOMALY → 新退款預留 100，失敗。
func TestAnAnomalousReversalKeepsHoldingItsReservationAndAPermanentFailureReleasesIt(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, _ := f.transferred(t, "RR-7")
	f.insertReversal(t, p.ID, "FAILED", "ANOMALY", 40, 40)
	big := f.createRefund(t, "RR-7", "big", 100, false)
	if big.ReversalAmount != 60 || big.ReversalStatus != ReversalWaiting {
		t.Fatalf("ANOMALY 的預留沒有佔住：%+v", big)
	}
	must(t, f.runRefund(big.ID, false))
	if f.reversalOf(t, big.ID) != "REVERSING" {
		t.Fatalf("前置：%s", f.reversalOf(t, big.ID))
	}
	f.fake.FailReversal(fmt.Errorf("%w：不能撤回", ErrChargePermanent))
	must(t, f.module.ReversePaymentTransfer(ctx, big.ID, false))
	if f.reversalOf(t, big.ID) != "RELEASED" || f.refundText(t, "status", big.ID) != "SUCCEEDED" {
		t.Fatalf("撤回永久失敗：%s，退款 %s", f.reversalOf(t, big.ID), f.refundText(t, "status", big.ID))
	}
	if len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyReversal || f.anomalies[0].ObjectID != f.reversalID(t, big.ID) || !f.anomalyInTx[0] {
		t.Fatalf("事件=%+v", f.anomalies)
	}
	f.noViolations(t)
}

// 轉帳的 ANOMALY 人工結案成功：等待中的預留依 provider 核實的轉帳為準重新分配（核實的金額 − provider 上已撤回的額度），不用列上的數字；
// provider 上已被另外撤回 70 時，預留 40 的退款只能收回 30，撤回呼叫的金額也是 30；結案成失敗則釋出等待中的預留。
// mutation：resolveTransfer 以 0 取代 proof.reversed（用列上的數字）→ 預留仍是 40、撤回超過 provider 的剩餘額度被拒絕，失敗。
func TestResolvingATransferAnomalyAsSucceededCapsTheWaitingReversalsByTheVerifiedTransfer(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.anomalousTransfer(t, "RR-8", true)
	r := f.createRefund(t, "RR-8", "k", 40, false)
	if r.ReversalStatus != ReversalWaiting || r.ReversalAmount != 40 {
		t.Fatalf("轉帳 ANOMALY 時的退款：%+v", r)
	}
	ext := f.fake.TransferIDs()[0]
	if _, err := f.fake.ReverseTransfer(ctx, ChargeReversalInput{ExternalTransferID: ext, Amount: 70, IdempotencyKey: "dashboard-1"}); err != nil {
		t.Fatal(err)
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr.ID, AnomalySucceeded, ext))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.reversedOf(t, tr.ID) != 70 {
		t.Fatalf("結案之後：%s，reversed %d", f.transferText(t, "status", tr.ID), f.reversedOf(t, tr.ID))
	}
	if f.reversalOf(t, r.ID) != "WAITING" || f.refundText(t, "reversal_amount", r.ID) != "30" {
		t.Fatalf("預留沒有依核實的額度重新分配：%s %s", f.reversalOf(t, r.ID), f.refundText(t, "reversal_amount", r.ID))
	}
	must(t, f.runRefund(r.ID, false))
	if f.reversalOf(t, r.ID) != "REVERSING" {
		t.Fatalf("退款被接受之後：%s", f.reversalOf(t, r.ID))
	}
	calls := f.fake.ReversalCalls()
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.reversalOf(t, r.ID) != "REVERSED" || f.reversedOf(t, tr.ID) != 100 || f.fake.ReversalCalls() != calls+1 {
		t.Fatalf("撤回之後：%s，reversed %d", f.reversalOf(t, r.ID), f.reversedOf(t, tr.ID))
	}

	if f.count(t, "SELECT amount FROM payment.refund_reversals WHERE refund_id = $1", r.ID) != 30 || f.count(t, "SELECT count(*) FROM payment.refund_reversals WHERE refund_id = $1", r.ID) != 1 {
		t.Fatalf("撤回列沒有依核實的額度建立：%d 列", f.count(t, "SELECT count(*) FROM payment.refund_reversals WHERE refund_id = $1", r.ID))
	}

	// 結案成失敗：等待中的預留釋出。
	f.paidPayment(t, "RR-8b", 100)
	tr2 := f.requestTransfer(t, "RR-8b")
	f.fake.FailTransfer(errLost)
	if err := f.runTransfer(tr2.ID, false); err == nil {
		t.Fatal("呼叫沒有到達 provider 卻沒有回錯誤")
	}
	f.pastResendWindow()
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f.fake.FailLookup(errLost)
		_ = f.runTransfer(tr2.ID, false)
	}
	r2 := f.createRefund(t, "RR-8b", "k", 40, false)
	if r2.ReversalStatus != ReversalWaiting {
		t.Fatalf("轉帳 ANOMALY 時的退款：%+v", r2)
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr2.ID, AnomalyFailed, ""))
	if f.transferText(t, "status", tr2.ID) != "FAILED" || f.reversalOf(t, r2.ID) != "RELEASED" || f.refundText(t, "reversal_reason", r2.ID) != "transfer_failed" {
		t.Fatalf("結案成失敗：%s %s", f.transferText(t, "status", tr2.ID), f.reversalOf(t, r2.ID))
	}
	f.noViolations(t)
}

// 退款已涵蓋整筆付款、其中還有進行中的退款：轉帳等它們終局，不先終結成 fully_refunded（那樣退款失敗之後賣家永遠收不到）。
// 兩種時序：轉帳列在先、退款在後（claim 時才發現，轉帳保持 PENDING、不承諾金額）；退款在先、之後才請求轉帳（RequestTransfer 不拒絕，暫定金額是進行中退款的加總）。
// 退款失敗之後轉帳照常以 100 送出，賣家恰好收到一次。
// mutation：claimTransfer 拿掉 errRefundsInFlight 的判斷（算出不大於 0 就終結）→ 轉帳直接 FAILED（fully_refunded），退款失敗之後賣家沒有收到 100，失敗。
// mutation：transferTerms 在進行中的退款涵蓋整筆付款時仍拒絕（拿掉 InFlightRefundTotal 的判斷）→ 請求被拒、沒有轉帳列，失敗。
func TestATransferCoveredByInFlightRefundsWaitsForThemAndPaysTheSellerWhenTheyFail(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.enableAccount(t, 7, "acct_seller_7", true)

	f.paidPayment(t, "RR-10a", 100)
	early := f.requestTransfer(t, "RR-10a")
	a := f.createRefund(t, "RR-10a", "all", 100, false)
	if err := f.runTransfer(early.ID, false); err == nil || f.transferText(t, "status", early.ID) != "PENDING" || f.transferText(t, "called_at", early.ID) != "" || f.fake.TransferCalls() != 0 {
		t.Fatalf("退款進行中，轉帳該等：%v，%s，provider 呼叫 %d 次", err, f.transferText(t, "status", early.ID), f.fake.TransferCalls())
	}
	f.fake.FailRefund(errRefundDeclined)
	must(t, f.runRefund(a.ID, false))
	if f.refundText(t, "status", a.ID) != "FAILED" {
		t.Fatalf("前置：退款 %s", f.refundText(t, "status", a.ID))
	}
	must(t, f.runTransfer(early.ID, false))
	if got := f.fake.TransferAmounts(); len(got) != 1 || got[0] != 100 || f.transferText(t, "status", early.ID) != "SUCCEEDED" || f.transferText(t, "amount", early.ID) != "100" {
		t.Fatalf("退款失敗之後賣家該收到 100：%v，%s", got, f.transferText(t, "status", early.ID))
	}

	f.paidPayment(t, "RR-10b", 100)
	b := f.createRefund(t, "RR-10b", "all", 100, false)
	late := f.requestTransfer(t, "RR-10b")
	if late.Amount != 100 || f.transferText(t, "status", late.ID) != "PENDING" {
		t.Fatalf("進行中的退款涵蓋整筆時請求轉帳：金額 %d，%s", late.Amount, f.transferText(t, "status", late.ID))
	}
	if err := f.runTransfer(late.ID, false); err == nil || f.transferText(t, "status", late.ID) != "PENDING" || f.fake.TransferCalls() != 1 {
		t.Fatalf("退款進行中，轉帳該等：%v，provider 呼叫 %d 次", err, f.fake.TransferCalls())
	}
	f.fake.FailRefund(errRefundDeclined)
	must(t, f.runRefund(b.ID, false))
	must(t, f.runTransfer(late.ID, false))
	if got := f.fake.TransferAmounts(); len(got) != 2 || got[1] != 100 || f.fake.Transfers() != 2 || f.transferText(t, "status", late.ID) != "SUCCEEDED" {
		t.Fatalf("退款失敗之後賣家該收到 100 恰好一次：%v，%s", got, f.transferText(t, "status", late.ID))
	}
	f.noViolations(t)
}

// 退款呼叫的 ANOMALY 人工結案成失敗：先前已有撤回成立（reversal_recovered 大於 0）時，和退款自己失敗一樣發 MoneyAnomaly（refund_failed_after_reversal，撤回已落地、錢留在平台）。
// mutation：resolveRefund 的失敗分支不呼叫 afterRefundFailed → 沒有事件，失敗。
func TestResolvingARefundAnomalyAsFailedAfterAReversalLandedAlertsTheMoneyAnomaly(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, _ := f.transferred(t, "RR-11")
	id := f.insertReversal(t, p.ID, "REFUNDING", "REVERSED", 40, 40)
	f.exec(t, `UPDATE payment.refunds SET status = 'ANOMALY', failure_reason = 'x' WHERE id = $1`, id)
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyRefund, id, AnomalyFailed, ""))
	if f.refundText(t, "status", id) != "FAILED" {
		t.Fatalf("結案之後：%s", f.refundText(t, "status", id))
	}
	if len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyRefund || f.anomalies[0].ObjectID != id || !strings.Contains(f.anomalies[0].Reason, "refund_failed_after_reversal") || !f.anomalyInTx[0] {
		t.Fatalf("事件=%+v", f.anomalies)
	}
	f.noViolations(t)
}

// 開始收回時各目標的剩餘額度不夠預留額（預留的不變量被破壞）：預留縮到分得到的額度、發 MoneyAnomaly，事件的物件是退款（ObjectID 是退款 ID）；
// 造法：預留之後轉帳的 reversed_amount 被推到 30，剩餘只有 70，預留 100 的退款被接受時只分得 70。
// mutation：startReversals 的 MoneyAnomaly 改回 AnomalyReversal → 事件種類不是 AnomalyRefund，失敗。
func TestAReservationLargerThanTheTargetsRemainingIsCappedAndAlertsOnTheRefund(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	_, tr := f.transferred(t, "RR-12")
	r := f.createRefund(t, "RR-12", "k", 100, false)
	if r.ReversalStatus != ReversalWaiting || r.ReversalAmount != 100 {
		t.Fatalf("前置：%+v", r)
	}
	f.exec(t, `UPDATE payment.transfers SET reversed_amount = 30 WHERE id = $1`, tr.ID)
	must(t, f.runRefund(r.ID, false))
	if f.reversalOf(t, r.ID) != "REVERSING" || f.refundText(t, "reversal_amount", r.ID) != "70" {
		t.Fatalf("預留沒有縮到 70：%s %s", f.reversalOf(t, r.ID), f.refundText(t, "reversal_amount", r.ID))
	}
	if len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyRefund || f.anomalies[0].ObjectID != r.ID || !strings.Contains(f.anomalies[0].Reason, "reversal_exceeds_remaining") || !f.anomalyInTx[0] {
		t.Fatalf("事件=%+v", f.anomalies)
	}
	f.noViolations(t)
}

// 預留額度逐列算：已失敗的退款還有一列撤回在 REVERSING、另一列已 RELEASED（錢留在平台，不再佔額度）時，只有 REVERSING 的那一列佔預留；
// 新退款的預留 = min(退款額, 轉帳金額 − 已撤回 − 其他預留) = 90，不是把 RELEASED 那列也算進去的 40。
// mutation：PendingReversalTotal 改回彙總的 reversal_amount − reversal_recovered（WAITING、REVERSING、ANOMALY 的退款）→ 預留 40，失敗。
func TestAReleasedReversalRowOfAFailedRefundNoLongerReservesCapacity(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p, _ := f.transferred(t, "RR-13")
	a := f.insertReversal(t, p.ID, "FAILED", "REVERSING", 60, 60)
	f.exec(t, `UPDATE payment.refund_reversals SET amount = 10 WHERE refund_id = $1`, a)
	f.exec(t, `INSERT INTO payment.refund_reversals (id, refund_id, payment_id, seq, target_refund_id, target_external_id, amount, status, reason, created_at, updated_at)
VALUES (gen_random_uuid(), $1, $2, 1, $1, 'tr_topup', 50, 'RELEASED', 'x', $3, $3)`, a, p.ID, f.now)
	b := f.createRefund(t, "RR-13", "b", 100, false)
	if b.ReversalStatus != ReversalWaiting || b.ReversalAmount != 90 {
		t.Fatalf("預留應是 100 − 10 = 90：%+v", b)
	}
	f.noViolations(t)
}

// 轉帳的 claim 等付款列的鎖：另一個 transaction 只持有付款列的鎖（不碰轉帳列）寫入退款、尚未提交時，claim 必須等它，提交之後才看得到這筆退款（轉帳只付 60）。
// 與 TestAConcurrentRefundAndTheTransferSendNeverOverpay（退款走 CreateRefund，同時鎖了轉帳列，轉帳列的鎖也會擋住 claim）不同，這裡只有付款列的鎖能把兩者排成先後。
// mutation：claimTransfer 的 LockPayment 改成不鎖的讀取（PaymentByID）→ claim 不等這個 transaction、以 100 送出，下面「claim 在付款列被鎖時就結束」失敗。
func TestTheTransferClaimWaitsOnThePaymentRowLockAloneAndSeesTheRefundCommittedUnderIt(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RR-14", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RR-14")
	payID := f.idOf(t, "RR-14")
	ctx := context.Background()
	reached, release := make(chan struct{}), make(chan struct{})
	unblock := func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}
	defer unblock()
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			conn := f.db.Conn(ctx)
			if _, err := conn.Exec(ctx, `SELECT id FROM payment.payments WHERE id = $1 FOR UPDATE`, payID); err != nil {
				return err
			}
			if _, err := conn.Exec(ctx, `INSERT INTO payment.refunds (id, payment_id, request_key, amount, created_at, updated_at) VALUES (gen_random_uuid(), $1, 'k', 40, $2, $2)`, payID, f.now); err != nil {
				return err
			}
			close(reached)
			<-release
			return nil
		})
	}()
	select {
	case <-reached:
	case err := <-holderDone:
		t.Fatalf("持有付款列鎖的 transaction 提早結束：%v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("持有付款列鎖的 transaction 沒有走到寫入退款")
	}
	transferDone := make(chan error, 1)
	go func() { transferDone <- f.runTransfer(tr.ID, false) }()
	deadline := time.After(30 * time.Second)
	for blocked := false; !blocked; {
		select {
		case err := <-transferDone:
			t.Fatalf("claim 在付款列被鎖時就結束了：%v", err)
		case <-deadline:
			t.Fatal("轉帳的 claim 一直沒有等在付款列的鎖上")
		case <-time.After(10 * time.Millisecond):
			blocked = f.count(t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%FOR UPDATE%'") > 0
		}
	}
	unblock()
	must(t, <-holderDone)
	must(t, <-transferDone)
	if got := f.fake.TransferAmounts(); len(got) != 1 || got[0] != 60 {
		t.Fatalf("provider 收到的轉帳金額=%v", got)
	}
	f.noViolations(t)
}
