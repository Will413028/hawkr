package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"go.uber.org/zap"
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/ai-service/internal/client/pricebook"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
)

func TestAllToolsCreation(t *testing.T) {
	deps := &ToolDeps{}

	tools := []struct {
		fn   func()
		name string
	}{

		{func() { NewGetSellerOrdersTool(deps) }, "get_seller_orders"},
		{func() { NewGetOrderDetailTool(deps) }, "get_order_detail"},
		{func() { NewCheckInventoryShortageTool(deps) }, "check_inventory_shortage"},
		{func() { NewQueryPaymentSummaryTool(deps) }, "query_payment_summary"},
		{func() { NewGetSeasonalPriceSuggestionTool(deps) }, "get_seasonal_price_suggestion"},

		{func() { NewSearchCategoryTool(deps) }, "search_category"},
		{func() { NewUpdateDraftTool(deps) }, "update_draft"},

		{func() { NewGetMyOrdersTool(deps) }, "get_my_orders"},
		{func() { NewGetBuyerOrderDetailTool(deps) }, "get_buyer_order_detail"},
		{func() { NewTrackMyPackageTool(deps) }, "track_my_package"},
		{func() { NewRecommendSeasonalFruitsTool(deps) }, "recommend_seasonal_fruits"},
		{func() { NewSearchCandidateSellersTool(deps) }, "search_candidate_sellers"},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn()
		})
	}
}

func TestSearchCategoryTool_NilIndex(t *testing.T) {
	deps := &ToolDeps{CategoryIndex: nil}

	_ = NewSearchCategoryTool(deps)
}

func TestUpdateDraftTool_SchemaFields(t *testing.T) {
	deps := &ToolDeps{}

	_ = NewUpdateDraftTool(deps)
}

func TestSkuPatch_UnitValues(t *testing.T) {
	valid := []string{"KG", "CATTY", "GRAM", "NONE", ""}
	validSet := map[string]bool{}
	for _, v := range valid {
		validSet[v] = true
	}

	invalid := []string{"kg", "catty", "gram", "公斤", "台斤", "克", "袋", "盒"}
	for _, u := range invalid {
		if validSet[u] {
			t.Errorf("unit %q should be invalid", u)
		}
	}
}

type stubCandidateLeadSearcher struct {
	matches []usecase.CandidateMatch
	err     error
}

func (s stubCandidateLeadSearcher) Search(_ context.Context, _ usecase.CandidateSearchParams) ([]usecase.CandidateMatch, error) {
	return s.matches, s.err
}

func TestSearchCandidateSellers_EmptyResult_ReturnsEmptySliceNotNil(t *testing.T) {
	deps := &ToolDeps{CandidateLeadSearcher: stubCandidateLeadSearcher{matches: nil}}
	out, err := searchCandidateSellers(context.Background(), deps, SearchCandidateSellersInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Candidates == nil {
		t.Fatal("Candidates 應為非 nil 空 slice，得到 nil")
	}
	if len(out.Candidates) != 0 {
		t.Fatalf("預期 0 筆 candidates，得到 %d 筆", len(out.Candidates))
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(string(b), `"candidates":null`) {
		t.Fatalf("candidates 序列化為 null，得到: %s", b)
	}
	if !strings.Contains(string(b), `"candidates":[]`) {
		t.Fatalf("candidates 應序列化為 []，得到: %s", b)
	}
}

func TestSearchCandidateSellers_MapsBrandName(t *testing.T) {
	deps := &ToolDeps{CandidateLeadSearcher: stubCandidateLeadSearcher{matches: []usecase.CandidateMatch{
		{Vendor: usecase.ProspectVendor{ID: 1, DisplayName: "十八養場", Category: "水果", Region: "台南"}},
		{Vendor: usecase.ProspectVendor{ID: 2, DisplayName: "歐昱詮", Category: "水果", Region: "台南東山"}},
	}}}
	out, err := searchCandidateSellers(context.Background(), deps, SearchCandidateSellersInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Candidates) != 2 {
		t.Fatalf("預期 2 筆 candidates，得到 %d 筆", len(out.Candidates))
	}
	if out.Candidates[0].BrandName != "十八養場" {
		t.Errorf("candidate[0].BrandName = %q, want %q", out.Candidates[0].BrandName, "十八養場")
	}
	if out.Candidates[1].BrandName != "歐昱詮" {
		t.Errorf("candidate[1].BrandName = %q, want %q", out.Candidates[1].BrandName, "歐昱詮")
	}
	b, _ := json.Marshal(out.Candidates[0])
	if !strings.Contains(string(b), `"brand_name":"十八養場"`) {
		t.Fatalf("brand_name json tag 未正確序列化: %s", b)
	}
}

func TestSearchCandidateSellers_LargeTSIDSurvivesAsExactString(t *testing.T) {
	const largeTSID int64 = 741058396217344558
	deps := &ToolDeps{CandidateLeadSearcher: stubCandidateLeadSearcher{matches: []usecase.CandidateMatch{
		{Vendor: usecase.ProspectVendor{ID: largeTSID, DisplayName: "測試廠商", Category: "水果", Region: "台南"}},
	}}}
	out, err := searchCandidateSellers(context.Background(), deps, SearchCandidateSellersInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("預期 1 筆 candidates，得到 %d 筆", len(out.Candidates))
	}

	wantID := strconv.FormatInt(largeTSID, 10)
	if out.Candidates[0].ID != wantID {
		t.Fatalf("candidate.ID = %q, want %q（strconv.FormatInt 映射失準）", out.Candidates[0].ID, wantID)
	}

	b, err := json.Marshal(out.Candidates[0])
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	wantQuoted := fmt.Sprintf(`"id":"%d"`, largeTSID)
	if !strings.Contains(string(b), wantQuoted) {
		t.Fatalf("id 應以加引號字串序列化為 %s，得到: %s", wantQuoted, b)
	}
}

type declarationProvider interface {
	Declaration() *genai.FunctionDeclaration
}

func TestSearchCandidateSellersTool_OutputSchema_AcceptsRealMarshaledOutput(t *testing.T) {
	deps := &ToolDeps{}
	tl := NewSearchCandidateSellersTool(deps)
	dp, ok := tl.(declarationProvider)
	if !ok {
		t.Fatalf("search_candidate_sellers tool 型別 %T 未實作 Declaration()，無法取得 output schema", tl)
	}
	decl := dp.Declaration()
	rawSchema, ok := decl.ResponseJsonSchema.(*jsonschema.Schema)
	if !ok {
		t.Fatalf("Declaration().ResponseJsonSchema 型別 = %T，want *jsonschema.Schema", decl.ResponseJsonSchema)
	}
	resolved, err := rawSchema.Resolve(nil)
	if err != nil {
		t.Fatalf("output schema Resolve failed: %v", err)
	}

	const largeTSID int64 = 741058396217344558
	out := SearchCandidateSellersOutput{Candidates: []CandidateSellerResult{{
		ID:        strconv.FormatInt(largeTSID, 10),
		BrandName: "測試廠商",
		Category:  "水果",
		Region:    "台南",
	}}}

	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if err := resolved.Validate(decoded); err != nil {
		t.Fatalf("search_candidate_sellers 真實 output schema 拒絕了真實序列化後的 tool 輸出，"+
			"functiontool.Run 在生產環境會對每一次呼叫回傳這個錯誤: %v\nmarshaled: %s", err, raw)
	}
}

func TestSearchCandidateSellersTool_OutputSchemaOmitsContactFields(t *testing.T) {
	tl := NewSearchCandidateSellersTool(&ToolDeps{})
	dp, ok := tl.(declarationProvider)
	if !ok {
		t.Fatalf("search_candidate_sellers tool 型別 %T 未實作 Declaration()，無法取得 output schema", tl)
	}
	rawSchema, ok := dp.Declaration().ResponseJsonSchema.(*jsonschema.Schema)
	if !ok {
		t.Fatalf("Declaration().ResponseJsonSchema 型別 = %T，want *jsonschema.Schema", dp.Declaration().ResponseJsonSchema)
	}
	schemaJSON, err := json.Marshal(rawSchema)
	if err != nil {
		t.Fatalf("json.Marshal(schema) failed: %v", err)
	}
	got := string(schemaJSON)

	for _, banned := range []string{"public_email", "public_phone", "public_social"} {
		if strings.Contains(got, banned) {
			t.Errorf("output schema 宣告了聯絡欄位 %q，該欄位不得出現在模型上下文\nschema: %s", banned, got)
		}
	}

	if !strings.Contains(got, "public_website") {
		t.Errorf("output schema 應保留 public_website\nschema: %s", got)
	}
}

func TestAllAgentsCreation(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() 非預期 error: %v", err)
	}
	if cfg.GeminiAPIKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping agent creation test")
	}

	pricebookC, err := pricebook.NewPricebookClient(cfg)
	if err != nil {
		t.Fatalf("NewPricebookClient failed: %v", err)
	}
	deps := &ToolDeps{PricebookClient: pricebookC}

	agents, err := NewAgents(llmprovider.NewAssistant(cfg, zap.NewNop()), deps)
	if err != nil {
		t.Fatalf("NewAgents failed: %v", err)
	}

	if agents.OrderPresenter == nil {
		t.Error("OrderPresenter is nil")
	}
	for _, role := range []string{"seller", "buyer"} {
		if agents.Coordinators[role] == nil {
			t.Errorf("Coordinators[%q] is nil", role)
		}
	}
}

type fakeState struct {
	data map[string]any
}

func (s fakeState) Get(key string) (any, error) {
	v, ok := s.data[key]
	if !ok {
		return nil, session.ErrStateKeyNotExist
	}
	return v, nil
}

func (s fakeState) Set(key string, value any) error {
	s.data[key] = value
	return nil
}

func (s fakeState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for k, v := range s.data {
			if !yield(k, v) {
				return
			}
		}
	}
}

type fakeContext struct {
	adkagent.ContextMock
	state fakeState
}

func (c *fakeContext) State() session.State { return c.state }

func jsonRoundTrip(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return out
}

func TestGetInt64State_LargeTSIDSurvivesJSONRoundTrip(t *testing.T) {

	const largeID int64 = 7489534863566774901

	t.Run("string encoding survives the round trip exactly (the fix)", func(t *testing.T) {
		roundTripped := jsonRoundTrip(t, map[string]any{StateKeyBuyer: strconv.FormatInt(largeID, 10)})
		ctx := &fakeContext{state: fakeState{data: roundTripped}}
		if got := int64(getBuyerID(ctx)); got != largeID {
			t.Fatalf("getBuyerID() = %d, want %d — string encoding must survive the JSONB round trip exactly", got, largeID)
		}
	})

	t.Run("raw int64 encoding does NOT survive the same round trip (documents why the fix is needed)", func(t *testing.T) {
		roundTripped := jsonRoundTrip(t, map[string]any{StateKeyStore: largeID})
		ctx := &fakeContext{state: fakeState{data: roundTripped}}
		if got := int64(getStoreID(ctx)); got == largeID {
			t.Fatalf("getStoreID() unexpectedly recovered the exact value %d via raw int64 encoding — "+
				"this comparison is EXPECTED to fail (that's the whole point of writing state values as "+
				"strings instead); if it now passes, Go's JSON float64 handling changed or this test is wrong", largeID)
		}
	})

	t.Run("small ids (real account_id range) survive either encoding", func(t *testing.T) {
		const smallID int64 = 2670447993
		roundTripped := jsonRoundTrip(t, map[string]any{StateKeyStore: smallID})
		ctx := &fakeContext{state: fakeState{data: roundTripped}}
		if got := int64(getStoreID(ctx)); got != smallID {
			t.Fatalf("getStoreID() = %d, want %d — small ids should never have been at risk, "+
				"this is why the pre-#2270 architecture (which only ever threaded account_id, never a "+
				"19-digit TSID, through this code path) never hit this bug", got, smallID)
		}
	})

	t.Run("missing key returns 0, not an error", func(t *testing.T) {
		ctx := &fakeContext{state: fakeState{data: map[string]any{}}}
		if got := int64(getBuyerID(ctx)); got != 0 {
			t.Fatalf("getBuyerID() = %d, want 0 for a missing key", got)
		}
	})
}

func TestSearchCandidateSellers_OutputOmitsContactPII(t *testing.T) {
	deps := &ToolDeps{CandidateLeadSearcher: stubCandidateLeadSearcher{matches: []usecase.CandidateMatch{
		{Vendor: usecase.ProspectVendor{
			ID:            1,
			DisplayName:   "紅玉果園",
			Category:      "水果",
			Region:        "台南",
			PublicWebsite: "https://example.com",
		}},
	}}}

	out, err := searchCandidateSellers(context.Background(), deps, SearchCandidateSellersInput{Category: "水果"})
	if err != nil {
		t.Fatalf("searchCandidateSellers() error = %v", err)
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(out.Candidates))
	}

	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	body := string(raw)

	if strings.Contains(body, "public_email") {
		t.Errorf("body should not contain \"public_email\"")
	}
	if strings.Contains(body, "public_phone") {
		t.Errorf("body should not contain \"public_phone\"")
	}
	if strings.Contains(body, "public_social") {
		t.Errorf("body should not contain \"public_social\"")
	}

	if !strings.Contains(body, "https://example.com") {
		t.Errorf("body should contain \"https://example.com\"")
	}
}
