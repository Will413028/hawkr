// Package domain（本檔延伸既有 package domain，見 slots.go 開頭 doc comment）。
package domain

// ActCode 是 DialoguePlanner 決定的「這一輪要溝通的意圖」——對應 issue #2549 §B
// 的 act registry（17 個值，產品已核准的完整清單）。Phase 1 只有 9 個會被
// PlanDialogue 實際選中（見 dialogue_planner.go 的 selectActCode 與其 doc
// comment 完整 ruling）；其餘 8 個常數仍先定義，避免之後各自接線時再命名一次、
// 產生 drift（不是過度設計——這是 issue 已核准、非本計畫發明的清單）。
type ActCode string

const (
	ActAckAndAskRequired           ActCode = "ACK_AND_ASK_REQUIRED"
	ActAckAndAskOptional           ActCode = "ACK_AND_ASK_OPTIONAL"
	ActCorrectionApplied           ActCode = "CORRECTION_APPLIED"
	ActInputRejected               ActCode = "INPUT_REJECTED"
	ActExtractionDegraded          ActCode = "EXTRACTION_DEGRADED"
	ActRequestBasicsReady          ActCode = "REQUEST_BASICS_READY"
	ActPricePreviewAvailable       ActCode = "PRICE_PREVIEW_AVAILABLE"
	ActNoReferenceNeedsBuyerRange  ActCode = "NO_REFERENCE_NEEDS_BUYER_RANGE" // Phase 1 不可達：見 spec §3.4（#2403 端點零持久化）
	ActPriceBasisUnavailable       ActCode = "PRICE_BASIS_UNAVAILABLE"        // Phase 1 不可達：同上
	ActRfqReconfirmRequired        ActCode = "RFQ_RECONFIRM_REQUIRED"
	ActRfqConfirmed                ActCode = "RFQ_CONFIRMED" // Phase 1 不可達：見 dialogue_planner.go selectActCode 的 ruling（readinessGate 一旦達到 RFQ_CONFIRMATION_READY 立刻前進到 PUBLICATION_READY，取代成 PublicationApprovalRequired）
	ActPublicationApprovalRequired ActCode = "PUBLICATION_APPROVAL_REQUIRED"
	ActSearchRunAccepted           ActCode = "SEARCH_RUN_ACCEPTED"           // Phase 1 不可達：DialoguePlanInput 無候選搜尋欄位
	ActCandidatesReady             ActCode = "CANDIDATES_READY"              // 同上
	ActNoCandidatesOfferWishPool   ActCode = "NO_CANDIDATES_OFFER_WISH_POOL" // 同上
	ActSellerResponsesReady        ActCode = "SELLER_RESPONSES_READY"        // 同上（#2538/#2539 未開工）
	ActExactAcceptanceConfirmed    ActCode = "EXACT_ACCEPTANCE_CONFIRMED"    // 同上
	// ActItemNeedsDisambiguation 是 issue #2549 §B 原始 17 值清單之外的第 18 個
	// 值——核准依據是 PM 2026-08-23 的產品需求（品項須落在平台分類樹），
	// 不是 #2549 原始範圍。
	ActItemNeedsDisambiguation ActCode = "ITEM_NEEDS_DISAMBIGUATION"
)

// AllActCodes 是完整 18 值，供 dialogue_templates.go 的安全網測試窮舉
// （RenderPlan 對任一值都不能 panic／回空字串）。
var AllActCodes = []ActCode{
	ActAckAndAskRequired, ActAckAndAskOptional, ActCorrectionApplied,
	ActInputRejected, ActExtractionDegraded, ActRequestBasicsReady,
	ActPricePreviewAvailable, ActNoReferenceNeedsBuyerRange, ActPriceBasisUnavailable,
	ActRfqReconfirmRequired, ActRfqConfirmed, ActPublicationApprovalRequired,
	ActSearchRunAccepted, ActCandidatesReady, ActNoCandidatesOfferWishPool,
	ActSellerResponsesReady, ActExactAcceptanceConfirmed, ActItemNeedsDisambiguation,
}

// PlannableActCodes 是 Phase 1 的 PlanDialogue 實際會選中的 10 個 act，供
// dialogue_planner_test.go／dialogue_templates_test.go 窮舉「每個都要有專屬
// template entry、且要有至少一組能選中它的 fixture 輸入」。
var PlannableActCodes = []ActCode{
	ActAckAndAskRequired, ActAckAndAskOptional, ActCorrectionApplied,
	ActInputRejected, ActExtractionDegraded, ActRequestBasicsReady,
	ActPricePreviewAvailable, ActRfqReconfirmRequired, ActPublicationApprovalRequired,
	ActItemNeedsDisambiguation,
}

// ReadinessGate 是這一輪「目前正在推進哪一層」的焦點——語意是「尚未達成、
// 現在是重點」，不是「已經達成的最高層」（對齊 issue §C 範例：
// actCode=ACK_AND_ASK_REQUIRED 時 readinessGate=INTAKE_READY，此時 intake
// 明明還沒達成，代表這個欄位回答的是「現在卡在哪一層」而非「爬到哪一層」）。
type ReadinessGate string

const (
	IntakeReady          ReadinessGate = "INTAKE_READY"
	PricePreviewReady    ReadinessGate = "PRICE_PREVIEW_READY"
	RfqConfirmationReady ReadinessGate = "RFQ_CONFIRMATION_READY"
	PublicationReady     ReadinessGate = "PUBLICATION_READY"
)

// ToneHints 目前只有一個值可用——先定義完整結構（跟 issue §C 的 JSON 對齊），
// 值域留給之後真的有第二種語氣時再擴充，不在 Phase 1 預先發明。
type ToneHints struct {
	Verbosity string `json:"verbosity"` // 目前恆為 "CONCISE"
}

// DialoguePlanV1 逐欄位對齊 issue #2549 §C 的 JSON schema。PlanDialogue 是唯一
// 產生者，RenderPlan 是唯一消費者（Phase 1 範圍內）。
type DialoguePlanV1 struct {
	SchemaVersion       string        `json:"schemaVersion"` // 恆為 "dialogue-plan.v1"
	ActCode             ActCode       `json:"actCode"`
	StateVersion        string        `json:"stateVersion"`
	ReadinessGate       ReadinessGate `json:"readinessGate"`
	SalientFactRefs     []string      `json:"salientFactRefs"`
	NextQuestionRef     string        `json:"nextQuestionRef,omitempty"`
	ArtifactRefs        []string      `json:"artifactRefs"`
	AllowedClaimCodes   []string      `json:"allowedClaimCodes"`
	ForbiddenClaimCodes []string      `json:"forbiddenClaimCodes"`
	ToneHints           ToneHints     `json:"toneHints"`
	MaxSentences        int           `json:"maxSentences"`
	PolicyVersion       string        `json:"policyVersion"`
	TemplateVersion     string        `json:"templateVersion"`
	PlanHash            string        `json:"planHash"`
}
