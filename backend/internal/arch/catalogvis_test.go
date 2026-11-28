package arch

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func buyerQ(name, body string) string {
	return "-- name: " + name + " :many\n" + body
}

func visible(alias string) string  { return CatalogVisibilityPredicate(CatalogVisible, alias) }
func viewable(alias string) string { return CatalogVisibilityPredicate(CatalogViewable, alias) }

func TestCatalogVisibilityLintRejectsUnlistedProductQuery(t *testing.T) {
	allowed := map[string]string{"GetOwnProduct": "賣家讀自己的商品"}
	buyer := map[string]CatalogBuyerQuery{}
	for name, q := range map[string]string{
		"沒登記又直接讀商品表": "-- name: ListEverything :many\nSELECT p.id FROM catalog.products p WHERE p.deleted_at IS NULL;",
		"JOIN 商品表":   "-- name: HashtagsOf :many\nSELECT h.name FROM catalog.product_hashtags h JOIN catalog.products p ON p.id = h.product_id;",
		"大寫與引號識別字":   "-- name: Quoted :many\nSELECT 1 FROM \"catalog\".\"PRODUCTS\" p;",
		"沒登記卻帶標準條件":  "-- name: NewBuyerList :many\nSELECT p.id FROM catalog.products p WHERE " + visible("p") + ";",
		"沒登記又直接讀措施":  "-- name: Takedown :many\nSELECT 1 FROM seller.pub_content_takedowns() t;",
	} {
		if got := CheckCatalogVisibility("q.sql", q, allowed, buyer); len(got) == 0 {
			t.Errorf("%s：預期被擋下", name)
		}
	}
	for name, q := range map[string]string{
		"白名單內的賣家面 query": "-- name: GetOwnProduct :one\nSELECT p.id FROM catalog.products p WHERE p.id = sqlc.arg(id)::uuid;",
		"不碰商品表":          "-- name: Skus :many\nSELECT s.id FROM catalog.product_skus s;",
		"註解裡提到商品表":       "-- name: Skus :many\n-- catalog.products 與 seller.pub_exposable_stores 只在註解\nSELECT s.id FROM catalog.product_skus s;",
	} {
		if got := CheckCatalogVisibility("q.sql", q, allowed, buyer); len(got) != 0 {
			t.Errorf("%s：不該被擋：%v", name, got)
		}
	}
	if got := BuyerFacingOnAllowlist(map[string]string{"ListPublicProductsLatest": "x"}); len(got) != 1 {
		t.Errorf("買家面 query 放進白名單要被擋下：%v", got)
	}
	if got := BuyerFacingOnAllowlist(CatalogNonBuyerProductQueries); len(got) != 0 {
		t.Errorf("白名單放了買家面的 query：%v", got)
	}
}

func TestCatalogBuyerQueriesMustCarryTheCanonicalPredicate(t *testing.T) {
	visibleOnly := CatalogBuyerQuery{Variants: []CatalogVariant{CatalogVisible}}
	viewableOnly := CatalogBuyerQuery{Variants: []CatalogVariant{CatalogViewable}}
	flagged := CatalogBuyerQuery{Variants: []CatalogVariant{CatalogViewable}, RawAliases: []string{"p"}}
	cases := []struct {
		name string
		spec CatalogBuyerQuery
		body string
		bad  bool
	}{
		{"標準條件在 WHERE", visibleOnly, "SELECT p.id FROM catalog.products p WHERE " + visible("p") + " AND p.id = sqlc.arg(id)::uuid;", false},
		{"標準條件在 AND 之後、AS 別名", visibleOnly, "SELECT p.id FROM catalog.products AS p WHERE p.id = sqlc.arg(id)::uuid AND " + visible("p") + " ORDER BY p.id;", false},
		{"空白與換行不同仍算同一份", visibleOnly, "SELECT p.id FROM catalog.products p\nWHERE   " + fmtNewlines(visible("p")) + ";", false},
		{"含 ARCHIVED 的版本給單筆頁", viewableOnly, "SELECT p.id FROM catalog.products p WHERE " + viewable("p") + ";", false},
		{"完全沒有條件", visibleOnly, "SELECT p.id FROM catalog.products p WHERE p.id = sqlc.arg(id)::uuid;", true},
		{"只有狀態與刪除", visibleOnly, "SELECT p.id FROM catalog.products p WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL;", true},
		{"版本錯：單筆頁用了 ACTIVE 版本", viewableOnly, "SELECT p.id FROM catalog.products p WHERE " + visible("p") + ";", true},
		{"版本錯：列表用了含 ARCHIVED 版本", visibleOnly, "SELECT p.id FROM catalog.products p WHERE " + viewable("p") + ";", true},
		{"條件少了曝光", visibleOnly, "SELECT p.id FROM catalog.products p WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM seller.pub_content_takedowns() t WHERE t.content_kind = 'PRODUCT' AND t.content_id = p.id);", true},
		{"條件綁在別的別名上", visibleOnly, "SELECT p.id FROM catalog.products p WHERE " + visible("q") + ";", true},
		{"條件被 OR 繞過", visibleOnly, "SELECT p.id FROM catalog.products p WHERE p.id = sqlc.arg(id)::uuid OR " + visible("p") + ";", true},
		{"條件前有 OR 讓 AND 的優先權繞過", visibleOnly, "SELECT p.id FROM catalog.products p WHERE p.id = sqlc.arg(id)::uuid OR p.title = 'x' AND " + visible("p") + ";", true},
		{"條件被 NOT 反轉", visibleOnly, "SELECT p.id FROM catalog.products p WHERE NOT " + visible("p") + ";", true},
		{"條件放在 LEFT JOIN 之後", visibleOnly, "SELECT p.id FROM catalog.product_hashtags h LEFT JOIN catalog.products p ON p.id = h.product_id AND " + visible("p") + ";", true},
		{"UNION 另一支沒有條件", visibleOnly, "SELECT p.id FROM catalog.products p WHERE " + visible("p") + " UNION SELECT p.id FROM catalog.products p;", true},
		{"同別名在子查詢重複綁定只有一份條件", visibleOnly, "SELECT p.id FROM catalog.products p WHERE " + visible("p") + " AND EXISTS (SELECT 1 FROM catalog.products p WHERE p.id = sqlc.arg(id)::uuid);", true},
		{"反例：外層商品列沒有條件，條件只套在對不上的子查詢", visibleOnly, "SELECT p.id FROM catalog.products p WHERE EXISTS (SELECT 1 FROM catalog.products v WHERE " + visible("v") + " AND v.id = other.id);", true},
		{"反例：旗標子查詢的 v 對不到 p", flagged, "SELECT p.id, EXISTS (SELECT 1 FROM catalog.products v WHERE v.id = other.id AND " + viewable("v") + ") AS visible FROM catalog.products p;", true},
		{"旗標查詢：v 對到 p", flagged, "SELECT p.id, EXISTS (SELECT 1 FROM catalog.products v WHERE v.id = p.id AND " + viewable("v") + ") AS visible FROM catalog.products p;", false},
		{"子表以 product_id 綁在有條件的商品上", visibleOnly, "SELECT p.id, (SELECT count(*) FROM catalog.product_skus k WHERE k.product_id = p.id) FROM catalog.products p WHERE " + visible("p") + ";", false},
		{"子表沒綁商品", visibleOnly, "SELECT p.id, (SELECT count(*) FROM catalog.product_skus k WHERE k.id = sqlc.arg(id)::uuid) FROM catalog.products p WHERE " + visible("p") + ";", true},
		{"圖片表沒綁商品", visibleOnly, "SELECT p.id, (SELECT i.object_key FROM catalog.product_images i LIMIT 1) FROM catalog.products p WHERE " + visible("p") + ";", true},
		{"標籤表沒綁商品", visibleOnly, "SELECT h.name FROM catalog.product_hashtags h, catalog.products p WHERE " + visible("p") + ";", true},
		{"宣告了版本卻沒用", CatalogBuyerQuery{Variants: []CatalogVariant{CatalogVisible, CatalogViewable}}, "SELECT p.id FROM catalog.products p WHERE " + visible("p") + ";", true},
		{"沒有讀商品表", visibleOnly, "SELECT s.id FROM catalog.product_skus s;", true},
	}
	for _, c := range cases {
		q := CatalogQueries(buyerQ("Probe", c.body))[0]
		got := CheckCatalogBuyerQuery("q.sql", q, c.spec)
		if c.bad && len(got) == 0 {
			t.Errorf("%s：預期被擋下", c.name)
		}
		if !c.bad && len(got) != 0 {
			t.Errorf("%s：不該被擋：%v", c.name, got)
		}
	}
}

func fmtNewlines(s string) string {
	return fmt.Sprint(strings.ReplaceAll(s, " AND ", "\n  AND "))
}

func TestCatalogLintRequiresNonWhitelistedProductReadsToBeListed(t *testing.T) {
	buyer := map[string]CatalogBuyerQuery{"Public": {Variants: []CatalogVariant{CatalogVisible}}}
	allowed := map[string]string{"GetOwnProduct": "賣家讀自己的商品"}
	newBuyerQuery := "-- name: ListFeatured :many\nSELECT p.id FROM catalog.products p WHERE p.deleted_at IS NULL;"
	if got := CheckCatalogVisibility("q.sql", newBuyerQuery, allowed, buyer); len(got) == 0 {
		t.Error("新的買家面 query 沒進白名單也沒進賣家面清單，要被擋下")
	}
	listed := "-- name: GetOwnProduct :one\nSELECT p.id FROM catalog.products p WHERE p.id = sqlc.arg(id)::uuid;"
	if got := CheckCatalogVisibility("q.sql", listed, allowed, buyer); len(got) != 0 {
		t.Errorf("賣家面清單內的 query 不該被擋：%v", got)
	}
	gate := "-- name: OwnWithGate :one\nSELECT p.id FROM catalog.products p WHERE p.id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e);"
	if got := CheckCatalogVisibility("q.sql", gate, map[string]string{"OwnWithGate": "x"}, buyer); len(got) == 0 {
		t.Error("賣家面清單的 query 不得自己參照 seller 的 pub_*")
	}
	stale := StaleCatalogAllowlist(CatalogQueries(listed), map[string]string{"GetOwnProduct": "x", "Gone": "y"})
	if len(stale) != 1 {
		t.Errorf("過期的白名單項目要被列出：%v", stale)
	}
}

func TestCatalogQueriesCarryTheCanonicalVisibilityPredicate(t *testing.T) {
	var all []CatalogQuery
	for file, content := range catalogQueryFiles(t) {
		for _, v := range CheckCatalogVisibility(file, content, CatalogNonBuyerProductQueries, CatalogBuyerFacingQueries) {
			t.Error(v)
		}
		all = append(all, CatalogQueries(content)...)
	}
	for _, v := range StaleCatalogAllowlist(all, CatalogNonBuyerProductQueries) {
		t.Error(v)
	}
	for name := range CatalogBuyerFacingQueries {
		if !slices.ContainsFunc(all, func(q CatalogQuery) bool { return q.Name == name }) {
			t.Errorf("買家面清單的 %s 在 catalog/db/queries 裡沒有這支 query", name)
		}
	}
}
