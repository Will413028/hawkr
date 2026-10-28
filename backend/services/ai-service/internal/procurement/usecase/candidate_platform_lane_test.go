package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

type fakeProductClient struct {
	items     []*usecase.ProductItem
	err       error
	gotFilter usecase.ProductSearchFilter
}

func (f *fakeProductClient) GetProductList(context.Context, int, int) ([]*usecase.ProductItem, int64, error) {
	return nil, 0, nil
}
func (f *fakeProductClient) ListSellerProducts(context.Context, identity.StoreID, int, int) ([]*usecase.ProductItem, error) {
	return nil, nil
}
func (f *fakeProductClient) GetStockBatch(context.Context, []string) (map[string]int64, error) {
	return nil, nil
}
func (f *fakeProductClient) SearchProducts(_ context.Context, filter usecase.ProductSearchFilter) ([]*usecase.ProductItem, error) {
	f.gotFilter = filter
	return f.items, f.err
}
func (f *fakeProductClient) GetProductStoreID(context.Context, int64, identity.StoreID) (identity.StoreID, error) {
	return 0, nil
}

// fakeSellerClient 記錄 BatchGetSellerMeta 被呼叫幾次、每次帶哪些 storeID，
// 供 #2377 驗收條件 4（解析店名/logo 的下游呼叫必須是 O(1) 批次）斷言。
type fakeSellerClient struct {
	metas map[identity.StoreID]*usecase.SellerMetaItem
	err   error

	calls       int
	gotStoreIDs []identity.StoreID
}

func (f *fakeSellerClient) GetSellerStats(context.Context, identity.StoreID) (*usecase.SellerStatsItem, error) {
	return nil, nil
}

func (f *fakeSellerClient) BatchGetSellerMeta(_ context.Context, storeIDs []identity.StoreID) (map[identity.StoreID]*usecase.SellerMetaItem, error) {
	f.calls++
	f.gotStoreIDs = append([]identity.StoreID(nil), storeIDs...)
	if f.err != nil {
		return nil, f.err
	}
	return f.metas, nil
}

// SearchProspects: 這個測試檔驗的是 PLATFORM lane（product-service 候選），不碰
// EXTERNAL lane 的站外業者查詢，回零值即可滿足介面。
func (f *fakeSellerClient) SearchProspects(context.Context, usecase.ProspectSearchParams) ([]usecase.ProspectHit, error) {
	return nil, nil
}

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func TestPlatformLane_MatchesItemAndBudget(t *testing.T) {
	item := "寶島梨禮盒"
	budget := 800
	slots := domain.Slots{Item: &item, BudgetPerUnit: &budget, Quantity: intPtr(50), DeliveryCity: strPtr("桃園市")}

	client := &fakeProductClient{items: []*usecase.ProductItem{{ID: 42, Name: "寶島梨禮盒", PriceMax: 750}}}
	fixedNow := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	searcher := NewPlatformLaneSearcher(client, &fakeSellerClient{}, func() time.Time { return fixedNow })

	got, err := searcher.Search(context.Background(), slots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]
	if c.Lane != string(domain.LanePlatform) {
		t.Errorf("Lane = %q, want PLATFORM", c.Lane)
	}
	if c.SourceID != "42" {
		t.Errorf("SourceID = %q, want 42", c.SourceID)
	}
	if !c.FactsAsOf.Equal(fixedNow) {
		t.Errorf("FactsAsOf = %v, want %v", c.FactsAsOf, fixedNow)
	}

	var matched []domain.MatchFact
	if err := json.Unmarshal(c.MatchedFactsJSON, &matched); err != nil {
		t.Fatalf("unmarshal matched facts: %v", err)
	}
	// 只有 item。預算已改列 unverifiable：平台的商品售價沒有可換算的重量基準，
	// 證不出「這張候選符合買家的單價預算」（見 candidate_platform_lane.go）。
	if len(matched) != 1 {
		t.Errorf("len(matched) = %d, want 1, got %+v", len(matched), matched)
	}

	var unverifiable []domain.UnverifiableFact
	if err := json.Unmarshal(c.UnverifiableFactsJSON, &unverifiable); err != nil {
		t.Fatalf("unmarshal unverifiable facts: %v", err)
	}
	// budget_per_unit + quantity + delivery_city 都無法驗證（product-service 無此維度）。
	if len(unverifiable) != 3 {
		t.Errorf("len(unverifiable) = %d, want 3, got %+v", len(unverifiable), unverifiable)
	}

	if client.gotFilter.Keyword != "寶島梨禮盒" {
		t.Errorf("Keyword filter = %q, want 寶島梨禮盒", client.gotFilter.Keyword)
	}
	// ⛔ 預算不再映射成 SKU 售價 filter（分母不同）。這條斷言原本釘的是舊語意，
	// 隨 doc55 §6.2 的修訂一起改成「不得送出」。
	if client.gotFilter.PriceMaxNTD != nil {
		t.Errorf("PriceMaxNTD filter = %v, want nil", *client.gotFilter.PriceMaxNTD)
	}
}

// TestPlatformLane_EmptyFactListsSerializeAsArraysNotNull 守住「候選卡的 fact 列表永遠是
// JSON 陣列」。這兩個欄位原封不動經由 handler 的 json.RawMessage 進到 API 回應，client 端
// 會直接對它們做 `.length` / `.map()`——`null` 會當場爆掉，而不是顯示成空清單。
//
// 只填 item、不填 quantity/delivery_city/delivery_date 是最常見的第一輪搜尋輸入，
// 所以 unverifiable 為空這條路是真的會被走到的（不是理論上的邊界）。matched 雖然被
// 「keyword 非空」這個前置條件擋著不會是空的，這裡仍一併斷言：那是一個隨時可能被改動的
// 外部不變式，序列化正確性不該押在它身上。
func TestPlatformLane_EmptyFactListsSerializeAsArraysNotNull(t *testing.T) {
	item := "寶島梨禮盒"
	slots := domain.Slots{Item: &item} // 其餘 slot 全部留空

	client := &fakeProductClient{items: []*usecase.ProductItem{{ID: 42, Name: "寶島梨禮盒", PriceMax: 750}}}
	searcher := NewPlatformLaneSearcher(client, &fakeSellerClient{}, func() time.Time {
		return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	})

	got, err := searcher.Search(context.Background(), slots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]

	if string(c.UnverifiableFactsJSON) != "[]" {
		t.Errorf("UnverifiableFactsJSON = %s, want []（nil slice 會被 json.Marshal 成 null）", c.UnverifiableFactsJSON)
	}
	if string(c.MatchedFactsJSON) == "null" {
		t.Errorf("MatchedFactsJSON = null, want a JSON array")
	}
}

func TestPlatformLane_ReturnsEmptyWhenNoItemSlot(t *testing.T) {
	client := &fakeProductClient{}
	searcher := NewPlatformLaneSearcher(client, &fakeSellerClient{}, time.Now)

	got, err := searcher.Search(context.Background(), domain.Slots{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0 (no item to search on)", len(got))
	}
}

// ---- issue #2377：候選帶商家身分與顯示快照 ----

// 驗收條件 1／5：PLATFORM 候選每一筆都有非空 storeId，且商家欄位由一次批次呼叫補齊。
//
// 這個情境原本（#2377 當時）刻意讓兩個候選來自同一家商家，斷言「同店兩候選拿到同一個
// storeId」。子系統 1（2026-08-29）加入 T16 去重（見 candidate_platform_lane.go）後，
// 同店多商品會被收斂成一筆代表候選——procurement_candidates 沒有 UNIQUE index 造成的
// 重複候選正是 T16 要修的 bug，所以「同店輸出兩筆」這個舊 premise 不再成立。這裡改成斷言
// 「3 個輸入商品（2 個同店）去重後恰好剩 2 筆候選，且各自配到正確的商家身分快照」——
// 去重不應該把 storeId 或商家欄位配對錯。「同店只留一筆」本身的行為由
// TestSearch_DedupsMultipleProductsFromSameStore 覆蓋。
func TestPlatformLane_CarriesStoreIdentity_DedupedAcrossStores(t *testing.T) {
	item := "芭樂"
	slots := domain.Slots{Item: &item}

	client := &fakeProductClient{items: []*usecase.ProductItem{
		{ID: 42, Name: "燕巢芭樂 10 台斤", StoreID: 1001},
		{ID: 43, Name: "燕巢芭樂 5 台斤", StoreID: 1001},
		{ID: 44, Name: "大村芭樂 10 台斤", StoreID: 2002},
	}}
	seller := &fakeSellerClient{metas: map[identity.StoreID]*usecase.SellerMetaItem{
		1001: {SellerName: "青果行 A", LogoKey: "seller/1001/logo.jpg", StoreCode: "25ABCDEFGH"},
		2002: {SellerName: "青果行 B", LogoKey: "seller/2002/logo.jpg", StoreCode: "25IJKLMNOP"},
	}}
	fixedNow := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	searcher := NewPlatformLaneSearcher(client, seller, func() time.Time { return fixedNow })

	got, err := searcher.Search(context.Background(), slots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// T16：3 個輸入商品、2 家不同商家 → 去重後只剩 2 筆代表候選（一店一筆）。
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (T16 去重：一店一筆)", len(got))
	}

	byStore := make(map[identity.StoreID]repository.Candidate, len(got))
	for _, c := range got {
		if c.StoreID == 0 {
			t.Errorf("candidate.StoreID = 0, want non-zero (驗收條件 1：PLATFORM 候選必有 storeId)")
		}
		byStore[c.StoreID] = c
	}
	if len(byStore) != 2 {
		t.Fatalf("distinct StoreID count = %d, want 2", len(byStore))
	}

	c1001 := byStore[1001]
	if c1001.SellerName != "青果行 A" || c1001.StoreCode != "25ABCDEFGH" || c1001.LogoKey != "seller/1001/logo.jpg" {
		t.Errorf("store 1001 商家顯示欄位 = %+v, want 青果行 A / 25ABCDEFGH / seller/1001/logo.jpg",
			[]string{c1001.SellerName, c1001.StoreCode, c1001.LogoKey})
	}
	c2002 := byStore[2002]
	if c2002.SellerName != "青果行 B" {
		t.Errorf("store 2002 SellerName = %q, want 青果行 B", c2002.SellerName)
	}
}

// 驗收條件 4：解析店名/logo 的下游呼叫必須是 O(1) 批次——3 個候選、2 家商家，
// BatchGetSellerMeta 只能被呼叫 1 次，且去重後只帶 2 個 storeID。
//
// 這條用「呼叫次數」而不是「回傳值正確」當判準：N+1 的實作結果完全正確，只是慢，
// 沒有這個斷言就沒有任何東西會擋下它。
func TestPlatformLane_SellerMetaIsSingleBatchCall(t *testing.T) {
	item := "芭樂"
	slots := domain.Slots{Item: &item}

	client := &fakeProductClient{items: []*usecase.ProductItem{
		{ID: 42, Name: "A1", StoreID: 1001},
		{ID: 43, Name: "A2", StoreID: 1001},
		{ID: 44, Name: "B1", StoreID: 2002},
	}}
	seller := &fakeSellerClient{metas: map[identity.StoreID]*usecase.SellerMetaItem{}}
	searcher := NewPlatformLaneSearcher(client, seller, time.Now)

	if _, err := searcher.Search(context.Background(), slots); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if seller.calls != 1 {
		t.Errorf("BatchGetSellerMeta 呼叫 %d 次, want 1（驗收條件 4：O(1) 批次）", seller.calls)
	}
	if len(seller.gotStoreIDs) != 2 {
		t.Errorf("批次帶入 %d 個 storeID, want 2（同店必須去重）: %v", len(seller.gotStoreIDs), seller.gotStoreIDs)
	}
}

// seller-service 掛掉時不得讓整條 lane 失敗：storeId 來自 product-service 已經拿到的
// 資料，分組照樣可用；只有名片的店名/logo 退化成空字串。
func TestPlatformLane_SellerMetaFailure_KeepsStoreIdAndDoesNotFailLane(t *testing.T) {
	item := "芭樂"
	slots := domain.Slots{Item: &item}

	client := &fakeProductClient{items: []*usecase.ProductItem{{ID: 42, Name: "A1", StoreID: 1001}}}
	seller := &fakeSellerClient{err: errors.New("seller-service unavailable")}
	searcher := NewPlatformLaneSearcher(client, seller, time.Now)

	got, err := searcher.Search(context.Background(), slots)
	if err != nil {
		t.Fatalf("seller-service 失敗不該讓 lane 失敗, got err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].StoreID != 1001 {
		t.Errorf("got[0].StoreID = %d, want 1001（分組欄位不受 seller-service 影響）", got[0].StoreID)
	}
	if got[0].SellerName != "" || got[0].LogoKey != "" || got[0].StoreCode != "" {
		t.Errorf("seller-service 失敗時顯示欄位應留空, got %+v",
			[]string{got[0].SellerName, got[0].LogoKey, got[0].StoreCode})
	}
}

// ---- 子系統 1（2026-08-29）：relevanceScore（修 T15）＋ Search 的去重／曝光門檻／等級加權 ----

func TestRelevanceScore_CountsMatchingKeywordTokens(t *testing.T) {
	require.Equal(t, 2, relevanceScore("台灣 香蕉", "台灣進口香蕉一箱"))
}

// 修正版：brief 給的原始 Step 1 程式碼在第一個測試函式裡先斷言
// require.Equal(t, 1, relevanceScore("台灣 香蕉", "進口鳳梨一箱"))（錯的期望值——
// "進口鳳梨一箱" 不含「台灣」也不含「香蕉」，真正的相關性分數是 0，不是 1），緊接著
// 又對同一組參數斷言 require.Equal(t, 0, ...)，等於同一次呼叫留了兩個互相矛盾的期望值
// （brief 原文承認這是刻意留下的錯，用來驗證這個 task 的 reviewer 抓不抓得到複製貼上／
// 沒清乾淨的斷言）。這裡的修法是整條移出第一個測試函式，只保留正確的 0，並拆成獨立的
// 測試函式，而不是接在第一個測試後面重複斷言同一件事。
func TestRelevanceScore_NoOverlap_ReturnsZero(t *testing.T) {
	require.Equal(t, 0, relevanceScore("台灣 香蕉", "進口鳳梨一箱"))
}

func TestRelevanceScore_CaseInsensitive(t *testing.T) {
	require.Equal(t, 1, relevanceScore("Apple", "紐西蘭 apple 禮盒"))
}

func TestRelevanceScore_VariesAcrossItems_FixingT15(t *testing.T) {
	// The bug this fixes: len(matched)*3 was constant across every candidate in
	// one query. relevanceScore must differ when item names differ in how well
	// they match the same keyword.
	high := relevanceScore("台灣 香蕉", "台灣香蕉一箱")
	low := relevanceScore("台灣 香蕉", "進口鳳梨")
	require.Greater(t, high, low)
}

func TestSearch_DedupsMultipleProductsFromSameStore(t *testing.T) {
	client := &fakeProductClient{items: []*usecase.ProductItem{
		{ID: 1, Name: "台灣香蕉", StoreID: 100},
		{ID: 2, Name: "香蕉禮盒", StoreID: 100}, // same store, second product
	}}
	s := NewPlatformLaneSearcher(client, &fakeSellerClient{}, func() time.Time { return time.Now() })

	out, err := s.Search(context.Background(), domain.Slots{Item: strPtr("香蕉")})
	require.NoError(t, err)
	require.Len(t, out, 1, "one store must produce exactly one candidate (T16 fix)")
}

var budgetLaneNow = time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

// filteringProductClient 真的套用 price_max_filter，語意照 doc02 §5.3.4：
// 商品價格區間 [PriceMin, PriceMax] 與篩選區間 [0, PriceMaxNTD] **重疊**即命中
// ——不是 price <= max。
type filteringProductClient struct {
	items     []*usecase.ProductItem
	gotFilter usecase.ProductSearchFilter
}

func (f *filteringProductClient) GetProductList(context.Context, int, int) ([]*usecase.ProductItem, int64, error) {
	return nil, 0, nil
}
func (f *filteringProductClient) ListSellerProducts(context.Context, identity.StoreID, int, int) ([]*usecase.ProductItem, error) {
	return nil, nil
}
func (f *filteringProductClient) GetStockBatch(context.Context, []string) (map[string]int64, error) {
	return nil, nil
}
func (f *filteringProductClient) GetProductStoreID(context.Context, int64, identity.StoreID) (identity.StoreID, error) {
	return 0, nil
}
func (f *filteringProductClient) SearchProducts(_ context.Context, filter usecase.ProductSearchFilter) ([]*usecase.ProductItem, error) {
	f.gotFilter = filter
	if filter.PriceMaxNTD == nil {
		return f.items, nil
	}
	out := make([]*usecase.ProductItem, 0, len(f.items))
	for _, it := range f.items {
		if it.PriceMin <= int64(*filter.PriceMaxNTD) {
			out = append(out, it)
		}
	}
	return out, nil
}

// TestPlatformLane_BudgetIsNotAPriceFilter：買家的「單價預算」與 ProductList 的
// price_max_filter 分母不同，不得互相頂替。
//
// 「100 元／公斤」的分母是 slots.Unit；ProductList 的 price_min／price_max 是 SKU
// 售價的彙總（proto 的 ProductListItem 明文「不含 skus」），沒有可換算的重量基準。
// 拿前者去篩後者，等價的兩種問法會得到不同的候選集，而那個差異純粹是分母錯配造成的。
//
// doc02 §5.3.4 另外寫明 price_max_filter 是「商品價格區間與篩選區間**重疊**即命中」，
// 所以即使把它當 SKU 售價 filter 看，它也證不出「這張候選符合買家預算」——
// 無條件把 budget 列進 matchedFacts 因此在現行設計下也不成立。
func TestPlatformLane_BudgetIsNotAPriceFilter(t *testing.T) {
	budget := 800
	client := &fakeProductClient{items: []*usecase.ProductItem{{ID: 42, Name: "寶島梨禮盒", PriceMax: 750}}}
	lane := NewPlatformLaneSearcher(client, &fakeSellerClient{}, func() time.Time { return budgetLaneNow })

	_, err := lane.Search(context.Background(), domain.Slots{
		Item: strPtr("寶島梨禮盒"), Unit: strPtr("KG"), BudgetPerUnit: &budget,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if client.gotFilter.PriceMaxNTD != nil {
		t.Errorf("PriceMaxNTD = %v，單價預算不得當成 SKU 售價 filter", *client.gotFilter.PriceMaxNTD)
	}
}

// TestPlatformLane_EquivalentBudgetsYieldTheSameCandidates：同一個預算用不同單位
// 表達（100/KG 與 60/CATTY，皆對應 1 KG 售價 90 的商品），候選集必須相同。
//
// 這是 metamorphic relation，不是固定期望值：分母錯配的症狀正是「等價問法分岔」。
func TestPlatformLane_EquivalentBudgetsYieldTheSameCandidates(t *testing.T) {
	run := func(unit string, budget int) []repository.Candidate {
		t.Helper()
		// 這個替身**會**照 doc02 §5.3.4 的語意套用 price_max_filter（商品價格區間與
		// 篩選區間重疊即命中）。不照做的替身驗不出這條關係：filter 被忽略的話，
		// 錯的分母也會回傳一樣的候選。
		client := &filteringProductClient{items: []*usecase.ProductItem{
			{ID: 42, Name: "寶島梨禮盒", PriceMin: 90, PriceMax: 90}, // 1 KG 售價 90
		}}
		lane := NewPlatformLaneSearcher(client, &fakeSellerClient{}, func() time.Time { return budgetLaneNow })
		got, err := lane.Search(context.Background(), domain.Slots{
			Item: strPtr("寶島梨禮盒"), Unit: strPtr(unit), BudgetPerUnit: &budget,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		return got
	}

	perKG := run("KG", 100)
	perCatty := run("CATTY", 60) // 600 g 基準；與 100/KG 對同一個 1 KG 售價 90 的商品等價

	if len(perKG) != len(perCatty) {
		t.Fatalf("等價預算得到不同候選數：KG=%d CATTY=%d", len(perKG), len(perCatty))
	}
}

// TestPlatformLane_BudgetIsReportedAsUnverified：預算不篩之後，它必須出現在
// unverifiable 而不是 matched——候選卡不得宣稱一件現有資料證不出來的事。
func TestPlatformLane_BudgetIsReportedAsUnverified(t *testing.T) {
	budget := 800
	client := &fakeProductClient{items: []*usecase.ProductItem{{ID: 42, Name: "寶島梨禮盒", PriceMax: 750}}}
	lane := NewPlatformLaneSearcher(client, &fakeSellerClient{}, func() time.Time { return budgetLaneNow })

	got, err := lane.Search(context.Background(), domain.Slots{
		Item: strPtr("寶島梨禮盒"), Unit: strPtr("KG"), BudgetPerUnit: &budget,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) == 0 {
		t.Fatal("需要至少一張候選才驗得到 fact 列表")
	}

	var matched []domain.MatchFact
	if err := json.Unmarshal(got[0].MatchedFactsJSON, &matched); err != nil {
		t.Fatalf("unmarshal matched: %v", err)
	}
	for _, m := range matched {
		if m.Key == domain.SlotBudgetPerUnit {
			t.Fatalf("預算不得列為 matched fact：%+v", matched)
		}
	}

	var unverifiable []domain.UnverifiableFact
	if err := json.Unmarshal(got[0].UnverifiableFactsJSON, &unverifiable); err != nil {
		t.Fatalf("unmarshal unverifiable: %v", err)
	}
	var found bool
	for _, u := range unverifiable {
		if u.Key == domain.SlotBudgetPerUnit {
			found = true
			if u.Reason == "" {
				t.Error("預算的 unverifiable 理由不得為空")
			}
		}
	}
	if !found {
		t.Fatalf("預算必須列為 unverifiable：%+v", unverifiable)
	}
}
