package arch

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
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

var (
	catalogQueryName      = regexp.MustCompile(`(?m)^[ \t]*--[ \t]*name:[ \t]*(\w+)`)
	catalogProductsTable  = regexp.MustCompile(`(?i)\bcatalog\.products\b`)
	catalogVisibilityFunc = regexp.MustCompile(`(?i)\bcatalog\.buyer_(visible|viewable)_products\s*\(`)
	catalogSellerGate     = regexp.MustCompile(`(?i)\bseller\.pub_(exposable_stores|content_takedowns)\b`)
)

type CatalogQuery struct {
	Name           string
	ReadsProducts  bool
	UsesVisibility bool
	UsesSellerGate bool
}

func CatalogQueries(content string) []CatalogQuery {
	marks := catalogQueryName.FindAllStringSubmatchIndex(content, -1)
	out := make([]CatalogQuery, 0, len(marks))
	for i, m := range marks {
		end := len(content)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		body := normalizeSQLNames(content[m[1]:end])
		out = append(out, CatalogQuery{
			Name:           content[m[2]:m[3]],
			ReadsProducts:  catalogProductsTable.MatchString(body),
			UsesVisibility: catalogVisibilityFunc.MatchString(body),
			UsesSellerGate: catalogSellerGate.MatchString(body),
		})
	}
	return out
}

func CheckCatalogVisibility(file, content string, allowed map[string]string) []string {
	var out []string
	for _, q := range CatalogQueries(content) {
		if q.UsesSellerGate {
			out = append(out, fmt.Sprintf("catalog 的 %s 的 %s 直接參照 seller.pub_exposable_stores 或 seller.pub_content_takedowns：買家看得到的條件只有 db/objects.sql 的 catalog.buyer_visible_products 與 catalog.buyer_viewable_products 一份定義", file, q.Name))
		}
		if q.ReadsProducts && !q.UsesVisibility {
			if _, ok := allowed[q.Name]; !ok {
				out = append(out, fmt.Sprintf("catalog 的 %s 的 %s 讀 catalog.products 卻沒有經 catalog.buyer_visible_products 或 catalog.buyer_viewable_products：買家面的讀取要走函式；賣家、後台與內部用的在 arch.CatalogNonBuyerProductQueries 登記並寫明用途", file, q.Name))
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
		case !q.ReadsProducts || q.UsesVisibility:
			out = append(out, fmt.Sprintf("arch.CatalogNonBuyerProductQueries 的 %s 不是直接讀 catalog.products 的非買家面 query（沒讀 catalog.products，或已經走可見性函式），把它從白名單拿掉", name))
		}
	}
	return out
}

var CatalogBuyerFacingQueries = []string{
	"GetPublicProduct", "ListPublicProducts", "SuggestProductNames", "SuggestHashtags", "TrendingHashtags",
	"GetProductCards", "ListStoresWithPublicProducts", "GetSkuSnapshots", "GetSkuRestockState", "DeductSkuStock",
}

func BuyerFacingOnAllowlist(allowed map[string]string) []string {
	var out []string
	for _, name := range CatalogBuyerFacingQueries {
		if _, ok := allowed[name]; ok {
			out = append(out, fmt.Sprintf("買家面的 catalog query %s 不得在 arch.CatalogNonBuyerProductQueries：它必須走可見性函式", name))
		}
	}
	slices.Sort(out)
	return out
}
