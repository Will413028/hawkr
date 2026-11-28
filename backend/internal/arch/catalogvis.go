package arch

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var CatalogNonBuyerProductQueries = map[string]string{
	"AddProductSales":           "銷量累計（訂單事件）",
	"LockProduct":               "評分更新前鎖商品列",
	"SetProductRating":          "評分寫入",
	"IncrementProductFavorites": "收藏數加一",
	"DecrementProductFavorites": "收藏數減一",
	"GetProductStoreIDs":        "商品所屬店家（內部歸屬查詢，不看可見性）",
	"CountAdminProducts":        "後台計數",
	"CountSellerProducts":       "賣家自己的商品計數",
	"GetAdminProduct":           "後台讀單筆（任何狀態）",
	"GetOwnProduct":             "賣家讀自己的商品",
	"GetProductByRequest":       "賣家建立的冪等查詢",
	"InsertProduct":             "賣家建立商品",
	"ListAdminProducts":         "後台清單（任何狀態）",
	"ListSellerProducts":        "賣家自己的清單",
	"LockOwnProduct":            "賣家寫入前鎖自己的商品列",
	"OwnProductDeleted":         "賣家確認自己的商品是否已刪",
	"RecomputePriceRange":       "價格區間重算",
	"SetProductStatus":          "賣家切換自己商品的狀態",
	"SoftDeleteProduct":         "賣家刪除自己的商品",
	"UpdateProductContent":      "賣家更新自己的商品內容",
	"AdjustOwnSkuStock":         "賣家調整自己的庫存",
	"GetOwnLiveSku":             "賣家讀自己的 SKU",
	"GetOwnSkuState":            "賣家讀自己的 SKU 狀態",
	"SetOwnSkuStock":            "賣家設定自己的庫存",
	"SkuInStore":                "SKU 是否屬於某家店（歸屬查詢）",
}

type CatalogVariant string

const (
	CatalogVisible  CatalogVariant = "visible"
	CatalogViewable CatalogVariant = "viewable"
)

type CatalogBuyerQuery struct {
	Variants   []CatalogVariant
	RawAliases []string
}

var CatalogBuyerFacingQueries = map[string]CatalogBuyerQuery{
	"GetPublicProduct":             {Variants: []CatalogVariant{CatalogViewable}},
	"ListPublicProductsLatest":     {Variants: []CatalogVariant{CatalogVisible}},
	"ListPublicProductsPriceAsc":   {Variants: []CatalogVariant{CatalogVisible}},
	"ListPublicProductsPriceDesc":  {Variants: []CatalogVariant{CatalogVisible}},
	"ListPublicProductsSales":      {Variants: []CatalogVariant{CatalogVisible}},
	"ListPublicProductsRating":     {Variants: []CatalogVariant{CatalogVisible}},
	"ListPublicProductsRelevance":  {Variants: []CatalogVariant{CatalogVisible}},
	"SuggestProductNames":          {Variants: []CatalogVariant{CatalogVisible}},
	"SuggestHashtags":              {Variants: []CatalogVariant{CatalogVisible}},
	"TrendingHashtags":             {Variants: []CatalogVariant{CatalogVisible}},
	"ListStoresWithPublicProducts": {Variants: []CatalogVariant{CatalogVisible}},
	"DeductSkuStock":               {Variants: []CatalogVariant{CatalogVisible}},
	"GetLiveSkuStock":              {Variants: []CatalogVariant{CatalogVisible}},
	"GetProductCards":              {Variants: []CatalogVariant{CatalogVisible, CatalogViewable}, RawAliases: []string{"p"}},
	"GetSkuSnapshots":              {Variants: []CatalogVariant{CatalogVisible}, RawAliases: []string{"p"}},
	"GetSkuRestockState":           {Variants: []CatalogVariant{CatalogVisible}, RawAliases: []string{"p"}},
}

func CatalogVisibilityPredicate(variant CatalogVariant, alias string) string {
	status := alias + ".status = 'ACTIVE'"
	if variant == CatalogViewable {
		status = alias + ".status IN ('ACTIVE', 'ARCHIVED')"
	}
	return status + " AND " + alias + ".deleted_at IS NULL" +
		" AND " + alias + ".store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)" +
		" AND NOT EXISTS (SELECT 1 FROM seller.pub_content_takedowns() t WHERE t.content_kind = 'PRODUCT' AND t.content_id = " + alias + ".id)"
}

var (
	catalogQueryName     = regexp.MustCompile(`(?m)^[ \t]*--[ \t]*name:[ \t]*(\w+)`)
	catalogProductsTable = regexp.MustCompile(`(?i)\bcatalog\.products\b`)
	catalogSellerGate    = regexp.MustCompile(`(?i)\bseller\.pub_(exposable_stores|content_takedowns)\b`)
	catalogSetOperation  = regexp.MustCompile(`(?i)\b(union|intersect|except)\b`)
	catalogOuterJoin     = regexp.MustCompile(`(?i)\b(left|right|full|outer)\s+(outer\s+)?join\b`)
	catalogOrToken       = regexp.MustCompile(`(?i)\bor\b`)
	catalogConjunctLead  = regexp.MustCompile(`(?i)\b(where|and)\s$`)
	catalogOrLead        = regexp.MustCompile(`(?i)^\s*or\b`)
	catalogStringLiteral = regexp.MustCompile(`'(?:[^']|'')*'`)
	catalogSpaces        = regexp.MustCompile(`\s+`)
	catalogChildTables   = []string{"product_skus", "product_images", "product_hashtags"}
	catalogAliasStop     = map[string]bool{"where": true, "join": true, "inner": true, "left": true, "right": true, "full": true, "cross": true, "on": true, "set": true, "using": true, "group": true, "order": true, "limit": true, "as": true, "natural": true, "returning": true, "union": true, "intersect": true, "except": true, "for": true}
)

type CatalogQuery struct {
	Name           string
	Body           string
	ReadsProducts  bool
	UsesSellerGate bool
}

func catalogNormalize(s string) string {
	return strings.TrimSpace(catalogSpaces.ReplaceAllString(normalizeSQLNames(s), " "))
}

func CatalogQueries(content string) []CatalogQuery {
	marks := catalogQueryName.FindAllStringSubmatchIndex(content, -1)
	out := make([]CatalogQuery, 0, len(marks))
	for i, m := range marks {
		end := len(content)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		body := catalogNormalize(content[m[1]:end])
		out = append(out, CatalogQuery{
			Name:           content[m[2]:m[3]],
			Body:           body,
			ReadsProducts:  catalogProductsTable.MatchString(body),
			UsesSellerGate: catalogSellerGate.MatchString(body),
		})
	}
	return out
}

type catalogBinding struct {
	alias string
	at    int
}

func catalogBindings(body, table string) []catalogBinding {
	re := regexp.MustCompile(`(?i)\bcatalog\.` + table + `\b(?:\s+(?:as\s+)?(\w+))?`)
	var out []catalogBinding
	for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
		alias := ""
		if m[2] >= 0 {
			alias = body[m[2]:m[3]]
			if catalogAliasStop[strings.ToLower(alias)] && !strings.EqualFold(strings.TrimSpace(body[m[0]+len("catalog."+table):m[2]]), "as") {
				alias = ""
			}
		}
		out = append(out, catalogBinding{alias: alias, at: m[0]})
	}
	return out
}

func catalogGroups(body string) []int {
	masked := catalogStringLiteral.ReplaceAllStringFunc(body, func(m string) string { return strings.Repeat("x", len(m)) })
	groups := make([]int, len(masked))
	stack := []int{0}
	next := 1
	for i := 0; i < len(masked); i++ {
		switch masked[i] {
		case '(':
			groups[i] = stack[len(stack)-1]
			stack = append(stack, next)
			next++
			continue
		case ')':
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		}
		groups[i] = stack[len(stack)-1]
	}
	return groups
}

func catalogCovered(body string, groups []int, b catalogBinding, variant CatalogVariant) bool {
	if b.alias == "" {
		return false
	}
	pred := CatalogVisibilityPredicate(variant, b.alias)
	for from := b.at; ; {
		i := strings.Index(body[from:], pred)
		if i < 0 {
			return false
		}
		k := from + i
		from = k + 1
		if groups[k] != groups[b.at] || !catalogConjunctLead.MatchString(body[:k]) || catalogOrLead.MatchString(body[k+len(pred):]) {
			continue
		}
		return true
	}
}

func catalogOrAtGroup(body string, groups []int, group int) bool {
	masked := catalogStringLiteral.ReplaceAllStringFunc(body, func(m string) string { return strings.Repeat("x", len(m)) })
	for _, m := range catalogOrToken.FindAllStringIndex(masked, -1) {
		if groups[m[0]] == group {
			return true
		}
	}
	return false
}

func CheckCatalogBuyerQuery(file string, q CatalogQuery, spec CatalogBuyerQuery) []string {
	var out []string
	flag := func(format string, a ...any) {
		out = append(out, fmt.Sprintf("catalog 的 %s 的 %s ", file, q.Name)+fmt.Sprintf(format, a...))
	}
	if catalogSetOperation.MatchString(q.Body) || catalogOuterJoin.MatchString(q.Body) {
		flag("用了 UNION／INTERSECT／EXCEPT 或外連接：可見性條件綁不到商品別名，買家面 query 不得使用")
	}
	groups := catalogGroups(q.Body)
	products := catalogBindings(q.Body, "products")
	if len(products) == 0 {
		flag("是買家面 query 卻沒有讀 catalog.products：從 arch.CatalogBuyerFacingQueries 拿掉，或補上可見性條件")
	}
	raw := map[string]bool{}
	for _, a := range spec.RawAliases {
		raw[a] = true
	}
	used := map[CatalogVariant]bool{}
	covered := map[string]catalogBinding{}
	for _, b := range products {
		if b.alias == "" {
			flag("讀 catalog.products 沒有別名：可見性條件綁不到商品別名")
			continue
		}
		if raw[b.alias] {
			continue
		}
		ok := false
		for _, v := range spec.Variants {
			if catalogCovered(q.Body, groups, b, v) {
				ok = true
				used[v] = true
				covered[b.alias+fmt.Sprint(b.at)] = b
			}
		}
		switch {
		case !ok:
			flag("的 catalog.products 別名 %s 沒有在自己的範圍以 WHERE／AND 帶上標準可見性條件（arch.CatalogVisibilityPredicate 的 %v 版本，別名要與該別名一致）", b.alias, spec.Variants)
		case catalogOrAtGroup(q.Body, groups, groups[b.at]):
			flag("的 catalog.products 別名 %s 的範圍有 OR：可見性條件可能被繞過，OR 要放進括號", b.alias)
		}
	}
	for _, v := range spec.Variants {
		if !used[v] {
			flag("宣告了 %s 版本的可見性條件，query 裡沒有任何商品別名綁到它", v)
		}
	}
	for _, b := range products {
		if !raw[b.alias] {
			continue
		}
		linked := false
		for _, c := range covered {
			if c.alias == b.alias {
				continue
			}
			link := c.alias + ".id = " + b.alias + ".id"
			for from := 0; ; {
				i := strings.Index(q.Body[from:], link)
				if i < 0 {
					break
				}
				k := from + i
				from = k + 1
				if groups[k] == groups[c.at] {
					linked = true
				}
			}
		}
		if !linked {
			flag("宣告 %s 是不過濾的商品別名（只回傳可見旗標），但沒有任何帶標準可見性條件的別名在自己的範圍以 <別名>.id = %s.id 對到它", b.alias, b.alias)
		}
	}
	aliases := map[string]bool{}
	for _, b := range products {
		aliases[b.alias] = true
	}
	for _, table := range catalogChildTables {
		for _, b := range catalogBindings(q.Body, table) {
			if b.alias == "" {
				flag("讀 catalog.%s 沒有別名：無法確認它綁到商品", table)
				continue
			}
			linked := false
			for p := range aliases {
				if p != "" && (strings.Contains(q.Body, b.alias+".product_id = "+p+".id") || strings.Contains(q.Body, p+".id = "+b.alias+".product_id")) {
					linked = true
				}
			}
			if !linked {
				flag("讀 catalog.%s（別名 %s）卻沒有以 product_id 對到 query 裡任何 catalog.products 別名：子表的讀取要綁在已帶可見性條件的商品上", table, b.alias)
			}
		}
	}
	return out
}

func CheckCatalogVisibility(file, content string, allowed map[string]string, buyer map[string]CatalogBuyerQuery) []string {
	var out []string
	for _, q := range CatalogQueries(content) {
		spec, isBuyer := buyer[q.Name]
		if isBuyer {
			out = append(out, CheckCatalogBuyerQuery(file, q, spec)...)
			continue
		}
		if q.UsesSellerGate {
			out = append(out, fmt.Sprintf("catalog 的 %s 的 %s 不在 arch.CatalogBuyerFacingQueries 卻參照 seller.pub_exposable_stores 或 seller.pub_content_takedowns：買家看得到的條件只內嵌在買家面 query", file, q.Name))
		}
		if q.ReadsProducts {
			if _, ok := allowed[q.Name]; !ok {
				out = append(out, fmt.Sprintf("catalog 的 %s 的 %s 讀 catalog.products 卻不在 arch.CatalogBuyerFacingQueries（買家面，內嵌標準可見性條件）也不在 arch.CatalogNonBuyerProductQueries（賣家、後台與內部，寫明用途）", file, q.Name))
			}
		}
	}
	return out
}

func StaleCatalogAllowlist(queries []CatalogQuery, allowed map[string]string) []string {
	byName := make(map[string]CatalogQuery, len(queries))
	for _, q := range queries {
		byName[q.Name] = q
	}
	names := make([]string, 0, len(allowed))
	for name := range allowed {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []string
	for _, name := range names {
		switch q, ok := byName[name]; {
		case !ok:
			out = append(out, fmt.Sprintf("arch.CatalogNonBuyerProductQueries 的 %s 在 catalog/db/queries 裡沒有這支 query，把它從白名單拿掉", name))
		case !q.ReadsProducts:
			out = append(out, fmt.Sprintf("arch.CatalogNonBuyerProductQueries 的 %s 沒有直接讀 catalog.products，把它從白名單拿掉", name))
		}
	}
	return out
}

func BuyerFacingOnAllowlist(allowed map[string]string) []string {
	var out []string
	for name := range CatalogBuyerFacingQueries {
		if _, ok := allowed[name]; ok {
			out = append(out, fmt.Sprintf("買家面的 catalog query %s 不得在 arch.CatalogNonBuyerProductQueries：它必須內嵌標準可見性條件", name))
		}
	}
	sort.Strings(out)
	return out
}
