//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"uuid"
)

var sqlcMacro = regexp.MustCompile(`sqlc\.n?arg\((\w+)\)`)

func catalogStatement(t *testing.T, file, name string, args map[string]any) (string, []any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "modules", "catalog", "db", "queries", file))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "-- name: "+name+" ")
	if start < 0 {
		t.Fatalf("%s 沒有 %s", file, name)
	}
	body := text[start:]
	body = body[strings.Index(body, "\n")+1:]
	if end := strings.Index(body, "\n-- name: "); end >= 0 {
		body = body[:end]
	}
	body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), ";"))
	var order []string
	sql := sqlcMacro.ReplaceAllStringFunc(body, func(m string) string {
		arg := sqlcMacro.FindStringSubmatch(m)[1]
		i := slices.Index(order, arg)
		if i < 0 {
			order = append(order, arg)
			i = len(order) - 1
		}
		return "$" + strconv.Itoa(i+1)
	})
	values := make([]any, len(order))
	for i, arg := range order {
		v, ok := args[arg]
		if !ok {
			t.Fatalf("%s 的參數 %s 沒有給值", name, arg)
		}
		values[i] = v
	}
	return sql, values
}

type planNode struct {
	Type     string
	Relation string
	Index    string
}

func planNodes(t *testing.T, raw []byte) []planNode {
	t.Helper()
	var doc []struct {
		Plan map[string]any `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc) != 1 {
		t.Fatalf("EXPLAIN 結果不是一份計畫：%v\n%s", err, raw)
	}
	var out []planNode
	var walk func(n map[string]any)
	walk = func(n map[string]any) {
		node := planNode{}
		node.Type, _ = n["Node Type"].(string)
		node.Relation, _ = n["Relation Name"].(string)
		node.Index, _ = n["Index Name"].(string)
		out = append(out, node)
		children, _ := n["Plans"].([]any)
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
	}
	walk(doc[0].Plan)
	return out
}

func TestBuyerVisibilityPredicateIsInlinedAndUsesNamedIndexes(t *testing.T) {
	t.Parallel()
	f := newVisibilityFixture(t)
	ctx := context.Background()
	var storeIDs []int64
	for range 200 {
		id, _ := f.store()
		storeIDs = append(storeIDs, id)
	}
	for _, id := range storeIDs[:20] {
		f.suspend(id)
	}
	var leaf uuid.UUID
	if err := f.d.Pool().QueryRow(ctx, `SELECT id FROM catalog.categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`).Scan(&leaf); err != nil {
		t.Fatal(err)
	}
	f.exec(`INSERT INTO catalog.products (id, store_id, category_id, title, status, first_published_at, min_price, max_price, sales_count, searchable_text, created_at, updated_at)
		SELECT uuidv7(), ($1::bigint[])[1 + (i % 200)], $2::uuid, 'p' || i,
		       CASE WHEN i % 10 = 0 THEN 'DRAFT' WHEN i % 10 = 1 THEN 'ARCHIVED' ELSE 'ACTIVE' END,
		       CASE WHEN i % 10 = 0 THEN NULL ELSE $3::timestamptz - (i || ' minutes')::interval END,
		       100, 200, i % 500, 'p' || i, $3, $3
		FROM generate_series(1, 100000) i`, storeIDs, leaf, f.now)
	f.exec(`INSERT INTO catalog.product_skus (id, product_id, price, stock_qty, created_at, updated_at)
		SELECT uuidv7(), id, 150, 10, $1, $1 FROM catalog.products`, f.now)
	rows, err := f.d.Pool().Query(ctx, `SELECT id, store_id FROM catalog.products WHERE status = 'ACTIVE' AND store_id = ANY($1) ORDER BY id LIMIT 50`, storeIDs[20:])
	if err != nil {
		t.Fatal(err)
	}
	type target struct {
		id    uuid.UUID
		store int64
	}
	var targets []target
	for rows.Next() {
		var x target
		if err := rows.Scan(&x.id, &x.store); err != nil {
			t.Fatal(err)
		}
		targets = append(targets, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(targets) != 50 {
		t.Fatalf("選出 %d 件商品：%v", len(targets), err)
	}
	for _, x := range targets {
		f.takedown(x.store, x.id)
	}
	f.exec(`ANALYZE`)
	var sku uuid.UUID
	if err := f.d.Pool().QueryRow(ctx, `SELECT id FROM catalog.product_skus ORDER BY id LIMIT 1`).Scan(&sku); err != nil {
		t.Fatal(err)
	}

	type planCase struct {
		sql        string
		args       []any
		relations  []string
		wantAnyIdx []string
	}
	cases := map[string]planCase{}
	listModes := []struct{ name, query, index string }{
		{"Latest", "", "products_public_new_idx"},
		{"PriceAsc", "", "products_public_price_asc_idx"},
		{"PriceDesc", "", "products_public_price_desc_idx"},
		{"Sales", "", "products_public_sales_idx"},
		{"Rating", "", "products_public_rating_idx"},
		{"Relevance", "p9999", "products_search_idx"},
	}
	for _, m := range listModes {
		pattern := ""
		if m.query != "" {
			pattern = "%" + m.query + "%"
		}
		for _, shape := range []struct {
			label  string
			stores []int64
			want   []string
		}{
			{"unfiltered", []int64{}, []string{m.index}},
			{"store", []int64{storeIDs[100]}, []string{m.index, "products_store_idx"}},
		} {
			sql, args := catalogStatement(t, "products.sql", "ListPublicProducts"+m.name, map[string]any{
				"query": m.query, "at": f.now, "category_ids": []uuid.UUID{}, "price_min": nil, "price_max": nil, "hashtags": []string{}, "hashtag_all": false,
				"store_ids": shape.stores, "query_pattern": pattern, "after_key": nil, "after_id": nil, "row_limit": int32(20),
			})
			cases["ListPublicProducts"+m.name+"/"+shape.label] = planCase{sql, args, []string{"products"}, shape.want}
		}
	}
	deduct, deductArgs := catalogStatement(t, "stock.sql", "DeductSkuStock", map[string]any{"quantity": int32(1), "at": f.now, "id": sku})
	cases["DeductSkuStock"] = planCase{deduct, deductArgs, []string{"products", "product_skus"}, []string{"product_skus_pkey"}}
	for name, c := range cases {
		var raw []byte
		if err := f.d.Pool().QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+c.sql, c.args...).Scan(&raw); err != nil {
			t.Fatalf("%s：%v\n%s", name, err, c.sql)
		}
		var indexes []string
		for _, n := range planNodes(t, raw) {
			if n.Type == "Function Scan" {
				t.Errorf("%s：計畫有 Function Scan，內嵌可見性條件裡的 seller.pub_exposable_stores 或 pub_content_takedowns 沒有被內聯：\n%s", name, raw)
			}
			if n.Type == "Seq Scan" && slices.Contains(c.relations, n.Relation) {
				t.Errorf("%s：對 catalog.%s 做了 Seq Scan：\n%s", name, n.Relation, raw)
			}
			if n.Index != "" {
				indexes = append(indexes, n.Index)
			}
		}
		if !slices.ContainsFunc(c.wantAnyIdx, func(want string) bool { return slices.Contains(indexes, want) }) {
			t.Errorf("%s：計畫沒有走 %v 其中之一（用到的索引 %v）：\n%s", name, c.wantAnyIdx, indexes, raw)
		}
		t.Logf("%s 用到的索引：%v", name, indexes)
	}
}
