//go:build integration

package payment_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var errRetryable429 = fmt.Errorf("%w：429", ErrChargeRetryableRefusal)

// 轉帳的第一次呼叫結果不明（provider 那邊已建立），重送被可重試拒絕：這把鍵之前的呼叫可能已執行，不能換鍵；保持同一把鍵，過了視窗查回找到那一筆，provider 只有一筆。
// mutation：transfer() 在 callRetryable 拿掉 firstCall 判斷（重送被拒絕也換鍵）→ 鍵變了，失敗。
func TestARetryableRefusalOnAResendOfAnUnknownTransferKeepsTheKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.lostTransfer(t, "R1-T")
	key := f.transferText(t, "idempotency_key", tr.ID)
	f.fake.FailTransfer(errRetryable429)
	if err := f.runTransfer(tr.ID, false); !errors.Is(err, ErrChargeRetryableRefusal) {
		t.Fatalf("重送被拒絕：%v", err)
	}
	if f.transferText(t, "idempotency_key", tr.ID) != key || f.transferText(t, "called_at", tr.ID) == "" || f.transferText(t, "status", tr.ID) != "PENDING" {
		t.Fatal("重送被可重試拒絕之後換了鍵或清掉了呼叫時刻")
	}
	f.pastResendWindow()
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.fake.Transfers() != 1 || f.transferText(t, "idempotency_key", tr.ID) != key {
		t.Fatalf("查回之後：%s，provider %d 筆", f.transferText(t, "status", tr.ID), f.fake.Transfers())
	}
}

// 撤回同理：第一次結果不明、重送被可重試拒絕，不換撤回鍵，查回找到原本那次撤回，provider 只有一筆撤回。
// mutation：reverse() 在 callRetryable 拿掉 firstCall 判斷 → 撤回鍵變了，失敗。
func TestARetryableRefusalOnAResendOfAnUnknownReversalKeepsTheKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, tr := f.transferred(t, "R1-R")
	refund := f.insertReversal(t, p.ID, "REFUNDING", "REVERSING", 10, 10)
	key := f.segText(t, "idempotency_key", refund)
	f.fake.FailReversalAfterCreate(errLost)
	if err := f.module.ReversePaymentTransfer(ctx, refund, false); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	f.fake.FailReversal(errRetryable429)
	if err := f.module.ReversePaymentTransfer(ctx, refund, false); !errors.Is(err, ErrChargeRetryableRefusal) {
		t.Fatalf("重送被拒絕：%v", err)
	}
	if f.segText(t, "idempotency_key", refund) != key || f.segText(t, "called_at", refund) == "" {
		t.Fatal("重送被可重試拒絕之後換了撤回鍵或清掉了呼叫時刻")
	}
	f.pastResendWindow()
	must(t, f.module.ReversePaymentTransfer(ctx, refund, false))
	if f.refundText(t, "reversal_status", refund) != "REVERSED" || len(f.fake.ReversalIDs()) != 1 || f.reversedOf(t, tr.ID) != 10 {
		t.Fatalf("查回之後：%s，provider %d 筆撤回", f.refundText(t, "status", refund), len(f.fake.ReversalIDs()))
	}
}

// 建立付款同理：第一次結果不明（PaymentIntent 已建立），重送被可重試拒絕，不換鍵、不寫新嘗試，查回找到原本那個，provider 只有一個 PaymentIntent。
// mutation：create() 在 callRetryable 拿掉 firstCall 判斷 → 鍵變了、多一列嘗試，失敗。
func TestARetryableRefusalOnAResendOfAnUnknownCreateKeepsTheKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("R1-C", 11, 7, 100)
	f.fake.FailCreateAfterCreate(errLost)
	p := f.create(t, 11, "R1-C")
	key := f.keyOf(t, "R1-C")
	f.fake.FailCreate(errRetryable429)
	if err := f.module.CreatePaymentIntent(ctx, p.ID, false); !errors.Is(err, ErrChargeRetryableRefusal) {
		t.Fatalf("重送被拒絕：%v", err)
	}
	if f.keyOf(t, "R1-C") != key || f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE payment_id = $1", p.ID) != 1 {
		t.Fatal("重送被可重試拒絕之後換了鍵或寫了新嘗試")
	}
	f.pastResendWindow()
	must(t, f.module.CreatePaymentIntent(ctx, p.ID, false))
	if f.statusOf(t, "R1-C") != PaymentAwaitingPayment || f.fake.Intents() != 1 || f.keyOf(t, "R1-C") != key {
		t.Fatalf("查回之後：%s，PaymentIntent %d 個", f.statusOf(t, "R1-C"), f.fake.Intents())
	}
}

type afterTransfer struct {
	ChargeProvider
	after func()
}

func (a *afterTransfer) CreateTransfer(ctx context.Context, in ChargeTransferInput) (ChargeTransferSnapshot, error) {
	snap, err := a.ChargeProvider.CreateTransfer(ctx, in)
	a.after()
	return snap, err
}

// provider 已建立轉帳，轉帳列卻已不在 PENDING（寫結果之前被別的寫入改成 FAILED）：不靜默——記 error log（含轉帳與 provider 轉帳 ID）並發 MoneyAnomaly，列不被改寫。
// mutation：acceptTransfer 在 SucceedTransfer 查無列時直接 return nil → 沒有事件也沒有 log，失敗。
func TestATransferForARowThatIsNoLongerPendingIsReportedNotSilent(t *testing.T) {
	t.Parallel()
	var f *chargeFixture
	var transferID uuid.UUID
	f = setupChargeWith(t, func(fake *ChargeFake) ChargeProvider {
		return &afterTransfer{ChargeProvider: fake, after: func() {
			f.exec(t, "UPDATE payment.transfers SET status = 'FAILED', failure_reason = 'manual' WHERE id = $1", transferID)
		}}
	})
	f.enableAccount(t, 7, "acct_seller_7", true)
	f.paidPayment(t, "R2", 100)
	tr := f.requestTransfer(t, "R2")
	transferID = tr.ID
	must(t, f.runTransfer(tr.ID, false))
	if f.transferText(t, "status", tr.ID) != "FAILED" || f.transferText(t, "external_transfer_id", tr.ID) != "" {
		t.Fatalf("列被改寫：%s", f.transferText(t, "status", tr.ID))
	}
	if len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyTransfer || f.anomalies[0].ObjectID != tr.ID || !f.anomalyInTx[0] {
		t.Fatalf("事件=%+v", f.anomalies)
	}
	if !strings.Contains(f.logs.String(), tr.ID.String()) || !strings.Contains(f.logs.String(), f.fake.TransferIDs()[0]) {
		t.Fatalf("log 沒有轉帳與 provider 轉帳 ID：%s", f.logs.String())
	}
}

// anomalousTransfer 讓一筆結果不明的轉帳因查回連續失敗進 ANOMALY。
func (f *chargeFixture) anomalousTransfer(t *testing.T, orderNo string, lost bool) PaymentTransfer {
	t.Helper()
	var tr PaymentTransfer
	if lost {
		tr = f.lostTransfer(t, orderNo)
	} else {
		tr = f.unreachedTransfer(t, orderNo)
	}
	f.pastResendWindow()
	boom := errors.New("查回逾時")
	for i := 0; i < PaymentLookupFailureLimit; i++ {
		f.fake.FailLookup(boom)
		_ = f.runTransfer(tr.ID, false)
	}
	if f.transferText(t, "status", tr.ID) != "ANOMALY" {
		t.Fatalf("沒有進 ANOMALY：%s", f.transferText(t, "status", tr.ID))
	}
	return tr
}

// 人工結案成功之前先向 provider 核對證據（轉帳）：管理員貼的 ID 在 provider 查不到、金額不符、或核對本身失敗都被擋下，列與稽核不動；核對相符才結案。
// mutation：ResolveAnomaly 拿掉 verifyEvidence 的呼叫 → 亂貼的 ID 被記成 SUCCEEDED，失敗。
func TestResolvingATransferAnomalyAsSucceededRequiresTheProviderToConfirmTheEvidence(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.enableAccount(t, 7, "acct_seller_7", true)
	tr := f.anomalousTransfer(t, "R3-T", true)
	real := f.fake.TransferIDs()[0]
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", "tr_typo"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("亂貼的 ID：%v", err)
	}
	must(t, f.fake.SetTransferAmount(real, 1))
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", real); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("金額不符：%v", err)
	}
	must(t, f.fake.SetTransferAmount(real, 100))
	f.fake.FailLookup(errors.New("查回逾時"))
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", real); err == nil || apperr.KindOf(err) == apperr.KindFailedPrecondition {
		t.Fatalf("核對失敗應回一般錯誤：%v", err)
	}
	if f.transferText(t, "status", tr.ID) != "ANOMALY" || f.auditCount(t) != 0 {
		t.Fatalf("被擋下的結案改動了列：%s，稽核 %d 筆", f.transferText(t, "status", tr.ID), f.auditCount(t))
	}
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyTransfer, tr.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", real))
	if f.transferText(t, "status", tr.ID) != "SUCCEEDED" || f.transferText(t, "external_transfer_id", tr.ID) != real || f.auditCount(t) != 1 {
		t.Fatalf("相符之後：%s", f.transferText(t, "status", tr.ID))
	}
}

// 人工結案成功之前先向 provider 核對證據（付款）：證據是另一張訂單的 PaymentIntent、或金額不符都被擋下；核對相符才連上。
// mutation：ResolveAnomaly 拿掉 verifyEvidence 的呼叫 → 別張訂單的 PaymentIntent 被連到這筆付款，失敗。
func TestResolvingAPaymentAnomalyAsSucceededRequiresTheProviderToConfirmTheEvidence(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("R3-P", 11, 7, 100)
	f.fake.FailCreateAfterCreate(errLost)
	p := f.create(t, 11, "R3-P")
	own := f.fake.IntentFor("R3-P")[0]
	must(t, f.fake.SetAmount(own, 1))
	f.pastResendWindow()
	must(t, f.module.CreatePaymentIntent(ctx, p.ID, false))
	if f.statusOf(t, "R3-P") != PaymentAnomaly {
		t.Fatalf("狀態=%s", f.statusOf(t, "R3-P"))
	}
	f.order("R3-other", 12, 7, 100)
	f.create(t, 12, "R3-other")
	other := f.fake.IntentFor("R3-other")[0]
	must(t, f.fake.SetAmount(own, 100))
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyPayment, p.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", other); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("別張訂單的 PaymentIntent：%v", err)
	}
	must(t, f.fake.SetAmount(own, 1))
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyPayment, p.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", own); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("金額不符：%v", err)
	}
	if f.statusOf(t, "R3-P") != PaymentAnomaly || f.externalOf(t, "R3-P") != "" || f.auditCount(t) != 0 {
		t.Fatalf("被擋下的結案改動了列：%s", f.statusOf(t, "R3-P"))
	}
	must(t, f.fake.SetAmount(own, 100))
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyPayment, p.ID, AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", own))
	if f.externalOf(t, "R3-P") != own || f.auditCount(t) != 1 {
		t.Fatalf("相符之後：%s", f.externalOf(t, "R3-P"))
	}
}

// 人工結案成功之前先向 provider 核對證據（撤回的 ANOMALY）：撤回 ID 在 provider 查不到、金額不符被擋下，轉帳不累加；核對相符才結案。
// mutation：ResolveAnomaly 拿掉 verifyEvidence 的呼叫 → 沒有對應撤回的 ID 被記成 REVERSED 並累加，失敗。
func TestResolvingAReversalAnomalyAsSucceededRequiresTheProviderToConfirmTheEvidence(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, tr := f.transferred(t, "R3-R")
	refund := f.insertReversal(t, p.ID, "REFUNDING", "ANOMALY", 30, 30)
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyReversal, f.reversalID(t, refund), AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", "trr_typo"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("亂貼的 ID：%v", err)
	}
	rev, err := f.fake.ReverseTransfer(callDeadline(t, ctx), ChargeReversalInput{ExternalTransferID: f.transferText(t, "external_transfer_id", tr.ID), Amount: 30, IdempotencyKey: f.segText(t, "idempotency_key", refund)})
	must(t, err)
	must(t, f.fake.SetReversalAmount(rev.ExternalID, 5))
	if err := f.module.ResolveAnomaly(ctx, f.admin(), AnomalyReversal, f.reversalID(t, refund), AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", rev.ExternalID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("金額不符：%v", err)
	}
	if f.refundText(t, "reversal_status", refund) != "ANOMALY" || f.reversedOf(t, tr.ID) != 0 || f.auditCount(t) != 0 {
		t.Fatalf("被擋下的結案改動了列：%s reversed %d", f.refundText(t, "reversal_status", refund), f.reversedOf(t, tr.ID))
	}
	must(t, f.fake.SetReversalAmount(rev.ExternalID, 30))
	must(t, f.module.ResolveAnomaly(ctx, f.admin(), AnomalyReversal, f.reversalID(t, refund), AnomalySucceeded, ResolutionProviderExecutedNotRecorded, "核對 provider 之後確認已執行", rev.ExternalID))
	if f.refundText(t, "reversal_status", refund) != "REVERSED" || f.reversedOf(t, tr.ID) != 30 {
		t.Fatalf("相符之後：%s", f.refundText(t, "reversal_status", refund))
	}
}

// ANOMALY 的付款收到帶外部 ID 的快照：列維持 ANOMALY，但同 transaction 發 MoneyAnomaly（provider 那邊其實有東西，不能只記警示 log）。
// mutation：writeRow 的 keep 分支拿掉 ANOMALY 的 publishAnomaly → 沒有第二個事件，失敗。
func TestAnAnomalousPaymentThatReceivesASnapshotWithAnExternalIDPublishesMoneyAnomaly(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("R4", 11, 7, 100)
	f.fake.FailCreateAfterCreate(errLost)
	p := f.create(t, 11, "R4")
	ext := f.fake.IntentFor("R4")[0]
	must(t, f.fake.SetAmount(ext, 1))
	f.pastResendWindow()
	must(t, f.module.CreatePaymentIntent(ctx, p.ID, false))
	if f.statusOf(t, "R4") != PaymentAnomaly || len(f.anomalies) != 1 {
		t.Fatalf("狀態=%s，事件 %d 個", f.statusOf(t, "R4"), len(f.anomalies))
	}
	must(t, f.fake.SetAmount(ext, 100))
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	f.exec(t, "UPDATE payment.payment_attempts SET external_payment_id = $1 WHERE payment_id = $2 AND superseded_at IS NULL", ext, p.ID)
	f.sync(t, p.ID)
	if f.statusOf(t, "R4") != PaymentAnomaly || len(f.anomalies) != 2 || f.anomalies[1].Kind != AnomalyPayment || f.anomalies[1].ObjectID != p.ID || !f.anomalyInTx[1] || !strings.Contains(f.anomalies[1].Reason, ext) {
		t.Fatalf("狀態=%s，事件 %+v", f.statusOf(t, "R4"), f.anomalies)
	}
}
