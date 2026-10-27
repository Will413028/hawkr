package usecase

import (
	"context"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

var (
	testNow     = time.Date(2026, 8, 30, 2, 0, 0, 0, time.UTC) // 台北 2026-08-30 10:00
	testCutover = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
)

type harness struct {
	uc       *Issuance
	docs     *fakeDocRepo
	ops      *fakeOpRepo
	outbox   *fakeOutbox
	audit    *fakeAudit
	provider *fakeProvider
	corrs    *fakeCorrections
	receipts *fakeReceiptRenderer
	receiptS *fakeReceiptUploader
}

func newHarness(t *testing.T, profile domain.SellerTaxProfile, account domain.ProviderAccount) *harness {
	t.Helper()
	docs := &fakeDocRepo{}
	ops := &fakeOpRepo{}
	ob := &fakeOutbox{}
	au := &fakeAudit{}
	pv := &fakeProvider{result: &amegoIssueOK}
	rr := &fakeReceiptRenderer{}
	ru := &fakeReceiptUploader{}

	var seq int64
	corrs := &fakeCorrections{}
	uc := NewIssuance(
		directTx{},
		&fakeIssuerRepo{issuer: domain.Issuer{ID: 700001, DisplayName: "測試賣場"}},
		&fakeProfileRepo{profile: profile},
		&fakeAccountRepo{account: account, taxID: "12345678", appKey: "k"},
		docs, ops, corrs, ob, au, pv,
		rr, ru,
		func() int64 { seq++; return 900000 + seq },
		IssuanceConfig{
			MaxAttempts: 13,
			BaseBackoff: 30 * time.Second,
			MaxBackoff:  30 * time.Minute,
			IssueLease:  120 * time.Second,
			RecheckBase: 30 * time.Second,
		},
		nil,
	).WithClock(func() time.Time { return testNow })

	return &harness{uc: uc, docs: docs, ops: ops, corrs: corrs, outbox: ob, audit: au, provider: pv,
		receipts: rr, receiptS: ru}
}

// approvedProfile 是「已核定、應開統一發票」的賣家。
func approvedProfile(taxCategory int16) domain.SellerTaxProfile {
	legal := "測試股份有限公司"
	tax := "12345678"
	return domain.SellerTaxProfile{
		ID:                 1,
		ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		DefaultTaxCategory: taxCategory,
		LegalName:          &legal,
		TaxID:              &tax,
	}
}

func activeAccount() domain.ProviderAccount {
	return domain.ProviderAccount{
		ID:                  555,
		Status:              codemeta.InvoiceProviderAccountStatusActive.Value,
		AutomationCutoverAt: &testCutover,
		Version:             3,
	}
}

// baseInput 是一張最單純的 B2C 訂單：一件商品 100 元、無運費、無折扣。
func baseInput() OrderPaidInput {
	return OrderPaidInput{
		OrderNo:         "O000123456",
		BuyerID:         2623900520,
		StoreID:         2685914682,
		SnapshotVersion: 1,
		PaidAt:          testNow,
		Snapshot: domain.OrderSnapshot{
			TotalOriginalAmount: 100,
			ShippingFee:         0,
			DiscountAmount:      0,
			TotalActualAmount:   100,
			DiscountFunding:     domain.DiscountFundingSeller,
			Items: []domain.OrderItem{
				{OrderItemID: 1, SkuID: 11, ProductName: "有機高麗菜", Quantity: 1, UnitPrice: 100},
			},
		},
	}
}

var amegoIssueOK = amegoResult()

// ---------------------------------------------------------------------------
// §7.1.3 決策樹：逐葉節點
//
// 判準不是「有幾個測試」而是「每一個葉節點都有一支測試走到它」。逐葉列表對帳，不數個數：
// 數個數在新增一格分支時不會轉紅，而那正是決策樹最常見的改法。
// ---------------------------------------------------------------------------

func TestPrepare_DecisionTreeLeaves(t *testing.T) {
	exemptCat := int16(domain.TaxCategoryExempt)

	cases := []struct {
		leaf    string // §7.1.3 mermaid 的節點代號
		profile domain.SellerTaxProfile
		account domain.ProviderAccount
		mutate  func(*OrderPaidInput)

		wantMode   string
		wantKind   string
		wantStatus string
		wantNA     string
		wantManual string
		// wantOp 是「這一格會不會建 operation」以及它建完之後的狀態。
		wantOpStatus string
	}{
		{
			leaf:    "NA1 零元",
			profile: approvedProfile(1), account: activeAccount(),
			mutate: func(in *OrderPaidInput) {
				in.Snapshot.TotalActualAmount = 0
				in.Snapshot.TotalOriginalAmount = 0
				in.Snapshot.Items[0].UnitPrice = 0
				in.Snapshot.Items = nil
			},
			wantMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusNotApplicable.Value,
			wantNA:       codemeta.InvoiceNotApplicableReasonZeroAmount.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusNotApplicable.Value,
		},
		{
			leaf: "R1 免用統一發票 ＋ 已授權 → 平台代製收據待產製",
			profile: domain.SellerTaxProfile{
				ReviewStatus:        codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
				DefaultTaxCategory:  1,
				ReceiptAuthorizedAt: &testNow,
			},
			account: activeAccount(),
			// #3026：收據產製已落地 ⇒ 這一葉翻回 PENDING ＋ operation READY。
			// **READY 是這一格的重點**：它代表 INV.op 會來撿，而 submit.go 的
			// PLATFORM_RECEIPT 分支會接住它走本地產製（而不是 #3022 那時的 ISSUE_FAILED）。
			wantMode:     codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
			wantKind:     codemeta.InvoiceDocumentKindReceipt.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusPending.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusReady.Value,
		},
		{
			leaf: "R1 免用統一發票 ＋ 未授權 → 轉人工催授權",
			profile: domain.SellerTaxProfile{
				ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
				DefaultTaxCategory: 1,
				// ReceiptAuthorizedAt 為 nil ＝ 賣家沒在 S05 按同意。
			},
			account: activeAccount(),
			// 🔴 operation **不得**是 READY：沒有授權就不該有任何東西去產製它。
			wantMode:     codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
			wantKind:     codemeta.InvoiceDocumentKindReceipt.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			wantManual:   codemeta.InvoiceManualReasonReceiptAuthMissing.Value,
			wantOpStatus: "",
		},
		{
			leaf: "S1 賣家自行開票",
			profile: domain.SellerTaxProfile{
				ReviewStatus:       codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
				DefaultTaxCategory: 1,
			},
			account:      activeAccount(),
			wantMode:     codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusNotApplicable.Value,
			wantNA:       codemeta.InvoiceNotApplicableReasonSelfIssue.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusNotApplicable.Value,
		},
		{
			leaf: "S2 稅務資格未核定",
			profile: domain.SellerTaxProfile{
				ReviewStatus:       codemeta.SellerTaxReviewStatusUnderReview.Value,
				DefaultTaxCategory: 1,
			},
			account:      activeAccount(),
			wantMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusNotApplicable.Value,
			wantNA:       codemeta.InvoiceNotApplicableReasonTaxProfileNotApproved.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusNotApplicable.Value,
		},
		{
			leaf:    "S3 provider 帳戶不可用",
			profile: approvedProfile(1),
			account: domain.ProviderAccount{
				ID:                  555,
				Status:              codemeta.InvoiceProviderAccountStatusSuspended.Value,
				AutomationCutoverAt: &testCutover,
			},
			wantMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusNotApplicable.Value,
			wantNA:       codemeta.InvoiceNotApplicableReasonProviderNotActive.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusNotApplicable.Value,
		},
		{
			leaf:    "S4 切點未到",
			profile: approvedProfile(1),
			account: domain.ProviderAccount{
				ID:     555,
				Status: codemeta.InvoiceProviderAccountStatusActive.Value,
				// cutover 未設定 ＝ 尚未切過去。
			},
			wantMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusNotApplicable.Value,
			wantNA:       codemeta.InvoiceNotApplicableReasonCutoverNotReached.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusNotApplicable.Value,
		},
		{
			leaf:    "AM-a 平台出資券",
			profile: approvedProfile(1), account: activeAccount(),
			mutate:     func(in *OrderPaidInput) { in.HasPlatformFundedDiscount = true },
			wantMode:   codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			wantKind:   codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus: codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			wantManual: codemeta.InvoiceManualReasonPlatformFundedDiscount.Value,
			// AM 節點明文「operation 不建」。
			wantOpStatus: "",
		},
		{
			leaf:    "AM-b 和單訂單",
			profile: approvedProfile(1), account: activeAccount(),
			mutate:       func(in *OrderPaidInput) { in.IsConsolidatedOrder = true },
			wantMode:     codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			wantManual:   codemeta.InvoiceManualReasonConsolidatedOrder.Value,
			wantOpStatus: "",
		},
		{
			leaf:    "AM-c 混合稅別（免稅商品 ＋ 應稅運費）",
			profile: approvedProfile(exemptCat), account: activeAccount(),
			mutate: func(in *OrderPaidInput) {
				in.Snapshot.ShippingFee = 60
				in.Snapshot.TotalActualAmount = 160
			},
			wantMode:     codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			wantManual:   codemeta.InvoiceManualReasonMixedTaxCategory.Value,
			wantOpStatus: "",
		},
		{
			leaf:    "B2C 自動開票",
			profile: approvedProfile(1), account: activeAccount(),
			wantMode:     codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2c.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusPending.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusReady.Value,
		},
		{
			leaf:    "B2B 自動開票（買家帶統編）",
			profile: approvedProfile(1), account: activeAccount(),
			mutate: func(in *OrderPaidInput) {
				in.BuyerTaxID = "36333333"
				in.BuyerTitle = "光貿科技股份有限公司"
			},
			wantMode:     codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			wantKind:     codemeta.InvoiceDocumentKindB2b.Value,
			wantStatus:   codemeta.InvoiceDocumentStatusPending.Value,
			wantOpStatus: codemeta.InvoiceOperationStatusReady.Value,
		},
	}

	for _, tc := range cases {
		t.Run(tc.leaf, func(t *testing.T) {
			h := newHarness(t, tc.profile, tc.account)
			in := baseInput()
			if tc.mutate != nil {
				tc.mutate(&in)
			}
			got, err := h.uc.Prepare(context.Background(), in)
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			if got.FulfillmentMode != tc.wantMode {
				t.Errorf("fulfillment_mode = %q, want %q", got.FulfillmentMode, tc.wantMode)
			}
			if got.DocumentKind != tc.wantKind {
				t.Errorf("document_kind = %q, want %q", got.DocumentKind, tc.wantKind)
			}
			if got.DocumentStatus != tc.wantStatus {
				t.Errorf("document.status = %q, want %q", got.DocumentStatus, tc.wantStatus)
			}
			if got.NotApplicableReason != tc.wantNA {
				t.Errorf("not_applicable_reason = %q, want %q", got.NotApplicableReason, tc.wantNA)
			}
			if got.ManualReasonCode != tc.wantManual {
				t.Errorf("manual_reason_code = %q, want %q", got.ManualReasonCode, tc.wantManual)
			}
			if got.OperationStatus != tc.wantOpStatus {
				t.Errorf("operation.status = %q, want %q", got.OperationStatus, tc.wantOpStatus)
			}

			// chk_invdoc_deadline_presence 是雙向等式且**與 status 無關**：
			// 連 NOT_APPLICABLE 的 B2C 也必須有期限，RECEIPT 則必須沒有。
			doc := h.docs.created[0]
			wantDeadline := tc.wantKind != codemeta.InvoiceDocumentKindReceipt.Value
			if (doc.StatutoryDeadlineAt != nil) != wantDeadline {
				t.Errorf("statutory_deadline_at present = %v, want %v (kind=%s)",
					doc.StatutoryDeadlineAt != nil, wantDeadline, tc.wantKind)
			}
			// chk_invdoc_receipt 是雙向等式：RECEIPT ⇔ receipt_no 非空。
			if (doc.ReceiptNo != "") != (tc.wantKind == codemeta.InvoiceDocumentKindReceipt.Value) {
				t.Errorf("receipt_no = %q for kind=%s", doc.ReceiptNo, tc.wantKind)
			}
			// chk_invdoc_kind_mode 是雙向等式。
			if (doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value) !=
				(doc.FulfillmentMode == codemeta.InvoiceFulfillmentModePlatformReceipt.Value) {
				t.Errorf("kind=%s mode=%s 違反 chk_invdoc_kind_mode", doc.DocumentKind, doc.FulfillmentMode)
			}
		})
	}
}

// 逐葉對帳：上表必須覆蓋 InvoiceNotApplicableReason 的全部 5 值。
//
// 這一支不是「再數一次個數」——它從 **CodeMeta 的值域**反向檢查，所以碼表長胖而決策樹
// 沒跟上時它會轉紅，而上面那張表不會。
func TestPrepare_CoversEveryNotApplicableReason(t *testing.T) {
	produced := map[string]bool{}

	type scenario struct {
		profile domain.SellerTaxProfile
		account domain.ProviderAccount
		mutate  func(*OrderPaidInput)
	}
	for _, s := range []scenario{
		{approvedProfile(1), activeAccount(), func(in *OrderPaidInput) {
			in.Snapshot.TotalActualAmount = 0
			in.Snapshot.Items = nil
		}},
		{domain.SellerTaxProfile{ReviewStatus: codemeta.SellerTaxReviewStatusSelfIssuePending.Value}, activeAccount(), nil},
		{domain.SellerTaxProfile{ReviewStatus: codemeta.SellerTaxReviewStatusDraft.Value}, activeAccount(), nil},
		{approvedProfile(1), domain.ProviderAccount{ID: 1, Status: codemeta.InvoiceProviderAccountStatusRevoked.Value, AutomationCutoverAt: &testCutover}, nil},
		{approvedProfile(1), domain.ProviderAccount{ID: 1, Status: codemeta.InvoiceProviderAccountStatusActive.Value}, nil},
		// R1 免用統一發票（#3022）：這一格在 2026-09-02 之前產不出任何 reason
		// ——它落 PENDING，而 PENDING 沒有 not_applicable_reason。
		{domain.SellerTaxProfile{
			ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
			DefaultTaxCategory: 1,
		}, activeAccount(), nil},
	} {
		h := newHarness(t, s.profile, s.account)
		in := baseInput()
		if s.mutate != nil {
			s.mutate(&in)
		}
		got, err := h.uc.Prepare(context.Background(), in)
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		if got.NotApplicableReason != "" {
			produced[got.NotApplicableReason] = true
		}
	}

	for _, meta := range codemeta.AllInvoiceNotApplicableReasons {
		if !produced[meta.Value] {
			t.Errorf("InvoiceNotApplicableReason %q 沒有任何測試走到它的 emitter", meta.Value)
		}
	}
}

// ---------------------------------------------------------------------------
// §5A.11 fail-closed：斷言失敗一律轉人工，且**沒有任何請求送出去**
// ---------------------------------------------------------------------------

func TestPrepare_InvariantFailureGoesToAwaitingManualAndNeverSends(t *testing.T) {
	h := newHarness(t, approvedProfile(1), activeAccount())
	in := baseInput()
	// A1：Σ line.gross(100) != total_actual(999)。
	in.Snapshot.TotalActualAmount = 999

	got, err := h.uc.Prepare(context.Background(), in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if got.DocumentStatus != codemeta.InvoiceDocumentStatusAwaitingManual.Value {
		t.Fatalf("status = %q, want AWAITING_MANUAL", got.DocumentStatus)
	}
	if got.ManualReasonCode != domain.CodeAmountMismatch {
		t.Fatalf("manual_reason_code = %q, want %q", got.ManualReasonCode, domain.CodeAmountMismatch)
	}
	// 陰性斷言：這一格的價值就在「沒有送出去」。
	if n := len(h.provider.requests); n != 0 {
		t.Fatalf("provider 收到 %d 次請求，斷言失敗的草稿不得送出", n)
	}
	if got.OperationID != 0 {
		t.Fatalf("AWAITING_MANUAL 不應建立 operation，卻建了 %d", got.OperationID)
	}
	// 落庫路徑：先建 PENDING（M1 的建立邊只有兩條），同一交易內接 PENDING → AWAITING_MANUAL。
	if h.docs.created[0].Status != codemeta.InvoiceDocumentStatusPending.Value {
		t.Errorf("建立邊寫成 %q，M1 沒有 [*] → AWAITING_MANUAL 這條邊", h.docs.created[0].Status)
	}
	if len(h.docs.awaitingManual) != 1 || h.docs.awaitingManual[0] != domain.CodeAmountMismatch {
		t.Errorf("manual_reason_code 未與狀態同一交易寫入：%v", h.docs.awaitingManual)
	}
	// P7：fail-closed 落點必須通知賣家（#3055 的橫向稽查）。
	//
	// 🔴 斷言掛在**這一支**（走 Prepare 的外緣）而不是另寫一支直接呼叫 emitter 的：
	// 直接呼叫 emitter 的測試對「接線被拿掉」完全無感 —— 本波第一版就是那樣，
	// 變異驗證沒轉紅才發現。
	// 少了這一則，賣家只會發現發票一直沒開，而系統已經把它掛在 ADM 的人工佇列上等人。
	if len(h.outbox.inserted) != 1 ||
		h.outbox.inserted[0].RoutingKey != pkgmq.RoutingKeyInvoiceOperationReviewRequired {
		t.Fatalf("轉人工必須發 P7，實得 %v", routingKeysOf(h.outbox.inserted))
	}
	// store_id 走 issuer_ref_id：dispatcher 拿它解析賣家帳號，填 PK 會讓通知靜默不送。
	if !containsBytes(h.outbox.inserted[0].Payload, `"review_reason_code":"`+domain.CodeAmountMismatch+`"`) {
		t.Errorf("review_reason_code 沒帶上：%s", h.outbox.inserted[0].Payload)
	}
}

// ---------------------------------------------------------------------------
// 冪等：重複消費 order.paid 不得再建一次
// ---------------------------------------------------------------------------

func TestPrepare_DuplicateOrderPaidIsIdempotent(t *testing.T) {
	h := newHarness(t, approvedProfile(1), activeAccount())
	h.docs.notInserted = true

	got, err := h.uc.Prepare(context.Background(), baseInput())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !got.AlreadyExisted {
		t.Fatal("uq_invdoc_order_identity 命中時應回 AlreadyExisted")
	}
	if len(h.docs.lines) != 0 {
		t.Errorf("冪等命中仍寫了 %d 行明細", len(h.docs.lines))
	}
	if len(h.ops.created) != 0 {
		t.Errorf("冪等命中仍建了 %d 個 operation", len(h.ops.created))
	}
}

// ---------------------------------------------------------------------------
// 冪等鍵與衍生識別字（§5.3.2）
// ---------------------------------------------------------------------------

func TestPrepare_DerivedIdentifiers(t *testing.T) {
	h := newHarness(t, approvedProfile(1), activeAccount())
	if _, err := h.uc.Prepare(context.Background(), baseInput()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	doc := h.docs.created[0]
	if doc.ProviderRequestNo != "O000123456-1-1" {
		t.Errorf("provider_request_no = %q，兩個版本軸（snapshot_version／issue_attempt_seq）必須都在鍵裡", doc.ProviderRequestNo)
	}
	op := h.ops.created[0]
	wantKey := "issue:" + itoa(doc.ID) + ":1"
	if op.OperationKey != wantKey {
		t.Errorf("operation_key = %q, want %q", op.OperationKey, wantKey)
	}
	if op.ProviderRequestNo != doc.ProviderRequestNo {
		t.Errorf("operation.provider_request_no 必須等於 document 的")
	}
	// I-52：operation 的 statutory_deadline_at 是 document 同名欄的副本。
	if op.StatutoryDeadlineAt == nil || doc.StatutoryDeadlineAt == nil ||
		!op.StatutoryDeadlineAt.Equal(*doc.StatutoryDeadlineAt) {
		t.Errorf("statutory_deadline_at 副本未在同一交易內複製")
	}
	if op.MaxAttempts != 13 {
		t.Errorf("max_attempts = %d, want 13（§0.2：12 段退避覆蓋 4.02h，取 12 只有 3.52h）", op.MaxAttempts)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// R08：作廢重開的新票**開失敗**時，correction 必須落 REISSUE_FAILED。
//
// 🔴 少了它，correction 永遠停在 VOID_REISSUE_IN_PROGRESS —— 那個狀態沒有任何
// 掃描器，而買家看到「處理中」，實際上他手上那張已經被作廢、新的一張不存在。
// 設計書逐字稱 REISSUE_FAILED 為「本設計書中最嚴重的單一狀態」，正是這個組合。
func TestFailDocument_ReissueMarksCorrectionReissueFailed(t *testing.T) {
	h := newHarness(t, approvedProfile(1), activeAccount())
	orig := int64(4242)
	doc := domain.Document{
		ID: 9001, IssuerID: 700001, Status: codemeta.InvoiceDocumentStatusIssuing.Value,
		OriginalDocumentID: &orig,
	}
	h.corrs.openByDoc = map[int64]domain.CorrectionRequest{
		orig: {ID: 8001, Status: codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value},
	}

	if err := h.uc.markReissueFailedIfAny(context.Background(), doc, testNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.corrs.reissueFail) != 1 || h.corrs.reissueFail[0] != 8001 {
		t.Fatalf("correction 沒被標成 REISSUE_FAILED：%v", h.corrs.reissueFail)
	}
}

// 非重開票（一般開票失敗）不得動任何 correction。
//
// 判準是 document 自己的 original_document_id —— 用它而不是「查有沒有 open
// correction」是因為買家可能對一張**還沒開出來**的發票提更正（R03 路徑），
// 那時 correction 是開著的，但這次失敗與它無關。
func TestFailDocument_PlainIssueDoesNotTouchCorrection(t *testing.T) {
	h := newHarness(t, approvedProfile(1), activeAccount())
	doc := domain.Document{ID: 9001, IssuerID: 700001}
	h.corrs.openByDoc = map[int64]domain.CorrectionRequest{9001: {ID: 8001}}

	if err := h.uc.markReissueFailedIfAny(context.Background(), doc, testNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.corrs.reissueFail) != 0 {
		t.Fatalf("一般開票失敗卻動了 correction：%v", h.corrs.reissueFail)
	}
}
