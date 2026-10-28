package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/stretchr/testify/require"
)

func TestDisambiguationUseCase_CategoryCandidates_ReachInterruptPayload(t *testing.T) {
	uc := cropidentity.NewDisambiguationUseCase(testkit.VegetableTree(), nil)

	res, err := uc.Resolve(context.Background(), "蔬菜")
	require.NoError(t, err)
	require.True(t, res.NeedsDisambiguation)

	payloadJSON, err := json.Marshal(CropDisambiguationInterruptPayload{
		Kind:          cropDisambiguationInterruptKind,
		PendingReason: domain.CropPendingReasonAmbiguous,
		Candidates:    res.Candidates,
		Selection:     res.Selection,
	})
	require.NoError(t, err)

	var decoded CropDisambiguationInterruptPayload
	require.NoError(t, json.Unmarshal(payloadJSON, &decoded))
	require.Equal(t, domain.CropPendingReasonAmbiguous, decoded.PendingReason)
	require.Len(t, decoded.Candidates, 2, "葉菜類與根莖類兩個中層分類")
	require.Equal(t, "4", decoded.Candidates[0].CategoryID)
	require.Equal(t, 2, decoded.Candidates[0].CategoryLevel)
	require.Equal(t, domain.CropSelectionGroup, decoded.Selection.Stage)
}

func categoryResumeQuestion(stage string) *CropDisambiguationInterruptPayload {
	return &CropDisambiguationInterruptPayload{
		Kind: cropDisambiguationInterruptKind, Selection: &domain.CropSelection{Stage: stage},
		Candidates: []domain.CropDisambiguationOption{{Label: "豬肉", CategoryID: "8", CategoryLevel: 2}},
	}
}

func TestCategoryResumeInput_ValidatesBeforeConsumingInterrupt(t *testing.T) {
	for _, tc := range []struct {
		name, stage, optionID, text string
		valid                       bool
	}{
		{name: "broad choice", stage: "GROUP", optionID: "category-8", valid: true},
		{name: "broad typed", stage: "GROUP", text: "豬肉", valid: true},
		{name: "narrow typed", stage: "ITEM", text: "五花肉", valid: true},
		{name: "unknown option", stage: "GROUP", optionID: "category-forged"},
		{name: "blank retry", stage: "GROUP", text: "  "},
		{name: "two answers", stage: "GROUP", optionID: "category-8", text: "牛肉"},
		{name: "retry after exhausted", stage: "DESCRIBE", text: "五花肉"},
		{name: "manual missing description", stage: "DESCRIBE", optionID: domain.CropOtherOptionID, text: "  "},
		{name: "manual description", stage: "DESCRIBE", optionID: domain.CropOtherOptionID, text: "臺灣豬五花", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := AnswerInput{PendingCrop: categoryResumeQuestion(tc.stage), OptionID: tc.optionID, FreeText: tc.text}
			err := validateCropResumeAnswer(in)
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrInvalidOptionID)
			store := newFakeResumeStore()
			store.resolveCtx = resumeCropInterruptCtx(4242, 77)
			store.resolveCtx.InterruptPayload, err = json.Marshal(in.PendingCrop)
			require.NoError(t, err)
			body, err := json.Marshal(cropDisambiguationResponseV1{OptionID: tc.optionID, FreeText: tc.text})
			require.NoError(t, err)
			_, err = ClaimAndDispatchResume(t.Context(), ResumeInput{BuyerID: 1234, InterruptID: "category-int", ClientRequestID: tc.name, ResponsePayload: body}, store, NewRunDispatcher(), GraphControllerDeps{})
			require.ErrorIs(t, err, ErrInvalidOptionID)
			require.Zero(t, store.lastResolved)
			require.Empty(t, store.terminals)
		})
	}
}

type stageResumeResolver struct {
	testkit.FakeResolver[cropidentity.DisambiguationResult]
	result  cropidentity.DisambiguationResult
	gotText string
}

func (r *stageResumeResolver) ResolveSelection(_ context.Context, text string, _ *domain.CropSelection, _ []domain.CropDisambiguationOption, _, _ string) (cropidentity.DisambiguationResult, error) {
	r.gotText = text
	return r.result, nil
}

func TestCategoryResumeOutput_KeepsOriginalItemAndReparksNextStage(t *testing.T) {
	r := &stageResumeResolver{result: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Selection:           &domain.CropSelection{Stage: "ITEM", ParentCategoryID: "8", ParentLabel: "豬肉"},
		Candidates:          []domain.CropDisambiguationOption{{CategoryID: "24", CategoryLevel: 3, Label: "五花肉"}},
	}}
	out, err := resolveResumedCropGate(t.Context(), r, domain.Slots{Item: item("臺灣豬五花")}, AnswerInput{PendingCrop: categoryResumeQuestion("GROUP"), OptionID: "category-8"})
	require.NoError(t, err)
	require.Equal(t, "臺灣豬五花", r.gotText)
	require.Equal(t, "臺灣豬五花", *out.Item)
	require.True(t, out.Pending)
	require.Equal(t, "ITEM", out.Selection.Stage)
	require.Nil(t, out.Resolution)
	require.Equal(t, "category-24", domain.CropDisambiguationOptionID(out.Candidates[0]))
}

func TestCategoryManualDescription_NoConfirmedCategory_RecordsBuyerDescribed(t *testing.T) {
	for _, tc := range []struct {
		name        string
		source      *string
		description string
	}{
		{name: "unchanged", source: item("臺灣豬五花"), description: "臺灣豬五花"},
		{name: "replacement", source: item("臺灣豬五花"), description: "有機蘋果"},
		{name: "trimmed", source: item("臺灣豬五花"), description: "  臺灣豬五花（去皮） "},
		{name: "missing source", description: "臺灣豬五花"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			question := categoryResumeQuestion(domain.CropSelectionGroup)
			originalSelection := *question.Selection
			out, err := resolveResumedCropGate(t.Context(), nil, domain.Slots{Item: tc.source}, AnswerInput{
				PendingCrop: question, OptionID: domain.CropOtherOptionID, FreeText: tc.description,
			})
			require.NoError(t, err)
			require.False(t, out.Pending)
			require.Equal(t, strings.TrimSpace(tc.description), *out.Item)
			require.Equal(t, &domain.ItemResolution{
				Kind: domain.ItemResolvedBuyerDescribed, Label: strings.TrimSpace(tc.description),
			}, out.Resolution)
			require.Equal(t, originalSelection, *question.Selection, "the stored question remains immutable")
		})
	}
}

func TestCategoryManualDescription_ConfirmedParent_FallsBackToParentCategory(t *testing.T) {
	for _, stage := range []string{domain.CropSelectionItem, domain.CropSelectionDescribe} {
		t.Run(stage, func(t *testing.T) {
			question := categoryResumeQuestion(stage)
			question.Selection.ParentCategoryID, question.Selection.ParentLabel = "8", "豬肉"
			originalSelection := *question.Selection
			out, err := resolveResumedCropGate(t.Context(), nil, domain.Slots{Item: item("臺灣豬五花")}, AnswerInput{
				PendingCrop: question, OptionID: domain.CropOtherOptionID, FreeText: " 臺灣豬五花（去皮） ",
			})
			require.NoError(t, err)
			require.False(t, out.Pending)
			require.Equal(t, &domain.ItemResolution{
				Kind: domain.ItemResolvedCategoryConfirmed, Label: "臺灣豬五花（去皮）", CategoryID: "8",
			}, out.Resolution)
			require.Equal(t, originalSelection, *question.Selection, "the stored question remains immutable")
		})
	}
}

type describeLoopResolver struct{ calls int }

func (r *describeLoopResolver) Resolve(context.Context, string) (cropidentity.DisambiguationResult, error) {
	r.calls++
	return cropidentity.DisambiguationResult{
		Unavailable: true, Selection: &domain.CropSelection{Stage: domain.CropSelectionDescribe},
	}, nil
}

func TestBuyerDescribedItem_NextTurnDoesNotAskToDescribeAgain(t *testing.T) {
	resolver := &describeLoopResolver{}
	now := time.Now()

	turn1 := cropidentity.ComputeGate(t.Context(), resolver, domain.Slots{}, domain.Slots{Item: item("查無此物")})
	require.True(t, turn1.Pending)
	require.Equal(t, domain.CropPendingReasonUnavailable, turn1.PendingReason)

	pending := &CropDisambiguationInterruptPayload{
		Kind: cropDisambiguationInterruptKind, PendingReason: turn1.PendingReason,
		Candidates: turn1.Candidates, Selection: turn1.Selection,
	}
	resumed, err := resolveResumedCropGate(t.Context(), resolver, domain.Slots{Item: item("查無此物")}, AnswerInput{
		PendingCrop: pending, OptionID: domain.CropOtherOptionID, FreeText: "野生黑木耳",
	})
	require.NoError(t, err)
	patch := domain.SlotPatch{Item: resumed.Item, ItemResolution: resumed.Resolution}
	recorded, rejected := domain.Slots{Item: item("查無此物")}.Apply(patch, now)
	require.Empty(t, rejected)
	require.NotNil(t, recorded.ItemResolution)

	callsBefore := resolver.calls
	turn2 := cropidentity.ComputeGate(t.Context(), resolver, recorded, recorded)
	require.False(t, turn2.Pending, "買家已描述過品項，下一輪不得再出『找不到符合的品項』卡")
	require.Equal(t, callsBefore, resolver.calls, "已有解析結果就不再查分類樹")
}

func TestCategoryResumeOutput_LeafSelectionPassesWithResolution(t *testing.T) {
	r := &stageResumeResolver{result: cropidentity.DisambiguationResult{
		Unavailable: true,
		Selection:   &domain.CropSelection{Stage: domain.CropSelectionDescribe, ParentCategoryID: "8", ParentLabel: "豬肉", CategoryID: "24", Label: "五花肉"},
	}}
	out, err := resolveResumedCropGate(t.Context(), r, domain.Slots{Item: item("臺灣豬五花")}, AnswerInput{PendingCrop: categoryResumeQuestion("ITEM"), OptionID: "category-8"})
	require.NoError(t, err)
	require.False(t, out.Pending)
	require.Nil(t, out.Selection)
	require.Equal(t, domain.ItemResolvedCategoryConfirmed, out.Resolution.Kind)
	require.Equal(t, "24", out.Resolution.CategoryID)
	require.Equal(t, "五花肉", out.Resolution.Label)
}

func TestCategoryResumeOutput_NoLeafSelectionAsksBuyerToDescribe(t *testing.T) {
	r := &stageResumeResolver{result: cropidentity.DisambiguationResult{
		Unavailable: true,
		Selection:   &domain.CropSelection{Stage: domain.CropSelectionDescribe, ParentCategoryID: "8", ParentLabel: "豬肉"},
	}}
	out, err := resolveResumedCropGate(t.Context(), r, domain.Slots{Item: item("臺灣豬五花")}, AnswerInput{PendingCrop: categoryResumeQuestion("ITEM"), OptionID: "category-8"})
	require.NoError(t, err)
	require.True(t, out.Pending)
	require.Equal(t, domain.CropPendingReasonUnavailable, out.PendingReason)
	require.Nil(t, out.Resolution)
}
