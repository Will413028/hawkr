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
	q := BuildCropDisambiguationQuestion(nil, &CropSelection{Stage: "DESCRIBE"})
	require.False(t, q.AllowFreeText)
	require.Len(t, q.Options, 1)
	require.Equal(t, "找不到符合的品項，請描述你要的品項：", q.Prompt)
	require.Equal(t, "自行描述品項", q.Options[0].Label)
}

func TestCropSelectionQuestion_NoSelectionNoCandidatesAsksForDescription(t *testing.T) {
	q := BuildCropDisambiguationQuestion(nil)
	require.Equal(t, "找不到符合的品項，請描述你要的品項：", q.Prompt)
	require.Equal(t, CropOtherOptionID, q.Options[0].OptionID)
	require.Equal(t, "自行描述品項", q.Options[0].Label)
}

func TestCropSelectionQuestion_DescribeStageRetainsConfirmedParentWithoutPriceWording(t *testing.T) {
	q := BuildCropDisambiguationQuestion(nil, &CropSelection{Stage: CropSelectionDescribe, ParentCategoryID: "8", ParentLabel: "豬肉"})
	require.NotContains(t, q.Prompt, "單價")
	require.False(t, q.AllowFreeText)
}
