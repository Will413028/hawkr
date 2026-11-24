//go:build integration

package payment_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var errLost = errors.New("回應逾時")

func (f *chargeFixture) admin() AdminActor {
	return AdminActor{AdminID: 5, Action: "/hawkr.admin.v1alpha1.PaymentAdminService/ResolveAnomaly"}
}

func (f *chargeFixture) auditCount(t *testing.T) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM adminaudit.records")
}

// unreachedTransfer 建立一筆轉帳並讓第一次呼叫沒有到達 provider（結果不明，provider 那邊沒有）。
func (f *chargeFixture) unreachedTransfer(t *testing.T, orderNo string) PaymentTransfer {
	t.Helper()
	f.paidPayment(t, orderNo, 100)
	tr := f.requestTransfer(t, orderNo)
	f.fake.FailTransfer(errLost)
	if err := f.runTransfer(tr.ID, false); err == nil {
		t.Fatal("呼叫沒有到達 provider 卻沒有回錯誤")
	}
	return tr
}

// lostTransfer 建立一筆轉帳並讓第一次呼叫「已建立、回應遺失」（結果不明，provider 那邊有一筆）。
func (f *chargeFixture) lostTransfer(t *testing.T, orderNo string) PaymentTransfer {
	t.Helper()
	f.paidPayment(t, orderNo, 100)
	tr := f.requestTransfer(t, orderNo)
	f.fake.FailTransferAfterCreate(errLost)
	if err := f.runTransfer(tr.ID, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	return tr
}

func (f *chargeFixture) pastResendWindow() { f.now = f.now.Add(PaymentResendWindow + time.Minute) }

// 轉帳結果不明、過了 ResendWindow：只查回，找到就收下，不建第二筆、不再呼叫 CreateTransfer、鍵不變。
// mutation：transfer() 在過了視窗之後改成先換新鍵再呼叫 CreateTransfer（查回之前就換鍵）→ provider 出現第二筆轉帳，失敗。
func TestAnUnknownTransferFoundByLookupCreatesNoSecondTransfer(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.lostTransfer(t, "C-1")
	key := f.transferText(t, "idempotency_key", tr.ID)
	calls := f.fake.TransferCalls()
	f.pastResendWindow()
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.transferText(t, "external_transfer_id", tr.ID) != f.fake.TransferIDs()[0] {
		t.Fatalf("查回沒有收下：%s", f.transferText(t, "status", tr.ID))
	}
	if f.fake.Transfers() != 1 || f.fake.TransferCalls() != calls || f.fake.LookupCalls() != 1 || f.transferText(t, "idempotency_key", tr.ID) != key {
		t.Fatalf("轉帳 %d 筆、呼叫 %d → %d、查回 %d 次", f.fake.Transfers(), calls, f.fake.TransferCalls(), f.fake.LookupCalls())
	}
	f.noViolations(t)
}

// 查無：要滿 NotFoundAfter 才算確定不存在；之前仍是 PENDING、佔著唯一索引（RequestTransfer 回同一筆、不建新的）、不呼叫 CreateTransfer；
// 滿了才 FAILED，之後 RequestTransfer 才建得出新的一筆、換新鍵。
// mutation：lookupTransfer 拿掉 NotFoundAfter 判斷（查無立刻 FAILED）→ 「滿等待之前仍是 PENDING」失敗。
func TestAnUnknownTransferNotFoundIsRebuiltOnlyAfterTheNotFoundWait(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.enableAccount(t, 7, "acct_seller_7", true)
	f.paidPayment(t, "C-2", 100)
	tr := f.requestTransfer(t, "C-2")
	f.fake.FailTransfer(errLost)
	if err := f.runTransfer(tr.ID, false); err == nil {
		t.Fatal("呼叫沒有到達 provider 卻沒有回錯誤")
	}
	start := f.now
	calls := f.fake.TransferCalls()
	f.now = start.Add(PaymentResendWindow + time.Minute)
	if err := f.runTransfer(tr.ID, false); err == nil || f.transferText(t, "status", tr.ID) != "PENDING" {
		t.Fatalf("滿等待之前：%v，%s", err, f.transferText(t, "status", tr.ID))
	}
	if again := f.requestTransfer(t, "C-2"); again.ID != tr.ID || f.count(t, "SELECT count(*) FROM payment.transfers") != 1 {
		t.Fatalf("滿等待之前就能建新的一筆：%+v", again)
	}
	f.now = start.Add(PaymentNotFoundAfter + time.Minute)
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "FAILED" || f.transferText(t, "failure_reason", tr.ID) != "provider_not_found" || f.fake.TransferCalls() != calls {
		t.Fatalf("滿等待之後：%s，呼叫 %d → %d", f.transferText(t, "status", tr.ID), calls, f.fake.TransferCalls())
	}
	next := f.requestTransfer(t, "C-2")
	if next.ID == tr.ID || next.Status != TransferPending || f.transferText(t, "idempotency_key", next.ID) == f.transferText(t, "idempotency_key", tr.ID) {
		t.Fatalf("重建=%+v", next)
	}
}

// 查回連續失敗到上限：進 ANOMALY（原因 lookup_failed）、發 MoneyAnomaly（在 transaction 內）、佔住唯一索引（再請求回同一筆、不能手寫第二筆有效轉帳）、補掃與 job 都不再碰它。
// 成功的查回把失敗次數清零，所以間斷的失敗不累計。
// mutation：schema 的 transfers_active_payment 不含 ANOMALY → 手寫第二筆 PENDING 不再違反唯一索引，失敗。
// mutation：transferLookupFailed 不累計（永遠不進 ANOMALY）→ 狀態仍是 PENDING，失敗。
func TestRepeatedLookupFailuresPutTheTransferInAnomalyWhichHoldsTheUniqueSlot(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.unreachedTransfer(t, "C-3")
	f.pastResendWindow()
	boom := errors.New("查回逾時")
	f.fake.FailLookup(boom, boom)
	for i := 0; i < 2; i++ {
		if err := f.runTransfer(tr.ID, false); !errors.Is(err, boom) {
			t.Fatalf("第 %d 次：%v", i+1, err)
		}
	}
	if f.transferText(t, "lookup_failures", tr.ID) != "2" {
		t.Fatalf("失敗次數=%s", f.transferText(t, "lookup_failures", tr.ID))
	}
	// 一次成功的查回（查無、還沒滿等待）清零。
	if err := f.runTransfer(tr.ID, false); err == nil || f.transferText(t, "lookup_failures", tr.ID) != "0" {
		t.Fatalf("成功查回之後：%v，失敗次數 %s", err, f.transferText(t, "lookup_failures", tr.ID))
	}
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f.fake.FailLookup(boom)
		err := f.runTransfer(tr.ID, false)
		if (i < PaymentLookupFailureLimit-1) != (err != nil) {
			t.Fatalf("第 %d 次查回失敗：%v", i+1, err)
		}
	}
	if f.transferText(t, "status", tr.ID) != "ANOMALY" || f.transferText(t, "failure_reason", tr.ID) != "lookup_failed" {
		t.Fatalf("狀態=%s", f.transferText(t, "status", tr.ID))
	}
	if len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyTransfer || f.anomalies[0].ObjectID != tr.ID || !f.anomalyInTx[0] {
		t.Fatalf("告警事件=%+v 在 transaction 內=%v", f.anomalies, f.anomalyInTx)
	}
	if again := f.requestTransfer(t, "C-3"); again.ID != tr.ID || again.Status != TransferAnomaly || f.count(t, "SELECT count(*) FROM payment.transfers") != 1 {
		t.Fatalf("ANOMALY 沒有佔住位置：%+v", again)
	}
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.transfers (id, payment_order_id, store_id, amount, destination_account_id, created_at, updated_at)
VALUES ($1, (SELECT id FROM payment.payment_orders WHERE payment_id = $2), 7, 100, 'acct_seller_7', now(), now())`, uuid.NewV7(), f.idOf(t, "C-3"))
	if err == nil {
		t.Fatal("ANOMALY 的轉帳存在時又寫入第二筆有效轉帳")
	}
	calls, lookups := f.fake.TransferCalls(), f.fake.LookupCalls()
	must(t, f.runTransfer(tr.ID, false))
	f.clearJobs()
	f.now = f.now.Add(72 * time.Hour)
	_, _, swept := f.module.SweepPayments(ctx)
	must(t, swept)
	if len(f.jobIDs(t, kindTransferCreate, "transfer_id")) != 0 || f.fake.TransferCalls() != calls || f.fake.LookupCalls() != lookups {
		t.Fatal("ANOMALY 的轉帳又被排 job 或呼叫 provider")
	}
}

// provider 回的轉帳與列不符（金額不同）進 ANOMALY，不寫成 SUCCEEDED：查回找到的與呼叫回應的都一樣。
// mutation：acceptTransfer 拿掉金額比對 → 以錯誤金額記成 SUCCEEDED，失敗。
// 列上的 failure_reason 只存原因碼（上限 200 字）；外部 ID 只進事件與 log。
func TestAMismatchedProviderTransferIsAnAnomalyNotASuccess(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.lostTransfer(t, "C-4")
	must(t, f.fake.SetTransferAmount(f.fake.TransferIDs()[0], 1))
	f.pastResendWindow()
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "ANOMALY" || f.transferText(t, "failure_reason", tr.ID) != "reply_mismatch" || f.transferText(t, "external_transfer_id", tr.ID) != "" {
		t.Fatalf("查回對不起來：%s %s", f.transferText(t, "status", tr.ID), f.transferText(t, "failure_reason", tr.ID))
	}
	if len(f.anomalies) != 1 || !strings.Contains(f.anomalies[0].Reason, f.fake.TransferIDs()[0]) || !strings.Contains(f.logs.String(), f.fake.TransferIDs()[0]) {
		t.Fatalf("外部 ID 應只進事件與 log：%+v", f.anomalies)
	}
	// 建立的回應不符：provider 回的金額是錯的。
	f2 := setupCharge(t)
	f2.enableAccount(t, 7, "acct_seller_7", true)
	f2.paidPayment(t, "C-4b", 100)
	tr2 := f2.requestTransfer(t, "C-4b")
	f2.fake.FailTransferAfterCreate(errLost)
	_ = f2.runTransfer(tr2.ID, false)
	must(t, f2.fake.SetTransferAmount(f2.fake.TransferIDs()[0], 2))
	must(t, f2.runTransfer(tr2.ID, false))
	if f2.transferText(t, "status", tr2.ID) != "ANOMALY" {
		t.Fatalf("重送回的金額不符：%s", f2.transferText(t, "status", tr2.ID))
	}
}

// 前置條件異常（付款沒有 charge）：轉帳進 ANOMALY、不呼叫 provider、補掃不再重排。
func TestATransferWhoseGroundsAreMissingParksInAnomalyAndTheSweepStops(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("C-5", 11, 7, 100)
	p := f.create(t, 11, "C-5")
	id := uuid.NewV7()
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.transfers (id, payment_order_id, store_id, amount, destination_account_id, created_at, updated_at)
VALUES ($1, (SELECT id FROM payment.payment_orders WHERE payment_id = $2), 7, 100, 'acct_seller_7', $3, $3)`, id, p.ID, f.now)
	must(t, err)
	must(t, f.runTransfer(id, false))
	if f.transferText(t, "status", id) != "ANOMALY" || f.transferText(t, "failure_reason", id) != "payment_without_charge" || f.fake.TransferCalls() != 0 {
		t.Fatalf("狀態=%s", f.transferText(t, "status", id))
	}
	f.clearJobs()
	f.now = f.now.Add(48 * time.Hour)
	_, _, err = f.module.SweepPayments(ctx)
	must(t, err)
	if len(f.jobIDs(t, kindTransferCreate, "transfer_id")) != 0 {
		t.Fatal("ANOMALY 的轉帳被補掃重排")
	}
}

// 後台人工結案（轉帳）：成功要附 provider 物件 ID，記成 SUCCEEDED；失敗讓出唯一索引、之後可重建；都在同一個 transaction 寫一筆 adminaudit；
// 寫入失敗（證據與別的轉帳衝突）時紀錄一起回滾；不在 ANOMALY 的不能結案。
// mutation：ResolveAnomaly 拿掉 Audit.Record → 稽核筆數為 0，失敗。
func TestResolvingATransferAnomalyWritesTheAuditInTheSameTransaction(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.enableAccount(t, 7, "acct_seller_7", true)
	a := f.lostTransfer(t, "C-6a")
	b := f.unreachedTransfer(t, "C-6b")
	f.pastResendWindow()
	boom := errors.New("查回逾時")
	for _, id := range []uuid.UUID{a.ID, b.ID} {
		for i := 0; i < PaymentLookupFailureLimit; i++ {
			f.fake.FailLookup(boom)
			_ = f.runTransfer(id, false)
		}
		if f.transferText(t, "status", id) != "ANOMALY" {
			t.Fatalf("沒有進 ANOMALY：%s", f.transferText(t, "status", id))
		}
	}
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, a.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", " "); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("成功沒有證據：%v", err)
	}
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, uuid.NewV7(), AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", ""); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在：%v", err)
	}
	ids := f.fake.TransferIDs()
	if len(ids) != 1 {
		t.Fatalf("provider 的轉帳=%v", ids)
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, a.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", ids[0]))
	if f.transferText(t, "status", a.ID) != "SUCCEEDED" || f.transferText(t, "external_transfer_id", a.ID) != ids[0] || f.transferText(t, "failure_reason", a.ID) != "" || f.auditCount(t) != 1 {
		t.Fatalf("成功結案：%s，稽核 %d 筆", f.transferText(t, "status", a.ID), f.auditCount(t))
	}
	detail := f.text(t, "SELECT detail::text FROM adminaudit.records WHERE target_id = $1", a.ID.String())
	if !strings.Contains(detail, ids[0]) || !strings.Contains(detail, "lookup_failed") || f.text(t, "SELECT target_type FROM adminaudit.records WHERE target_id = $1", a.ID.String()) != "payment_transfer" {
		t.Fatalf("稽核內容=%s", detail)
	}
	// 證據指到別的轉帳的 provider ID：核對 b 的嘗試 ID 查不到它，結案被擋下，列與稽核都不動。
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, b.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", ids[0]); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("證據指到別的轉帳：%v", err)
	}
	if f.transferText(t, "status", b.ID) != "ANOMALY" || f.auditCount(t) != 1 {
		t.Fatalf("衝突沒有回滾：%s，稽核 %d 筆", f.transferText(t, "status", b.ID), f.auditCount(t))
	}
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, a.ID, AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", ""); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已不在 ANOMALY：%v", err)
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, b.ID, AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", ""))
	if f.transferText(t, "status", b.ID) != "FAILED" || f.auditCount(t) != 2 {
		t.Fatalf("失敗結案：%s，稽核 %d 筆", f.transferText(t, "status", b.ID), f.auditCount(t))
	}
	if next := f.requestTransfer(t, "C-6b"); next.ID == b.ID || next.Status != TransferPending {
		t.Fatalf("結案成失敗之後沒有讓出位置：%+v", next)
	}
}

// 撤回：結果不明過了視窗查回找到 → 收下（累加轉帳、撤回 REVERSED），不再呼叫 ReverseTransfer；查無滿等待才釋出預留（RELEASED，退款維持原狀）；
// 可重試拒絕換新的撤回鍵；查回連續失敗進 ANOMALY，ANOMALY 的退款仍佔累計退款額度（再退一元被拒）。
// mutation：reverse() 在過了視窗之後仍呼叫 ReverseTransfer → 呼叫次數增加或重複累加，失敗。
// mutation：NonFailedRefundTotal 排除 ANOMALY（status NOT IN ('FAILED','ANOMALY')）→ 額度沒有被佔，最後一段失敗。
func TestAReversalConvergesByLookupAndAnAnomalousRefundStillHoldsItsQuota(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, tr := f.transferred(t, "C-7")
	found := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 10, 10)
	f.fake.FailReversalAfterCreate(errLost)
	if err := f.module.ReversePaymentTransfer(ctx, found, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	calls := f.fake.ReversalCalls()
	start := f.now
	f.now = start.Add(PaymentResendWindow + time.Minute)
	must(t, f.module.ReversePaymentTransfer(ctx, found, false))
	if f.refundText(t, "reversal_status", found) != "REVERSED" || f.reversedOf(t, tr.ID) != 10 || f.fake.ReversalCalls() != calls {
		t.Fatalf("查回收下：%s，reversed %d，呼叫 %d → %d", f.refundText(t, "reversal_status", found), f.reversedOf(t, tr.ID), calls, f.fake.ReversalCalls())
	}
	must(t, f.module.ReversePaymentTransfer(ctx, found, false))
	if f.reversedOf(t, tr.ID) != 10 {
		t.Fatalf("重複累加：%d", f.reversedOf(t, tr.ID))
	}

	notFound := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 10, 10)
	f.fake.FailReversal(errLost)
	if err := f.module.ReversePaymentTransfer(ctx, notFound, false); err == nil {
		t.Fatal("呼叫沒有到達 provider 卻沒有回錯誤")
	}
	start = f.now
	f.now = start.Add(PaymentResendWindow + time.Minute)
	if err := f.module.ReversePaymentTransfer(ctx, notFound, false); err == nil || f.refundText(t, "reversal_status", notFound) != "REVERSING" {
		t.Fatalf("滿等待之前：%v，%s", err, f.refundText(t, "reversal_status", notFound))
	}
	f.now = start.Add(PaymentNotFoundAfter + time.Minute)
	must(t, f.module.ReversePaymentTransfer(ctx, notFound, false))
	if f.refundText(t, "reversal_status", notFound) != "RELEASED" || f.refundText(t, "reversal_reason", notFound) != "provider_not_found" || f.refundText(t, "status", notFound) != "REFUNDING" {
		t.Fatalf("查無滿等待：%s", f.refundText(t, "reversal_status", notFound))
	}

	refusal := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 10, 10)
	oldKey := f.segText(t, "idempotency_key", refusal)
	f.fake.FailReversal(fmt.Errorf("%w：429", ErrChargeRetryableRefusal))
	if err := f.module.ReversePaymentTransfer(ctx, refusal, false); !errors.Is(err, ErrChargeRetryableRefusal) {
		t.Fatalf("可重試拒絕：%v", err)
	}
	if f.segText(t, "idempotency_key", refusal) == oldKey || f.segText(t, "called_at", refusal) != "" || f.refundText(t, "reversal_status", refusal) != "REVERSING" {
		t.Fatal("可重試拒絕沒有換新的撤回鍵")
	}
	f.now = f.now.Add(40 * time.Hour)
	must(t, f.module.ReversePaymentTransfer(ctx, refusal, false))
	if f.refundText(t, "reversal_status", refusal) != "REVERSED" {
		t.Fatalf("超過舊期限的重試：%s", f.refundText(t, "reversal_status", refusal))
	}

	// 查回連續失敗：撤回進 ANOMALY；它的 10 仍佔額度。
	stuckPayment := f.paidPayment(t, "C-7b", 100)
	stuckTransfer := f.requestTransfer(t, "C-7b")
	must(t, f.runTransfer(stuckTransfer.ID, false))
	stuck := f.insertReversal(t, stuckPayment.ID, "REFUNDING", "REVERSING", 100, 100)
	f.fake.FailReversalAfterCreate(errLost)
	_ = f.module.ReversePaymentTransfer(ctx, stuck, false)
	f.now = f.now.Add(PaymentResendWindow + time.Minute)
	boom := errors.New("查回逾時")
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f.fake.FailLookup(boom)
		_ = f.module.ReversePaymentTransfer(ctx, stuck, false)
	}
	if f.refundText(t, "reversal_status", stuck) != "ANOMALY" || f.refundText(t, "reversal_reason", stuck) != "lookup_failed" || len(f.anomalies) == 0 {
		t.Fatalf("狀態=%s", f.refundText(t, "reversal_status", stuck))
	}
	f.paidPayment(t, "C-7c", 100)
	f.insertRefund(t, f.idOf(t, "C-7c"), "ANOMALY", 100, 0)
	if _, err := f.module.CreatePaymentRefund(ctx, RefundRequest{OrderNo: "C-7c", Amount: 1, RequestKey: "over-cap"}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("ANOMALY 的退款沒有佔額度：%v", err)
	}
}

// 孤兒結果：provider 已撤回，列的撤回狀態卻已不在 REVERSING（寫結果之前被別的寫入改成 RELEASED）：不靜默——記 error log（含退款與撤回 ID）並發 MoneyAnomaly，轉帳不累加。
// mutation：acceptReversal 在 ApplyRefundReversal 查無列時直接 return nil → 沒有事件也沒有 log，失敗。
func TestAReversalThatIsNoLongerReversingIsReportedNotSilent(t *testing.T) {
	t.Parallel()
	var f *chargeFixture
	var refundID uuid.UUID
	f = setupChargeWith(t, func(fake *ChargeFake) ChargeProvider {
		return &afterReverse{ChargeProvider: fake, after: func() {
			ctx := context.Background()
			_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.refund_reversals SET status = 'RELEASED', reason = 'manual' WHERE refund_id = $1", refundID)
			must(t, err)
		}}
	})
	p, tr := f.transferred(t, "C-8")
	refundID = f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 40, 40)
	must(t, f.module.ReversePaymentTransfer(context.Background(), refundID, false))
	if f.reversedOf(t, tr.ID) != 0 || len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyReversal || f.anomalies[0].ObjectID != f.reversalID(t, refundID) {
		t.Fatalf("reversed %d，事件 %+v", f.reversedOf(t, tr.ID), f.anomalies)
	}
	if !strings.Contains(f.logs.String(), refundID.String()) || !strings.Contains(f.logs.String(), f.fake.ReversalIDs()[0]) {
		t.Fatalf("log 沒有退款與撤回 ID：%s", f.logs.String())
	}
}

// 後台人工結案（撤回的 ANOMALY，AnomalyReversal）：成功記撤回 ID、累加轉帳的 reversed_amount 並讓撤回進 REVERSED；失敗釋出預留（RELEASED，退款維持原狀）；都寫稽核。
func TestResolvingARefundReversalAnomalyAccruesTheReversalOnSuccess(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, tr := f.transferred(t, "C-9")
	ok := f.insertReversal(t, p.ID, "REFUNDING", "ANOMALY", 30, 30)
	bad := f.insertReversal(t, p.ID, "REFUNDING", "ANOMALY", 20, 20)
	f.exec(t, "UPDATE payment.refund_reversals SET reason = 'lookup_failed' WHERE status = 'ANOMALY'")
	rev, err := f.fake.ReverseTransfer(callDeadline(t, ctx), ChargeReversalInput{ExternalTransferID: f.transferText(t, "external_transfer_id", tr.ID), Amount: 30, IdempotencyKey: f.segText(t, "idempotency_key", ok)})
	must(t, err)
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyReversal, f.reversalID(t, ok), AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", rev.ExternalID))
	if f.refundText(t, "reversal_status", ok) != "REVERSED" || f.segText(t, "external_id", ok) != rev.ExternalID || f.reversedOf(t, tr.ID) != 30 || f.auditCount(t) != 1 {
		t.Fatalf("成功結案：%s，reversed %d", f.refundText(t, "reversal_status", ok), f.reversedOf(t, tr.ID))
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyReversal, f.reversalID(t, bad), AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", ""))
	if f.refundText(t, "reversal_status", bad) != "RELEASED" || f.refundText(t, "status", bad) != "REFUNDING" || f.reversedOf(t, tr.ID) != 30 || f.auditCount(t) != 2 {
		t.Fatalf("失敗結案：%s", f.refundText(t, "reversal_status", bad))
	}
}

// 建立付款的結果不明：過了視窗查回找到 → 收下、不建第二個 PaymentIntent、不再呼叫 CreatePayment。
// mutation：create() 在過了視窗之後先換新鍵再呼叫 → provider 出現第二個 PaymentIntent，失敗。
func TestAnUnknownCreateFoundByLookupCreatesNoSecondIntent(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("C-10", 11, 7, 100)
	f.fake.FailCreateAfterCreate(errLost)
	p := f.create(t, 11, "C-10")
	if p.Status != PaymentCreating {
		t.Fatalf("回應遺失應回 pending：%+v", p)
	}
	key := f.keyOf(t, "C-10")
	calls, _, _ := f.fake.Calls()
	f.pastResendWindow()
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if after, _, _ := f.fake.Calls(); f.statusOf(t, "C-10") != PaymentAwaitingPayment || f.fake.Intents() != 1 || after != calls || f.keyOf(t, "C-10") != key || f.externalOf(t, "C-10") != f.fake.IntentFor("C-10")[0] {
		t.Fatalf("查回沒有收下：%s，PaymentIntent %d 個，呼叫 %d → %d", f.statusOf(t, "C-10"), f.fake.Intents(), calls, after)
	}
	f.noViolations(t)
}

// 建立付款被可重試拒絕：換新鍵（舊嘗試記被取代、新嘗試另寫一列）、不計期限（過了 40 小時仍照常呼叫 CreatePayment 並成功）。
// mutation：create() 把 callRetryable 當不明（同鍵重送）→ 鍵沒變，失敗。
func TestARetryableRefusalOfTheCreateRotatesTheKeyAndTheAttemptAndIsNeverCounted(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("C-11", 11, 7, 100)
	f.fake.FailCreate(fmt.Errorf("%w：429", ErrChargeRetryableRefusal))
	p := f.create(t, 11, "C-11")
	refusedKey := f.fake.CreateKeys()[0]
	if f.keyOf(t, "C-11") == refusedKey {
		t.Fatal("可重試拒絕之後列上還是被拒絕的那把鍵")
	}
	f.now = f.now.Add(40 * time.Hour)
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if keys := f.fake.CreateKeys(); len(keys) != 2 || keys[1] == refusedKey || f.statusOf(t, "C-11") != PaymentAwaitingPayment || f.fake.LookupCalls() != 0 || f.fake.Intents() != 1 {
		t.Fatalf("狀態=%s，查回 %d 次", f.statusOf(t, "C-11"), f.fake.LookupCalls())
	}
	if f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE payment_id = $1", p.ID) != 2 || f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE payment_id = $1 AND superseded_at IS NOT NULL", p.ID) != 1 {
		t.Fatal("舊嘗試應記為被取代、新嘗試另寫一列")
	}
}

// 建立付款：查回連續失敗進 ANOMALY（佔住這張訂單：再次建立仍回 ANOMALY、不換鍵、不呼叫）；查回找到的金額不符也進 ANOMALY；後台結案：失敗進 CREATE_FAILED，
// 成功連上 provider 的 PaymentIntent 並排同步、由重讀推導真正的狀態。
// mutation：Next 讓 ANOMALY 可被快照改寫 → 結案前同步把它改成別的狀態，失敗。
func TestACreateAnomalyHoldsTheOrderAndIsResolvedByAnAdmin(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("C-12", 11, 7, 100)
	f.fake.FailCreate(errLost)
	p := f.create(t, 11, "C-12")
	f.pastResendWindow()
	boom := errors.New("查回逾時")
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f.fake.FailLookup(boom)
		_ = f.module.CreatePaymentIntent(ctx, p.ID, false)
	}
	if f.statusOf(t, "C-12") != PaymentAnomaly || len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyPayment || !f.anomalyInTx[0] {
		t.Fatalf("狀態=%s，事件 %+v", f.statusOf(t, "C-12"), f.anomalies)
	}
	key := f.keyOf(t, "C-12")
	calls, _, _ := f.fake.Calls()
	again := f.create(t, 11, "C-12")
	must(t, f.module.CreatePaymentIntent(ctx, p.ID, false))
	if after, _, _ := f.fake.Calls(); again.Status != PaymentAnomaly || f.keyOf(t, "C-12") != key || after != calls {
		t.Fatalf("ANOMALY 沒有佔住：%+v，呼叫 %d → %d", again, calls, after)
	}
	f.jobs.mu.Lock()
	f.jobs.args, f.jobs.opts = nil, nil
	f.jobs.mu.Unlock()
	f.now = f.now.Add(72 * time.Hour)
	_, _, err := f.module.SweepPayments(ctx)
	must(t, err)
	if len(f.paymentIDs(t, kindChargeCreate)) != 0 {
		t.Fatal("ANOMALY 的付款被補掃重排")
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyPayment, p.ID, AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", ""))
	if f.statusOf(t, "C-12") != PaymentCreateFailed || f.auditCount(t) != 1 {
		t.Fatalf("失敗結案：%s", f.statusOf(t, "C-12"))
	}

	// 查回找到的金額不符。
	f.order("C-13", 11, 7, 100)
	f.fake.FailCreateAfterCreate(errLost)
	q := f.create(t, 11, "C-13")
	must(t, f.fake.SetAmount(f.fake.IntentFor("C-13")[0], 1))
	f.pastResendWindow()
	must(t, f.module.CreatePaymentIntent(ctx, q.ID, false))
	if f.statusOf(t, "C-13") != PaymentAnomaly || !strings.HasPrefix(f.text(t, "SELECT failure_reason FROM payment.payments WHERE id = (SELECT payment_id FROM payment.payment_orders WHERE order_no = 'C-13')"), "reply_mismatch") || f.externalOf(t, "C-13") != "" {
		t.Fatalf("金額不符：%s", f.statusOf(t, "C-13"))
	}
	// 結案成成功：連上證據指的 PaymentIntent（provider 那邊確實有），排同步，重讀後是真實狀態。
	evidence := f.fake.IntentFor("C-13")[0]
	must(t, f.fake.SetAmount(evidence, 100))
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyPayment, q.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", evidence))
	if f.statusOf(t, "C-13") != PaymentAwaitingPayment || f.externalOf(t, "C-13") != evidence || !containsID(f.paymentIDs(t, kindChargeSync), q.ID) {
		t.Fatalf("成功結案：%s", f.statusOf(t, "C-13"))
	}
	must(t, f.fake.Settle(evidence, PaymentStateSucceeded, f.now))
	f.sync(t, q.ID)
	if f.statusOf(t, "C-13") != PaymentSucceeded || f.auditCount(t) != 2 {
		t.Fatalf("同步之後：%s，稽核 %d 筆", f.statusOf(t, "C-13"), f.auditCount(t))
	}
}

// 會呼叫 provider 的 job 方法不能在 transaction 內呼叫（與 RequestPaymentTransfer、CreatePayment 相同的守門）。
// mutation：拿掉 Transfer、Reverse、Create、Sync、CancelAttempt、Sweep 或 ResolveAnomaly 的 TxFrom 檢查 → 對應的呼叫在 transaction 內執行，失敗。
func TestProviderCallingJobMethodsRefuseToRunInsideATransaction(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	for name, call := range map[string]func(context.Context) error{
		"transfer": func(ctx context.Context) error { return f.module.CreatePaymentTransfer(ctx, uuid.NewV7(), false) },
		"reversal": func(ctx context.Context) error { return f.module.ReversePaymentTransfer(ctx, uuid.NewV7(), false) },
		"create":   func(ctx context.Context) error { return f.module.CreatePaymentIntent(ctx, uuid.NewV7(), false) },
		"sync": func(ctx context.Context) error {
			_, err := f.module.SyncPayment(ctx, uuid.NewV7())
			return err
		},
		"cancel": func(ctx context.Context) error { return f.module.CancelPaymentAttempt(ctx, uuid.NewV7()) },
		"sweep": func(ctx context.Context) error {
			_, _, err := f.module.SweepPayments(ctx)
			return err
		},
		"resolve": func(ctx context.Context) error {
			return f.module.ResolveAnomaly(ctx, f.admin(), AnomalyPayment, uuid.NewV7(), AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", "")
		},
	} {
		err := f.db.InTx(ctx, func(ctx context.Context) error { return call(ctx) })
		if err == nil || !strings.Contains(err.Error(), "transaction 內呼叫") {
			t.Fatalf("%s 在 transaction 內沒有被擋下：%v", name, err)
		}
	}
	f.noViolations(t)
}

// 查詢賣家帳戶失敗（DB 錯誤）時立刻回傳該錯誤，不當成「現在不能轉」的拒絕、也不在已中止的 transaction 內再下 SQL（後者會變成 25P02）。
// mutation：RequestTransfer 把 transferTerms 的 DB 錯誤當成 refusal → 回的是 25P02「current transaction is aborted」，失敗。
func TestRequestTransferReturnsADatabaseErrorImmediately(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "C-14", 100)
	f.exec(t, "DROP TABLE payment.accounts CASCADE")
	_, err := f.module.RequestPaymentTransfer(context.Background(), "C-14")
	if err == nil || apperr.KindOf(err) != apperr.KindInternal || strings.Contains(err.Error(), "current transaction is aborted") || !strings.Contains(err.Error(), "accounts") {
		t.Fatalf("DB 錯誤：%v", err)
	}
}

// 掃描的查詢有對應的 partial index（PENDING 轉帳、撤回 REVERSING 的退款），ANOMALY 與已結案的不在索引內。
func TestSweepQueriesHavePartialIndexes(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	for name, want := range map[string]string{"transfers_pending": "PENDING", "refund_reversals_reversing": "REVERSING"} {
		def := f.text(t, "SELECT indexdef FROM pg_indexes WHERE schemaname = 'payment' AND indexname = $1", name)
		if !strings.Contains(def, "updated_at") || !strings.Contains(def, "'"+want+"'") {
			t.Errorf("%s：%q", name, def)
		}
	}
}
