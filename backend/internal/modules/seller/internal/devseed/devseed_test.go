package devseed

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// seller schema 的每一張表都要有種子的處理方式：只因使用而存在的店家子列由 clearChildrenSQL 清除，其餘（店家本身、公告列、media ref、不屬於店家的名單）
// 保留。新增的表沒有分類就失敗，讓維護者決定；漏掉會讓重跑 dev-seed 留下舊的 KYC、停權等，種子店家不再回到 fixture。
func TestEverySellerTableHasASeedDecision(t *testing.T) {
	kept := []string{"stores", "store_closures", "store_exposures", "media_ref_sync", "retired_assets", "prospects", "prospect_contacts", "prospect_distinct_pairs", "prospect_categories"}
	schema, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for _, m := range regexp.MustCompile(`(?m)^CREATE TABLE seller\.(\w+)`).FindAllStringSubmatch(string(schema), -1) {
		tables = append(tables, m[1])
	}
	if len(tables) < 20 {
		t.Fatalf("只解析出 %d 張表，schema.sql 的格式變了？", len(tables))
	}
	cleared := map[string]bool{}
	for _, stmt := range clearChildrenSQL {
		m := regexp.MustCompile(`^DELETE FROM seller\.(\w+) WHERE `).FindStringSubmatch(stmt)
		if m == nil {
			t.Fatalf("clearChildrenSQL 的語句不是對 seller 表的 DELETE：%s", stmt)
		}
		cleared[m[1]] = true
	}
	for _, name := range tables {
		switch {
		case cleared[name] && slices.Contains(kept, name):
			t.Errorf("seller.%s 同時在清除與保留的清單", name)
		case !cleared[name] && !slices.Contains(kept, name):
			t.Errorf("seller.%s 沒有種子的處理方式：只因使用而存在的店家子列加進 clearChildrenSQL，其餘加進這個測試的 kept", name)
		}
	}
	for name := range cleared {
		if !slices.Contains(tables, name) {
			t.Errorf("clearChildrenSQL 清的 seller.%s 不在 schema.sql", name)
		}
	}
	if strings.Contains(strings.Join(clearChildrenSQL, "\n"), "store_exposures") {
		t.Error("公告列不能刪：version 只增不減")
	}
}

// 「生效中的措施」不在 devseed 寫第二份：種子檢查問 stint.EffectiveMany（EffectiveMeasures 那一支查詢），自己的 SQL 不讀 measures。
func TestDevSeedHasNoMeasureSQLOfItsOwn(t *testing.T) {
	src, err := os.ReadFile("devseed.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"m.kind", "kind = 'SUSPENSION'", "m.status = 'ENFORCED'", "created_at <= $", "starts_at <= $"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("devseed.go 不得有自己的措施 SQL：%q", banned)
		}
	}
	if !strings.Contains(string(src), "stint.EffectiveMany") {
		t.Error("種子檢查要用 stint.EffectiveMany 判斷生效中的措施")
	}
}
