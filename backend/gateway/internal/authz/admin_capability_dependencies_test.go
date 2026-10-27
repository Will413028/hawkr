package authz

import (
	"os"
	"strings"
	"testing"
)

// 依賴閉包是 #2444 新增的業務規則，而它在 DB **沒有任何落點**：
// chk_sda_capabilities_closed 的九個 CASE 是去重加總（PG 的 CHECK 不能用子查詢做
// DISTINCT），不是依賴檢查。⇒ 只勾 products.write 不勾 products.read 的組合，DB 照收，
// 而畫面上「編輯商品」這個入口會憑空少掉一半功能，零錯誤訊號。
//
// 保證因此只有兩層：本檔的 Go 測試，與 scripts/check-delegation-capability-closure.py
// 的 Go ↔ golden 對拍。兩者都在 repo 內，兩者都必須有鑑別力。

// parseDependencyGolden 讀 golden 檔並解析成 (capability, deps) 的有序清單。
func parseDependencyGolden(t *testing.T) [][2]string {
	t.Helper()
	raw, err := os.ReadFile("testdata/admin_capability_dependencies.golden")
	if err != nil {
		t.Fatalf("讀不到依賴 golden 檔：%v", err)
	}
	var out [][2]string
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, deps, found := strings.Cut(line, "=")
		if !found {
			t.Fatalf("第 %d 行格式不正確（缺 `=`）：%q", i+1, line)
		}
		out = append(out, [2]string{strings.TrimSpace(name), strings.TrimSpace(deps)})
	}
	return out
}

// golden 與 Go 表逐行等值且順序一致。順序也比對的理由與九值那份相同：golden 是給人讀
// 的副本，順序漂掉之後逐行比對就退化成集合比對，少一層保護。
func TestAdminCapabilityDependenciesMatchGolden(t *testing.T) {
	golden := parseDependencyGolden(t)
	if len(golden) != len(AllAdminCapabilities) {
		t.Fatalf("golden 有 %d 行、capability 有 %d 個——每一個都必須明寫依賴（無依賴也要有一行）",
			len(golden), len(AllAdminCapabilities))
	}
	for i, cap := range AllAdminCapabilities {
		if golden[i][0] != string(cap) {
			t.Fatalf("第 %d 行：golden %q、Go 常數 %q（順序也必須一致）", i+1, golden[i][0], cap)
		}
		var want []string
		for _, d := range AdminCapabilityDependsOn(cap) {
			want = append(want, string(d))
		}
		if got := strings.Join(want, ", "); got != golden[i][1] {
			t.Errorf("%s 的依賴：Go %q、golden %q", cap, got, golden[i][1])
		}
	}
}

// 每一個 capability 都必須在兩張表內各有一筆。少了依賴那筆＝新增能力時沒有人被迫回答
// 「它依賴什麼」；少了風險那筆＝勾選介面對它不顯示任何危險提示，而 orders.read 這種
// 「唯讀但高風險」的項正是靠這個等級被看見的。
func TestAdminCapabilityTablesCoverEveryCapability(t *testing.T) {
	validRisks := map[string]struct{}{
		AdminCapRiskLow: {}, AdminCapRiskMedium: {}, AdminCapRiskHigh: {},
	}
	for _, cap := range AllAdminCapabilities {
		if _, ok := adminCapabilityDependencies[cap]; !ok {
			t.Errorf("%s 沒有依賴表條目", cap)
		}
		risk := AdminCapabilityRisk(cap)
		if _, ok := validRisks[risk]; !ok {
			t.Errorf("%s 的風險等級為 %q，必須是 low／medium／high 之一", cap, risk)
		}
	}
	if len(adminCapabilityDependencies) != len(AllAdminCapabilities) {
		t.Errorf("依賴表有 %d 條、capability 有 %d 個", len(adminCapabilityDependencies), len(AllAdminCapabilities))
	}
	if len(adminCapabilityRisks) != len(AllAdminCapabilities) {
		t.Errorf("風險表有 %d 條、capability 有 %d 個", len(adminCapabilityRisks), len(AllAdminCapabilities))
	}
	// 依賴項本身必須是九值之一，且不得自我依賴——自我依賴會讓閉包展開對它恆為
	// 「缺自己」，任何含它的組合都送不出去。
	for cap, deps := range adminCapabilityDependencies {
		for _, d := range deps {
			if !IsKnownAdmin(d) {
				t.Errorf("%s 依賴未知 token %q", cap, d)
			}
			if d == cap {
				t.Errorf("%s 依賴自己", cap)
			}
		}
	}
}

// AdminCapabilityDependsOn 回傳副本：呼叫端拿去 append／排序都不得動到表本身。
// 共用底層陣列的症狀是「某次請求之後，某個能力的依賴清單永久多了一項」。
func TestAdminCapabilityDependsOnReturnsCopy(t *testing.T) {
	got := AdminCapabilityDependsOn(AdminCapProductsPublish)
	if len(got) != 2 {
		t.Fatalf("products.publish 應有 2 個直接依賴，實得 %v", got)
	}
	got[0] = "tampered"
	if again := AdminCapabilityDependsOn(AdminCapProductsPublish); again[0] != AdminCapProductsRead {
		t.Fatalf("回傳值與表共用底層陣列，被呼叫端改壞了：%v", again)
	}
	// 無依賴者回空 slice 而不是 nil：呼叫端會序列化成 JSON 陣列，nil 變成 null 之後
	// 前端對 dependsOn 做 .length 就炸。
	if deps := AdminCapabilityDependsOn(AdminCapOrdersRead); deps == nil {
		t.Fatal("無依賴的能力必須回空 slice，不得回 nil")
	}
}

// R4／R5：閉包不齊時必須指名缺了什麼，而且是**全部**缺的，不是第一個。
func TestMissingAdminCapabilityDependencies(t *testing.T) {
	cases := []struct {
		name string
		have []AdminCapability
		want []AdminCapability
	}{
		{"空集合沒有缺項（空是 9260 的責任，不是 9287 的）", nil, nil},
		{"只讀不缺", []AdminCapability{AdminCapProductsRead}, nil},
		{"write 缺 read", []AdminCapability{AdminCapProductsWrite}, []AdminCapability{AdminCapProductsRead}},
		{"write 補齊", []AdminCapability{AdminCapProductsRead, AdminCapProductsWrite}, nil},
		{
			"publish 一次缺兩項（只回報第一項的話，使用者要送出兩次才知道完整答案）",
			[]AdminCapability{AdminCapProductsPublish},
			[]AdminCapability{AdminCapProductsRead, AdminCapProductsWrite},
		},
		{
			"publish 只補了 read",
			[]AdminCapability{AdminCapProductsRead, AdminCapProductsPublish},
			[]AdminCapability{AdminCapProductsWrite},
		},
		{
			"跨族群各缺各的，順序依 AllAdminCapabilities",
			[]AdminCapability{AdminCapStorefrontWrite, AdminCapPostsPublish},
			[]AdminCapability{AdminCapStorefrontRead, AdminCapPostsRead, AdminCapPostsWrite},
		},
		{"orders.read 無依賴", []AdminCapability{AdminCapOrdersRead}, nil},
		{
			"未知 token 被忽略（值域是 9260 的責任，同一個錯不給兩個碼）",
			[]AdminCapability{"products.bulk", AdminCapPostsWrite},
			[]AdminCapability{AdminCapPostsRead},
		},
		{"全選必然自洽", AllAdminCapabilities, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MissingAdminCapabilityDependencies(tc.have)
			if len(got) != len(tc.want) {
				t.Fatalf("缺項 = %v，預期 %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("缺項 = %v，預期 %v（順序也必須是 AllAdminCapabilities 的順序）", got, tc.want)
				}
			}
		})
	}
}

// 閉包展開必須是遞移的：日後若有人把 publish 的依賴改成只寫 write（讓 read 由 write
// 遞移而來），這支仍必須回報缺 read 與 write 兩項。直接讀表的實作會只回報 write。
func TestMissingAdminCapabilityDependenciesIsTransitive(t *testing.T) {
	original := adminCapabilityDependencies[AdminCapProductsPublish]
	adminCapabilityDependencies[AdminCapProductsPublish] = []AdminCapability{AdminCapProductsWrite}
	defer func() { adminCapabilityDependencies[AdminCapProductsPublish] = original }()

	got := MissingAdminCapabilityDependencies([]AdminCapability{AdminCapProductsPublish})
	want := []AdminCapability{AdminCapProductsRead, AdminCapProductsWrite}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("遞移閉包未展開：缺項 = %v，預期 %v", got, want)
	}
}
