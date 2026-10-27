package reads_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

var update = flag.Bool("update", false, "重寫 testdata/golden 的 SQL 快照")

// rendered 是語句渲染出來的 SQL 加上參數（golden 檔的內容）。
func rendered(e reads.Entry, v visibility.Viewer) string {
	q, args := e.Build(v).Sql()
	var b strings.Builder
	b.WriteString(strings.TrimSpace(q))
	b.WriteString("\n\n-- args:\n")
	for i, a := range args {
		fmt.Fprintf(&b, "-- $%d = %v\n", i+1, a)
	}
	return b.String()
}

func goldenPath(e reads.Entry, suffix string) string {
	return filepath.Join("testdata", "golden", e.Name+suffix+".sql")
}

// 每支登記的 query 渲染出來的 SQL 有 golden 檔：改 query 或述詞，review 看得到 SQL 的差異；`go test ./... -update` 重寫。
// 有觀看者的語句文字不同於匿名讀取時另存一份（目前觀看者不影響任何述詞，兩份相同就只存匿名的）。
func TestRenderedQueriesMatchGolden(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range reads.Registry() {
		for _, c := range []struct {
			suffix string
			v      visibility.Viewer
		}{{"", visibility.Anonymous}, {".viewer", visibility.Viewer{BuyerID: 42}}} {
			got := rendered(e, c.v)
			if c.suffix != "" && got == rendered(e, visibility.Anonymous) {
				if _, err := os.Stat(goldenPath(e, c.suffix)); err == nil {
					t.Errorf("%s：觀看者不影響語句了，刪掉 %s", e.Name, goldenPath(e, c.suffix))
				}
				continue
			}
			path := goldenPath(e, c.suffix)
			seen[filepath.Base(path)] = true
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s：沒有 golden 檔 %s（新增 query 後跑 go test -update）", e.Name, path)
				continue
			}
			if string(want) != got {
				t.Errorf("%s 渲染出來的 SQL 與 golden 不同（有意的改動跑 go test -update 並 review 差異）：\n--- golden\n%s\n--- 現在\n%s", e.Name, want, got)
			}
		}
	}
	files, _ := filepath.Glob(filepath.Join("testdata", "golden", "*.sql"))
	for _, f := range files {
		if !seen[filepath.Base(f)] && !*update {
			t.Errorf("golden 檔 %s 沒有對應的登記 query", f)
		}
	}
}
