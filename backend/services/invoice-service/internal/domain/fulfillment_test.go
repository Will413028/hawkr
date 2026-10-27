package domain

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

func cutover(t time.Time) *time.Time { return &t }

func baseInput() FulfillmentInput {
	past := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	return FulfillmentInput{
		TotalActualAmount:   1000,
		SellerReviewStatus:  codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		ProviderStatus:      codemeta.InvoiceProviderAccountStatusActive.Value,
		AutomationCutoverAt: cutover(past),
		InvoiceDate:         time.Date(2026, time.August, 29, 0, 0, 0, 0, time.UTC),
	}
}

func TestResolveFulfillmentMode(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(*FulfillmentInput)
		mode       string
		kind       string
		status     string
		naReason   string
		manualCode string
	}{
		{"零元訂單", func(in *FulfillmentInput) { in.TotalActualAmount = 0 },
			codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusNotApplicable.Value,
			codemeta.InvoiceNotApplicableReasonZeroAmount.Value, ""},
		// #3026：收據產製已落地 ⇒ 已授權的免用發票賣家落 PENDING（進候選集合，
		// 由 INV.op 走本地 PDF 產生）。這一格從 #3022 的 NOT_APPLICABLE 翻回來，
		// 前提是 settleReceipt 真的會接手——否則會直接重現 #3022。
		{"免用統一發票 ＋ 已授權 ⇒ 平台代製收據待產製", func(in *FulfillmentInput) {
			in.SellerReviewStatus = codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value
			in.SellerReceiptAuthorized = true
		},
			codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
			codemeta.InvoiceDocumentKindReceipt.Value,
			codemeta.InvoiceDocumentStatusPending.Value,
			"", ""},
		// 🔴 陰性對照：**未授權**不得落 PENDING。收據的出據人是賣家（Q-B7），
		// 平台沒有 S05 授權就不得以他的名義產出文件 ⇒ AWAITING_MANUAL ＋ 理由碼。
		// 也**不得**落 NOT_APPLICABLE：那是終態，這筆訂單會再也沒有人回頭補收據。
		{"免用統一發票 ＋ 未授權 ⇒ 轉人工催授權", func(in *FulfillmentInput) {
			in.SellerReviewStatus = codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value
			in.SellerReceiptAuthorized = false
		},
			codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
			codemeta.InvoiceDocumentKindReceipt.Value,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			"", codemeta.InvoiceManualReasonReceiptAuthMissing.Value},
		{"具結待審 ⇒ 賣家自行開立", func(in *FulfillmentInput) {
			in.SellerReviewStatus = codemeta.SellerTaxReviewStatusSelfIssuePending.Value
		},
			codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusNotApplicable.Value,
			codemeta.InvoiceNotApplicableReasonSelfIssue.Value, ""},
		{"資格未核准 ⇒ LEGACY_MANUAL", func(in *FulfillmentInput) {
			in.SellerReviewStatus = codemeta.SellerTaxReviewStatusUnderReview.Value
		},
			codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusNotApplicable.Value,
			codemeta.InvoiceNotApplicableReasonTaxProfileNotApproved.Value, ""},
		{"provider 被停權 ⇒ PROVIDER_NOT_ACTIVE", func(in *FulfillmentInput) {
			in.ProviderStatus = codemeta.InvoiceProviderAccountStatusSuspended.Value
		},
			codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusNotApplicable.Value,
			codemeta.InvoiceNotApplicableReasonProviderNotActive.Value, ""},
		{"切點未設定", func(in *FulfillmentInput) { in.AutomationCutoverAt = nil },
			codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusNotApplicable.Value,
			codemeta.InvoiceNotApplicableReasonCutoverNotReached.Value, ""},
		{"發票日早於切點", func(in *FulfillmentInput) {
			in.AutomationCutoverAt = cutover(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC))
		},
			codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusNotApplicable.Value,
			codemeta.InvoiceNotApplicableReasonCutoverNotReached.Value, ""},
		{"平台出資券 ⇒ AWAITING_MANUAL", func(in *FulfillmentInput) { in.HasPlatformFundedDiscount = true },
			codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value, "",
			codemeta.InvoiceManualReasonPlatformFundedDiscount.Value},
		{"和單訂單", func(in *FulfillmentInput) { in.IsConsolidatedOrder = true },
			codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value, "",
			codemeta.InvoiceManualReasonConsolidatedOrder.Value},
		{"混合稅別", func(in *FulfillmentInput) { in.HasMixedTaxCategory = true },
			codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value, "",
			codemeta.InvoiceManualReasonMixedTaxCategory.Value},
		{"B2C happy path", func(*FulfillmentInput) {},
			codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusPending.Value, "", ""},
		{"B2B happy path", func(in *FulfillmentInput) { in.BuyerHasTaxID = true },
			codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			codemeta.InvoiceDocumentKindB2b.Value,
			codemeta.InvoiceDocumentStatusPending.Value, "", ""},
		{"DEGRADED 仍可自動開", func(in *FulfillmentInput) {
			in.ProviderStatus = codemeta.InvoiceProviderAccountStatusDegraded.Value
		},
			codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			codemeta.InvoiceDocumentKindB2c.Value,
			codemeta.InvoiceDocumentStatusPending.Value, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := baseInput()
			c.mutate(&in)
			got := ResolveFulfillmentMode(in)
			want := FulfillmentDecision{
				FulfillmentMode: c.mode, DocumentKind: c.kind, DocumentStatus: c.status,
				NotApplicableReason: c.naReason, ManualReasonCode: c.manualCode,
			}
			if got != want {
				t.Fatalf("got %+v\nwant %+v", got, want)
			}
		})
	}
}

// 模式判定是雙條件：只比 paid_at vs cutover 會在連線被停權之後仍判成 AMEGO_AUTO。
func TestResolveFulfillmentMode_ProviderStatusIsNotOptional(t *testing.T) {
	for _, status := range []string{
		codemeta.InvoiceProviderAccountStatusNotConfigured.Value,
		codemeta.InvoiceProviderAccountStatusCredentialSubmitted.Value,
		codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value,
		codemeta.InvoiceProviderAccountStatusInvalidCredential.Value,
		codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
		codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
		codemeta.InvoiceProviderAccountStatusRotationPending.Value,
		codemeta.InvoiceProviderAccountStatusSuspended.Value,
		codemeta.InvoiceProviderAccountStatusRevoked.Value,
		codemeta.InvoiceProviderAccountStatusNotStarted.Value,
		codemeta.InvoiceProviderAccountStatusConfigError.Value,
	} {
		in := baseInput()
		in.ProviderStatus = status
		if got := ResolveFulfillmentMode(in); got.FulfillmentMode == codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
			t.Errorf("provider status %s must not reach AMEGO_AUTO", status)
		}
	}
}

// 賣家資格值域長胖時，新狀態必須落在 LEGACY_MANUAL（deny-by-default），不得靜默放行。
func TestResolveFulfillmentMode_UnknownReviewStatusIsDenyByDefault(t *testing.T) {
	in := baseInput()
	in.SellerReviewStatus = "SOME_FUTURE_STATUS"
	got := ResolveFulfillmentMode(in)
	if got.FulfillmentMode != codemeta.InvoiceFulfillmentModeLegacyManual.Value ||
		got.NotApplicableReason != codemeta.InvoiceNotApplicableReasonTaxProfileNotApproved.Value {
		t.Fatalf("got %+v", got)
	}
}

// SelfIssueEffective 必須與決策樹對「誰自行開票」的判定逐值一致。
//
// 🔴 這一支是**跨兩個函式的交叉檢查**，不是把 SelfIssueEffective 的實作重寫一遍：
// 它枚舉 SellerTaxReviewStatus 的全部值域，斷言「SelfIssueEffective ⟺ 決策樹判出
// SELLER_SELF_ISSUE」。任何一邊被單獨改動（多一個狀態、少一個狀態）都會轉紅。
//
// 為什麼要守：`SelfIssueEffective` 點亮的是「審核期間發票由你自行開立與交付」這句
// **法律告知**，而真正決定誰開票的是決策樹。兩者一旦分岔，畫面說的與系統做的就相反，
// 而這種分岔不會編譯錯、不會有任何執行期訊號。裁決 C 之前它就是靠 `attested_at != nil`
// 隱含成立的，裁決 C 把寫入點前移之後當場失效——所以這裡改成明寫的不變式。
func TestSelfIssueEffectiveMatchesFulfillmentDecision(t *testing.T) {
	if len(codemeta.AllSellerTaxReviewStatuss) == 0 {
		t.Fatal("值域是空的，這支測試沒有鑑別力")
	}
	seenTrue := false
	for _, st := range codemeta.AllSellerTaxReviewStatuss {
		in := baseInput()
		in.SellerReviewStatus = st.Value
		selfIssuedByTree := ResolveFulfillmentMode(in).FulfillmentMode ==
			codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value
		effective := SellerTaxProfile{ReviewStatus: st.Value}.SelfIssueEffective()
		if effective != selfIssuedByTree {
			t.Fatalf("%s：SelfIssueEffective=%v 但決策樹判 SELLER_SELF_ISSUE=%v —— "+
				"畫面的法律告知與後端行為分岔了", st.Value, effective, selfIssuedByTree)
		}
		seenTrue = seenTrue || effective
	}
	if !seenTrue {
		t.Fatal("整個值域都是 false ⇒ 兩邊一起壞掉也會通過，這支測試沒有鑑別力")
	}
}
