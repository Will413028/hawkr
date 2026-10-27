package mapping_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

// realSeedPaths 指向本 repo 實際會 commit 的種子表——happy path 測試直接用它，
// 而不是重造一份假資料，才能同時驗證「LoadSeed 的邏輯正確」與「
// reference/product_crop_seed.csv 與 .manifest.json 這兩個檔案彼此一致」，
// 後者正是 CI 唯一會抓到「CSV 改了但忘記重算 manifest」的地方。
func realSeedPaths(t *testing.T) (csvPath, manifestPath string) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "reference"))
	require.NoError(t, err)
	return filepath.Join(dir, "product_crop_seed.csv"), filepath.Join(dir, "product_crop_seed.manifest.json")
}

// writeSeedFixture 把 csvContent 寫進暫存目錄，並產生與其內容相符（或依
// wrongDigest 蓄意錯誤）的 manifest，回傳兩個檔案路徑。失敗案例的測試用它
// 造出「CSV 合法但內容有問題」或「manifest 對不上」的組合。
func writeSeedFixture(t *testing.T, csvContent string, wrongDigest string) (csvPath, manifestPath string) {
	t.Helper()
	dir := t.TempDir()
	csvPath = filepath.Join(dir, "seed.csv")
	manifestPath = filepath.Join(dir, "seed.manifest.json")

	require.NoError(t, os.WriteFile(csvPath, []byte(csvContent), 0o644))

	digest := wrongDigest
	if digest == "" {
		sum := sha256.Sum256([]byte(csvContent))
		digest = hex.EncodeToString(sum[:])
	}
	manifestBytes, err := json.Marshal(map[string]any{
		"schema":    "hawkr.product-crop-seed/v1",
		"file":      "seed.csv",
		"sha256":    digest,
		"row_count": 1,
		"sources":   []string{"test fixture"},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, manifestBytes, 0o644))
	return csvPath, manifestPath
}

const seedHeader = "alias_name,match_kind,crop_group_name,crop_code,market_code,note\n"

// TestLoadSeedRealFixtureLoadsAndResolves 是 happy path：用 repo 實際會 commit
// 的 CSV/manifest 驗證 LoadSeed 成功，且幾條有代表性的別名能透過 Resolve
// 查到——包含未正規化（直接命中）與需要正規化（modifier 前綴／規格尾綴／
// 括號）才能命中的情形。
func TestLoadSeedRealFixtureLoadsAndResolves(t *testing.T) {
	csvPath, manifestPath := realSeedPaths(t)

	seed, err := mapping.LoadSeed(csvPath, manifestPath)
	require.NoError(t, err)
	require.Equal(t, 27, seed.Len(), "reference/product_crop_seed.csv 目前應有 27 列種子資料；若此斷言失敗，代表 CSV 列數變了但測試沒有同步更新")

	entry, ok := seed.Resolve("高麗菜")
	require.True(t, ok)
	require.Equal(t, mapping.MatchKindConsumerAlias, entry.MatchKind)
	require.Equal(t, "甘藍", entry.CropGroupName)
	require.Empty(t, entry.CropCode)

	// category_l3 列：分類樹名稱本身即 MOA 大類名。
	catEntry, ok := seed.Resolve("甘藍")
	require.True(t, ok)
	require.Equal(t, mapping.MatchKindCategoryL3, catEntry.MatchKind)
	require.Equal(t, "甘藍", catEntry.CropGroupName)

	// 已知分歧：大白菜以 MOA「包心白菜」為準，不是分類樹的「包心白」。
	divergent, ok := seed.Resolve("大白菜")
	require.True(t, ok)
	require.Equal(t, "包心白菜", divergent.CropGroupName)

	// 只存在於 category_index synonyms 的三個分歧項。
	for alias, want := range map[string]string{
		"香菇":  "濕香菇",
		"青花菜": "青花苔",
		"蔥":   "青蔥",
	} {
		e, ok := seed.Resolve(alias)
		require.Truef(t, ok, "alias=%s 應可解析", alias)
		require.Equalf(t, want, e.CropGroupName, "alias=%s", alias)
	}
}

// TestLoadSeedDigestMismatchIsHardError 驗證種子表被改過但 manifest 沒同步
// 更新時，LoadSeed 回 error（不是印警告後繼續載入）。
func TestLoadSeedDigestMismatchIsHardError(t *testing.T) {
	content := seedHeader + "高麗菜,consumer_alias,甘藍,,,\"note\"\n"
	csvPath, manifestPath := writeSeedFixture(t, content, "0000000000000000000000000000000000000000000000000000000000000000")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
	require.ErrorIs(t, err, mapping.ErrDigestMismatch)
}

// TestLoadSeedBothCropFieldsFilledIsError 驗證 crop_group_name 與 crop_code
// 都填時回 error（必須恰填一個）。
func TestLoadSeedBothCropFieldsFilledIsError(t *testing.T) {
	content := seedHeader + "高麗菜,consumer_alias,甘藍,LA1,,\"note\"\n"
	csvPath, manifestPath := writeSeedFixture(t, content, "")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
	require.ErrorIs(t, err, mapping.ErrFieldsNotExclusive)
}

// TestLoadSeedBothCropFieldsEmptyIsError 驗證 crop_group_name 與 crop_code
// 都空時回 error（同樣違反「恰填一個」）。
func TestLoadSeedBothCropFieldsEmptyIsError(t *testing.T) {
	content := seedHeader + "高麗菜,consumer_alias,,,,\"note\"\n"
	csvPath, manifestPath := writeSeedFixture(t, content, "")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
	require.ErrorIs(t, err, mapping.ErrFieldsNotExclusive)
}

// TestLoadSeedDuplicateAliasIsError 驗證同一個 alias_name 出現兩次時回 error。
func TestLoadSeedDuplicateAliasIsError(t *testing.T) {
	content := seedHeader +
		"高麗菜,consumer_alias,甘藍,,,\"note 1\"\n" +
		"高麗菜,consumer_alias,甘藍,,,\"note 2（重複）\"\n"
	csvPath, manifestPath := writeSeedFixture(t, content, "")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
	require.ErrorIs(t, err, mapping.ErrDuplicateAlias)
}

// TestLoadSeedUnknownMatchKindIsError 驗證 match_kind 不是 consumer_alias /
// category_l3 之一時回 error（不悄悄接受未知值）。
func TestLoadSeedUnknownMatchKindIsError(t *testing.T) {
	content := seedHeader + "高麗菜,fuzzy_guess,甘藍,,,\"note\"\n"
	csvPath, manifestPath := writeSeedFixture(t, content, "")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
	require.ErrorIs(t, err, mapping.ErrUnknownMatchKind)
}

// TestLoadSeedMissingCSVColumnIsError 驗證某一列欄位數不足（漏 market_code
// 欄）時回 error，而不是把後面欄位的值錯位塞進前面欄位。
func TestLoadSeedMissingCSVColumnIsError(t *testing.T) {
	// 只有 5 欄（缺 note），對照 expectedHeader 的 6 欄。
	content := seedHeader + "高麗菜,consumer_alias,甘藍,,\n"
	csvPath, manifestPath := writeSeedFixture(t, content, "")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
}

// TestLoadSeedEmptyAliasIsError 驗證 alias_name 空字串時回 error。
func TestLoadSeedEmptyAliasIsError(t *testing.T) {
	content := seedHeader + ",consumer_alias,甘藍,,,\"note\"\n"
	csvPath, manifestPath := writeSeedFixture(t, content, "")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
	require.ErrorIs(t, err, mapping.ErrEmptyAlias)
}

// TestLoadSeedNoDataRowsIsError 驗證只有表頭、沒有任何資料列時回 error。
func TestLoadSeedNoDataRowsIsError(t *testing.T) {
	csvPath, manifestPath := writeSeedFixture(t, seedHeader, "")

	_, err := mapping.LoadSeed(csvPath, manifestPath)
	require.Error(t, err)
	require.ErrorIs(t, err, mapping.ErrNoRows)
}

// TestResolveNormalizesModifierPrefixBracketAndSpec 驗證 Resolve 的三種
// 正規化步驟——去修飾前綴、去括號內容、去帶單位的規格尾綴——分別與疊加時
// 都能命中同一個 alias_name。
func TestResolveNormalizesModifierPrefixBracketAndSpec(t *testing.T) {
	csvPath, manifestPath := realSeedPaths(t)
	seed, err := mapping.LoadSeed(csvPath, manifestPath)
	require.NoError(t, err)

	cases := []struct {
		name  string
		input string
	}{
		{"去修飾前綴", "新鮮高麗菜"},
		{"去修飾前綴（疊加）", "有機新鮮高麗菜"},
		{"去括號說明", "高麗菜（宜蘭）"},
		{"去半形括號", "高麗菜(宜蘭)"},
		{"去帶空白的規格尾綴", "高麗菜 600g"},
		{"去無空白的規格尾綴", "高麗菜600g"},
		{"三者疊加", "有機新鮮高麗菜（宜蘭）600g"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, ok := seed.Resolve(tc.input)
			require.Truef(t, ok, "input=%q 正規化後應命中「高麗菜」", tc.input)
			require.Equal(t, "甘藍", entry.CropGroupName)
		})
	}
}

// TestResolveDoesNotFuzzyMatch 是本套件最重要的行為斷言之一：正規化之後
// 找不到「完全相同」的 alias_name 時，Resolve 必須回 false，不能用子字串、
// 編輯距離或任何近似邏輯硬湊一個答案。
func TestResolveDoesNotFuzzyMatch(t *testing.T) {
	csvPath, manifestPath := realSeedPaths(t)
	seed, err := mapping.LoadSeed(csvPath, manifestPath)
	require.NoError(t, err)

	cases := []struct {
		name  string
		input string
	}{
		{"表內沒有的相近字（甘藍菜非種子表用字）", "甘藍菜"},
		{"表內沒有的完全不同作物", "西瓜"},
		{"整串都是修飾詞，正規化後為空字串", "有機新鮮"},
		{"整串是修飾詞加規格，正規化後為空字串", "新鮮 600g"},
		{"單一個字，正規化後仍非種子表用字", "菜"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := seed.Resolve(tc.input)
			require.Falsef(t, ok, "input=%q 不應被 Resolve 命中任何列（沒有模糊比對）", tc.input)
		})
	}
}

// TestResolveCaseSensitiveDistinctAliases 驗證「A菜」與「a菜」是兩個不同的
// alias_name（大小寫不折疊），對齊 ai-service cropNameAliases 把兩者列為
// 獨立條目的做法——Resolve 不做任何形式的模糊正規化，大小寫折疊也算一種。
func TestResolveCaseSensitiveDistinctAliases(t *testing.T) {
	csvPath, manifestPath := realSeedPaths(t)
	seed, err := mapping.LoadSeed(csvPath, manifestPath)
	require.NoError(t, err)

	upper, ok := seed.Resolve("A菜")
	require.True(t, ok)
	require.Equal(t, "萵苣菜", upper.CropGroupName)

	lower, ok := seed.Resolve("a菜")
	require.True(t, ok)
	require.Equal(t, "萵苣菜", lower.CropGroupName)

	_, ok = seed.Resolve("B菜")
	require.False(t, ok, "大小寫不折疊，B菜 不應命中任何列")
}
