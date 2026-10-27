// Package mapping 載入並查詢「賣家商品名 → 農業部 (MOA) 作物」的種子對照表
// （P4 Task 13，設計書 §7 commodity-mapping 的 v1 做法：手工策劃 10–30 個
// 高價值品項，不做自動名稱比對）。
//
// 範圍邊界（讀這段前不要假設本套件做得比這裡寫的多）：
//   - 只有種子表的載入與名稱解析。不建 product_crop_mapping 表、不接
//     product-service——mapping 的讀寫（商品實際綁定哪個作物）歸 Plan C，
//     P4 建一張沒有讀者的表是投機。
//   - CSV 本身是唯一真相（source of truth）。同目錄的
//     product_crop_seed.manifest.json 只是完整性校驗用的 sha256 + 來源清單，
//     不是另一份資料。
//
// 資料來源（都是 repo 內已人工逐列驗證過的資料，不是本套件發明的）：
//   - services/ai-service/internal/client/moa/cropname.go 的
//     cropNameAliases：俗名 → MOA 官方作物名，每一條都附 2026-07-24 實測的
//     命中列數。
//   - services/ai-service/internal/cache/category_index.go 的 synonyms：
//     消費者用詞 → 我方分類樹實際存在的 L2/L3 名稱。
//   - 兩張表已知分歧（大白菜／香菇／青花菜／蔥）已在 CSV 的 note 欄逐條裁決，
//     裁決原則見 reference/README.md：本表的目的是對應到
//     crop_group.name / crop.crop_code（我方 DB 從 MOA 行情資料建的），
//     分歧一律以 MOA 官方名為準。
package mapping

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// MatchKind 標記一條種子列的來源類別。兩者都是「名稱 → 作物」，但來源不同、
// 可信度不同——consumer_alias 是消費者可能打出來的俗名（需要正規化 + 轉換
// 才對得上 MOA），category_l3 是我方分類樹的 L2/L3 名稱本身恰好等於 MOA
// 官方作物名（不需轉換，但仍值得記一列，證明「這個名字直接可用」是查證過的
// 事實而非巧合）。混在一起會讓日後的維護無法判斷該不該改某一列。
type MatchKind string

const (
	MatchKindConsumerAlias MatchKind = "consumer_alias"
	MatchKindCategoryL3    MatchKind = "category_l3"
)

var knownMatchKinds = map[MatchKind]bool{
	MatchKindConsumerAlias: true,
	MatchKindCategoryL3:    true,
}

// SeedEntry 是種子表的一列。CropGroupName 與 CropCode 恰有一個非空：填前者是
// 大類對應（多數情形——分類樹 L3 名對上的是 crop_group.name「甘藍」而不是
// crop.name「甘藍-初秋」），填後者是指定到特定子品項。MarketCode 空字串代表
// 用預設市場。
type SeedEntry struct {
	AliasName     string
	MatchKind     MatchKind
	CropGroupName string
	CropCode      string
	MarketCode    string
	Note          string
}

// Seed 是載入後的種子表：entries 保留 CSV 原始順序供逐列檢查（例如
// cmd/pricebook-seed-check 要把每一列都拿去打 DB），index 供 Resolve 做
// 正規化後的精確查表。
type Seed struct {
	entries []SeedEntry
	index   map[string]SeedEntry
}

// Entries 回傳種子表全部列，依 CSV 原始順序。
func (s Seed) Entries() []SeedEntry {
	return s.entries
}

// Len 回傳種子表列數。
func (s Seed) Len() int {
	return len(s.entries)
}

// 驗證失敗的錯誤分類。用 errors.Is 而非只比對錯誤字串，讓呼叫端（含測試）能
// 精確分辨是哪一類驗證失敗，不必依賴訊息措辭。
var (
	// ErrDigestMismatch 是最重要的一條：manifest 記錄的 sha256 與 CSV 實際內容
	// 不符，代表種子表被改過而 manifest 沒有同步更新——也就是有人繞過了審查
	// 流程。這裡刻意回 error 而不是印警告後繼續：警告會被忽略，error 會讓
	// LoadSeed 的呼叫端（服務啟動、seed-check CLI）硬性中止。
	ErrDigestMismatch = errors.New("mapping: 種子表 sha256 與 manifest 不符")
	// ErrUnknownMatchKind 是 match_kind 欄位不是 consumer_alias / category_l3
	// 其中之一。
	ErrUnknownMatchKind = errors.New("mapping: 未知的 match_kind")
	// ErrFieldsNotExclusive 是 crop_group_name 與 crop_code 沒有恰填一個
	// （兩者都填或兩者都空）。
	ErrFieldsNotExclusive = errors.New("mapping: crop_group_name 與 crop_code 必須恰填一個")
	// ErrDuplicateAlias 是同一個 alias_name 出現超過一次。重複會讓 Resolve
	// 的查表結果取決於「CSV 裡哪一列比較晚被讀到」這種未定義行為，必須擋在
	// 載入階段。
	ErrDuplicateAlias = errors.New("mapping: alias_name 重複")
	// ErrEmptyAlias 是 alias_name 欄位是空字串。
	ErrEmptyAlias = errors.New("mapping: alias_name 為空")
	// ErrNoRows 是 CSV 除了表頭外沒有任何資料列。
	ErrNoRows = errors.New("mapping: 種子表沒有任何資料列")
)

// seedManifest 對應 product_crop_seed.manifest.json 的欄位。
type seedManifest struct {
	Schema   string   `json:"schema"`
	File     string   `json:"file"`
	SHA256   string   `json:"sha256"`
	RowCount int      `json:"row_count"`
	Sources  []string `json:"sources"`
}

// expectedHeader 是 CSV 的欄位契約，順序也是契約的一部分（LoadSeed 依位置
// 讀欄，不是依欄名查找）。
var expectedHeader = []string{
	"alias_name", "match_kind", "crop_group_name", "crop_code", "market_code", "note",
}

// LoadSeed 讀取 csvPath 與 manifestPath，驗證兩者一致且每一列合法，回傳可查詢
// 的 Seed。
//
// 驗證項目（任一失敗即整份拒收，不做部分載入——種子表是給定價建議用的，
// 載入一半的資料表比完全沒有資料表更危險）：
//  1. csvPath 的 sha256 必須與 manifestPath 記錄的一致（ErrDigestMismatch）。
//  2. CSV 表頭必須恰為 expectedHeader（順序、欄位皆同）。
//  3. 每一列的 match_kind 必須是已知值（ErrUnknownMatchKind）。
//  4. 每一列的 crop_group_name 與 crop_code 恰填一個（ErrFieldsNotExclusive）。
//  5. 每一列的 alias_name 非空（ErrEmptyAlias）且不重複（ErrDuplicateAlias）。
//  6. CSV 格式本身合法（encoding/csv 對欄位數不一致的列回傳的錯誤原樣往外傳）。
func LoadSeed(csvPath, manifestPath string) (Seed, error) {
	csvBytes, err := os.ReadFile(csvPath)
	if err != nil {
		return Seed{}, fmt.Errorf("讀取種子表 %s 失敗: %w", csvPath, err)
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return Seed{}, fmt.Errorf("讀取 manifest %s 失敗: %w", manifestPath, err)
	}

	var mf seedManifest
	if err := json.Unmarshal(manifestBytes, &mf); err != nil {
		return Seed{}, fmt.Errorf("解析 manifest %s 失敗: %w", manifestPath, err)
	}

	sum := sha256.Sum256(csvBytes)
	digest := hex.EncodeToString(sum[:])
	if digest != mf.SHA256 {
		return Seed{}, fmt.Errorf("%w：%s 實際 sha256=%s，manifest %s 記錄=%s（種子表被改過但 manifest 未同步更新，代表繞過了審查流程，拒絕載入；改動流程見 reference/README.md）",
			ErrDigestMismatch, csvPath, digest, manifestPath, mf.SHA256)
	}

	r := csv.NewReader(bytes.NewReader(csvBytes))
	header, err := r.Read()
	if err != nil {
		return Seed{}, fmt.Errorf("讀取 %s 表頭失敗: %w", csvPath, err)
	}
	if !equalHeader(header, expectedHeader) {
		return Seed{}, fmt.Errorf("%s 表頭不符預期：得到 %v，預期 %v", csvPath, header, expectedHeader)
	}
	// 強制之後每一列的欄位數都必須與表頭相同；欄位數不符時 r.Read() 會直接
	// 回傳 csv.ErrFieldCount，這裡不用另外手動檢查「CSV 缺欄位」。
	r.FieldsPerRecord = len(expectedHeader)

	var entries []SeedEntry
	seenAlias := make(map[string]int) // alias_name -> 出現的列號（第一次），用於重複時的錯誤訊息
	index := make(map[string]SeedEntry)

	rowNum := 1 // 第 1 列是表頭，資料列從第 2 列起算，對齊人眼在編輯器看到的行號
	for {
		rowNum++
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Seed{}, fmt.Errorf("%s 第 %d 列格式錯誤: %w", csvPath, rowNum, err)
		}

		entry := SeedEntry{
			AliasName:     strings.TrimSpace(record[0]),
			MatchKind:     MatchKind(strings.TrimSpace(record[1])),
			CropGroupName: strings.TrimSpace(record[2]),
			CropCode:      strings.TrimSpace(record[3]),
			MarketCode:    strings.TrimSpace(record[4]),
			Note:          strings.TrimSpace(record[5]),
		}

		if entry.AliasName == "" {
			return Seed{}, fmt.Errorf("%s 第 %d 列: %w", csvPath, rowNum, ErrEmptyAlias)
		}
		if !knownMatchKinds[entry.MatchKind] {
			return Seed{}, fmt.Errorf("%s 第 %d 列（alias_name=%s）: %w：%q（僅接受 %s / %s）",
				csvPath, rowNum, entry.AliasName, ErrUnknownMatchKind, entry.MatchKind,
				MatchKindConsumerAlias, MatchKindCategoryL3)
		}

		hasGroup := entry.CropGroupName != ""
		hasCrop := entry.CropCode != ""
		if hasGroup == hasCrop { // 同為 true（都填）或同為 false（都空）皆非法
			return Seed{}, fmt.Errorf("%s 第 %d 列（alias_name=%s）: %w（實際 crop_group_name=%q crop_code=%q）",
				csvPath, rowNum, entry.AliasName, ErrFieldsNotExclusive, entry.CropGroupName, entry.CropCode)
		}

		if prevRow, dup := seenAlias[entry.AliasName]; dup {
			return Seed{}, fmt.Errorf("%s 第 %d 列（alias_name=%s）與第 %d 列: %w",
				csvPath, rowNum, entry.AliasName, prevRow, ErrDuplicateAlias)
		}
		seenAlias[entry.AliasName] = rowNum

		entries = append(entries, entry)
		index[entry.AliasName] = entry
	}

	if len(entries) == 0 {
		return Seed{}, fmt.Errorf("%s: %w", csvPath, ErrNoRows)
	}

	return Seed{entries: entries, index: index}, nil
}

func equalHeader(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if strings.TrimSpace(got[i]) != want[i] {
			return false
		}
	}
	return true
}

// ---- 名稱正規化（Resolve 用；不是模糊比對，見下方大段註解）----

// modifierPrefixes、bracketedRe、spacedSpecRe、gluedSpecRe 逐一比照
// services/ai-service/internal/client/moa/cropname.go 的
// normalizeCropName／modifierPrefixes：ai-service 與 pricebook-service 是各自
// 獨立的 Go module（各自的 go.mod），彼此的 internal 套件無法跨 module
// 匯入，只能複製同一份正規化邏輯——來源清單已由 ai-service 團隊逐條對 MOA
// API 實測驗證過（見該檔案頂端註解與 cropNameAliases 各條目的命中列數
// 註記），這裡不重新發明，只原樣搬過來。修改前綴清單或正規表達式前，先確認
// 來源檔案是否已經更新，避免兩邊語意漂移。
var modifierPrefixes = []string{
	"產地直送", "有機轉型期", "無毒", "有機", "新鮮", "嚴選", "精選", "特級", "頂級",
	"優質", "台灣", "臺灣", "國產", "本土", "冷藏", "冷凍", "當季", "現採",
}

var (
	// 括號內容通常是產地或規格說明，例：甘藍（宜蘭）
	bracketedRe = regexp.MustCompile(`[（(【\[][^）)】\]]*[）)】\]]`)
	// 有空白分隔的尾綴數量，單位可省略，例：「甘藍 600g」「甘藍 3」
	spacedSpecRe = regexp.MustCompile(`\s+\d+(\.\d+)?\s*(公斤|公克|台斤|市斤|kg|KG|Kg|g|G|斤|兩|包|盒|箱|入|顆|把|串|片|條|袋)?$`)
	// 無空白分隔時必須帶單位才視為規格，避免誤刪「甘薯-臺農57號」這類官方名尾數
	gluedSpecRe = regexp.MustCompile(`\d+(\.\d+)?\s*(公斤|公克|台斤|市斤|kg|KG|Kg|g|G|斤|兩|包|盒|箱|入|顆|把|串|片|條|袋)$`)
)

// normalizeProductName 去掉括號說明、規格尾綴與修飾前綴，邏輯與
// ai-service/internal/client/moa/cropname.go 的 normalizeCropName 逐步對應。
func normalizeProductName(name string) string {
	out := bracketedRe.ReplaceAllString(name, "")
	out = strings.ReplaceAll(out, "　", " ") // 全形空白
	out = strings.TrimSpace(out)

	// 規格可能疊加（例：「甘藍 600g 2入」），去到不再變動為止。
	for {
		trimmed := spacedSpecRe.ReplaceAllString(out, "")
		trimmed = gluedSpecRe.ReplaceAllString(trimmed, "")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == out {
			break
		}
		out = trimmed
	}

	// 修飾詞同樣可能疊加（例：「有機新鮮甘藍」）。
	for {
		stripped := out
		for _, prefix := range modifierPrefixes {
			stripped = strings.TrimPrefix(stripped, prefix)
		}
		stripped = strings.TrimSpace(stripped)
		if stripped == out {
			break
		}
		out = stripped
	}

	return out
}

// Resolve 對 name（賣家商品名）做正規化後，在種子表裡查完全相同的
// alias_name。找不到回 (SeedEntry{}, false)。
//
// 🔴 正規化 vs 模糊比對的分界——讀這段前不要在這裡加任何「順便」的模糊邏輯，
// 這是刻意的邊界，不是還沒做完：
//
//   - 正規化＝去掉「已知一定不是作物名一部分」的裝飾：修飾前綴（"新鮮"、
//     "有機"…）、括號內的產地/規格說明、帶單位的規格尾綴（"600g"、"2入"）。
//     這些字串片段的存在與否不會改變賣家指的是哪個作物——去掉它們是「還原」，
//     不是「猜測」，因為規則是確定性的、雙向可解釋的（拿掉「新鮮」不會讓
//     「高麗菜」變成別的作物）。
//
//   - 模糊比對＝在「正規化後仍找不到完全相同的字串」時，用編輯距離、子字串
//     包含、部分比對，或任何形式的「猜哪個最像」去湊一個答案。這裡不做。
//
//   - 為什麼不做：設計書 §7 明講現有 get_seasonal_price_suggestion
//     （services/ai-service/internal/agent/tools.go）就是自動比對的失敗版本
//     ——它把賣家商品名整串直接當 CropName 丟給 MOA API 做子字串查詢。
//     這個做法的兩種失敗模式都已由 ai-service moa/cropname.go 的頂端註解
//     實測記錄：長字串比對不到（"高麗菜"／"新鮮高麗菜"／"高麗菜 600g" 全部
//     回 0 列，因為官方名是「甘藍」）；短字串比對到錯的東西（"蔥" 會同時
//     命中「青蔥-北蔥」與「洋蔥」）。兩種失敗模式都不是「差一點對」，加
//     fuzzy 只會把「查無結果」換成「查到錯的」，而給錯作物的定價建議比不給
//     建議更糟（設計書 §7 原文：「不產生假建議」）。
//
//   - 找不到完全相同的 alias_name 時，呼叫端必須走「未對應」流程（UI 顯示
//     unmapped + 「設定對應」入口），不能自己再做任何形式的近似猜測。
func (s Seed) Resolve(name string) (SeedEntry, bool) {
	normalized := normalizeProductName(strings.TrimSpace(name))
	if normalized == "" {
		// 整串都是修飾詞或規格（例：「有機」「新鮮 600g」），沒有作物本體可查。
		return SeedEntry{}, false
	}
	entry, ok := s.index[normalized]
	return entry, ok
}
