package arch

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// SQL 的跨模組讀取（ADR 2026-09-24-go-modular-monolith 文末修訂「跨模組讀取改用擁有者發布的 pub_*」的強制機制）：模組的 sqlc 只讀自己的
// schema.sql（與 objects.sql）加依賴表允許的模組的 db/pub.sql，所以 query 參照別的模組的內部表在 sqlc generate 時就失敗；pub.sql 只宣告擁有者發布的
// `<owner>.pub_*` 的形狀。這裡檢查兩件事：sqlc.yaml 的 schema 清單（CheckSQLCConfig）與 pub.sql 的內容（CheckPubSQL）。pub.sql 與真實
// 物件一致由 cmd/hawkr 的契約測試（TestPubContractsMatchMigrations）檢查。

// pubSchemaPath 是 sqlc.yaml（在 internal/modules/<模組>/db）指向別的模組 pub.sql 的唯一寫法。
var pubSchemaPath = regexp.MustCompile(`^\.\./\.\./([a-z]+)/db/pub\.sql$`)

type sqlcConfig struct {
	SQL []struct {
		Schema yaml.Node `yaml:"schema"`
	} `yaml:"sql"`
}

// CheckSQLCConfig 回報模組 module 的 sqlc.yaml（config 是檔案內容）違反的規則：模組有自己的 db/schema.sql（schema 為真）時，每個 sql 項目的
// schema 清單必須正好列一次；沒有時（沒有自己資料的模組）不得列，而且至少讀一份依賴表允許的 pub.sql。模組有 db/objects.sql（objects 為真）時
// 緊接著 schema.sql 列一次 objects.sql（view 與函式建在表上，順序在後），沒有時不得列；其餘只能是依賴表（deps）允許的模組的
// `../../<模組>/db/pub.sql`，不得重複。
func CheckSQLCConfig(module string, config []byte, deps map[string][]string, schema, objects bool) []string {
	var cfg sqlcConfig
	if err := yaml.Unmarshal(config, &cfg); err != nil {
		return []string{fmt.Sprintf("%s 的 sqlc.yaml 無法解析：%v", module, err)}
	}
	if len(cfg.SQL) == 0 {
		return []string{fmt.Sprintf("%s 的 sqlc.yaml 沒有 sql 項目", module)}
	}
	var out []string
	for i, entry := range cfg.SQL {
		var schemas []string
		switch entry.Schema.Kind {
		case yaml.ScalarNode:
			schemas = []string{entry.Schema.Value}
		case yaml.SequenceNode:
			if err := entry.Schema.Decode(&schemas); err != nil {
				out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 第 %d 個 sql 項目的 schema 不是字串清單：%v", module, i+1, err))
				continue
			}
		case yaml.DocumentNode, yaml.MappingNode, yaml.AliasNode:
			out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 第 %d 個 sql 項目的 schema 必須是字串或字串清單", module, i+1))
			continue
		default:
			out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 第 %d 個 sql 項目沒有 schema", module, i+1))
			continue
		}
		own, ownObjects, pubs := 0, 0, 0
		seen := map[string]bool{}
		for j, s := range schemas {
			if seen[s] {
				out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 重複列出 %s", module, s))
				continue
			}
			seen[s] = true
			if s == "schema.sql" {
				if !schema {
					out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 列了 schema.sql，但模組沒有 db/schema.sql（沒有自己資料的模組不建 schema）", module))
				}
				own++
				continue
			}
			if s == "objects.sql" {
				switch {
				case !objects:
					out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 列了 objects.sql，但模組沒有 db/objects.sql", module))
				case j == 0 || schemas[j-1] != "schema.sql":
					out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 的 objects.sql 要緊接在 schema.sql 之後（view 與函式建在表上）", module))
				}
				ownObjects++
				continue
			}
			m := pubSchemaPath.FindStringSubmatch(s)
			switch {
			case m == nil:
				out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 列了 %s：只能是自己的 schema.sql、objects.sql 與依賴表允許的模組的 ../../<模組>/db/pub.sql（不得讀別的模組的內部表）", module, s))
			case m[1] == module:
				out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 列了自己的 pub.sql：自己的 query 讀 schema.sql", module))
			case !slices.Contains(deps[module], m[1]):
				out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 讀 %s 的 pub.sql，但依賴表不允許 %s 依賴 %s", module, m[1], module, m[1]))
			default:
				pubs++
			}
		}
		if schema && own != 1 {
			out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 第 %d 個 sql 項目必須正好列一次自己的 schema.sql", module, i+1))
		}
		if !schema && pubs == 0 {
			out = append(out, fmt.Sprintf("%s 沒有 db/schema.sql，sqlc.yaml 第 %d 個 sql 項目至少要讀一份依賴表允許的 pub.sql", module, i+1))
		}
		if objects && ownObjects == 0 {
			out = append(out, fmt.Sprintf("%s 的 sqlc.yaml 第 %d 個 sql 項目沒有列 objects.sql：模組的 view 與函式在 db/objects.sql", module, i+1))
		}
	}
	return out
}

var (
	dollarQuoted = regexp.MustCompile(`(?s)\$([A-Za-z_]*)\$.*?\$([A-Za-z_]*)\$`)
	lineComment  = regexp.MustCompile(`--[^\n]*`)
	createSchema = regexp.MustCompile(`(?is)^CREATE\s+SCHEMA\s+([a-z_]+)$`)
	createObject = regexp.MustCompile(`(?is)^CREATE\s+(TABLE|VIEW|FUNCTION)\s+([a-z_]+)\.([a-z0-9_]+)\b(.*)$`)
	returnsSetOf = regexp.MustCompile(`(?is)\bRETURNS\s+SETOF\s+([a-z_]+)\.([a-z0-9_]+)\b`)
)

// CheckPubSQL 回報模組 module 的 db/pub.sql（content 是檔案內容）違反的規則：只能有 `CREATE SCHEMA <module>`（sqlc 要先有 schema）與
// `CREATE TABLE|VIEW|FUNCTION <module>.pub_*`；函式必須 RETURNS SETOF `<module>.pub_*`（sqlc 不型別化 RETURNS TABLE 與複合型別）；
// 至少要有一個 pub_* 物件。
func CheckPubSQL(module, content string) []string {
	body := dollarQuoted.ReplaceAllString(content, "$$$$")
	body = lineComment.ReplaceAllString(body, "")
	var out []string
	objects := 0
	for _, raw := range strings.Split(body, ";") {
		stmt := strings.Join(strings.Fields(raw), " ")
		if stmt == "" {
			continue
		}
		if m := createSchema.FindStringSubmatch(stmt); m != nil {
			if m[1] != module {
				out = append(out, fmt.Sprintf("%s 的 pub.sql 建立了別的 schema %s", module, m[1]))
			}
			continue
		}
		m := createObject.FindStringSubmatch(stmt)
		if m == nil {
			out = append(out, fmt.Sprintf("%s 的 pub.sql 只能宣告 %s.pub_* 的形狀，不能有：%s", module, module, stmt))
			continue
		}
		if m[2] != module || !strings.HasPrefix(m[3], "pub_") {
			out = append(out, fmt.Sprintf("%s 的 pub.sql 宣告了 %s.%s：名字必須是 %s.pub_*", module, m[2], m[3], module))
			continue
		}
		objects++
		if strings.EqualFold(m[1], "FUNCTION") {
			r := returnsSetOf.FindStringSubmatch(m[4])
			if len(r) != 3 || r[1] != module || !strings.HasPrefix(r[2], "pub_") {
				out = append(out, fmt.Sprintf("%s 的 pub.sql 的函式 %s 必須宣告成 RETURNS SETOF %s.pub_*", module, m[3], module))
			}
		}
	}
	if objects == 0 {
		out = append(out, fmt.Sprintf("%s 的 pub.sql 沒有任何 pub_* 物件", module))
	}
	return out
}

// PubShapes 回傳 pub.sql（content）宣告成 TABLE 的 pub_*（`<module>.<名字>`）。它們只是函式回傳列的形狀（sqlc 要表才型別化欄位），
// 真實物件是複合型別：消費端的 query 寫 `FROM seller.pub_exposable_store` 能通過 sqlc generate，到執行期才失敗。
func PubShapes(module, content string) []string {
	body := dollarQuoted.ReplaceAllString(content, "$$$$")
	body = lineComment.ReplaceAllString(body, "")
	var out []string
	for _, raw := range strings.Split(body, ";") {
		m := createObject.FindStringSubmatch(strings.Join(strings.Fields(raw), " "))
		if len(m) == 5 && strings.EqualFold(m[1], "TABLE") && m[2] == module {
			out = append(out, m[2]+"."+m[3])
		}
	}
	return out
}

// CheckPubShapeUse 回報 query 檔（content）直接參照 shapes（PubShapes 的結果）的地方：消費端只能呼叫擁有者的 pub_* 函式，不能
// SELECT、INSERT 或 join 形狀本身。名字後面緊接著字母、數字或底線的不算（`seller.pub_exposable_stores(...)` 是函式）。比對前去掉
// `--` 與 `/* */` 註解、引號識別字換成名字、點的前後空白去掉（normalizeSQLNames），`"seller"."pub_exposable_store"` 也擋得到。
func CheckPubShapeUse(module, file, content string, shapes []string) []string {
	body := normalizeSQLNames(content)
	var out []string
	for _, shape := range shapes {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(shape) + `\b`)
		if re.MatchString(body) {
			out = append(out, fmt.Sprintf("%s 的 %s 直接參照 %s：它只是 pub_* 函式回傳列的形狀（真實物件是複合型別），要呼叫擁有者的函式", module, file, shape))
		}
	}
	return out
}

var (
	enumType    = regexp.MustCompile(`(?is)^CREATE\s+TYPE\s+\S+\s+AS\s+ENUM\b`)
	desiredStmt = regexp.MustCompile(`(?is)^CREATE\s+(?:OR\s+REPLACE\s+)?(?:(?:TEMP|TEMPORARY|RECURSIVE|MATERIALIZED)\s+)*(VIEW|FUNCTION|PROCEDURE|TYPE|DOMAIN|AGGREGATE|TRIGGER|RULE)\b`)
)

// CheckDesiredStateSplit 回報模組 db/ 的 desired state 放錯檔的語句（file 是 schema.sql 或 objects.sql，content 是檔案內容）：
//   - schema.sql 是 Atlas（`atlas migrate diff`）唯一讀的檔，只放表、約束與索引；view、函式、程序、型別、domain、aggregate、trigger
//     與 rule 放 objects.sql（Atlas 社群版遇到 view 直接報錯，其餘不處理，一律手寫 migration，單體規則 6）；enum 型別例外，Atlas 會 diff 它，放 schema.sql；
//   - objects.sql 只放上面那些（與它們的 COMMENT ON），表與索引放回 schema.sql，Atlas 才產生得出它們的 migration。
func CheckDesiredStateSplit(module, file, content string) []string {
	body := dollarQuoted.ReplaceAllString(StripSQLComments(content), "$$$$")
	var out []string
	for _, stmt := range splitStatements(body) {
		stmt = strings.Join(strings.Fields(stmt), " ")
		if stmt == "" {
			continue
		}
		object := desiredStmt.FindStringSubmatch(stmt)
		if len(object) > 1 && strings.EqualFold(object[1], "TYPE") && enumType.MatchString(stmt) {
			object = nil // enum 是 Atlas 社群版會 diff 的 desired state，跟表一樣放 schema.sql
		}
		switch file {
		case "schema.sql":
			if object != nil {
				out = append(out, fmt.Sprintf("%s 的 schema.sql 有 %s：view、函式與型別放 db/objects.sql（Atlas 只讀 schema.sql，單體規則 6）", module, strings.ToUpper(object[1])))
			}
		case "objects.sql":
			if object == nil && !strings.HasPrefix(strings.ToUpper(stmt), "COMMENT ON ") {
				out = append(out, fmt.Sprintf("%s 的 objects.sql 只能有 view、函式與型別，表、約束與索引放 schema.sql：%.80s", module, stmt))
			}
		}
	}
	return out
}

// splitStatements 以分號切開 body（dollar quote 已換掉、註解已去掉）。字串裡的分號不切；`BEGIN ATOMIC … END` 的 body 裡的分號也不切：
// 從 BEGIN ATOMIC 起，CASE 與 END 配對，回到零層的 END 才結束 body。
func splitStatements(body string) []string {
	blank := blankStrings(body)
	keywords := atomicKeyword.FindAllStringSubmatchIndex(blank, -1)
	var out []string
	start, k, depth := 0, 0, 0
	for i := 0; i < len(blank); i++ {
		for k < len(keywords) && keywords[k][0] < i {
			k++
		}
		if k < len(keywords) && keywords[k][0] == i {
			switch word := strings.ToUpper(blank[keywords[k][2]:keywords[k][3]]); {
			case strings.HasPrefix(word, "BEGIN"):
				depth = 1
			case depth > 0 && word == "CASE":
				depth++
			case depth > 0 && word == "END":
				depth--
			}
		}
		if blank[i] == ';' && depth == 0 {
			out = append(out, body[start:i])
			start = i + 1
		}
	}
	return append(out, body[start:])
}

var atomicKeyword = regexp.MustCompile(`(?i)\b(BEGIN\s+ATOMIC|CASE|END)\b`)
