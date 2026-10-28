package cropidentity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

func TestSplitItems_SplitsOnConjunctionsWhenBothSidesAreSubstantial(t *testing.T) {
	require.Equal(t, []string{"高麗菜", "紅蘿蔔"}, splitItems("高麗菜和紅蘿蔔"))
	require.Equal(t, []string{"高麗菜", "紅蘿蔔"}, splitItems("高麗菜、紅蘿蔔"))
	require.Equal(t, []string{"白蝦", "虱目魚"}, splitItems("白蝦與虱目魚"))
}

func TestSplitItems_KeepsSingleItemNamesThatContainAConjunction(t *testing.T) {
	for _, name := range []string{"和牛", "大和牛", "台灣和牛", "蘋果和梨"} {
		require.Equal(t, []string{name}, splitItems(name),
			"寧可漏切也不能誤切——誤切會把一個真實品名拆成兩個不存在的品項")
	}
}

func TestResolveV2_MultiItemAsksInsteadOfSilentlyKeepingOne(t *testing.T) {
	uc := v2UC(testkit.VegetableTree(), nil)

	got, err := uc.Resolve(context.Background(), "高麗菜和地瓜")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.ElementsMatch(t, []string{"category-4", "category-5"}, optionIDs(got.Candidates),
		"整串查不到任何分類，切開後兩段各自查得到 ⇒ 兩個都要上卡讓買家挑")
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
}

func TestResolveV2_MultiItemWithoutSplitWouldFindNothing(t *testing.T) {
	uc := v2UC(testkit.VegetableTree(), nil)

	got, err := uc.Resolve(context.Background(), "高麗菜和紅蘿蔔")

	require.NoError(t, err)
	require.Equal(t, domain.CropSelectionDescribe, got.Selection.Stage)
	require.Equal(t, "17", got.Selection.CategoryID,
		"只有一段查得到時仍要走切開後的召回——沒切就是零命中，這條證明切法真的有作用")
}

func TestResolveV2_WholeTextIsARealCategoryNameSoItIsNotSplit(t *testing.T) {
	active := codemeta.CommonStatusActive.Value
	tree := &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "1", Name: "花卉", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "9", Name: "蘭花", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "901", Name: "虎頭蘭-中、小輪", ParentId: "9", Status: active},
			}},
		}},
	}}}
	uc := v2UC(tree, nil)

	got, err := uc.Resolve(context.Background(), "虎頭蘭-中、小輪")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, "901", got.Selection.CategoryID,
		"整串就是分類樹裡的真實名稱，不能因為中間有頓號就被當成兩個品項")
}
