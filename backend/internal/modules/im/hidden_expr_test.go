package im

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const hiddenExpr = "(h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden"

func queryCode(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// 訊息「是否隱藏」（未還原的隱藏或生效中的下架）只有一種寫法：函式不能參照 seller 的表（單體規則 6），所以靠這個檢查讓每個輸出 hidden 的查詢都用同一個運算式。
func TestEveryHiddenColumnUsesTheCanonicalExpression(t *testing.T) {
	files, err := filepath.Glob("db/queries/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到查詢檔：%v", err)
	}
	var hiddenCols, canonical, takedownRefs int
	for _, f := range files {
		code := queryCode(t, f)
		hiddenCols += strings.Count(code, "AS hidden")
		canonical += strings.Count(code, hiddenExpr)
		takedownRefs += strings.Count(code, "pub_content_takedowns()")
	}
	if hiddenCols == 0 || hiddenCols != canonical || takedownRefs != canonical {
		t.Fatalf("hidden 欄位 %d、標準運算式 %d、pub_content_takedowns 出現 %d：每個 hidden 都要用標準運算式，且不得另有寫法", hiddenCols, canonical, takedownRefs)
	}
}
