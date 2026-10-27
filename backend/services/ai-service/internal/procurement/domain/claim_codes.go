package domain

// ClaimCode 是 SurfaceRealizer 每個 block 自我分類的「這段文字主張了什麼」
// 詞彙（issue #2549 §E「不得出現未在 allowed refs 的…」；spec §3／§4）。
// Semantic validator 拿它跟 DialoguePlanV1.AllowedClaimCodes／
// ForbiddenClaimCodes 比對，不是對外契約碼，不進 CodeMeta。
type ClaimCode string

const (
	ClaimRequestFactRecorded     ClaimCode = "REQUEST_FACT_RECORDED"
	ClaimNextQuestionRequired    ClaimCode = "NEXT_QUESTION_REQUIRED"
	ClaimNextQuestionOptional    ClaimCode = "NEXT_QUESTION_OPTIONAL"
	ClaimCorrectionApplied       ClaimCode = "CORRECTION_APPLIED"
	ClaimInputRejected           ClaimCode = "INPUT_REJECTED"
	ClaimExtractionDegraded      ClaimCode = "EXTRACTION_DEGRADED"
	ClaimBasicsComplete          ClaimCode = "BASICS_COMPLETE"
	ClaimPriceReferenceAvailable ClaimCode = "PRICE_REFERENCE_AVAILABLE"
	ClaimReconfirmationRequired  ClaimCode = "RECONFIRMATION_REQUIRED"
	ClaimRfqPendingApproval      ClaimCode = "RFQ_PENDING_APPROVAL"

	// 以下 7 個是全域禁用——不論哪個 act，都不得宣稱這些未發生的副作用
	// （issue #2549 §E：「不得宣稱搜尋／發布／通知／成交／訂單／付款等未發生
	// 副作用」；spec §3 全域禁用清單）。
	ClaimSearchStarted    ClaimCode = "SEARCH_STARTED"
	ClaimSearchCompleted  ClaimCode = "SEARCH_COMPLETED"
	ClaimRfqPublished     ClaimCode = "RFQ_PUBLISHED"
	ClaimSellerNotified   ClaimCode = "SELLER_NOTIFIED"
	ClaimDealCreated      ClaimCode = "DEAL_CREATED"
	ClaimOrderCreated     ClaimCode = "ORDER_CREATED"
	ClaimPaymentProcessed ClaimCode = "PAYMENT_PROCESSED"
)

// GlobalForbiddenClaimCodes 適用所有 9 個 plannable act（spec §3）——
// 不需要在 dialogue_planner.go 逐 act 重複列出，validator 與 claim code
// 回填共用這份清單。
var GlobalForbiddenClaimCodes = []string{
	string(ClaimSearchStarted), string(ClaimSearchCompleted), string(ClaimRfqPublished),
	string(ClaimSellerNotified), string(ClaimDealCreated), string(ClaimOrderCreated),
	string(ClaimPaymentProcessed),
}
