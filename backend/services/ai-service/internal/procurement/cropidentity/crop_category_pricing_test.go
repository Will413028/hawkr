package cropidentity

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/stretchr/testify/require"
)

type categoryMappingPricebookFake struct {
	*categoryPricebookFake
	mapping          pbusecase.PricebookCategoryMapping
	mappingErr       error
	mappedCategoryID string
}

func (f *categoryMappingPricebookFake) ResolveCategoryMapping(_ context.Context, categoryID string) (pbusecase.PricebookCategoryMapping, error) {
	f.mappedCategoryID = categoryID
	return f.mapping, f.mappingErr
}

func selectedPorkCategory() *domain.CropSelection {
	return &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉", CategoryID: "301", Label: "五花肉"}
}
func resolvedPorkMapping() pbusecase.PricebookCategoryMapping {
	return pbusecase.PricebookCategoryMapping{CategoryID: "301", Status: "resolved", GroupCode: stringRef("PORK-CURATED"), Label: stringRef("策展豬肉行情"), MappingVersion: "pricebook-category-v1"}
}
func mappedPricebook(matches []pbusecase.PriceableEntityMatch) *categoryMappingPricebookFake {
	return &categoryMappingPricebookFake{categoryPricebookFake: &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: matches}}, mapping: resolvedPorkMapping()}
}

func curatedGroup(hasForecast bool) []pbusecase.PriceableEntityMatch {
	return []pbusecase.PriceableEntityMatch{
		pm("CROP_GROUP", "PORK-CURATED", "策展豬肉行情", "策展豬肉行情", "CANONICAL", 0, 1, nil, "PORK-CURATED", hasForecast),
	}
}

func TestCropCategoryPricing_UsesCuratedGroupEvenWhenLabelsDiffer(t *testing.T) {
	pb := mappedPricebook(curatedGroup(true))
	uc := NewDisambiguationUseCase(pb, nil, nil)
	uc.categories = porkCategoryTree()
	result, err := uc.ResolveSelection(context.Background(), "臺灣豬五花", &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"}, []domain.CropDisambiguationOption{{CategoryID: "301", CategoryLevel: 3}}, "category-301", "")
	require.NoError(t, err)
	require.False(t, result.Unavailable)
	require.False(t, result.NeedsDisambiguation)
	require.Equal(t, "301", pb.mappedCategoryID)
	require.Equal(t, []string{"策展豬肉行情"}, pb.GotVariants)
	require.Equal(t, "PORK-CURATED", *result.GroupCode)
	require.Equal(t, "PORK-CURATED", *result.FallbackGroupCode)
	require.Nil(t, result.CropCode, "沒有品種收窄（CropVarieties 為空）時維持大類——2026-09-15 起 L3 可以收窄到品種，但空集合的語意仍是整個大類")
	require.Equal(t, "301", result.Selection.CategoryID)
	require.Equal(t, "五花肉", result.Selection.Label)
	require.Equal(t, "策展豬肉行情", result.Label)
}

func TestCropCategoryPricing_UnmappedAmbiguousOrMalformedNeverNameFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*pbusecase.PricebookCategoryMapping)
	}{
		{name: "unmapped", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.Status = "unmapped" }},
		{name: "ambiguous", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.Status = "ambiguous" }},
		{name: "unknown status", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.Status = "unexpected" }},
		{name: "different category", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.CategoryID = "302" }},
		{name: "missing group", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.GroupCode = nil }},
		{name: "empty group", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.GroupCode = stringRef(" ") }},
		{name: "missing label", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.Label = nil }},
		{name: "empty label", mutate: func(m *pbusecase.PricebookCategoryMapping) { m.Label = stringRef(" ") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := mappedPricebook([]pbusecase.PriceableEntityMatch{
				pm("CROP", "SAME-NAME-CROP", "五花肉", "五花肉", "CANONICAL", 0, 1, nil, "PORK-CURATED", true),
			})
			tc.mutate(&pb.mapping)
			result, err := NewDisambiguationUseCase(pb, nil, nil).priceSelectedCategory(context.Background(), selectedPorkCategory())
			require.NoError(t, err)
			require.True(t, result.Unavailable)
			require.Equal(t, domain.CropSelectionPrice, result.Selection.Stage)
			require.Equal(t, "301", result.Selection.CategoryID)
			require.Nil(t, result.CropCode)
			require.Nil(t, result.GroupCode)
			require.Empty(t, pb.GotVariants, "A name match cannot override curator intent or malformed mapping")
		})
	}
}

func TestCropCategoryPricing_ForecastMustBelongToAuthoritativeGroup(t *testing.T) {
	for _, tc := range []struct {
		name  string
		match pbusecase.PriceableEntityMatch
	}{
		{name: "mapped group no forecast", match: pm("CROP_GROUP", "PORK-CURATED", "策展豬肉行情", "策展豬肉行情", "CANONICAL", 0, 1, nil, "PORK-CURATED", false)},
		{name: "same label other group", match: pm("CROP_GROUP", "WRONG-GROUP", "策展豬肉行情", "策展豬肉行情", "CANONICAL", 0, 1, nil, "WRONG-GROUP", true)},
		{name: "specific crop cannot replace mapped group", match: pm("CROP", "GUESS-CROP", "五花肉", "五花肉", "CANONICAL", 0, 1, nil, "PORK-CURATED", true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := mappedPricebook([]pbusecase.PriceableEntityMatch{tc.match})
			result, err := NewDisambiguationUseCase(pb, nil, nil).priceSelectedCategory(context.Background(), selectedPorkCategory())
			require.NoError(t, err)
			require.True(t, result.Unavailable)
			require.Nil(t, result.GroupCode)
			require.Nil(t, result.CropCode)
		})
	}
}

func TestCropCategoryPricing_FindsMappedGroupBeyondFirstTenResults(t *testing.T) {
	var matches []pbusecase.PriceableEntityMatch
	for i := 0; i < 10; i++ {
		matches = append(matches, pm("CROP_GROUP", fmt.Sprintf("other-%d", i), "其他行情", "其他行情", "CANONICAL", 1, i+1, nil, fmt.Sprintf("other-%d", i), true))
	}
	matches = append(matches, pm("CROP_GROUP", "PORK-CURATED", "策展豬肉行情", "策展豬肉行情", "CANONICAL", 0, 11, nil, "PORK-CURATED", true))
	pb := mappedPricebook(matches)
	result, err := NewDisambiguationUseCase(pb, nil, nil).priceSelectedCategory(context.Background(), selectedPorkCategory())
	require.NoError(t, err)
	require.Equal(t, "PORK-CURATED", *result.GroupCode)
	require.Nil(t, result.CropCode)
}

func TestCropCategoryPricing_ErrorsAreNotPersistedAsManualDecisions(t *testing.T) {
	for _, mappingFails := range []bool{true, false} {
		pb := mappedPricebook(nil)
		want := errors.New("temporary upstream failure")
		if mappingFails {
			pb.mappingErr = want
		} else {
			pb.PriceableErr = want
		}
		result, err := NewDisambiguationUseCase(pb, nil, nil).priceSelectedCategory(context.Background(), selectedPorkCategory())
		require.ErrorIs(t, err, want)
		require.False(t, result.Unavailable)
		require.Nil(t, result.Selection)
	}
}

func TestCropCategoryPricing_MissingMapperCannotGuessByName(t *testing.T) {
	pb := &categoryPricebookFake{byQuery: map[string][]pbusecase.CropCatalogMatch{"五花肉": {{MatchType: "crop", CropCode: stringRef("SAME-NAME"), CropName: stringRef("五花肉"), HasForecast: true}}}}
	result, err := NewDisambiguationUseCase(pb, nil, nil).priceSelectedCategory(context.Background(), selectedPorkCategory())
	require.NoError(t, err)
	require.True(t, result.Unavailable)
	require.Empty(t, pb.queries)
	require.Nil(t, result.CropCode)
}

func TestCropCategoryPricing_CommodityMappingResolves(t *testing.T) {
	pb := mappedPricebook(nil)
	pb.mapping = pbusecase.PricebookCategoryMapping{
		CategoryID: "301", Status: "resolved",
		CommodityCode: stringRef("958000000000000001"),
		Label:         stringRef("毛豬（規格豬）"), MappingVersion: "pricebook-category-v1",
	}
	uc := NewDisambiguationUseCase(pb, nil, nil)
	uc.categories = porkCategoryTree()

	result, err := uc.ResolveSelection(context.Background(), "臺灣豬五花",
		&domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"},
		[]domain.CropDisambiguationOption{{CategoryID: "301", CategoryLevel: 3}}, "category-301", "")
	require.NoError(t, err)
	require.False(t, result.Unavailable)
	require.False(t, result.NeedsDisambiguation)
	require.NotNil(t, result.CommodityCode)
	require.Equal(t, "958000000000000001", *result.CommodityCode)
	require.Nil(t, result.GroupCode, "commodity 與 group 互斥")
	require.Nil(t, result.CropCode)
	require.Empty(t, pb.queries, "commodity 映射不該觸發 catalog 搜尋")
	require.Equal(t, "毛豬（規格豬）", result.Label)
	require.Equal(t, "301", result.Selection.CategoryID)
}

func TestCropCategoryPricing_ResolvedWithNoTargetDegradesToUnmapped(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mapping pbusecase.PricebookCategoryMapping
	}{
		{name: "兩者皆 nil", mapping: pbusecase.PricebookCategoryMapping{
			CategoryID: "301", Status: "resolved", Label: stringRef("策展豬肉行情"),
		}},
		{name: "兩者皆空白字串", mapping: pbusecase.PricebookCategoryMapping{
			CategoryID: "301", Status: "resolved", GroupCode: stringRef("  "),
			CommodityCode: stringRef(""), Label: stringRef("策展豬肉行情"),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := mappedPricebook(nil)
			pb.mapping = tc.mapping
			uc := NewDisambiguationUseCase(pb, nil, nil)
			uc.categories = porkCategoryTree()

			result, err := uc.ResolveSelection(context.Background(), "臺灣豬五花",
				&domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"},
				[]domain.CropDisambiguationOption{{CategoryID: "301", CategoryLevel: 3}}, "category-301", "")
			require.NoError(t, err)
			require.Nil(t, result.CommodityCode)
			require.Nil(t, result.GroupCode)
		})
	}
}
func TestCropCategoryPricing_SingleVarietyNarrowsToCropCode(t *testing.T) {
	pb := mappedPricebook(curatedGroup(true))
	pb.mapping.CropVarieties = []pbusecase.PricebookCropVariety{{CropCode: "PORK-BELLY", Label: "五花"}}
	uc := NewDisambiguationUseCase(pb, nil, nil)
	uc.categories = porkCategoryTree()

	result, err := uc.ResolveSelection(context.Background(), "臺灣豬五花", &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"}, []domain.CropDisambiguationOption{{CategoryID: "301", CategoryLevel: 3}}, "category-301", "")
	require.NoError(t, err)
	require.NotNil(t, result.CropCode, "收窄到單一品種時必須鎖定該品種，否則買家挑的是「五花」卻拿到整個豬肉大類的混合價")
	require.Equal(t, "PORK-BELLY", *result.CropCode)
	require.Nil(t, result.GroupCode, "鎖定品種時不再回大類碼")
	require.Equal(t, "PORK-CURATED", *result.FallbackGroupCode, "大類必須留成 fallback：品種查無行情時要退得回去")
	require.Equal(t, "五花", result.Label, "顯示名用品種的字，不是大類的")
}

func TestCropCategoryPricing_MultipleVarietiesKeepGroup(t *testing.T) {
	pb := mappedPricebook(curatedGroup(true))
	pb.mapping.CropVarieties = []pbusecase.PricebookCropVariety{
		{CropCode: "PORK-BELLY", Label: "五花"},
		{CropCode: "PORK-BELLY-IMPORT", Label: "進口 五花"},
	}
	uc := NewDisambiguationUseCase(pb, nil, nil)
	uc.categories = porkCategoryTree()

	result, err := uc.ResolveSelection(context.Background(), "臺灣豬五花", &domain.CropSelection{Stage: domain.CropSelectionItem, ParentCategoryID: "200", ParentLabel: "豬肉"}, []domain.CropDisambiguationOption{{CategoryID: "301", CategoryLevel: 3}}, "category-301", "")
	require.NoError(t, err)
	require.Nil(t, result.CropCode, "兩個品種時硬挑一個會系統性排除另一半（國產／進口），維持大類才是誠實的")
	require.Equal(t, "PORK-CURATED", *result.GroupCode)
}
