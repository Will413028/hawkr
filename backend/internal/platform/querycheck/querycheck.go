// Package querycheck 檢查「不是 sqlc 寫的 query」是否守住單體規則 2（模組只碰自己的 schema 與依賴表允許的 pub_*）。
package querycheck

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/pgtree"
)


var builtinFuncs = []string{"count", "coalesce", "lower", "upper", "greatest", "least", "max", "min", "sum", "length", "char_length"}

var timeFuncs = []string{"now", "clock_timestamp", "statement_timestamp", "transaction_timestamp", "timeofday", "localtimestamp", "current_timestamp", "current_date", "current_time"}

// Check 回傳 sql 違反邊界的地方；own 是模組自己的 schema，allowedOwners 是它可以呼叫 pub_* 的擁有者（依賴表允許且發布了 pub 的模組；
// cmd/jetgen 只為依賴表（internal/arch 的 Dependencies）允許的擁有者生成 pub 包裝，模組的 pubsql 目錄就是這份清單）。
func Check(sql, own string, allowedOwners []string) ([]string, error) {
	tree, err := pgtree.Parse(sql)
	if err != nil {
		return nil, err
	}
	ctes := map[string]bool{}
	collectCTEs(tree, ctes)
	var bad []string
	pgtree.Walk(tree, func(v map[string]any) {
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
	})
	slices.Sort(bad)
	return slices.Compact(bad), nil
}

func collectCTEs(n any, out map[string]bool) {
	pgtree.Walk(n, func(v map[string]any) {
		if cte, ok := v["CommonTableExpr"].(map[string]any); ok {
			if name, _ := cte["ctename"].(string); name != "" {
				out[name] = true
			}
		}
	})
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
