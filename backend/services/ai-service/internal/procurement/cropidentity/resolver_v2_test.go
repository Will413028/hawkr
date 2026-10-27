package cropidentity

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/fuse"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

func pm(kind, code, name, form, formKind string, tier, rank int, cats []string, group string, hasForecast bool) pbusecase.PriceableEntityMatch {
	return pbusecase.PriceableEntityMatch{Kind: kind, Code: code, Name: name, CategoryIDs: cats, MatchedForm: form, FormKind: formKind, Tier: tier, Rank: rank, HasForecast: hasForecast, GroupCode: group}
}

func commodityMapping(categoryID, label, commodityCode string) *categoryMappingFake {
	return &categoryMappingFake{mapping: pbusecase.PricebookCategoryMapping{
		CategoryID: categoryID, Status: "resolved", Label: &label, CommodityCode: &commodityCode,
	}}
}

func v2UC(pb *categoryPricebookFake, tree cropCategoryReader, mapping *categoryMappingFake, model *categorySuggestFake) *DisambiguationUseCase {
	uc := NewDisambiguationUseCase(pb, tree, nil)
	if mapping != nil {
		uc.categoryMapping = mapping
	}
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

func TestResolveV2_SingleStrongCommodityAutoResolvesViaMapping(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "白肉雞（2.0 公斤以上）產地價", "白肉雞", "SOURCE_TERM", 0, 1, []string{"2011", "2258"}, "", true),
		pm("COMMODITY", "8", "白肉雞（1.75–1.95 公斤）產地價", "白肉雞", "SOURCE_TERM", 0, 2, nil, "", true),
		pm("COMMODITY", "9", "白肉雞門市價（高屏）", "白肉雞", "SOURCE_TERM", 0, 3, nil, "", true),
	}}}
	mapping := commodityMapping("2011", "白肉雞（2.0 公斤以上）產地價", "TaijinPrice_2.0kgup")
	uc := v2UC(pb, v2CategoryTree(), mapping, nil)

	got, err := uc.Resolve(context.Background(), "白肉雞")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.False(t, got.Unavailable)
	require.NotNil(t, got.CommodityCode)
	require.Equal(t, "TaijinPrice_2.0kgup", *got.CommodityCode)
	require.Equal(t, "2011", got.Selection.CategoryID)
	require.Equal(t, domain.CropSelectionPrice, got.Selection.Stage)
	require.Equal(t, []string{"2011"}, mapping.categoryIDs)
	require.Equal(t, 1, pb.PriceableCalls)
	require.Empty(t, pb.queries, "v2 不打 SearchCropCatalog")
}

func TestResolveV2_PinnedTaxonomyAbsorbedIntoWeakEntity(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "毛豬（規格豬）", "毛豬（規格豬）", "CANONICAL", 1, 1, []string{"2008", "2257"}, "", true),
	}}}
	mapping := commodityMapping("2008", "毛豬（規格豬）", "SpecPig_AvgPrice")
	uc := v2UC(pb, v2CategoryTree(), mapping, nil)

	got, err := uc.Resolve(context.Background(), "毛豬")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.NotNil(t, got.CommodityCode)
	require.Equal(t, "SpecPig_AvgPrice", *got.CommodityCode)
	require.Equal(t, "2008", got.Selection.CategoryID)
}

func TestResolveV2_SingleStrongGroupDrillsDown(t *testing.T) {

	la1, la2, cab := "LA1", "LA2", "G-CAB"
	n1, n2 := "甘藍-初秋", "甘藍-秋冬"
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{
		PriceableMatches: []pbusecase.PriceableEntityMatch{pm("CROP_GROUP", cab, "甘藍", "高麗菜", "ALIAS", 0, 1, nil, cab, true)},
		GroupCrops: []pbusecase.CropCatalogMatch{
			{MatchType: "crop", GroupCode: cab, GroupName: "甘藍", CropCode: &la1, CropName: &n1, HasForecast: true},
			{MatchType: "crop", GroupCode: cab, GroupName: "甘藍", CropCode: &la2, CropName: &n2, HasForecast: true},
		},
	}}
	uc := v2UC(pb, nil, nil, nil)

	got, err := uc.Resolve(context.Background(), "高麗菜")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Nil(t, got.Selection, "pricebook 卡沒有 taxonomy stage")
	require.Equal(t, []string{"crop-LA1", "crop-LA2", "group-G-CAB"}, optionIDs(got.Candidates))
	require.Equal(t, "甘藍-初秋", got.Candidates[0].Label)
	require.Equal(t, cab, pb.GotGroupCode)
}

func TestResolveV2_ExactLeafCategoryAutoResolves(t *testing.T) {

	pb := &categoryPricebookFake{}
	mapping := commodityMapping("2008", "毛豬（規格豬）", "SpecPig_AvgPrice")
	uc := v2UC(pb, v2CategoryTree(), mapping, nil)

	got, err := uc.Resolve(context.Background(), "豬肉")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.NotNil(t, got.CommodityCode)
	require.Equal(t, "2008", got.Selection.CategoryID)
	require.Equal(t, "200", got.Selection.ParentCategoryID)
	require.Equal(t, "毛豬（規格豬）", got.Label)
}

func TestResolveV2_UniqueWeakCandidateAutoResolves(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "12", "白米零售價（稉種）", "白米零售價（稉種）", "CANONICAL", 2, 1, []string{"2022"}, "", true),
		pm("COMMODITY", "13", "白米躉售價（稉種）", "白米躉售價（稉種）", "CANONICAL", 2, 2, nil, "", true),
	}}}
	mapping := commodityMapping("2022", "白米零售價（稉種）", "pt_1japt")
	uc := v2UC(pb, v2CategoryTree(), mapping, nil)

	got, err := uc.Resolve(context.Background(), "稉種")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, "pt_1japt", *got.CommodityCode)
}

func TestResolveV2_MediumOnlyAsksWithList(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "J3", "荔枝-糯米", "糯米", "VARIANT", 0, 1, nil, "G-LY", true),
		pm("CROP", "FV6", "辣椒-糯米椒", "糯米椒", "VARIANT", 1, 2, nil, "G-PEP", true),
		pm("CROP", "FY7", "玉米-糯米白", "糯米白", "VARIANT", 1, 3, nil, "G-CORN", true),
	}}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "糯米")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Nil(t, got.Selection)
	require.Equal(t, []string{"crop-J3", "crop-FV6", "crop-FY7"}, optionIDs(got.Candidates))
	require.Equal(t, "G-LY", *got.Candidates[0].FallbackGroupCode)
}

func TestResolveV2_TwoStrongSameParentAsksItemCard(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "毛豬（規格豬）", "毛豬", "SOURCE_TERM", 0, 1, []string{"2008"}, "", true),
		pm("COMMODITY", "8", "生鮮豬肉批發", "毛豬", "SOURCE_TERM", 0, 2, []string{"2257"}, "", true),
	}}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "毛豬")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.NotNil(t, got.Selection)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "200", got.Selection.ParentCategoryID)
	require.Equal(t, "豬肉", got.Selection.ParentLabel)
	require.Equal(t, "毛豬", got.Selection.QueryText)
	require.Equal(t, []string{"category-2008", "category-2257"}, optionIDs(got.Candidates))
	require.Equal(t, 3, got.Candidates[0].CategoryLevel)
	require.Equal(t, "豬肉", got.Candidates[0].Label, "卡上的字是分類名，不是 commodity 名")
}

func TestResolveV2_TwoStrongAcrossParentsAsksGroupCard(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "毛豬（規格豬）", "肉", "SOURCE_TERM", 0, 1, []string{"2008"}, "", true),
		pm("COMMODITY", "8", "白肉雞（2.0 公斤以上）產地價", "肉", "SOURCE_TERM", 0, 2, []string{"2011"}, "", true),
	}}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "肉")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-200", "category-210"}, optionIDs(got.Candidates))
	require.Equal(t, 2, got.Candidates[0].CategoryLevel)
	require.Equal(t, "豬肉", got.Candidates[0].Label)
}

func TestResolveV2_FoldedToOneCategoryAutoResolves(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "白肉雞（2.0 公斤以上）產地價", "雞", "SOURCE_TERM", 0, 1, []string{"2011"}, "", true),
		pm("COMMODITY", "8", "白肉雞（1.75–1.95 公斤）產地價", "雞", "SOURCE_TERM", 0, 2, []string{"2011"}, "", true),
	}}}
	mapping := commodityMapping("2011", "白肉雞（2.0 公斤以上）產地價", "TaijinPrice_2.0kgup")
	uc := v2UC(pb, v2CategoryTree(), mapping, nil)

	got, err := uc.Resolve(context.Background(), "雞")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, "2011", got.Selection.CategoryID)
}

func TestResolveV2_TreeDownWithEmptyRecallIsEscapeWithoutLLM(t *testing.T) {
	model := &categorySuggestFake{ids: []string{"200"}}
	uc := v2UC(&categoryPricebookFake{}, &categoryTreeFake{err: errors.New("tree down")}, nil, model)

	got, err := uc.Resolve(context.Background(), "臺灣豬五花")

	require.NoError(t, err)
	require.True(t, got.Unavailable)
	require.Equal(t, 0, model.calls, "沒有樹就沒有白名單，不得問模型")
}

func TestResolveV2_LLMNotCalledWhenRecallHasCandidates(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "雞蛋產地價", "雞蛋產地價", "CANONICAL", 1, 1, []string{"2011"}, "", true),
	}}}
	model := &categorySuggestFake{ids: []string{"200"}}
	uc := v2UC(pb, v2CategoryTree(), commodityMapping("2011", "雞蛋產地價", "Egg"), model)

	got, err := uc.Resolve(context.Background(), "雞蛋")

	require.NoError(t, err)
	require.False(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, 0, model.calls)
}

func TestResolveV2_EntityErrorPropagates(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableErr: errors.New("pricebook down")}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	_, err := uc.Resolve(context.Background(), "豬肉")

	require.Error(t, err, "entity retriever 是主要來源，失敗不得被固化成解析結果")
}

func TestResolveV2_TaxonomyErrorDegrades(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "毛豬（規格豬）", "毛豬", "SOURCE_TERM", 0, 1, []string{"2008"}, "", true),
	}}}
	mapping := commodityMapping("2008", "毛豬（規格豬）", "SpecPig_AvgPrice")
	uc := v2UC(pb, &categoryTreeFake{err: errors.New("tree down")}, mapping, nil)

	got, err := uc.Resolve(context.Background(), "毛豬")

	require.NoError(t, err, "taxonomy 是補充來源，失敗只降級")
	require.NotNil(t, got.CommodityCode)
}

func TestResolveV2_DropsUnpriceable(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "LA9", "甘藍-無模型", "甘藍-無模型", "CANONICAL", 0, 1, nil, "G-CAB", false),
		pm("COMMODITY", "8", "白肉雞門市價（高屏）", "白肉雞門市價（高屏）", "CANONICAL", 0, 2, nil, "", true),
	}}}
	uc := v2UC(pb, nil, nil, nil)

	got, err := uc.Resolve(context.Background(), "甘藍-無模型")

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
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

func TestResolveV2_CardOptionCapsArePinned(t *testing.T) {
	var crops []pbusecase.PriceableEntityMatch
	for i := 0; i < 12; i++ {
		crops = append(crops, pm("CROP", fmt.Sprintf("C%d", i), fmt.Sprintf("品%d", i), "x", "CANONICAL", 0, i+1, nil, "G", true))
	}
	got, err := v2UC(&categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: crops}}, nil, nil, nil).Resolve(context.Background(), "x")
	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation)
	require.Len(t, got.Candidates, searchCropCatalogLimit, "pricebook 卡上限")

	var commodities []pbusecase.PriceableEntityMatch
	for i := 0; i < 12; i++ {
		commodities = append(commodities, pm("COMMODITY", fmt.Sprintf("M%d", i), fmt.Sprintf("品%d", i), "x", "SOURCE_TERM", 0, i+1, []string{fmt.Sprintf("3%03d", i)}, "", true))
	}
	got, err = v2UC(&categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: commodities}}, manyLeafTree(12), nil, nil).Resolve(context.Background(), "x")
	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Len(t, got.Candidates, cropCategoryMaxOptions, "分類卡上限")
}

func TestResolveV2_WeakCategoryContainmentDoesNotHijackCropCard(t *testing.T) {
	active := codemeta.CommonStatusActive.Value
	tree := &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "1", Name: "蔬菜", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "66", Name: "辣椒", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "667", Name: "糯米椒", ParentId: "66", Status: active},
				{Id: "668", Name: "青辣椒", ParentId: "66", Status: active},
			}},
		}},
	}}}
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "J3", "荔枝-糯米", "糯米", "VARIANT", 0, 1, nil, "G-LY", true),
		pm("CROP", "FV6", "辣椒-糯米椒", "糯米椒", "VARIANT", 1, 2, []string{"667", "668"}, "G-PEP", true),
		pm("CROP", "FY7", "玉米-糯米白", "糯米白", "VARIANT", 1, 3, []string{"1248"}, "G-CORN", true),
		pm("CROP", "FY5", "玉米-糯米黑", "糯米黑", "VARIANT", 1, 4, []string{"1248"}, "G-CORN", true),
		pm("CROP", "FY97", "玉米-糯米", "玉米-糯米", "CANONICAL", 2, 5, []string{"1248"}, "G-CORN", false),
	}}}
	uc := v2UC(pb, tree, nil, nil)

	got, err := uc.Resolve(context.Background(), "糯米")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Nil(t, got.Selection)
	require.Equal(t, []string{"crop-J3", "crop-FV6", "crop-FY7", "crop-FY5"}, optionIDs(got.Candidates))
}

func TestResolveV2_MixedStrongTieGoesToPricebookCard(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "LA1", "甘藍-初秋", "x", "CANONICAL", 0, 1, nil, "G-CAB", true),
		pm("COMMODITY", "7", "毛豬（規格豬）", "x", "SOURCE_TERM", 0, 2, []string{"2008"}, "", true),
		pm("CROP_GROUP", "G2", "雞肉大類", "x", "CANONICAL", 0, 3, []string{"2011"}, "G2", true),
	}}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "x")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Nil(t, got.Selection)
	require.Equal(t, []string{"crop-LA1", "group-G2"}, optionIDs(got.Candidates))
}

func TestResolveV2_StrongerCategoryFamilyWinsTheCard(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "LA1", "甘藍-初秋", "x", "CANONICAL", 2, 1, nil, "G-CAB", true),
		pm("COMMODITY", "7", "毛豬（規格豬）", "x", "SOURCE_TERM", 0, 2, []string{"2008"}, "", true),
		pm("COMMODITY", "8", "白肉雞（2.0 公斤以上）產地價", "x", "SOURCE_TERM", 0, 3, []string{"2011"}, "", true),
	}}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "x")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-200", "category-210"}, optionIDs(got.Candidates))
}

func TestResolveV2_UnmappedStrongCategoryFallsBackToRemainingCandidates(t *testing.T) {
	active := codemeta.CommonStatusActive.Value
	tree := &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "2", Name: "水果", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "84", Name: "荔枝", ParentId: "2", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "843", Name: "糯米", ParentId: "84", Status: active},
				{Id: "844", Name: "玉荷包", ParentId: "84", Status: active},
			}},
		}},
	}}}
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "J3", "荔枝-糯米", "糯米", "VARIANT", 0, 1, nil, "G-LY", true),
		pm("CROP", "FV6", "辣椒-糯米椒", "糯米椒", "VARIANT", 1, 2, []string{"668"}, "G-PEP", true),
		pm("CROP", "FY7", "玉米-糯米白", "糯米白", "VARIANT", 1, 3, []string{"1254"}, "G-CORN", true),
		pm("CROP", "FY5", "玉米-糯米黑", "糯米黑", "VARIANT", 1, 4, []string{"1255"}, "G-CORN", true),
	}}}
	mapping := &categoryMappingFake{mapping: pbusecase.PricebookCategoryMapping{CategoryID: "843", Status: "unmapped"}}
	uc := v2UC(pb, tree, mapping, nil)

	got, err := uc.Resolve(context.Background(), "糯米")

	require.NoError(t, err)
	require.Equal(t, []string{"843"}, mapping.categoryIDs, "強訊號先試映射")
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Nil(t, got.Selection)
	require.Equal(t, []string{"crop-J3", "crop-FV6", "crop-FY7", "crop-FY5"}, optionIDs(got.Candidates))
}

func TestResolveV2_UnmappedStrongCategoryAloneStillEscapes(t *testing.T) {
	active := codemeta.CommonStatusActive.Value
	tree := &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "2", Name: "水果", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "84", Name: "荔枝", ParentId: "2", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "843", Name: "糯米", ParentId: "84", Status: active},
			}},
		}},
	}}}
	mapping := &categoryMappingFake{mapping: pbusecase.PricebookCategoryMapping{CategoryID: "843", Status: "unmapped"}}
	uc := v2UC(&categoryPricebookFake{}, tree, mapping, nil)

	got, err := uc.Resolve(context.Background(), "糯米")

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
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

func strongCommodities(cats ...string) []pbusecase.PriceableEntityMatch {
	out := make([]pbusecase.PriceableEntityMatch, 0, len(cats))
	for i, c := range cats {
		out = append(out, pm("COMMODITY", fmt.Sprintf("M%d", i), "品"+c, "肉", "SOURCE_TERM", 0, i+1, []string{c}, "", true))
	}
	return out
}

func TestResolveV2_FacetDominantGroupSkipsGroupQuestion(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: strongCommodities("2008", "2257", "2301", "2011", "2302")}}
	uc := v2UC(pb, facetTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "肉")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "200", got.Selection.ParentCategoryID)
	require.Equal(t, []string{"category-2008", "category-2257", "category-2301", "category-2302"}, optionIDs(got.Candidates))
}

func TestResolveV2_FacetBalancedGroupsAskGroupCard(t *testing.T) {

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: strongCommodities("2011", "2008", "2257", "2258", "2301")}}
	uc := v2UC(pb, facetTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "肉")

	require.NoError(t, err)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-210", "category-200"}, optionIDs(got.Candidates))
}

func llmL2(id, label string) fuse.Fused {
	return fuse.Fused{Candidate: recall.Candidate{
		Key: recall.CandidateKey(recall.KindCategory, id), Kind: recall.KindCategory, Code: id, Label: label,
		CategoryLevel: 2, HasForecast: true, MatchedBy: label, Quality: recall.Weak,
	}, Sources: []string{"llm"}}
}

func TestCategoryCard_LLMMultipleGroupsAskGroupCardWithoutExpanding(t *testing.T) {

	model := &categorySuggestFake{}
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, model)

	got, err := uc.ask(context.Background(), "臺灣豬五花", []fuse.Fused{llmL2("200", "豬肉"), llmL2("210", "雞肉")})

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
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, model)

	got, err := uc.ask(context.Background(), "臺灣豬五花", []fuse.Fused{llmL2("200", "豬肉")})

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
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, model)

	got, err := uc.ask(context.Background(), "在來米", []fuse.Fused{llmL2("220", "米")})

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, domain.CropSelectionItem, got.Selection.Stage)
	require.Equal(t, "220", got.Selection.ParentCategoryID)
	require.Equal(t, []string{"category-2022"}, optionIDs(got.Candidates))
	require.Equal(t, 0, model.calls)
}

func TestCategoryCard_LLMGroupWithNoUsableChildIsEscapeWithoutConfirmedCategory(t *testing.T) {

	model := &categorySuggestFake{idsByStage: map[string][]string{"ITEM": nil}}
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, model)

	got, err := uc.ask(context.Background(), "臺灣豬五花", []fuse.Fused{llmL2("200", "豬肉")})

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
	id, _ := got.Selection.ConfirmedCategory()
	require.Empty(t, id)
}

func TestCategoryCard_LLMGroupCorroboratesExistingLeavesWithoutDoubleCounting(t *testing.T) {

	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, &categorySuggestFake{})
	porkLeaf := fuse.Fused{Candidate: recall.Candidate{Key: "CATEGORY|2008", Kind: recall.KindCategory, Code: "2008", Label: "豬肉", CategoryIDs: []string{"2008"}, CategoryLevel: 3, ParentID: "200", ParentLabel: "豬肉", HasForecast: true, Quality: recall.Weak}}
	chickenLeaf := fuse.Fused{Candidate: recall.Candidate{Key: "CATEGORY|2011", Kind: recall.KindCategory, Code: "2011", Label: "雞肉", CategoryIDs: []string{"2011"}, CategoryLevel: 3, ParentID: "210", ParentLabel: "雞肉", HasForecast: true, Quality: recall.Weak}}

	got, err := uc.ask(context.Background(), "肉", []fuse.Fused{porkLeaf, llmL2("200", "豬肉"), chickenLeaf})

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

	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7505355983145664512", "白米零售價（稉種）", "白米零售價（稉種）", "CANONICAL", 1, 1, []string{"2022"}, "", true),
		pm("CROP", "J3", "荔枝-糯米", "糯米", "VARIANT", 0, 2, nil, "G-LY", true),
		pm("CROP", "FY7", "玉米-糯米白", "糯米白", "VARIANT", 1, 3, nil, "G-CORN", true),
		pm("CROP", "FV6", "辣椒-糯米椒", "糯米椒", "VARIANT", 1, 4, nil, "G-CHILI", true),
	}}}
	uc := v2UC(pb, riceTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "糯米")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.NotNil(t, got.Selection)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-212", "category-710"}, optionIDs(got.Candidates),
		"pinned 的米排第一；沒有映射的 843 仍然上卡，讓真的要荔枝品種的買家選得到")
}

func TestNarrowToGroup_KeepsOnlyCandidatesUnderThatL2(t *testing.T) {
	uc := v2UC(&categoryPricebookFake{}, facetTree(), nil, nil)
	pork := fuse.Fused{Candidate: recall.Candidate{Key: "CATEGORY|2008", Kind: recall.KindCategory, Code: "2008", Label: "豬肉", CategoryIDs: []string{"2008"}, CategoryLevel: 3, ParentID: "200"}}
	chicken := fuse.Fused{Candidate: recall.Candidate{Key: "CATEGORY|2011", Kind: recall.KindCategory, Code: "2011", Label: "雞肉", CategoryIDs: []string{"2011"}, CategoryLevel: 3, ParentID: "210"}}
	commodity := fuse.Fused{Candidate: recall.Candidate{Key: "COMMODITY|7", Kind: recall.KindCommodity, Code: "7", Label: "毛豬（規格豬）", CategoryIDs: []string{"2257"}}}
	orphan := fuse.Fused{Candidate: recall.Candidate{Key: "CROP|LA1", Kind: recall.KindCrop, Code: "LA1", Label: "甘藍-初秋"}}

	got := uc.narrowToGroup(context.Background(), []fuse.Fused{pork, chicken, commodity, orphan}, "200")

	require.Len(t, got, 2)
	require.Equal(t, []string{"CATEGORY|2008", "COMMODITY|7"}, []string{got[0].Key, got[1].Key})
}

func TestUnexpandedGroup_IsAWeakLevelTwoCandidate(t *testing.T) {
	got := unexpandedGroup(cropCategory{ID: "200", Label: "豬肉", Level: 2})
	require.Equal(t, recall.KindCategory, got.Kind)
	require.Equal(t, "200", got.Code)
	require.Equal(t, 2, got.CategoryLevel)
	require.Equal(t, recall.Weak, got.Quality)
	require.True(t, got.HasForecast)
	require.Empty(t, got.CategoryIDs)
}

func TestResolveV2_L1QueryAsksWhichGroupInsteadOfResolvingASubstringLeaf(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "FK6", "甜椒-水果彩椒", "甜椒-水果彩椒", "CANONICAL", 1, 1, []string{"665"}, "G-PEP", true),
	}}}
	uc := v2UC(pb, fruitTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "水果")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "打上位詞不該被定案成某個子串命中的品項：%+v", got)
	require.Equal(t, domain.CropSelectionGroup, got.Selection.Stage)
	require.Equal(t, []string{"category-133", "category-134"}, optionIDs(got.Candidates))
	require.Equal(t, 2, got.Candidates[0].CategoryLevel)
	require.Nil(t, got.CropCode)
}

func TestResolveV2_ResolvedCommodityCarriesCategoryIDIntoGate(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("COMMODITY", "7", "毛豬（規格豬）", "毛豬", "SOURCE_TERM", 0, 1, []string{"2008"}, "", true),
	}}}
	uc := v2UC(pb, facetTree(), commodityMapping("2008", "毛豬（規格豬）", "SpecPig_AvgPrice"), nil)

	res, err := uc.Resolve(context.Background(), "毛豬")
	require.NoError(t, err)
	require.False(t, res.NeedsDisambiguation, "%+v", res)

	got := domain.NewSystemItemResolution(res.CropCode, res.GroupCode, res.CommodityCode, res.FallbackGroupCode, res.Label, domain.CategoryFromSelection(res.Selection))

	require.NotNil(t, got)
	require.Equal(t, domain.ItemResolvedCommodity, got.Kind)
	require.Equal(t, "2008", got.CategoryID, "下游的站外業者比對要靠它")
}

func TestResolveV2_CropCardOptionsCarryEntityCategoryID(t *testing.T) {
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{PriceableMatches: []pbusecase.PriceableEntityMatch{
		pm("CROP", "LA1", "甘藍-初秋", "甘藍", "VARIANT", 0, 1, []string{"533"}, "G-CAB", true),
		pm("CROP", "LA2", "甘藍-改良種", "甘藍", "VARIANT", 1, 2, []string{"533"}, "G-CAB", true),
	}}}
	uc := v2UC(pb, v2CategoryTree(), nil, nil)

	got, err := uc.Resolve(context.Background(), "甘藍")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	require.Equal(t, []string{"crop-LA1", "crop-LA2"}, optionIDs(got.Candidates))
	require.Equal(t, "533", got.Candidates[0].EntityCategoryID,
		"作物卡的選項要帶該實體自己的分類，買家選完才填得進 ItemResolution.CategoryID")
}

func TestResolveV2_EmptyDeterministicRecallEscapesWithoutGuessingNeighbour(t *testing.T) {
	pb := &categoryPricebookFake{}
	model := &categorySuggestFake{idsByStage: map[string][]string{cropCategoryStageL1: {"100"}, "GROUP": {"200", "210"}}}
	uc := v2UC(pb, v2CategoryTree(), nil, model)

	got, err := uc.Resolve(context.Background(), "松露")

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
	require.Empty(t, model.stages,
		"索引裡沒有精確命中就直接走逃生艙，不讓模型猜一個鄰近分類（松露被當成松茸、土豆被當成馬鈴薯）")
}

func TestResolveV2_EmptyRecallEscapesWithoutLegacyCatalogOrModel(t *testing.T) {
	pb := &categoryPricebookFake{}
	model := &categorySuggestFake{idsByStage: map[string][]string{cropCategoryStageL1: {"100"}, "GROUP": {"200"}, "ITEM": {"2257"}}}
	uc := v2UC(pb, v2CategoryTree(), nil, model)

	got, err := uc.Resolve(context.Background(), "水泥")

	require.NoError(t, err)
	require.True(t, got.Unavailable, "%+v", got)
	require.Empty(t, pb.queries, "v2 不打 SearchCropCatalog——委派 v1 fallback 的路徑已退場")
	require.Empty(t, model.stages, "索引零命中時不再請模型猜分類，直接讓買家自填價")
	id, _ := got.Selection.ConfirmedCategory()
	require.Empty(t, id)
}

func TestResolveV2_GroupDrillDownOptionsCarryEntityCategoryID(t *testing.T) {
	la1, la2, cab := "LA1", "LA2", "G-CAB"
	n1, n2 := "甘藍-初秋", "甘藍-秋冬"
	pb := &categoryPricebookFake{FakePricebookClient: testkit.FakePricebookClient{
		PriceableMatches: []pbusecase.PriceableEntityMatch{pm("CROP_GROUP", cab, "甘藍", "高麗菜", "ALIAS", 0, 1, []string{"1242"}, cab, true)},
		GroupCrops: []pbusecase.CropCatalogMatch{
			{MatchType: "crop", GroupCode: cab, GroupName: "甘藍", CropCode: &la1, CropName: &n1, HasForecast: true},
			{MatchType: "crop", GroupCode: cab, GroupName: "甘藍", CropCode: &la2, CropName: &n2, HasForecast: true},
		},
	}}
	uc := v2UC(pb, nil, nil, nil)

	got, err := uc.Resolve(context.Background(), "高麗菜")

	require.NoError(t, err)
	require.True(t, got.NeedsDisambiguation, "%+v", got)
	for i, c := range got.Candidates {
		require.Equal(t, "1242", c.EntityCategoryID,
			"大類下鑽出來的每個選項都要帶分類 id（第 %d 個：%s）——ListGroupCropOptions 不回分類，只能從大類候選帶下來", i, c.Label)
	}
}
