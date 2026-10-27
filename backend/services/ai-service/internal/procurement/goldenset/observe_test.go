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

// 已定案時分類來自 Selection.ConfirmedCategory()，不是 CommodityCode
// （後者是 pricebook 的 commodity code，不是分類樹節點 id）。
func TestObserveFrom_ResolvedUsesSelection(t *testing.T) {
	res := cropidentity.DisambiguationResult{
		Label:     "白肉雞（2.0 公斤以上）產地價",
		Selection: &domain.CropSelection{Stage: domain.CropSelectionPrice, CategoryID: "2011", Label: "雞肉"},
	}
	o := ObserveFrom(Query{Text: "白肉雞"}, res, pathStub)
	require.Equal(t, []string{"肉類 > 雞肉 > 雞肉"}, o.RankedCategories)
	require.False(t, o.IsNIL)
	require.False(t, o.Unmappable)
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

// 🔴 crop 路徑定案時沒有 CategoryID（pricebook 命中不經分類樹）⇒ 映射不到分類
// 路徑。那是**評測工具的限制**，不是解析錯了，必須與「解析錯誤」分開，否則
// baseline 會把正確的 crop 解析誤記成失敗。
func TestObserveFrom_CropResolutionIsUnmappable(t *testing.T) {
	code := "LA1"
	res := cropidentity.DisambiguationResult{CropCode: &code, Label: "甘藍-初秋"}
	o := ObserveFrom(Query{Text: "高麗菜"}, res, pathStub)
	require.True(t, o.Unmappable, "有解析結果但帶不出分類路徑")
	require.Equal(t, "甘藍-初秋", o.ResolvedLabel)
	require.Empty(t, o.RankedCategories)
	require.False(t, o.IsNIL, "它不是逃生艙——確實解析出東西了")
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
