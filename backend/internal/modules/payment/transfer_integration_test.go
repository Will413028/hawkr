//go:build integration

package payment_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	kindTransferCreate   = "payment.transfer_create"
	kindTransferReversal = "payment.transfer_reversal"
)

var errInsufficientFunds = fmt.Errorf("%w：stripe insufficient available funds（HTTP 400）", ErrChargeRetryableRefusal)

func (f *chargeFixture) paidPayment(t *testing.T, orderNo string, amount int64) Payment {
	t.Helper()
	f.order(orderNo, 11, 7, amount)
	p := f.create(t, 11, orderNo)
	must(t, f.fake.Settle(f.externalOf(t, orderNo), PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	if f.statusOf(t, orderNo) != PaymentSucceeded {
		t.Fatalf("付款沒有入帳：%s", f.statusOf(t, orderNo))
	}
	return p
}

func (f *chargeFixture) enableAccount(t *testing.T, storeID int64, external string, active bool) {
	t.Helper()
	ctx := context.Background()
	status := "ENABLED"
	if !active {
		status = "DISABLED"
	}
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.accounts (store_id, provider, external_account_id, requirement_collection, status, transfers_active, first_activated_at, create_called_at, create_completed_at, created_at, updated_at)
VALUES ($1, 'fake', $2, 'PLATFORM', $3, $4::boolean, CASE WHEN $4::boolean THEN now() END, now(), now(), now(), now())
ON CONFLICT (store_id) DO UPDATE SET external_account_id = EXCLUDED.external_account_id, status = EXCLUDED.status, transfers_active = EXCLUDED.transfers_active,
  first_activated_at = COALESCE(payment.accounts.first_activated_at, EXCLUDED.first_activated_at), updated_at = now()`, storeID, external, status, active)
	must(t, err)
}

func (f *chargeFixture) insertRefund(t *testing.T, paymentID uuid.UUID, status string, amount, reversalAmount int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.refunds (id, payment_order_id, request_key, amount, status, reversal_amount, failure_reason, created_at, updated_at)
VALUES ($1, (SELECT id FROM payment.payment_orders WHERE payment_id = $2), $3, $4, $5::text, $6, CASE WHEN $5::text IN ('FAILED', 'ANOMALY') THEN 'x' END, $7, $7)`, id, paymentID, id.String(), amount, status, reversalAmount, f.now)
	must(t, err)
	return id
}

func callDeadline(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func (f *chargeFixture) insertReversal(t *testing.T, paymentID uuid.UUID, refundStatus, reversalStatus string, amount, reversalAmount int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.refunds (id, payment_order_id, request_key, amount, status, reversal_status, reversal_amount, reversal_recovered, reversal_reason,
	external_refund_id, refund_called_at, failure_reason, created_at, updated_at)
VALUES ($1, (SELECT id FROM payment.payment_orders WHERE payment_id = $2), $3, $4, $5::text, $6::text, $7, CASE WHEN $6::text = 'REVERSED' THEN $7::bigint ELSE 0 END, CASE WHEN $6::text IN ('RELEASED', 'ANOMALY') THEN 'x' END,
	CASE WHEN $5::text IN ('REFUNDING', 'SUCCEEDED', 'FAILED') THEN 're_' || $3 END, CASE WHEN $5::text IN ('REFUNDING', 'SUCCEEDED', 'FAILED') THEN $8::timestamptz END,
	CASE WHEN $5::text IN ('FAILED', 'ANOMALY') THEN 'x' END, $8, $8)`, id, paymentID, id.String(), amount, refundStatus, reversalStatus, reversalAmount, f.now)
	must(t, err)
	switch reversalStatus {
	case "REVERSING", "REVERSED", "RELEASED", "ANOMALY":
		_, err = f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.refund_reversals (id, refund_id, payment_order_id, seq, target_refund_id, target_external_id, amount, status, reason, called_at, external_id, created_at, updated_at)
VALUES ($1, $2, (SELECT id FROM payment.payment_orders WHERE payment_id = $3), 0, NULL, COALESCE((SELECT external_transfer_id FROM payment.transfers WHERE payment_order_id = (SELECT id FROM payment.payment_orders WHERE payment_id = $3) AND status = 'SUCCEEDED'), 'tr_none'), $4, $5::text,
	CASE WHEN $5::text IN ('RELEASED', 'ANOMALY') THEN 'x' END, CASE WHEN $5::text = 'REVERSED' THEN $6::timestamptz END, CASE WHEN $5::text = 'REVERSED' THEN 'trr_' || $2::uuid::text END, $6, $6)`,
			uuid.NewV7(), id, paymentID, reversalAmount, reversalStatus, f.now)
		must(t, err)
	}
	return id
}

func (f *chargeFixture) segText(t *testing.T, col string, refundID uuid.UUID) string {
	t.Helper()
	return f.text(t, "SELECT "+col+"::text FROM payment.refund_reversals WHERE refund_id = $1 ORDER BY seq LIMIT 1", refundID)
}

func (f *chargeFixture) reversalID(t *testing.T, refundID uuid.UUID) uuid.UUID {
	t.Helper()
	return uuid.MustParse(f.segText(t, "id", refundID))
}

func (f *chargeFixture) jobIDs(t *testing.T, kind, field string) []uuid.UUID {
	t.Helper()
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	var out []uuid.UUID
	for _, a := range f.jobs.args {
		if a.Kind() != kind {
			continue
		}
		raw, err := json.Marshal(a)
		must(t, err)
		v := map[string]uuid.UUID{}
		must(t, json.Unmarshal(raw, &v))
		out = append(out, v[field])
	}
	return out
}

func (f *chargeFixture) clearJobs() {
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	f.jobs.args, f.jobs.opts, f.jobs.out = nil, nil, nil
}

func (f *chargeFixture) requestTransfer(t *testing.T, orderNo string) PaymentTransfer {
	t.Helper()
	tr, err := f.module.RequestPaymentTransfer(context.Background(), orderNo)
	must(t, err)
	return tr
}

func (f *chargeFixture) runTransfer(id uuid.UUID, final bool) error {
	return f.module.CreatePaymentTransfer(context.Background(), id, final)
}

func (f *chargeFixture) transferText(t *testing.T, col string, id uuid.UUID) string {
	t.Helper()
	return f.text(t, "SELECT "+col+"::text FROM payment.transfers WHERE id = $1", id)
}

// 轉帳請求：transaction 內只寫 PENDING 轉帳（金額是付款扣掉非失敗退款、賣家帳戶的外部 ID 寫成快照）與一個 job，不呼叫 provider。
// mutation：把 CreateTransfer 呼叫搬進寫列的 transaction → Violations 記錄 transaction 內的呼叫，失敗。
// mutation：轉帳金額不扣退款 → 金額 100，失敗。
func TestRequestTransferWritesThePendingRowAndTheJobWithoutCallingTheProvider(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidPayment(t, "T-1", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	f.insertRefund(t, p.ID, "PENDING", 30, 0)
	f.insertRefund(t, p.ID, "FAILED", 20, 0)
	f.clearJobs()
	tr := f.requestTransfer(t, "T-1")
	if tr.Status != TransferPending || tr.Amount != 70 || tr.StoreID != 7 || len(p.Orders) != 1 || tr.PaymentOrderID != p.Orders[0].ID {
		t.Fatalf("轉帳=%+v", tr)
	}
	if got := f.transferText(t, "destination_account_id", tr.ID); got != "acct_seller_7" {
		t.Fatalf("去向快照=%s", got)
	}
	if ids := f.jobIDs(t, kindTransferCreate, "transfer_id"); len(ids) != 1 || ids[0] != tr.ID {
		t.Fatalf("job=%v", ids)
	}
	f.jobs.mu.Lock()
	for _, out := range f.jobs.out {
		if out {
			f.jobs.mu.Unlock()
			t.Fatal("轉帳 job 不是在 transaction 內排入的")
		}
	}
	f.jobs.mu.Unlock()
	if f.fake.TransferCalls() != 0 {
		t.Fatal("請求內就呼叫了 provider")
	}
	f.noViolations(t)
}

// 重複請求：已有有效（PENDING、SUCCEEDED）轉帳時回傳同一筆、不寫第二筆也不再排 job；FAILED 不佔位，之後的請求建新的一筆（新鍵）。
// mutation：移除 transfers_active_payment 唯一索引 → INSERT 的 ON CONFLICT 沒有對應的索引而失敗。
func TestADuplicateTransferRequestReturnsTheExistingActiveTransfer(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "T-2", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	f.clearJobs()
	first := f.requestTransfer(t, "T-2")
	second := f.requestTransfer(t, "T-2")
	if second.ID != first.ID || f.count(t, "SELECT count(*) FROM payment.transfers") != 1 || len(f.jobIDs(t, kindTransferCreate, "transfer_id")) != 1 {
		t.Fatalf("重複請求：%+v / %+v，列數 %d", first, second, f.count(t, "SELECT count(*) FROM payment.transfers"))
	}
	must(t, f.runTransfer(first.ID, false))
	third := f.requestTransfer(t, "T-2")
	if third.ID != first.ID || third.Status != TransferSucceeded || f.count(t, "SELECT count(*) FROM payment.transfers") != 1 {
		t.Fatalf("成功之後的請求：%+v", third)
	}
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET status = 'DISABLED', transfers_active = false WHERE store_id = 7")
	must(t, err)
	if again := f.requestTransfer(t, "T-2"); again.ID != first.ID {
		t.Fatalf("帳戶停用後的重複請求：%+v", again)
	}
}

// 請求被拒：付款不存在、未入帳、賣家帳戶沒有或未開通、已全額退款；被拒絕的請求不寫列、不排 job；transaction 內呼叫被擋下。
// mutation：拿掉 transferTerms 的「付款已入帳」、「帳戶 transfers_active」、「全額退款」任一判斷 → 對應的案例沒有被拒，失敗。
func TestRequestTransferRefusesWhatCannotBeTransferred(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("T-3a", 11, 6, 100)
	f.create(t, 11, "T-3a")
	f.enableAccount(t, 6, "acct_6", true)
	f.paidPayment(t, "T-3b", 100)
	f.order("T-3c", 11, 8, 100)
	f.create(t, 11, "T-3c")
	must(t, f.fake.Settle(f.externalOf(t, "T-3c"), PaymentStateSucceeded, f.now))
	f.sync(t, f.idOf(t, "T-3c"))
	f.enableAccount(t, 8, "acct_8", false)
	f.order("T-3d", 11, 9, 100)
	f.create(t, 11, "T-3d")
	must(t, f.fake.Settle(f.externalOf(t, "T-3d"), PaymentStateSucceeded, f.now))
	f.sync(t, f.idOf(t, "T-3d"))
	f.enableAccount(t, 9, "acct_9", true)
	f.insertReversal(t, f.idOf(t, "T-3d"), "SUCCEEDED", "NONE", 100, 0)
	for _, c := range []struct {
		no   string
		kind apperr.Kind
	}{{"missing", apperr.KindNotFound}, {"T-3a", apperr.KindFailedPrecondition}, {"T-3b", apperr.KindFailedPrecondition}, {"T-3c", apperr.KindFailedPrecondition}, {"T-3d", apperr.KindFailedPrecondition}} {
		if _, err := f.module.RequestPaymentTransfer(ctx, c.no); apperr.KindOf(err) != c.kind {
			t.Fatalf("%s：%v，預期 %v", c.no, err, c.kind)
		}
	}
	if f.count(t, "SELECT count(*) FROM payment.transfers") != 0 || len(f.jobIDs(t, kindTransferCreate, "transfer_id")) != 0 {
		t.Fatal("被拒絕的請求寫了轉帳或排了 job")
	}
	f.enableAccount(t, 7, "acct_7", true)
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		_, err := f.module.RequestPaymentTransfer(ctx, "T-3b")
		return err
	})
	if err == nil {
		t.Fatal("在 transaction 內請求轉帳沒有被拒絕")
	}
	f.noViolations(t)
}

func (f *chargeFixture) idOf(t *testing.T, orderNo string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT id FROM payment.payments WHERE id = (SELECT payment_id FROM payment.payment_orders WHERE order_no = $1)", orderNo).Scan(&id))
	return id
}

// 轉帳 job：provider 在 transaction 外、帶列上的冪等鍵與付款的 charge 建立轉帳；成功記外部 ID 與 SUCCEEDED；已 SUCCEEDED 的列再跑什麼都不做。
func TestTheTransferJobCallsTheProviderOutsideTheTransactionWithTheStoredKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "T-4", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "T-4")
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.transferText(t, "external_transfer_id", tr.ID) == "" || f.transferText(t, "called_at", tr.ID) == "" {
		t.Fatalf("轉帳沒有成功：%s", f.transferText(t, "status", tr.ID))
	}
	if keys := f.fake.TransferKeys(); len(keys) != 1 || keys[0] != f.transferText(t, "idempotency_key", tr.ID) {
		t.Fatalf("冪等鍵=%v", keys)
	}
	must(t, f.runTransfer(tr.ID, false))
	if f.fake.TransferCalls() != 1 || f.fake.Transfers() != 1 {
		t.Fatalf("已成功的轉帳又呼叫了 provider：%d 次", f.fake.TransferCalls())
	}
	f.noViolations(t)
}

// 平台餘額不足是可重試拒絕（沒有執行），不是終局也不是結果不明：每次換新冪等鍵重試、不計任何期限（超過舊的 12 小時仍是 PENDING 並繼續呼叫 provider、不進查回），之後成功；
// 用完重試的最後一次只記警示、留給掃描。
// mutation：Transfer 把 callRetryable 當 callUnknown（同鍵重送）→ 下面「每次換新鍵」失敗。
// mutation：Transfer 把可重試拒絕留著 called_at 不清（計入期限）→ 超過視窗後改走查回、不再呼叫 provider，「繼續呼叫」失敗。
func TestPlatformInsufficientFundsRotatesTheKeyKeepsThePendingRowPastTheOldDeadlineAndALaterRetrySucceeds(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "T-5", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "T-5")
	f.fake.FailTransfer(errInsufficientFunds, errInsufficientFunds, errInsufficientFunds)
	for i := 0; i < 2; i++ {
		if err := f.runTransfer(tr.ID, false); !errors.Is(err, ErrChargeRetryableRefusal) {
			t.Fatalf("第 %d 次：%v", i+1, err)
		}
		if f.transferText(t, "status", tr.ID) != "PENDING" || f.transferText(t, "failure_reason", tr.ID) != "" || f.transferText(t, "called_at", tr.ID) != "" {
			t.Fatalf("餘額不足之後的狀態=%s called_at=%q", f.transferText(t, "status", tr.ID), f.transferText(t, "called_at", tr.ID))
		}
		f.now = f.now.Add(30 * time.Hour)
	}
	if err := f.runTransfer(tr.ID, true); err != nil || f.transferText(t, "status", tr.ID) != "PENDING" {
		t.Fatalf("最後一次嘗試：%v，狀態 %s", err, f.transferText(t, "status", tr.ID))
	}
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.fake.LookupCalls() != 0 || f.fake.Transfers() != 1 {
		t.Fatalf("之後的重試沒有成功：%s，查回 %d 次", f.transferText(t, "status", tr.ID), f.fake.LookupCalls())
	}
	keys := f.fake.TransferKeys()
	if len(keys) != 4 {
		t.Fatalf("呼叫次數=%d", len(keys))
	}
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			t.Fatalf("可重試拒絕之後沒有換新鍵：%v", keys)
		}
		seen[k] = true
	}
	if keys[3] != f.transferText(t, "idempotency_key", tr.ID) {
		t.Fatalf("成功的是列上目前的鍵：%v", keys)
	}
	f.noViolations(t)
}

// provider 永久拒絕：轉帳進 FAILED（不回錯誤、不重試），不佔有效轉帳的位置，之後的請求建新的一筆、換新鍵。
func TestAPermanentRefusalOfTheTransferIsTerminalAndFreesTheActiveSlot(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "T-6", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "T-6")
	f.fake.FailTransfer(fmt.Errorf("%w：目的帳戶不能收款", ErrChargePermanent))
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "FAILED" || f.transferText(t, "failure_reason", tr.ID) != "provider_refused" {
		t.Fatalf("永久拒絕之後=%s", f.transferText(t, "status", tr.ID))
	}
	must(t, f.runTransfer(tr.ID, false))
	if f.fake.TransferCalls() != 1 {
		t.Fatalf("終局之後又呼叫：%d", f.fake.TransferCalls())
	}
	next := f.requestTransfer(t, "T-6")
	if next.ID == tr.ID || next.Status != TransferPending || f.transferText(t, "idempotency_key", next.ID) == f.transferText(t, "idempotency_key", tr.ID) {
		t.Fatalf("重建=%+v", next)
	}
}

// 結果不明（呼叫過、回應遺失）：ResendWindow 內以同一把鍵重送取回同一筆轉帳（provider 只有一筆）。
// mutation：拿掉呼叫之前的 markTransferCalled → called_at 一直是空，視窗不起算，converge 測試的查回路徑失敗。
func TestAnUnknownTransferIsReplayedWithTheSameKeyWithinTheResendWindow(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "T-7", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	lost := f.requestTransfer(t, "T-7")
	key := f.transferText(t, "idempotency_key", lost.ID)
	f.fake.FailTransferAfterCreate(errors.New("回應逾時"))
	if err := f.runTransfer(lost.ID, false); err == nil || f.transferText(t, "status", lost.ID) != "PENDING" || f.transferText(t, "called_at", lost.ID) == "" {
		t.Fatalf("回應遺失之後：%v，狀態 %s", err, f.transferText(t, "status", lost.ID))
	}
	f.now = f.now.Add(PaymentResendWindow - time.Minute)
	must(t, f.runTransfer(lost.ID, false))
	if f.transferText(t, "status", lost.ID) != "SUCCEEDED" || f.fake.Transfers() != 1 || f.transferText(t, "idempotency_key", lost.ID) != key || f.fake.LookupCalls() != 0 {
		t.Fatalf("視窗內重送沒有取回同一筆：%s，provider 有 %d 筆", f.transferText(t, "status", lost.ID), f.fake.Transfers())
	}
	f.noViolations(t)
}

// 掃描：閒置超過門檻的 PENDING 轉帳與撤回 REVERSING 的退款補排各自的 job；剛寫入的與已結案的不排。
// mutation：掃描不查 PENDING 轉帳（或 REVERSING 退款）→ 對應的 job 沒有補排，失敗。
func TestTheSweepRequeuesIdlePendingTransfersAndReversingRefunds(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidPayment(t, "T-8", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, "T-8")
	f.clearJobs()
	ctx := context.Background()
	if _, _, err := f.module.SweepPayments(ctx); err != nil || len(f.jobIDs(t, kindTransferCreate, "transfer_id")) != 0 {
		t.Fatalf("剛寫入的轉帳被補排：%v", err)
	}
	f.now = f.now.Add(time.Hour)
	_, _, err := f.module.SweepPayments(ctx)
	must(t, err)
	if ids := f.jobIDs(t, kindTransferCreate, "transfer_id"); len(ids) != 1 || ids[0] != tr.ID {
		t.Fatalf("閒置的轉帳沒有補排：%v", ids)
	}
	must(t, f.runTransfer(tr.ID, false))
	reversing := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 40, 40)
	f.insertReversal(t, p.ID, "SUCCEEDED", "REVERSED", 10, 10)
	f.clearJobs()
	f.now = f.now.Add(time.Hour)
	_, _, err = f.module.SweepPayments(ctx)
	must(t, err)
	if ids := f.jobIDs(t, kindTransferReversal, "refund_id"); len(ids) != 1 || ids[0] != reversing {
		t.Fatalf("閒置的撤回沒有補排：%v", ids)
	}
	if len(f.jobIDs(t, kindTransferCreate, "transfer_id")) != 0 {
		t.Fatal("已成功的轉帳被補排")
	}
}

func (f *chargeFixture) transferred(t *testing.T, orderNo string) (Payment, PaymentTransfer) {
	t.Helper()
	p := f.paidPayment(t, orderNo, 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.requestTransfer(t, orderNo)
	must(t, f.runTransfer(tr.ID, false))
	return p, tr
}

func (f *chargeFixture) refundText(t *testing.T, col string, id uuid.UUID) string {
	t.Helper()
	return f.text(t, "SELECT "+col+"::text FROM payment.refunds WHERE id = $1", id)
}

func (f *chargeFixture) reversedOf(t *testing.T, id uuid.UUID) int64 {
	t.Helper()
	return f.count(t, "SELECT reversed_amount FROM payment.transfers WHERE id = $1", id)
}

// 撤回：provider 在 transaction 外、帶退款列的 reversal 冪等鍵；成功後同一個 transaction 累加轉帳的 reversed_amount、退款列記撤回 ID 並讓撤回進 REVERSED；
// 已 REVERSED 再跑不重複累加；部分撤回與多筆退款各自累加。
// mutation：拿掉 reversed_amount 的累加 → 轉帳的 reversed_amount 仍是 0，失敗。
func TestAReversalUpdatesTheReversedAmountAndTheRefundStatusTogether(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p, tr := f.transferred(t, "T-9")
	ctx := context.Background()
	a := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 50, 30)
	b := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 20, 20)
	f.clearJobs()
	must(t, f.module.EnqueueRefundReversal(ctx, a))
	if ids := f.jobIDs(t, kindTransferReversal, "refund_id"); len(ids) != 1 || ids[0] != a {
		t.Fatalf("排入的撤回 job=%v", ids)
	}
	must(t, f.module.ReversePaymentTransfer(ctx, a, false))
	if f.refundText(t, "reversal_status", a) != "REVERSED" || f.segText(t, "external_id", a) == "" || f.reversedOf(t, tr.ID) != 30 {
		t.Fatalf("撤回之後：退款 %s，reversed_amount %d", f.refundText(t, "reversal_status", a), f.reversedOf(t, tr.ID))
	}
	if keys := f.fake.ReversalKeys(); len(keys) != 1 || keys[0] != f.segText(t, "idempotency_key", a) {
		t.Fatalf("撤回的冪等鍵=%v", keys)
	}
	must(t, f.module.ReversePaymentTransfer(ctx, a, false))
	must(t, f.module.ReversePaymentTransfer(ctx, b, false))
	if f.reversedOf(t, tr.ID) != 50 || f.fake.ReversalCalls() != 2 || f.refundText(t, "reversal_status", b) != "REVERSED" {
		t.Fatalf("累計=%d，呼叫 %d 次", f.reversedOf(t, tr.ID), f.fake.ReversalCalls())
	}
	f.noViolations(t)
}

// 暫時性失敗與賣家餘額：暫時性失敗留在 REVERSING、以同一把鍵重送，之後成功；撤回完成後賣家負餘額不是這裡的錯誤（provider 回成功就是成功，沒有對應的分支）。
func TestAReversalRetriesWithTheSameKeyAfterATransientFailure(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p, tr := f.transferred(t, "T-10")
	ctx := context.Background()
	a := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 40, 40)
	f.fake.FailReversal(errors.New("連線逾時"))
	if err := f.module.ReversePaymentTransfer(ctx, a, false); err == nil || f.refundText(t, "reversal_status", a) != "REVERSING" || f.reversedOf(t, tr.ID) != 0 {
		t.Fatalf("暫時性失敗之後：%v，%s", err, f.refundText(t, "reversal_status", a))
	}
	must(t, f.module.ReversePaymentTransfer(ctx, a, false))
	keys := f.fake.ReversalKeys()
	if len(keys) != 2 || keys[0] != keys[1] || f.refundText(t, "reversal_status", a) != "REVERSED" || f.reversedOf(t, tr.ID) != 40 {
		t.Fatalf("重送：%v，%s，%d", keys, f.refundText(t, "reversal_status", a), f.reversedOf(t, tr.ID))
	}
}

// 永久拒絕與結果不明：永久拒絕釋出預留（RELEASED，退款維持原狀、錢由平台吸收）並發 MoneyAnomaly，reversed_amount 不動；視窗內以同一把鍵取回同一次撤回。
func TestAReversalPermanentRefusalLeavesTheTransferUntouchedAndAnUnknownOneIsReplayedWithinTheWindow(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p, tr := f.transferred(t, "T-11")
	ctx := context.Background()
	refused := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 10, 10)
	f.fake.FailReversal(fmt.Errorf("%w：不能撤回", ErrChargePermanent))
	must(t, f.module.ReversePaymentTransfer(ctx, refused, false))
	if f.refundText(t, "reversal_status", refused) != "RELEASED" || f.refundText(t, "reversal_reason", refused) != "provider_refused" || f.refundText(t, "status", refused) != "REFUNDING" || f.reversedOf(t, tr.ID) != 0 {
		t.Fatalf("永久拒絕之後：%s", f.refundText(t, "reversal_status", refused))
	}
	if n := len(f.anomalies); n != 1 || f.anomalies[0].Kind != AnomalyReversal || f.anomalies[0].ObjectID != f.reversalID(t, refused) {
		t.Fatalf("撤回永久失敗沒有發 MoneyAnomaly：%+v", f.anomalies)
	}
	replayed := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 10, 10)
	key := f.segText(t, "idempotency_key", replayed)
	f.fake.FailReversalAfterCreate(errors.New("回應逾時"))
	if err := f.module.ReversePaymentTransfer(ctx, replayed, false); err == nil || f.segText(t, "called_at", replayed) == "" {
		t.Fatalf("回應遺失：%v", err)
	}
	must(t, f.module.ReversePaymentTransfer(ctx, replayed, false))
	if f.refundText(t, "reversal_status", replayed) != "REVERSED" || f.reversedOf(t, tr.ID) != 10 || f.segText(t, "idempotency_key", replayed) != key {
		t.Fatalf("視窗內重送沒有取回同一次撤回：%s，%d", f.refundText(t, "reversal_status", replayed), f.reversedOf(t, tr.ID))
	}
}

// 對不到已成立的轉帳、或撤回額度超過剩餘可撤額度的退款是前置條件異常：不呼叫 provider、退款進 ANOMALY（原因記在 failure_reason、發 MoneyAnomaly）、
// 補掃不再重排（以前只取消 job，掃描會永遠重排）。
// mutation：reverse() 的前置條件異常改回只回 ErrAnomaly、不寫 ANOMALY → 狀態仍是 REVERSING，失敗。
func TestAReversalThatCannotMatchTheTransferParksTheRefundInAnomalyAndTheSweepStops(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	bare := f.paidPayment(t, "T-12a", 100)
	noTransfer := f.insertReversal(t, bare.ID, "REFUNDING", "REVERSING", 10, 10)
	p, _ := f.transferred(t, "T-12b")
	tooMuch := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 100, 100)
	other := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 10, 10)
	must(t, f.module.ReversePaymentTransfer(ctx, other, false))
	for id, reason := range map[uuid.UUID]string{noTransfer: "no_succeeded_transfer", tooMuch: "reversal_exceeds_remaining"} {
		must(t, f.module.ReversePaymentTransfer(ctx, id, false))
		if f.refundText(t, "reversal_status", id) != "ANOMALY" || f.refundText(t, "reversal_reason", id) != reason {
			t.Fatalf("%s：%s %s", id, f.refundText(t, "reversal_status", id), f.refundText(t, "reversal_reason", id))
		}
	}
	if f.fake.ReversalCalls() != 1 || len(f.anomalies) != 2 {
		t.Fatalf("異常的退款呼叫了 provider（%d 次）或事件數 %d", f.fake.ReversalCalls(), len(f.anomalies))
	}
	f.clearJobs()
	f.now = f.now.Add(48 * time.Hour)
	_, _, err := f.module.SweepPayments(ctx)
	must(t, err)
	if len(f.jobIDs(t, kindTransferReversal, "refund_id")) != 0 {
		t.Fatal("ANOMALY 的退款被補掃重排")
	}
}

// 寫結果的 transaction 失敗時整筆回滾：撤回不會單獨進 REVERSED、轉帳也不會單獨累加（同一個 transaction）。
// 造法：provider 回成功之後、寫結果之前，另一個寫入把轉帳的 reversed_amount 推到上限，累加違反 CHECK。
func TestAFailedReversalWriteRollsBackTheRefundStatusToo(t *testing.T) {
	t.Parallel()
	var f *chargeFixture
	var transferID uuid.UUID
	f = setupChargeWith(t, func(fake *ChargeFake) ChargeProvider {
		return &afterReverse{ChargeProvider: fake, after: func() {
			ctx := context.Background()
			_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.transfers SET reversed_amount = amount WHERE id = $1", transferID)
			must(t, err)
		}}
	})
	p, tr := f.transferred(t, "T-13")
	transferID = tr.ID
	a := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 40, 40)
	if err := f.module.ReversePaymentTransfer(context.Background(), a, false); err == nil {
		t.Fatal("寫結果違反 CHECK 卻沒有回錯誤")
	}
	if f.refundText(t, "reversal_status", a) != "REVERSING" || f.segText(t, "external_id", a) != "" || f.reversedOf(t, tr.ID) != 100 {
		t.Fatalf("回滾之後：退款 %s，reversed %d", f.refundText(t, "reversal_status", a), f.reversedOf(t, tr.ID))
	}
}

type afterReverse struct {
	ChargeProvider
	after func()
}

func (a *afterReverse) ReverseTransfer(ctx context.Context, in ChargeReversalInput) (ChargeReversalSnapshot, error) {
	snap, err := a.ChargeProvider.ReverseTransfer(ctx, in)
	a.after()
	return snap, err
}
