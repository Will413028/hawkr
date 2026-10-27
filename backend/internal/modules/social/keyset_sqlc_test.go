package social_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// keyset 的游標條件只能是單一列比較（Index Cond）；`NOT has_cursor OR (…)` 這種布林開關在 generic plan 下是 Filter，成本隨游標深度成正比
// （step-1a-perf/README.md）。第一頁由 Go 給方向的極值游標（model.DescStart／AscStart）。這裡檢查還留在 sqlc 的 query；go-jet 建的 query
// 由 internal/reads 的 TestKeysetQueriesCompareOneRowAndHaveNoBooleanSwitch 檢查渲染出來的 SQL。
func TestKeysetSQLCQueriesHaveNoBooleanCursorSwitch(t *testing.T) {
	files, err := filepath.Glob("db/queries/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到 query 檔：%v", err)
	}
	sw := regexp.MustCompile(`(?i)has_cursor|\bNOT\s+sqlc\.arg\(\w+\)::bool\s+OR\b`)
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "--") {
				continue
			}
			if sw.MatchString(line) {
				t.Errorf("%s:%d 有布林開關的游標條件，改成單一列比較加極值游標：%s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
}
