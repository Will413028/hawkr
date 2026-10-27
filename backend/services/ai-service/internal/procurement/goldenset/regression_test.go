package goldenset

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 這支守的是「評測機制本身沒壞」，不是解析品質——後者要打真實服務，由
// cmd/resolver-eval 負責（同 TestGeneratedAliases_ConsistentWithTreeSnapshot
// 對快照不對線上的理由：營運改資料不該弄紅無關的 PR）。

func loadGoldenSet(t *testing.T) []Query {
	t.Helper()
	raw, err := os.ReadFile("testdata/golden_queries.yaml")
	require.NoError(t, err)
	qs, err := Load(raw)
	require.NoError(t, err)
	return qs
}

// known_gap 只增不減就是訊號。上限刻意訂得很緊：要放寬它，先回答為什麼缺口在長大。
func TestGoldenSet_KnownGapCountIsCapped(t *testing.T) {
	gaps := []string{}
	for _, q := range loadGoldenSet(t) {
		if q.Status == StatusKnownGap {
			gaps = append(gaps, q.Text)
		}
	}
	require.LessOrEqualf(t, len(gaps), 3,
		"known_gap 目前 %v；只增不減就是訊號，要放寬上限先問為什麼缺口在長大", gaps)
}

// 查詢集要同時涵蓋會解析出分類的查詢與 NIL，否則它只能量到其中一邊。
func TestGoldenSet_CoversResolvedAndNIL(t *testing.T) {
	var resolved, nilCase int
	for _, q := range loadGoldenSet(t) {
		if q.ExpectsNIL() {
			nilCase++
		} else {
			resolved++
		}
	}
	require.GreaterOrEqual(t, resolved, 10, "解析出分類的查詢至少十條")
	require.GreaterOrEqual(t, nilCase, 2, "NIL 至少兩條——沒有它就量不到系統會不會硬給答案")
}

// 每條都要有期望值或明確是 NIL；沒有期望值的條目會靜靜地永遠通過。
func TestGoldenSet_EveryQueryIsScorable(t *testing.T) {
	for _, q := range loadGoldenSet(t) {
		if q.ExpectsNIL() {
			continue
		}
		require.Truef(t, q.ExpectCategory != "" || q.ExpectLabel != "",
			"%q 既沒有 category 也沒有 label，評不了分", q.Text)
	}
}

// 輸入空間有好幾個維度，而 2026-09-18 之前只變品項名：resolver-eval 把每條查詢
// 塞進固定句型「我要買X 50 公斤，送桃園市」，所以句子長短、標點、單位寫法、資訊
// 順序全都是常數。當天用 16 條不同形態的問句找到 5 個缺陷，而 14 條黃金集全綠。
// 這支守的是「維度不會靜靜縮回去」，不是解析品質。
func TestGoldenSet_CoversInputSpaceDimensions(t *testing.T) {
	qs := loadGoldenSet(t)

	var withMessage, exactName, aliasOrGeneric, nilRows, nilWithReason int
	var hasLatinUnit, hasChineseUnit bool
	for _, q := range qs {
		if q.Message != "" {
			withMessage++
			if strings.Contains(strings.ToLower(q.Message), "kg") {
				hasLatinUnit = true
			}
			if strings.Contains(q.Message, "公斤") {
				hasChineseUnit = true
			}
		}
		switch {
		case q.ExpectsNIL():
			nilRows++
			if q.Note != "" {
				nilWithReason++
			}
		case q.ExpectCategory != "" && strings.HasSuffix(q.ExpectCategory, " > "+q.Text):
			exactName++
		default:
			aliasOrGeneric++
		}
	}

	require.GreaterOrEqual(t, withMessage, 3,
		"至少三條要自帶 message，否則句子形態這個維度等於沒量——固定句型是常數")
	require.True(t, hasLatinUnit, "要有拉丁字母單位寫法（kg），買家真的這樣打")
	require.True(t, hasChineseUnit, "也要有中文單位（公斤）當對照，否則量不出兩種寫法會不會分岔")
	require.GreaterOrEqual(t, exactName, 1,
		"至少一條精確品名：那條路一輪定案、不出卡片，2026-09-19 實測它的分類會掉而經卡片的不會")
	require.GreaterOrEqual(t, aliasOrGeneric, 5, "別名與泛稱仍是主要形態，維持足夠條數")
	require.GreaterOrEqual(t, nilRows, 5,
		"NIL 至少五條：系統會不會硬給答案，只有這一類量得到")
	require.GreaterOrEqual(t, nilWithReason, 3,
		"NIL 條目要寫明為什麼該是 NIL（查無、鄰近字串不算答案、缺別名），否則下一個人會以為是漏填期望值")
}
