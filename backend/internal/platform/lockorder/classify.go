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

var classified sync.Map

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

var dmlKinds = []struct{ node, kind string }{{"DeleteStmt", "DELETE"}, {"MergeStmt", "MERGE"}, {"UpdateStmt", "UPDATE"}}

func nodeLocks(v map[string]any, out *[]Lock) {
	if s, ok := v["SelectStmt"].(map[string]any); ok {
		setOpLocks(s, out)
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

func setOpLocks(s map[string]any, out *[]Lock) {
	selectLocks(s, out)
	for _, side := range []string{"larg", "rarg"} {
		if b, ok := s[side].(map[string]any); ok {
			setOpLocks(b, out)
		}
	}
}

var strengths = map[string]string{
	"LCS_FORUPDATE":      "FOR UPDATE",
	"LCS_FORNOKEYUPDATE": "FOR NO KEY UPDATE",
	"LCS_FORSHARE":       "FOR SHARE",
	"LCS_FORKEYSHARE":    "FOR KEY SHARE",
}

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

type fromItem struct {
	name   string
	tables []any
}

func aliasOf(r map[string]any) string {
	if alias, ok := r["alias"].(map[string]any); ok {
		name, _ := alias["aliasname"].(string)
		return name
	}
	return ""
}

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
			wrapper, _ := rs["subquery"].(map[string]any)
			sub, _ := wrapper["SelectStmt"].(map[string]any)
			return []fromItem{{name: aliasOf(rs), tables: subqueryTables(sub)}}
		}
	}
	return nil
}

func subqueryTables(sel map[string]any) []any {
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
