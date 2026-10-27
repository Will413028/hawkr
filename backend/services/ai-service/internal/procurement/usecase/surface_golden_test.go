// services/ai-service/internal/procurement/usecase/surface_golden_test.go
package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

// TestRealizeOrFallback_GoldenFixtures_AllPlannableActs 涵蓋 10 個 plannable
// act 的 realizer-error→fallback 這一條路徑：realizer 回傳 error 時，每個
// act 都必須正確退回 domain.RenderPlan 直接呼叫的結果（byte-equivalent，
// 冪等性的必要條件）且 mode 回 "TEMPLATE"。
//
// final review finding 2：這支測試的 doc comment 之前宣稱涵蓋「realize
// 成功／validator reject／realizer error 三種路徑」，但迴圈本身只測了
// error→fallback 一種；success／reject 路徑分別由下面
// TestRealizeOrFallback_GoldenFixtures_RealPlanPerAct（success）與
// TestRealizeOrFallback_REQUEST_BASICS_READY_RejectsSearchClaim／
// TestRealizeOrFallback_PUBLICATION_APPROVAL_REQUIRED_RejectsPublishClaim
// （reject，僅 2/10 act）分開覆蓋，不是本測試本身涵蓋——這裡改成準確描述本測試
// 實際測的範圍，不誇大涵蓋面。
func TestRealizeOrFallback_GoldenFixtures_AllPlannableActs(t *testing.T) {
	for _, act := range domain.PlannableActCodes {
		act := act
		t.Run(string(act)+"_realizer_error_falls_back", func(t *testing.T) {
			plan := domain.DialoguePlanV1{ActCode: act, MaxSentences: 2}
			args := domain.SafeRenderArgs{}
			realizer := &fakeSurfaceRealizer{err: errBoom}
			blocks, mode := realizeOrFallback(context.Background(), realizer, plan, args, domain.Slots{}, nil)
			require.Equal(t, "TEMPLATE", mode)
			require.Equal(t, domain.RenderPlan(plan, args), blocks)
		})
	}
}

// TestRealizeOrFallback_REQUEST_BASICS_READY_RejectsSearchClaim 是 spec §9
// 「REQUEST_BASICS_READY 與 PUBLICATION_APPROVAL_REQUIRED 兩個 act 的
// forbidden claim code 測試必須涵蓋關鍵詞 blocklist 案例」的直接實作。
func TestRealizeOrFallback_REQUEST_BASICS_READY_RejectsSearchClaim(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActRequestBasicsReady,
		AllowedClaimCodes:   []string{"BASICS_COMPLETE"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{
		{Kind: domain.BlockUnderstood, Text: "需求齊全，已經開始搜尋候選了。", ClaimCodes: []string{"BASICS_COMPLETE"}},
	}}
	blocks, mode := realizeOrFallback(context.Background(), realizer, plan, domain.SafeRenderArgs{}, domain.Slots{}, nil)
	require.Equal(t, "TEMPLATE", mode)
	require.Equal(t, domain.RenderPlan(plan, domain.SafeRenderArgs{}), blocks)
}

func TestRealizeOrFallback_PUBLICATION_APPROVAL_REQUIRED_RejectsPublishClaim(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{
		{Kind: domain.BlockComplete, Text: "報價單已確認，也已經送出給賣家了。", ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
	}}
	blocks, mode := realizeOrFallback(context.Background(), realizer, plan, domain.SafeRenderArgs{}, domain.Slots{}, nil)
	require.Equal(t, "TEMPLATE", mode)
	require.Equal(t, domain.RenderPlan(plan, domain.SafeRenderArgs{}), blocks)
}

// TestValidateSurfaceBlocks_RealPlanPerAct_Passes 是 final review finding 2
// 的補洞測試：此前只有 2 個 act 曾經用 domain.PlanDialogue 真正算出的
// AllowedClaimCodes 餵過 validateSurfaceBlocks，其餘 8 個 act 只被手動組的
// 局部 allowlist 或恆定失敗的假 realizer 覆蓋，從未驗證過真實 plan 的
// allowlist 真的能通過 validator。這裡對每個 plannable act 用跟
// dialogue_planner_test.go TestPlanDialogue_PlannableActCodes_AllReachableViaSomeFixture
// 相同的輸入 fixture 呼叫真正的 domain.PlanDialogue，取它真實的
// AllowedClaimCodes[0] 組一個合法 RealizedBlock，直接斷言
// validateSurfaceBlocks 放行——刻意不經過 realizeOrFallback／假 realizer，
// 因為這裡要驗證的是「plan 的真實 allowlist 能否通過 validator」，跟
// realize 本身成功或失敗無關。
func TestValidateSurfaceBlocks_RealPlanPerAct_Passes(t *testing.T) {
	fixtures := map[domain.ActCode]domain.DialoguePlanInput{
		domain.ActAckAndAskRequired: {Missing: []domain.SlotKey{domain.SlotItem}, Ready: false},
		domain.ActAckAndAskOptional: {Ready: true, Question: &domain.ClarifyQuestion{SlotKey: domain.SlotQuantity}},
		domain.ActCorrectionApplied: {
			Ready: true, Current: domain.Slots{Item: strPtr("A")}, Next: domain.Slots{Item: strPtr("B")},
			Recorded: []domain.RecordedField{{Key: "item", Value: "B"}},
		},
		domain.ActInputRejected:         {HasRejected: true, Ready: true, Recorded: []domain.RecordedField{{Key: "item"}}},
		domain.ActExtractionDegraded:    {Degraded: true},
		domain.ActRequestBasicsReady:    {Ready: true},
		domain.ActPricePreviewAvailable: {Ready: true, PricePreviewAvailable: true},
		domain.ActRfqReconfirmRequired: {
			Ready: true, PricePreviewAvailable: true, RfqConfirmationState: domain.RfqStale,
		},
		domain.ActPublicationApprovalRequired: {
			Ready: true, PricePreviewAvailable: true, RfqConfirmationState: domain.RfqConfirmed,
		},
		domain.ActItemNeedsDisambiguation: {
			CropDisambiguationPending: true, CropDisambiguationCandidates: []domain.CropDisambiguationOption{{Label: "小白菜"}},
		},
	}
	require.Len(t, fixtures, len(domain.PlannableActCodes))

	for _, act := range domain.PlannableActCodes {
		act := act
		t.Run(string(act), func(t *testing.T) {
			in, ok := fixtures[act]
			require.True(t, ok, "missing fixture for act %s", act)
			plan := domain.PlanDialogue(in)
			require.Equal(t, act, plan.ActCode, "fixture no longer selects the expected act")
			require.NotEmpty(t, plan.AllowedClaimCodes, "act %s has no AllowedClaimCodes to test against", act)

			blocks := []RealizedBlock{
				{Kind: domain.BlockUnderstood, Text: "已收到你的訊息。", ClaimCodes: []string{plan.AllowedClaimCodes[0]}},
			}
			ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
			require.True(t, ok, "act=%s reason=%s", act, reason)
		})
	}
}
