package content_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/content"
)

// AppType 的擁有者仍是 CodeMeta（payment、aftersales 還在用），content 的 AppType 是同一個概念，
// 值必須相同。
func TestCodeMetaProjectionMatchesTheModule(t *testing.T) {
	md, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "代碼管理（CodeMeta）", "業務類型碼.md"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		owner string
		want  []string
	}{
		{"AppType", "", stringsOf(content.AppTypes())},
	}
	for _, c := range cases {
		owner, values := codeMetaSection(string(md), c.name)
		if !strings.HasPrefix(owner, c.owner) || (c.owner == "" && owner != "") {
			t.Errorf("CodeMeta 的 %s：擁有者 = %q，預期 %q", c.name, owner, c.owner)
		}
		slices.Sort(values)
		want := slices.Sorted(slices.Values(c.want))
		if !slices.Equal(values, want) {
			t.Errorf("CodeMeta 的 %s = %v，content 是 %v", c.name, values, want)
		}
	}
}

// codeMetaSection 回傳 CodeMeta Markdown 裡 `## name` 那一段的擁有者屬性與表格的 Value 欄。
func codeMetaSection(md, name string) (owner string, values []string) {
	lines := strings.Split(md, "\n")
	start := slices.Index(lines, "## "+name)
	if start < 0 {
		return "", nil
	}
	valueCol := -1
	for _, line := range lines[start+1:] {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if v, ok := strings.CutPrefix(line, "- **擁有者**: "); ok {
			owner = v
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		switch {
		case valueCol < 0:
			valueCol = slices.Index(cells, "Value")
		case strings.HasPrefix(cells[0], "---"):
		case valueCol < len(cells):
			values = append(values, cells[valueCol])
		}
	}
	return owner, values
}

func stringsOf[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}
