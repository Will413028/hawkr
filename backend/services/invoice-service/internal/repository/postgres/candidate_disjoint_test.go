// 本檔沒有 build tag，所以在一般 `go test ./...` 就會跑：它守的兩條不變式一個 DB
// 都不需要，掛在 integration tag 下等於只在有 Docker 的環境才存在。
package postgres

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 設計書 §5.4 的不變式表逐條指名了守護者。以下兩條的「落點」欄寫的是
// `operation_predicate_disjoint_test.go` 與 `db/sqlc_override_test.go`
// —— 2026-09-05 第 6 輪稽核實查：**那兩個檔案不存在**，兩條不變式今天都成立，
// 但沒有任何東西會因為它們被破壞而變紅。本檔補上它們。

// queriesFile 讀 db/queries 下的一支檔案。找不到一律 t.Fatalf 不 t.Skip：
// 檔案被搬走時 SKIP 會讓這道檢查靜默消失，而它守的正是「沒有人在看」。
func queriesFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "db", "queries", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀不到 %s：%v", path, err)
	}
	return string(raw)
}

// statusSetOf 取某支 query 的 `status IN ('A','B',…)` 值集合。
func statusSetOf(t *testing.T, body, queryName string) map[string]struct{} {
	t.Helper()
	marker := "-- name: " + queryName + " "
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("db/queries 裡找不到 %q —— 它被改名或刪掉了", queryName)
	}
	rest := body[i:]
	if j := strings.Index(rest[len(marker):], "-- name: "); j >= 0 {
		rest = rest[:len(marker)+j]
	}
	m := regexp.MustCompile(`(?s)status\s+IN\s*\(([^)]*)\)`).FindStringSubmatch(rest)
	if m == nil {
		t.Fatalf("%s 裡找不到 `status IN (...)` —— 述詞被改寫了，這支檢查要跟著改", queryName)
	}
	out := map[string]struct{}{}
	for _, v := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(m[1], -1) {
		out[v[1]] = struct{}{}
	}
	if len(out) == 0 {
		t.Fatalf("%s 的 status 值集合解析成空的", queryName)
	}
	return out
}

// TestOperationCandidateSetsAreDisjoint 是 I-55。
//
// 🔴 為什麼它是正確性條件而不是風格：兩組候選共用租約欄 next_attempt_at，
// 但租約長度不同（INV.op 120s 打寫入類 API、INV.rc 60s 唯讀對帳）。交集非空
// ⇒ 短租約那支會在長租約那支**還在打 f0401 的途中**合法搶走它 ⇒ 重複開票
// （db/queries/invoice_operations.sql 的檔頭與 §4.6 W-03 逐字寫著同一句）。
//
// 症狀是重複的稅務憑證，不是任何錯誤訊息。
func TestOperationCandidateSetsAreDisjoint(t *testing.T) {
	body := queriesFile(t, "invoice_operations.sql")
	op := statusSetOf(t, body, "FetchDueInvoiceOperations")
	rc := statusSetOf(t, body, "FetchPendingInvoiceRechecks")

	var both []string
	for s := range op {
		if _, ok := rc[s]; ok {
			both = append(both, s)
		}
	}
	sort.Strings(both)
	if len(both) > 0 {
		t.Errorf("兩組候選集合的交集非空：%v\n"+
			"INV.op（租約 120s，打 AMEGO 寫入類）與 INV.rc（租約 60s，唯讀對帳）共用 next_attempt_at\n"+
			"⇒ 交集裡的列會在 INV.op 還在打 f0401 的途中被 INV.rc 合法搶走 ⇒ 重複開票（§4.6 W-03）",
			both)
	}
	// 陰性對照：兩邊都非空才代表這支測試真的在比對東西。
	if len(op) == 0 || len(rc) == 0 {
		t.Fatalf("解析結果不合理：op=%d rc=%d", len(op), len(rc))
	}
}

// TestEveryJsonbColumnHasSqlcOverride 是 I-44。
//
// 漏一條 override 的症狀**不是編譯錯也不是測試紅**：pgx v5 的
// QueryExecModeExec 跳過 server-side Describe，裸 []byte 會被推成 bytea 而非 jsonb
// ⇒ 寫入時 runtime 22P02（db/sqlc.yaml 的檔頭逐字寫著這一句）。
func TestEveryJsonbColumnHasSqlcOverride(t *testing.T) {
	schema := repoFile(t, "..", "..", "..", "db", "schema", "schema.sql")
	cfg := repoFile(t, "..", "..", "..", "db", "sqlc.yaml")

	// 目前所在的表：schema.sql 的 CREATE TABLE "x" ( … );
	tableRe := regexp.MustCompile(`(?m)^CREATE TABLE(?: IF NOT EXISTS)?\s+"?([a-z_]+)"?`)
	colRe := regexp.MustCompile(`(?m)^\s+"([a-z_]+)"\s+jsonb\b`)

	type col struct{ table, name string }
	var cols []col
	var table string
	for _, line := range strings.Split(schema, "\n") {
		if m := tableRe.FindStringSubmatch(line); m != nil {
			table = m[1]
			continue
		}
		if m := colRe.FindStringSubmatch(line); m != nil && table != "" {
			cols = append(cols, col{table, m[1]})
		}
	}
	if len(cols) == 0 {
		t.Fatal("schema.sql 裡一個 jsonb 欄位都沒解析到 —— 解析壞了，不是「真的沒有」")
	}

	var missing []string
	for _, c := range cols {
		want := `column: "` + c.table + "." + c.name + `"`
		if !strings.Contains(cfg, want) {
			missing = append(missing, c.table+"."+c.name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下 jsonb 欄位在 db/sqlc.yaml 沒有 override：%v\n"+
			"漏了不會編譯錯、不會測試紅，只會在寫入時 runtime 22P02"+
			"（pgx v5 QueryExecModeExec 跳過 Describe，裸 []byte 被推成 bytea）", missing)
	}
}
