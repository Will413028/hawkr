package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCropSelectionQuestion_StagesAndOpaqueCategoryIDs(t *testing.T) {
	for _, stage := range []string{"GROUP", "ITEM"} {
		t.Run(stage, func(t *testing.T) {
			q := BuildCropDisambiguationQuestion([]CropDisambiguationOption{
				{Label: "豬肉", CategoryID: "8", CategoryLevel: 2},
			}, &CropSelection{Stage: stage, ParentLabel: "豬肉"})
			require.True(t, q.AllowFreeText)
			require.Equal(t, "category-8", q.Options[0].OptionID)
			require.Equal(t, "豬肉", q.Options[0].Value)
			require.Equal(t, CropOtherOptionID, q.Options[len(q.Options)-1].OptionID)
			require.False(t, q.CanSkip)
		})
	}
	q := BuildCropDisambiguationQuestion(nil, &CropSelection{Stage: "PRICE", CategoryID: "24", Label: "五花肉"})
	require.False(t, q.AllowFreeText)
	require.Len(t, q.Options, 1)
	require.Contains(t, q.Prompt, "五花肉")
	require.NotContains(t, q.Prompt, "找不到吻合的品項分類")
}

func TestSelfPricedClassification_IsRetainedWithoutPricebookCode(t *testing.T) {
	r := newSelfPricedForTest(220)
	r.CategoryID = "24"
	r.Label = "豬肉／五花肉"
	require.Empty(t, validateItemResolution(*r))
	require.Contains(t, r.DisplayValue(), "豬肉／五花肉")
	require.Contains(t, r.DisplayValue(), "220")
	crop, group := r.PricebookQueryCodes()
	require.Nil(t, crop)
	require.Nil(t, group)
	other := *r
	other.CategoryID = "25"
	require.False(t, r.Equal(&other))
}

func TestCropSelectionQuestion_PriceRetainsConfirmedParent(t *testing.T) {
	q := BuildCropDisambiguationQuestion(nil, &CropSelection{Stage: CropSelectionPrice, ParentCategoryID: "8", ParentLabel: "豬肉"})
	require.Contains(t, q.Prompt, "已確認品項「豬肉」")
	require.False(t, q.AllowFreeText)
}
