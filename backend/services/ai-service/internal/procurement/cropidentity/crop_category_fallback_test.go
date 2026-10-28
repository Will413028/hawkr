package cropidentity

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"github.com/stretchr/testify/require"
)

type categoryTreeFake struct {
	response *categorypb.GetCategoryTreeResponse
	err      error
	calls    int
	status   string
}

func (f *categoryTreeFake) GetCategoryTree(_ context.Context, status string) (*categorypb.GetCategoryTreeResponse, error) {
	f.calls++
	f.status = status
	return f.response, f.err
}

type categorySuggestFake struct {
	ids []string

	idsByStage map[string][]string
	err        error
	calls      int
	stages     []string
	allowed    []cropCategory
	queries    []string
	before     func()
}

func (f *categorySuggestFake) Suggest(_ context.Context, query string, stage, _ string, allowed []cropCategory) ([]string, error) {
	f.calls++
	f.stages = append(f.stages, stage)
	f.queries = append(f.queries, query)
	f.allowed = allowed
	if f.before != nil {
		f.before()
	}
	if got, ok := f.idsByStage[stage]; ok {
		return got, f.err
	}
	return f.ids, f.err
}

func porkCategoryTree() *categoryTreeFake {
	return &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "100", Name: "肉類", Status: codemeta.CommonStatusActive.Value, Children: []*categorypb.CategoryNode{
			{Id: "200", Name: "豬肉", ParentId: "100", Status: codemeta.CommonStatusActive.Value, Children: []*categorypb.CategoryNode{
				{Id: "301", Name: "五花肉", ParentId: "200", Status: codemeta.CommonStatusActive.Value},
				{Id: "302", Name: "梅花肉", ParentId: "200", Status: codemeta.CommonStatusActive.Value},
				{Id: "303", Name: "停售肉", ParentId: "200", Status: codemeta.CommonStatusInactive.Value},
			}},
			{Id: "201", Name: "牛肉", ParentId: "100", Status: codemeta.CommonStatusActive.Value, Children: []*categorypb.CategoryNode{
				{Id: "401", Name: "牛腩", ParentId: "201", Status: codemeta.CommonStatusActive.Value},
			}},
		}},
	}}}
}

func newCategoryResolver(model *categorySuggestFake) *DisambiguationUseCase {
	uc := NewDisambiguationUseCase(porkCategoryTree(), nil)
	if model != nil {
		uc.categoryModel = model
	}
	return uc
}

func TestCropCategoryFallback_GroupToItemThenManualDescriptionPreservesCategory(t *testing.T) {
	model := &categorySuggestFake{ids: []string{"301"}}
	uc := newCategoryResolver(model)
	group := &domain.CropSelection{Stage: domain.CropSelectionGroup}
	options := []domain.CropDisambiguationOption{{Label: "豬肉", CategoryID: "200", CategoryLevel: 2}}
	item, err := uc.ResolveSelection(context.Background(), "不知名部位", group, options, "category-200", "")
	require.NoError(t, err)
	require.True(t, item.NeedsDisambiguation)
	require.Equal(t, domain.CropSelectionItem, item.Selection.Stage)
	require.Equal(t, "豬肉", item.Selection.ParentLabel)
	require.Len(t, model.allowed, 2)
	for _, c := range model.allowed {
		require.Equal(t, "200", c.ParentID)
		require.Equal(t, 3, c.Level)
	}
	require.Equal(t, "五花肉", item.Candidates[0].Label)

	final, err := uc.ResolveSelection(context.Background(), "臺灣豬五花", item.Selection, item.Candidates, "category-301", "")
	require.NoError(t, err)
	require.True(t, final.Unavailable)
	require.Equal(t, domain.CropSelectionDescribe, final.Selection.Stage)
	require.Equal(t, "200", final.Selection.ParentCategoryID)
	require.Equal(t, "301", final.Selection.CategoryID)
	require.Equal(t, "五花肉", final.Selection.Label)
	require.Equal(t, 1, model.calls)
}

func TestCropCategoryFallback_TypedGroupAndItemRespectStage(t *testing.T) {
	uc := newCategoryResolver(&categorySuggestFake{})
	group, err := uc.ResolveSelection(context.Background(), "五花肉", &domain.CropSelection{Stage: domain.CropSelectionGroup}, nil, "", "豬肉")
	require.NoError(t, err)
	require.Equal(t, domain.CropSelectionGroup, group.Selection.Stage)
	require.Equal(t, "200", group.Candidates[0].CategoryID)
	itemStage := &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"}
	item, err := uc.ResolveSelection(context.Background(), "五花肉", itemStage, nil, "", "五花肉")
	require.NoError(t, err)
	require.Equal(t, domain.CropSelectionItem, item.Selection.Stage)
	require.Equal(t, "301", item.Candidates[0].CategoryID)
	require.Empty(t, item.Selection.CategoryID, "Typed search must wait for explicit confirmation")
	final, err := uc.ResolveSelection(context.Background(), "五花肉", item.Selection, item.Candidates, "category-301", "")
	require.NoError(t, err)
	require.Equal(t, "301", final.Selection.CategoryID)
	require.Equal(t, domain.CropSelectionDescribe, final.Selection.Stage)
	other, err := uc.ResolveSelection(context.Background(), "五花肉", item.Selection, item.Candidates, "", "牛腩")
	require.NoError(t, err)
	require.True(t, other.Unavailable)
	require.Empty(t, other.Selection.CategoryID, "Free text cannot escape the confirmed pork parent")
}

func TestCropCategoryFallback_StaleFabricatedAndCrossParentChoicesNeverAdopted(t *testing.T) {
	uc := newCategoryResolver(nil)
	stage := &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"}
	for _, id := range []string{"401", "303", "999"} {
		result, err := uc.ResolveSelection(context.Background(), "五花肉", stage, []domain.CropDisambiguationOption{{CategoryID: id, CategoryLevel: 3}}, "category-"+id, "")
		require.NoError(t, err)
		require.True(t, result.Unavailable)
		require.Empty(t, result.Selection.CategoryID)
	}
	result, err := uc.ResolveSelection(context.Background(), "五花肉", stage, nil, "category-301", "")
	require.NoError(t, err)
	require.True(t, result.Unavailable, "An ACTIVE ID still must have been an offered option")
}

func stringRef(s string) *string { return &s }

func TestCropCategoryFallback_ProductServiceNumericStatusWire(t *testing.T) {
	tree := &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "3", ParentId: "0", Name: "肉類", Status: "9004", Children: []*categorypb.CategoryNode{
			{Id: "8", ParentId: "3", Name: "豬肉", Status: "9004", Children: []*categorypb.CategoryNode{
				{Id: "24", ParentId: "8", Name: "黑毛豬五花", Status: "9004"},
				{Id: "25", ParentId: "8", Name: "停售豬五花", Status: "9003"},
			}},
		}},
	}}}
	uc := NewDisambiguationUseCase(tree, nil)
	result, err := uc.Resolve(context.Background(), "豬五花")
	require.NoError(t, err)
	require.Equal(t, "active", tree.status)

	require.True(t, result.Unavailable, "%+v", result)
	require.Equal(t, "24", result.Selection.CategoryID)
	require.Equal(t, "8", result.Selection.ParentCategoryID)
}

func TestMatchCropCategories_ConsumerAliasResolvesToCanonicalName(t *testing.T) {
	groups := []cropCategory{
		{ID: "210", Label: "甘藍", Level: 2, Children: []cropCategory{{ID: "311", Label: "初秋甘藍", Level: 3}}},
		{ID: "211", Label: "甘薯", Level: 2},
	}

	for _, tc := range []struct {
		text string
		want string
	}{
		{text: "高麗菜", want: "210"},
		{text: "高麗菜。", want: "210"},
		{text: "地瓜", want: "211"},
		{text: "甘藍", want: "210"},
	} {
		matches := matchCropCategories(tc.text, groups, true)
		require.Len(t, matches, 1, "text=%s", tc.text)
		require.Equal(t, tc.want, matches[0].ID, "text=%s", tc.text)
	}
}

func TestMatchCropCategories_ExactAliasBeatsContainsOnRawText(t *testing.T) {
	groups := []cropCategory{
		{ID: "900", Label: "高麗菜乾", Level: 2},
		{ID: "210", Label: "甘藍", Level: 2},
	}

	matches := matchCropCategories("高麗菜", groups, false)

	require.Len(t, matches, 1)
	require.Equal(t, "210", matches[0].ID, "別名的精確命中應勝過原話的包含命中")
}
