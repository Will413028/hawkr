package cache

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// 這支就是 PR gate 本體。刻意用 go test 實作而不是新增 CI step：Go Checks
// 本來就跑，不必新增 workflow，也避免「新 gate 對舊 merge base 不存在」的缺項。
//
// 對照的是與別名表一起 commit 的樹快照，不是線上的樹——打線上樹的 gate 會讓
// 營運改一個分類名就把無關的 PR 弄紅。線上漂移由每日排程稽核負責。
func TestGeneratedAliases_ConsistentWithTreeSnapshot(t *testing.T) {
	raw, err := os.ReadFile("testdata/category_tree_snapshot.json")
	require.NoError(t, err)
	snap, err := LoadTreeSnapshot(raw)
	require.NoError(t, err)

	nodeNames := snap.NodeNames()
	l2 := snap.L2ByName()

	for alias, target := range generatedCategoryAliases {
		t.Run(alias, func(t *testing.T) {
			require.False(t, nodeNames[alias],
				"別名 %q 撞到既有節點名——會讓買家再也命中不了那個節點", alias)
			_, ok := l2[target]
			require.True(t, ok, "別名 %q 的 target %q 不是快照裡的 L2", alias, target)
			require.Empty(t, synonyms[alias],
				"別名 %q 已被人工覆寫層擁有，生成表不該重複", alias)
		})
	}
}

func TestManualAliasKeys_ReturnsACopy(t *testing.T) {
	got := ManualAliasKeys()
	require.Equal(t, "甘藍", got["高麗菜"])

	got["高麗菜"] = "竄改"
	require.Equal(t, "甘藍", synonyms["高麗菜"], "回傳的必須是複本，不能讓呼叫端改到本尊")
}

// TestManualSynonymTargets_ExistInTreeSnapshot 是人工覆寫層的對應 gate。
//
// 生成層那支只驗 generatedCategoryAliases；synonyms 是手寫的，原本沒有任何機制
// 檢查 target 真的存在——寫錯或分類改名之後，該條 synonym 會靜默失效（命中
// synonym、然後查不到節點 ⇒ 零命中），沒有錯誤訊息。2026-09-17 加這道時存量就有
// 一條這種死條目（`鳳眼果→蘋婆`，快照與線上樹都查無「蘋婆」），同一刀清掉。
//
// 對照快照而不是線上樹，理由與生成層那支相同：線上改名不該弄紅無關的 PR。
//
// ⚠️ 刻意**不**驗「alias 撞既有節點名」——那是生成層的規則。人工層正好相反：把某個
// L3 節點名導到它的 L2 是它的用途之一（`文旦→柚子`、`玉米筍→玉米`，見
// TestSynonym_PomeloDirection），加那條會擋掉一票合法覆寫。
func TestManualSynonymTargets_ExistInTreeSnapshot(t *testing.T) {
	raw, err := os.ReadFile("testdata/category_tree_snapshot.json")
	require.NoError(t, err)
	snap, err := LoadTreeSnapshot(raw)
	require.NoError(t, err)

	nodeNames := snap.NodeNames()
	for alias, target := range synonyms {
		t.Run(alias, func(t *testing.T) {
			require.True(t, nodeNames[target],
				"人工別名 %q 的 target %q 不在快照的節點名裡——這條會靜默失效（命中後查不到節點）", alias, target)
		})
	}
}
