package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// receiptHarness 擺一張已 claim 到 SENDING 的**收據** operation。
//
// 與 submitHarness 的差別逐項都是刻意的：fulfillment_mode=PLATFORM_RECEIPT、
// document_kind=RECEIPT、receipt_no 非空、**沒有 provider_request_no**（收據不打 AMEGO）。
func receiptHarness(t *testing.T, authorized bool) *harness {
	t.Helper()
	consent := "RECEIPT-TERMS-v1"
	authAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	legal := "陳記蔬果行"
	addr := "台北市內湖區瑞光路1號"
	profile := domain.SellerTaxProfile{
		ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
		DefaultTaxCategory: 1,
		LegalName:          &legal,
		BusinessAddress:    &addr,
	}
	if authorized {
		profile.ReceiptAuthorizedAt = &authAt
		profile.ReceiptConsentVersion = &consent
	}
	h := newHarness(t, profile, activeAccount())

	buyerID := int64(2623900520)
	invoiceDate := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	receiptNo := "R000000000042"
	doc := domain.Document{
		ID: 900001, IssuerID: 700001, SourceRef: strPtrOrNil("O318051141"),
		DocumentKind:    codemeta.InvoiceDocumentKindReceipt.Value,
		FulfillmentMode: codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
		Status:          codemeta.InvoiceDocumentStatusIssuing.Value,
		RecipientType:   codemeta.InvoiceRecipientTypeBuyer.Value,
		RecipientRefID:  &buyerID,
		RecipientName:   "消費者", RecipientTaxID: domain.B2CBuyerIdentifier,
		SalesAmount: 1055, TotalAmount: 1055, TaxType: 1, TaxRateBps: 500,
		ReceiptNo: &receiptNo, InvoiceDateTaipei: &invoiceDate,
	}
	h.docs.stored = doc
	h.docs.docLine = []domain.DocumentLine{
		{DocumentID: doc.ID, LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
			Description: "有機高麗菜", Quantity: 2, UnitPrice: 120, NetAmount: 240,
			TaxCategory: 1, TaxRateBps: 500, AllowanceableBase: 240},
		{DocumentID: doc.ID, LineSeq: 2, LineType: codemeta.InvoiceLineTypeProduct.Value,
			Description: "溫室小番茄", Quantity: 1, UnitPrice: 815, NetAmount: 815,
			TaxCategory: 1, TaxRateBps: 500, AllowanceableBase: 815},
	}
	h.ops.stored = domain.Operation{
		ID: 900002, DocumentID: doc.ID,
		Status:      codemeta.InvoiceOperationStatusSending.Value,
		MaxAttempts: 13, AttemptCount: 0,
	}
	return h
}

// ---------------------------------------------------------------------------
// happy path
// ---------------------------------------------------------------------------

func TestSubmit_PlatformReceiptProducesPdfAndIssues(t *testing.T) {
	h := receiptHarness(t, true)

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// 🔴 收據線**絕不打 AMEGO**（§7.1.2 S-6 逐字）。這一條是這張票的核心，
	// 而它同時是「有沒有走錯分支」唯一不靠狀態值就看得出來的訊號。
	if n := len(h.provider.requests); n != 0 {
		t.Fatalf("收據線打了 AMEGO %d 次，應為 0", n)
	}
	if h.receipts.calls != 1 {
		t.Fatalf("renderer 呼叫 %d 次，want 1", h.receipts.calls)
	}
	if h.receiptS.calls != 1 {
		t.Fatalf("uploader 呼叫 %d 次，want 1", h.receiptS.calls)
	}

	// document 落 ISSUED，且走的是**收據專用**那一句（帶不了 provider 欄位）。
	if len(h.docs.issued) != 0 {
		t.Error("收據不得走 MarkIssued —— 那一句會寫 provider='AMEGO'，撞 chk_invdoc_receipt_no_provider")
	}
	if len(h.docs.receiptIssued) != 1 {
		t.Fatalf("document 未經 MarkReceiptIssued 進 ISSUED（%d 次）", len(h.docs.receiptIssued))
	}
	got := h.docs.receiptIssued[0]
	if got.PdfObjectKey != "secure/receipt/fake.pdf" {
		t.Errorf("pdf_object_key = %q，應是 uploader 回的 objectKey", got.PdfObjectKey)
	}
	// 授權版本必須在開立當下快照：來源（seller_tax_profiles.receipt_consent_version）
	// 會被「取消授權」清成 NULL，不快照就再也查不到他同意的是哪一版。
	if got.AuthorizationVersion != "RECEIPT-TERMS-v1" {
		t.Errorf("receipt_authorization_version = %q，want RECEIPT-TERMS-v1", got.AuthorizationVersion)
	}

	// 事件走 P6 而不是 P1：兩者的通知模板不同，收據的文案不得讓買家以為拿到發票。
	if len(h.outbox.inserted) != 1 {
		t.Fatalf("outbox 事件 %d 筆，want 1", len(h.outbox.inserted))
	}
	ev := h.outbox.inserted[0]
	if ev.RoutingKey != pkgmq.RoutingKeyInvoiceReceiptIssued {
		t.Errorf("routing_key = %q，want %q", ev.RoutingKey, pkgmq.RoutingKeyInvoiceReceiptIssued)
	}
	if !strings.Contains(string(ev.Payload), `"invoice_no":""`) {
		t.Errorf("收據沒有發票號碼，invoice_no 必須是空字串：%s", ev.Payload)
	}
}

// 🔴 版面資料的內容才是這張票真正交付的東西。狀態值對而資料錯，買家拿到的是一份
// 出據人錯誤或帶著別人個資的正式文件——那比沒有收據更糟。
func TestSubmit_PlatformReceiptRendersCorrectData(t *testing.T) {
	h := receiptHarness(t, true)
	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	in := h.receipts.in
	if in.Seller.LegalName != "陳記蔬果行" {
		t.Errorf("出據人 = %q，必須是賣家法定名稱（Q-B7：收據以賣家為出據人）", in.Seller.LegalName)
	}
	if in.Seller.Address != "台北市內湖區瑞光路1號" {
		t.Errorf("營業地址 = %q", in.Seller.Address)
	}
	// 免用統一發票的小規模營業人通常沒有統編 ⇒ 空字串是正確值。
	if in.Seller.TaxID != "" {
		t.Errorf("統編 = %q，本測試的賣家沒有統編", in.Seller.TaxID)
	}
	if in.ReceiptNo != "R000000000042" || in.OrderNo != "O318051141" {
		t.Errorf("receipt_no=%q order_no=%q", in.ReceiptNo, in.OrderNo)
	}
	if in.TotalAmount != 1055 {
		t.Errorf("合計 = %d，want 1055", in.TotalAmount)
	}
	if len(in.Lines) != 2 {
		t.Fatalf("品項 %d 行，want 2", len(in.Lines))
	}
	if in.Lines[0].Name != "有機高麗菜" || in.Lines[0].Quantity != 2 ||
		in.Lines[0].UnitPrice != 120 || in.Lines[0].Amount != 240 {
		t.Errorf("第 1 行 = %+v", in.Lines[0])
	}

	// 檔名用收據編號：買家下載下來看到的就是這個。
	if h.receiptS.filename != "R000000000042.pdf" {
		t.Errorf("filename = %q", h.receiptS.filename)
	}
	// 🔴 mime_type 記的是**明文**型別（#3024 定版）：機敏上傳寫的是密文，
	// 但下載通道靠這一欄決定 Content-Type，寫成 octet-stream 買家會下載到不會開的檔。
	if h.receiptS.contentType != "application/pdf" {
		t.Errorf("contentType = %q，want application/pdf", h.receiptS.contentType)
	}
}

// ---------------------------------------------------------------------------
// 陰性對照：這幾條全部**不得**落 ISSUE_FAILED（§5 L11／#3022）
// ---------------------------------------------------------------------------

// 授權在「建立 document」與「產製」之間被取消 ⇒ 轉人工，不得以賣家名義產出。
func TestSubmit_PlatformReceiptWithoutAuthorizationGoesManual(t *testing.T) {
	h := receiptHarness(t, false)

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if h.receipts.calls != 0 {
		t.Error("未授權卻仍然產製了收據 —— 那是以賣家的名義出具他沒同意的文件")
	}
	if h.receiptS.calls != 0 {
		t.Error("未授權卻仍然上傳了收據")
	}
	if h.docs.issueFailed != 0 {
		t.Error("收據不得落 ISSUE_FAILED（L11：PLATFORM_RECEIPT 不可混入發票狀態機）")
	}
	if len(h.docs.awaitingManual) != 1 ||
		h.docs.awaitingManual[0] != codemeta.InvoiceManualReasonReceiptAuthMissing.Value {
		t.Fatalf("awaitingManual = %v，want [RECEIPT_AUTH_MISSING]", h.docs.awaitingManual)
	}
	// 🔴 斷言的是「不得發 receipt.issued」而不是「outbox 必須為空」：這條路徑**應該**
	// 發 P7（invoice.operation.review_required，#3055 的橫向稽查）通知賣家有東西卡住。
	// 原本寫成「為空」是因為當時這裡一則都不發，而那個寫法會把 P7 也一起禁掉。
	for _, ev := range h.outbox.inserted {
		if ev.RoutingKey == pkgmq.RoutingKeyInvoiceReceiptIssued {
			t.Error("沒有產出收據就不得發 invoice.receipt.issued —— 買家會收到一封指向不存在檔案的通知")
		}
	}
	if len(h.outbox.inserted) != 1 ||
		h.outbox.inserted[0].RoutingKey != pkgmq.RoutingKeyInvoiceOperationReviewRequired {
		t.Fatalf("轉人工必須發 P7 通知賣家（否則卡住是無聲的），實得 %v", routingKeysOf(h.outbox.inserted))
	}
}

// 資料湊不齊（缺法定名稱）⇒ 轉人工。重試一百次也一樣，所以不留在退避曲線上。
func TestSubmit_PlatformReceiptMissingLegalNameGoesManual(t *testing.T) {
	h := receiptHarness(t, true)
	h.uc.profiles.(*fakeProfileRepo).profile.LegalName = nil

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if h.receipts.calls != 0 {
		t.Error("缺法定名稱仍然產製 —— 出據人會是空的或被別的欄位頂替")
	}
	if h.docs.issueFailed != 0 {
		t.Error("收據不得落 ISSUE_FAILED")
	}
	if len(h.docs.awaitingManual) != 1 {
		t.Fatalf("awaitingManual = %v，want 1 筆", h.docs.awaitingManual)
	}
}

// 上傳失敗 ⇒ **回 error 讓 operation 留在退避曲線上重試**，不是轉人工也不是落 ISSUED。
// 這一格守的是「暫時性故障不得被當成永久結論」。
func TestSubmit_PlatformReceiptUploadFailureIsRetryable(t *testing.T) {
	h := receiptHarness(t, true)
	h.receiptS.err = context.DeadlineExceeded

	err := h.uc.Submit(context.Background(), 900002)
	if err == nil {
		t.Fatal("上傳失敗必須回 error（讓 worker 重試），不得靜默成功")
	}
	if len(h.docs.receiptIssued) != 0 {
		t.Error("上傳失敗卻落了 ISSUED —— pdf_object_key 會是空的，買家按下載得到 404")
	}
	if len(h.docs.awaitingManual) != 0 {
		t.Error("一次逾時不該直接轉人工")
	}
	if len(h.outbox.inserted) != 0 {
		t.Error("沒有產出檔案就不得發事件")
	}
}
