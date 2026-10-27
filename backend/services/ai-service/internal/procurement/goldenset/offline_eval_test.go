package goldenset

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

// 離線評測：用本機種子分類樹（backend/infra/seed-data/04-product.sql，33 節點）當分類樹，
// 不接任何外部服務、模型為 nil，逐筆對黃金查詢集呼叫 Resolve。量的是「分類樹召回＋決策」
// 本身；線上品質仍由 cmd/resolver-eval 負責。

const seedProductSQL = "../../../../../infra/seed-data/04-product.sql"

// seedCategory 是種子分類的一列。id 1~3 由 Java 時期的 V1 migration 建立、不在種子檔裡，
// 所以這張表是替身；TestSeedTreeFake_MatchesSeedSQL 守它與種子檔不分岔。
type seedCategory struct {
	id, name, parent string
}

var seedTree = []seedCategory{
	{"1", "蔬菜", ""}, {"2", "水果", ""}, {"3", "肉類", ""},
	{"10", "海鮮", ""}, {"13", "茶葉", ""}, {"15", "加工食品", ""},
	{"4", "葉菜類", "1"}, {"5", "根莖類", "1"},
	{"6", "熱帶水果", "2"}, {"7", "溫帶水果", "2"},
	{"8", "豬肉", "3"}, {"9", "雞肉", "3"},
	{"11", "蝦蟹", "10"}, {"12", "魚類", "10"}, {"33", "貝類", "10"},
	{"14", "烏龍茶", "13"}, {"16", "果乾蜜餞", "15"},
	{"17", "高麗菜", "4"}, {"18", "小白菜", "4"}, {"19", "沙拉葉菜", "4"},
	{"20", "地瓜", "5"}, {"21", "紅蘿蔔", "5"},
	{"22", "芒果", "6"}, {"23", "水蜜桃", "7"},
	{"24", "黑毛豬五花", "8"}, {"25", "土雞", "9"},
	{"26", "明蝦", "11"}, {"27", "海膽醬", "11"},
	{"28", "透抽", "12"}, {"29", "黑鮪魚", "12"},
	{"30", "高山烏龍", "14"}, {"31", "鮮蚵", "33"}, {"32", "芒果乾", "16"},
}

// categoryTreeFake 回傳固定的種子分類樹；pricebook 在這裡沒有任何替身，因為解析路徑
// 不該有它。
type categoryTreeFake struct{}

func (categoryTreeFake) GetCategoryTree(context.Context, string) (*categorypb.GetCategoryTreeResponse, error) {
	active := codemeta.CommonStatusActive.Value
	nodes := map[string]*categorypb.CategoryNode{}
	for _, c := range seedTree {
		nodes[c.id] = &categorypb.CategoryNode{Id: c.id, Name: c.name, ParentId: c.parent, Status: active}
	}
	var roots []*categorypb.CategoryNode
	for _, c := range seedTree {
		if c.parent == "" {
			roots = append(roots, nodes[c.id])
			continue
		}
		p := nodes[c.parent]
		p.Children = append(p.Children, nodes[c.id])
	}
	return &categorypb.GetCategoryTreeResponse{Categories: roots}, nil
}

func seedPaths() map[string]string {
	byID := map[string]seedCategory{}
	for _, c := range seedTree {
		byID[c.id] = c
	}
	paths := map[string]string{}
	for _, c := range seedTree {
		var parts []string
		for cur, ok := c, true; ok; cur, ok = byID[cur.parent] {
			parts = append([]string{cur.name}, parts...)
			if cur.parent == "" {
				break
			}
		}
		paths[c.id] = strings.Join(parts, " > ")
	}
	return paths
}

var seedInsertRow = regexp.MustCompile(`\(\s*(\d+),\s*'([^']+)',\s*(\d+|NULL),`)

func TestSeedTreeFake_MatchesSeedSQL(t *testing.T) {
	raw, err := os.ReadFile(seedProductSQL)
	require.NoError(t, err)
	want := map[string]seedCategory{}
	for _, c := range seedTree {
		want[c.id] = c
	}
	var inSQL int
	for _, block := range strings.Split(string(raw), "INSERT INTO category ")[1:] {
		block = block[:strings.Index(block, ";")]
		for _, m := range seedInsertRow.FindAllStringSubmatch(block, -1) {
			parent := m[3]
			if parent == "NULL" {
				parent = ""
			}
			c, ok := want[m[1]]
			require.Truef(t, ok, "種子檔的分類 %s（%s）不在測試替身裡", m[1], m[2])
			require.Equalf(t, m[2], c.name, "分類 %s 名稱", m[1])
			require.Equalf(t, parent, c.parent, "分類 %s 的父節點", m[1])
			inSQL++
		}
	}
	require.Equal(t, len(seedTree)-3, inSQL, "替身多出來的只有 V1 建立的 id 1~3")
	require.Len(t, seedTree, 33)
}

type evalRow struct {
	query    string
	expect   string
	actual   string
	hit      bool
	observed Observation
}

func describeResult(res cropidentity.DisambiguationResult, o Observation) string {
	switch {
	case o.IsNIL:
		return "NIL（逃生艙）"
	case o.Unmappable:
		return "UNMAPPABLE"
	case res.NeedsDisambiguation:
		return "出卡：" + strings.Join(o.RankedCategories, " ／ ")
	default:
		return "定案：" + strings.Join(o.RankedCategories, " ／ ")
	}
}

func TestOfflineEval_SeedTreeRecallMeetsThreshold(t *testing.T) {
	queries := loadGoldenSet(t)
	paths := seedPaths()
	resolver := cropidentity.NewDisambiguationUseCase(categoryTreeFake{}, nil)
	pathOf := func(id string) string { return paths[id] }

	obs := make([]Observation, 0, len(queries))
	rows := make([]evalRow, 0, len(queries))
	unresolved := 0
	for _, q := range queries {
		res, err := resolver.Resolve(context.Background(), q.Text)
		require.NoErrorf(t, err, "查詢 %q", q.Text)
		o := ObserveFrom(q, res, pathOf)
		obs = append(obs, o)
		if len(o.RankedCategories) == 0 && !q.ExpectsNIL() {
			unresolved++
		}
		single := Evaluate([]Observation{o}, 10)
		rows = append(rows, evalRow{
			query: q.Text, expect: q.ExpectDescription(), actual: describeResult(res, o),
			hit: len(single.Failures) == 0 && single.RecallAtK == 1, observed: o,
		})
	}

	report := Evaluate(obs, 10)
	var b strings.Builder
	fmt.Fprintf(&b, "%-14s | %-26s | %-48s | 命中\n", "查詢", "期望", "實際")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-14s | %-26s | %-48s | %v\n", r.query, r.expect, r.actual, r.hit)
	}
	fmt.Fprintf(&b, "total=%d precision@1=%.3f recall@10=%.3f mrr=%.3f unmappable=%d unresolvedNonNIL=%d\n",
		report.Total, report.PrecisionAt1, report.RecallAtK, report.MRR, report.Unmappable, unresolved)
	t.Log("\n" + b.String())
	if path := os.Getenv("OFFLINE_EVAL_REPORT"); path != "" {
		require.NoError(t, os.WriteFile(path, []byte(b.String()), 0o644))
	}

	require.Zerof(t, report.Unmappable, "不應有帶不出分類路徑的結果：%+v", report.Failures)
	require.LessOrEqual(t, unresolved*3, len(queries),
		"解析不出任何分類的（非 NIL 期望）查詢超過全部三分之一：分類樹不夠當品項來源，回報再決定")
	require.GreaterOrEqual(t, report.RecallAtK, 0.9, "recall@10 門檻；失敗清單：%+v", report.Failures)
	require.GreaterOrEqual(t, report.PrecisionAt1, 0.75, "precision@1 門檻；失敗清單：%+v", report.Failures)
}
