//go:build integration

package payment_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func (f *chargeFixture) paidGroup(t *testing.T) Payment {
	t.Helper()
	f.order("PO-A", 11, 7, 100)
	f.order("PO-B", 11, 8, 250)
	p, err := f.module.CreatePayment(context.Background(), 11, []string{"PO-A", "PO-B"})
	must(t, err)
	must(t, f.fake.Settle(f.externalOf(t, "PO-A"), PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	if f.statusOf(t, "PO-A") != PaymentSucceeded {
		t.Fatalf("付款沒有入帳：%s", f.statusOf(t, "PO-A"))
	}
	return p
}

func (f *chargeFixture) orderID(t *testing.T, orderNo string) uuid.UUID {
	t.Helper()
	return uuid.MustParse(f.text(t, "SELECT id::text FROM payment.payment_orders WHERE order_no = $1", orderNo))
}

// 一筆付款對 N 張訂單應收：一個 PaymentIntent、金額是各訂單應收的總額、每張訂單一列 payment_orders；入帳事件帶全部訂單應收。
// mutation：付款金額只算第一張訂單 → Amount 與 provider 的金額不是 350，失敗；只寫第一張訂單應收 → payment_orders 只有一列，失敗。
func TestAPaymentOfTwoOrdersChargesTheTotalOnceAndKeepsOneReceivablePerOrder(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidGroup(t)
	if p.Amount != 350 || len(p.Orders) != 2 || f.fake.Intents() != 1 {
		t.Fatalf("付款=%+v，PaymentIntent %d 個", p, f.fake.Intents())
	}
	if f.count(t, "SELECT count(*) FROM payment.payment_orders WHERE payment_id = $1", p.ID) != 2 || f.count(t, "SELECT count(*) FROM payment.payments") != 1 {
		t.Fatal("一次結帳應該是一筆付款、兩張訂單應收")
	}
	if f.count(t, "SELECT amount FROM payment.payment_orders WHERE order_no = 'PO-B'") != 250 || f.count(t, "SELECT store_id FROM payment.payment_orders WHERE order_no = 'PO-B'") != 8 {
		t.Fatal("PO-B 的應收不是自己的店與金額")
	}
	last := f.changes[len(f.changes)-1]
	if last.Status != PaymentSucceeded || last.Amount != 350 || len(last.Orders) != 2 {
		t.Fatalf("入帳事件=%+v", last)
	}
	f.noViolations(t)
}

// 同一批訂單再建立回同一筆付款；訂單集合不同（少一張、或混進已屬別筆付款的訂單）回 FailedPrecondition，不寫第二個 PaymentIntent。
// mutation：existingGroup 不檢查訂單集合 → 少一張訂單的呼叫也成功，失敗。
func TestTheSameOrdersReuseThePaymentAndADifferentSetIsRefused(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p := f.paidGroup(t)
	again, err := f.module.CreatePayment(ctx, 11, []string{"PO-B", "PO-A"})
	must(t, err)
	if again.ID != p.ID || f.fake.Intents() != 1 {
		t.Fatalf("同一批訂單應該回同一筆付款：%v 與 %v，PaymentIntent %d 個", again.ID, p.ID, f.fake.Intents())
	}
	f.order("PO-C", 11, 7, 10)
	for name, nos := range map[string][]string{"少一張": {"PO-A"}, "混進別筆付款的訂單": {"PO-A", "PO-C"}, "多一張新訂單": {"PO-A", "PO-B", "PO-C"}} {
		if _, err := f.module.CreatePayment(ctx, 11, nos); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：err=%v，預期 FailedPrecondition", name, err)
		}
	}
	if f.fake.Intents() != 1 || f.count(t, "SELECT count(*) FROM payment.payment_orders") != 2 {
		t.Fatal("被拒絕的呼叫不能留下 PaymentIntent 或訂單應收")
	}
	if _, err := f.module.CreatePayment(ctx, 11, []string{"PO-C", "PO-C"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("重複的訂單編號：%v", err)
	}
}

// 同一筆付款底下的兩張訂單應收各有自己的退款上限：A 退滿 100 不影響 B 的 250，A 再退 1 元被擋；B 也一樣。
// mutation：退款上限改用付款總額（pay.Amount）而不是訂單應收（order.Amount）→ A 的第二筆 1 元退款通過，失敗。
func TestTwoOrderReceivablesUnderOnePaymentGetIndependentRefundCaps(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidGroup(t)
	f.createRefund(t, "PO-A", "a1", 100, false)
	_, err := f.module.CreatePaymentRefund(context.Background(), RefundRequest{OrderNo: "PO-A", Amount: 1, RequestKey: "a2"})
	wantKind(t, err, apperr.KindFailedPrecondition, "A 的應收已退滿，再退 1 元")
	f.createRefund(t, "PO-B", "b1", 250, false)
	_, err = f.module.CreatePaymentRefund(context.Background(), RefundRequest{OrderNo: "PO-B", Amount: 1, RequestKey: "b2"})
	wantKind(t, err, apperr.KindFailedPrecondition, "B 的應收已退滿，再退 1 元")
	if f.refundCount(t, "PO-A") != 1 || f.refundCount(t, "PO-B") != 1 {
		t.Fatalf("退款列：A %d、B %d", f.refundCount(t, "PO-A"), f.refundCount(t, "PO-B"))
	}
}

// 兩張訂單應收各自有一筆有效轉帳：同一筆付款底下可以有兩筆（金額各是自己的應收，同一個 transfer_group）；同一張訂單重複請求回同一筆、不寫第二筆。
// mutation：轉帳唯一索引改回依付款（payment_id）→ 第二張訂單的轉帳被擋（或沒寫），失敗。
func TestTwoOrderReceivablesUnderOnePaymentGetIndependentTransfers(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidGroup(t)
	f.enableAccount(t, 7, "acct_7", true)
	f.enableAccount(t, 8, "acct_8", true)
	a := f.requestTransfer(t, "PO-A")
	b := f.requestTransfer(t, "PO-B")
	if a.ID == b.ID || a.Amount != 100 || b.Amount != 250 || a.StoreID != 7 || b.StoreID != 8 || a.PaymentOrderID == b.PaymentOrderID {
		t.Fatalf("A=%+v B=%+v", a, b)
	}
	if again := f.requestTransfer(t, "PO-A"); again.ID != a.ID {
		t.Fatalf("同一張訂單重複請求應該回同一筆：%v 與 %v", again.ID, a.ID)
	}
	if n := f.count(t, "SELECT count(*) FROM payment.transfers WHERE status IN ('PENDING', 'SUCCEEDED', 'ANOMALY')"); n != 2 {
		t.Fatalf("有效轉帳 %d 筆，預期每張訂單一筆共 2 筆", n)
	}
	must(t, f.runTransfer(a.ID, false))
	must(t, f.runTransfer(b.ID, false))
	amounts := f.fake.TransferAmounts()
	slices.Sort(amounts)
	if f.fake.Transfers() != 2 || !slices.Equal(amounts, []int64{100, 250}) {
		t.Fatalf("provider 的轉帳 %d 筆，金額 %v", f.fake.Transfers(), amounts)
	}
	f.noViolations(t)
}

// 向賣家收回只落在退款所屬訂單應收的轉帳上：A 的退款撤回 A 的轉帳，B 的轉帳與撤回列都不動。
// mutation：撤回目標改成「付款底下任一筆成功轉帳」→ 可能撤到 B 的轉帳，failed；撤回列不記 payment_order_id → 對 B 的計數不為 0，失敗。
func TestARefundOnOrderANeverReversesOrderBsTransfer(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidGroup(t)
	f.enableAccount(t, 7, "acct_7", true)
	f.enableAccount(t, 8, "acct_8", true)
	a := f.requestTransfer(t, "PO-A")
	b := f.requestTransfer(t, "PO-B")
	must(t, f.runTransfer(a.ID, false))
	must(t, f.runTransfer(b.ID, false))
	ctx := context.Background()
	r := f.createRefund(t, "PO-A", "ka", 40, false)
	must(t, f.runRefund(r.ID, false))
	must(t, f.module.ReversePaymentTransfer(ctx, r.ID, false))
	if f.reversedOf(t, a.ID) != 40 || f.reversedOf(t, b.ID) != 0 {
		t.Fatalf("A 的轉帳撤回 %d、B 的轉帳撤回 %d，預期 40 與 0", f.reversedOf(t, a.ID), f.reversedOf(t, b.ID))
	}
	if n := f.count(t, "SELECT count(*) FROM payment.refund_reversals WHERE payment_order_id = $1", f.orderID(t, "PO-B")); n != 0 {
		t.Fatalf("B 的訂單應收底下有 %d 筆撤回列", n)
	}
	if n := f.count(t, "SELECT count(*) FROM payment.refund_reversals WHERE payment_order_id = $1 AND target_external_id = $2", f.orderID(t, "PO-A"), f.transferText(t, "external_transfer_id", a.ID)); n != 1 {
		t.Fatalf("A 的撤回列沒有指向 A 的轉帳：%d", n)
	}
	if f.fake.ReversalCalls() != 1 {
		t.Fatalf("撤回呼叫 %d 次，預期 1 次", f.fake.ReversalCalls())
	}
	f.noViolations(t)
}

// 轉帳金額的承諾（D11）以訂單應收為準：A 已有進行中的退款時 A 的轉帳金額扣掉它，B 的轉帳不受影響。
// mutation：claimTransfer 的金額改用付款總額減付款底下全部退款 → A 的轉帳金額不是 70、B 的不是 250，失敗。
func TestTheTransferAmountClaimDeductsOnlyThatOrdersRefunds(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidGroup(t)
	f.enableAccount(t, 7, "acct_7", true)
	f.enableAccount(t, 8, "acct_8", true)
	f.createRefund(t, "PO-A", "ka", 30, false)
	a := f.requestTransfer(t, "PO-A")
	b := f.requestTransfer(t, "PO-B")
	must(t, f.runTransfer(a.ID, false))
	must(t, f.runTransfer(b.ID, false))
	if f.transferText(t, "amount", a.ID) != "70" || f.transferText(t, "amount", b.ID) != "250" {
		t.Fatalf("A 的轉帳金額 %s、B 的 %s，預期 70 與 250", f.transferText(t, "amount", a.ID), f.transferText(t, "amount", b.ID))
	}
}

// N=1 的舊路徑不變：以訂單編號讀付款、轉帳、退款上限都對得到唯一的訂單應收，transfer_group 是付款 ID。
// mutation：PaymentByOrderNo 不填 Order → Order.OrderNo 為空，失敗。
func TestTheSingleOrderPathStillResolvesByOrderNo(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p := f.paidPayment(t, "L-1", 100)
	got, err := f.module.PaymentByOrderNo(context.Background(), "L-1")
	must(t, err)
	if got.ID != p.ID || got.Order.OrderNo != "L-1" || got.Order.Amount != 100 || got.Amount != 100 || len(got.Orders) != 1 || got.Order.StoreID != 7 {
		t.Fatalf("PaymentByOrderNo=%+v", got)
	}
	if ids := f.fake.IntentFor(TransferGroupOf(p.ID)); len(ids) != 1 {
		t.Fatalf("transfer_group 應該是付款 ID：%v", ids)
	}
	f.enableAccount(t, 7, "acct_seller_7", true)
	if tr := f.requestTransfer(t, "L-1"); tr.Amount != 100 || tr.PaymentOrderID != p.Orders[0].ID {
		t.Fatalf("轉帳=%+v", tr)
	}
	f.createRefund(t, "L-1", "k", 70, false)
	_, err = f.module.CreatePaymentRefund(context.Background(), RefundRequest{OrderNo: "L-1", Amount: 31, RequestKey: "k2"})
	wantKind(t, err, apperr.KindFailedPrecondition, "N=1 的退款上限是訂單金額")
}

// GrantPayment（RequestPaymentTransfer）重複呼叫回成功、同一筆轉帳：已有有效轉帳時不再檢查付款狀態與賣家帳戶，帳戶事後停用也一樣（D-G）。
// mutation：RequestTransfer 先算 transferTerms 再看既有轉帳 → 帳戶停用後的重複呼叫回 FailedPrecondition，失敗。
func TestAGrantRequestRepeatedAfterTheTransferExistsSucceedsIdempotently(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.paidPayment(t, "G-1", 100)
	f.enableAccount(t, 7, "acct_seller_7", true)
	first := f.requestTransfer(t, "G-1")
	must(t, f.runTransfer(first.ID, false))
	f.exec(t, "UPDATE payment.accounts SET status = 'DISABLED', transfers_active = false WHERE store_id = 7")
	again, err := f.module.RequestPaymentTransfer(context.Background(), "G-1")
	if err != nil || again.ID != first.ID || again.Status != TransferSucceeded {
		t.Fatalf("重複請求：%+v err=%v，預期同一筆已成功的轉帳", again, err)
	}
	if f.count(t, "SELECT count(*) FROM payment.transfers") != 1 {
		t.Fatal("重複請求寫了第二筆轉帳")
	}
}

// D-E：結果不明的建立過了 24 小時（ResendWindow）之後，舊鍵與新鍵都不再送出建立呼叫；查回帶 created 下限（第一次呼叫的記錄時刻減時鐘誤差）；
// 查回失敗不算證明不存在；查無但還沒滿 NotFoundAfter 繼續等；證明不存在（滿 NotFoundAfter 而且查回成功）才進 CREATE_FAILED；連續查回失敗進 ANOMALY。
// mutation：過了視窗仍重送 CreatePayment → 呼叫次數增加，失敗；查回不帶下限（createdLowerBound 回零值）→ 下限斷言失敗；
// 查回失敗當成查無 → 提早進 CREATE_FAILED，失敗。
func TestAfterTheResendWindowAnUnknownCreateIsNeverReplayedWithoutProvenAbsence(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("DE-1", 11, 7, 100)
	f.fake.FailCreate(errors.New("回應沒有到達"))
	p := f.create(t, 11, "DE-1")
	if p.Status != PaymentCreating {
		t.Fatalf("狀態=%s", p.Status)
	}
	firstCallAt := f.now
	key := f.keyOf(t, "DE-1")
	creates, _, _ := f.fake.Calls()
	f.pastResendWindow()
	ctx := context.Background()
	unchanged := func(what string) {
		t.Helper()
		if after, _, _ := f.fake.Calls(); after != creates || f.keyOf(t, "DE-1") != key || f.fake.Intents() != 0 {
			t.Fatalf("%s：建立呼叫 %d → %d 次，鍵 %s → %s，PaymentIntent %d 個", what, creates, after, key, f.keyOf(t, "DE-1"), f.fake.Intents())
		}
	}

	f.fake.FailLookup(errors.New("查回逾時"))
	if err := f.module.CreatePaymentIntent(ctx, p.ID, false); err == nil || f.statusOf(t, "DE-1") != PaymentCreating {
		t.Fatalf("查回失敗不能當成查無：err=%v 狀態=%s", err, f.statusOf(t, "DE-1"))
	}
	unchanged("查回失敗")

	if err := f.module.CreatePaymentIntent(ctx, p.ID, false); err == nil || f.statusOf(t, "DE-1") != PaymentCreating {
		t.Fatalf("查無但還沒滿 NotFoundAfter：err=%v 狀態=%s", err, f.statusOf(t, "DE-1"))
	}
	unchanged("查無但還沒滿等待時間")
	want := firstCallAt.Add(-PaymentListClockSkew)
	bounds := f.fake.CreatedAfterSeen()
	if len(bounds) != 2 || !bounds[0].Equal(want) || !bounds[1].Equal(want) {
		t.Fatalf("查回的 created 下限 %v，預期都是第一次呼叫 %v 減 %v", bounds, firstCallAt, PaymentListClockSkew)
	}

	f.now = firstCallAt.Add(PaymentNotFoundAfter + time.Minute)
	must(t, f.module.CreatePaymentIntent(ctx, p.ID, false))
	if f.statusOf(t, "DE-1") != PaymentCreateFailed {
		t.Fatalf("證明不存在之後應該是 CREATE_FAILED：%s", f.statusOf(t, "DE-1"))
	}
	unchanged("證明不存在")
}

// 連續查回失敗到上限進 ANOMALY（佔住訂單、補掃不再重試），期間沒有任何建立呼叫。
// mutation：查回失敗不累計次數 → 永遠不進 ANOMALY，失敗。
func TestRepeatedLookupFailuresPastTheResendWindowParkTheCreateInAnomalyWithoutReplaying(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("DE-2", 11, 7, 100)
	f.fake.FailCreate(errors.New("回應沒有到達"))
	p := f.create(t, 11, "DE-2")
	creates, _, _ := f.fake.Calls()
	f.pastResendWindow()
	ctx := context.Background()
	for range PaymentLookupFailureLimit {
		f.fake.FailLookup(errors.New("查回逾時"))
		_ = f.module.CreatePaymentIntent(ctx, p.ID, false)
	}
	if f.statusOf(t, "DE-2") != PaymentAnomaly {
		t.Fatalf("連續 %d 次查回失敗之後應該是 ANOMALY：%s", PaymentLookupFailureLimit, f.statusOf(t, "DE-2"))
	}
	if after, _, _ := f.fake.Calls(); after != creates {
		t.Fatalf("查回期間重送了建立呼叫：%d → %d", creates, after)
	}
}

// 人工結案的原因與備註寫進同一個 transaction 的後台紀錄；原因缺漏、與結果不相容、備註空白或過長都回 InvalidInput，列不動、沒有紀錄。
// mutation：Record 的 detail 不帶 resolution_reason 或 note → 對應斷言失敗；validateResolution 不檢查相容性／長度 → 對應案例失敗。
func TestResolvingAnAnomalyRecordsTheReasonAndNoteInTheAuditRowAndRejectsBadOnes(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.unreachedTransfer(t, "RA-1")
	f.pastResendWindow()
	for range PaymentLookupFailureLimit {
		f.fake.FailLookup(errors.New("查回逾時"))
		_ = f.runTransfer(tr.ID, false)
	}
	if f.transferText(t, "status", tr.ID) != "ANOMALY" {
		t.Fatalf("轉帳狀態=%s", f.transferText(t, "status", tr.ID))
	}
	ctx := context.Background()
	long := make([]rune, AnomalyResolutionNoteMaxRunes+1)
	for i := range long {
		long[i] = '字'
	}
	for name, c := range map[string]struct {
		outcome AnomalyOutcome
		reason  AnomalyResolutionReason
		note    string
	}{
		"沒有原因":       {AnomalyFailed, "", "備註"},
		"不認得的原因":     {AnomalyFailed, "WHATEVER", "備註"},
		"成功搭配沒執行的原因": {AnomalyFailed, ResolutionProviderExecutedNotRecorded, "備註"},
		"空白備註":       {AnomalyFailed, ResolutionProviderNotExecuted, " \t "},
		"過長的備註":      {AnomalyFailed, ResolutionProviderNotExecuted, string(long)},
	} {
		err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr.ID, c.outcome, c.reason, c.note, "")
		wantKind(t, err, apperr.KindInvalidInput, name)
	}
	if f.auditCount(t) != 0 || f.transferText(t, "status", tr.ID) != "ANOMALY" {
		t.Fatal("被拒絕的結案改動了列或寫了紀錄")
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr.ID, AnomalyFailed, ResolutionProviderNotExecuted, "  Stripe dashboard 查無這筆轉帳  ", ""))
	if f.auditCount(t) != 1 {
		t.Fatalf("後台紀錄 %d 筆，預期 1 筆", f.auditCount(t))
	}
	if got := f.text(t, "SELECT detail->>'resolution_reason' FROM adminaudit.records"); got != string(ResolutionProviderNotExecuted) {
		t.Fatalf("紀錄的原因代碼=%q", got)
	}
	if got := f.text(t, "SELECT detail->>'note' FROM adminaudit.records"); got != "Stripe dashboard 查無這筆轉帳" {
		t.Fatalf("紀錄的備註=%q（要去頭尾空白）", got)
	}
}
