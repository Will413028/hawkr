package domain

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// FulfillmentInput 是 §7.1.3 決策樹的全部輸入。這棵樹只在 order.paid 消費當下跑一次。
type FulfillmentInput struct {
	TotalActualAmount int64

	// SellerReviewStatus 是 seller_tax_profiles.review_status（CodeMeta SellerTaxReviewStatus）。
	SellerReviewStatus string
	// ProviderStatus 是 invoice_provider_accounts.status（CodeMeta InvoiceProviderAccountStatus）。
	ProviderStatus string

	// AutomationCutoverAt 為 nil 表示尚未設定切點。
	AutomationCutoverAt *time.Time
	// InvoiceDate 是 COALESCE(provider_paid_at, pay_time)，UTC。
	InvoiceDate time.Time

	// fail-closed 三格（三格都必須在建立 document 的同一事務內判定並落 manual_reason_code，
	// 不是先建成 PENDING 再靠掃描器改過去）。
	HasPlatformFundedDiscount bool
	IsConsolidatedOrder       bool
	HasMixedTaxCategory       bool

	// BuyerHasTaxID 表示買家快照帶有效統編。
	BuyerHasTaxID bool

	// SellerReceiptAuthorized ＝ seller_tax_profiles.receipt_authorized_at 非空
	// （S05，§6.1.9）。只對 APPROVED_INVOICE_EXEMPT 這一格有意義。
	//
	// 🔴 它是 fail-closed 的輸入不是選配：收據的**出據人是賣家**（Q-B7 的那半句
	// 「收據以賣家為出據人、不以平台名義開立」），平台拿他的法定名稱、統編與營業地址
	// 產出一份對外文件，靠的就是這個授權。沒有它而照樣產出＝以賣家名義出具他沒同意的文件。
	SellerReceiptAuthorized bool
}

// FulfillmentDecision 是決策樹的葉節點結果，逐欄對應 invoice_documents 的五個欄位。
type FulfillmentDecision struct {
	FulfillmentMode     string // CodeMeta InvoiceFulfillmentMode
	DocumentKind        string // CodeMeta InvoiceDocumentKind
	DocumentStatus      string // CodeMeta InvoiceDocumentStatus
	NotApplicableReason string // CodeMeta InvoiceNotApplicableReason；空字串 = 不適用
	ManualReasonCode    string // CodeMeta InvoiceManualReason；空字串 = 不適用
}

// ResolveFulfillmentMode 走完 §7.1.3 的決策樹。
//
// 模式判定是雙條件（賣家資格 ＋ provider 狀態），不能只比 paid_at vs cutover——只比時間會在
// 連線被 SUSPENDED／REVOKED 之後仍判成 AMEGO_AUTO，每一筆都開不出來。
//
// SELLER_SELF_ISSUE 與 LEGACY_MANUAL 仍要建 invoice_documents 列（否則買家訂單詳情沒有東西
// 可顯示、沒有申訴對象、每日對帳把它們全列成異常），建了之後 document 恆為 NOT_APPLICABLE。
func ResolveFulfillmentMode(in FulfillmentInput) FulfillmentDecision {
	kind := documentKindOf(in.BuyerHasTaxID)

	if in.TotalActualAmount <= 0 {
		return FulfillmentDecision{
			FulfillmentMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			DocumentKind:        kind,
			DocumentStatus:      codemeta.InvoiceDocumentStatusNotApplicable.Value,
			NotApplicableReason: codemeta.InvoiceNotApplicableReasonZeroAmount.Value,
		}
	}

	switch in.SellerReviewStatus {
	case codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value:
		// 免用統一發票賣家：這條線是**收據**不是發票，本地產生收據 PDF、不打 AMEGO
		// （§7.1.2 S-6：「INV.op 不打 AMEGO，改走本地 PDF 產生」）。
		//
		// 🔴 授權是硬前置，不是「有更好、沒有也行」（#3026，2026-09-02）：
		// 收據的出據人是**賣家**（Q-B7），平台是代製方。沒有 S05 授權就產出一份
		// 印著他的法定名稱、統編與營業地址的對外文件，等於以他的名義出具他沒同意的東西。
		// ⇒ 未授權時落 AWAITING_MANUAL ＋ RECEIPT_AUTH_MISSING，讓 ADM 去催。
		//
		// ⚠️ **不落 NOT_APPLICABLE**：那是終態，這筆訂單會再也沒有人回頭補收據
		// ——那正是 #3022→#3026 那一輪的教訓（誠實地說「沒開通」，然後買家永遠沒有憑證）。
		//
		// 歷史：2026-09-02 之前這一格因收據產製未落地而落 NOT_APPLICABLE ＋
		// RECEIPT_NOT_AVAILABLE（#3022 的短期修法）。收據產製上線後那個理由碼成為死碼，
		// 已依 §8.5 從碼表移除。
		if !in.SellerReceiptAuthorized {
			return FulfillmentDecision{
				FulfillmentMode:  codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
				DocumentKind:     codemeta.InvoiceDocumentKindReceipt.Value,
				DocumentStatus:   codemeta.InvoiceDocumentStatusAwaitingManual.Value,
				ManualReasonCode: codemeta.InvoiceManualReasonReceiptAuthMissing.Value,
			}
		}
		return FulfillmentDecision{
			FulfillmentMode: codemeta.InvoiceFulfillmentModePlatformReceipt.Value,
			DocumentKind:    codemeta.InvoiceDocumentKindReceipt.Value,
			DocumentStatus:  codemeta.InvoiceDocumentStatusPending.Value,
		}
	case codemeta.SellerTaxReviewStatusSelfIssuePending.Value:
		return FulfillmentDecision{
			FulfillmentMode:     codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value,
			DocumentKind:        kind,
			DocumentStatus:      codemeta.InvoiceDocumentStatusNotApplicable.Value,
			NotApplicableReason: codemeta.InvoiceNotApplicableReasonSelfIssue.Value,
		}
	case codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value:
		// 落到下面的 provider／切點判定。
	default:
		// DRAFT / SUBMITTED / UNDER_REVIEW / SUPPLEMENT_REQUIRED / REJECTED / EXPIRED / REVOKED
		return FulfillmentDecision{
			FulfillmentMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			DocumentKind:        kind,
			DocumentStatus:      codemeta.InvoiceDocumentStatusNotApplicable.Value,
			NotApplicableReason: codemeta.InvoiceNotApplicableReasonTaxProfileNotApproved.Value,
		}
	}

	if !providerUsable(in.ProviderStatus) {
		return FulfillmentDecision{
			FulfillmentMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			DocumentKind:        kind,
			DocumentStatus:      codemeta.InvoiceDocumentStatusNotApplicable.Value,
			NotApplicableReason: codemeta.InvoiceNotApplicableReasonProviderNotActive.Value,
		}
	}

	if in.AutomationCutoverAt == nil || in.InvoiceDate.Before(*in.AutomationCutoverAt) {
		return FulfillmentDecision{
			FulfillmentMode:     codemeta.InvoiceFulfillmentModeLegacyManual.Value,
			DocumentKind:        kind,
			DocumentStatus:      codemeta.InvoiceDocumentStatusNotApplicable.Value,
			NotApplicableReason: codemeta.InvoiceNotApplicableReasonCutoverNotReached.Value,
		}
	}

	// fail-closed 三格。順序即優先序：平台出資券最先（它是稅務錯誤而不只是流程例外）。
	if reason := failClosedReason(in); reason != "" {
		return FulfillmentDecision{
			FulfillmentMode:  codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
			DocumentKind:     kind,
			DocumentStatus:   codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			ManualReasonCode: reason,
		}
	}

	return FulfillmentDecision{
		FulfillmentMode: codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		DocumentKind:    kind,
		DocumentStatus:  codemeta.InvoiceDocumentStatusPending.Value,
	}
}

// SelfIssueEffective 回答「這位賣家的**自行開票責任是否生效中**」——即「審核期間的
// 發票由賣家自行開立與交付」這個法律狀態現在成不成立。
//
// 🔴 **錨在 review_status，不是 attested_at。** 兩者在裁決 C（2026-08-31）之前等價
// （`attested_at` 只由 T04 寫，而 T04 同時把狀態推到 SELF_ISSUE_PENDING），裁決 C 把
// 簽署寫入點前移到 T02（端點 19 的送審同一句 UPDATE）之後就斷了：`SUBMITTED` 的列
// 同樣已簽，但它在上面的決策樹裡落 LEGACY_MANUAL／TAX_PROFILE_NOT_APPROVED、
// `intakeLimitOf` 也不認它 ⇒ 用 `attested_at != nil` 點亮「自行開票責任生效中」
// 等於對賣家做出與後端行為相反的法律告知。
//
// 值域取自本檔 ResolveFulfillmentMode 的 SELF_ISSUE_PENDING 分支——那是全服務**唯一**
// 判出 SELLER_SELF_ISSUE 的入口，改這裡就要改那裡（反之亦然）。⚠️ **不含
// SUPPLEMENT_REQUIRED**：它雖然同樣適用待審接單上限（`intakeLimitOf`），但發票模式
// 是 LEGACY_MANUAL——「可以接單」與「責任在賣家」是兩件事。
//
// 「條款簽過沒」是另一個問題，答案仍是 `AttestedAt != nil`（ADM 佇列的
// `selfIssueAccepted`／端點 21 的 acceptedAt、termsVersion）。
func (p SellerTaxProfile) SelfIssueEffective() bool {
	return p.ReviewStatus == codemeta.SellerTaxReviewStatusSelfIssuePending.Value
}

func documentKindOf(buyerHasTaxID bool) string {
	if buyerHasTaxID {
		return codemeta.InvoiceDocumentKindB2b.Value
	}
	return codemeta.InvoiceDocumentKindB2c.Value
}

// providerUsable 正面列舉可用的 provider 狀態（§7.1.3 的 C 節點）。
// 正面列舉而非 NOT IN：值域日後長胖時，新狀態預設是「不可用」而不是「靜默放行」。
func providerUsable(status string) bool {
	switch status {
	case codemeta.InvoiceProviderAccountStatusActive.Value,
		codemeta.InvoiceProviderAccountStatusDegraded.Value:
		return true
	default:
		return false
	}
}

func failClosedReason(in FulfillmentInput) string {
	switch {
	case in.HasPlatformFundedDiscount:
		return codemeta.InvoiceManualReasonPlatformFundedDiscount.Value
	case in.IsConsolidatedOrder:
		return codemeta.InvoiceManualReasonConsolidatedOrder.Value
	case in.HasMixedTaxCategory:
		return codemeta.InvoiceManualReasonMixedTaxCategory.Value
	default:
		return ""
	}
}
