package seller

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// PublicStore.UpdateTime（sitemap 的 lastmod）靠 stores.updated_at：每條 UPDATE seller.stores 都要自己寫 updated_at。
// StampFirstExposed 是唯一的例外：它寫的 first_exposed_at 本身就是 UpdateTime 的一個來源（model.PublicStoreUpdateTime）。
func TestEveryStoresUpdateSetsUpdatedAtExceptTheFirstExposedStamp(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("db", "queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到 queries：%v", err)
	}
	update := regexp.MustCompile(`(?is)\bupdate\s+(only\s+)?seller\.stores\b[^;]*;`)
	updatedAtAssign := regexp.MustCompile(`(?i)(^|[\s,])updated_at\s*=`)
	setClause := regexp.MustCompile(`(?is)\bset\b(.*?)(?:\bwhere\b|\bfrom\b|\breturning\b|;)`)
	setsUpdatedAt := func(stmt []byte) bool {
		m := setClause.FindSubmatch(stmt)
		return len(m) > 1 && updatedAtAssign.Match(m[1])
	}
	name := regexp.MustCompile(`(?m)^-- name: (\w+)`)
	seen := 0
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, loc := range update.FindAllIndex(body, -1) {
			seen++
			names := name.FindAllStringSubmatch(string(body[:loc[0]]), -1)
			if len(names) == 0 {
				t.Fatalf("%s：UPDATE 不在任何 query 裡", f)
			}
			query := names[len(names)-1][1]
			if query == "StampFirstExposed" {
				continue
			}
			if !setsUpdatedAt(body[loc[0]:loc[1]]) {
				t.Errorf("%s 的 %s 更新了 seller.stores 卻沒有寫 updated_at", f, query)
			}
		}
	}
	if seen == 0 {
		t.Error("找不到任何 UPDATE seller.stores：檢查可能指向了錯的目錄")
	}
}
