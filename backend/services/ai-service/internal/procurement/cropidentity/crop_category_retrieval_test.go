package cropidentity

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

func TestCropCategoryResume_CorrectedGroupUsesNewQueryForChildren(t *testing.T) {
	uc := v2UC(facetTree(), &categorySuggestFake{})

	group, err := uc.ResolveSelection(context.Background(), "臺灣豬五花", &domain.CropSelection{Stage: domain.CropSelectionGroup}, nil, "", "雞肉")
	require.NoError(t, err)
	require.Equal(t, "雞肉", group.Selection.QueryText)
	require.Equal(t, []string{"category-210"}, optionIDs(group.Candidates))

	item, err := uc.ResolveSelection(context.Background(), "臺灣豬五花", group.Selection, group.Candidates, "category-210", "")
	require.NoError(t, err)
	require.Equal(t, "210", item.Selection.ParentCategoryID)
	require.Equal(t, []string{"category-2011", "category-2258"}, optionIDs(item.Candidates),
		"下一層要用改過的 query（雞肉）去召回，不是原始的臺灣豬五花")
}

func TestCropCategoryResume_GroupChoiceUsesV2Recall(t *testing.T) {
	uc := v2UC(facetTree(), nil)

	got, err := uc.ResolveSelection(context.Background(), "豬", &domain.CropSelection{Stage: domain.CropSelectionGroup, QueryText: "豬"},
		[]domain.CropDisambiguationOption{{CategoryID: "200", CategoryLevel: 2}}, "category-200", "")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "200", got.Selection.ParentCategoryID)
	require.Equal(t, []string{"category-2008", "category-2257", "category-2301", "category-2302"}, optionIDs(got.Candidates))
}

func TestCropCategoryResume_GroupChoiceWithEmptyRecallExpandsThatGroup(t *testing.T) {
	model := &categorySuggestFake{idsByStage: map[string][]string{"ITEM": {"2257"}}}
	uc := v2UC(facetTree(), model)

	got, err := uc.ResolveSelection(context.Background(), "未收錄俗稱", &domain.CropSelection{Stage: domain.CropSelectionGroup, QueryText: "未收錄俗稱"},
		[]domain.CropDisambiguationOption{{CategoryID: "200", CategoryLevel: 2}}, "category-200", "")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, []string{"category-2257"}, optionIDs(got.Candidates))
	require.Equal(t, []string{"ITEM"}, model.stages)
}

func TestCropCategoryResume_ItemChoiceConfirmsDirectly(t *testing.T) {
	uc := v2UC(facetTree(), nil)

	got, err := uc.ResolveSelection(context.Background(), "豬肉", &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"},
		[]domain.CropDisambiguationOption{{CategoryID: "2008", CategoryLevel: 3}}, "category-2008", "")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, "2008", got.Selection.CategoryID)
}

func TestCropCategoryResume_TypedTextUsesV2AndNeverAutoConfirms(t *testing.T) {
	model := &categorySuggestFake{}
	uc := v2UC(facetTree(), model)

	got, err := uc.ResolveSelection(context.Background(), "原品項", &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"}, nil, "", " 豬肉 ")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "豬肉", got.Selection.QueryText)
	require.Equal(t, []string{"category-2008", "category-2257", "category-2301", "category-2302"}, optionIDs(got.Candidates),
		"打的字等於 L2 名時，taxonomy 把整個大類的子項都召回")
	require.Empty(t, got.Selection.CategoryID)
	require.Zero(t, model.calls)
}

func TestCropCategoryResume_TypedTextWithEmptyRecallAsksTheModel(t *testing.T) {
	model := &categorySuggestFake{ids: []string{"2257"}}
	uc := v2UC(facetTree(), model)

	got, err := uc.ResolveSelection(context.Background(), "原品項", &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"}, nil, "", "未收錄俗稱")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, []string{"category-2257"}, optionIDs(got.Candidates))
	require.Empty(t, got.Selection.CategoryID)
	require.Equal(t, 1, model.calls)
}
