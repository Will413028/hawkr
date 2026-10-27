package identity

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// admin_role_events 只追加：queries 裡沒有任何 UPDATE、DELETE、TRUNCATE 指向它（schema.sql 的註解提到的檢查）。
func TestAdminRoleEventsAreAppendOnly(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("db", "queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到 queries：%v", err)
	}
	mutation := regexp.MustCompile(`(?is)\b(update|delete\s+from|truncate(\s+table)?)\s+(only\s+)?identity\.admin_role_events\b`)
	insertSeen := false
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := mutation.Find(body); m != nil {
			t.Errorf("%s 修改了 admin_role_events：%q", f, m)
		}
		if regexp.MustCompile(`(?i)insert\s+into\s+identity\.admin_role_events\b`).Match(body) {
			insertSeen = true
		}
	}
	if !insertSeen {
		t.Error("沒有任何 query 寫入 admin_role_events：檢查可能指向了錯的目錄")
	}
}
