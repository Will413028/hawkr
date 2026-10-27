package cache

import (
	"sync"
	"testing"
	"time"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

// buildTestIndex 直接注入測試用 index（不經 gRPC）
func buildTestIndex(entries map[string][]leafEntry) *CategoryIndex {
	ci := &CategoryIndex{
		index:     entries,
		updatedAt: time.Now(),
		ready:     true,
	}
	return ci
}

// --- 基本 Match 行為 ---

func TestCategoryIndex_ExactMatch(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"番石榴": {{catalogID: "174", path: "水果 > 番石榴 > 珍珠芭"}},
	})

	results := ci.Match("番石榴", 3)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].CatalogID != "174" {
		t.Errorf("expected catalogID 174, got %s", results[0].CatalogID)
	}
}

func TestCategoryIndex_PartialMatch(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"甘藍芽": {{catalogID: "1244", path: "蔬菜 > 甘藍 > 甘藍芽"}},
		"甘藍心": {{catalogID: "1243", path: "蔬菜 > 甘藍 > 甘藍心"}},
		"甜甘藍": {{catalogID: "1245", path: "蔬菜 > 甘藍 > 甜甘藍"}},
	})

	// 搜尋「甘藍」前綴，應找到包含「甘藍」的 leaf
	results := ci.Match("甘藍", 3)
	if len(results) == 0 {
		t.Error("expected at least 1 partial match for 甘藍")
	}
}

func TestCategoryIndex_TopNLimit(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"番茄": {
			{catalogID: "1", path: "蔬菜 > 番茄 > 牛番茄"},
			{catalogID: "2", path: "蔬菜 > 番茄 > 聖女"},
			{catalogID: "3", path: "蔬菜 > 番茄 > 黑柿"},
			{catalogID: "4", path: "蔬菜 > 番茄 > 桃太郎"},
		},
	})

	results := ci.Match("番茄", 3)
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestCategoryIndex_ConfidenceDecreasing(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"蓮霧": {
			{catalogID: "1", path: "水果 > 蓮霧 > 黑珍珠"},
			{catalogID: "2", path: "水果 > 蓮霧 > 大目"},
		},
	})

	results := ci.Match("蓮霧", 3)
	if len(results) < 2 {
		t.Skip("not enough results to check confidence order")
	}
	if results[0].Confidence < results[1].Confidence {
		t.Errorf("confidence should be descending, got %s < %s", results[0].Confidence, results[1].Confidence)
	}
}

func TestCategoryIndex_EmptyKeyword(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"番石榴": {{catalogID: "174", path: "水果 > 番石榴 > 珍珠芭"}},
	})

	results := ci.Match("", 3)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty keyword, got %d", len(results))
	}
}

func TestCategoryIndex_ReadyFalseWhenNotInitialized(t *testing.T) {
	ci := &CategoryIndex{}
	if ci.Ready() {
		t.Error("expected Ready() = false before initialization")
	}
}

// --- synonym 行為 ---

// TestSynonym_GuavaDirection：芭樂（消費者用語）→ 番石榴（分類名稱）
func TestSynonym_GuavaDirection(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"番石榴": {{catalogID: "829", path: "水果 > 番石榴 > 珍珠芭"}},
	})

	results := ci.Match("芭樂", 3)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for synonym 芭樂→番石榴, got %d", len(results))
	}
	if results[0].CatalogID != "829" {
		t.Errorf("expected catalogID 829, got %s", results[0].CatalogID)
	}
}

// TestSynonym_PomeloDirection：文旦 → 柚子（L2 分類名稱）
func TestSynonym_PomeloDirection(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"文旦": {{catalogID: "763", path: "水果 > 柚子 > 文旦"}},
		"柚子": {{catalogID: "764", path: "水果 > 柚子 > 白柚"}},
	})

	// "文旦" 是 key，直接精確匹配
	results := ci.Match("文旦", 3)
	if len(results) == 0 {
		t.Fatal("expected results for 文旦")
	}

	// synonym "文旦" → "柚子" 也應讓 "文旦" 找到 "柚子" key 下的 entries
	// (當 test index 只有 "柚子" key 時)
	ci2 := buildTestIndex(map[string][]leafEntry{
		"柚子": {{catalogID: "763", path: "水果 > 柚子 > 文旦"}},
	})
	results2 := ci2.Match("文旦", 3)
	if len(results2) == 0 {
		t.Error("synonym 文旦→柚子 should resolve when index key is 柚子")
	}
}

// TestSynonym_StarfruitNotRedirected：楊桃（已是分類名）不應被亂轉
func TestSynonym_StarfruitNotRedirected(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"楊桃": {{catalogID: "847", path: "水果 > 楊桃 > 紅龍"}},
	})

	results := ci.Match("楊桃", 3)
	if len(results) == 0 {
		t.Error("楊桃 is a real category name and should match directly")
	}
}

// TestSynonym_CitrusNotRedirected：柚子（已是分類名）不應被亂轉
func TestSynonym_CitrusNotRedirected(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"柚子": {{catalogID: "763", path: "水果 > 柚子 > 文旦"}},
	})

	results := ci.Match("柚子", 3)
	if len(results) == 0 {
		t.Error("柚子 is a real category name and should match directly")
	}
}

// TestSynonym_Typos：錯字修正
func TestSynonym_Typos(t *testing.T) {
	cases := []struct {
		alias     string
		realKey   string
		catalogID string
	}{
		{"輪霧", "蓮霧", "789"},
		{"蕃茄", "番茄", "500"},
		{"番荔枝", "釋迦", "178"},
	}
	for _, tc := range cases {
		ci := buildTestIndex(map[string][]leafEntry{
			tc.realKey: {{catalogID: tc.catalogID, path: "水果 > " + tc.realKey}},
		})
		results := ci.Match(tc.alias, 3)
		if len(results) == 0 {
			t.Errorf("typo/alias %q should resolve to %q, got 0 results", tc.alias, tc.realKey)
			continue
		}
		if results[0].CatalogID != tc.catalogID {
			t.Errorf("%q → %q: expected catalogID %s, got %s", tc.alias, tc.realKey, tc.catalogID, results[0].CatalogID)
		}
	}
}

// TestSynonym_VegetableConsumerNames：台灣消費者慣用蔬菜名稱
func TestSynonym_VegetableConsumerNames(t *testing.T) {
	cases := []struct {
		alias   string
		realKey string
	}{
		{"高麗菜", "甘藍"},
		{"空心菜", "蕹菜"},
		{"四季豆", "敏豆"},
		{"洋芋", "馬鈴薯"},
		{"紅蘿蔔", "胡蘿蔔"},
		{"香菇", "濕香菇"},
		{"木耳", "濕木耳"},
		{"秋葵", "黃秋葵"},
		{"蔥", "青蔥"},
		{"大白菜", "包心白"},
		{"白花椰", "花椰菜"},
		{"花菜", "花椰菜"},
		{"青花菜", "青花苔"},
		{"青椒", "甜椒"},
		{"地瓜", "甘薯"},
		{"地瓜葉", "甘薯葉"},
		{"芋頭", "芋"},
		{"玉米筍", "玉米"},
	}
	for _, tc := range cases {
		ci := buildTestIndex(map[string][]leafEntry{
			tc.realKey: {{catalogID: "999", path: "蔬菜 > " + tc.realKey}},
		})
		results := ci.Match(tc.alias, 3)
		if len(results) == 0 {
			t.Errorf("consumer alias %q should resolve to category %q", tc.alias, tc.realKey)
		}
	}
}

// TestSynonym_FruitConsumerNames：台灣消費者慣用水果名稱
func TestSynonym_FruitConsumerNames(t *testing.T) {
	cases := []struct {
		alias   string
		realKey string
	}{
		{"芭樂", "番石榴"},
		{"文旦", "柚子"},
		{"橘子", "椪柑"},
		{"柳橙", "甜橙"},
		{"梨子", "梨"},
		{"李子", "李"},
	}
	for _, tc := range cases {
		ci := buildTestIndex(map[string][]leafEntry{
			tc.realKey: {{catalogID: "888", path: "水果 > " + tc.realKey}},
		})
		results := ci.Match(tc.alias, 3)
		if len(results) == 0 {
			t.Errorf("consumer alias %q should resolve to category %q", tc.alias, tc.realKey)
		}
	}
}

// --- walkNode L2 索引行為 ---

// TestWalkNode_L2IndexedUnderOwnName：L2 節點應以自身名稱被索引，指向 L3 子節點
func TestWalkNode_L2IndexedUnderOwnName(t *testing.T) {
	newIdx := make(map[string][]leafEntry)

	// 模擬：L1=水果, L2=番石榴(id=174), L3=珍珠芭(id=829) + 帝王芭(id=827)
	l3a := newMockNode("829", "珍珠芭", nil)
	l3b := newMockNode("827", "帝王芭", nil)
	l2 := newMockNode("174", "番石榴", []mockNode{l3a, l3b})
	l1 := newMockNode("2", "水果", []mockNode{l2})

	walkNode(l1.toProto(), "", newIdx)

	// L3 leaves 應被索引
	if _, ok := newIdx["珍珠芭"]; !ok {
		t.Error("L3 leaf '珍珠芭' should be indexed")
	}
	if _, ok := newIdx["帝王芭"]; !ok {
		t.Error("L3 leaf '帝王芭' should be indexed")
	}

	// L2 '番石榴' 也應被索引，catalogID 指向 L3 子節點
	l2Entries, ok := newIdx["番石榴"]
	if !ok {
		t.Fatal("L2 node '番石榴' should also be indexed")
	}
	if len(l2Entries) != 2 {
		t.Errorf("expected 2 entries for L2 '番石榴' (one per L3 child), got %d", len(l2Entries))
	}

	// L1 '水果' 不應被索引
	if _, ok := newIdx["水果"]; ok {
		t.Error("L1 node '水果' should NOT be indexed")
	}
}

// TestWalkNode_SameNameL2AndL3：L2 與 L3 同名時（如「草莓 > 草莓」），entries 不重複
func TestWalkNode_SameNameL2AndL3(t *testing.T) {
	newIdx := make(map[string][]leafEntry)

	l3 := newMockNode("1991", "草莓", nil)
	l2 := newMockNode("170", "草莓", []mockNode{l3})
	l1 := newMockNode("2", "水果", []mockNode{l2})

	walkNode(l1.toProto(), "", newIdx)

	entries := newIdx["草莓"]
	if len(entries) == 0 {
		t.Fatal("expected entries for 草莓")
	}
	// L2 的 entry（指向 L3）+ L3 自身 = 同一個 catalogID，所以可能有 2 條（都有效）
	for _, e := range entries {
		if e.catalogID != "1991" {
			t.Errorf("unexpected catalogID %s for 草莓 entry", e.catalogID)
		}
	}
}

// --- 無匹配情境 ---

// TestMatch_NoMatchForUncatalogued：平台沒有的品類（如茶葉）應回空
func TestMatch_NoMatchForUncatalogued(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"番茄": {{catalogID: "1", path: "蔬菜 > 番茄"}},
	})

	uncatalogued := []string{"茶葉", "烏龍茶", "高山茶", "珍珠奶茶"}
	for _, kw := range uncatalogued {
		results := ci.Match(kw, 3)
		if len(results) != 0 {
			t.Errorf("%q should return 0 results (not in catalog), got %d", kw, len(results))
		}
	}
}

// TestMatch_NoMatchReturnsEmptyNotNil：空結果應回 []CategoryCandidate{} 而非 nil
func TestMatch_NoMatchReturnsEmptyNotNil(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{})
	results := ci.Match("任何東西", 3)
	if results == nil {
		t.Error("Match should return empty slice, not nil")
	}
}

// --- 並發安全性 ---

func TestCategoryIndex_ConcurrentMatch(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"番石榴": {{catalogID: "829", path: "水果 > 番石榴 > 珍珠芭"}},
		"蓮霧":  {{catalogID: "789", path: "水果 > 蓮霧 > 黑珍珠"}},
	})

	var wg sync.WaitGroup
	errs := make(chan string, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 直接搜尋 L2 名稱
			r1 := ci.Match("番石榴", 3)
			if len(r1) == 0 {
				errs <- "concurrent 番石榴 match failed"
			}
			// 透過 synonym 搜尋
			r2 := ci.Match("芭樂", 3)
			if len(r2) == 0 {
				errs <- "concurrent synonym 芭樂→番石榴 match failed"
			}
			// 搜尋不存在的品類不 panic
			_ = ci.Match("茶葉", 3)
		}()
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}
}

// --- index refresh ---

func TestCategoryIndex_RefreshUpdatesIndex(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"舊資料": {{catalogID: "old", path: "舊路徑"}},
	})

	ci.mu.Lock()
	ci.index = map[string][]leafEntry{
		"新資料": {{catalogID: "new", path: "新路徑"}},
	}
	ci.updatedAt = time.Now()
	ci.mu.Unlock()

	results := ci.Match("新資料", 3)
	if len(results) == 0 || results[0].CatalogID != "new" {
		t.Error("expected updated index to return new data")
	}
	results2 := ci.Match("舊資料", 3)
	if len(results2) != 0 {
		t.Error("expected old data to be gone after refresh")
	}
}

// --- 輔助：mock proto node ---

type mockNode struct {
	id       string
	name     string
	children []mockNode
}

func newMockNode(id, name string, children []mockNode) mockNode {
	return mockNode{id: id, name: name, children: children}
}

func (m mockNode) toProto() *categorypb.CategoryNode {
	node := &categorypb.CategoryNode{}
	node.Id = m.id
	node.Name = m.name
	for _, c := range m.children {
		node.Children = append(node.Children, c.toProto())
	}
	return node
}

// --- GetCategoryTree：採購的大品項／小品項選擇共用這份快取 ---

// client 是 nil，所以任何想重新抓樹的路徑都會失敗——回傳成功即證明走的是快取。
func TestCategoryIndex_GetCategoryTree_ServesActiveFromCache(t *testing.T) {
	tree := &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "100", Name: "肉類"},
	}}
	ci := buildTestIndex(map[string][]leafEntry{})
	ci.tree = tree

	got, err := ci.GetCategoryTree(t.Context(), "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tree {
		t.Error("expected the cached tree, not a freshly fetched one")
	}
}

// NewReadyCategoryIndex 是 ready 但 client／tree 皆 nil，不得在這條路徑上 panic。
func TestCategoryIndex_GetCategoryTree_NilClientReturnsError(t *testing.T) {
	ci := NewReadyCategoryIndex(nil)

	got, err := ci.GetCategoryTree(t.Context(), "active")
	if err == nil {
		t.Fatal("expected an error when no tree is cached and no client is wired")
	}
	if got != nil {
		t.Error("expected no tree alongside the error")
	}
}

func TestCanonicalCategoryName(t *testing.T) {
	for alias, canonical := range map[string]string{
		"高麗菜":  "甘藍",
		"地瓜":   "甘薯",
		"青椒":   "甜椒",
		" 蕃茄 ": "番茄",
	} {
		if got := CanonicalCategoryName(alias); got != canonical {
			t.Errorf("CanonicalCategoryName(%q) = %q, want %q", alias, got, canonical)
		}
	}
	if got := CanonicalCategoryName("不存在的品項ZZZ"); got != "不存在的品項ZZZ" {
		t.Errorf("非別名應原樣回傳，got %q", got)
	}
}

// TestSynonym_CommoditySourceTerms：commodity 那批的來源術語必須有人工覆寫。
//
// 2026-09-17 dev 實測：買家問「白肉雞」被 LLM 收斂到**葡萄柚**的「白肉」品種
// （category-767），拿到 17 元/kg 的葡萄柚行情，而白肉雞真正的分類（2011，
// 對應 commodity「白肉雞（2.0 公斤以上）產地價」）是 58 元/kg——差 3.4 倍，
// 且全程 referenceAvailability=AVAILABLE、三分位齊全、零錯誤訊息。temperature 0
// 下可穩定重現。
//
// 這八個詞是農業部來源自己的用字（commodity.name），而 851 條生成別名表一條都
// 沒收，全部靠 LLM 收斂——「肉鵝」猜對了、「白肉雞」猜錯了，沒有任何機制保證。
func TestSynonym_CommoditySourceTerms(t *testing.T) {
	ci := buildTestIndex(map[string][]leafEntry{
		"雞肉": {{catalogID: "2011", path: "肉類 > 雞肉 > 雞肉"}},
		"豬肉": {{catalogID: "2008", path: "肉類 > 豬肉 > 豬肉"}},
		"鵝肉": {{catalogID: "2013", path: "肉類 > 鵝肉 > 鵝肉"}},
		"鴨肉": {{catalogID: "2012", path: "肉類 > 鴨肉 > 鴨肉"}},
		"米":  {{catalogID: "2022", path: "米 > 米 > 米"}},
	})

	for _, tc := range []struct {
		term, wantCatalogID, why string
	}{
		{"白肉雞", "2011", "來源 commodity 名，實測會被收斂到葡萄柚"},
		{"毛豬", "2008", "來源 commodity 名（毛豬（規格豬））"},
		{"肉鵝", "2013", "來源 commodity 名（肉鵝（白羅曼））"},
		{"白羅曼", "2013", "肉鵝的品種名，同一支來源"},
		{"土番鴨", "2012", "來源 commodity 名（土番鴨（75 天））"},
		{"正番鴨", "2012", "同為鴨肉分類；價格取該 L3 掛的 commodity"},
		{"稉種", "2022", "白米零售價（稉種）的品種名"},
		{"硬秈", "2022", "同為米分類；價格取該 L3 掛的 commodity"},
	} {
		results := ci.Match(tc.term, 3)
		if len(results) == 0 {
			t.Errorf("%s（%s）：零命中", tc.term, tc.why)
			continue
		}
		if results[0].CatalogID != tc.wantCatalogID {
			t.Errorf("%s（%s）：期望 catalogID %s，實得 %s", tc.term, tc.why, tc.wantCatalogID, results[0].CatalogID)
		}
	}
}

// --- PathOf：categoryID → 完整分類路徑 ---
//
// 必須涵蓋所有層級，不只 leaf：Resolve 的候選可能是 L2（實測「豬肉」第一段給
// category-198）也可能是 L3（第二段給 category-2008）。既有的 index 只扁平化到
// leaf，catalogID 一律是 L3，所以這份要另建。

func TestCategoryIndex_PathOf(t *testing.T) {
	ci := &CategoryIndex{
		pathByID:  map[string]string{"2011": "肉類 > 雞肉 > 雞肉", "198": "肉類 > 豬肉"},
		updatedAt: time.Now(),
		ready:     true,
	}
	if got := ci.PathOf("2011"); got != "肉類 > 雞肉 > 雞肉" {
		t.Errorf("PathOf(2011) = %q", got)
	}
	if got := ci.PathOf("198"); got != "肉類 > 豬肉" {
		t.Errorf("PathOf(198) = %q，L2 也要查得到", got)
	}
	if got := ci.PathOf("不存在"); got != "" {
		t.Errorf("查無應回空字串，得到 %q", got)
	}
}

func TestWalkPaths_CoversEveryLevel(t *testing.T) {
	tree := &categorypb.CategoryNode{
		Id: "1", Name: "肉類",
		Children: []*categorypb.CategoryNode{
			{Id: "198", Name: "豬肉", Children: []*categorypb.CategoryNode{
				{Id: "2008", Name: "豬肉"},
				{Id: "2257", Name: "生鮮豬肉"},
			}},
		},
	}
	out := map[string]string{}
	walkPaths(tree, "", out)

	want := map[string]string{
		"1":    "肉類",
		"198":  "肉類 > 豬肉",
		"2008": "肉類 > 豬肉 > 豬肉",
		"2257": "肉類 > 豬肉 > 生鮮豬肉",
	}
	for id, path := range want {
		if out[id] != path {
			t.Errorf("walkPaths[%s] = %q, want %q", id, out[id], path)
		}
	}
	if len(out) != len(want) {
		t.Errorf("節點數 = %d, want %d", len(out), len(want))
	}
}
