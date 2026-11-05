// Package pgtree 是 SQL 解析樹的共用走訪：parse 一句 SQL，依固定順序走過每個節點。querycheck（規則 2）與
// lockorder（規則 4）都從解析樹找東西，共用這一份，避免各寫一套解析與走訪。
package pgtree

import (
	"encoding/json"
	"fmt"
	"sort"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Parse 把 sql 解析成 pg_query 的 JSON 樹（map[string]any、[]any 與純量）。
func Parse(sql string) (any, error) {
	js, err := pg_query.ParseToJSON(sql)
	if err != nil {
		return nil, fmt.Errorf("無法解析 SQL：%w", err)
	}
	var tree any
	if err := json.Unmarshal([]byte(js), &tree); err != nil {
		return nil, fmt.Errorf("解析樹：%w", err)
	}
	return tree, nil
}

// Walk 深度優先走過 n 底下每個物件節點，對每個節點呼叫 visit，再走它的子節點。
// 同一個物件的鍵依字串排序、陣列依原順序，所以相同的 SQL 每次的走訪順序相同（Go 的 map 迭代順序是隨機的，
// 不能直接 range）。
func Walk(n any, visit func(map[string]any)) {
	switch v := n.(type) {
	case map[string]any:
		visit(v)
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			Walk(v[k], visit)
		}
	case []any:
		for _, c := range v {
			Walk(c, visit)
		}
	}
}
