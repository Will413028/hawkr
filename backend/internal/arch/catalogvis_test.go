package arch

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func catalogQueryFiles(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(moduleRoot(t), "internal", "modules", "catalog", "db", "queries", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("沒有任何 catalog query 檔，檢查範圍不對")
	}
	out := make(map[string]string, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = string(raw)
	}
	return out
}

func TestCatalogVisibilityLintRejectsUnlistedProductQuery(t *testing.T) {
	allowed := map[string]string{"GetOwnProduct": "賣家讀自己的商品"}
	for name, q := range map[string]string{
		"沒登記又直接讀商品表":   "-- name: ListEverything :many\nSELECT p.id FROM catalog.products p WHERE p.deleted_at IS NULL;",
		"JOIN 商品表":     "-- name: HashtagsOf :many\nSELECT h.name FROM catalog.product_hashtags h JOIN catalog.products p ON p.id = h.product_id;",
		"大寫與引號識別字":     "-- name: Quoted :many\nSELECT 1 FROM \"catalog\".\"PRODUCTS\" p;",
		"直接 join 店家曝光": "-- name: Direct :many\nSELECT p.id FROM catalog.buyer_visible_products(sqlc.arg(at)::timestamptz) AS p WHERE p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e);",
		"直接讀措施":        "-- name: Takedown :many\nSELECT 1 FROM seller.pub_content_takedowns() t;",
	} {
		if got := CheckCatalogVisibility("q.sql", q, allowed); len(got) == 0 {
			t.Errorf("%s：預期被擋下", name)
		}
	}
	for name, q := range map[string]string{
		"白名單內的賣家面 query": "-- name: GetOwnProduct :one\nSELECT p.id FROM catalog.products p WHERE p.id = sqlc.arg(id)::uuid;",
		"走可見性函式":         "-- name: GetPublicProduct :one\nSELECT p.id FROM catalog.buyer_viewable_products(sqlc.arg(at)::timestamptz) AS p WHERE p.id = sqlc.arg(id)::uuid;",
		"不碰商品表":          "-- name: Skus :many\nSELECT s.id FROM catalog.product_skus s;",
		"註解裡提到商品表":       "-- name: Skus :many\n-- catalog.products 與 seller.pub_exposable_stores 只在註解\nSELECT s.id FROM catalog.product_skus s;",
	} {
		if got := CheckCatalogVisibility("q.sql", q, allowed); len(got) != 0 {
			t.Errorf("%s：不該被擋：%v", name, got)
		}
	}
	if got := BuyerFacingOnAllowlist(map[string]string{"ListPublicProducts": "x"}); len(got) != 1 {
		t.Errorf("買家面 query 放進白名單要被擋下：%v", got)
	}
	if got := BuyerFacingOnAllowlist(CatalogNonBuyerProductQueries); len(got) != 0 {
		t.Errorf("白名單放了買家面的 query：%v", got)
	}
}

func TestCatalogQueriesReadProductsOnlyThroughTheVisibilityFunctions(t *testing.T) {
	var all []CatalogQuery
	for file, content := range catalogQueryFiles(t) {
		for _, v := range CheckCatalogVisibility(file, content, CatalogNonBuyerProductQueries) {
			t.Error(v)
		}
		all = append(all, CatalogQueries(content)...)
	}
	for _, v := range StaleCatalogAllowlist(all, CatalogNonBuyerProductQueries) {
		t.Error(v)
	}
	for _, name := range CatalogBuyerFacingQueries {
		if !slices.ContainsFunc(all, func(q CatalogQuery) bool { return q.Name == name }) {
			t.Errorf("買家面清單的 %s 在 catalog/db/queries 裡沒有這支 query", name)
		}
	}
}
