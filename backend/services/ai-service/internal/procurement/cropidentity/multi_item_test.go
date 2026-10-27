package cropidentity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
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
	pb := &perVariantPricebookFake{byVariant: map[string][]pbusecase.PriceableEntityMatch{
		"高麗菜": {pm("CROP", "LA1", "甘藍-初秋", "高麗菜", "ALIAS", 0, 1, []string{"533"}, "G-CAB", true)},
		"紅蘿蔔": {pm("CROP", "SD5", "蘿蔔-紅蘿蔔", "紅蘿蔔", "ALIAS", 0, 1, []string{"601"}, "G-RAD", true)},
	}}
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, nil)
	uc.pricebook = pb

	got, err := uc.Resolve(context.Background(), "高麗菜和紅蘿蔔")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.ElementsMatch(t, []string{"crop-LA1", "crop-SD5"}, optionIDs(got.Candidates),
		"整串查不到任何實體，切開後兩段各自查得到 ⇒ 兩個都要上卡讓買家挑")
}

func TestResolveV2_MultiItemWithoutSplitWouldFindNothing(t *testing.T) {
	pb := &perVariantPricebookFake{byVariant: map[string][]pbusecase.PriceableEntityMatch{
		"高麗菜": {pm("CROP", "LA1", "甘藍-初秋", "高麗菜", "ALIAS", 0, 1, []string{"533"}, "G-CAB", true)},
	}}
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, nil)
	uc.pricebook = pb

	got, err := uc.Resolve(context.Background(), "高麗菜和紅蘿蔔")

	require.NoError(t, err)
	require.False(t, got.Unavailable, "%+v", got)
	require.Equal(t, []string{"crop-LA1"}, optionIDs(got.Candidates),
		"只有一段查得到時仍要走切開後的召回——沒切就是零命中，這條證明切法真的有作用")
}

func TestResolveV2_WholeTextIsARealProductSoItIsNotSplit(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "OR9", "虎頭蘭-中、小輪", "虎頭蘭-中、小輪", "CANONICAL", 0, 1, []string{"901"}, "G-ORC", true),
	}}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "虎頭蘭-中、小輪")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, "OR9", *got.CropCode,
		"整串就是目錄裡的真實品名，不能因為中間有頓號就被當成兩個品項")
}

type perVariantPricebookFake struct {
	categoryPricebookFake
	byVariant map[string][]pbusecase.PriceableEntityMatch
}

func (f *perVariantPricebookFake) SearchPriceableEntities(_ context.Context, variants []string, _ int32) ([]pbusecase.PriceableEntityMatch, error) {
	out := []pbusecase.PriceableEntityMatch{}
	for _, v := range variants {
		out = append(out, f.byVariant[v]...)
	}
	return out, nil
}
