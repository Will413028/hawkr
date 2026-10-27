// Package querycheck 檢查「不是 sqlc 寫的 query」是否守住單體規則 2（模組只碰自己的 schema 與依賴表允許的 pub_*）。
package querycheck

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// 模組單體規則 2 對「不是 sqlc 寫的 query」的機械檢查。sqlc 只讀自己的 schema.sql 與依賴表允許的 pub.sql，所以參照別的模組的內部表在生成時就
// 失敗；用 go-jet 這類 builder 建的 query 沒有這道關（builder 生成的表型別只有模組自己的表，但原始 SQL 與表達式的逃生口可以寫任何東西），
// 所以對 builder 渲染出來的 SQL 文字做同一件事，不管它是誰產生的：
//   - 每個表（RangeVar）都在模組自己的 schema；
//   - 每個函式：內建的（不帶 schema，限白名單）、模組自己的 schema 的、pg_catalog 的，或依賴表允許的模組的 `<擁有者>.pub_*`。
//
// Check 只看一句 SQL，不需要資料庫。

// builtinFuncs 是 query 可以呼叫的不帶 schema 的內建函式；要用別的先擴充這張表（不帶 schema 的名稱會依 search_path 解析，不能放行任意名稱）。
var builtinFuncs = []string{"count", "coalesce", "lower", "upper", "greatest", "least", "max", "min", "sum", "length", "char_length"}

// 這份白名單沒有 now：query 的時間一律是請求的 clock（參數），不讀資料庫的現在（可測、同一個請求內一致，也是 social 的不變量）。
// timeFuncs 是取得「現在」的函式，不論帶不帶 schema（pg_catalog.now()）都擋；CURRENT_TIMESTAMP 等 SQL 標準的寫法是 SQLValueFunction 節點，另外擋。
// 要在某個模組放行，先說明理由並在這裡開出口。
var timeFuncs = []string{"now", "clock_timestamp", "statement_timestamp", "transaction_timestamp", "timeofday", "localtimestamp", "current_timestamp", "current_date", "current_time"}

// Check 回傳 sql 違反邊界的地方；own 是模組自己的 schema，allowedOwners 是它可以呼叫 pub_* 的擁有者（依賴表允許且發布了 pub 的模組；
// cmd/jetgen 只為依賴表（internal/arch 的 Dependencies）允許的擁有者生成 pub 包裝，模組的 pubsql 目錄就是這份清單）。
func Check(sql, own string, allowedOwners []string) ([]string, error) {
	js, err := pg_query.ParseToJSON(sql)
	if err != nil {
		return nil, fmt.Errorf("無法解析 SQL：%w", err)
	}
	var tree any
	if err := json.Unmarshal([]byte(js), &tree); err != nil {
		return nil, err
	}
	ctes := map[string]bool{}
	collectCTEs(tree, ctes)
	var bad []string
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			// 表的參照：FROM 裡是 RangeVar 節點，INSERT／UPDATE／DELETE 的目標直接是帶 relname 的節點。
			if rel, ok := v["relname"].(string); ok {
				schema, _ := v["schemaname"].(string)
				switch {
				case schema == "" && ctes[rel]:
				case schema != own:
					bad = append(bad, fmt.Sprintf("表 %s.%s 不在模組的 schema %s", schema, rel, own))
				}
			}
			if _, ok := v["SQLValueFunction"].(map[string]any); ok {
				bad = append(bad, "CURRENT_TIMESTAMP、CURRENT_DATE 等取得現在時間的寫法不允許：時間要是請求的 clock 參數")
			}
			if fc, ok := v["FuncCall"].(map[string]any); ok {
				if msg := checkFunc(fc, own, allowedOwners); msg != "" {
					bad = append(bad, msg)
				}
			}
			for _, c := range v {
				walk(c)
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		}
	}
	walk(tree)
	slices.Sort(bad)
	return slices.Compact(bad), nil
}

func collectCTEs(n any, out map[string]bool) {
	switch v := n.(type) {
	case map[string]any:
		if cte, ok := v["CommonTableExpr"].(map[string]any); ok {
			if name, _ := cte["ctename"].(string); name != "" {
				out[name] = true
			}
		}
		for _, c := range v {
			collectCTEs(c, out)
		}
	case []any:
		for _, c := range v {
			collectCTEs(c, out)
		}
	}
}

func checkFunc(fc map[string]any, own string, allowedOwners []string) string {
	var parts []string
	names, _ := fc["funcname"].([]any)
	for _, x := range names {
		s, _ := x.(map[string]any)["String"].(map[string]any)
		sval, _ := s["sval"].(string)
		parts = append(parts, sval)
	}
	full := strings.Join(parts, ".")
	if len(parts) > 0 && slices.Contains(timeFuncs, parts[len(parts)-1]) {
		return fmt.Sprintf("函式 %s 取得現在時間：時間要是請求的 clock 參數，不用資料庫的 now()", full)
	}
	switch len(parts) {
	case 1:
		if !slices.Contains(builtinFuncs, parts[0]) {
			return fmt.Sprintf("函式 %s 不帶 schema 也不在內建白名單（querycheck.builtinFuncs）", full)
		}
	case 2:
		switch {
		case parts[0] == own || parts[0] == "pg_catalog":
		case slices.Contains(allowedOwners, parts[0]) && strings.HasPrefix(parts[1], "pub_"):
		default:
			return fmt.Sprintf("函式 %s 不是模組自己的，也不是依賴表允許的 <擁有者>.pub_*", full)
		}
	default:
		return fmt.Sprintf("函式 %s 的名稱層數不合法", full)
	}
	return ""
}
