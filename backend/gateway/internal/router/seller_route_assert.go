package router

import (
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/gin-gonic/gin"
)

var appPrefix = APIV1Path + AppPath

var strictSellerPrefixes = []string{
	appPrefix + "/products",
	appPrefix + "/shipments",
	appPrefix + "/shipping_onboardings",
	appPrefix + "/shipping_methods",
	appPrefix + "/shipping_business_days",
	appPrefix + "/shipping_quotes",
	appPrefix + "/categories",
	appPrefix + "/hashtags",
	appPrefix + "/seller/after_sales",
	appPrefix + "/seller/invoices",
}

var sellerAreaExemptions = map[string]struct{}{
	"POST " + appPrefix + "/products/:id/restock_subscriptions":   {},
	"DELETE " + appPrefix + "/products/:id/restock_subscriptions": {},
}

func underPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func underAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if underPrefix(path, p) {
			return true
		}
	}
	return false
}

var adminPrefix = APIV1Path + AdminPath

var delegatedPathForbiddenWords = []string{
	"shipment", "fulfillment", "payment", "payout", "grant",
	"refund", "invoice", "bank", "identity", "certification",
}

var adminNoAuthWhitelist = []string{
	"GET " + adminPrefix + "/delegated-operations/current",
	"DELETE " + adminPrefix + "/delegated-operations/current",
	"GET " + adminPrefix + "/delegated-operations/current/summary",
	"POST " + adminPrefix + "/delegated-operations/handoff/redeem",
	"GET " + adminPrefix + "/sellers/:store_id/storefront",
	"PATCH " + adminPrefix + "/sellers/:store_id/storefront",
	"POST " + adminPrefix + "/sellers/:store_id/storefront/logo",
	"POST " + adminPrefix + "/sellers/:store_id/storefront/cover",
	"GET " + adminPrefix + "/sellers/:store_id/products",
	"POST " + adminPrefix + "/sellers/:store_id/products",
	"POST " + adminPrefix + "/sellers/:store_id/products/batch-unpublish",
	"GET " + adminPrefix + "/sellers/:store_id/products/:product_id",
	"PATCH " + adminPrefix + "/sellers/:store_id/products/:product_id",
	"POST " + adminPrefix + "/sellers/:store_id/products/:product_id/publish",
	"POST " + adminPrefix + "/sellers/:store_id/products/:product_id/unpublish",
	"GET " + adminPrefix + "/sellers/:store_id/orders",
	"GET " + adminPrefix + "/sellers/:store_id/orders/:order_no",
	"GET " + adminPrefix + "/sellers/:store_id/orders/:order_no/recipient-contact",
}

func checkAdminCapabilityDeclared(routes []authz.RegisteredAdminRoute) []string {
	var problems []string
	for _, rr := range routes {
		if rr.Cap == "" {
			problems = append(problems, "代行路由未宣告 capability（宣告＝掛載合一）: "+rr.Method+" "+rr.Path)
		}
	}
	return problems
}

func checkAdminPathForbiddenWords(routes []authz.RegisteredAdminRoute) []string {
	var problems []string
	for _, rr := range routes {
		for _, word := range delegatedPathForbiddenWords {
			if strings.Contains(rr.Path, word) {
				problems = append(problems,
					"代行路由路徑命中履約／金流禁字 \""+word+"\"（三層鎖定的層 2）: "+rr.Method+" "+rr.Path)
			}
		}
	}
	return problems
}

func checkAdminNoWildcard(routes []gin.RouteInfo) []string {
	var problems []string
	for _, ri := range routes {
		if underPrefix(ri.Path, adminPrefix) && strings.Contains(ri.Path, "/*") {
			problems = append(problems,
				"wildcard route under /admin (deny-by-default 保證失效，路由表≠攻擊面全集): "+ri.Method+" "+ri.Path)
		}
	}
	return problems
}

func checkAdminNoAuthWhitelist(mounted map[string]struct{}, noAuth []string, whitelist []string) []string {
	allowed := make(map[string]struct{}, len(whitelist))
	for _, w := range whitelist {
		allowed[w] = struct{}{}
	}
	registered := make(map[string]struct{}, len(noAuth))
	for _, k := range noAuth {
		registered[k] = struct{}{}
	}

	var problems []string
	for _, k := range noAuth {
		if _, ok := allowed[k]; !ok {
			problems = append(problems,
				"/api/v1/admin 前綴不再蘊含 admin 身分，此路由略過 requireAdmin() 卻未列入 A31 白名單: "+k)
		}
	}
	for _, w := range whitelist {
		if _, ok := registered[w]; !ok {
			problems = append(problems, "A31 白名單有一支未被掛載為 credential-only 路由: "+w)
			continue
		}
		if _, ok := mounted[w]; !ok {
			problems = append(problems, "A31 白名單路由未出現在 engine.Routes()（registry 漂移）: "+w)
		}
	}
	return problems
}

func assertRouteCoverage(r *gin.Engine) {
	routes := r.Routes()
	routeSet := make(map[string]struct{}, len(routes))
	for _, ri := range routes {
		routeSet[ri.Method+" "+ri.Path] = struct{}{}
	}

	var problems []string

	for _, ri := range routes {
		if underPrefix(ri.Path, appPrefix) && strings.Contains(ri.Path, "/*") {
			problems = append(problems, "wildcard route under /app (deny-by-default 保證失效，路由表≠攻擊面全集): "+ri.Method+" "+ri.Path)
		}
	}

	for _, rr := range authz.Registered() {
		if _, ok := routeSet[rr.Method+" "+rr.Path]; !ok {
			problems = append(problems, "declared seller route missing from engine.Routes() (registry drift): "+rr.Method+" "+rr.Path+" ["+string(rr.Cap)+"]")
		}
	}

	for _, ri := range routes {
		if !underAnyPrefix(ri.Path, strictSellerPrefixes) {
			continue
		}
		if _, exempt := sellerAreaExemptions[ri.Method+" "+ri.Path]; exempt {
			continue
		}
		if _, ok := authz.LookupRoute(ri.Method, ri.Path); !ok {
			problems = append(problems, "seller-area route not declared via requireSeller(cap) (deny-by-default 漏宣告): "+ri.Method+" "+ri.Path)
		}
	}

	adminRoutes := authz.RegisteredAdmin()
	problems = append(problems, checkAdminCapabilityDeclared(adminRoutes)...)
	problems = append(problems, checkAdminPathForbiddenWords(adminRoutes)...)

	problems = append(problems, checkAdminNoWildcard(routes)...)

	problems = append(problems, checkAdminNoAuthWhitelist(routeSet, registeredAdminNoAuth(), adminNoAuthWhitelist)...)

	noAuthSet := make(map[string]struct{})
	for _, k := range registeredAdminNoAuth() {
		noAuthSet[k] = struct{}{}
	}
	for _, rr := range adminRoutes {
		if _, ok := noAuthSet[rr.Method+" "+rr.Path]; !ok {
			problems = append(problems,
				"代行資源路由未掛在 credential-only 群組（www host 會拿到 9261）: "+rr.Method+" "+rr.Path)
		}
	}

	if len(problems) > 0 {
		panic("route capability assertion failed (46 §3.8 / 56 §8.1.2):\n  " + strings.Join(problems, "\n  "))
	}
}
