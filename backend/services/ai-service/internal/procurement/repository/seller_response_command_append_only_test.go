package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// 指令紀錄與報價版本一樣只靠「沒有任何改寫它的查詢」保證不可變：
// db/queries 裡不得出現 UPDATE 或 DELETE 這張表的語句。
func TestSellerResponseCommands_NoQueryRewritesThem(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "db", "queries", "procurement", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no query files found: %v", err)
	}
	rewrite := regexp.MustCompile(`(?is)\b(UPDATE\s+procurement_seller_response_commands\b|DELETE\s+FROM\s+procurement_seller_response_commands\b|TRUNCATE\s+(TABLE\s+)?procurement_seller_response_commands\b)`)
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := rewrite.Find(body); m != nil {
			t.Fatalf("%s rewrites the append-only command table: %q", filepath.Base(f), m)
		}
	}
}
