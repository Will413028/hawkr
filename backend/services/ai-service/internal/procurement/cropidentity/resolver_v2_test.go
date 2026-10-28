package cropidentity

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

func v2UC(tree cropCategoryReader, model *categorySuggestFake) *DisambiguationUseCase {
	uc := NewDisambiguationUseCase(tree, nil)
	if model != nil {
		uc.categoryModel = model
	}
	return uc
}

func optionIDs(cands []domain.CropDisambiguationOption) []string {
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		out = append(out, domain.CropDisambiguationOptionID(c))
	}
	return out
}

func TestResolveV2_ExactLeafCategoryAutoResolves(t *testing.T) {
	uc := v2UC(v2CategoryTree(), nil)

	got, err := uc.Resolve(context.Background(), "豬肉")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.True(t, got.Unavailable, "分類確認後不再往下問，由買家確認或自行描述品項")
	require.Equal(t, domain.CropSelectionDescribe, got.Selection.Stage)
	require.Equal(t, "2008", got.Selection.CategoryID)
	require.Equal(t, "200", got.Selection.ParentCategoryID)
}

func TestResolveV2_UniqueWeakCandidateAutoResolves(t *testing.T) {
	uc := v2UC(v2CategoryTree(), nil)

	got, err := uc.Resolve(context.Background(), "生鮮豬")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, "2257", got.Selection.CategoryID)
}

func TestResolveV2_L1NameAsksWhichGroup(t *testing.T) {
	uc := v2UC(v2CategoryTree(), nil)

	got, err := uc.Resolve(context.Background(), "肉類")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-200", "category-210"}, optionIDs(got.Candidates))
	require.Equal(t, 2, got.Candidates[0].CategoryLevel)
	require.Equal(t, "豬肉", got.Candidates[0].Label)
}

func TestResolveV2_TreeDownPropagatesTheError(t *testing.T) {
	model := &categorySuggestFake{ids: []string{"200"}}
	uc := v2UC(&categoryTreeFake{err: errors.New("tree down")}, model)

	_, err := uc.Resolve(context.Background(), "臺灣豬五花")

	require.Error(t, err, "分類樹是唯一的確定性來源，失敗不得被固化成「查無此品項」")
	require.Equal(t, 0, model.calls, "沒有樹就沒有白名單，不得問模型")
}

func TestResolveV2_LLMNotCalledWhenRecallHasCandidates(t *testing.T) {
	model := &categorySuggestFake{ids: []string{"200"}}
	uc := v2UC(v2CategoryTree(), model)

	got, err := uc.Resolve(context.Background(), "雞肉")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, 0, model.calls)
}

func manyLeafTree(n int) *categoryTreeFake {
	active := codemeta.CommonStatusActive.Value
	l2 := &categorypb.CategoryNode{Id: "200", Name: "豬肉", ParentId: "100", Status: active}
	for i := 0; i < n; i++ {
		l2.Children = append(l2.Children, &categorypb.CategoryNode{Id: fmt.Sprintf("3%03d", i), Name: fmt.Sprintf("部位%d", i), ParentId: "200", Status: active})
	}
	return &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "100", Name: "肉類", Status: active, Children: []*categorypb.CategoryNode{l2}},
	}}}
}

func TestResolveV2_CardOptionCapIsPinned(t *testing.T) {
	got, err := v2UC(manyLeafTree(12), nil).Resolve(context.Background(), "豬肉")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Len(t, got.Candidates, cropCategoryMaxOptions, "分類卡上限")
}

func facetTree() *categoryTreeFake {
	active := codemeta.CommonStatusActive.Value
	return &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "100", Name: "肉類", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "200", Name: "豬肉", ParentId: "100", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "2008", Name: "豬肉", ParentId: "200", Status: active},
				{Id: "2257", Name: "生鮮豬肉", ParentId: "200", Status: active},
				{Id: "2301", Name: "豬里肌", ParentId: "200", Status: active},
				{Id: "2302", Name: "豬五花", ParentId: "200", Status: active},
			}},
			{Id: "210", Name: "雞肉", ParentId: "100", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "2011", Name: "雞肉", ParentId: "210", Status: active},
				{Id: "2258", Name: "生鮮雞肉", ParentId: "210", Status: active},
			}},
		}},
	}}}
}

func weakLeaf(id, label, parentID, parentLabel string) recall.Candidate {
	return recall.Candidate{
		Code: id, Label: label,
		CategoryIDs: []string{id}, CategoryLevel: 3, ParentID: parentID, ParentLabel: parentLabel,
		MatchedBy: label, Quality: recall.Weak,
	}
}

func TestResolveV2_FacetDominantGroupSkipsGroupQuestion(t *testing.T) {
	uc := v2UC(facetTree(), nil)
	pool := []recall.Candidate{
		weakLeaf("2008", "豬肉", "200", "豬肉"), weakLeaf("2257", "生鮮豬肉", "200", "豬肉"),
		weakLeaf("2301", "豬里肌", "200", "豬肉"), weakLeaf("2302", "豬五花", "200", "豬肉"),
		weakLeaf("2011", "雞肉", "210", "雞肉"),
	}

	got, err := uc.categoryCard(context.Background(), "肉", pool)

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "200", got.Selection.ParentCategoryID)
	require.Equal(t, []string{"category-2008", "category-2257", "category-2301", "category-2302"}, optionIDs(got.Candidates))
}

func TestResolveV2_FacetBalancedGroupsAskGroupCard(t *testing.T) {
	uc := v2UC(facetTree(), nil)

	got, err := uc.Resolve(context.Background(), "生鮮")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.ElementsMatch(t, []string{"category-200", "category-210"}, optionIDs(got.Candidates))
}

func llmL2(id, label string) recall.Candidate {
	return recall.Candidate{
		Code: id, Label: label,
		CategoryLevel: 2, MatchedBy: label, Quality: recall.Weak,
	}
}

func TestCategoryCard_LLMMultipleGroupsAskGroupCardWithoutExpanding(t *testing.T) {
	model := &categorySuggestFake{}
	uc := v2UC(v2CategoryTree(), model)

	got, err := uc.categoryCard(context.Background(), "臺灣豬五花", []recall.Candidate{llmL2("200", "豬肉"), llmL2("210", "雞肉")})

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, "臺灣豬五花", got.Selection.QueryText)
	require.Equal(t, []string{"category-200", "category-210"}, optionIDs(got.Candidates))
	require.Equal(t, 2, got.Candidates[0].CategoryLevel)
	require.Equal(t, "豬肉", got.Candidates[0].Label)
	require.Equal(t, 0, model.calls)
}

func TestCategoryCard_LLMSingleGroupExpandsToItemCard(t *testing.T) {
	model := &categorySuggestFake{idsByStage: map[string][]string{"ITEM": {"2257"}}}
	uc := v2UC(v2CategoryTree(), model)

	got, err := uc.categoryCard(context.Background(), "臺灣豬五花", []recall.Candidate{llmL2("200", "豬肉")})

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "200", got.Selection.ParentCategoryID)
	require.Equal(t, "豬肉", got.Selection.ParentLabel)
	require.Equal(t, []string{"category-2257"}, optionIDs(got.Candidates))
	require.Equal(t, 3, got.Candidates[0].CategoryLevel)
	require.Equal(t, []string{"ITEM"}, model.stages)
	require.Len(t, model.allowed, 2, "ITEM 那次給模型看的是豬肉的兩個子類")
}

func TestCategoryCard_LLMSingleChildGroupSkipsItemCall(t *testing.T) {
	model := &categorySuggestFake{}
	uc := v2UC(v2CategoryTree(), model)

	got, err := uc.categoryCard(context.Background(), "在來米", []recall.Candidate{llmL2("220", "米")})

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "220", got.Selection.ParentCategoryID)
	require.Equal(t, []string{"category-2022"}, optionIDs(got.Candidates))
	require.Equal(t, 0, model.calls)
}

func TestCategoryCard_LLMGroupWithNoUsableChildIsEscapeWithoutConfirmedCategory(t *testing.T) {
	model := &categorySuggestFake{idsByStage: map[string][]string{"ITEM": nil}}
	uc := v2UC(v2CategoryTree(), model)

	got, err := uc.categoryCard(context.Background(), "臺灣豬五花", []recall.Candidate{llmL2("200", "豬肉")})

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
	id, _ := got.Selection.ConfirmedCategory()
	require.Empty(t, id)
}

func TestCategoryCard_LLMGroupCorroboratesExistingLeavesWithoutDoubleCounting(t *testing.T) {
	uc := v2UC(v2CategoryTree(), &categorySuggestFake{})
	porkLeaf := recall.Candidate{Code: "2008", Label: "豬肉", CategoryIDs: []string{"2008"}, CategoryLevel: 3, ParentID: "200", ParentLabel: "豬肉", Quality: recall.Weak}
	chickenLeaf := recall.Candidate{Code: "2011", Label: "雞肉", CategoryIDs: []string{"2011"}, CategoryLevel: 3, ParentID: "210", ParentLabel: "雞肉", Quality: recall.Weak}

	got, err := uc.categoryCard(context.Background(), "肉", []recall.Candidate{porkLeaf, llmL2("200", "豬肉"), chickenLeaf})

	require.NoError(t, err)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-200", "category-210"}, optionIDs(got.Candidates))
}

func riceTree() *categoryTreeFake {
	active := codemeta.CommonStatusActive.Value
	return &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "600", Name: "穀物", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "212", Name: "米", ParentId: "600", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "2022", Name: "米", ParentId: "212", Status: active},
			}},
		}},
		{Id: "700", Name: "水果", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "710", Name: "荔枝", ParentId: "700", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "843", Name: "糯米", ParentId: "710", Status: active},
			}},
		}},
	}}}
}

func TestResolveV2_RiceSynonymAsksBetweenRiceAndLychee(t *testing.T) {
	uc := v2UC(riceTree(), nil)

	got, err := uc.Resolve(context.Background(), "糯米")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.NotNil(t, got.Selection)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-212", "category-710"}, optionIDs(got.Candidates),
		"人工同義詞 pinned 的米排第一；糯米葉節點仍然上卡，讓真的要荔枝品種的買家選得到")
}

func TestNarrowToGroup_KeepsOnlyCandidatesUnderThatL2(t *testing.T) {
	uc := v2UC(facetTree(), nil)
	pork := recall.Candidate{Code: "2008", Label: "豬肉", CategoryIDs: []string{"2008"}, CategoryLevel: 3, ParentID: "200"}
	chicken := recall.Candidate{Code: "2011", Label: "雞肉", CategoryIDs: []string{"2011"}, CategoryLevel: 3, ParentID: "210"}
	porkLoin := recall.Candidate{Code: "2301", Label: "豬里肌", CategoryIDs: []string{"2301"}, CategoryLevel: 3, ParentID: "200"}
	orphan := recall.Candidate{Code: "9999", Label: "不在樹裡"}

	got := uc.narrowToGroup(context.Background(), []recall.Candidate{pork, chicken, porkLoin, orphan}, "200")

	require.Len(t, got, 2)
	require.Equal(t, []string{"2008", "2301"}, []string{got[0].Code, got[1].Code})
}

func TestUnexpandedGroup_IsAWeakLevelTwoCandidate(t *testing.T) {
	got := unexpandedGroup(cropCategory{ID: "200", Label: "豬肉", Level: 2})
	require.Equal(t, "200", got.Code)
	require.Equal(t, 2, got.CategoryLevel)
	require.Equal(t, recall.Weak, got.Quality)
	require.Empty(t, got.CategoryIDs)
}

func TestResolveV2_L1QueryAsksWhichGroupInsteadOfResolvingASubstringLeaf(t *testing.T) {
	uc := v2UC(fruitTree(), nil)

	got, err := uc.Resolve(context.Background(), "水果")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "打上位詞不該被定案成某個子串命中的品項：%+v", got)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-133", "category-134"}, optionIDs(got.Candidates))
	require.Equal(t, 2, got.Candidates[0].CategoryLevel)
}

func TestResolveV2_EmptyDeterministicRecallEscapesWithoutGuessingNeighbour(t *testing.T) {
	model := &categorySuggestFake{idsByStage: map[string][]string{cropCategoryStageL1: {"100"}, "GROUP": {"200", "210"}}}
	uc := v2UC(v2CategoryTree(), model)

	got, err := uc.Resolve(context.Background(), "松露")

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
	require.Empty(t, model.stages,
		"索引裡沒有精確命中就直接走逃生艙，不讓模型猜一個鄰近分類（松露被當成松茸、土豆被當成馬鈴薯）")
}

func TestResolveV2_EmptyRecallEscapesWithoutModel(t *testing.T) {
	model := &categorySuggestFake{idsByStage: map[string][]string{cropCategoryStageL1: {"100"}, "GROUP": {"200"}, "ITEM": {"2257"}}}
	uc := v2UC(v2CategoryTree(), model)

	got, err := uc.Resolve(context.Background(), "水泥")

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
	require.Empty(t, model.stages, "索引零命中時不再請模型猜分類，直接讓買家自行描述品項")
	id, _ := got.Selection.ConfirmedCategory()
	require.Empty(t, id)
}
