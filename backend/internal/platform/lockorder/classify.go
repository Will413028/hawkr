package lockorder

import (
	"fmt"
	"sync"

	"github.com/Will413028/hawkr/backend/internal/platform/pgtree"
)

// Lock 是一句 SQL 對一個表的列鎖。
type Lock struct {
	Schema string
	Table  string
	// Kind 是 FOR UPDATE、FOR NO KEY UPDATE、FOR SHARE、FOR KEY SHARE、UPDATE、DELETE、MERGE 或
	// INSERT ON CONFLICT DO UPDATE。
	Kind string
	// Waits 是遇到別人持有的鎖時會不會等：NOWAIT 與 SKIP LOCKED 不等，不會是死鎖裡等待的那一邊。
	Waits bool
}

// 算進來的是會鎖住「已經存在的列」的語句：SELECT … FOR …、UPDATE、DELETE、MERGE、INSERT … ON CONFLICT DO UPDATE。
// 單純的 INSERT（含 ON CONFLICT DO NOTHING）只會在唯一鍵和另一個還沒 commit 的 INSERT 撞到時等待，這裡不算；
// 外鍵檢查加的 FOR KEY SHARE 只在同一個 schema 內（規則 3 不建跨 schema FK），不影響跨模組的順序。
// advisory lock 沒有表，看不出屬於哪個模組，也不算。

// COPY FROM 同理：只新增列（表上是 RowExclusiveLock，不鎖既有的列），沒有 ON CONFLICT，和單純的 INSERT 一樣不算。
// 若把它算成列寫入，COPY 進上層模組的表會被誤報成「持有高位鎖」，但它不會讓任何 transaction 等待既有列的鎖。

var classified sync.Map // SQL 文字 → []Lock；測試裡同一句 sqlc query 會跑很多次

// Classify 回傳 sql 會加的列鎖。不帶 schema 的表（CTE 的名稱）略過。
func Classify(sql string) ([]Lock, error) {
	if v, ok := classified.Load(sql); ok {
		return v.([]Lock), nil
	}
	tree, err := pgtree.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("lockorder：%w", err)
	}
	var locks []Lock
	pgtree.Walk(tree, func(v map[string]any) { nodeLocks(v, &locks) })
	classified.Store(sql, locks)
	return locks, nil
}

// dmlKinds 是目標表整個被鎖的語句；用 slice 讓同一個節點的處理順序固定。
var dmlKinds = []struct{ node, kind string }{{"DeleteStmt", "DELETE"}, {"MergeStmt", "MERGE"}, {"UpdateStmt", "UPDATE"}}

// nodeLocks 處理解析樹的一個節點；pgtree.Walk 以固定順序走過每個節點，所以同一句 SQL 的鎖順序每次相同。
func nodeLocks(v map[string]any, out *[]Lock) {
	if s, ok := v["SelectStmt"].(map[string]any); ok {
		selectLocks(s, out)
	}
	for _, d := range dmlKinds {
		if s, ok := v[d.node].(map[string]any); ok {
			addRelation(s["relation"], d.kind, true, out)
		}
	}
	if s, ok := v["InsertStmt"].(map[string]any); ok {
		if oc, ok := s["onConflictClause"].(map[string]any); ok && oc["action"] == "ONCONFLICT_UPDATE" {
			addRelation(s["relation"], "INSERT ON CONFLICT DO UPDATE", true, out)
		}
	}
}

var strengths = map[string]string{
	"LCS_FORUPDATE":      "FOR UPDATE",
	"LCS_FORNOKEYUPDATE": "FOR NO KEY UPDATE",
	"LCS_FORSHARE":       "FOR SHARE",
	"LCS_FORKEYSHARE":    "FOR KEY SHARE",
}

// selectLocks 處理一層 SELECT 的鎖定子句：有 OF 就只鎖列出的 FROM 項目，沒有就鎖 FROM 裡的每個項目。
// 依 PostgreSQL 文件（SELECT, The Locking Clause）：鎖定子句套在 view 或子查詢上時，會影響其中用到的所有表。
// 所以 FROM 項目是基底表（RangeVar）、JOIN 兩邊（遞迴）、或 FROM 清單裡的子查詢（RangeSubselect，含 LATERAL；
// 子查詢內的表依它自己的 FROM 規則遞迴收集）。只出現在 WHERE 或 JOIN ON 的條件、運算式裡的子查詢（sublink）、
// 目標列與函式參數裡的表不算。OF 後面寫的是 FROM 裡的別名或不帶 schema 的表名；指名子查詢別名就鎖它的全部表。
func selectLocks(s map[string]any, out *[]Lock) {
	clauses, _ := s["lockingClause"].([]any)
	from := fromItems(s["fromClause"])
	for _, c := range clauses {
		lc, _ := c.(map[string]any)["LockingClause"].(map[string]any)
		if lc == nil {
			continue
		}
		strength, _ := lc["strength"].(string)
		kind, ok := strengths[strength]
		if !ok {
			kind = strength
		}
		policy, _ := lc["waitPolicy"].(string)
		waits := policy != "LockWaitSkip" && policy != "LockWaitError"
		rels, _ := lc["lockedRels"].([]any)
		if len(rels) == 0 {
			for _, it := range from {
				for _, rv := range it.tables {
					addRelation(rv, kind, waits, out)
				}
			}
			continue
		}
		for _, r := range rels {
			name, _ := r.(map[string]any)["RangeVar"].(map[string]any)["relname"].(string)
			for _, it := range from {
				if it.name == name {
					for _, rv := range it.tables {
						addRelation(rv, kind, waits, out)
					}
				}
			}
		}
	}
}

// fromItem 是 FROM 清單裡一個可被 OF 指名的項目：name 是它在同一句 SQL 裡被引用的名字，tables 是它鎖住的表。
type fromItem struct {
	name   string
	tables []any
}

// aliasOf 回傳節點的別名；沒有別名回傳空字串。
func aliasOf(r map[string]any) string {
	if alias, ok := r["alias"].(map[string]any); ok {
		name, _ := alias["aliasname"].(string)
		return name
	}
	return ""
}

// fromItems 收集 FROM 的項目：RangeVar（名字是別名，沒有就是表名）、JoinExpr 左右兩邊（遞迴）、
// RangeSubselect（名字是別名，表是子查詢自己的 FROM 項目的全部表）。不進 JOIN 的 quals、RangeFunction 等其他節點。
func fromItems(n any) []fromItem {
	switch v := n.(type) {
	case []any:
		var out []fromItem
		for _, c := range v {
			out = append(out, fromItems(c)...)
		}
		return out
	case map[string]any:
		if rv, ok := v["RangeVar"].(map[string]any); ok {
			name := aliasOf(rv)
			if name == "" {
				name, _ = rv["relname"].(string)
			}
			return []fromItem{{name: name, tables: []any{rv}}}
		}
		if j, ok := v["JoinExpr"].(map[string]any); ok {
			return append(fromItems(j["larg"]), fromItems(j["rarg"])...)
		}
		if rs, ok := v["RangeSubselect"].(map[string]any); ok {
			sub, _ := rs["subquery"].(map[string]any)
			return []fromItem{{name: aliasOf(rs), tables: subqueryTables(sub)}}
		}
	}
	return nil
}

// subqueryTables 收集子查詢被鎖住的表：它自己的 FROM 項目（遞迴），UNION 等集合運算則兩邊都收。
func subqueryTables(n map[string]any) []any {
	sel, _ := n["SelectStmt"].(map[string]any)
	if sel == nil {
		return nil
	}
	var out []any
	for _, it := range fromItems(sel["fromClause"]) {
		out = append(out, it.tables...)
	}
	for _, side := range []string{"larg", "rarg"} {
		if l, ok := sel[side].(map[string]any); ok {
			out = append(out, subqueryTables(l)...)
		}
	}
	return out
}

func addRelation(rel any, kind string, waits bool, out *[]Lock) {
	r, _ := rel.(map[string]any)
	if r == nil {
		return
	}
	table, _ := r["relname"].(string)
	schema, _ := r["schemaname"].(string)
	if table == "" || schema == "" {
		return
	}
	*out = append(*out, Lock{Schema: schema, Table: table, Kind: kind, Waits: waits})
}
