package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

// fakePrefRepo 嵌入介面（見 fake_test.go 檔頭）：沒被覆寫的方法是 nil，呼叫即 panic
// 並指名方法 —— 「這條路徑本來就不該被走到」與「回了零值」因此分得開。
type fakePrefRepo struct {
	domain.BuyerPreferenceRepository

	// byID 是 GetByID 的回應。err 非 nil 時優先回它。
	byID domain.BuyerPreference
	err  error

	// def / defFound 是 GetDefault 的回應。
	def      domain.BuyerPreference
	defFound bool

	// gotByID 記下 GetByID 實際收到的 (id, buyerID)。歸屬驗證是「buyerID 有沒有進到
	// 查詢述詞」，而那件事只有記下實參才驗得到。
	gotByID [2]int64
}

func (f *fakePrefRepo) GetByID(_ context.Context, id, buyerID int64) (domain.BuyerPreference, error) {
	f.gotByID = [2]int64{id, buyerID}
	if f.err != nil {
		return domain.BuyerPreference{}, f.err
	}
	return f.byID, nil
}

func (f *fakePrefRepo) GetDefault(context.Context, int64) (domain.BuyerPreference, bool, error) {
	return f.def, f.defFound, nil
}

type checkoutHarness struct {
	uc    *Checkout
	prefs *fakePrefRepo
}

func newCheckoutHarness(t *testing.T, profile domain.SellerTaxProfile, account domain.ProviderAccount) *checkoutHarness {
	t.Helper()
	prefs := &fakePrefRepo{}
	uc := NewCheckout(
		&fakeIssuerRepo{issuer: domain.Issuer{ID: 700001, DisplayName: "測試賣場"}},
		&fakeProfileRepo{profile: profile},
		&fakeAccountRepo{account: account},
		prefs,
		nil,
	).WithClock(func() time.Time { return testNow })
	return &checkoutHarness{uc: uc, prefs: prefs}
}

// previewOf 跑一次 preview（100 元商品、無運費、無折扣），與 issuance_test.go 的
// baseInput 是同一張訂單。
func (h *checkoutHarness) previewOf(t *testing.T, mutate func(*ResolveModeInput)) ResolveModeOutput {
	t.Helper()
	in := ResolveModeInput{
		StoreID:       2685914682,
		BuyerID:       2623900520,
		ProductAmount: 100,
		CheckoutID:    "CK-0001",
	}
	if mutate != nil {
		mutate(&in)
	}
	out, err := h.uc.ResolveMode(context.Background(), in)
	if err != nil {
		t.Fatalf("ResolveMode 失敗：%v", err)
	}
	return out
}

func assertLeaf(t *testing.T, got ResolveModeOutput, wantMode, wantKind, wantNA string) {
	t.Helper()
	if got.InvoiceMode != wantMode {
		t.Errorf("invoice_mode = %q, want %q", got.InvoiceMode, wantMode)
	}
	if got.DocumentKind != wantKind {
		t.Errorf("document_kind = %q, want %q", got.DocumentKind, wantKind)
	}
	if got.NotApplicableReason != wantNA {
		t.Errorf("not_applicable_reason = %q, want %q", got.NotApplicableReason, wantNA)
	}
}

// ---------------------------------------------------------------------------
// §7.1.3 決策樹：逐葉一支測試
//
// 刻意**不用表格迴圈**：迴圈少一個 case 不會有任何訊號，而決策樹最常見的改法正是
// 「新增一格分支」。一葉一函式時，漏掉的那一葉在檔案裡看得見。
// ---------------------------------------------------------------------------

func TestResolveMode_LeafNA1_ZeroAmount(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	got := h.previewOf(t, func(in *ResolveModeInput) { in.ProductAmount = 0 })
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModeLegacyManual.Value,
		codemeta.InvoiceDocumentKindB2c.Value,
		codemeta.InvoiceNotApplicableReasonZeroAmount.Value)
}

func TestResolveMode_LeafR1_InvoiceExemptToPlatformReceipt(t *testing.T) {
	authorized := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	h := newCheckoutHarness(t, domain.SellerTaxProfile{
		ReviewStatus:        codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
		DefaultTaxCategory:  1,
		ReceiptAuthorizedAt: &authorized,
	}, activeAccount())
	got := h.previewOf(t, nil)
	// #3026：已授權 ⇒ 收據會被產製，preview 不帶 notApplicableReason。
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
		codemeta.InvoiceDocumentKindReceipt.Value,
		"")
}

// 🔴 preview 與 Prepare 必須把 **同一個** SellerReceiptAuthorized 值餵進決策樹。
//
// 為什麼不在 previewOf 那一層驗：ResolveModeOutput **沒有 status 欄位**，而已授權與
// 未授權的差別正好只在 status（PENDING vs AWAITING_MANUAL）⇒ 從 preview 的輸出看
// 兩者完全相同，那樣的測試對這條線零鑑別力（拿掉 checkout.go 那一行也不會轉紅）。
// 所以直接驗組裝函式本身。
func TestFulfillmentInputFor_CarriesReceiptAuthorization(t *testing.T) {
	authorized := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)

	facts := checkoutSellerFacts{
		profile: domain.SellerTaxProfile{ReceiptAuthorizedAt: &authorized},
		account: activeAccount(),
	}
	if got := fulfillmentInputFor(facts, 1000, now, false); !got.SellerReceiptAuthorized {
		t.Error("已授權的賣家，SellerReceiptAuthorized 應為 true")
	}

	facts.profile.ReceiptAuthorizedAt = nil
	if got := fulfillmentInputFor(facts, 1000, now, false); got.SellerReceiptAuthorized {
		t.Error("未授權的賣家，SellerReceiptAuthorized 應為 false")
	}
}

func TestResolveMode_LeafS1_SelfIssuePending(t *testing.T) {
	h := newCheckoutHarness(t, domain.SellerTaxProfile{
		ReviewStatus:            codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
		DefaultTaxCategory:      1,
		PendingOrderLimitCount:  50,
		PendingOrderLimitAmount: 100000,
	}, activeAccount())
	got := h.previewOf(t, nil)
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value,
		codemeta.InvoiceDocumentKindB2c.Value,
		codemeta.InvoiceNotApplicableReasonSelfIssue.Value)
}

func TestResolveMode_LeafS2_TaxProfileNotApproved(t *testing.T) {
	h := newCheckoutHarness(t, domain.SellerTaxProfile{
		ReviewStatus:       codemeta.SellerTaxReviewStatusUnderReview.Value,
		DefaultTaxCategory: 1,
	}, activeAccount())
	got := h.previewOf(t, nil)
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModeLegacyManual.Value,
		codemeta.InvoiceDocumentKindB2c.Value,
		codemeta.InvoiceNotApplicableReasonTaxProfileNotApproved.Value)
}

func TestResolveMode_LeafS3_ProviderNotActive(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), domain.ProviderAccount{
		ID:                  555,
		Status:              codemeta.InvoiceProviderAccountStatusSuspended.Value,
		AutomationCutoverAt: &testCutover,
	})
	got := h.previewOf(t, nil)
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModeLegacyManual.Value,
		codemeta.InvoiceDocumentKindB2c.Value,
		codemeta.InvoiceNotApplicableReasonProviderNotActive.Value)
}

func TestResolveMode_LeafS4_CutoverNotReached(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), domain.ProviderAccount{
		ID:     555,
		Status: codemeta.InvoiceProviderAccountStatusActive.Value,
		// cutover 未設定 ＝ 尚未切過去。
	})
	got := h.previewOf(t, nil)
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModeLegacyManual.Value,
		codemeta.InvoiceDocumentKindB2c.Value,
		codemeta.InvoiceNotApplicableReasonCutoverNotReached.Value)
}

func TestResolveMode_LeafB2C_AmegoAuto(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	got := h.previewOf(t, nil)
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		codemeta.InvoiceDocumentKindB2c.Value, "")
	if got.Disclosure.Required {
		t.Error("AMEGO_AUTO 不需要法定告知；required=true 會讓每一筆正常結帳都多一個必勾框")
	}
}

// TestResolveMode_LeafB2B_AmegoAuto 走 E 節點的「是」分支。
//
// preview 沒有買家輸入可讀（那是 confirm 的事），所以 B2B 的來源只有一個：
// 買家的預設常用發票資料是 TAX_ID 型。這也是 §6.1.2(a) 的 defaultProfile 的用途。
func TestResolveMode_LeafB2B_AmegoAuto(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	h.prefs.def, h.prefs.defFound = taxIDPreference(), true

	got := h.previewOf(t, nil)
	assertLeaf(t, got,
		codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		codemeta.InvoiceDocumentKindB2b.Value, "")
	if got.DefaultProfile.BuyerTaxID != "12345678" {
		t.Errorf("defaultProfile.buyerTaxId = %q, want 12345678", got.DefaultProfile.BuyerTaxID)
	}
}

// AM-a／AM-b／AM-c：fail-closed 三格。
//
// 這三葉在 §7.1.3 上的差異是 document.status（AWAITING_MANUAL）與 manual_reason_code，
// **fulfillment_mode 仍是 AMEGO_AUTO**。preview 不回那兩個欄位，所以這三支測試驗的是
// 那個「不變」：preview 說 AMEGO_AUTO 而實際落 AWAITING_MANUAL，**不是矛盾**，
// 而 preview 若因為這三格改口說成別的模式，前端就會顯示錯的法定告知。
//
// 三支各自獨立，因為它們是三個獨立的偵測（平台出資券／和單／混合稅別），
// 合成一支的話刪掉其中一個偵測不會轉紅。

func TestResolveMode_LeafAMa_PlatformFundedDiscountKeepsAmegoAuto(t *testing.T) {
	assertPreviewMatchesPrepare(t, approvedProfile(1), activeAccount(),
		func(in *OrderPaidInput) { in.HasPlatformFundedDiscount = true })
}

func TestResolveMode_LeafAMb_ConsolidatedOrderKeepsAmegoAuto(t *testing.T) {
	assertPreviewMatchesPrepare(t, approvedProfile(1), activeAccount(),
		func(in *OrderPaidInput) { in.IsConsolidatedOrder = true })
}

func TestResolveMode_LeafAMc_MixedTaxCategoryKeepsAmegoAuto(t *testing.T) {
	// 免稅賣家 ＋ 應稅運費 ⇒ 混合稅別（運費行恆為應稅，§7.1.3 FC 表）。
	exempt := approvedProfile(int16(domain.TaxCategoryExempt))
	assertPreviewMatchesPrepare(t, exempt, activeAccount(),
		func(in *OrderPaidInput) {
			in.Snapshot.ShippingFee = 60
			in.Snapshot.TotalActualAmount = 160
		})
}

func taxIDPreference() domain.BuyerPreference {
	taxID, title, mask := "12345678", "測試買方股份有限公司", "1234****"
	return domain.BuyerPreference{
		ID:               8801,
		BuyerID:          2623900520,
		PreferenceKind:   codemeta.InvoiceCarrierTypeTaxId.Value,
		TaxID:            &taxID,
		TaxTitle:         &title,
		CarrierValueMask: &mask,
		IsDefault:        true,
		CreatedAt:        testNow,
		UpdatedAt:        testNow,
	}
}

// ---------------------------------------------------------------------------
// preview 的三個對外欄位
// ---------------------------------------------------------------------------

func TestResolveMode_PreviewExpiresAtComesFromBackend(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	got := h.previewOf(t, nil)
	want := testNow.Add(checkoutPreviewTTL)
	if !got.PreviewExpiresAt.Equal(want) {
		t.Errorf("previewExpiresAt = %v, want %v（前端不得自算 TTL，§6.1.2(a)）", got.PreviewExpiresAt, want)
	}
}

func TestResolveMode_AmountBreakdownIsClampedAtZero(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	got := h.previewOf(t, func(in *ResolveModeInput) {
		in.ProductAmount = 100
		in.ShippingAmount = 60
		in.DiscountAmount = 500 // 折抵超過總額
	})
	if got.Amount.InvoiceAmount != 0 {
		t.Errorf("invoiceAmount = %d, want 0（與 OrderSnapshot.TotalActualAmount 的 max(0,…) 同式）",
			got.Amount.InvoiceAmount)
	}
	if got.InvoiceMode != codemeta.InvoiceFulfillmentModeLegacyManual.Value ||
		got.NotApplicableReason != codemeta.InvoiceNotApplicableReasonZeroAmount.Value {
		t.Errorf("全額折抵應落 NA1，實得 mode=%q reason=%q", got.InvoiceMode, got.NotApplicableReason)
	}
}

// TestResolveMode_IntakeLimitOnlyDuringPendingWindow 守 §6.1.2(a) 的
// 「`intakeLimit` 非待審期間為 null」。
//
// 陰性面才是保證本身：已核定的賣家若回一組全 0 的上限，前端會顯示「已達上限」。
func TestResolveMode_IntakeLimitOnlyDuringPendingWindow(t *testing.T) {
	approved := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	if approved.previewOf(t, nil).IntakeLimit.Applicable {
		t.Error("已核定賣家不在待審期間，intakeLimit 必須為 null")
	}

	pending := newCheckoutHarness(t, domain.SellerTaxProfile{
		ReviewStatus:            codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
		DefaultTaxCategory:      1,
		PendingOrderLimitCount:  50,
		PendingOrderLimitAmount: 100000,
		PendingOrderCount:       3,
		PendingOrderAmount:      4200,
	}, activeAccount())
	got := pending.previewOf(t, nil).IntakeLimit
	if !got.Applicable {
		t.Fatal("SELF_ISSUE_PENDING 在待審期間，intakeLimit 必須有值")
	}
	if got.Blocked {
		t.Error("3/50 筆、4200/100000 元，不該 blocked")
	}
	if got.OrderCountLimit != 50 || got.AmountLimit != 100000 {
		t.Errorf("上限必須讀該賣家那一列，實得 count=%d amount=%d", got.OrderCountLimit, got.AmountLimit)
	}
}

// TestResolveMode_IntakeLimitIsOrNotAnd 守 §6.1.2 C5 第 2 條：兩個上限是 OR。
//
// 寫成 AND 的話，筆數爆掉但金額還沒爆的賣家會繼續接單 —— 而那正是上限要防的情況。
func TestResolveMode_IntakeLimitIsOrNotAnd(t *testing.T) {
	h := newCheckoutHarness(t, domain.SellerTaxProfile{
		ReviewStatus:            codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
		DefaultTaxCategory:      1,
		PendingOrderLimitCount:  50,
		PendingOrderLimitAmount: 100000,
		PendingOrderCount:       50, // 筆數到頂
		PendingOrderAmount:      10, // 金額還很空
	}, activeAccount())
	if !h.previewOf(t, nil).IntakeLimit.Blocked {
		t.Error("筆數已達上限即應 blocked（兩個上限是 OR 不是 AND）")
	}
}

// TestResolveMode_BlockedStillReturnsOK 守 §6.1.2(a)：blocked=true 時 preview 仍回 code 0
// （「是狀態不是錯誤」），閘門在 confirm。
func TestResolveMode_BlockedStillReturnsOK(t *testing.T) {
	h := newCheckoutHarness(t, blockedPendingProfile(), activeAccount())
	out, err := h.uc.ResolveMode(context.Background(), ResolveModeInput{
		StoreID: 2685914682, BuyerID: 2623900520, ProductAmount: 100,
	})
	if err != nil {
		t.Fatalf("blocked 不是錯誤，preview 必須成功回應：%v", err)
	}
	if !out.IntakeLimit.Blocked {
		t.Error("intakeLimit.blocked 應為 true")
	}
	if out.InvoiceMode == "" {
		t.Error("blocked 時仍要回可判定的 invoice_mode，否則 B01 會進 invalid 而阻擋付款")
	}
}

func TestResolveMode_NoDefaultProfileIsNotAnError(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	h.prefs.defFound = false
	got := h.previewOf(t, nil)
	if got.DefaultProfile.Found {
		t.Error("沒有預設常用發票資料時不得合成任何列（§6.1.1 定版）")
	}
}

func blockedPendingProfile() domain.SellerTaxProfile {
	return domain.SellerTaxProfile{
		ReviewStatus:            codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
		DefaultTaxCategory:      1,
		PendingOrderLimitCount:  50,
		PendingOrderLimitAmount: 100000,
		PendingOrderCount:       50,
		PendingOrderAmount:      100000,
	}
}

// ---------------------------------------------------------------------------
// confirm：六個失敗分支 ＋ 快照
// ---------------------------------------------------------------------------

func baseValidateInput() ValidateInvoiceInput {
	return ValidateInvoiceInput{
		StoreID:         2685914682,
		BuyerID:         2623900520,
		OrderNo:         "O000123456",
		Amount:          CheckoutAmount{ProductAmount: 100},
		SnapshotVersion: 1,
		IdempotencyKey:  "IK-0001",
	}
}

func (h *checkoutHarness) validate(t *testing.T, mutate func(*ValidateInvoiceInput)) ValidateInvoiceOutput {
	t.Helper()
	in := baseValidateInput()
	if mutate != nil {
		mutate(&in)
	}
	out, err := h.uc.ValidateInput(context.Background(), in)
	if err != nil {
		t.Fatalf("ValidateInput 失敗：%v", err)
	}
	return out
}

// assertRejected 同時斷言三件事：不放行、碼對、**data 不是空的**。
//
// 第三項是本檔最容易被改壞的一條：把 data 拿掉之後所有「回 code≠0」的測試照樣綠，
// 而 9326 的 `data.intakeLimit`（§6.1.2 明文要求）就永遠到不了前端。
func assertRejected(t *testing.T, out ValidateInvoiceOutput, wantCode int32) {
	t.Helper()
	if out.Valid {
		t.Errorf("valid = true，want false（code=%d）", wantCode)
	}
	if out.BizCode != wantCode {
		t.Errorf("code = %d, want %d", out.BizCode, wantCode)
	}
	if out.Message == "" {
		t.Error("非 0 業務碼必須帶訊息")
	}
	if out.InvoiceMode == "" {
		t.Error("拒絕時仍必須帶 invoice_mode（前端據此決定顯不顯示法定告知）")
	}
	if out.SnapshotJSON != nil {
		t.Error("拒絕時不得回快照：那份快照會被 order-service 存進 order_invoice_snapshots")
	}
}

func TestValidateInput_OmittedInvoiceBlockIsAccepted(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, nil) // Present=false ＝ 整包省略
	if !out.Valid {
		t.Fatalf("整包省略 invoice 必須放行（已上架 App 直接呼叫此端點），實得 code=%d", out.BizCode)
	}
	if got := snapshotField(t, out, "carrier_selection_mode"); got != codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value {
		t.Errorf("carrier_selection_mode = %q, want USE_BUYER_DEFAULT", got)
	}
	if got := snapshotField(t, out, "invoice_kind"); got != codemeta.InvoiceDocumentKindB2c.Value {
		t.Errorf("invoice_kind = %q, want B2C", got)
	}
}

// 法定告知**不再擋門**（DEC-20260831-210934：只顯示不強制勾選）。
//
// 本測試取代了三支舊測試（DisclosureRequiredWhenNotAmegoAuto／DisclosureVersionMismatch／
// DisclosureAcceptedPasses），它們斷言的是「沒勾就回 9314」。**刪掉不補會讓新保證
// 無人看守**：迴歸成擋門不會有任何測試變紅，而症狀是免用統一發票賣家的結帳全部 500 不了、
// 是靜默地全部被 409 擋住。
//
// 🔴 這一格守的是「**不擋**」，所以它必須用最容易被擋的那個情境：
// 賣家已核定免用統一發票（disclosureFor 對它回 Required=true），且請求完全不帶任何
// 告知欄位（那兩欄已從 proto 移除、欄號 reserved）。
func TestValidateInput_DisclosureNeverBlocks(t *testing.T) {
	h := newCheckoutHarness(t, domain.SellerTaxProfile{
		ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
		DefaultTaxCategory: 1,
	}, activeAccount())

	out := h.validate(t, nil)
	if !out.Valid {
		t.Fatalf("告知不得擋住結帳，實得 code=%d message=%q", out.BizCode, out.Message)
	}
	if out.BizCode == bizcode.InvoiceDisclosureNotAccepted {
		t.Fatalf("9314 應為零 emitter（DEC-20260831-210934），卻仍被丟出")
	}
}

// disclosureFor 的職責沒有消失：它決定**要不要顯示、顯示哪一段文案**。
//
// 拿掉擋門之後這支特別容易被順手刪掉，而刪了的症狀是免用統一發票的買家完全不知道
// 自己拿到的是收據——那正是這次裁決想保住的唯一價值。
func TestDisclosureStillTellsFrontendWhatToShow(t *testing.T) {
	exempt := disclosureFor(codemeta.InvoiceFulfillmentModePlatformReceipt.Value)
	if !exempt.Required {
		t.Error("免用統一發票必須告訴前端要顯示")
	}
	if exempt.Title == "" || exempt.Body == "" {
		t.Error("要顯示卻沒有文案 ⇒ 前端只能寫死，那會繞過版本控管")
	}

	auto := disclosureFor(codemeta.InvoiceFulfillmentModeAmegoAuto.Value)
	if auto.Required {
		t.Error("平台全自動代開不需要顯示——買家本來就會拿到電子發票")
	}
}

// TestValidateInput_DisclosureNotRequiredForAmegoAuto 是 9314 的陰性面：
// 一般賣家不該被要求勾一個不存在的告知框。
func TestValidateInput_DisclosureNotRequiredForAmegoAuto(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	if out := h.validate(t, nil); !out.Valid {
		t.Fatalf("AMEGO_AUTO 不需要法定告知，實得 code=%d", out.BizCode)
	}
}

// TestValidateInput_ProfileOwnershipIsInThePredicate 守 §8.1.4(b)：
// buyer_id 必須進到查詢述詞，不是查回來再比對。
func TestValidateInput_ProfileOwnershipIsInThePredicate(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	h.prefs.err = notFoundErr(bizcode.InvoiceProfileNotFound)

	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Invoice = CheckoutInvoiceSpec{Present: true, ProfileID: 8801}
	})
	assertRejected(t, out, bizcode.InvoiceProfileNotFound)

	if h.prefs.gotByID != [2]int64{8801, 2623900520} {
		t.Errorf("GetByID 收到 %v，want [8801 2623900520]；buyer_id 沒進述詞 ＝ 任何買家都讀得到別人的載具",
			h.prefs.gotByID)
	}
}

// TestValidateInput_MissingBuyerIDDoesNotSkipOwnership：沒有買家鍵時**不得退化成不驗**（I-47）。
func TestValidateInput_MissingBuyerIDDoesNotSkipOwnership(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.BuyerID = 0
		in.Invoice = CheckoutInvoiceSpec{Present: true, ProfileID: 8801}
	})
	assertRejected(t, out, bizcode.InvoiceProfileNotFound)
	if h.prefs.gotByID != [2]int64{} {
		t.Error("沒有 buyer_id 時不該去查 —— 查了就代表述詞裡可以沒有擁有者")
	}
}

func TestValidateInput_CarrierFormatRejected(t *testing.T) {
	cases := []struct {
		name string
		spec CheckoutInvoiceSpec
	}{
		{"手機條碼少一碼", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value, Value: "/ABC123"}},
		{"手機條碼沒有斜線", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value, Value: "ABC1234"}},
		{"自然人憑證長度不對", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeCitizenCert.Value, Value: "AB1234"}},
		{"捐贈碼含非數字", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeDonation.Value, Value: "12A45"}},
		{"捐贈碼超過 7 碼", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeDonation.Value, Value: "12345678"}},
		{"統編不是 8 碼", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value,
			BuyerTaxID: "1234567", BuyerName: "測試公司"}},
		{"統編缺抬頭", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value, BuyerTaxID: "12345678"}},
		{"TAX_ID 不得帶 value", CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value,
			BuyerTaxID: "12345678", BuyerName: "測試公司", Value: "/ABC1239"}},
		{"值域外的 carrierType", CheckoutInvoiceSpec{
			Present: true, CarrierType: "NO_CARRIER", Value: ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
			out := h.validate(t, func(in *ValidateInvoiceInput) { in.Invoice = c.spec })
			assertRejected(t, out, bizcode.InvoiceCarrierInvalid)
		})
	}
}

func TestValidateInput_ValidDonationPasses(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Invoice = CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeDonation.Value, Value: "25885",
		}
	})
	if !out.Valid {
		t.Fatalf("合法捐贈碼必須放行，實得 code=%d message=%q", out.BizCode, out.Message)
	}
	if got := snapshotField(t, out, "carrier_selection_mode"); got != codemeta.InvoiceCarrierSelectionModeDonation.Value {
		t.Errorf("carrier_selection_mode = %q, want DONATION", got)
	}
	if got := snapshotField(t, out, "donation_code"); got != "25885" {
		t.Errorf("donation_code = %q, want 25885", got)
	}
}

func TestValidateInput_ValidTaxIDPasses(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Invoice = CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value,
			BuyerTaxID: "12345678", BuyerName: "測試買方股份有限公司",
		}
	})
	if !out.Valid {
		t.Fatalf("合法統編必須放行，實得 code=%d message=%q", out.BizCode, out.Message)
	}
	if got := snapshotField(t, out, "invoice_kind"); got != codemeta.InvoiceDocumentKindB2b.Value {
		t.Errorf("invoice_kind = %q, want B2B", got)
	}
	if got := snapshotField(t, out, "carrier_selection_mode"); got != codemeta.InvoiceCarrierSelectionModeNone.Value {
		t.Errorf("carrier_selection_mode = %q, want NONE（chk_ois_b2b 要求 B2B 恆為 NONE）", got)
	}
}

// TestValidateInput_TargetConflictGoesThroughA15 是 9306 —— 三選一的唯一攔截點。
//
// AMEGO 對這三種組合實測全部通過（§6.1.1），所以這裡漏掉一種，就會開出一張
// 買受人身分自相矛盾的憑證，而 provider 不會有任何抱怨。
func TestValidateInput_TargetConflictGoesThroughA15(t *testing.T) {
	cases := []struct {
		name string
		pref domain.BuyerPreference
	}{
		{"統編 ＋ 手機條碼", conflictPreference(true, "/ABC1239", "")},
		{"統編 ＋ 捐贈碼", conflictPreference(true, "", "25885")},
		{"手機條碼 ＋ 捐贈碼", conflictPreference(false, "/ABC1239", "25885")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
			h.prefs.byID = c.pref
			out := h.validate(t, func(in *ValidateInvoiceInput) {
				in.Invoice = CheckoutInvoiceSpec{Present: true, ProfileID: 8801}
			})
			assertRejected(t, out, bizcode.InvoiceTargetConflict)
		})
	}
}

// conflictPreference 造一列刻意違反三選一的常用發票資料。
//
// 這種列在 DB 上被 chk_bip_shape 擋著，但它模擬的是「規則收緊之前的存量列」與
// 「有人繞過端點 2 直接寫庫」——結帳是最後一道，不能假設上游一定乾淨。
func conflictPreference(withTaxID bool, carrierValue, donation string) domain.BuyerPreference {
	p := domain.BuyerPreference{ID: 8801, BuyerID: 2623900520, CreatedAt: testNow, UpdatedAt: testNow}
	switch {
	case carrierValue != "":
		mask := "1234"
		p.PreferenceKind = codemeta.InvoiceCarrierTypeMobileBarcode.Value
		p.CarrierValueMask = &mask
	default:
		p.PreferenceKind = codemeta.InvoiceCarrierTypeDonation.Value
	}
	if withTaxID {
		taxID, title := "12345678", "測試買方股份有限公司"
		p.TaxID, p.TaxTitle = &taxID, &title
	}
	if donation != "" {
		p.DonationCode = &donation
	}
	return p
}

// TestValidateInput_IntakeLimitReached 是 9326：回 code≠0 但 **data.intakeLimit 必須有值**。
func TestValidateInput_IntakeLimitReached(t *testing.T) {
	h := newCheckoutHarness(t, blockedPendingProfile(), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Invoice = CheckoutInvoiceSpec{
			Present: true,
		}
	})
	assertRejected(t, out, bizcode.SellerInvoiceIntakeLimitReached)
	if !out.IntakeLimit.Applicable || !out.IntakeLimit.Blocked {
		t.Fatal("9326 必須帶 intakeLimit 現況（§6.1.2：HTTP 200 ＋ preserveData）")
	}
	if out.IntakeLimit.OrderCountLimit != 50 {
		t.Errorf("intakeLimit.orderCountLimit = %d, want 50", out.IntakeLimit.OrderCountLimit)
	}
}

// TestValidateInput_InvoiceAmountIsRecomputedNotTrusted：呼叫端送來的 invoice_amount
// 不採信，一律由 product/shipping/discount 重算。
//
// 採信它 ＝ 讓上游決定要開多少錢的發票，而發票金額是稅。
func TestValidateInput_InvoiceAmountIsRecomputedNotTrusted(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Amount = CheckoutAmount{
			ProductAmount: 100, ShippingAmount: 60, DiscountAmount: 10,
			InvoiceAmount: 999999, // 惡意／錯誤的總額
			TaxCategory:   9,      // 值域外
		}
	})
	if !out.Valid {
		t.Fatalf("金額本身合法，應放行，實得 code=%d", out.BizCode)
	}
	// 重算後是 150；若採信 request，A1 會在 forecastDraft 上炸成一行 WARN 而不是被修正。
	// 直接驗 usecase 的重算結果。
	got := buildCheckoutAmount(100, 60, 10, approvedProfile(1))
	if got.InvoiceAmount != 150 {
		t.Fatalf("重算的 invoiceAmount = %d, want 150", got.InvoiceAmount)
	}
	if got.TaxCategory != 1 {
		t.Errorf("taxCategory 必須取賣家的 default_tax_category，實得 %d", got.TaxCategory)
	}
}

// TestValidateInput_SnapshotCarriesNoPII 守 §5.4.3 表註解「零 PII，密文一律留在 invoice-service」。
//
// 判準是**欄位集合**而不是「有沒有看到某個值」：上游長胖時新欄位會靜默流出去，
// 而只斷言「沒有 carrier_value」的測試對新欄位全盲。
func TestValidateInput_SnapshotCarriesNoPII(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Invoice = CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value,
			BuyerTaxID: "12345678", BuyerName: "測試買方股份有限公司",
			Email: "buyer@example.com",
		}
	})
	if !out.Valid {
		t.Fatalf("應放行，實得 code=%d", out.BizCode)
	}

	want := map[string]bool{
		"order_no": true, "snapshot_version": true, "invoice_kind": true,
		"carrier_selection_mode": true, "buyer_invoice_preference_id": true,
		"buyer_tax_id": true, "buyer_title": true, "donation_code": true,
	}
	got := snapshotKeys(t, out)
	for k := range got {
		if !want[k] {
			t.Errorf("快照多了欄位 %q —— order_invoice_snapshots 沒有這一欄，且該表是零 PII", k)
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("快照少了欄位 %q", k)
		}
	}
}

// TestValidateInput_SnapshotVersionIsNeverInvented 守 I-50：本服務不遞增 snapshot_version。
func TestValidateInput_SnapshotVersionIsNeverInvented(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) { in.SnapshotVersion = 7 })
	if got := snapshotField(t, out, "snapshot_version"); got != "7" {
		t.Errorf("snapshot_version = %v, want 7（原樣回聲，不遞增）", got)
	}
}

// TestValidateInput_ExplicitPreferenceSnapshot 走「指定既有常用發票資料」的完整路徑。
//
// 這是唯一會在快照裡填 buyer_invoice_preference_id 的分支（chk_ois_pref_pair 是雙向等式），
// 也是唯一需要 resolved_display_value 是遮罩值的分支 —— 載具明碼不出服務邊界。
func TestValidateInput_ExplicitPreferenceSnapshot(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	mask := "/ABC***9"
	h.prefs.byID = domain.BuyerPreference{
		ID:               8801,
		BuyerID:          2623900520,
		PreferenceKind:   codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		CarrierValueMask: &mask,
		IsDefault:        true,
		CreatedAt:        testNow,
		UpdatedAt:        testNow,
	}

	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Invoice = CheckoutInvoiceSpec{Present: true, ProfileID: 8801}
	})
	if !out.Valid {
		t.Fatalf("應放行，實得 code=%d message=%q", out.BizCode, out.Message)
	}
	if got := snapshotField(t, out, "carrier_selection_mode"); got != codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value {
		t.Errorf("carrier_selection_mode = %q, want EXPLICIT_PREFERENCE", got)
	}
	if got := snapshotField(t, out, "buyer_invoice_preference_id"); got != "8801" {
		t.Errorf("buyer_invoice_preference_id = %q, want 8801", got)
	}
	if out.ResolvedDisplayValue != mask {
		t.Errorf("resolved_display_value = %q, want %q（遮罩值，不得是明碼）", out.ResolvedDisplayValue, mask)
	}
	if out.ResolvedCarrierType != codemeta.InvoiceCarrierTypeMobileBarcode.Value {
		t.Errorf("resolved_carrier_type = %q", out.ResolvedCarrierType)
	}
}

// TestValidateInput_UnsavedOneOffCarrierIsRejected 記錄一個**設計書矛盾下的取捨**：
// §6.1.2(b) 允許結帳時手動輸入載具，而 §5.4.3 的快照沒有任何欄位或模式承載它
// （carrier_selection_mode 只有 4 值、載具值一律留在 invoice-service）。
//
// 本波取 fail-closed 的那一邊：擋在結帳（買家看得到、改得掉、還沒付錢），
// 而不是靜默開成無載具（發票不可回收、載具歸戶落空）。
func TestValidateInput_UnsavedOneOffCarrierIsRejected(t *testing.T) {
	h := newCheckoutHarness(t, approvedProfile(1), activeAccount())
	out := h.validate(t, func(in *ValidateInvoiceInput) {
		in.Invoice = CheckoutInvoiceSpec{
			Present: true, CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value, Value: "/ABC1239",
		}
	})
	assertRejected(t, out, bizcode.InvoiceCarrierInvalid)
	if out.Message != errUnsavedOneOffCarrier {
		t.Errorf("message = %q, want %q（與「格式錯」同碼，訊息必須分得開）", out.Message, errUnsavedOneOffCarrier)
	}
}
