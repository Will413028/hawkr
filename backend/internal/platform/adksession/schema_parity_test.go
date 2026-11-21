package adksession

import (
	"maps"
	"os"
	"regexp"
	"strings"
	"testing"
)

var adkTables = []string{"sessions", "events", "app_states", "user_states"}

var (
	sqlLineComment = regexp.MustCompile(`--[^\n]*`)
	sqlSpace       = regexp.MustCompile(`\s+`)
	sqlPunct       = regexp.MustCompile(` ?([(),]) ?`)
)

// adkDDL 取出 schema 檔中 schema.<表> 的 CREATE TABLE 與指向它的 CREATE INDEX，去掉註解、把 schema 名換成 S、壓縮空白。
// 回傳「表名 → 正規化後的 DDL」。
func adkDDL(t *testing.T, sql, schema string) map[string]string {
	t.Helper()
	sql = sqlLineComment.ReplaceAllString(sql, "")
	out := map[string]string{}
	for _, stmt := range strings.Split(sql, ";") {
		stmt = strings.TrimSpace(sqlPunct.ReplaceAllString(sqlSpace.ReplaceAllString(stmt, " "), "$1"))
		for _, tbl := range adkTables {
			ref := regexp.QuoteMeta(schema + "." + tbl)
			if regexp.MustCompile(`^CREATE TABLE `+ref+`[ (]`).MatchString(stmt) ||
				regexp.MustCompile(`^CREATE (UNIQUE )?INDEX .* ON `+ref+`[ (]`).MatchString(stmt) {
				out[tbl] += strings.ReplaceAll(stmt, schema+".", "S.") + ";"
			}
		}
	}
	return out
}

func readSchema(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// 兩個模組各自在自己的 schema 建 ADK 的四張表；形狀只能一致，這個測試是唯一的同步機制。
func TestAdkTablesHaveTheSameShapeInEveryModuleSchema(t *testing.T) {
	assistant := adkDDL(t, readSchema(t, "../../modules/assistant/db/schema.sql"), "assistant")
	procurement := adkDDL(t, readSchema(t, "../../modules/procurement/db/schema.sql"), "procurement")
	for _, tbl := range adkTables {
		if assistant[tbl] == "" || procurement[tbl] == "" {
			t.Fatalf("%s：兩份 schema 都要有這張表（assistant %t、procurement %t）", tbl, assistant[tbl] != "", procurement[tbl] != "")
		}
	}
	if !maps.Equal(assistant, procurement) {
		for _, tbl := range adkTables {
			if assistant[tbl] != procurement[tbl] {
				t.Errorf("%s 的形狀不一致：\n  assistant:   %s\n  procurement: %s", tbl, assistant[tbl], procurement[tbl])
			}
		}
	}
}

// 比對器本身要分得出差異，也要忽略註解與空白。
func TestAdkDDLComparisonSeesColumnDifferencesButNotComments(t *testing.T) {
	base := "CREATE TABLE a.app_states (\n  app_name TEXT PRIMARY KEY, -- 註解\n  state JSONB NOT NULL\n);"
	same := "-- 別的註解\nCREATE TABLE b.app_states (app_name TEXT PRIMARY KEY, state JSONB NOT NULL);"
	diff := "CREATE TABLE b.app_states (app_name TEXT PRIMARY KEY, state JSON NOT NULL);"
	if !maps.Equal(adkDDL(t, base, "a"), adkDDL(t, same, "b")) {
		t.Fatal("註解與空白不同不算形狀不同")
	}
	if maps.Equal(adkDDL(t, base, "a"), adkDDL(t, diff, "b")) {
		t.Fatal("欄位型別不同要被看出來")
	}
}
