package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	procurementdomain "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	rootusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"

	"github.com/stretchr/testify/require"
)

type fakeCandidateLeadSearcher struct {
	matches   []rootusecase.CandidateMatch
	gotParams rootusecase.CandidateSearchParams
}

func (f *fakeCandidateLeadSearcher) Search(_ context.Context, params rootusecase.CandidateSearchParams) ([]rootusecase.CandidateMatch, error) {
	f.gotParams = params
	return f.matches, nil
}

func TestExternalLane_UsesKeywordsOnly_NotCategoryOrRegion(t *testing.T) {
	item := "寶島梨禮盒"
	city := "桃園市"
	slots := procurementdomain.Slots{Item: &item, DeliveryCity: &city}

	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{
		{Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "老王果園"}, Score: 3, MatchedFacets: []string{"keywords"}},
	}}
	searcher := NewExternalLaneSearcher(fake, nil)

	got, err := searcher.Search(context.Background(), slots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.gotParams.Region != "" {
		t.Errorf("Region = %q, want empty (delivery_city has no honest mapping)", fake.gotParams.Region)
	}
	if fake.gotParams.Category != "" {
		t.Errorf("Category = %q, want empty", fake.gotParams.Category)
	}
	if fake.gotParams.Keywords != "寶島梨禮盒" {
		t.Errorf("Keywords = %q, want %q", fake.gotParams.Keywords, "寶島梨禮盒")
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]
	if c.Lane != string(procurementdomain.LaneExternal) {
		t.Errorf("Lane = %q, want EXTERNAL", c.Lane)
	}
	if c.SourceID != "7" {
		t.Errorf("SourceID = %q, want 7", c.SourceID)
	}
	if c.DisplayName != "老王果園" {
		t.Errorf("DisplayName = %q, want 老王果園", c.DisplayName)
	}

	if c.ProspectID == nil || *c.ProspectID != 7 {
		t.Errorf("ProspectID = %v, want pointer to 7", c.ProspectID)
	}

	var unverifiable []procurementdomain.UnverifiableFact
	if err := json.Unmarshal(c.UnverifiableFactsJSON, &unverifiable); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	found := false
	for _, u := range unverifiable {
		if u.Key == procurementdomain.SlotDeliveryCity {
			found = true
		}
	}
	if !found {
		t.Errorf("expected delivery_city in unverifiable facts, got %+v", unverifiable)
	}
}

func TestExternalLane_EmptyFactListsSerializeAsArraysNotNull(t *testing.T) {
	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{

		{Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "老王果園"}, Score: 1},
	}}
	searcher := NewExternalLaneSearcher(fake, nil)

	got, err := searcher.Search(context.Background(), procurementdomain.Slots{Item: strPtr("水果")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]

	if string(c.MatchedFactsJSON) != "[]" {
		t.Errorf("MatchedFactsJSON = %s, want []（nil slice 會被 json.Marshal 成 null）", c.MatchedFactsJSON)
	}
	var unverifiable []procurementdomain.UnverifiableFact
	if err := json.Unmarshal(c.UnverifiableFactsJSON, &unverifiable); err != nil {
		t.Fatalf("unmarshal UnverifiableFactsJSON: %v", err)
	}
	if unverifiable == nil {
		t.Errorf("UnverifiableFactsJSON = %s, want JSON 陣列（nil slice 會被 json.Marshal 成 null）", c.UnverifiableFactsJSON)
	}
}

func TestExternalLane_DoesNotFabricateStoreIdentity(t *testing.T) {
	item := "芭樂"
	slots := procurementdomain.Slots{Item: &item}

	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{
		{Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "站外果園"}, Score: 50},
	}}
	searcher := NewExternalLaneSearcher(fake, nil)

	got, err := searcher.Search(context.Background(), slots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]
	if c.StoreID != 0 {
		t.Errorf("EXTERNAL 候選不得有平台 StoreID, got %d", c.StoreID)
	}
	if c.SellerName != "" || c.LogoKey != "" || c.StoreCode != "" {
		t.Errorf("EXTERNAL 候選不得有商家顯示欄位, got %+v", []string{c.SellerName, c.LogoKey, c.StoreCode})
	}
}

func TestExternalLane_FactsAsOf_UsesVendorFactsAsOf(t *testing.T) {
	factsAsOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{
		{Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "老王果園", FactsAsOf: factsAsOf}, Score: 1},
	}}
	searcher := NewExternalLaneSearcher(fake, nil)

	got, err := searcher.Search(context.Background(), procurementdomain.Slots{Item: strPtr("水果")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if !got[0].FactsAsOf.Equal(factsAsOf) {
		t.Errorf("FactsAsOf = %v, want %v（seller-service 這一列的事實截止時間，不是查詢當下）", got[0].FactsAsOf, factsAsOf)
	}
}

func TestExternalLane_DoesNotDeduplicate_OneRowPerVendorIsUpstreamInvariant(t *testing.T) {
	item := "虱目魚"
	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{
		{Vendor: rootusecase.ProspectVendor{ID: 1, DisplayName: "同名業者"}, Score: 5, MatchedFacets: []string{"keywords"}},
		{Vendor: rootusecase.ProspectVendor{ID: 2, DisplayName: "同名業者"}, Score: 3, MatchedFacets: []string{"keywords"}},
	}}
	got, err := NewExternalLaneSearcher(fake, nil).Search(context.Background(),
		procurementdomain.Slots{Item: &item})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2（去重是上游不變式，這一層不得再做）", len(got))
	}
}

func TestExternalLane_AlwaysCarriesDisclosure_EvenWhenBuyerFilledNothingElse(t *testing.T) {
	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{
		{Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "老王果園"}, Score: 1},
	}}

	got, err := NewExternalLaneSearcher(fake, nil).Search(context.Background(),
		procurementdomain.Slots{Item: strPtr("水果")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	var facts []procurementdomain.UnverifiableFact
	if err := json.Unmarshal(got[0].UnverifiableFactsJSON, &facts); err != nil {
		t.Fatalf("unmarshal UnverifiableFactsJSON: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("len(facts) = %d, want 1（只填品項時仍必須有常設揭露）: %s", len(facts), got[0].UnverifiableFactsJSON)
	}
	if facts[0] != procurementdomain.ExternalLaneDisclosure() {
		t.Errorf("facts[0] = %+v, want %+v", facts[0], procurementdomain.ExternalLaneDisclosure())
	}
}

func TestExternalLane_DisclosureComesFirst_BeforePerSlotReasons(t *testing.T) {
	qty := 50
	city := "台南市"
	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{
		{Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "老王果園"}, Score: 1},
	}}

	got, err := NewExternalLaneSearcher(fake, nil).Search(context.Background(),
		procurementdomain.Slots{Item: strPtr("芭樂"), Quantity: &qty, DeliveryCity: &city})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var facts []procurementdomain.UnverifiableFact
	if err := json.Unmarshal(got[0].UnverifiableFactsJSON, &facts); err != nil {
		t.Fatalf("unmarshal UnverifiableFactsJSON: %v", err)
	}
	if len(facts) != 3 {
		t.Fatalf("len(facts) = %d, want 3（常設揭露＋數量＋配送城市）: %s", len(facts), got[0].UnverifiableFactsJSON)
	}
	if facts[0].Key != procurementdomain.EvidenceExternalUnverified {
		t.Errorf("facts[0].Key = %q, want %q", facts[0].Key, procurementdomain.EvidenceExternalUnverified)
	}
}

type fakeAncestors struct {
	chain  map[string][]string
	gotIDs []string
}

func (f *fakeAncestors) AncestorIDs(_ context.Context, id string) []string {
	f.gotIDs = append(f.gotIDs, id)
	return f.chain[id]
}

func TestExternalLane_ExpandsResolvedCategoryIntoAncestorChain(t *testing.T) {
	item := "豬五花"
	slots := procurementdomain.Slots{
		Item: &item,
		ItemResolution: &procurementdomain.ItemResolution{
			Kind: procurementdomain.ItemResolvedCategoryConfirmed, Label: "豬肉", CategoryID: "2008",
			Provenance: procurementdomain.ItemResolutionByBuyerChoice,
		},
	}
	fake := &fakeCandidateLeadSearcher{}
	ancestors := &fakeAncestors{chain: map[string][]string{"2008": {"2008", "198", "4"}}}

	_, err := NewExternalLaneSearcher(fake, ancestors).Search(context.Background(), slots)
	require.NoError(t, err)

	require.Equal(t, []string{"2008"}, ancestors.gotIDs, "展開的是解析結果的分類，不是買家原話")
	require.Equal(t, []string{"2008", "198", "4"}, fake.gotParams.CategoryIDs,
		"買家要 L3，掛 L2 或 L1 的業者也該被找到")
	require.Equal(t, "豬五花", fake.gotParams.Keywords, "原文仍然照舊送出，兩條路是加權 OR")
}

func TestExternalLane_NoResolvedCategorySendsNoCategoryIDs(t *testing.T) {
	item := "豬五花"
	slots := procurementdomain.Slots{Item: &item}
	fake := &fakeCandidateLeadSearcher{}
	ancestors := &fakeAncestors{}

	_, err := NewExternalLaneSearcher(fake, ancestors).Search(context.Background(), slots)
	require.NoError(t, err)

	require.Empty(t, fake.gotParams.CategoryIDs)
	require.Empty(t, ancestors.gotIDs, "沒有解析結果就不必查分類樹")
}

func TestExternalLane_CategoryIDsHitIsAMatchedFact(t *testing.T) {
	item := "豬五花"
	slots := procurementdomain.Slots{
		Item: &item,
		ItemResolution: &procurementdomain.ItemResolution{
			Kind: procurementdomain.ItemResolvedCategoryConfirmed, Label: "豬肉", CategoryID: "2008",
			Provenance: procurementdomain.ItemResolutionByBuyerChoice,
		},
	}
	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{
		{Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "究好豬"}, Score: 5, MatchedFacets: []string{"category_ids"}},
	}}
	ancestors := &fakeAncestors{chain: map[string][]string{"2008": {"2008", "198", "4"}}}

	got, err := NewExternalLaneSearcher(fake, ancestors).Search(context.Background(), slots)
	require.NoError(t, err)
	require.Len(t, got, 1)

	var facts []procurementdomain.MatchFact
	require.NoError(t, json.Unmarshal(got[0].MatchedFactsJSON, &facts))
	keys := make([]string, 0, len(facts))
	for _, f := range facts {
		keys = append(keys, string(f.Key))
	}
	require.Contains(t, keys, string(procurementdomain.SlotItem), "分類命中的理由掛回品項這一格")
}

// 站外業者的品牌素材要進候選，但**不得**填進平台賣家身分那四欄。ai.proto 10–13
// 明文「EXTERNAL lane 一律空字串，不偽造平台 seller identity」；把品牌圖塞進
// logo_key 就是在偽造，因為那一欄的消費端語意是「平台商家的 logo」。
func TestExternalLane_CarriesBrandMediaWithoutFakingPlatformIdentity(t *testing.T) {
	item := "梅子"
	slots := procurementdomain.Slots{Item: &item}

	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{{
		Vendor: rootusecase.ProspectVendor{
			ID: 7504399775467307008, DisplayName: "天御梅製所",
			LogoKey:  "public/seller_prospect_logo/900000001/20260920/a.png",
			CoverKey: "public/seller_prospect_cover/900000001/20260920/b.png",
		},
		Score: 3, MatchedFacets: []string{"keywords"},
	}}}

	got, err := NewExternalLaneSearcher(fake, nil).Search(context.Background(), slots)
	require.NoError(t, err)
	require.Len(t, got, 1)
	c := got[0]

	require.Equal(t, "public/seller_prospect_logo/900000001/20260920/a.png", c.BrandLogoKey)
	require.Equal(t, "public/seller_prospect_cover/900000001/20260920/b.png", c.BrandCoverKey)

	// 平台身分四欄必須維持空——這是 G6 判定為「正確現況」的不變式。
	require.Empty(t, c.LogoKey, "品牌圖不得填進平台商家的 logo_key")
	require.Empty(t, c.SellerName)
	require.Empty(t, c.StoreCode)
	require.Zero(t, c.StoreID)
	require.NotNil(t, c.ProspectID)
}

// 有品牌圖不代表少了誠實揭露。logo 會讓卡片看起來像經過平台篩選，
// 常設揭露正是 G7「不做事前驗證」這個策略成立的前提。
func TestExternalLane_BrandMediaDoesNotDisplaceDisclosure(t *testing.T) {
	item := "梅子"
	slots := procurementdomain.Slots{Item: &item}
	fake := &fakeCandidateLeadSearcher{matches: []rootusecase.CandidateMatch{{
		Vendor: rootusecase.ProspectVendor{ID: 7, DisplayName: "天御梅製所",
			LogoKey: "public/seller_prospect_logo/1/20260920/a.png"},
		Score: 3, MatchedFacets: []string{"keywords"},
	}}}

	got, err := NewExternalLaneSearcher(fake, nil).Search(context.Background(), slots)
	require.NoError(t, err)
	require.Len(t, got, 1)

	var facts []procurementdomain.UnverifiableFact
	require.NoError(t, json.Unmarshal(got[0].UnverifiableFactsJSON, &facts))
	require.NotEmpty(t, facts)
	require.Equal(t, "外部名單，供應能力未經平台驗證，以賣家回覆為準", facts[0].Reason)
}
