package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/stretchr/testify/require"
)

func cropOpt(code, name string, hasForecast bool) pbusecase.CropCatalogMatch {
	c, n := code, name
	return pbusecase.CropCatalogMatch{
		MatchType: "crop", GroupCode: "N04-001", GroupName: "甘藍",
		CropCode: &c, CropName: &n, HasForecast: hasForecast,
	}
}

func TestDisambiguationUseCase_DrilledDown_CandidatesReachInterruptPayload(t *testing.T) {
	client := &testkit.FakePricebookClient{
		PriceableMatches: []pbusecase.PriceableEntityMatch{
			{Kind: "CROP_GROUP", Code: "N04-001", Name: "甘藍", MatchedForm: "甘藍", FormKind: "CANONICAL", Tier: 0, Rank: 1, GroupCode: "N04-001", HasForecast: true},
		},
		GroupCrops: []pbusecase.CropCatalogMatch{
			cropOpt("LA1", "甘藍-初秋", true),
			cropOpt("LA2", "甘藍-改良種", true),
		},
	}
	uc := cropidentity.NewDisambiguationUseCase(client, nil, nil)

	res, err := uc.Resolve(context.Background(), "高麗菜")
	require.NoError(t, err)
	require.True(t, res.NeedsDisambiguation)

	payloadJSON, err := json.Marshal(CropDisambiguationInterruptPayload{
		Kind:          cropDisambiguationInterruptKind,
		PendingReason: domain.CropPendingReasonAmbiguous,
		Candidates:    res.Candidates,
	})
	require.NoError(t, err)

	var decoded CropDisambiguationInterruptPayload
	require.NoError(t, json.Unmarshal(payloadJSON, &decoded))
	require.Equal(t, domain.CropPendingReasonAmbiguous, decoded.PendingReason)
	require.Len(t, decoded.Candidates, 3, "兩個子品項 + 一個「整個大類都可以」")
	require.Equal(t, "LA1", *decoded.Candidates[0].CropCode)
}

func categoryResumeQuestion(stage string) *CropDisambiguationInterruptPayload {
	return &CropDisambiguationInterruptPayload{
		Kind: cropDisambiguationInterruptKind, Selection: &domain.CropSelection{Stage: stage},
		Candidates: []domain.CropDisambiguationOption{{Label: "豬肉", CategoryID: "8", CategoryLevel: 2}},
	}
}

func TestCategoryResumeInput_ValidatesBeforeConsumingInterrupt(t *testing.T) {
	for _, tc := range []struct {
		name, stage, optionID, text, price string
		valid                              bool
	}{
		{name: "broad choice", stage: "GROUP", optionID: "category-8", valid: true},
		{name: "broad typed", stage: "GROUP", text: "豬肉", valid: true},
		{name: "narrow typed", stage: "ITEM", text: "五花肉", valid: true},
		{name: "unknown option", stage: "GROUP", optionID: "category-forged"},
		{name: "blank retry", stage: "GROUP", text: "  "},
		{name: "two answers", stage: "GROUP", optionID: "category-8", text: "牛肉"},
		{name: "price on retry", stage: "ITEM", text: "五花肉", price: "220"},
		{name: "retry after exhausted", stage: "PRICE", text: "五花肉"},
		{name: "manual missing price", stage: "PRICE", optionID: domain.CropOtherOptionID, text: "臺灣豬五花"},
		{name: "manual price", stage: "PRICE", optionID: domain.CropOtherOptionID, text: "臺灣豬五花", price: "220", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := AnswerInput{PendingCrop: categoryResumeQuestion(tc.stage), OptionID: tc.optionID, FreeText: tc.text, SelfEnteredPriceAmount: tc.price}
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
			body, err := json.Marshal(cropDisambiguationResponseV1{OptionID: tc.optionID, FreeText: tc.text, SelfEnteredPriceAmount: tc.price})
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

func TestCategoryManualPrice_PreservesConfirmedCategoryWithoutPricingCodes(t *testing.T) {
	question := categoryResumeQuestion("PRICE")
	question.Selection.CategoryID = "24"
	question.Selection.Label = "豬肉／五花肉"
	out, err := resolveResumedCropGate(t.Context(), nil, domain.Slots{Item: item("臺灣豬五花")}, AnswerInput{
		PendingCrop: question, OptionID: domain.CropOtherOptionID, FreeText: "臺灣豬五花", SelfEnteredPriceAmount: "220",
	})
	require.NoError(t, err)
	require.False(t, out.Pending)
	require.Equal(t, "24", out.Resolution.CategoryID)
	require.Equal(t, domain.ItemResolvedSelfPriced, out.Resolution.Kind)
	require.Nil(t, out.Resolution.Code)
	require.Equal(t, 220, *out.Resolution.PricePerKg)
}

func TestCategoryManualPrice_PreservesBroadCategoryWhenNoChildMatches(t *testing.T) {
	for _, stage := range []string{domain.CropSelectionItem, domain.CropSelectionPrice} {
		t.Run(stage, func(t *testing.T) {
			question := categoryResumeQuestion(stage)
			question.Selection.ParentCategoryID = "8"
			question.Selection.ParentLabel = "豬肉"
			out, err := resolveResumedCropGate(t.Context(), nil, domain.Slots{Item: item("臺灣豬五花")}, AnswerInput{
				PendingCrop: question, OptionID: domain.CropOtherOptionID, FreeText: "臺灣豬五花", SelfEnteredPriceAmount: "220",
			})
			require.NoError(t, err)
			require.Equal(t, "8", out.Resolution.CategoryID)
			require.Equal(t, "豬肉", out.Resolution.Label)
			require.Nil(t, out.Resolution.Code)
			require.Equal(t, domain.ItemResolvedSelfPriced, out.Resolution.Kind)
		})
	}
}

func TestCategoryManualPrice_DoesNotClassifyReplacementDescriptionAsOriginalItem(t *testing.T) {
	for _, tc := range []struct {
		name        string
		source      *string
		description string
		keep        bool
	}{
		{name: "unchanged", source: item("臺灣豬五花"), description: "臺灣豬五花", keep: true},
		{name: "normalized spelling and punctuation", source: item("臺灣豬五花"), description: " 台灣 豬五花。 ", keep: true},
		{name: "different product", source: item("臺灣豬五花"), description: "有機蘋果"},
		{name: "substantive refinement needs confirmation", source: item("臺灣豬五花"), description: "臺灣豬五花（去皮）"},
		{name: "canonical label is still an edited description", source: item("臺灣豬五花"), description: "五花肉"},
		{name: "missing source", description: "臺灣豬五花"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, stage := range []string{domain.CropSelectionItem, domain.CropSelectionPrice} {
				t.Run(stage, func(t *testing.T) {
					question := categoryResumeQuestion(stage)
					question.Selection.ParentCategoryID, question.Selection.ParentLabel = "8", "豬肉"
					categoryID, label := "8", "豬肉"
					if stage == domain.CropSelectionPrice {
						question.Selection.CategoryID, question.Selection.Label = "24", "五花肉"
						categoryID, label = "24", "五花肉"
					}
					originalSelection := *question.Selection
					out, err := resolveResumedCropGate(t.Context(), nil, domain.Slots{Item: tc.source}, AnswerInput{
						PendingCrop: question, OptionID: domain.CropOtherOptionID,
						FreeText: tc.description, SelfEnteredPriceAmount: "220",
					})
					require.NoError(t, err, "manual description edits remain supported")
					require.False(t, out.Pending)
					require.Equal(t, strings.TrimSpace(tc.description), *out.Item)
					require.Equal(t, domain.ItemResolvedSelfPriced, out.Resolution.Kind)
					require.Equal(t, 220, *out.Resolution.PricePerKg)
					require.Nil(t, out.Resolution.Code)
					require.Equal(t, domain.ItemResolutionByBuyerChoice, out.Resolution.Provenance)
					if tc.keep {
						require.Equal(t, categoryID, out.Resolution.CategoryID)
						require.Equal(t, label, out.Resolution.Label)
					} else {
						require.Empty(t, out.Resolution.CategoryID, "a prior confirmation does not classify edited item text")
						require.Empty(t, out.Resolution.Label, "the old category label must not appear for the replacement")
					}
					require.Equal(t, originalSelection, *question.Selection, "the stored question remains immutable")
				})
			}
		})
	}
}
