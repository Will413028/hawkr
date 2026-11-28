package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

var resendNow = time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)

const (
	resendDocID    = int64(8001)
	resendStoreID  = int64(2685914682)
	resendBuyerID  = int64(2623900520)
	resendIssuerID = int64(900)
)

type resendFixture struct {
	uc     *NotificationResend
	docs   *fakeDocRepo
	outbox *fakeOutbox
}

func newResendFixture(t *testing.T, status string) *resendFixture {
	t.Helper()
	buyer := resendBuyerID
	no := "ZA10068421"
	date := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	docs := &fakeDocRepo{
		ownerStoreID: resendStoreID,
		stored: domain.Document{
			ID: resendDocID, IssuerID: resendIssuerID, Status: status,
			DocumentKind:  codemeta.InvoiceDocumentKindB2c.Value,
			RecipientType: codemeta.InvoiceRecipientTypeBuyer.Value, RecipientRefID: &buyer,
			RecipientTaxID:    domain.B2CBuyerIdentifier,
			ProviderInvoiceNo: &no, InvoiceDateTaipei: &date,
			TotalAmount: 1000, SourceRef: strPtrOrNil("O2026082900001"),
		},
	}
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{
		ID: resendIssuerID, IssuerType: "SELLER", IssuerRefID: resendStoreID,
	}}
	outbox := &fakeOutbox{}
	uc := NewNotificationResend(directTx{}, docs, issuers, outbox, zap.NewNop()).
		WithClock(func() time.Time { return resendNow })
	return &resendFixture{uc: uc, docs: docs, outbox: outbox}
}

func TestResend_SellerSuccess(t *testing.T) {
	fx := newResendFixture(t, codemeta.InvoiceDocumentStatusIssued.Value)

	got, err := fx.uc.ResendForSeller(context.Background(), resendStoreID, resendDocID)
	if err != nil {
		t.Fatalf("ResendForSeller 失敗：%v", err)
	}
	if !got.Resent || !got.SentAt.Equal(resendNow) {
		t.Fatalf("回應不符：%+v", got)
	}
	if len(fx.outbox.inserted) != 1 {
		t.Fatalf("應發一則事件，實得 %d", len(fx.outbox.inserted))
	}
	ev := fx.outbox.inserted[0]
	if ev.RoutingKey != pkgmq.RoutingKeyInvoiceIssued {
		t.Fatalf("routing key 應沿用 P1，實得 %q", ev.RoutingKey)
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatalf("payload 不是合法 JSON：%v", err)
	}
	if payload["notification_ref_id"] != "INVRS:8001:1" {
		t.Fatalf("notification_ref_id 應為 INVRS:8001:1，實得 %v", payload["notification_ref_id"])
	}
	if payload["invoice_no"] != "ZA10068421" {
		t.Fatalf("invoice_no 應取 provider_invoice_no，實得 %v", payload["invoice_no"])
	}
	if payload["store_id"] != "2685914682" {
		t.Fatalf("store_id 應為 issuer_ref_id，實得 %v", payload["store_id"])
	}
}

// 🔴 三個入口共用同一個計數器（key ＝ document_id，不是 document_id × actor）。
func TestResend_WindowIsSharedAcrossEntrances(t *testing.T) {
	fx := newResendFixture(t, codemeta.InvoiceDocumentStatusIssued.Value)

	if _, err := fx.uc.ResendForAdmin(context.Background(), resendDocID); err != nil {
		t.Fatalf("ADM 第一次應成功：%v", err)
	}
	_, err := fx.uc.ResendForSeller(context.Background(), resendStoreID, resendDocID)
	assertBizCode(t, err, bizcode.InvoiceNotificationResendTooFrequent)
	_, err = fx.uc.ResendForBuyer(context.Background(), resendBuyerID, resendDocID)
	assertBizCode(t, err, bizcode.InvoiceNotificationResendTooFrequent)

	if len(fx.outbox.inserted) != 1 {
		t.Fatalf("窗口內只能發一則事件，實得 %d", len(fx.outbox.inserted))
	}
}

func TestResend_WindowReopensAfterTenMinutes(t *testing.T) {
	fx := newResendFixture(t, codemeta.InvoiceDocumentStatusIssued.Value)
	if _, err := fx.uc.ResendForSeller(context.Background(), resendStoreID, resendDocID); err != nil {
		t.Fatalf("第一次失敗：%v", err)
	}

	fx.uc.WithClock(func() time.Time { return resendNow.Add(resendWindow) })
	if _, err := fx.uc.ResendForSeller(context.Background(), resendStoreID, resendDocID); err != nil {
		t.Fatalf("滿 10 分鐘應可重寄：%v", err)
	}
	if len(fx.outbox.inserted) != 2 {
		t.Fatalf("應發兩則事件，實得 %d", len(fx.outbox.inserted))
	}
	var p2 map[string]any
	_ = json.Unmarshal(fx.outbox.inserted[1].Payload, &p2)
	if p2["notification_ref_id"] != "INVRS:8001:2" {
		t.Fatalf("第二次的 ref id 應為 INVRS:8001:2，實得 %v", p2["notification_ref_id"])
	}
}

// Tier A：不是自己的文件 ⇒ 9301（與查無同碼、不可區分）。
func TestResend_OwnershipGuards(t *testing.T) {
	t.Run("別的賣家", func(t *testing.T) {
		fx := newResendFixture(t, codemeta.InvoiceDocumentStatusIssued.Value)
		_, err := fx.uc.ResendForSeller(context.Background(), 9999999, resendDocID)
		assertBizCode(t, err, bizcode.InvoiceDocumentNotFound)
		if len(fx.outbox.inserted) != 0 {
			t.Fatal("越權時不得發事件")
		}
		if fx.docs.stored.NotificationResendCount != 0 {
			t.Fatal("越權時不得消耗窗口額度")
		}
	})
	t.Run("別的買家", func(t *testing.T) {
		fx := newResendFixture(t, codemeta.InvoiceDocumentStatusIssued.Value)
		_, err := fx.uc.ResendForBuyer(context.Background(), 9999999, resendDocID)
		assertBizCode(t, err, bizcode.InvoiceDocumentNotFound)
		if len(fx.outbox.inserted) != 0 {
			t.Fatal("越權時不得發事件")
		}
	})
}

// 🔴 沒有東西可寄的文件**先擋再 claim**：否則按一次就把十分鐘的額度用掉了，
// 而什麼都沒發生。
func TestResend_RejectsDocumentsWithNothingToSend(t *testing.T) {
	for _, status := range []string{
		codemeta.InvoiceDocumentStatusPending.Value,
		codemeta.InvoiceDocumentStatusIssueFailed.Value,
		codemeta.InvoiceDocumentStatusAwaitingManual.Value,
		codemeta.InvoiceDocumentStatusNotApplicable.Value,
	} {
		t.Run(status, func(t *testing.T) {
			fx := newResendFixture(t, status)
			_, err := fx.uc.ResendForSeller(context.Background(), resendStoreID, resendDocID)
			assertBizCode(t, err, bizcode.InvoiceDocumentNotFound)
			if fx.docs.stored.NotificationResendCount != 0 {
				t.Fatal("被擋下時不得消耗窗口額度")
			}
			if len(fx.outbox.inserted) != 0 {
				t.Fatal("被擋下時不得發事件")
			}
		})
	}
}

// 賣家自行開立的文件重寄時，號碼必須取 external_invoice_no。
//
// 🔴 取錯的後果不是欄位錯，是**平台冒充開立人**：下游只要照 P1 的習慣讀 invoice_no，
// 就會把賣家自行開立的號碼當成平台開的（§6.1.14 對 P12 的同一條紅字）。
func TestResend_ExternalIssuanceUsesExternalNumber(t *testing.T) {
	fx := newResendFixture(t, codemeta.InvoiceDocumentStatusResolvedExternally.Value)
	ext := "AB12345678"
	fx.docs.stored.ProviderInvoiceNo = nil
	fx.docs.stored.ExternalInvoiceNo = &ext

	if _, err := fx.uc.ResendForAdmin(context.Background(), resendDocID); err != nil {
		t.Fatalf("ResendForAdmin 失敗：%v", err)
	}
	var payload map[string]any
	_ = json.Unmarshal(fx.outbox.inserted[0].Payload, &payload)
	if payload["invoice_no"] != ext {
		t.Fatalf("應取 external_invoice_no，實得 %v", payload["invoice_no"])
	}
}

func TestNextResendAvailableAt(t *testing.T) {
	justNow := resendNow.Add(-3 * time.Minute)
	longAgo := resendNow.Add(-30 * time.Minute)

	if got := NextResendAvailableAt(domain.Document{}, resendNow); got != nil {
		t.Fatalf("從未重寄過應回 nil，實得 %v", got)
	}
	got := NextResendAvailableAt(domain.Document{NotificationResentAt: &justNow}, resendNow)
	if got == nil || !got.Equal(justNow.Add(resendWindow)) {
		t.Fatalf("窗口內應回 resentAt+10min，實得 %v", got)
	}
	if got := NextResendAvailableAt(domain.Document{NotificationResentAt: &longAgo}, resendNow); got != nil {
		t.Fatalf("窗口已過應回 nil，實得 %v", got)
	}
}
