package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func item(s string) *string { return &s }

func TestPlanDialogue_MissingRequired_SelectsAckAndAskRequired(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Current: Slots{}, Next: Slots{Item: item("蘋果")},
		Missing: []SlotKey{SlotDeliveryCity, SlotUnit}, Ready: false,
	})
	require.Equal(t, ActAckAndAskRequired, plan.ActCode)
	require.Equal(t, IntakeReady, plan.ReadinessGate)
}

func TestPlanDialogue_Degraded_TakesPriorityOverEverythingElse(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Degraded: true, Ready: false, HasRejected: true,
	})
	require.Equal(t, ActExtractionDegraded, plan.ActCode)
}

func TestPlanDialogue_Rejected_TakesPriorityOverCorrection(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		HasRejected: true, Ready: true,
		Current: Slots{Item: item("蘋果")}, Next: Slots{Item: item("蘋果")},
		Recorded: []RecordedField{{Key: "item", Value: "蘋果"}},
	})
	require.Equal(t, ActInputRejected, plan.ActCode)
}

func TestPlanDialogue_CorrectionOfExistingField_SelectsCorrectionApplied(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Ready: true, Question: nil,
		Current:  Slots{Item: item("蘋果")},
		Next:     Slots{Item: item("富士蘋果")},
		Recorded: []RecordedField{{Key: "item", Value: "富士蘋果"}},
	})
	require.Equal(t, ActCorrectionApplied, plan.ActCode)
}

func TestPlanDialogue_FirstTimeFill_DoesNotSelectCorrectionApplied(t *testing.T) {
	// current.Item 是 nil（第一次填），不該被判定成「更正」。
	plan := PlanDialogue(DialoguePlanInput{
		Ready: true, Question: nil,
		Current:  Slots{},
		Next:     Slots{Item: item("蘋果")},
		Recorded: []RecordedField{{Key: "item", Value: "蘋果"}},
	})
	require.NotEqual(t, ActCorrectionApplied, plan.ActCode)
	require.Equal(t, ActRequestBasicsReady, plan.ActCode)
}

func TestPlanDialogue_ReadyWithFollowUpQuestion_SelectsAckAndAskOptional(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Ready: true, Question: &ClarifyQuestion{SlotKey: SlotQuantity},
	})
	require.Equal(t, ActAckAndAskOptional, plan.ActCode)
}

func TestPlanDialogue_ReadyNoFollowUp_SelectsRequestBasicsReady(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{Ready: true, Question: nil})
	require.Equal(t, ActRequestBasicsReady, plan.ActCode)
}

func TestPlanDialogue_PricePreviewAvailable_SelectsPricePreviewAvailable(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Ready: true, Question: nil, PricePreviewAvailable: true,
	})
	require.Equal(t, ActPricePreviewAvailable, plan.ActCode)
	require.Equal(t, PricePreviewReady, plan.ReadinessGate)
}

func TestPlanDialogue_RfqStale_SelectsRfqReconfirmRequired(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Ready: true, PricePreviewAvailable: true, RfqConfirmationState: RfqStale,
	})
	require.Equal(t, ActRfqReconfirmRequired, plan.ActCode)
	require.Equal(t, RfqConfirmationReady, plan.ReadinessGate)
}

func TestPlanDialogue_RfqConfirmed_SelectsPublicationApprovalRequired_NotRfqConfirmed(t *testing.T) {
	// 見本任務開頭的完整 ruling：readinessGate 一旦達到 RfqConfirmationReady 就
	// 立刻前進到 PublicationReady，所以 act 選中的是 PublicationApprovalRequired
	// 而不是 RfqConfirmed——這是刻意的設計結果，不是漏掉 RfqConfirmed 分支。
	plan := PlanDialogue(DialoguePlanInput{
		Ready: true, PricePreviewAvailable: true, RfqConfirmationState: RfqConfirmed,
	})
	require.Equal(t, ActPublicationApprovalRequired, plan.ActCode)
	require.Equal(t, PublicationReady, plan.ReadinessGate)
}

// TestPlanDialogue_CorrectionTakesPriorityOverNotReady 釘住 selectActCode
// 目前的優先序：priority 3（HasCorrection，由 computeHasCorrection 判斷）排在
// priority 4（!Ready）之前。買家更正一個已有值的欄位時，即使這一輪還有其他
// 必填欄位缺著，也要回 ActCorrectionApplied，不是 ActAckAndAskRequired——不然
// 買家剛更正完一個值，卻只收到「還缺什麼什麼」的固定文案，完全看不出剛剛的更正
// 有沒有被系統聽懂。這支測試只釘住現況，不改 selectActCode 的邏輯（final review
// finding：這兩個優先序此前沒有 fixture 覆蓋）。
func TestPlanDialogue_CorrectionTakesPriorityOverNotReady(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Ready: false, Missing: []SlotKey{SlotDeliveryCity},
		Current:  Slots{Item: item("蘋果")},
		Next:     Slots{Item: item("富士蘋果")},
		Recorded: []RecordedField{{Key: "item", Value: "富士蘋果"}},
	})
	require.Equal(t, ActCorrectionApplied, plan.ActCode)
}

// TestPlanDialogue_QuestionTakesPriorityOverPricePreviewAvailable 釘住
// selectActCode 的優先序：priority 7（Question != nil）排在 priority 8
// （PricePreviewAvailable）之前，所以兩者同時成立時選中的是
// ActAckAndAskOptional，不是 ActPricePreviewAvailable。
//
// 這個輸入組合在正式環境會真的發生：必填齊了（Ready）、報價試算已經有結果
// （PricePreviewAvailable），但還有一個選填欄位沒問到（Question 非
// nil）——final review 抓到這個順序此前沒有 fixture 覆蓋，並指出若
// PricePreviewAvailable 贏，回覆文字只會講報價、完全不提同一輪還附著的問題卡，
// 是跟這個功能原本要根治的「回覆說 X、畫面做 Y」（issue #2403）同一類問題。
// 已決定讓待答問題（可操作的請求）優先於報價可看（純資訊性提示）——先把選填
// 問題問完，報價可看的提示延到下一輪講，不會遺失，只是晚一輪；這也是
// task-oriented dialogue system 處理「可操作請求 vs. 資訊性通知」搶版面的常見
// 慣例。
func TestPlanDialogue_QuestionTakesPriorityOverPricePreviewAvailable(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Ready: true, PricePreviewAvailable: true,
		Question: &ClarifyQuestion{SlotKey: SlotQuantity},
	})
	require.Equal(t, ActAckAndAskOptional, plan.ActCode)
}

func TestPlanDialogue_PlannableActCodes_AllReachableViaSomeFixture(t *testing.T) {
	// 窮舉性測試：above 9 個測試分別餵不同輸入，這裡把它們的輸出集合起來，
	// 確認 PlannableActCodes 的每一個值都至少被其中一個 fixture 選中過——
	// 防止之後 PlannableActCodes 加了新值、卻忘記在 selectActCode 加對應分支。
	reachable := map[ActCode]bool{}
	fixtures := []DialoguePlanInput{
		{Missing: []SlotKey{SlotItem}, Ready: false},
		{Degraded: true},
		{HasRejected: true, Ready: true, Recorded: []RecordedField{{Key: "item"}}},
		{Ready: true, Current: Slots{Item: item("A")}, Next: Slots{Item: item("B")}, Recorded: []RecordedField{{Key: "item", Value: "B"}}},
		{Ready: true, Question: &ClarifyQuestion{SlotKey: SlotQuantity}},
		{Ready: true},
		{Ready: true, PricePreviewAvailable: true},
		{Ready: true, PricePreviewAvailable: true, RfqConfirmationState: RfqStale},
		{Ready: true, PricePreviewAvailable: true, RfqConfirmationState: RfqConfirmed},
		{CropDisambiguationPending: true, CropDisambiguationCandidates: []CropDisambiguationOption{{Label: "小白菜"}}},
	}
	for _, in := range fixtures {
		reachable[PlanDialogue(in).ActCode] = true
	}
	for _, code := range PlannableActCodes {
		require.True(t, reachable[code], "PlannableActCode %s has no fixture that reaches it", code)
	}
}

func TestSelectActCode_CropDisambiguationPending_TakesPriorityOverNotReady(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		Ready:                        false, // 城市/單位都還沒填
		CropDisambiguationPending:    true,
		CropDisambiguationCandidates: []CropDisambiguationOption{{Label: "小白菜"}},
	})
	require.Equal(t, ActItemNeedsDisambiguation, plan.ActCode)
}

func TestSelectActCode_HasRejected_TakesPriorityOverCropDisambiguationPending(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{
		HasRejected: true, CropDisambiguationPending: true,
	})
	require.Equal(t, ActInputRejected, plan.ActCode)
}

func TestPlanDialogue_ClaimCodes_ItemNeedsDisambiguation(t *testing.T) {
	plan := PlanDialogue(DialoguePlanInput{CropDisambiguationPending: true})
	require.Equal(t, ActItemNeedsDisambiguation, plan.ActCode)
	require.NotEmpty(t, plan.AllowedClaimCodes)
	require.Contains(t, plan.ForbiddenClaimCodes, "SEARCH_STARTED")
}

func TestPlanDialogue_ClaimCodes_PerAct(t *testing.T) {
	cases := []struct {
		name       string
		in         DialoguePlanInput
		act        ActCode
		allowed    []string
		mustForbid []string // 子集檢查，不要求逐一列全域清單
	}{
		{
			name:    "ACK_AND_ASK_REQUIRED",
			in:      DialoguePlanInput{Ready: false},
			act:     ActAckAndAskRequired,
			allowed: []string{"REQUEST_FACT_RECORDED", "NEXT_QUESTION_REQUIRED"},
		},
		{
			name:       "REQUEST_BASICS_READY forbids SEARCH_STARTED",
			in:         DialoguePlanInput{Ready: true, PricePreviewAvailable: false},
			act:        ActRequestBasicsReady,
			allowed:    []string{"BASICS_COMPLETE"},
			mustForbid: []string{"SEARCH_STARTED"},
		},
		{
			name: "PUBLICATION_APPROVAL_REQUIRED forbids RFQ_PUBLISHED",
			in: DialoguePlanInput{
				Ready: true, PricePreviewAvailable: true,
				RfqConfirmationState: RfqConfirmed,
			},
			act:        ActPublicationApprovalRequired,
			allowed:    []string{"RFQ_PENDING_APPROVAL"},
			mustForbid: []string{"RFQ_PUBLISHED", "SELLER_NOTIFIED"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := PlanDialogue(tc.in)
			require.Equal(t, tc.act, plan.ActCode, "unexpected act code for input")
			require.ElementsMatch(t, tc.allowed, plan.AllowedClaimCodes)
			for _, f := range tc.mustForbid {
				require.Contains(t, plan.ForbiddenClaimCodes, f)
			}
		})
	}
}
