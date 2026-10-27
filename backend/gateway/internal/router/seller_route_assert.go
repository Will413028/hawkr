package router

import (
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/gin-gonic/gin"
)

// appPrefix is the /api/v1/app namespace root (all seller-touchable HTTP lives here).
var appPrefix = APIV1Path + AppPath

// strictSellerPrefixes are the /app areas that are 100% seller-gated — every route
// under them MUST be declared via requireSeller(cap) (46 §3.8-2b). They contain no
// buyer/shared/whitelist-JWTAuth-only routes, so an undeclared route under any of
// them is a deny-by-default hole (new seller route that forgot the guard) and the
// boot assertion panics. Mixed areas (notably /app/sellers, which holds appSellers
// /me JWTAuth-only routes, governance appeal, and shared social/forum) are NOT
// listed here — routes there that lack requireSeller are legitimately buyer-facing /
// shared / exemption (§4.4) and must not panic. The compile-time force
// (requireSeller requires a cap → sellerHandle auto-registers) remains the primary
// deny-by-default net for those; this strict-prefix check is the boot backstop.
var strictSellerPrefixes = []string{
	appPrefix + "/products",
	appPrefix + "/product_import_jobs",
	appPrefix + "/shipments",
	appPrefix + "/shipping_onboardings",
	appPrefix + "/shipping_methods",
	appPrefix + "/shipping_business_days",
	appPrefix + "/shipping_quotes",
	appPrefix + "/categories",
	appPrefix + "/hashtags",
	appPrefix + "/seller/after_sales", // 單數 /seller/，與 /sellers 區隔
	appPrefix + "/seller/invoices",    // 58 號電子發票賣家面（§8.1.6(d)）
}

// sellerAreaExemptions are buyer/shared routes that happen to live UNDER a strict
// seller prefix and are therefore legitimately NOT gated by requireSeller (they must
// not trip the strict-prefix coverage check). Each entry is a documented exception
// (46 §4.4 spirit: exemptions carry a reason, checked by review):
//   - restock_subscriptions POST/DELETE: buyer subscribes to a product's restock
//     notification (routes_buyer.go) — buyer-facing, mounted on /app/products/:id.
var sellerAreaExemptions = map[string]struct{}{
	"POST " + appPrefix + "/products/:id/restock_subscriptions":   {},
	"DELETE " + appPrefix + "/products/:id/restock_subscriptions": {},
}

// underPrefix reports whether path is prefix itself or a sub-path of it (segment
// boundary aware, so "/products" does not match "/product_import_jobs").
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

// adminPrefix is the /api/v1/admin namespace root. Since 56 §A31 this prefix no
// longer implies an admin identity — 26 of its routes are authenticated by an
// operation credential instead — which is exactly why checks (f) and (g) exist.
var adminPrefix = APIV1Path + AdminPath

// delegatedPathForbiddenWords are the 10 substrings a delegated route path may never
// contain (56 §6.1.1 / §8.1.2 第 2 條). This is the D1 leg of the four mechanical
// guarantees: it stops someone mounting "代查物流" under the delegated namespace when
// it is in fact a fulfilment mutation, and it stops it at boot rather than at review.
//
// ⚠️ 這 10 個是**路由路徑的字串禁區**，與 admin repo policy.ts 的 9 個「禁區機器名」
// 是兩份不同的清單（比對對象不同：URL path vs capability 識別字），差集是刻意的——
// 本清單多了 payout／grant／invoice／bank 四個金流財務字，少了 cancel／account-security
// 兩個非路徑慣用字。任一份增修都不自動同步到另一份。
var delegatedPathForbiddenWords = []string{
	"shipment", "fulfillment", "payment", "payout", "grant",
	"refund", "invoice", "bank", "identity", "certification",
}

// adminNoAuthWhitelist is the named list of the ONLY routes allowed to live under
// /api/v1/admin without requireAdmin() (56 §8.1.2 第 4 條, A31). Exactly 26: the 20
// delegated resource endpoints plus S3 GET current, S4 DELETE current,
// GET current/summary, the two media upload-session endpoints and S6 handoff redeem.
//
// The check is bidirectional — an entry that is not mounted is as much a defect as a
// route that is not listed — because a stale whitelist entry silently pre-authorises
// whatever path later takes that name.
var adminNoAuthWhitelist = []string{
	// S3 / S4 / current summary
	"GET " + adminPrefix + "/delegated-operations/current",
	"DELETE " + adminPrefix + "/delegated-operations/current",
	"GET " + adminPrefix + "/delegated-operations/current/summary",
	// S6 交接兌換
	"POST " + adminPrefix + "/delegated-operations/handoff/redeem",
	// 媒體 upload session（requireDelegatedSessionOnly）
	"POST " + adminPrefix + "/delegated-operations/media/upload_sessions",
	"POST " + adminPrefix + "/delegated-operations/media/upload_sessions/:session_id/complete",
	// storefront（4）
	"GET " + adminPrefix + "/sellers/:store_id/storefront",
	"PATCH " + adminPrefix + "/sellers/:store_id/storefront",
	"POST " + adminPrefix + "/sellers/:store_id/storefront/logo",
	"POST " + adminPrefix + "/sellers/:store_id/storefront/cover",
	// products（7）
	"GET " + adminPrefix + "/sellers/:store_id/products",
	"POST " + adminPrefix + "/sellers/:store_id/products",
	"POST " + adminPrefix + "/sellers/:store_id/products/batch-unpublish",
	"GET " + adminPrefix + "/sellers/:store_id/products/:product_id",
	"PATCH " + adminPrefix + "/sellers/:store_id/products/:product_id",
	"POST " + adminPrefix + "/sellers/:store_id/products/:product_id/publish",
	"POST " + adminPrefix + "/sellers/:store_id/products/:product_id/unpublish",
	// posts（6）
	"GET " + adminPrefix + "/sellers/:store_id/posts",
	"POST " + adminPrefix + "/sellers/:store_id/posts",
	"GET " + adminPrefix + "/sellers/:store_id/posts/:post_id",
	"PATCH " + adminPrefix + "/sellers/:store_id/posts/:post_id",
	"POST " + adminPrefix + "/sellers/:store_id/posts/:post_id/publish",
	"POST " + adminPrefix + "/sellers/:store_id/posts/:post_id/unpublish",
	// orders（3）
	"GET " + adminPrefix + "/sellers/:store_id/orders",
	"GET " + adminPrefix + "/sellers/:store_id/orders/:order_no",
	"GET " + adminPrefix + "/sellers/:store_id/orders/:order_no/recipient-contact",
}

// checkAdminCapabilityDeclared is boot check (d): every route registered via
// delegatedHandle carries a non-empty capability. RegisterAdmin already panics on an
// unknown token (the empty string included), so this is the second net — it also
// catches a registry mutated by anything other than that helper.
//
// 作用域是註冊表，不是路徑前綴。用前綴當判準會把 S1 preflight 掃進來（它落在同一個
// 前綴下，卻沒有 capability 可宣告——判斷能不能拿到 capability 正是它的工作），於是
// Gateway 啟動即 panic。
func checkAdminCapabilityDeclared(routes []authz.RegisteredAdminRoute) []string {
	var problems []string
	for _, rr := range routes {
		if rr.Cap == "" {
			problems = append(problems, "代行路由未宣告 capability（宣告＝掛載合一）: "+rr.Method+" "+rr.Path)
		}
	}
	return problems
}

// checkAdminPathForbiddenWords is boot check (e): no delegated route path may contain
// any of the 10 forbidden substrings. Substring match, not segment match — a path
// like /shipment_notes must be caught too.
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

// checkAdminNoWildcard is boot check (f): no wildcard under /api/v1/admin, mirroring
// the rule already in force for /api/v1/app.
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

// checkAdminNoAuthWhitelist is boot check (g): the set of /api/v1/admin routes that
// skip requireAdmin() must equal the named whitelist, and every whitelisted route
// must actually be mounted.
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

// assertRouteCoverage is the boot fail-fast for both namespaces — 46 號's seller
// capability layer (checks a–c) and 56 號's delegated operations (checks d–g). Called
// at the end of SetupRoutes, it panics if any of:
//
//	(a) a wildcard route (`/*`) exists under /api/v1/app — it makes engine.Routes()
//	    ≠ 完整攻擊面 so deny-by-default can no longer be machine-verified (§3.7);
//	(b) a route declared via requireSeller(cap) is missing from engine.Routes()
//	    (registry drift — a path constant changed under a stale registration);
//	(c) a route under a strict seller prefix is NOT declared via requireSeller(cap)
//	    (a new seller route that forgot the guard = deny-by-default hole);
//	(d) a route registered via delegatedHandle declared no capability;
//	(e) a delegated route path contains one of the 10 forbidden substrings;
//	(f) a wildcard route exists under /api/v1/admin;
//	(g) a route under /api/v1/admin skips requireAdmin() without being on the
//	    26-entry A31 whitelist, or a whitelist entry is not mounted.
//
// Panicking here means a mis-wired deploy dies at boot rather than shipping a
// suspension bypass or an ungated delegated route (§12 boot 斷言即部署守門).
func assertRouteCoverage(r *gin.Engine) {
	routes := r.Routes()
	routeSet := make(map[string]struct{}, len(routes))
	for _, ri := range routes {
		routeSet[ri.Method+" "+ri.Path] = struct{}{}
	}

	var problems []string

	// (a) no wildcard under /app.
	for _, ri := range routes {
		if underPrefix(ri.Path, appPrefix) && strings.Contains(ri.Path, "/*") {
			problems = append(problems, "wildcard route under /app (deny-by-default 保證失效，路由表≠攻擊面全集): "+ri.Method+" "+ri.Path)
		}
	}

	// (b) every declared seller route must actually be mounted.
	for _, rr := range authz.Registered() {
		if _, ok := routeSet[rr.Method+" "+rr.Path]; !ok {
			problems = append(problems, "declared seller route missing from engine.Routes() (registry drift): "+rr.Method+" "+rr.Path+" ["+string(rr.Cap)+"]")
		}
	}

	// (c) every route under a strict seller prefix must be declared.
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

	// (d)(e) 代行路由註冊表：漏宣告 capability / 路徑命中禁字。
	adminRoutes := authz.RegisteredAdmin()
	problems = append(problems, checkAdminCapabilityDeclared(adminRoutes)...)
	problems = append(problems, checkAdminPathForbiddenWords(adminRoutes)...)

	// (f) /admin 之下不得有 wildcard。
	problems = append(problems, checkAdminNoWildcard(routes)...)

	// (g) /admin 之下略過 requireAdmin() 的路由必須逐條命中 26 支具名白名單。
	problems = append(problems, checkAdminNoAuthWhitelist(routeSet, registeredAdminNoAuth(), adminNoAuthWhitelist)...)

	// 每一條經 delegatedHandle 註冊的路由都必須掛在 credential-only 群組上：掛回
	// requireAdmin 那個 group 會讓工作區每一次請求都拿到 9261，而路由表看起來完全正常。
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
