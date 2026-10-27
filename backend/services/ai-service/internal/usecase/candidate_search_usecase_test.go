package usecase

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/stretchr/testify/require"
)

// fakeCandidateSellerClient 是 SellerClient 的最小假實作，只給
// CandidateSearchUseCase 用：GetSellerStats／BatchGetSellerMeta 這條測試不碰，
// 回零值即可滿足介面。
//
// candidate_search_usecase.go 這一輪把資料源從本地 sqlc 換成這支 RPC
// （usecase.SellerClient.SearchProspects，Task 8），原本靠 setupAiTestPool
// 種真實 Postgres 列的 integration test（//go:build integration）因此整批
// 失去意義——ILIKE 部分比對、category/region 的 substring 語意這些從此活在
// seller-service 的 SearchSellerProspects RPC 裡，不再是 ai-service 這一層
// 的行為。這裡改成不需要 DB 的一般 unit test，直接餵 HitCategory／HitRegion／
// HitKeywords 旗標，驗證 CandidateSearchUseCase.Search 仍原樣保留的部分：
// 加權 OR 計分、keywordRelevanceScore、優先級 tiebreak、10 筆上限。
type fakeCandidateSellerClient struct {
	hits      []ProspectHit
	err       error
	gotParams ProspectSearchParams
	calls     int
}

func (f *fakeCandidateSellerClient) GetSellerStats(context.Context, identity.StoreID) (*SellerStatsItem, error) {
	return nil, nil
}

func (f *fakeCandidateSellerClient) BatchGetSellerMeta(context.Context, []identity.StoreID) (map[identity.StoreID]*SellerMetaItem, error) {
	return nil, nil
}

func (f *fakeCandidateSellerClient) SearchProspects(_ context.Context, params ProspectSearchParams) ([]ProspectHit, error) {
	f.calls++
	f.gotParams = params
	if f.err != nil {
		return nil, f.err
	}
	return f.hits, nil
}

func TestCandidateSearchUseCase_Search_TrimsParamsAndPassesThemToSellerClient(t *testing.T) {
	seller := &fakeCandidateSellerClient{}
	uc := NewCandidateSearchUseCase(seller)

	_, err := uc.Search(context.Background(), CandidateSearchParams{
		Category: "  水果  ", Region: " 台南 ", Keywords: " 水梨 ",
	})
	require.NoError(t, err)
	require.Equal(t, 1, seller.calls)
	require.Equal(t, ProspectSearchParams{Category: "水果", Region: "台南", Keywords: "水梨", Limit: 0}, seller.gotParams,
		"三個條件都要先 TrimSpace 再送出；Limit 固定 0，交給服務端的安全上限（見 seller.proto），"+
			"不是「回傳幾筆給買家」——那個決定在下面的排序＋截斷，見 TestCandidateSearchUseCase_Search_TakesTopTenByScoreWhenMoreThanTenMatch")
}

func TestCandidateSearchUseCase_Search_PropagatesSellerClientError(t *testing.T) {
	wantErr := errors.New("seller-service unavailable")
	seller := &fakeCandidateSellerClient{err: wantErr}
	uc := NewCandidateSearchUseCase(seller)

	_, err := uc.Search(context.Background(), CandidateSearchParams{})
	require.ErrorIs(t, err, wantErr)
}

// TestCandidateSearchUseCase_Search_WeightedOrFacetScoring 驗證加權 OR 語意
// （不是嚴格 AND）：只命中 category 的候選一樣入選，只是分數比全命中的低——
// 這正是 2026-08-07 live 驗證發現的問題，修法原樣保留（見 Search 上方註解）。
func TestCandidateSearchUseCase_Search_WeightedOrFacetScoring(t *testing.T) {
	seller := &fakeCandidateSellerClient{hits: []ProspectHit{
		{ID: 1, DisplayName: "全命中", HitCategory: true, HitRegion: true},
		{ID: 2, DisplayName: "只命中分類", HitCategory: true, HitRegion: false},
		{ID: 3, DisplayName: "完全不命中", HitCategory: false, HitRegion: false},
	}}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{Category: "水果", Region: "台南"})
	require.NoError(t, err)
	require.Len(t, got, 3, "加權 OR 下三筆都入選，即使部分或全部未命中")

	require.Equal(t, int64(1), got[0].Vendor.ID, "全命中者排最前")
	require.ElementsMatch(t, []string{"category", "region"}, got[0].MatchedFacets)
	require.Empty(t, got[0].MissedFacets)

	require.Equal(t, int64(2), got[1].Vendor.ID)
	require.ElementsMatch(t, []string{"category"}, got[1].MatchedFacets)
	require.ElementsMatch(t, []string{"region"}, got[1].MissedFacets)

	require.Equal(t, int64(3), got[2].Vendor.ID)
	require.Empty(t, got[2].MatchedFacets)
	require.ElementsMatch(t, []string{"category", "region"}, got[2].MissedFacets)

	require.Greater(t, got[0].Score, got[1].Score)
	require.Greater(t, got[1].Score, got[2].Score)
}

func TestCandidateSearchUseCase_Search_NoFiltersReturnsAllWithEmptyFacets(t *testing.T) {
	seller := &fakeCandidateSellerClient{hits: []ProspectHit{
		{ID: 1, DisplayName: "甲"},
		{ID: 2, DisplayName: "乙"},
	}}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	// 三個條件皆空 → 不篩選（維持原行為），但每筆的 MatchedFacets／MissedFacets 皆空：
	// 沒指定的條件既不算命中也不算未命中。
	require.Empty(t, got[0].MatchedFacets)
	require.Empty(t, got[0].MissedFacets, "沒指定的條件不算 missed")
}

// TestCandidateSearchUseCase_Search_OrdersPopulatedPriorityLetterFirst 驗證 I2：
// priority_letter 為空字串（CRM1 全數 454 筆皆無 letter）在 ASC 排序下會排在 'A' 之前，
// 若不特別處理，有真實 A/B/C/D 優先級的候選會被排到沒有優先級資料的候選之後。
func TestCandidateSearchUseCase_Search_OrdersPopulatedPriorityLetterFirst(t *testing.T) {
	seller := &fakeCandidateSellerClient{hits: []ProspectHit{
		{ID: 100, DisplayName: "無優先級候選", HitCategory: true, PriorityLetter: "", PriorityStars: 3},
		{ID: 101, DisplayName: "A 級候選", HitCategory: true, PriorityLetter: "A", PriorityStars: 0},
	}}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{Category: "茶咖啡"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, int64(101), got[0].Vendor.ID, "priority_letter='A' 應排在空字串之前，即使 stars 較低")
	require.Equal(t, int64(100), got[1].Vendor.ID)
}

// TestCandidateSearchUseCase_Search_TiebreaksByStarsThenIDDesc 驗證 score 與
// priority_letter 都同分時，接著看 priority_stars（高的在前），最後 id DESC 當
// 穩定 tiebreaker（不依賴呼叫端偶然回傳的順序）。
func TestCandidateSearchUseCase_Search_TiebreaksByStarsThenIDDesc(t *testing.T) {
	seller := &fakeCandidateSellerClient{hits: []ProspectHit{
		{ID: 1, DisplayName: "低星等", HitCategory: true, PriorityLetter: "B", PriorityStars: 1},
		{ID: 2, DisplayName: "高星等", HitCategory: true, PriorityLetter: "B", PriorityStars: 3},
		{ID: 3, DisplayName: "同星等較大id", HitCategory: true, PriorityLetter: "B", PriorityStars: 3},
	}}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{Category: "水果"})
	require.NoError(t, err)
	gotIDs := make([]int64, len(got))
	for i, m := range got {
		gotIDs[i] = m.Vendor.ID
	}
	require.Equal(t, []int64{3, 2, 1}, gotIDs, "同 priority_letter 下先比 stars DESC，再比 id DESC")
}

// TestCandidateSearchUseCase_Search_KeywordRelevanceVariesByFieldOverlap 驗證修
// EXTERNAL lane 版本的 T15：關鍵字命中是二元旗標（hit_keywords），舊版把它換算成
// 固定的 weightKeywords 常數分數，同一次查詢下所有「純命中 keywords」的候選會拿到
// 一模一樣的分數，順序退化成跟本次查詢無關的 priority_letter/priority_stars/id。
// 這裡兩筆候選都只命中 keywords（不帶 category/region），但一筆在 ProductSummary
// 與 Positioning 兩個欄位都出現關鍵字、另一筆只在一個欄位出現——真正貼近查詢的
// 候選應該分數更高、排序更前。
func TestCandidateSearchUseCase_Search_KeywordRelevanceVariesByFieldOverlap(t *testing.T) {
	seller := &fakeCandidateSellerClient{hits: []ProspectHit{
		{ID: 10, DisplayName: "雙欄命中", HitKeywords: true,
			ProductSummary: "紅心水梨禮盒", Positioning: "水梨主打商品"},
		{ID: 11, DisplayName: "單欄命中", HitKeywords: true,
			ProductSummary: "水梨禮盒", Positioning: "在地小農直送"},
	}}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{Keywords: "水梨"})
	require.NoError(t, err)
	require.Len(t, got, 2)

	require.Equal(t, int64(10), got[0].Vendor.ID, "關鍵字同時命中兩個欄位者應排在只命中一個欄位者之前")
	require.Equal(t, int64(11), got[1].Vendor.ID)
	require.Greater(t, got[0].Score, got[1].Score, "命中欄位數不同時 score 不該是同一個常數")
}

func TestCandidateSearchUseCase_Search_CapsAtTenResults(t *testing.T) {
	hits := make([]ProspectHit, 0, 15)
	for i := int64(1); i <= 15; i++ {
		hits = append(hits, ProspectHit{ID: i, DisplayName: "候選", HitCategory: true})
	}
	seller := &fakeCandidateSellerClient{hits: hits}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{Category: "水果"})
	require.NoError(t, err)
	require.Len(t, got, 10, "即使 seller-service 回了 15 筆，這裡最多只留 10 筆")
}

// TestCandidateSearchUseCase_Search_TakesTopTenByScoreWhenMoreThanTenMatch 是 C1
// 的回歸測試。上面那條 TestCandidateSearchUseCase_Search_CapsAtTenResults 的 15
// 筆全部同分（都只有 HitCategory），任何 10 筆都滿足「回 10 筆」——抓不住「保留
// 的 10 筆是不是分數最高的 10 筆」這件事，而這正是 C1 的核心：repository 端把
// LIMIT 當成「回傳筆數」而非「安全上限」誤用時，DB 會用任意物理掃描順序先砍到
// 10 列，這裡的排序再正確也是對錯的樣本排序。
//
// 這裡讓 15 筆分數兩兩不同（用 15 個不重疊的 keyword token，命中 i 個 token 得
// i 分），且刻意讓分數與 ID 反相關（分數最高者 ID 反而最小）、送入順序也打亂——
// 只有「先看過全部 15 筆的分數再排序再截斷」才能通過；巧合依賴輸入順序或 ID
// 大小的實作都會在這裡露餡。
func TestCandidateSearchUseCase_Search_TakesTopTenByScoreWhenMoreThanTenMatch(t *testing.T) {
	tokens := make([]string, 15)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("kw%02d", i+1) // 固定寬度，彼此互不為子字串
	}
	keywords := strings.Join(tokens, " ")

	hits := make([]ProspectHit, 15)
	for score := 1; score <= 15; score++ {
		hits[score-1] = ProspectHit{
			ID:             int64(100 + (16 - score)), // 分數與 ID 刻意反相關
			DisplayName:    fmt.Sprintf("候選-score-%d", score),
			ProductSummary: strings.Join(tokens[:score], " "), // 命中前 score 個 token
			HitKeywords:    true,
		}
	}
	rand.Shuffle(len(hits), func(i, j int) { hits[i], hits[j] = hits[j], hits[i] })

	seller := &fakeCandidateSellerClient{hits: hits}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{Keywords: keywords})
	require.NoError(t, err)
	require.Len(t, got, 10)

	wantIDs := make([]int64, 0, 10)
	for score := 15; score >= 6; score-- {
		wantIDs = append(wantIDs, int64(100+(16-score)))
	}
	gotIDs := make([]int64, len(got))
	for i, m := range got {
		gotIDs[i] = m.Vendor.ID
	}
	require.Equal(t, wantIDs, gotIDs,
		"保留的 10 筆必須是分數最高的 10 筆、依分數 DESC 排序，不是任意 10 筆")
}

// TestCandidateSearchUseCase_Search_MapsAllVendorFieldsCorrectly 是 ProspectVendor
// 映射的 round-trip 測試（取代原本針對 fromGenRow 的同名測試）。多數映射欄位是同
// 型別的 string，一個欄位互換（例如 Positioning: h.ResearchSummary）會編譯成功、
// 也不會被其他測試抓到——它們只斷言 ID／DisplayName／PriorityLetter。這裡種一筆
// 每個欄位都是可辨識相異值的 hit，逐欄位個別斷言。
func TestCandidateSearchUseCase_Search_MapsAllVendorFieldsCorrectly(t *testing.T) {
	factsAsOf := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	hit := ProspectHit{
		ID:              999,
		DisplayName:     "field-round-trip-test-display-name",
		Category:        "field-round-trip-test-category",
		Region:          "field-round-trip-test-region",
		ProductSummary:  "field-round-trip-test-product",
		Positioning:     "field-round-trip-test-positioning",
		ResearchSummary: "field-round-trip-test-research",
		PublicWebsite:   "field-round-trip-test-website",
		PriorityLetter:  "Z",
		PriorityStars:   7,
		UpdatedAt:       factsAsOf,
	}
	seller := &fakeCandidateSellerClient{hits: []ProspectHit{hit}}
	uc := NewCandidateSearchUseCase(seller)

	got, err := uc.Search(context.Background(), CandidateSearchParams{})
	require.NoError(t, err)
	require.Len(t, got, 1)

	v := got[0].Vendor
	require.Equal(t, hit.ID, v.ID, "ID")
	require.Equal(t, hit.DisplayName, v.DisplayName, "DisplayName")
	require.Equal(t, hit.Category, v.Category, "Category")
	require.Equal(t, hit.Region, v.Region, "Region")
	require.Equal(t, hit.ProductSummary, v.ProductSummary, "ProductSummary")
	require.Equal(t, hit.Positioning, v.Positioning, "Positioning")
	require.Equal(t, hit.ResearchSummary, v.ResearchSummary, "ResearchSummary")
	require.Equal(t, hit.PublicWebsite, v.PublicWebsite, "PublicWebsite")
	require.Equal(t, hit.PriorityLetter, v.PriorityLetter, "PriorityLetter")
	require.Equal(t, hit.PriorityStars, v.PriorityStars, "PriorityStars")
	require.True(t, v.FactsAsOf.Equal(factsAsOf), "FactsAsOf 應等於 ProspectHit.UpdatedAt")
}
