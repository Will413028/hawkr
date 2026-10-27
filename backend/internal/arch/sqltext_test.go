package arch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// 每個模組的 schema.sql、objects.sql、pub.sql 與 queries/*.sql 不用只去 U+0020 的 trim 函式，也不另列空白碼位（platform/trimtext 是唯一的定義）。
// migrations/ 不看：已套用的不得修改，現況由 schema.sql 代表（TestSchemaFileMatchesMigrations 保證兩者相同）。
func TestModuleSQLDoesNotTrim(t *testing.T) {
	root := moduleRoot(t)
	var files []string
	for _, pattern := range []string{"db/schema.sql", "db/objects.sql", "db/pub.sql", "db/queries/*.sql"} {
		m, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 20 {
		t.Fatalf("只找到 %d 份 SQL，檢查範圍不對", len(files))
	}
	canonical := 0
	for _, path := range files {
		rel, _ := filepath.Rel(filepath.Join(root, "internal", "modules"), path)
		module, file, _ := strings.Cut(rel, string(filepath.Separator))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		canonical += strings.Count(string(raw), trimtext.Check(""))
		for _, v := range CheckTrimmedText(module, file, string(raw)) {
			t.Error(v)
		}
	}
	// 掃不到任何 trimtext.Check 的展開代表比對的寫法與 schema.sql 不同，上面的檢查形同虛設。
	if canonical == 0 {
		t.Fatal("沒有任何 schema.sql 用 trimtext.Check 的展開，檢查範圍不對")
	}
}

func TestCheckTrimmedTextFlagsWhitespaceOnlyTrimsAndStraySpaceCodePoints(t *testing.T) {
	for name, sql := range map[string]string{
		"CHECK 的 btrim":              "CREATE TABLE s.t (v TEXT CHECK (v = btrim(v)));",
		"索引的 btrim":                  "CREATE INDEX t_v ON s.t (lower(btrim(v)));",
		"大寫 BTRIM":                   "SELECT * FROM s.t WHERE BTRIM(v) = $1;",
		"ltrim":                      "SELECT ltrim(v) FROM s.t;",
		"rtrim 有空白":                  "SELECT rtrim (v) FROM s.t;",
		"TRIM(x)":                    "SELECT TRIM(v) FROM s.t;",
		"TRIM(BOTH FROM)":            "SELECT TRIM(BOTH FROM v) FROM s.t;",
		"巢狀參數":                       "SELECT btrim(lower(coalesce(v, ''))) FROM s.t;",
		"另列空白碼位":                     `CREATE TABLE s.t (v TEXT CHECK (v !~ '^[\u3000]'));`,
		"註解後的呼叫":                     "-- 註解\nSELECT btrim(v) FROM s.t;",
		"btrim 指定空白":                 "CREATE TABLE s.t (v TEXT CHECK (v = btrim(v, ' ')));",
		"btrim 指定空白與其他字元":            "SELECT btrim(v, '/ ') FROM s.t;",
		"rtrim 指定 E 字串的 tab":         `SELECT rtrim(v, E'\t') FROM s.t;`,
		"ltrim 指定 E 字串的 \\u3000":     `SELECT ltrim(v, E'\u3000') FROM s.t;`,
		"deparse 後的 btrim 指定空白":      "CHECK ((v = btrim(v, ' '::text)))",
		"btrim 的字元不是常數":              "SELECT btrim(v, $1) FROM s.t;",
		"TRIM LEADING 指定空白":          "SELECT TRIM(LEADING ' ' FROM v) FROM s.t;",
		"TRIM FROM 之後的字元":            "SELECT TRIM(BOTH FROM v, ' ') FROM s.t;",
		"TRIM 字元裡的 FROM 不算":          "SELECT TRIM(' FROM' FROM v) FROM s.t;",
		"regexp_replace 去頭尾空格":       "CREATE INDEX t_v ON s.t (lower(regexp_replace(v, '^ +| +$', '', 'g')));",
		"regexp_replace 去 \\s":       `SELECT regexp_replace(v, '^\s+|\s+$', '') FROM s.t;`,
		"regexp_replace 去 [:space:]": "SELECT regexp_replace(v, '^[[:space:]]+', '') FROM s.t;",
		"CHECK 用 [[:space:]]":        "CREATE TABLE s.t (v TEXT CHECK (v !~ '^[[:space:]]|[[:space:]]$'));",
		"CHECK 用 \\S":                `CREATE TABLE s.t (v TEXT CHECK (v ~ '^\S(.*\S)?$'));`,
		"CHECK 用 [:blank:]":          "CREATE TABLE s.t (v TEXT CHECK (v !~ '^[[:blank:]]'));",
		"CHECK 的正則有空格":               "CREATE TABLE s.t (v TEXT CHECK (v !~ '^ | $'));",
		"CHECK 的 NOT LIKE 空格":        "CREATE TABLE s.t (v TEXT CHECK (v NOT LIKE ' %' AND v NOT LIKE '% '));",
		"CHECK 的 E 字串 tab":           `CREATE TABLE s.t (v TEXT CHECK (v !~ E'^\t'));`,
		"deparse 後的正則有空格":            "CHECK ((v !~ '^ | $'::text))",
	} {
		if got := CheckTrimmedText("m", "f.sql", sql); len(got) == 0 {
			t.Errorf("%s：預期被擋下：%s", name, sql)
		}
	}
	for name, sql := range map[string]string{
		"指定字元的 btrim":    "SELECT btrim(v, '/') FROM s.t;",
		"指定字元的 TRIM":     "SELECT TRIM(BOTH '/' FROM v) FROM s.t;",
		"trimtext.Check": "CREATE TABLE s.t (v TEXT CHECK (" + trimtext.Check("v") + "));",
		"行註解":            "-- btrim(v) 只去 U+0020\nSELECT 1;",
		"區塊註解":           "/* btrim(v) /* 巢狀 */ 也是註解 */ SELECT 1;",
		"字串裡":            "SELECT 'btrim(v)';",
		"名字含 trim":       "SELECT trimmed_at FROM s.t;",
		"email 格式（整個值沒有空白）":   "CREATE TABLE s.t (v TEXT CHECK (v ~ '^[^@[:space:]]+@[^@[:space:]]+$'));",
		"正則沒有空白":              "CREATE TABLE s.t (v TEXT CHECK (v ~ '^[0-9]{8}$'));",
		"LIKE 沒有空白":           "SELECT 1 FROM s.t WHERE v LIKE 'abc%';",
		"比對以外的字串有空格":          "SELECT concat_ws(' ', a, b) FROM s.t;",
		"regexp_replace 沒有空白": "SELECT regexp_replace(v, '-', '', 'g') FROM s.t;",
		"deparse 後的 trimtext": "CHECK ((" + trimtext.Check("v") + "::text))",
	} {
		if got := CheckTrimmedText("m", "f.sql", sql); len(got) != 0 {
			t.Errorf("%s：不該被擋下：%v", name, got)
		}
	}
}
