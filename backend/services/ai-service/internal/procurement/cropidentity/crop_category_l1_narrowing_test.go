package cropidentity

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

// wideTree 造一棵「單一 L1 就超過模型候選上限」的樹，對應真實 dev 分類樹的
// 花卉（299 個 L2，>256）。
func wideTree(l1ID, l1Label string, n int) []cropCategory {
	out := make([]cropCategory, 0, n)
	for i := range n {
		id := fmt.Sprintf("%s-%d", l1ID, i)
		out = append(out, cropCategory{
			ID: id, Label: "節點" + id, ParentID: l1ID, L1Label: l1Label, Level: 2,
			Children: []cropCategory{{ID: id + "-c", Label: "子" + id, ParentID: id, Level: 3}},
		})
	}
	return out
}

// 兩段收斂：第一次問 L1，第二次只在選中的 L1 底下問 L2。
func TestSuggestCategories_NarrowsByL1BeforePickingGroups(t *testing.T) {
	groups := append(wideTree("1", "蔬菜", 3), wideTree("2", "肉類", 2)...)
	model := &categorySuggestFake{idsByStage: map[string][]string{
		cropCategoryStageL1:       {"2"},
		domain.CropSelectionGroup: {"2-0"},
	}}
	uc := NewDisambiguationUseCase(nil, nil)
	uc.categoryModel = model

	got := uc.suggestCategories(t.Context(), "某種肉", domain.CropSelectionGroup, "", groups)

	require.Len(t, got, 1)
	require.Equal(t, "2-0", got[0].ID)
	require.Equal(t, []string{cropCategoryStageL1, domain.CropSelectionGroup}, model.stages)
	require.Len(t, model.allowed, 2, "第二次只看得到選中 L1 底下的 L2")
	for _, c := range model.allowed {
		require.Equal(t, "2", c.ParentID)
	}
}

// 取 top-2 個 L1 是為了擋階層下鑽的錯誤傳播：L1 猜錯時第二段仍看得到正確答案。
func TestSuggestCategories_SecondL1RescuesAMisclassifiedFirst(t *testing.T) {
	groups := append(wideTree("1", "蔬菜", 2), wideTree("2", "肉類", 2)...)
	model := &categorySuggestFake{idsByStage: map[string][]string{
		cropCategoryStageL1:       {"1", "2"}, // 第一個猜錯
		domain.CropSelectionGroup: {"2-1"},    // 正確答案在第二個 L1 底下
	}}
	uc := NewDisambiguationUseCase(nil, nil)
	uc.categoryModel = model

	got := uc.suggestCategories(t.Context(), "某種肉", domain.CropSelectionGroup, "", groups)

	require.Len(t, got, 1)
	require.Equal(t, "2-1", got[0].ID)
	require.Len(t, model.allowed, 4, "兩個 L1 的 L2 聯集都要進候選")
}

// 超過 cropCategoryMaxL1Picks 的 L1 建議要被截到兩個，否則聯集會無界成長。
func TestSuggestCategories_CapsL1PicksAtTwo(t *testing.T) {
	groups := append(append(wideTree("1", "蔬菜", 1), wideTree("2", "肉類", 1)...), wideTree("3", "水果", 1)...)
	model := &categorySuggestFake{idsByStage: map[string][]string{
		cropCategoryStageL1:       {"1", "2", "3"},
		domain.CropSelectionGroup: {"1-0"},
	}}
	uc := NewDisambiguationUseCase(nil, nil)
	uc.categoryModel = model

	uc.suggestCategories(t.Context(), "隨便", domain.CropSelectionGroup, "", groups)

	require.Len(t, model.allowed, 2, "只取前兩個 L1")
}

// 單一 L1 就超過上限時切塊逐塊問，每塊都不得超過 cropCategoryModelMaxCandidates。
// 切塊而不是截斷：截斷會靜默丟掉正確答案，而模型看不到被丟掉的東西。
func TestSuggestCategories_ChunksOversizedL1WithoutExceedingTheBound(t *testing.T) {
	groups := wideTree("9", "花卉", cropCategoryModelMaxCandidates+50)
	model := &categorySuggestFake{idsByStage: map[string][]string{
		cropCategoryStageL1:       {"9"},
		domain.CropSelectionGroup: {"9-0"},
	}}
	uc := NewDisambiguationUseCase(nil, nil)
	uc.categoryModel = model
	var suppliedSizes []int
	model.before = func() { suppliedSizes = append(suppliedSizes, len(model.allowed)) }

	got := uc.suggestCategories(t.Context(), "某種花", domain.CropSelectionGroup, "", groups)

	require.NotEmpty(t, got)
	require.Equal(t, "9-0", got[0].ID)
	require.Greater(t, len(model.stages), 2, "超過上限要切成多塊")
	for _, n := range suppliedSizes {
		require.LessOrEqual(t, n, cropCategoryModelMaxCandidates,
			"每一塊都必須在硬上限之內——這道上限是斷路器，不該由正常路徑觸發")
	}
}

// 模型回了不在白名單裡的 ID（含跨 L1）一律整批不採用。
func TestSuggestCategories_RejectsIDsOutsideTheSelectedL1(t *testing.T) {
	groups := append(wideTree("1", "蔬菜", 2), wideTree("2", "肉類", 2)...)
	model := &categorySuggestFake{idsByStage: map[string][]string{
		cropCategoryStageL1:       {"2"},
		domain.CropSelectionGroup: {"1-0"}, // 不在選中的 L1 底下
	}}
	uc := NewDisambiguationUseCase(nil, nil)
	uc.categoryModel = model

	require.Empty(t, uc.suggestCategories(t.Context(), "某種肉", domain.CropSelectionGroup, "", groups))
}

// 硬上限被觸發代表收斂邏輯有漏，必須與偶發逾時可分辨——修前兩者共用一行 Warn，
// 正是「候選數超過上限」這個必然發生的錯誤從上線第一天沒人發現的直接原因。
func TestCropCategorySuggestBounds_AreDistinguishableSentinels(t *testing.T) {
	s := &modelCropCategorySuggester{}
	_, err := s.Suggest(t.Context(), "x", domain.CropSelectionGroup, "",
		wideTree("1", "蔬菜", cropCategoryModelMaxCandidates+1))

	require.Error(t, err)
	require.ErrorIs(t, err, errCropCategoryBoundExceeded)
	require.NotErrorIs(t, err, errCropCategoryPromptTooLarge)
	require.False(t, errors.Is(err, context.DeadlineExceeded))
}
