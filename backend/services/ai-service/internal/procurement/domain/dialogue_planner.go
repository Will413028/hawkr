package domain

// RfqConfirmationState 是這個 request 目前 RFQ 確認狀態的三選一判別式——由呼叫端
// （turn.go／answer.go／rewind.go）比對 Task 2 新增的
// FindLatestConfirmedRfqSnapshotByRequestID 結果的 RequestRevisionID 與目前
// active revision ID 算出，PlanDialogue 本身不做任何 I/O，只消費算好的結果。
type RfqConfirmationState int

const (
	RfqNone      RfqConfirmationState = iota // 沒有任何確認快照
	RfqStale                                 // 有快照，但 RequestRevisionID 跟目前 active revision 不同
	RfqConfirmed                             // 有快照，且對得上目前 active revision
)

// DialoguePlanInput 是 PlanDialogue 的輸入——欄位範圍刻意只涵蓋 turn.go／
// answer.go／rewind.go 三個呼叫點目前實際擁有或能低成本取得的 authoritative
// state（見 spec §3.2／§3.3）。之後若要讓候選搜尋／賣家回應等其他呼叫點也接上
// PlanDialogue，屬於各自那次改動的範圍：additive 擴充這個 struct 並在
// selectActCode／computeReadinessGate 加對應分支，不是本計畫要做的事。
type DialoguePlanInput struct {
	Current Slots // Apply 前
	Next    Slots // Apply 後

	Missing     []SlotKey
	Ready       bool // ＝ turn.go／answer.go／rewind.go 既有的 status=="REQUIREMENT_READY"
	Rejected    []RejectedField
	HasRejected bool
	Degraded    bool
	Recorded    []RecordedField // 這一輪 domain.RecordedThisTurn 的輸出（呼叫端必須排在呼叫 PlanDialogue 之前算好，見 Task 5）

	// CropDisambiguationPending／CropDisambiguationCandidates 由呼叫端的
	// cropidentity.ComputeGate（cropidentity/crop_gate.go）算好餵入——PlanDialogue 本身
	// 不做任何 I/O。Pending 為真時 Question 這一輪必須是
	// buildCropDisambiguationQuestion 的結果，不是 domain.PickNextQuestion
	// 的一般缺項/選填問句（兩者互斥，見 spec §5）。
	CropDisambiguationPending    bool
	CropDisambiguationCandidates []CropDisambiguationOption

	Question *ClarifyQuestion // 這一輪算出的下一題，nil 表示沒有

	PricePreviewAvailable bool // ＝ 既有 PricePreviewChecker.HasPricePreview 的結果
	RfqConfirmationState  RfqConfirmationState
}

// PlanDialogue 是唯一的 act 決策入口——純函式，同樣輸入必得同樣輸出，無 I/O、
// 不呼叫任何 LLM。完整的 act 優先序 ruling 見本檔案所在任務（Task 3）的計畫文字，
// 這裡的程式碼是那份 ruling 的直接翻譯，不重複展開理由。
func PlanDialogue(in DialoguePlanInput) DialoguePlanV1 {
	act := selectActCode(in)
	gate := computeReadinessGate(in)
	return DialoguePlanV1{
		SchemaVersion:       "dialogue-plan.v1",
		ActCode:             act,
		ReadinessGate:       gate,
		SalientFactRefs:     salientFactRefsFor(in),
		NextQuestionRef:     nextQuestionRefFor(in),
		ArtifactRefs:        []string{}, // Phase 1 不涉及 artifact（候選/報價單以外的產物），恆空陣列
		AllowedClaimCodes:   claimCodesFor(act),
		ForbiddenClaimCodes: GlobalForbiddenClaimCodes,
		ToneHints:           ToneHints{Verbosity: "CONCISE"},
		MaxSentences:        2,
		PolicyVersion:       "phase3-v1",
		TemplateVersion:     TemplateRegistryVersion, // 直接引用 dialogue_templates.go 的常數，避免同一個 package 內兩份字面值各自維護的漂移風險（5c68504fc 曾修過一次 stale literal）
		// PlanHash／StateVersion 由呼叫端接線時填（Task 6/7/8；需要 request/revision 的
		// 實際識別資訊，PlanDialogue 本身拿不到，呼叫端組完整 turnPayload 時補上）。
	}
}

func selectActCode(in DialoguePlanInput) ActCode {
	switch {
	case in.Degraded:
		return ActExtractionDegraded
	case in.HasRejected:
		return ActInputRejected
	case computeHasCorrection(in):
		return ActCorrectionApplied
	case in.CropDisambiguationPending:
		return ActItemNeedsDisambiguation
	case !in.Ready:
		return ActAckAndAskRequired
	case in.RfqConfirmationState == RfqStale:
		return ActRfqReconfirmRequired
	case in.RfqConfirmationState == RfqConfirmed:
		return ActPublicationApprovalRequired
	case in.Question != nil:
		return ActAckAndAskOptional
	case in.PricePreviewAvailable:
		return ActPricePreviewAvailable
	default:
		return ActRequestBasicsReady
	}
}

func computeReadinessGate(in DialoguePlanInput) ReadinessGate {
	switch {
	case !in.Ready:
		return IntakeReady
	case !in.PricePreviewAvailable:
		return PricePreviewReady
	case in.RfqConfirmationState == RfqStale:
		return RfqConfirmationReady
	case in.RfqConfirmationState == RfqConfirmed:
		return PublicationReady
	default: // Ready && PricePreviewAvailable && RfqConfirmationState == RfqNone —
		// RFQ never touched yet, distinct from RfqStale (touched but expired):
		// don't read this as "stuck at the RFQ layer" when it was never reached.
		return PricePreviewReady
	}
}

// computeHasCorrection 判斷這一輪 recorded 欄位裡，有沒有任何一個在 current
// （Apply 前）就已經非空——是的話代表這是「更正一個已有值的欄位」，不是第一次填。
func computeHasCorrection(in DialoguePlanInput) bool {
	for _, r := range in.Recorded {
		if slotWasAlreadySet(in.Current, SlotKey(r.Key)) {
			return true
		}
	}
	return false
}

func slotWasAlreadySet(s Slots, key SlotKey) bool {
	switch key {
	case SlotItem:
		return s.Item != nil
	case SlotQuantity:
		return s.Quantity != nil
	case SlotUnit:
		return s.Unit != nil
	case SlotPackaging:
		return s.Packaging != nil
	case SlotBudgetPerUnit:
		return s.BudgetPerUnit != nil
	case SlotDeliveryCity:
		return s.DeliveryCity != nil
	case SlotDeliveryDate:
		return s.DeliveryDate != nil
	case SlotExactMatchRequired:
		return s.ExactMatchRequired != nil
	default:
		return false
	}
}

// salientFactRefsFor 取這一輪 recorded 欄位的 key，最多 2 個（issue §C：
// 「最多 1–2 個已 commit 的非敏感事實」），維持 domain/recorded.go 既有的
// slotOrder 順序（recorded 本身已經照那個順序排好，這裡不重新排序）。
func salientFactRefsFor(in DialoguePlanInput) []string {
	refs := make([]string, 0, 2)
	for _, r := range in.Recorded {
		if len(refs) >= 2 {
			break
		}
		refs = append(refs, "request."+r.Key)
	}
	return refs
}

func nextQuestionRefFor(in DialoguePlanInput) string {
	if in.Question == nil {
		return ""
	}
	return "question." + string(in.Question.SlotKey)
}

// claimCodesFor 回傳這個 act 允許在敘述中主張的 claim code（spec §3 表格）。
// 全域禁用清單（domain.GlobalForbiddenClaimCodes）不在這裡重複列——validator
// 與呼叫端統一從 ForbiddenClaimCodes 讀，這裡只需要回傳每個 act 各自的
// allowlist。非 9 個 plannable act（Phase 1 尚不可達，見 dialogue_plan.go
// 註解）回傳空陣列——它們目前不會被 selectActCode 選中，claim allowlist
// 留給各自解除封鎖時再補（不是遺漏，是 spec §3 明確的範圍界定）。
func claimCodesFor(act ActCode) []string {
	switch act {
	case ActAckAndAskRequired:
		return []string{string(ClaimRequestFactRecorded), string(ClaimNextQuestionRequired)}
	case ActAckAndAskOptional:
		return []string{string(ClaimRequestFactRecorded), string(ClaimNextQuestionOptional)}
	case ActCorrectionApplied:
		return []string{string(ClaimCorrectionApplied)}
	case ActInputRejected:
		return []string{string(ClaimInputRejected), string(ClaimNextQuestionRequired)}
	case ActItemNeedsDisambiguation:
		return []string{string(ClaimNextQuestionRequired)}
	case ActExtractionDegraded:
		return []string{string(ClaimExtractionDegraded)}
	case ActRequestBasicsReady:
		return []string{string(ClaimBasicsComplete)}
	case ActPricePreviewAvailable:
		return []string{string(ClaimPriceReferenceAvailable), string(ClaimNextQuestionRequired)}
	case ActRfqReconfirmRequired:
		return []string{string(ClaimReconfirmationRequired)}
	case ActPublicationApprovalRequired:
		return []string{string(ClaimRfqPendingApproval)}
	default:
		return []string{}
	}
}
