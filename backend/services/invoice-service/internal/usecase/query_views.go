package usecase

import (
	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

var (
	abnormalDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusIssueFailed.Value,
		codemeta.InvoiceDocumentStatusAwaitingManual.Value,
	}

	pendingDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusPending.Value,
		codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusVoiding.Value,
	}

	manualDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusAwaitingManual.Value,
	}

	resolvedDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusVoided.Value,
		codemeta.InvoiceDocumentStatusSuperseded.Value,
		codemeta.InvoiceDocumentStatusCancelledBeforeIssue.Value,
		codemeta.InvoiceDocumentStatusNotApplicable.Value,
		codemeta.InvoiceDocumentStatusResolvedExternally.Value,
	}
)

var sellerComplaintViews = map[string][]string{
	"AWAITING_SELLER": {
		codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusOverdue.Value,
	},
	"SELLER_RESPONDED": {
		codemeta.InvoiceComplaintStatusSellerResponded.Value,
		codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
	},
	"CLOSED": {
		codemeta.InvoiceComplaintStatusResolved.Value,
		codemeta.InvoiceComplaintStatusViolationConfirmed.Value,
		codemeta.InvoiceComplaintStatusWithdrawn.Value,
	},
}

var openComplaintStatuses = []string{
	codemeta.InvoiceComplaintStatusOpen.Value,
	codemeta.InvoiceComplaintStatusOverdue.Value,
	codemeta.InvoiceComplaintStatusSellerResponded.Value,
	codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
}

var overdueComplaintStatuses = []string{
	codemeta.InvoiceComplaintStatusOverdue.Value,
}

type adminComplaintTab struct {
	Statuses          []string
	IncludeDueOverdue bool
	DueOverdue        []string
}

var adminComplaintTabs = map[string]adminComplaintTab{
	"AWAITING_SELLER": {Statuses: []string{
		codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusOverdue.Value,
	}},
	"SELLER_RESPONDED": {Statuses: []string{
		codemeta.InvoiceComplaintStatusSellerResponded.Value,
	}},
	"ADM_REVIEW": {Statuses: []string{
		codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
	}},
	"OVERDUE": {
		Statuses:          []string{codemeta.InvoiceComplaintStatusOverdue.Value},
		IncludeDueOverdue: true,
		DueOverdue: []string{
			codemeta.InvoiceComplaintStatusOpen.Value,
			codemeta.InvoiceComplaintStatusSellerResponded.Value,
			codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
		},
	},
	"CLOSED": {Statuses: []string{
		codemeta.InvoiceComplaintStatusResolved.Value,
		codemeta.InvoiceComplaintStatusViolationConfirmed.Value,
		codemeta.InvoiceComplaintStatusWithdrawn.Value,
	}},
}

var violationComplaintStatus = codemeta.InvoiceComplaintStatusViolationConfirmed.Value

var adminJobTabs = map[string][]string{
	"PENDING": {
		codemeta.InvoiceOperationStatusCreated.Value,
		codemeta.InvoiceOperationStatusAwaitingTrigger.Value,
		codemeta.InvoiceOperationStatusReady.Value,
		codemeta.InvoiceOperationStatusHeld.Value,
	},
	"RETRYING": {
		codemeta.InvoiceOperationStatusRetryScheduled.Value,
		codemeta.InvoiceOperationStatusSending.Value,
	},
	"NEEDS_ATTENTION": {
		codemeta.InvoiceOperationStatusFailedPermanent.Value,
		codemeta.InvoiceOperationStatusFailedRetryExhausted.Value,
		codemeta.InvoiceOperationStatusSellerActionRequired.Value,
	},
	"NEEDS_MANUAL": {
		codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
		codemeta.InvoiceOperationStatusProviderStatusRecheck.Value,
		codemeta.InvoiceOperationStatusFailedUnknown.Value,
		codemeta.InvoiceOperationStatusCancelRequested.Value,
		codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value,
		codemeta.InvoiceOperationStatusCancelledByAdmin.Value,
		codemeta.InvoiceOperationStatusManualProcessAllowed.Value,
	},
	"COMPLETED": {
		codemeta.InvoiceOperationStatusSucceeded.Value,
		codemeta.InvoiceOperationStatusResolvedExternally.Value,
	},
	"CANCELLED": {
		codemeta.InvoiceOperationStatusCancelledBeforeIssue.Value,
		codemeta.InvoiceOperationStatusNotApplicable.Value,
	},
}

const adminJobNeedsManualTab = "NEEDS_MANUAL"

func openJobStatuses() []string {
	out := make([]string, 0, 16)
	for _, tab := range []string{"PENDING", "RETRYING", "NEEDS_ATTENTION", adminJobNeedsManualTab} {
		out = append(out, adminJobTabs[tab]...)
	}
	return out
}

func pendingJobStatuses() []string {
	out := make([]string, 0, 6)
	out = append(out, adminJobTabs["PENDING"]...)
	out = append(out, adminJobTabs["RETRYING"]...)
	return out
}

func needsManualJobStatuses() []string { return adminJobTabs[adminJobNeedsManualTab] }

type uploadStatusProjection struct {
	Codes       []int16
	IncludeNull bool
}

var uploadStatusProjections = map[string]uploadStatusProjection{
	"PENDING":   {Codes: []int16{1, 2, 3, 31, 32}, IncludeNull: true},
	"UPLOADED":  {Codes: []int16{99}},
	"FAILED":    {Codes: []int16{91}},
	"NOT_FOUND": {Codes: []int16{-1}},
}

type adminTaxReviewFilter struct {
	Statuses     []string
	NearDeadline bool
	Extended     bool
}

var taxReviewQueueStatuses = []string{
	codemeta.SellerTaxReviewStatusSubmitted.Value,
	codemeta.SellerTaxReviewStatusUnderReview.Value,
	codemeta.SellerTaxReviewStatusSupplementRequired.Value,
	codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
}

var adminTaxReviewFilters = map[string]adminTaxReviewFilter{
	"NEW": {Statuses: []string{codemeta.SellerTaxReviewStatusSubmitted.Value}},
	"SUPPLEMENT_REQUIRED": {Statuses: []string{
		codemeta.SellerTaxReviewStatusSupplementRequired.Value,
	}},
	"UNDER_REVIEW": {Statuses: []string{
		codemeta.SellerTaxReviewStatusUnderReview.Value,
		codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
	}},
	"NEAR_DEADLINE": {Statuses: taxReviewQueueStatuses, NearDeadline: true},
	"EXTENDED":      {Statuses: taxReviewQueueStatuses, Extended: true},
	"EXPIRED":       {Statuses: []string{codemeta.SellerTaxReviewStatusExpired.Value}},
}

var providerErrorStatuses = []string{
	codemeta.InvoiceProviderAccountStatusInvalidCredential.Value,
	codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
	codemeta.InvoiceProviderAccountStatusDegraded.Value,
	codemeta.InvoiceProviderAccountStatusConfigError.Value,
}

func projectProviderStatus(hasAccount bool, status string) string {
	if !hasAccount {
		return codemeta.InvoiceProviderAccountStatusNotStarted.Value
	}
	if _, ok := codemeta.InvoiceProviderAccountStatusFromValue(status); !ok {
		return codemeta.InvoiceProviderAccountStatusConfigError.Value
	}
	return status
}

const (
	orderInvoiceStateAwaitingPayment  = "AWAITING_PAYMENT"
	orderInvoiceStateProcessing       = "PROCESSING"
	orderInvoiceStateIssued           = "ISSUED"
	orderInvoiceStateVoided           = "VOIDED"
	orderInvoiceStateAllowanceApplied = "ALLOWANCE_APPLIED"
	orderInvoiceStateFailed           = "FAILED"
	orderInvoiceStateSellerWillIssue  = "SELLER_WILL_ISSUE"
	orderInvoiceStateSelfIssued       = "SELLER_SELF_ISSUED"
	orderInvoiceStateOrderCancelled   = "ORDER_CANCELLED"
)

var orderInvoiceStateNotices = map[string]string{
	orderInvoiceStateAwaitingPayment:  "付款完成後會立即開立發票",
	orderInvoiceStateProcessing:       "系統正在處理，完成後會寄送 Email",
	orderInvoiceStateIssued:           "發票已開立，已寄送至您的電子信箱",
	orderInvoiceStateVoided:           "原發票已作廢",
	orderInvoiceStateAllowanceApplied: "已開立折讓單",
	orderInvoiceStateFailed:           "發票開立遇到問題，平台處理中",
	orderInvoiceStateSellerWillIssue:  "本筆由賣家自行開立發票",
	orderInvoiceStateSelfIssued:       "賣家已自行開立發票，如未收到請聯繫賣家",
	orderInvoiceStateOrderCancelled:   "訂單已取消，不會開立發票",
}

var receiptStateNotices = map[string]string{
	orderInvoiceStateAwaitingPayment: "付款完成後會立即產製收據",
	orderInvoiceStateProcessing:      "系統正在產製收據，完成後可在此下載",
	orderInvoiceStateIssued:          "收據已產製，可在此下載",
	orderInvoiceStateVoided:          "原收據已作廢",
	orderInvoiceStateFailed:          "收據產製遇到問題，平台處理中",
	orderInvoiceStateOrderCancelled:  "訂單已取消，不會產製收據",
}

// StateNoticeFor 依文件種類選文案。**這是唯一的取用入口**——直接查
// orderInvoiceStateNotices 就是本次 bug 的形狀。
//
// documentKind 為空字串（零 document 的訂單，還沒決定要開什麼）⇒ 走發票那份：
// 那時買家看到的是「付款完成後會立即開立發票」，而絕大多數訂單確實如此；
// 免用發票賣家的訂單在付款前就已知道會走收據線嗎？**不知道**——決策樹在
// order.paid 才跑。這一格是誠實的預設值，不是遺漏。
func StateNoticeFor(documentKind, state string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		if s, ok := receiptStateNotices[state]; ok {
			return s
		}
	}
	return orderInvoiceStateNotices[state]
}

// ReceiptShippingFeeNotice 是 ShippingFeeNotice 的收據版本（#3026）。
//
// 原句逐字提到兩次「發票」，掛在收據上就是把收據講成發票。句型與資訊量保持一致：
// 要說的是同一件事（運費已在總額內、另列的物流明細不是第二份憑證）。
const ReceiptShippingFeeNotice = "物流費用已包含在本張收據的總額中，另列的物流明細不是另一張收據。"

// ShippingFeeNoticeFor 依文件種類選運費說明。
func ShippingFeeNoticeFor(documentKind string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return ReceiptShippingFeeNotice
	}
	return ShippingFeeNotice
}

// ShippingFeeNotice 是 B04／B05 底部 info banner 的定版字串（58A §B04 畫面表，#3031）。
//
// 🔴 **逐字，不得改寫**：58A 兩處都寫「逐字顯示後端 shippingFeeNotice」，B05 那頁還註明
// 「本頁沒有該欄位時用同一句常數」⇒ 前端有一份硬編碼的同句話，兩邊一旦漂移，
// 同一個買家在兩個頁面會看到兩種說法。
const ShippingFeeNotice = "物流費用已包含在本張發票的銷售額中，另列的物流明細不是另一張發票。"

// CorrectionStatusNotices 是 B06 進度卡（端點 11／12）每個 status 的逐字文案（#3031）。
//
// 🔴 **「前端不解析 statusNotice」與「statusNotice 沒有內容」是兩件事**，程式碼原本的
// 註解把它們混成一件（寫著「這一欄沒有被規定的內容」）：58A §B06 逐字寫著
// 「`statusNotice`→成功 toast 的 body（逐字）」——前端**顯示**它，只是不**解析**它去
// 決定行為（那是 rejectReasonCode 的職責）。恆為空字串的結果是那個 toast 沒有內文。
//
// 文案規約沿用 orderInvoiceStateNotices：說買家下一步該預期什麼，不承諾本服務管不到的事
// （例如退款）。
//
// ⚠️ REJECTED 只給一句中性的引導：真正的原因由 rejectReasonCode 決定文案，
// 在這裡再寫一次會變成第二份真相，而兩份必然漂移。
var CorrectionStatusNotices = map[string]string{
	codemeta.InvoiceCorrectionStatusRequested.Value:             "已收到您的修改需求，正在確認可否修改",
	codemeta.InvoiceCorrectionStatusRejected.Value:              "這次修改無法進行，請見下方原因",
	codemeta.InvoiceCorrectionStatusUpdatedBeforeIssue.Value:    "發票尚未開立，已直接更新為新的資料",
	codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value: "原發票作廢與重新開立處理中，完成後會寄送 Email",
	codemeta.InvoiceCorrectionStatusCompleted.Value:             "已重新開立發票，已寄送至您的電子信箱",
	codemeta.InvoiceCorrectionStatusReissueFailed.Value:         "原發票已作廢、新發票尚未開立，平台正在處理",
}

var documentStatusToBuyerState = map[string]string{
	codemeta.InvoiceDocumentStatusPending.Value:              orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusIssuing.Value:              orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusVoiding.Value:              orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusAwaitingManual.Value:       orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusIssued.Value:               orderInvoiceStateIssued,
	codemeta.InvoiceDocumentStatusVoided.Value:               orderInvoiceStateVoided,
	codemeta.InvoiceDocumentStatusSuperseded.Value:           orderInvoiceStateVoided,
	codemeta.InvoiceDocumentStatusIssueFailed.Value:          orderInvoiceStateFailed,
	codemeta.InvoiceDocumentStatusNotApplicable.Value:        orderInvoiceStateSellerWillIssue,
	codemeta.InvoiceDocumentStatusResolvedExternally.Value:   orderInvoiceStateSelfIssued,
	codemeta.InvoiceDocumentStatusCancelledBeforeIssue.Value: orderInvoiceStateVoided,
}

func timelineEventOf(subjectType, toStatus string) (string, bool) {
	switch {
	case subjectType == subjectDocument && toStatus == codemeta.InvoiceDocumentStatusIssued.Value:
		return codemeta.InvoiceDocumentTimelineEventIssued.Value, true
	case subjectType == subjectDocument && toStatus == codemeta.InvoiceDocumentStatusVoided.Value:
		return codemeta.InvoiceDocumentTimelineEventVoided.Value, true
	case subjectType == subjectDocument && toStatus == codemeta.InvoiceDocumentStatusSuperseded.Value:
		return codemeta.InvoiceDocumentTimelineEventReissued.Value, true
	case subjectType == subjectCorrectionRequest &&
		toStatus == codemeta.InvoiceCorrectionStatusUpdatedBeforeIssue.Value:
		return codemeta.InvoiceDocumentTimelineEventUpdated.Value, true
	default:
		return "", false
	}
}

const subjectCorrectionRequest = "CORRECTION_REQUEST"

const (
	adjustmentTypeAllowance   = "ALLOWANCE"
	adjustmentStatusCompleted = "COMPLETED"
)
