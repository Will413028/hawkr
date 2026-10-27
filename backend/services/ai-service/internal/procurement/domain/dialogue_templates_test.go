// services/ai-service/internal/procurement/domain/dialogue_templates_test.go
package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderPlan_AllAllActCodes_NeverPanicsAndNeverEmpty(t *testing.T) {
	for _, code := range AllActCodes {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RenderPlan panicked for %s: %v", code, r)
				}
			}()
			got := RenderPlan(DialoguePlanV1{ActCode: code}, SafeRenderArgs{})
			require.NotEmpty(t, got, "RenderPlan returned no blocks for %s", code)
			for _, b := range got {
				require.NotEmpty(t, string(b.Kind), "empty Kind in block for %s", code)
				require.NotEmpty(t, b.Text, "empty Text in block for %s", code)
			}
		}()
	}
}

func TestRenderPlan_PlannableActCodes_HaveDedicatedTemplate_NotGenericFallback(t *testing.T) {
	for _, code := range PlannableActCodes {
		_, ok := templates[code]
		require.True(t, ok, "PlannableActCode %s has no dedicated template entry", code)
	}
}

func TestRenderPlan_AckAndAskRequired_NoFacts_ListsMissingFieldLabels(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActAckAndAskRequired}, SafeRenderArgs{
		MissingRequiredLabels: []string{"配送城市", "數量"},
	})
	require.Len(t, got, 2)
	require.Equal(t, BlockUnderstood, got[0].Kind)
	require.Equal(t, BlockNextStep, got[1].Kind)
	require.Contains(t, got[1].Text, "配送城市")
	require.Contains(t, got[1].Text, "數量")
}

func TestRenderPlan_AckAndAskRequired_WithFacts_UnderstoodListsRecordedFacts(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActAckAndAskRequired}, SafeRenderArgs{
		SalientFactLabels:     []string{"品項：寶島梨"},
		MissingRequiredLabels: []string{"配送城市"},
	})
	require.Len(t, got, 2)
	require.Contains(t, got[0].Text, "品項：寶島梨")
}

func TestRenderPlan_AckAndAskOptional_UsesNextQuestionPromptWhenPresent(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActAckAndAskOptional}, SafeRenderArgs{
		NextQuestionPrompt: "需要指定包裝方式嗎？",
	})
	require.Len(t, got, 2)
	require.Equal(t, BlockNextStep, got[1].Kind)
	require.Equal(t, "需要指定包裝方式嗎？", got[1].Text)
}

func TestRenderPlan_CorrectionApplied_SingleUnderstoodBlock(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActCorrectionApplied}, SafeRenderArgs{
		SalientFactLabels: []string{"數量：50 公斤"},
	})
	require.Len(t, got, 1)
	require.Equal(t, BlockUnderstood, got[0].Kind)
	require.Contains(t, got[0].Text, "數量：50 公斤")
}

func TestRenderPlan_InputRejected_WarningThenNextStep(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActInputRejected}, SafeRenderArgs{
		RejectedLabels: []string{"配送城市：不在服務範圍"},
	})
	require.Len(t, got, 2)
	require.Equal(t, BlockWarning, got[0].Kind)
	require.Contains(t, got[0].Text, "配送城市：不在服務範圍")
	require.Equal(t, BlockNextStep, got[1].Kind)
}

func TestRenderPlan_ExtractionDegraded_WarningThenContext(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActExtractionDegraded}, SafeRenderArgs{})
	require.Len(t, got, 2)
	require.Equal(t, BlockWarning, got[0].Kind)
	require.Equal(t, BlockContext, got[1].Kind)
}

func TestRenderPlan_RequestBasicsReady_UnderstoodThenNextStep(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActRequestBasicsReady}, SafeRenderArgs{})
	require.Len(t, got, 2)
	require.Equal(t, BlockUnderstood, got[0].Kind)
	require.Equal(t, BlockNextStep, got[1].Kind)
}

func TestRenderPlan_PricePreviewAvailable_ContextThenNextStep(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActPricePreviewAvailable}, SafeRenderArgs{})
	require.Len(t, got, 2)
	require.Equal(t, BlockContext, got[0].Kind)
	require.Equal(t, BlockNextStep, got[1].Kind)
}

func TestRenderPlan_RfqReconfirmRequired_WarningThenNextStep(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActRfqReconfirmRequired}, SafeRenderArgs{})
	require.Len(t, got, 2)
	require.Equal(t, BlockWarning, got[0].Kind)
	require.Equal(t, BlockNextStep, got[1].Kind)
}

func TestRenderPlan_PublicationApprovalRequired_CompleteThenRationale(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActPublicationApprovalRequired}, SafeRenderArgs{})
	require.Len(t, got, 2)
	require.Equal(t, BlockComplete, got[0].Kind)
	require.Equal(t, BlockRationale, got[1].Kind)
}

func TestRenderPlan_UnplannableActCode_FallsBackToGenericSingleBlock(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActRfqConfirmed}, SafeRenderArgs{})
	require.Len(t, got, 1)
	require.Equal(t, BlockUnderstood, got[0].Kind)
	require.Equal(t, genericFallbackText, got[0].Text)
}

func TestTextFallback_JoinsBlockTextsWithSpace(t *testing.T) {
	got := TextFallback([]Block{
		{Kind: BlockUnderstood, Text: "已收到你的需求。"},
		{Kind: BlockNextStep, Text: "還想請你補充：配送城市。"},
	})
	require.Equal(t, "已收到你的需求。 還想請你補充：配送城市。", got)
}

func TestTextFallback_EmptyBlocksReturnsEmptyString(t *testing.T) {
	require.Equal(t, "", TextFallback(nil))
}

func TestRenderPlan_ItemNeedsDisambiguation_UsesNextQuestionPrompt(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActItemNeedsDisambiguation}, SafeRenderArgs{
		NextQuestionPrompt: "找到幾個相符的品項，麻煩選一下：",
	})
	require.Len(t, got, 1)
	require.Equal(t, BlockNextStep, got[0].Kind)
	require.Contains(t, got[0].Text, "找到幾個相符的品項，麻煩選一下：")
}

func TestRenderPlan_ItemNeedsDisambiguation_FallsBackWhenNoPrompt(t *testing.T) {
	got := RenderPlan(DialoguePlanV1{ActCode: ActItemNeedsDisambiguation}, SafeRenderArgs{})
	require.Len(t, got, 1)
	require.NotEmpty(t, got[0].Text)
}

func TestRenderPlan_TextFallback_MatchesJoinedBlocksForEveryPlannableAct(t *testing.T) {
	for _, code := range PlannableActCodes {
		blocks := RenderPlan(DialoguePlanV1{ActCode: code}, SafeRenderArgs{
			MissingRequiredLabels: []string{"配送城市"},
			SalientFactLabels:     []string{"品項：寶島梨"},
			NextQuestionPrompt:    "需要指定包裝方式嗎？",
			RejectedLabels:        []string{"配送城市：不在服務範圍"},
		})
		fallback := TextFallback(blocks)
		for _, b := range blocks {
			require.Contains(t, fallback, b.Text, "TextFallback for %s is missing block text %q", code, b.Text)
		}
	}
}
