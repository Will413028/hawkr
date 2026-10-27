package goldenset

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

func pathStub(id string) string {
	return map[string]string{
		"2011": "肉類 > 雞肉 > 雞肉",
		"767":  "水果 > 葡萄柚 > 白肉",
		"198":  "肉類 > 豬肉",
	}[id]
}

// 品項定到分類樹節點後 Resolve 回 Unavailable（下一步是填單價），分類來自
// Selection.ConfirmedCategory()。
func TestObserveFrom_ConfirmedCategoryIsRankedFirst(t *testing.T) {
	res := cropidentity.DisambiguationResult{
		Unavailable: true,
		Selection:   &domain.CropSelection{Stage: domain.CropSelectionPrice, CategoryID: "2011", Label: "雞肉"},
	}
	o := ObserveFrom(Query{Text: "白肉雞"}, res, pathStub)
	require.Equal(t, []string{"肉類 > 雞肉 > 雞肉"}, o.RankedCategories)
	require.False(t, o.IsNIL)
	require.False(t, o.Unmappable)
}

func TestObserveFrom_ConfirmedCategoryOutsidePathMapIsUnmappable(t *testing.T) {
	res := cropidentity.DisambiguationResult{
		Unavailable: true,
		Selection:   &domain.CropSelection{Stage: domain.CropSelectionPrice, CategoryID: "9999", Label: "未知"},
	}
	o := ObserveFrom(Query{Text: "未知"}, res, pathStub)
	require.True(t, o.Unmappable)
	require.False(t, o.IsNIL)
}

// 候選的欄位是 CategoryID（不帶 "category-" 前綴，那是 wire 層才加的）。
func TestObserveFrom_CandidatesKeepOrderAsRank(t *testing.T) {
	res := cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{CategoryID: "767", CategoryLevel: 3, Label: "白肉"},
			{CategoryID: "2011", CategoryLevel: 3, Label: "雞肉"},
		},
	}
	o := ObserveFrom(Query{Text: "白肉雞"}, res, pathStub)
	require.Equal(t, []string{"水果 > 葡萄柚 > 白肉", "肉類 > 雞肉 > 雞肉"}, o.RankedCategories)
	require.False(t, o.Unmappable)
}

func TestObserveFrom_CandidatesWithoutCategoryIDAreUnmappable(t *testing.T) {
	res := cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "甘藍-初秋"},
			{Label: "甘藍-改良種"},
		},
	}
	o := ObserveFrom(Query{Text: "高麗菜"}, res, pathStub)
	require.True(t, o.Unmappable)
	require.Empty(t, o.RankedCategories)
}

func TestObserveFrom_UnavailableIsNIL(t *testing.T) {
	o := ObserveFrom(Query{Text: "鑽石"}, cropidentity.DisambiguationResult{Unavailable: true}, pathStub)
	require.True(t, o.IsNIL)
	require.False(t, o.Unmappable)
	require.Empty(t, o.RankedCategories)
}
