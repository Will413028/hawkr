package seller

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// agreement_acceptances 只追加：queries 裡沒有 DELETE、TRUNCATE；UPDATE 只允許一條，名稱是 PurgeAgreementSources，而且它只把
// client_ip、user_agent 清成 NULL 並記下 pii_purged_at。寫入的 INSERT 沒有 ON CONFLICT（每次同意事件一列）。
func TestAgreementAcceptancesAreAppendOnly(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("db", "queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到 queries：%v", err)
	}
	remove := regexp.MustCompile(`(?is)\b(delete\s+from|truncate(\s+table)?)\s+(only\s+)?seller\.agreement_acceptances\b`)
	update := regexp.MustCompile(`(?is)update\s+(only\s+)?seller\.agreement_acceptances\b[^;]*;`)
	insert := regexp.MustCompile(`(?is)insert\s+into\s+seller\.agreement_acceptances\b[^;]*;`)
	setClause := regexp.MustCompile(`(?is)\bset\s+(.*?)\s+from\b`)
	insertSeen, purgeSeen := false, false
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := remove.Find(body); m != nil {
			t.Errorf("%s 刪除了 agreement_acceptances：%q", f, m)
		}
		for _, loc := range update.FindAllIndex(body, -1) {
			stmt := string(body[loc[0]:loc[1]])
			// 往前找這條語句所屬的 query 名稱。
			names := regexp.MustCompile(`(?m)^-- name: (\w+)`).FindAllStringSubmatch(string(body[:loc[0]]), -1)
			if len(names) == 0 || names[len(names)-1][1] != "PurgeAgreementSources" {
				t.Errorf("%s 有不在允許清單的 UPDATE：%q", f, stmt)
				continue
			}
			purgeSeen = true
			set := setClause.FindStringSubmatch(stmt)
			if len(set) < 2 || !regexp.MustCompile(`(?is)^\s*client_ip\s*=\s*NULL\s*,\s*user_agent\s*=\s*NULL\s*,\s*pii_purged_at\s*=\s*sqlc\.arg\(now\)\s*$`).MatchString(set[1]) {
				t.Errorf("PurgeAgreementSources 只能把 client_ip、user_agent 清成 NULL 並記下 pii_purged_at：%q", stmt)
			}
		}
		for _, stmt := range insert.FindAll(body, -1) {
			insertSeen = true
			if regexp.MustCompile(`(?is)on\s+conflict`).Match(stmt) {
				t.Errorf("%s 的 INSERT 有 ON CONFLICT（每次同意事件一列）：%q", f, stmt)
			}
		}
	}
	if !insertSeen || !purgeSeen {
		t.Errorf("找不到 INSERT（%v）或清理的 UPDATE（%v）：檢查可能指向了錯的目錄", insertSeen, purgeSeen)
	}
}
