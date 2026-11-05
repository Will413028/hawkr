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

// selectLocks 處理一層 SELECT 的鎖定子句：有 OF 就只鎖列出的表，沒有就鎖 FROM 裡的每個基底表。
// 基底表是 FROM 清單直接寫的表與 JOIN 兩邊（遞迴）的表；JOIN ON 的條件、LATERAL、函式參數與子查詢裡出現的表
// 不算（不當成被這個子句鎖住）。OF 後面寫的是 FROM 裡的別名或不帶 schema 的表名，所以對回基底表才知道 schema。
func selectLocks(s map[string]any, out *[]Lock) {
	clauses, _ := s["lockingClause"].([]any)
	from := baseRelations(s["fromClause"])
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
			for _, rv := range from {
				addRelation(rv, kind, waits, out)
			}
			continue
		}
		for _, r := range rels {
			name, _ := r.(map[string]any)["RangeVar"].(map[string]any)["relname"].(string)
			for _, rv := range from {
				if refName(rv) == name {
					addRelation(rv, kind, waits, out)
				}
			}
		}
	}
}

// refName 是 FROM 裡的表在同一句 SQL 裡被引用的名字：有別名用別名，沒有就是表名。
func refName(rv any) string {
	r, _ := rv.(map[string]any)
	if alias, ok := r["alias"].(map[string]any); ok {
		if name, _ := alias["aliasname"].(string); name != "" {
			return name
		}
	}
	name, _ := r["relname"].(string)
	return name
}

// baseRelations 收集 FROM 的基底表：RangeVar 本身，以及 JoinExpr 左右兩邊（遞迴）的 RangeVar。
// 不進 JOIN 的 quals、RangeSubselect、RangeFunction 等其他節點。
func baseRelations(n any) []any {
	switch v := n.(type) {
	case []any:
		var out []any
		for _, c := range v {
			out = append(out, baseRelations(c)...)
		}
		return out
	case map[string]any:
		if rv, ok := v["RangeVar"].(map[string]any); ok {
			return []any{rv}
		}
		if j, ok := v["JoinExpr"].(map[string]any); ok {
			return append(baseRelations(j["larg"]), baseRelations(j["rarg"])...)
		}
	}
	return nil
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
