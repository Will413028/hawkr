package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// submitHarness 把一張已經 claim 到 SENDING 的 B2C 發票擺好。
func submitHarness(t *testing.T, mutateDoc func(*domain.Document)) *harness {
	t.Helper()
	h := newHarness(t, approvedProfile(1), activeAccount())

	reqNo := "O000123456-1-1"
	buyerID := int64(2623900520)
	invoiceDate := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	period := "11504"
	doc := domain.Document{
		ID: 900001, IssuerID: 700001, SourceRef: strPtrOrNil("O000123456"),
		DocumentKind:    codemeta.InvoiceDocumentKindB2c.Value,
		FulfillmentMode: codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		Status:          codemeta.InvoiceDocumentStatusIssuing.Value,
		RecipientType:   codemeta.InvoiceRecipientTypeBuyer.Value,
		RecipientRefID:  &buyerID,
		RecipientName:   "消費者", RecipientTaxID: domain.B2CBuyerIdentifier,
		SalesAmount: 100, TotalAmount: 100, TaxType: 1, TaxRateBps: 500,
		ProviderRequestNo: &reqNo, InvoiceDateTaipei: &invoiceDate, TaxPeriodCode: &period,
	}
	if mutateDoc != nil {
		mutateDoc(&doc)
	}
	h.docs.stored = doc
	h.docs.docLine = []domain.DocumentLine{{
		DocumentID: doc.ID, LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
		Description: "有機高麗菜", Quantity: 1, UnitPrice: 100, NetAmount: 100,
		TaxCategory: 1, TaxRateBps: 500, AllowanceableBase: 100,
	}}
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

func TestSubmit_SuccessIssuesAndEnqueuesOutbox(t *testing.T) {
	h := submitHarness(t, nil)

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if n := len(h.provider.requests); n != 1 {
		t.Fatalf("provider 收到 %d 次請求，want 1", n)
	}
	req := h.provider.requests[0]
	if req.OrderID != "O000123456-1-1" {
		t.Errorf("OrderId = %q，逾時重試必須沿用同一個鍵", req.OrderID)
	}
	if req.BuyerIdentifier != domain.B2CBuyerIdentifier || req.TaxAmount != "0" {
		t.Errorf("B2C 的 BuyerIdentifier/TaxAmount = %q/%q", req.BuyerIdentifier, req.TaxAmount)
	}
	if req.DetailVat != "1" || req.TaxRate != domain.TaxRateLiteral {
		t.Errorf("DetailVat=%q TaxRate=%q（A14；TaxRate 恆帶字面值不由 bps 除回來）", req.DetailVat, req.TaxRate)
	}
	if len(req.ProductItem) != 1 || req.ProductItem[0].Amount != "100" {
		t.Errorf("ProductItem = %+v", req.ProductItem)
	}

	if len(h.docs.issued) != 1 {
		t.Fatalf("document 未進 ISSUED")
	}
	got := h.docs.issued[0]
	if got.ProviderInvoiceNo != "AB12345678" || got.ProviderRandomCode != "4321" {
		t.Errorf("號碼未回填：%+v", got)
	}
	// 註 5：invoice_date 是我方認定的應開立日，document 建立當下固定、不隨 provider 回傳值移動。
	if got.InvoiceDateTaipei == nil || !got.InvoiceDateTaipei.Equal(time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("invoice_date_taipei 被 provider 回傳值覆蓋了：%v", got.InvoiceDateTaipei)
	}
	// 無載具 ⇒ 進 INV.pdf 候選集合；上傳輪詢一律進。
	if got.PdfFetchNextAt == nil {
		t.Error("無載具發票應進 PDF 抓取候選集合")
	}
	if got.UploadRecheckNextAt == nil {
		t.Error("upload_recheck_next_at 是上傳輪詢候選集合的入口，不得留 NULL")
	}
	if h.ops.done != 1 {
		t.Errorf("operation 未進 SUCCEEDED")
	}
	if len(h.outbox.inserted) != 1 {
		t.Fatalf("outbox 應恰有一列，實得 %d", len(h.outbox.inserted))
	}
	ev := h.outbox.inserted[0]
	if ev.RoutingKey != pkgmq.RoutingKeyInvoiceIssued || ev.Exchange != pkgmq.ExchangeInvoice {
		t.Errorf("routing key/exchange = %q/%q", ev.RoutingKey, ev.Exchange)
	}
	// §6.3.3.2 邊界②：random_code 是憑證不是資料（invoice_no + random_code 就查得到整張發票）。
	if containsBytes(ev.Payload, "4321") {
		t.Error("payload 帶了 random_code —— 它是憑證，不得進 MQ")
	}
}

func TestSubmit_CarrierInvoiceSkipsPdfCandidateSet(t *testing.T) {
	h := submitHarness(t, func(d *domain.Document) {
		ct := domain.AmegoCarrierMobileBarcode
		d.CarrierType = &ct
	})
	h.docs.secrets = domain.RecipientSecrets{CarrierID1: "/ABC+123"}

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if h.docs.issued[0].PdfFetchNextAt != nil {
		t.Error("載具發票沒有 PDF 可下載，不得進 INV.pdf 候選集合")
	}
	if h.provider.requests[0].CarrierID1 != "/ABC+123" {
		t.Error("載具明文未送出（解密只在 callback 內，但要真的用到）")
	}
}

// ---------------------------------------------------------------------------
// 四分類落點（§4.5.6 → §7.3.2）
// ---------------------------------------------------------------------------

func TestSubmit_ErrorClassRouting(t *testing.T) {
	cases := []struct {
		name string
		err  *amego.APIError

		wantSend     bool // 有沒有落 ISSUE_FAILED
		wantUnknown  bool // 有沒有落 WAITING_CONFIRMATION
		wantRetry    bool
		wantSeller   bool
		wantPermFail bool
		wantOutbox   int
	}{
		{
			name:       "RETRYABLE → J12 退避重排，document 退回 PENDING，不發事件",
			err:        &amego.APIError{Code: "10", Class: amego.ClassRetryable, Msg: "系統停機維護中"},
			wantRetry:  true,
			wantOutbox: 0,
		},
		{
			name:         "PERMANENT → J11，document ISSUE_FAILED ＋ P2",
			err:          &amego.APIError{Code: "3040123", Class: amego.ClassPermanent, Msg: "統編錯誤"},
			wantPermFail: true, wantSend: true, wantOutbox: 1,
		},
		{
			name: "PERMANENT ＋ 賣家可自解 → J32 SELLER_ACTION_REQUIRED（不叫 FAILED_* 但一樣要落 ISSUE_FAILED）",
			err: &amego.APIError{Code: "3040111", Class: amego.ClassPermanent,
				SellerActionRequired: true, Msg: "字軌不足"},
			wantSeller: true, wantSend: true, wantOutbox: 1,
		},
		{
			name:        "RECONCILE（含逾時與 3040171 永久衝突）→ J13，document 留在 ISSUING、不發事件",
			err:         &amego.APIError{Code: "3040171", Class: amego.ClassReconcile, SuccessEquivalent: true},
			wantUnknown: true, wantOutbox: 0,
		},
		{
			name: "MANUAL（環境閘門）→ J11：拒絕外呼是永久狀態，不是暫時性",
			err: &amego.APIError{Code: amego.CodeEnvGuard, Class: amego.ClassManual,
				Msg: "AMEGO_ALLOW_REAL_INVOICE=false"},
			wantPermFail: true, wantSend: true, wantOutbox: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := submitHarness(t, nil)
			h.provider.err = tc.err

			if err := h.uc.Submit(context.Background(), 900002); err != nil {
				t.Fatalf("Submit: %v", err)
			}
			if got := h.ops.failed > 0; got != tc.wantPermFail {
				t.Errorf("FAILED_PERMANENT = %v, want %v", got, tc.wantPermFail)
			}
			if got := h.ops.seller > 0; got != tc.wantSeller {
				t.Errorf("SELLER_ACTION_REQUIRED = %v, want %v", got, tc.wantSeller)
			}
			if got := len(h.ops.unknown) > 0; got != tc.wantUnknown {
				t.Errorf("WAITING_CONFIRMATION = %v, want %v", got, tc.wantUnknown)
			}
			if got := len(h.ops.attempts) > 0; got != tc.wantRetry {
				t.Errorf("RETRY_SCHEDULED = %v, want %v", got, tc.wantRetry)
			}
			if got := h.docs.issueFailed > 0; got != tc.wantSend {
				t.Errorf("document ISSUE_FAILED = %v, want %v", got, tc.wantSend)
			}
			if len(h.outbox.inserted) != tc.wantOutbox {
				t.Errorf("outbox 列數 = %d, want %d", len(h.outbox.inserted), tc.wantOutbox)
			}
			if len(h.docs.issued) != 0 {
				t.Error("失敗路徑不得寫 ISSUED")
			}
		})
	}
}

// 非 amego.APIError（ctx 取消、transport 未分類）一律走對帳：副作用可能已經發生，重送會開第二張。
func TestSubmit_NonProviderErrorGoesToReconcile(t *testing.T) {
	h := submitHarness(t, nil)
	h.provider.err = errors.New("context deadline exceeded")

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(h.ops.unknown) != 1 {
		t.Fatal("非 provider 錯誤必須落 WAITING_CONFIRMATION，不得直接重送")
	}
	if len(h.ops.attempts) != 0 || h.ops.failed != 0 {
		t.Error("結果不明時不得寫成 RETRY_SCHEDULED 或 FAILED_PERMANENT")
	}
}

// J14：attempt_count 達 MaxAttempts ⇒ 先走完 J12 再走 J14（後者的 allowed-from 是 RETRY_SCHEDULED）。
func TestSubmit_RetryExhaustedTakesBothEdges(t *testing.T) {
	h := submitHarness(t, nil)
	h.ops.stored.AttemptCount = 12 // 下一次 ＝ 13 ＝ MaxAttempts
	h.provider.err = &amego.APIError{Code: "18", Class: amego.ClassRetryable, Msg: "DB 忙碌"}

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(h.ops.attempts) != 1 || h.ops.attempts[0] != 13 {
		t.Fatalf("attempt_count 未寫成 13：%v", h.ops.attempts)
	}
	if h.ops.exhaust != 1 {
		t.Fatal("未走 J14 FAILED_RETRY_EXHAUSTED")
	}
	if h.docs.pendingRetry != 0 {
		t.Error("預算用完時 document 不得退回 PENDING（它不會再被撈起來）")
	}
	if h.docs.issueFailed != 1 || len(h.outbox.inserted) != 1 {
		t.Error("document 應落 ISSUE_FAILED 並發 P2")
	}
	if ev := h.outbox.inserted[0]; ev.RoutingKey != pkgmq.RoutingKeyInvoiceIssueFailed {
		t.Errorf("routing key = %q, want %q", ev.RoutingKey, pkgmq.RoutingKeyInvoiceIssueFailed)
	}
}

// resume checkpoint（§5.3.2）：已經拿到號碼就不再打 AMEGO。
func TestSubmit_ResumeCheckpointSkipsProviderCall(t *testing.T) {
	no := "AB99999999"
	h := submitHarness(t, func(d *domain.Document) { d.ProviderInvoiceNo = &no })

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if n := len(h.provider.requests); n != 0 {
		t.Fatalf("已有發票號碼仍打了 %d 次 AMEGO —— 那是第二張發票", n)
	}
	if len(h.docs.issued) != 1 || h.docs.issued[0].ProviderInvoiceNo != no {
		t.Error("checkpoint 路徑應直接落 ISSUED 並沿用既有號碼")
	}
}

// 沒有租約就送 ＝ 可能有第二個持有者同時在打 AMEGO。
func TestSubmit_RefusesWithoutClaim(t *testing.T) {
	h := submitHarness(t, nil)
	h.ops.stored.Status = codemeta.InvoiceOperationStatusReady.Value

	if err := h.uc.Submit(context.Background(), 900002); err == nil {
		t.Fatal("未 claim 就 Submit 應該失敗")
	}
	if n := len(h.provider.requests); n != 0 {
		t.Fatalf("未 claim 卻送出了 %d 次", n)
	}
}

// ClaimForSubmit 必須把 operation 與 document 兩邊一起推進。
func TestClaimForSubmit_MovesBothAggregates(t *testing.T) {
	h := submitHarness(t, func(d *domain.Document) { d.Status = codemeta.InvoiceDocumentStatusPending.Value })
	h.ops.stored.Status = codemeta.InvoiceOperationStatusReady.Value

	ok, err := h.uc.ClaimForSubmit(context.Background(), 900002)
	if err != nil {
		t.Fatalf("ClaimForSubmit: %v", err)
	}
	if !ok {
		t.Fatal("應搶到租約")
	}
	if h.ops.claimed != 1 {
		t.Error("operation 未 claim")
	}
	if h.docs.issuing != 1 {
		t.Error("document 未進 ISSUING —— 拆開之後賣家看到「待開立」而系統正在送")
	}
}

func containsBytes(b []byte, sub string) bool {
	return len(sub) > 0 && len(b) >= len(sub) && indexOf(string(b), sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// R08 必須真的**接在** failDocument 上，不是只有函式存在。
//
// 🔴 這一支存在的理由：2026-09-01 我先寫了一支直接呼叫 markReissueFailedIfAny 的
// 測試，變異驗證（把 failDocument 尾端那一行拿掉）**沒有轉紅** —— 因為那支測試
// 繞過了接線。而「函式寫對了但沒有人呼叫它」正是本服務反覆出現的那個 class
// （VoidInvoice 零呼叫端、REISSUE operation 停在 CREATED、FetchDueInboxEvents
// 零呼叫端）。⇒ 驗接線的測試必須從**外緣**進去。
func TestSubmit_ReissueFailureMarksCorrectionReissueFailed(t *testing.T) {
	orig := int64(4242)
	h := submitHarness(t, func(d *domain.Document) { d.OriginalDocumentID = &orig })
	h.corrs.openByDoc = map[int64]domain.CorrectionRequest{
		orig: {ID: 8001, Status: codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value},
	}
	h.provider.err = &amego.APIError{Code: "3040123", Class: amego.ClassPermanent, Msg: "統編錯誤"}

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if h.docs.issueFailed == 0 {
		t.Fatal("前提不成立：document 沒落 ISSUE_FAILED，這一輪沒走到 failDocument")
	}
	if len(h.corrs.reissueFail) != 1 || h.corrs.reissueFail[0] != 8001 {
		t.Fatalf("重開票開失敗，correction 卻沒落 REISSUE_FAILED：%v —— "+
			"買家手上那張已被作廢、新的不存在，而他看到的是「處理中」", h.corrs.reissueFail)
	}
}

// 一般開票失敗（非重開票）不得動任何 correction。
//
// 判準是 document 的 original_document_id 而不是「有沒有 open correction」：
// 買家可能對一張**還沒開出來**的發票提更正（R03 路徑），那時 correction 開著，
// 但這次開票失敗與它無關。
func TestSubmit_PlainIssueFailureDoesNotTouchCorrection(t *testing.T) {
	h := submitHarness(t, nil)
	h.corrs.openByDoc = map[int64]domain.CorrectionRequest{
		900001: {ID: 8001, Status: codemeta.InvoiceCorrectionStatusRequested.Value},
	}
	h.provider.err = &amego.APIError{Code: "3040123", Class: amego.ClassPermanent, Msg: "統編錯誤"}

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(h.corrs.reissueFail) != 0 {
		t.Fatalf("一般開票失敗卻把 correction 標成 REISSUE_FAILED：%v", h.corrs.reissueFail)
	}
}

// #3055：作廢重開成功必須發 P5（`invoice.reissued`），而不是只發一則 INVOICE_ISSUED。
//
// 🔴 這一支從**外緣**（Submit）進去，不直接呼叫 completeReissueIfAny：
// 「函式寫對了但沒有人呼叫它」是本服務反覆出現的那個 class（R08 的教訓就在上面）。
//
// 陽性對照在 TestSubmit_SuccessIssuesAndEnqueuesOutbox：那一支斷言初次開立**恰有一列**
// outbox ⇒ 「每次開票都發 reissued」的寫法會讓它轉紅。兩支合起來才分得出
// 「正確發出」與「每次都發」。
func TestSubmit_ReissueEmitsBothIssuedAndReissued(t *testing.T) {
	origID := int64(4242)
	origNo := "AA10000437"
	h := submitHarness(t, func(d *domain.Document) { d.OriginalDocumentID = &origID })
	h.docs.byID = map[int64]domain.Document{
		origID: {
			ID:                origID,
			Status:            codemeta.InvoiceDocumentStatusVoided.Value,
			ProviderInvoiceNo: &origNo,
		},
	}
	h.corrs.openByDoc = map[int64]domain.CorrectionRequest{
		origID: {ID: 8001, Status: codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value},
	}
	h.corrs.completeOK = true

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(h.outbox.inserted) != 2 {
		t.Fatalf("outbox 應有兩列（INVOICE_ISSUED ＋ INVOICE_REISSUED），實得 %d：%+v",
			len(h.outbox.inserted), routingKeysOf(h.outbox.inserted))
	}
	var reissued *domain.NewOutboxEvent
	for i := range h.outbox.inserted {
		if h.outbox.inserted[i].RoutingKey == pkgmq.RoutingKeyInvoiceReissued {
			reissued = &h.outbox.inserted[i]
		}
	}
	if reissued == nil {
		t.Fatalf("沒有 invoice.reissued：%v —— 重開票與初次開立在事件面不可區分",
			routingKeysOf(h.outbox.inserted))
	}
	if reissued.EventType != string(pkgmq.EventTypeInvoiceReissued) ||
		reissued.Exchange != pkgmq.ExchangeInvoice {
		t.Errorf("event_type/exchange = %q/%q", reissued.EventType, reissued.Exchange)
	}
	// 冪等鍵：CodeMeta 逐字 `INV:{new_document_id}`。用原票 id 會與原票那則的鍵相同
	// ⇒ notification 端的 UNIQUE(user_id,user_type,event_type,reference_id) 把它吞掉。
	if !containsBytes(reissued.Payload, `"notification_ref_id":"INV:900001"`) {
		t.Errorf("冪等鍵不是 INV:{新 document id}：%s", reissued.Payload)
	}
	// 🔴 兩個號碼必須取自不同來源。填反了買家會看到「原發票 X 已作廢並重新開立，
	// 新發票號碼 X」——同一個號碼，而且不會有任何東西報錯。
	if !containsBytes(reissued.Payload, `"voided_invoice_no":"AA10000437"`) {
		t.Errorf("voided_invoice_no 不是原票號：%s", reissued.Payload)
	}
	if !containsBytes(reissued.Payload, `"new_invoice_no":"AB12345678"`) {
		t.Errorf("new_invoice_no 不是新票號：%s", reissued.Payload)
	}
}

// 沒有 open correction 的重開（非更正來源）**一樣**要發 P5。
//
// 58 §7 把它掛在兩條轉移上（M1 `→ SUPERSEDED` 與 M6 R10，§8014 兩列都指向 P5），
// 而這一類只走得到前者 ⇒ 掛在 correction 那一段的話，這些買家收不到通知。
func TestSubmit_ReissueWithoutCorrectionStillEmitsReissued(t *testing.T) {
	origID := int64(4243)
	origNo := "AA10000999"
	h := submitHarness(t, func(d *domain.Document) { d.OriginalDocumentID = &origID })
	h.docs.byID = map[int64]domain.Document{
		// Status 必填：MarkSuperseded 的 allowed-from 是 ('VOIDED','ISSUED')，
		// 空字串在真實 SQL 上是 rows=0（2026-09-08 fake 補上該述詞後才現形）。
		origID: {ID: origID, Status: codemeta.InvoiceDocumentStatusVoided.Value, ProviderInvoiceNo: &origNo},
	}
	// 刻意不設 openByDoc ⇒ GetOpenByDocument 回 9307，completeReissueIfAny 提早 return。

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	found := false
	for _, ev := range h.outbox.inserted {
		if ev.RoutingKey == pkgmq.RoutingKeyInvoiceReissued {
			found = true
		}
	}
	if !found {
		t.Fatalf("非更正來源的重開沒發 P5：%v", routingKeysOf(h.outbox.inserted))
	}
}

// ---------------------------------------------------------------------------
// §P12：手機條碼載具的送出前驗證（#3256）
// ---------------------------------------------------------------------------

// barcodeHarness 給一張帶手機條碼載具的 B2C 發票。
func barcodeHarness(t *testing.T, barcode string) *harness {
	t.Helper()
	carrier := domain.AmegoCarrierMobileBarcode
	h := submitHarness(t, func(d *domain.Document) { d.CarrierType = &carrier })
	h.docs.secrets = domain.RecipientSecrets{CarrierID1: barcode}
	return h
}

// amegoErr 造一個帶碼的 provider 錯誤（*amego.APIError 是分流的唯一依據）。
func amegoErr(code string) error {
	return &amego.APIError{Endpoint: "/json/barcode", Code: code, Msg: "x", Class: amego.ClassPermanent}
}

// 載具驗證通過 ⇒ 照常送 f0401，而且驗的是 CarrierID1 本身。
func TestSubmit_ValidBarcodeStillIssues(t *testing.T) {
	h := barcodeHarness(t, "/ABC123+")

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got := h.provider.barcodeCalls; len(got) != 1 || got[0] != "/ABC123+" {
		t.Fatalf("應驗 CarrierID1 一次，實得 %v", got)
	}
	if len(h.provider.requests) != 1 {
		t.Fatalf("驗過就該照常開票，f0401 實得 %d 次", len(h.provider.requests))
	}
}

// 🔴 AMEGO 明確說載具不能用 ⇒ **不送 f0401**，轉人工。
//
// 不走 settleFailure 是重點：那會落 ISSUE_FAILED ＋ FAILED_PERMANENT，而那一格
// 沒有自動出口，法定期限一路倒數到逾期（dev 上三張票就是這樣來的）。
func TestSubmit_RejectedBarcodeGoesToAwaitingManualWithoutIssuing(t *testing.T) {
	for _, code := range []string{"9000112", "9000113"} {
		t.Run(code, func(t *testing.T) {
			h := barcodeHarness(t, "/ZZZZZZZ")
			h.provider.barcodeErr = amegoErr(code)

			if err := h.uc.Submit(context.Background(), 900002); err != nil {
				t.Fatalf("Submit: %v", err)
			}
			if len(h.provider.requests) != 0 {
				t.Fatalf("載具無效時**不得**送 f0401，實得 %d 次", len(h.provider.requests))
			}
			if h.docs.stored.Status != codemeta.InvoiceDocumentStatusAwaitingManual.Value {
				t.Fatalf("document 應轉 AWAITING_MANUAL，實得 %q", h.docs.stored.Status)
			}
			if h.docs.stored.ManualReasonCode == nil ||
				*h.docs.stored.ManualReasonCode != codemeta.InvoiceManualReasonCarrierInvalid.Value {
				t.Fatalf("manual_reason_code 應為 CARRIER_INVALID，實得 %v", h.docs.stored.ManualReasonCode)
			}
		})
	}
}

// 🔴 fail-open：驗證服務本身出問題時**不得**擋住開票。
//
// 這一支是上面那支的鑑別力來源：少了它，一個「只要 VerifyMobileBarcode 回錯就轉人工」
// 的實作也會通過——而那會讓一次 AMEGO 逾時或限流把所有帶載具的發票都推去人工排隊。
func TestSubmit_BarcodeVerifyFailureIsFailOpen(t *testing.T) {
	for _, name := range []string{"unknown_code", "not_api_error"} {
		t.Run(name, func(t *testing.T) {
			h := barcodeHarness(t, "/ABC123+")
			if name == "unknown_code" {
				h.provider.barcodeErr = amegoErr("9999999")
			} else {
				h.provider.barcodeErr = errors.New("dial tcp: i/o timeout")
			}

			if err := h.uc.Submit(context.Background(), 900002); err != nil {
				t.Fatalf("Submit: %v", err)
			}
			if len(h.provider.requests) != 1 {
				t.Fatalf("驗證本身失敗時應照常送出（fail-open），f0401 實得 %d 次",
					len(h.provider.requests))
			}
		})
	}
}

// 非手機條碼一律不驗：/json/barcode 只驗手機條碼，拿別的載具去問是白費一次配額。
func TestSubmit_NonMobileBarcodeSkipsVerification(t *testing.T) {
	h := submitHarness(t, nil) // 預設無載具
	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(h.provider.barcodeCalls) != 0 {
		t.Fatalf("無載具時不得呼叫載具驗證，實得 %v", h.provider.barcodeCalls)
	}
	if len(h.provider.requests) != 1 {
		t.Fatalf("無載具的發票照常開，f0401 實得 %d 次", len(h.provider.requests))
	}
}

// ---------------------------------------------------------------------------
// §5.2.1：已逾法定上傳期限 → AWAITING_MANUAL ＋ DEADLINE_MISSED
// ---------------------------------------------------------------------------

// 🔴 這支掃描之前不存在，而 §5.2.1 逐字要求它。business-alerts.yml 的
// InvoiceStatutoryDeadlineOverdue 註解 2026-09-05 已記載「那支掃描不存在」，
// 並警告「照那句話等掃描把它清掉的人會一直等下去」。
func TestSweepOverdueStatutoryDeadlines_MarksOnlyEligibleStatuses(t *testing.T) {
	h := submitHarness(t, nil)
	h.docs.deadlineRows = []domain.Document{
		{ID: 1, Status: codemeta.InvoiceDocumentStatusPending.Value},
		{ID: 2, Status: codemeta.InvoiceDocumentStatusIssueFailed.Value},
		// 🔴 ISSUING 代表正在送 AMEGO，逾期了也該等它跑完
		{ID: 3, Status: codemeta.InvoiceDocumentStatusIssuing.Value},
		// AWAITING_MANUAL 已在目標狀態（查詢述詞含它，是為了「逼近告警」那個用途）
		{ID: 4, Status: codemeta.InvoiceDocumentStatusAwaitingManual.Value},
	}
	// fake 的 MarkAwaitingManual 鏡射 allowed-from，需要 stored 處於可轉狀態
	h.docs.stored = domain.Document{ID: 1, Status: codemeta.InvoiceDocumentStatusPending.Value}

	now := time.Date(2026, 9, 9, 3, 0, 0, 0, time.UTC)
	n, err := h.uc.SweepOverdueStatutoryDeadlines(context.Background(), now)
	if err != nil {
		t.Fatalf("SweepOverdueStatutoryDeadlines: %v", err)
	}
	// 🔴 threshold 必須是 now 本身，不是 now+6h ——傳錯會把「還沒逾期」的文件
	// 提前標成 DEADLINE_MISSED，而那在買家面是「這張發票不會開了」。
	if !h.docs.deadlineThreshold.Equal(now) {
		t.Fatalf("threshold = %s，want %s（只取已逾期，不含逼近）", h.docs.deadlineThreshold, now)
	}
	// 🔴 這一格才是本測試的核心：4 列進來，只有 PENDING 與 ISSUE_FAILED 兩列被**嘗試**
	// （fake 把記錄放在 allowed-from 檢查之前，所以它記的是嘗試數）。ISSUING 與
	// AWAITING_MANUAL 連 UPDATE 都不該送出——前者正在打 AMEGO，後者已在目標狀態。
	if got := h.docs.awaitingManual; len(got) != 2 {
		t.Fatalf("應只嘗試 PENDING 與 ISSUE_FAILED 兩列，實得 %d 次（%v）", len(got), got)
	}
	for i, reason := range h.docs.awaitingManual {
		if reason != codemeta.InvoiceManualReasonDeadlineMissed.Value {
			t.Fatalf("第 %d 次的 reason = %q，want DEADLINE_MISSED", i, reason)
		}
	}
	// 回報數是**真的轉成功**的筆數：fake 只有一個 stored，第一次轉完它就離開
	// allowed-from ⇒ 第二次 rows=0（正常的併發結果，不是錯誤）。
	if n != 1 {
		t.Fatalf("回報標到 %d 筆，want 1（fake 的 stored 只有一列可轉）", n)
	}
}

// 空集合不是錯誤，也不得留下任何痕跡。
func TestSweepOverdueStatutoryDeadlines_EmptyIsNotAnError(t *testing.T) {
	h := submitHarness(t, nil)
	n, err := h.uc.SweepOverdueStatutoryDeadlines(context.Background(), time.Now().UTC())
	if err != nil || n != 0 {
		t.Fatalf("空集合應回 (0, nil)，實得 (%d, %v)", n, err)
	}
	if len(h.docs.awaitingManual) != 0 {
		t.Fatalf("沒有逾期文件時不得標記，實得 %v", h.docs.awaitingManual)
	}
}

func routingKeysOf(evs []domain.NewOutboxEvent) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.RoutingKey)
	}
	return out
}

// TestSubmit_CompletesCorrectionStuckAtReissueFailed 守 #3089 A 組第 1 項的**第二層**。
//
// 🔴 第二層比第一層隱蔽：Complete 的 allowed-from 明明含 REISSUE_FAILED（SQL 是對的），
// 但唯一的查表入口 GetOpenByDocument 的述詞只有 REQUESTED／VOID_REISSUE_IN_PROGRESS
// ⇒ 停在 REISSUE_FAILED 的查不到，查無時靜默 `return nil`。
// 後果是「ADM 重試成功、新票開出來了、買家也收到通知，而 correction 永遠停在
// REISSUE_FAILED、reissued_document_id 永遠 NULL」——那段 allowed-from 是一條
// 永遠走不到的死路徑。
//
// 這一道與端點 52 的 R09 連動是**兩件事**：R09 走的是「ADM 按重試 ⇒ correction 先回
// VOID_REISSUE_IN_PROGRESS」，那之後上面那一支就找得到；這一道接的是**繞過端點 52
// 的重開**（worker 自己重試成功、或任何未來新增的重開路徑）。
// 拿掉任一道，對應的情境就重新卡住 ⇒ 兩者各自要能單獨轉紅。
func TestSubmit_CompletesCorrectionStuckAtReissueFailed(t *testing.T) {
	origID := int64(4244)
	origNo := "AA10000501"
	h := submitHarness(t, func(d *domain.Document) { d.OriginalDocumentID = &origID })
	h.docs.byID = map[int64]domain.Document{
		origID: {
			ID:                origID,
			Status:            codemeta.InvoiceDocumentStatusVoided.Value,
			ProviderInvoiceNo: &origNo,
		},
	}
	// 🔴 刻意**只**設 failedByDoc、不設 openByDoc：這正是 dev 上那 3 列的形狀。
	// 用同一份 map 服務兩支查詢的替身會讓這個測試恆綠（見 fakeCorrections 的註解）。
	h.corrs.failedByDoc = map[int64]domain.CorrectionRequest{
		origID: {ID: 8002, Status: codemeta.InvoiceCorrectionStatusReissueFailed.Value},
	}
	h.corrs.completeOK = true

	if err := h.uc.Submit(context.Background(), 900002); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(h.corrs.completed) != 1 || h.corrs.completed[0] != 8002 {
		t.Fatalf("停在 REISSUE_FAILED 的 correction 沒有被收尾：%v —— "+
			"新票已經開出來了，而那一列會永遠停在那裡、reissued_document_id 永遠 NULL",
			h.corrs.completed)
	}
}

// #3082 第二階段：開票軸的失敗落點，reason_code 一律在 CodeMeta 值域內。
//
// 在此之前**九處**把 `apiErr.Code`／`providerCode`（AMEGO 的原始碼）直接寫進
// invoice_status_history.reason_code，而那一欄的值域是 InvoiceManualReason。
// 折讓軸裝 CodeMeta 值、開票與作廢軸裝 AMEGO 碼 ⇒ ADM 要對它做分類統計，
// 只能先判斷「這是哪一種碼」，而沒有欄位能依據。
//
// 🔴 這一支守的是**整個類**不是單一實例：它枚舉四種失敗分類各跑一次，
// 斷言每一筆狀態歷程的 reason_code 都解析得回 InvoiceManualReason。
// 未來任何新的失敗落點若又塞 AMEGO 碼，這裡會轉紅。
func TestSubmit_FailureReasonCodesAreAlwaysCodeMeta(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *amego.APIError
	}{
		{"可重試", &amego.APIError{Code: "3050121", Class: amego.ClassRetryable, Msg: "上一個異動還在排隊", KnownCode: true}},
		{"永久拒絕", &amego.APIError{Code: "3040123", Class: amego.ClassPermanent, Msg: "統編錯誤", KnownCode: true}},
		{"字軌不足", &amego.APIError{Code: "3040111", Class: amego.ClassManual, Msg: "字軌不足", KnownCode: true}},
		{"結果不明", &amego.APIError{Code: "TIMEOUT", Class: amego.ClassReconcile, Msg: "timeout", KnownCode: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := submitHarness(t, nil)
			h.provider.err = tc.err
			if err := h.uc.Submit(context.Background(), 900001); err != nil {
				t.Fatalf("Submit: %v", err)
			}
			if len(h.audit.entries) == 0 {
				t.Fatal("一筆狀態歷程都沒寫 —— 這一輪沒有走到失敗落點，測試沒有鑑別力")
			}
			for _, e := range h.audit.entries {
				if e.ReasonCode == nil || *e.ReasonCode == "" {
					continue // 沒填 reason_code 是另一件事，不在本測試範圍
				}
				if _, ok := codemeta.InvoiceManualReasonFromValue(*e.ReasonCode); !ok {
					t.Errorf("reason_code = %q 不在 InvoiceManualReason 值域內 —— "+
						"那一欄的值域是 CodeMeta，塞 AMEGO 的碼會讓 ADM 的分類統計"+
						"只能靠字串長相猜（#3082）。AMEGO 的原話應該走 reason_detail 與 provider_code。",
						*e.ReasonCode)
				}
			}
		})
	}
}

// #3082：開票軸的永久失敗也要把 provider 的原話落進專用欄。
//
// 🔴 這一支是 **dev 實機驗證抓到的遺漏**補的測試：第二階段只改了
// invoice_status_history 的 reason_code，而 repository 呼叫還是舊的 MarkFailed
// ⇒ 實測 op 7500463275386077184 落 FAILED_PERMANENT 之後 reason_code 對了
// （PROVIDER_REJECTED_PERMANENT）、provider_code 與 provider_message 仍是空的。
//
// **白盒推理與當時的 unit test 都看不到這一層**——因為替身只計數。
func TestSubmit_PermanentFailureRecordsProviderResult(t *testing.T) {
	h := submitHarness(t, nil)
	h.provider.err = &amego.APIError{
		Code: "3040122", Class: amego.ClassPermanent, KnownCode: true,
		Msg: "BuyerIdentifier 格式錯誤",
	}
	if err := h.uc.Submit(context.Background(), 900001); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if h.ops.gotProviderCode != "3040122" {
		t.Errorf("provider_code = %q，want \"3040122\" —— 這一欄在修復前從未被寫入，"+
			"ADM 的服務商往來只能去讀 last_error 的自由文字", h.ops.gotProviderCode)
	}
}

// TestSubmit_SupersedeHistoryUsesRealFromStatus 守 R10 那一列 history 的 from。
//
// 🔴 `MarkInvoiceDocumentSuperseded` 的 allowed-from 是 **兩個值**：
// `('VOIDED', 'ISSUED')`。後者是「作廢確認遲到」的補正路徑——f0501 已受理但
// `invoice_query` 還沒翻面時，重開票會先 ISSUED 而原票仍停在 ISSUED。
// 原本 from 硬寫 VOIDED ⇒ 那條路徑上留下一條**沒有發生過**的邊，而
// invoice_status_history 正是 A10 稽核與 AuditTrailDrawer 呈現給人看的東西。
//
// 期望值硬寫字面值，不引用被測物用到的常數。
func TestSubmit_SupersedeHistoryUsesRealFromStatus(t *testing.T) {
	for _, c := range []struct{ name, origStatus string }{
		{"原票已作廢（一般路徑）", "VOIDED"},
		{"原票仍 ISSUED（作廢確認遲到的補正路徑）", "ISSUED"},
	} {
		t.Run(c.name, func(t *testing.T) {
			origID := int64(4244)
			origNo := "AA10000555"
			h := submitHarness(t, func(d *domain.Document) { d.OriginalDocumentID = &origID })
			h.docs.byID = map[int64]domain.Document{
				origID: {ID: origID, Status: c.origStatus, ProviderInvoiceNo: &origNo},
			}
			if err := h.uc.Submit(context.Background(), 900002); err != nil {
				t.Fatalf("Submit: %v", err)
			}
			var got string
			found := false
			for _, e := range h.audit.entries {
				if e.SubjectType == subjectDocument && e.SubjectID == origID &&
					e.ToStatus == "SUPERSEDED" {
					got = derefString(e.FromStatus)
					found = true
				}
			}
			if !found {
				t.Fatalf("找不到原票的 → SUPERSEDED 那一列（走過 %d 列）", len(h.audit.entries))
			}
			if got != c.origStatus {
				t.Errorf("history 的 from = %q，want %q —— 硬寫 VOIDED 會在 ISSUED 那條"+
					"補正路徑上留下一條沒有發生過的邊", got, c.origStatus)
			}
		})
	}
}
