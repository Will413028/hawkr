//go:build integration

package payment_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	kindRefundCreate = "payment.refund_create"
	kindRefundSync   = "payment.refund_sync"
)

func (f *chargeFixture) createRefund(t *testing.T, orderNo, key string, amount int64, auto bool) PaymentRefund {
	t.Helper()
	r, err := f.module.CreatePaymentRefund(context.Background(), RefundRequest{OrderNo: orderNo, Amount: amount, RequestKey: key, Auto: auto})
	must(t, err)
	return r
}

func (f *chargeFixture) runRefund(id uuid.UUID, final bool) error {
	return f.module.RefundPayment(context.Background(), id, final)
}

func (f *chargeFixture) syncRefund(t *testing.T, ctx context.Context, id uuid.UUID) bool {
	t.Helper()
	again, err := f.module.SyncPaymentRefund(ctx, id)
	must(t, err)
	return again
}

func (f *chargeFixture) refundCount(t *testing.T, orderNo string) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM payment.refunds r JOIN payment.payments p ON p.id = r.payment_id WHERE p.order_no = $1", orderNo)
}

func wantKind(t *testing.T, err error, kind apperr.Kind, what string) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Fatalf("%s：%v", what, err)
	}
}

// 建立退款：transaction 內只寫 PENDING 列與一個退款 job（在 transaction 內排入），不呼叫 provider；同一把鍵重送回同一筆、不寫第二筆也不再排 job，
// 內容不同回 FailedPrecondition；累計超過付款金額回 FailedPrecondition。
// mutation：拿掉累計上限（taken+in.Amount > pay.Amount 的檢查）→ 第 101 元的退款被接受，失敗。
// mutation：同一把鍵的查找拿掉（RefundByRequestKey 區塊）→ 第二次呼叫撞唯一索引，失敗。
func TestCreateRefundWritesThePendingRowAndTheJobEnforcesTheCapAndIsIdempotentByKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidPayment(t, "RF-1", 100)
	f.clearJobs()
	r := f.createRefund(t, "RF-1", "case-1", 40, false)
	if r.Status != RefundPending || r.Amount != 40 || r.ReversalAmount != 0 || r.PaymentID != p.ID || r.Auto {
		t.Fatalf("退款=%+v", r)
	}
	if ids := f.jobIDs(t, kindRefundCreate, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("job=%v", ids)
	}
	f.jobs.mu.Lock()
	for _, out := range f.jobs.out {
		if out {
			f.jobs.mu.Unlock()
			t.Fatal("退款 job 不是在 transaction 內排入的")
		}
	}
	f.jobs.mu.Unlock()
	if f.fake.RefundCalls() != 0 {
		t.Fatal("請求內就呼叫了 provider")
	}
	again := f.createRefund(t, "RF-1", "case-1", 40, false)
	if again.ID != r.ID || f.refundCount(t, "RF-1") != 1 || len(f.jobIDs(t, kindRefundCreate, "refund_id")) != 1 {
		t.Fatalf("同一把鍵沒有回同一筆：%+v，筆數 %d", again, f.refundCount(t, "RF-1"))
	}
	ctx := context.Background()
	_, err := f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RF-1", Amount: 41, RequestKey: "case-1"})
	wantKind(t, err, apperr.KindFailedPrecondition, "同一把鍵、不同金額")
	_, err = f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RF-1", Amount: 40, RequestKey: "case-1", Auto: true})
	wantKind(t, err, apperr.KindFailedPrecondition, "同一把鍵、不同的自動退款標記")
	f.createRefund(t, "RF-1", "case-2", 60, false)
	_, err = f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RF-1", Amount: 1, RequestKey: "case-3"})
	wantKind(t, err, apperr.KindFailedPrecondition, "累計超過付款金額")
	if f.refundCount(t, "RF-1") != 2 {
		t.Fatalf("筆數=%d", f.refundCount(t, "RF-1"))
	}
	f.noViolations(t)
}

// 建立退款的前置條件與輸入：付款不存在、未入帳被拒絕，金額與鍵不合是 InvalidInput，不能在 transaction 內呼叫。
func TestCreateRefundRefusesWhatCannotBeRefunded(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	_, err := f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RF-none", Amount: 10, RequestKey: "k"})
	wantKind(t, err, apperr.KindNotFound, "付款不存在")
	f.order("RF-2", 11, 7, 100)
	f.create(t, 11, "RF-2")
	_, err = f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RF-2", Amount: 10, RequestKey: "k"})
	wantKind(t, err, apperr.KindFailedPrecondition, "付款未入帳")
	f.paidPayment(t, "RF-3", 100)
	for name, in := range map[string]RefundRequest{
		"零元":   {OrderNo: "RF-3", Amount: 0, RequestKey: "k"},
		"負數":   {OrderNo: "RF-3", Amount: -1, RequestKey: "k"},
		"沒有鍵":  {OrderNo: "RF-3", Amount: 1, RequestKey: "  "},
		"鍵過長":  {OrderNo: "RF-3", Amount: 1, RequestKey: string(make([]byte, 129))},
		"沒有訂單": {OrderNo: " ", Amount: 1, RequestKey: "k"},
	} {
		if _, err := f.module.CreatePaymentRefund(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s：%v", name, err)
		}
	}
	if err := f.db.InTx(ctx, func(ctx context.Context) error {
		_, err := f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RF-3", Amount: 1, RequestKey: "k"})
		return err
	}); err == nil {
		t.Fatal("在 transaction 內建立退款沒有被拒絕")
	}
	if f.refundCount(t, "RF-3") != 0 {
		t.Fatal("被拒絕的請求寫了列")
	}
}

// 併發：兩筆退款（各 60，付款 100）同時進來，只有一筆成立。證明競態真的形成：第一筆停在「已持有付款列鎖、列已寫入、尚未提交」（job 排入時阻塞），
// 第二筆開始之後必須等在付款列的鎖上（pg_stat_activity 看得到 Lock 等待），而不是已經做完；等得到才放行第一筆，之後第二筆看見第一筆而被拒絕。
// mutation：LockPaymentByOrderNo 拿掉 FOR UPDATE → 第二筆不會等鎖、在第一筆提交前就做完（加總看不到未提交的列），下面「第二筆在第一筆持有鎖時就結束」失敗。
func TestTwoConcurrentRefundsThatTogetherExceedThePaymentCreateExactlyOne(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RC-1", 100)
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
	type result struct {
		r   PaymentRefund
		err error
	}
	ctx := context.Background()
	run := func(key string) chan result {
		out := make(chan result, 1)
		go func() {
			r, err := f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RC-1", Amount: 60, RequestKey: key})
			out <- result{r, err}
		}()
		return out
	}
	a := run("a")
	select {
	case <-reached:
	case <-time.After(30 * time.Second):
		t.Fatal("第一筆沒有走到排入 job")
	}
	b := run("b")
	deadline := time.After(30 * time.Second)
	for blocked := false; !blocked; {
		select {
		case rb := <-b:
			t.Fatalf("第二筆在第一筆持有付款列鎖時就結束了：%+v %v", rb.r, rb.err)
		case <-deadline:
			t.Fatal("第二筆一直沒有等在付款列的鎖上")
		case <-time.After(10 * time.Millisecond):
			blocked = f.count(t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%FOR UPDATE%'") > 0
		}
	}
	unblock()
	ra, rb := <-a, <-b
	if ra.err != nil || ra.r.Amount != 60 {
		t.Fatalf("第一筆：%+v %v", ra.r, ra.err)
	}
	wantKind(t, rb.err, apperr.KindFailedPrecondition, "第二筆")
	if f.refundCount(t, "RC-1") != 1 {
		t.Fatalf("成立了 %d 筆", f.refundCount(t, "RC-1"))
	}
}

// 已轉帳的付款先撤回再退款：退款寫 REVERSING（reversal_amount 是退款額）、排的是撤回 job 而不是退款 job；撤回 job 之前跑退款 job 什麼都不做、provider 沒有收到退款；
// 撤回成立之後由撤回 job 在同一個 transaction 排退款 job，退款 job 才呼叫 provider 並進 SUCCEEDED。
// mutation：reservableReversal 永遠回 0（拿掉「先撤回」）→ 退款寫 PENDING，下面 REVERSING 的斷言失敗。
// mutation：acceptReversal 不排退款 job → 撤回之後沒有退款 job，失敗。
func TestARefundOnATransferredPaymentReversesFirstAndOnlyThenRefunds(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	_, tr := f.transferred(t, "RV-1")
	ctx := context.Background()
	f.clearJobs()
	r := f.createRefund(t, "RV-1", "case-1", 40, false)
	if r.Status != RefundReversing || r.ReversalAmount != 40 {
		t.Fatalf("已轉帳的付款：%+v", r)
	}
	if ids := f.jobIDs(t, kindTransferReversal, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("撤回 job=%v", ids)
	}
	if len(f.jobIDs(t, kindRefundCreate, "refund_id")) != 0 {
		t.Fatal("撤回之前就排了退款 job")
	}
	must(t, f.runRefund(r.ID, false))
	if f.fake.RefundCalls() != 0 || f.refundText(t, "status", r.ID) != "REVERSING" {
		t.Fatalf("撤回之前呼叫了退款：%d 次，%s", f.fake.RefundCalls(), f.refundText(t, "status", r.ID))
	}
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.refundText(t, "status", r.ID) != "REVERSED" || f.reversedOf(t, tr.ID) != 40 || f.fake.RefundCalls() != 0 {
		t.Fatalf("撤回之後：%s，reversed %d，退款呼叫 %d", f.refundText(t, "status", r.ID), f.reversedOf(t, tr.ID), f.fake.RefundCalls())
	}
	if ids := f.jobIDs(t, kindRefundCreate, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("撤回成立之後的退款 job=%v", ids)
	}
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "SUCCEEDED" || f.refundText(t, "external_refund_id", r.ID) == "" || f.fake.Refunds() != 1 || f.fake.ReversalCalls() != 1 {
		t.Fatalf("退款之後：%s", f.refundText(t, "status", r.ID))
	}
	if keys := f.fake.RefundKeys(); len(keys) != 1 || keys[0] != f.refundText(t, "idempotency_key", r.ID) {
		t.Fatalf("退款的冪等鍵=%v", keys)
	}
	if len(f.refundChanges) != 1 || f.refundChanges[0].Status != RefundSucceeded || f.refundChanges[0].Previous != RefundReversed || len(f.autoDone) != 0 {
		t.Fatalf("事件=%+v，自動退款事件 %d", f.refundChanges, len(f.autoDone))
	}
	f.noViolations(t)
}

// 撤回額度是轉帳「還沒預留」的部分：先有一筆 REVERSING 預留了 70，第二筆 30 仍可撤 30；轉帳前就退掉的款（轉帳金額已扣掉）不再撤回；
// 預留把剩餘額度用光時，下一筆退款（款項在平台手上）直接 PENDING、不撤回。
func TestTheReversalAmountIsWhatTheTransferHasNotYetReserved(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RA-1", 100)
	f.createRefund(t, "RA-1", "early", 30, false)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RA-1")
	if tr.Amount != 70 {
		t.Fatalf("轉帳金額=%d", tr.Amount)
	}
	must(t, f.runTransfer(tr.ID, false))
	a := f.createRefund(t, "RA-1", "a", 50, false)
	b := f.createRefund(t, "RA-1", "b", 20, false)
	if a.Status != RefundReversing || a.ReversalAmount != 50 || b.Status != RefundReversing || b.ReversalAmount != 20 {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
	f.exec(t, "UPDATE payment.refunds SET status = 'FAILED', failure_reason = 'x', reversal_amount = 0 WHERE id = $1", b.ID)
	c := f.createRefund(t, "RA-1", "c", 20, false)
	if c.Status != RefundReversing || c.ReversalAmount != 20 {
		t.Fatalf("c=%+v", c)
	}
	f.exec(t, "UPDATE payment.refunds SET status = 'FAILED', failure_reason = 'x', reversal_amount = 0 WHERE id = $1 OR id = $2", c.ID, a.ID)
	f.exec(t, "UPDATE payment.transfers SET reversed_amount = 70 WHERE id = $1", tr.ID)
	d := f.createRefund(t, "RA-1", "d", 20, false)
	if d.Status != RefundPending || d.ReversalAmount != 0 {
		t.Fatalf("沒有剩餘額度的退款：%+v", d)
	}
}

// 轉帳還在進行（PENDING）或待人工核對（ANOMALY）時退款被擋下（轉帳金額已定，這時退款會讓平台多付）；轉帳成功之後可以退款。
func TestARefundIsRefusedWhileTheTransferHasNoOutcomeYet(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RT-1", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "RT-1")
	ctx := context.Background()
	_, err := f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RT-1", Amount: 10, RequestKey: "k"})
	wantKind(t, err, apperr.KindFailedPrecondition, "轉帳 PENDING")
	f.exec(t, "UPDATE payment.transfers SET status = 'ANOMALY', failure_reason = 'x' WHERE id = $1", tr.ID)
	_, err = f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "RT-1", Amount: 10, RequestKey: "k"})
	wantKind(t, err, apperr.KindFailedPrecondition, "轉帳 ANOMALY")
	if f.refundCount(t, "RT-1") != 0 {
		t.Fatal("被擋下的退款寫了列")
	}
	f.exec(t, "UPDATE payment.transfers SET status = 'FAILED' WHERE id = $1", tr.ID)
	if r := f.createRefund(t, "RT-1", "k", 10, false); r.Status != RefundPending {
		t.Fatalf("轉帳 FAILED 之後：%+v", r)
	}
}

// 退款 job：provider 在 transaction 外、帶列上的冪等鍵與付款的外部 ID；provider 回成功進 SUCCEEDED 並發 RefundChanged（transaction 內）；已終局的列再跑什麼都不做。
// 自動退款成功額外發 AutoRefundCompleted（買家與店家、金額），失敗的自動退款只發 RefundChanged。
// mutation：publishRefundResult 拿掉 AutoRefundCompleted 的發布 → 沒有自動退款事件，失敗。
// mutation：publishRefundResult 對非自動退款也發 AutoRefundCompleted → 手動退款多一個事件，失敗。
func TestTheRefundJobCallsTheProviderAndPublishesTheResultAndAutoRefundCompleted(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidPayment(t, "RJ-1", 100)
	manual := f.createRefund(t, "RJ-1", "manual", 20, false)
	auto := f.createRefund(t, "RJ-1", "auto", 80, true)
	must(t, f.runRefund(manual.ID, false))
	must(t, f.runRefund(auto.ID, false))
	must(t, f.runRefund(auto.ID, false))
	if f.fake.RefundCalls() != 2 || f.refundText(t, "status", manual.ID) != "SUCCEEDED" || f.refundText(t, "status", auto.ID) != "SUCCEEDED" {
		t.Fatalf("呼叫 %d 次", f.fake.RefundCalls())
	}
	if len(f.refundChanges) != 2 || len(f.autoDone) != 1 {
		t.Fatalf("事件：%d 個 RefundChanged，%d 個 AutoRefundCompleted", len(f.refundChanges), len(f.autoDone))
	}
	if got := f.autoDone[0]; got.RefundID != auto.ID || got.PaymentID != p.ID || got.OrderNo != "RJ-1" || got.StoreID != 7 || got.BuyerID != 11 || got.Amount != 80 {
		t.Fatalf("AutoRefundCompleted=%+v", got)
	}
	for _, c := range f.refundChanges {
		if c.RequestKey == "" || c.OrderNo != "RJ-1" || c.Status != RefundSucceeded || c.Previous != RefundPending {
			t.Fatalf("RefundChanged=%+v", c)
		}
	}
	for i, in := range f.refundEventInTx {
		if !in {
			t.Fatalf("第 %d 個退款事件不在 transaction 內", i)
		}
	}
	f.noViolations(t)
}

// 終局拒絕：進 FAILED（provider_refused）、發 RefundChanged、不發 AutoRefundCompleted，額度釋出（同額的新退款可以建立）。
// mutation：NonFailedRefundTotal 連 FAILED 一起加總 → 額度沒有釋出，最後的重建失敗。
func TestAPermanentRefundRefusalFailsTheRefundFreesTheQuotaAndPublishesOnlyRefundChanged(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RP-1", 100)
	r := f.createRefund(t, "RP-1", "auto-1", 100, true)
	f.fake.FailRefund(fmt.Errorf("%w：不能退款", ErrChargePermanent))
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "FAILED" || f.refundText(t, "failure_reason", r.ID) != "provider_refused" {
		t.Fatalf("永久拒絕之後：%s", f.refundText(t, "status", r.ID))
	}
	if len(f.refundChanges) != 1 || f.refundChanges[0].Status != RefundFailed || f.refundChanges[0].Previous != RefundPending || len(f.autoDone) != 0 {
		t.Fatalf("事件=%+v，自動退款事件 %d", f.refundChanges, len(f.autoDone))
	}
	if next := f.createRefund(t, "RP-1", "auto-2", 100, true); next.ID == r.ID || next.Status != RefundPending {
		t.Fatalf("額度沒有釋出：%+v", next)
	}
}

// 可重試拒絕（沒有執行）：第一次呼叫被拒絕才換新鍵、清掉呼叫時刻、回錯誤由 River 重試、列保持 PENDING，不計期限；之後以新鍵成功。重送被拒絕時前一次結果不明，保持原鍵。
// mutation：refund() 把 callRetryable 當 callUnknown（同鍵重送）→ 「每次換新鍵」失敗。
// mutation：重送被拒絕也換鍵（拿掉 firstCall 判斷）→ 重送那段的「不換鍵」失敗。
func TestARetryableRefundRefusalRotatesTheKeyOnlyOnTheFirstCallOfAKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RR-1", 100)
	r := f.createRefund(t, "RR-1", "k", 50, false)
	key0 := f.refundText(t, "idempotency_key", r.ID)
	f.fake.FailRefund(fmt.Errorf("%w：429", ErrChargeRetryableRefusal))
	if err := f.runRefund(r.ID, false); err == nil || !errors.Is(err, ErrChargeRetryableRefusal) {
		t.Fatalf("可重試拒絕：%v", err)
	}
	key1 := f.refundText(t, "idempotency_key", r.ID)
	if key1 == key0 || f.refundText(t, "refund_called_at", r.ID) != "" || f.refundText(t, "status", r.ID) != "PENDING" {
		t.Fatalf("換鍵之後：%s→%s，called_at=%q，%s", key0, key1, f.refundText(t, "refund_called_at", r.ID), f.refundText(t, "status", r.ID))
	}
	f.now = f.now.Add(PaymentResendWindow + time.Hour)
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "SUCCEEDED" || f.fake.Refunds() != 1 || f.refundText(t, "idempotency_key", r.ID) != key1 {
		t.Fatalf("以新鍵重試之後：%s，provider %d 筆", f.refundText(t, "status", r.ID), f.fake.Refunds())
	}
	if keys := f.fake.RefundKeys(); len(keys) != 2 || keys[0] != key0 || keys[1] != key1 {
		t.Fatalf("冪等鍵=%v", keys)
	}
	resend := f.createRefund(t, "RR-1", "k2", 50, false)
	f.fake.FailRefundAfterCreate(errors.New("回應逾時"))
	if err := f.runRefund(resend.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	keyR := f.refundText(t, "idempotency_key", resend.ID)
	f.fake.FailRefund(fmt.Errorf("%w：429", ErrChargeRetryableRefusal))
	if err := f.runRefund(resend.ID, false); err == nil {
		t.Fatal("重送被拒絕沒有回錯誤")
	}
	if f.refundText(t, "idempotency_key", resend.ID) != keyR || f.refundText(t, "refund_called_at", resend.ID) == "" {
		t.Fatal("重送被拒絕卻換了鍵")
	}
}

// 結果不明（回應遺失）：ResendWindow 內以同一把鍵重送取回同一筆退款（provider 只有一筆、不查回）；
// 過了視窗只查回：找到就收下，不建第二筆（呼叫次數仍是 1、查回 1 次）。
// mutation：過了視窗先換新鍵再呼叫 provider → provider 出現第二筆退款，失敗。
// mutation：inResendWindow 判斷拿掉（永遠重送）→ 過視窗的列不查回，LookupCalls 為 0，失敗。
func TestAnUnknownRefundIsReplayedInTheWindowAndFoundByLookupAfterItWithoutASecondRefund(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RU-1", 100)
	inWindow := f.createRefund(t, "RU-1", "in", 30, false)
	key := f.refundText(t, "idempotency_key", inWindow.ID)
	f.fake.FailRefundAfterCreate(errors.New("回應逾時"))
	if err := f.runRefund(inWindow.ID, false); err == nil || f.refundText(t, "status", inWindow.ID) != "PENDING" || f.refundText(t, "refund_called_at", inWindow.ID) == "" {
		t.Fatalf("回應遺失之後：%v，%s", err, f.refundText(t, "status", inWindow.ID))
	}
	f.now = f.now.Add(PaymentResendWindow - time.Minute)
	must(t, f.runRefund(inWindow.ID, false))
	if f.refundText(t, "status", inWindow.ID) != "SUCCEEDED" || f.fake.Refunds() != 1 || f.refundText(t, "idempotency_key", inWindow.ID) != key || f.fake.LookupCalls() != 0 {
		t.Fatalf("視窗內重送：%s，provider %d 筆，查回 %d 次", f.refundText(t, "status", inWindow.ID), f.fake.Refunds(), f.fake.LookupCalls())
	}

	past := f.createRefund(t, "RU-1", "past", 30, false)
	f.fake.FailRefundAfterCreate(errors.New("回應逾時"))
	if err := f.runRefund(past.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	calls := f.fake.RefundCalls()
	f.now = f.now.Add(PaymentResendWindow + time.Minute)
	must(t, f.runRefund(past.ID, false))
	if f.refundText(t, "status", past.ID) != "SUCCEEDED" || f.fake.Refunds() != 2 || f.fake.RefundCalls() != calls || f.fake.LookupCalls() != 1 {
		t.Fatalf("過視窗查回：%s，provider %d 筆，呼叫 %d→%d 次，查回 %d 次", f.refundText(t, "status", past.ID), f.fake.Refunds(), calls, f.fake.RefundCalls(), f.fake.LookupCalls())
	}
	f.noViolations(t)
}

// 查無：過了視窗但還沒滿 NotFoundAfter 只回錯誤稍後再查、列不動；滿了才 FAILED（provider_not_found）、額度釋出、之後不再呼叫 provider。
// mutation：lookupRefund 拿掉 NotFoundAfter 的等待 → 剛過視窗就 FAILED，第一段失敗。
func TestAnUnknownRefundNotFoundIsFailedOnlyAfterTheNotFoundWait(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RN-1", 100)
	r := f.createRefund(t, "RN-1", "k", 100, false)
	f.fake.FailRefund(errors.New("連線逾時"))
	if err := f.runRefund(r.ID, false); err == nil {
		t.Fatal("沒有到達 provider 卻沒有回錯誤")
	}
	f.now = f.now.Add(PaymentResendWindow + time.Minute)
	if err := f.runRefund(r.ID, false); err == nil || f.refundText(t, "status", r.ID) != "PENDING" {
		t.Fatalf("還沒滿等待：%v，%s", err, f.refundText(t, "status", r.ID))
	}
	f.now = f.now.Add(PaymentNotFoundAfter)
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "FAILED" || f.refundText(t, "failure_reason", r.ID) != "provider_not_found" || f.fake.RefundCalls() != 1 {
		t.Fatalf("滿了之後：%s，呼叫 %d 次", f.refundText(t, "status", r.ID), f.fake.RefundCalls())
	}
	f.createRefund(t, "RN-1", "k2", 100, false)
}

// 查回連續失敗到上限進 ANOMALY（原因 lookup_failed、發 MoneyAnomaly）：仍佔額度（超過上限的新退款被擋下）、補掃不再重排；成功的查回把次數歸零。
// mutation：NonFailedRefundTotal 排除 ANOMALY（status NOT IN ('FAILED','ANOMALY')）→ 額度沒有被佔，最後的「仍佔額度」失敗。
// mutation：lookupRefund 的 refundLookupFailed 不累計（n 永遠 0）→ 不進 ANOMALY，失敗。
func TestRepeatedRefundLookupFailuresPutTheRefundInAnomalyWhichHoldsTheQuotaAndStopsTheSweep(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RL-1", 100)
	r := f.createRefund(t, "RL-1", "k", 100, false)
	f.fake.FailRefundAfterCreate(errors.New("回應逾時"))
	if err := f.runRefund(r.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	f.now = f.now.Add(PaymentResendWindow + time.Minute)
	for i := 0; i < PaymentLookupFailureLimit-1; i++ {
		f.fake.FailLookup(errors.New("查回逾時"))
		if err := f.runRefund(r.ID, false); err == nil || f.refundText(t, "status", r.ID) != "PENDING" {
			t.Fatalf("第 %d 次查回失敗：%v，%s", i+1, err, f.refundText(t, "status", r.ID))
		}
	}
	f.fake.FailLookup(errors.New("查回逾時"))
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "ANOMALY" || f.refundText(t, "failure_reason", r.ID) != "lookup_failed" || len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyRefund || f.anomalies[0].ObjectID != r.ID {
		t.Fatalf("上限之後：%s，事件 %+v", f.refundText(t, "status", r.ID), f.anomalies)
	}
	_, err := f.module.CreatePaymentRefund(context.Background(), RefundRequest{OrderNo: "RL-1", Amount: 1, RequestKey: "more"})
	wantKind(t, err, apperr.KindFailedPrecondition, "ANOMALY 仍佔額度")
	f.clearJobs()
	f.now = f.now.Add(48 * time.Hour)
	_, _, err = f.module.SweepPayments(context.Background())
	must(t, err)
	if len(f.jobIDs(t, kindRefundCreate, "refund_id")) != 0 || len(f.jobIDs(t, kindRefundSync, "refund_id")) != 0 {
		t.Fatal("ANOMALY 的退款被補掃重排")
	}
}

// provider 回的金額與列不符（建立的回應）：進 ANOMALY（reply_mismatch）、發 MoneyAnomaly，不寫成功。
func TestARefundReplyThatDoesNotMatchTheRowGoesToAnomaly(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RM-1", 100)
	r := f.createRefund(t, "RM-1", "k", 50, false)
	f.fake.FailRefundAfterCreate(errors.New("回應逾時"))
	if err := f.runRefund(r.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	must(t, f.fake.SetRefundAmount(f.fake.RefundIDs()[0], 5))
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "ANOMALY" || f.refundText(t, "failure_reason", r.ID) != "reply_mismatch" || f.refundText(t, "external_refund_id", r.ID) != "" || len(f.anomalies) != 1 {
		t.Fatalf("金額不符：%s", f.refundText(t, "status", r.ID))
	}
}

// provider 還在處理：REFUNDING 並記外部 ID；退款事件（內容宣稱 failed，不被信任）只排同步 job、不標記已處理；同步重讀 provider（成功）才進 SUCCEEDED、發事件，
// 並把事件標為已處理。provider 回不認得的狀態時列不動、回錯誤、事件保持未處理。對不到本地退款的事件直接標記已處理、不排 job。
// mutation：handleEvent 不查退款（退款事件被當成對不到付款）→ 沒有同步 job，失敗。
// mutation：SyncRefund 拿掉 markEventsProcessed → 事件永遠未處理，失敗。
func TestARefundEventOnlyQueuesASyncThatRereadsTheProviderAndMarksTheEventProcessed(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RE-1", 100)
	f.fake.NextRefundState(ChargeRefundPending)
	r := f.createRefund(t, "RE-1", "k", 100, true)
	must(t, f.runRefund(r.ID, false))
	ext := f.refundText(t, "external_refund_id", r.ID)
	if f.refundText(t, "status", r.ID) != "REFUNDING" || ext == "" || len(f.refundChanges) != 0 {
		t.Fatalf("處理中：%s", f.refundText(t, "status", r.ID))
	}
	must(t, f.fake.SetRefundState(ext, ChargeRefundSucceeded, ""))
	f.clearJobs()
	f.receive(t, "evt-re-1", ext+"|failed")
	if ids := f.jobIDs(t, kindRefundSync, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("同步 job=%v", ids)
	}
	if f.refundText(t, "status", r.ID) != "REFUNDING" {
		t.Fatal("handler 直接改了狀態")
	}
	pending, err := f.inbox.RoutePending(context.Background(), ChargeFakeName, ext)
	must(t, err)
	if !pending {
		t.Fatal("handler 標記了事件已處理")
	}
	must(t, f.fake.SetRefundState(ext, ChargeRefundUnknown, ""))
	if _, err := f.module.SyncPaymentRefund(context.Background(), r.ID); err == nil || f.refundText(t, "status", r.ID) != "REFUNDING" {
		t.Fatalf("不認得的狀態：%v，%s", err, f.refundText(t, "status", r.ID))
	}
	must(t, f.fake.SetRefundState(ext, ChargeRefundSucceeded, ""))
	f.syncRefund(t, context.Background(), r.ID)
	if f.refundText(t, "status", r.ID) != "SUCCEEDED" || len(f.refundChanges) != 1 || f.refundChanges[0].Previous != RefundRefunding || len(f.autoDone) != 1 {
		t.Fatalf("同步之後：%s，事件 %+v", f.refundText(t, "status", r.ID), f.refundChanges)
	}
	pending, err = f.inbox.RoutePending(context.Background(), ChargeFakeName, ext)
	must(t, err)
	if pending {
		t.Fatal("同步之後事件仍未處理")
	}
	f.syncRefund(t, context.Background(), r.ID)
	if len(f.refundChanges) != 1 {
		t.Fatal("重複同步又發了事件")
	}
	f.clearJobs()
	f.receive(t, "evt-re-2", "fake_re_999")
	if len(f.jobIDs(t, kindRefundSync, "refund_id")) != 0 {
		t.Fatal("對不到本地退款的事件排了 job")
	}
	f.noViolations(t)
}

// REFUNDING 之後 provider 回失敗：進 FAILED（記原因碼）、額度釋出、發 RefundChanged；已終局的列不被過期的「處理中」讀取倒退。
func TestARefundThatFailsAtTheProviderAfterBeingAcceptedFreesTheQuota(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RG-1", 100)
	f.fake.NextRefundState(ChargeRefundPending)
	r := f.createRefund(t, "RG-1", "k", 100, true)
	must(t, f.runRefund(r.ID, false))
	ext := f.refundText(t, "external_refund_id", r.ID)
	must(t, f.fake.SetRefundState(ext, ChargeRefundFailed, "declined"))
	f.syncRefund(t, context.Background(), r.ID)
	if f.refundText(t, "status", r.ID) != "FAILED" || f.refundText(t, "failure_reason", r.ID) != "provider_failed:declined" || len(f.autoDone) != 0 {
		t.Fatalf("失敗之後：%s %s", f.refundText(t, "status", r.ID), f.refundText(t, "failure_reason", r.ID))
	}
	must(t, f.fake.SetRefundState(ext, ChargeRefundPending, ""))
	f.syncRefund(t, context.Background(), r.ID)
	if f.refundText(t, "status", r.ID) != "FAILED" || len(f.refundChanges) != 1 || len(f.anomalies) != 0 {
		t.Fatalf("過期的讀取倒退了終局：%s，事件 %d，異常 %d", f.refundText(t, "status", r.ID), len(f.refundChanges), len(f.anomalies))
	}
	f.createRefund(t, "RG-1", "again", 100, false)
}

// 撤回的終局失敗讓退款進 FAILED 時也發 RefundChanged（Previous 是 REVERSING）。
func TestAFailedReversalPublishesRefundChanged(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.transferred(t, "RX-1")
	r := f.createRefund(t, "RX-1", "k", 40, false)
	f.fake.FailReversal(fmt.Errorf("%w：不能撤回", ErrChargePermanent))
	must(t, f.module.ReversePaymentTransfer(context.Background(), r.ID, false))
	if len(f.refundChanges) != 1 || f.refundChanges[0].Status != RefundFailed || f.refundChanges[0].Previous != RefundReversing || f.refundChanges[0].RefundID != r.ID {
		t.Fatalf("事件=%+v", f.refundChanges)
	}
}

// 補掃：閒置的 PENDING 與 REVERSED 退款補排退款 job、閒置的 REFUNDING 補排退款同步、有舊的未處理事件的退款補排同步；剛寫入的與 ANOMALY、終局的不排。
// mutation：掃描不查可呼叫的退款（CallableRefundIDs）→ 退款 job 沒有補排，失敗。
// mutation：掃描不查 REFUNDING（RefundingRefundIDs）→ 同步 job 沒有補排，失敗。
func TestTheSweepRequeuesIdleCallableAndRefundingRefundsButNotAnomalyOrFinalOnes(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidPayment(t, "RS-1", 1000)
	ctx := context.Background()
	pending := f.insertRefund(t, p.ID, "PENDING", 10, 0)
	reversed := f.insertRefund(t, p.ID, "REVERSED", 10, 0)
	f.insertRefund(t, p.ID, "ANOMALY", 10, 0)
	done := f.insertRefund(t, p.ID, "PENDING", 10, 0)
	f.exec(t, "UPDATE payment.refunds SET status = 'SUCCEEDED', refund_called_at = now(), external_refund_id = 'x_done' WHERE id = $1", done)
	f.insertRefund(t, p.ID, "FAILED", 10, 0)
	f.clearJobs()
	_, _, err := f.module.SweepPayments(ctx)
	must(t, err)
	if len(f.jobIDs(t, kindRefundCreate, "refund_id")) != 0 {
		t.Fatal("剛寫入的退款被補排")
	}
	f.now = f.now.Add(time.Hour)
	_, _, err = f.module.SweepPayments(ctx)
	must(t, err)
	got := map[uuid.UUID]bool{}
	for _, id := range f.jobIDs(t, kindRefundCreate, "refund_id") {
		got[id] = true
	}
	if len(got) != 2 || !got[pending] || !got[reversed] {
		t.Fatalf("補排的退款 job=%v", got)
	}
	if len(f.jobIDs(t, kindRefundSync, "refund_id")) != 0 {
		t.Fatal("沒有 REFUNDING 卻排了同步")
	}
	f.fake.NextRefundState(ChargeRefundPending)
	f.paidPayment(t, "RS-2", 100)
	r := f.createRefund(t, "RS-2", "k", 100, false)
	must(t, f.runRefund(r.ID, false))
	f.clearJobs()
	f.now = f.now.Add(time.Hour)
	_, _, err = f.module.SweepPayments(ctx)
	must(t, err)
	if ids := f.jobIDs(t, kindRefundSync, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("閒置的 REFUNDING 沒有補排同步：%v", ids)
	}
	ext := f.refundText(t, "external_refund_id", r.ID)
	f.exec(t, "UPDATE payment.refunds SET updated_at = $2 WHERE id = $1", r.ID, f.now)
	f.receive(t, "evt-rs-1", ext)
	f.clearJobs()
	f.exec(t, "UPDATE inbox.events SET received_at = $1 WHERE event_id = 'evt-rs-1'", f.now.Add(-time.Hour))
	f.now = f.now.Add(5 * time.Minute)
	_, _, err = f.module.SweepPayments(ctx)
	must(t, err)
	if ids := f.jobIDs(t, kindRefundSync, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("有舊的未處理退款事件的退款沒有補排同步：%v", ids)
	}
}

// 退款階段的 ANOMALY 人工結案：成功之前向 provider 以列上的嘗試 ID 查回證據（FindRefund）：亂貼的 ID、金額不符被擋下、列不動、沒有稽核紀錄；相符才結案成 REFUNDING 並排同步 job，
// 同 transaction 寫一筆 adminaudit；失敗結案進 FAILED、額度釋出；撤回階段與退款階段互不混用。
// mutation：resolveRefund 的退款階段拿掉 verifyEvidence 的退款分支（inReversalStage 永遠 true）→ 退款階段的結案走撤回路徑而失敗。
func TestResolvingARefundStageAnomalyChecksTheProviderEvidenceAndHandsBackToTheSync(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.paidPayment(t, "RZ-1", 100)
	r := f.createRefund(t, "RZ-1", "k", 100, false)
	f.fake.FailRefundAfterCreate(errors.New("回應逾時"))
	if err := f.runRefund(r.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	must(t, f.fake.SetRefundAmount(f.fake.RefundIDs()[0], 1))
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "ANOMALY" {
		t.Fatalf("前置：%s", f.refundText(t, "status", r.ID))
	}
	ext := f.fake.RefundIDs()[0]
	err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyRefund, r.ID, AnomalySucceeded, "re_typo")
	wantKind(t, err, apperr.KindFailedPrecondition, "亂貼的 ID")
	err = f.module.ResolveAnomaly(ctx, f.admin(), AnomalyRefund, r.ID, AnomalySucceeded, ext)
	wantKind(t, err, apperr.KindFailedPrecondition, "金額不符")
	if f.refundText(t, "status", r.ID) != "ANOMALY" || f.auditCount(t) != 0 {
		t.Fatalf("被擋下的結案改動了列：%s", f.refundText(t, "status", r.ID))
	}
	must(t, f.fake.SetRefundAmount(ext, 100))
	f.clearJobs()
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyRefund, r.ID, AnomalySucceeded, ext))
	if f.refundText(t, "status", r.ID) != "REFUNDING" || f.refundText(t, "external_refund_id", r.ID) != ext || f.auditCount(t) != 1 {
		t.Fatalf("相符之後：%s", f.refundText(t, "status", r.ID))
	}
	if ids := f.jobIDs(t, kindRefundSync, "refund_id"); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("同步 job=%v", ids)
	}
	f.syncRefund(t, ctx, r.ID)
	if f.refundText(t, "status", r.ID) != "SUCCEEDED" {
		t.Fatalf("同步之後：%s", f.refundText(t, "status", r.ID))
	}

	f.paidPayment(t, "RZ-2", 100)
	bad := f.createRefund(t, "RZ-2", "k", 100, false)
	f.exec(t, "UPDATE payment.refunds SET status = 'ANOMALY', failure_reason = 'lookup_failed' WHERE id = $1", bad.ID)
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyRefund, bad.ID, AnomalyFailed, ""))
	if f.refundText(t, "status", bad.ID) != "FAILED" || f.refundText(t, "failure_reason", bad.ID) != "admin_resolved" {
		t.Fatalf("結案成失敗：%s", f.refundText(t, "status", bad.ID))
	}
	f.createRefund(t, "RZ-2", "again", 100, false)
}

// 撤回階段的 ANOMALY 人工結案成功之後，由結案的 transaction 排退款 job（退款要接著往下）。
func TestResolvingAReversalStageAnomalyQueuesTheRefundJob(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, tr := f.transferred(t, "RY-1")
	refund := f.insertRefund(t, p.ID, "ANOMALY", 30, 30)
	rev, err := f.fake.ReverseTransfer(ctx, ChargeReversalInput{ExternalTransferID: f.transferText(t, "external_transfer_id", tr.ID), Amount: 30, IdempotencyKey: f.refundText(t, "reversal_idempotency_key", refund)})
	must(t, err)
	f.clearJobs()
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyRefund, refund, AnomalySucceeded, rev.ExternalID))
	if ids := f.jobIDs(t, kindRefundCreate, "refund_id"); len(ids) != 1 || ids[0] != refund {
		t.Fatalf("結案之後的退款 job=%v", ids)
	}
}

// F1：累計上限的比較不溢位。付款 100、已有一筆 40，再要求 MaxInt64：taken+amount 回繞成負數會通過檢查。
// mutation：refund-cap-overflow（改回 taken+in.Amount > pay.Amount）→ 這筆被接受，失敗。
func TestARefundOfMaxInt64AfterAnExistingRefundIsRefusedByTheCap(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "RX-1", 100)
	f.createRefund(t, "RX-1", "a", 40, false)
	_, err := f.module.CreatePaymentRefund(context.Background(), RefundRequest{OrderNo: "RX-1", Amount: math.MaxInt64, RequestKey: "b"})
	wantKind(t, err, apperr.KindFailedPrecondition, "巨額退款")
	if f.refundCount(t, "RX-1") != 1 {
		t.Fatal("巨額退款寫了列")
	}
}

// F2：同步讀取期間收到的退款事件不被併進執行中的 job 而遺失：同步回 again，由 worker 稍後再跑一輪。
// mutation：refund-sync-no-again（SyncRefund 結尾改回直接 return nil, nil）→ again 為 false，失敗。
func TestARefundEventArrivingWhileTheSyncReadsTheProviderAsksForAnotherRound(t *testing.T) {
	t.Parallel()
	var f *chargeFixture
	var ext string
	var fired bool
	f = setupChargeWith(t, func(fake *ChargeFake) ChargeProvider {
		return &afterGetRefund{ChargeProvider: fake, after: func() {
			if fired || ext == "" {
				return
			}
			fired = true
			f.now = f.now.Add(time.Minute)
			f.receive(t, "evt-during", ext)
		}}
	})
	f.paidPayment(t, "RX-2", 100)
	f.fake.NextRefundState(ChargeRefundPending)
	r := f.createRefund(t, "RX-2", "k", 100, false)
	must(t, f.runRefund(r.ID, false))
	ext = f.refundText(t, "external_refund_id", r.ID)
	must(t, f.fake.SetRefundState(ext, ChargeRefundSucceeded, ""))
	f.now = f.now.Add(time.Minute)
	if again := f.syncRefund(t, context.Background(), r.ID); !again {
		t.Fatal("讀取期間收到的事件沒有要求再跑一輪")
	}
	f.now = f.now.Add(time.Minute)
	if again := f.syncRefund(t, context.Background(), r.ID); again {
		t.Fatal("事件已涵蓋卻仍要求再跑")
	}
}

type afterGetRefund struct {
	ChargeProvider
	after func()
}

func (a *afterGetRefund) GetRefund(ctx context.Context, externalID string) (ChargeRefundSnapshot, error) {
	snap, err := a.ChargeProvider.GetRefund(ctx, externalID)
	a.after()
	return snap, err
}

// F3：provider 已改成別的（付款屬於舊 provider）時退款在建立就被拒絕，不先撤回賣家的轉帳、不寫退款列也不排 job。
// mutation：refund-create-no-refundable-check（拿掉 CreateRefund 的 refundable 檢查）→ 寫了 REVERSING 列並排撤回 job，失敗。
func TestARefundForAPaymentOfAnotherProviderIsRefusedBeforeAnyReversal(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p, tr := f.transferred(t, "RX-3")
	f.exec(t, "UPDATE payment.payments SET provider = 'old_provider' WHERE id = $1", p.ID)
	f.clearJobs()
	_, err := f.module.CreatePaymentRefund(context.Background(), RefundRequest{OrderNo: "RX-3", Amount: 40, RequestKey: "k"})
	wantKind(t, err, apperr.KindFailedPrecondition, "別的 provider 的付款")
	if f.refundCount(t, "RX-3") != 0 || f.reversedOf(t, tr.ID) != 0 || len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 || len(f.jobIDs(t, kindRefundCreate, "refund_id")) != 0 {
		t.Fatal("被拒絕的退款留下了列或 job")
	}
}

// F4：別的 provider 的付款上的退款：事件對不到（不排同步 job、事件標已處理），直接呼叫同步也不讀、不改列、不發 MoneyAnomaly。
// mutation：refund-event-no-provider-filter（RefundByExternal 拿掉 provider 條件）→ 排了同步 job，失敗；refund-sync-no-provider-skip（SyncRefund 拿掉 provider 比對）→ 列被改成 SUCCEEDED，失敗。
func TestARefundOfAPaymentOfAnotherProviderIsNeitherRoutedNorSynced(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidPayment(t, "RX-4", 100)
	f.fake.NextRefundState(ChargeRefundPending)
	r := f.createRefund(t, "RX-4", "k", 100, false)
	must(t, f.runRefund(r.ID, false))
	ext := f.refundText(t, "external_refund_id", r.ID)
	f.exec(t, "UPDATE payment.payments SET provider = 'old_provider' WHERE id = $1", p.ID)
	must(t, f.fake.SetRefundState(ext, ChargeRefundSucceeded, ""))
	f.clearJobs()
	f.receive(t, "evt-old", ext)
	if len(f.jobIDs(t, kindRefundSync, "refund_id")) != 0 || !f.processed(t, "evt-old") {
		t.Fatalf("別的 provider 的退款事件排了同步或沒標已處理：%v", f.jobIDs(t, kindRefundSync, "refund_id"))
	}
	f.syncRefund(t, context.Background(), r.ID)
	if f.refundText(t, "status", r.ID) != "REFUNDING" || len(f.anomalies) != 0 || len(f.refundChanges) != 0 {
		t.Fatalf("同步動了別的 provider 的退款：%s，事件 %+v", f.refundText(t, "status", r.ID), f.anomalies)
	}
}

// F5：退款階段結案成功時，鎖住付款列後要確認付款的外部 ID 仍是核對證據時的那個；核對之後被改掉回 FailedPrecondition，列不動。
// mutation：refund-resolve-parent-unchecked（拿掉對 pay.ExternalPaymentID 的比對）→ 結案成功，失敗。
func TestResolvingARefundStageAnomalyFailsWhenThePaymentExternalIDChangedAfterTheEvidence(t *testing.T) {
	t.Parallel()
	var f *chargeFixture
	var paymentID uuid.UUID
	f = setupChargeWith(t, func(fake *ChargeFake) ChargeProvider {
		return &afterFindRefund{ChargeProvider: fake, after: func() {
			f.exec(t, "UPDATE payment.payments SET external_payment_id = 'pi_changed' WHERE id = $1", paymentID)
		}}
	})
	p := f.paidPayment(t, "RX-5", 100)
	paymentID = p.ID
	r := f.createRefund(t, "RX-5", "k", 100, false)
	f.fake.FailRefundAfterCreate(errors.New("回應逾時"))
	if err := f.runRefund(r.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	must(t, f.fake.SetRefundAmount(f.fake.RefundIDs()[0], 1))
	must(t, f.runRefund(r.ID, false))
	ext := f.fake.RefundIDs()[0]
	must(t, f.fake.SetRefundAmount(ext, 100))
	err := f.module.ResolveAnomaly(context.Background(), f.admin(), AnomalyRefund, r.ID, AnomalySucceeded, ext)
	wantKind(t, err, apperr.KindFailedPrecondition, "付款外部 ID 變了")
	if f.refundText(t, "status", r.ID) != "ANOMALY" || f.auditCount(t) != 0 {
		t.Fatalf("結案改動了列：%s", f.refundText(t, "status", r.ID))
	}
}

type afterFindRefund struct {
	ChargeProvider
	after func()
}

func (a *afterFindRefund) FindRefund(ctx context.Context, externalPaymentID, attemptID string) (ChargeRefundSnapshot, bool, error) {
	snap, found, err := a.ChargeProvider.FindRefund(ctx, externalPaymentID, attemptID)
	a.after()
	return snap, found, err
}
