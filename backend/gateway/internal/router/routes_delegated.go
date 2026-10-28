package router

import (
	"net/http"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerDelegatedOperationRoutes mounts the 40 endpoints of 56 號 ADM 代行營運
// (§6.1.1 ＋ #2444 的授權列治理 ＋ §11.3 P1 的跨工作階段查詢), split across two
// groups that share the /api/v1/admin prefix:
//
//	admin        — 14 支 ADM host 端點，身分＝ADM session（既有 JWTAuth + requireAdmin）
//	adminNoAuth  — 26 支，身分＝operation credential（A31 起前綴不再蘊含 admin 身分）
//
// 那 26 支就是 §8.1.2 boot 斷言第 4 條的具名白名單。掛錯 group 的症狀是不對稱的：
// 把 www host 組掛回 requireAdmin ⇒ 工作區每一次輪詢 9261（吵，看得見）；反過來把一支
// 治理端點掛到 credential-only 群組上 ⇒ 一支**沒有任何身分要求**的治理端點，而 review
// 時它與其他 /admin/ 路由長得一模一樣，差別只在註冊時用了哪一個 group 變數。
//
// gin 註冊字串一律 snake_case（`:store_id`／`:order_no`），不是文件渲染用的
// `{storeId}`：① API 設計規範要求 URL snake_case；② 既有
// routes_admin.go 已用 `:store_id` 把 /api/v1/admin/sellers/ 這一段定調，同段再註冊
// `:storeId` 會在 gin 的 tree.go 直接 panic；③ metric 的 path label 取自
// c.FullPath()，寫成 camelCase 的查詢與告警會恆為零命中，而零命中與「真的沒有流量」
// 在 PromQL 上分不出來。
func registerDelegatedOperationRoutes(admin, adminNoAuth *gin.RouterGroup, h *handler.Holder) {
	deps := newDelegationDeps(h)

	// ── ADM host 組（14 支）：身分＝ADM session，失效時 9261 / 401 ───────────
	adminHandle(admin, http.MethodGet, "/sellers/:store_id/delegated-operation/preflight", h.GetDelegatedOperationPreflight)
	adminHandle(admin, http.MethodPost, "/delegated-operations", h.CreateDelegatedOperation)
	adminHandle(admin, http.MethodPost, "/delegated-operations/:session_id/revoke", h.RevokeDelegatedOperation)
	adminHandle(admin, http.MethodGet, "/delegated-operations/:session_id/audit-logs", h.ListDelegatedAuditLogs)
	adminHandle(admin, http.MethodGet, "/delegated-operations/:session_id/summary", h.GetDelegatedSessionSummary)

	// ── 跨工作階段治理查詢（2 支，§11.3 P1）：同樣是 ADM host 組 ─────────────
	//
	// 它們是**治理回看**，不是代行操作：不吃 operation credential、不宣告 capability，
	// 掛 delegatedHandle 會落進 credential-only 白名單而讓 boot 斷言 panic。
	//
	// ⚠️ 也刻意不掛 admin_accounts.delegation_enabled 的 allowlist（那把尺管的是「誰
	// 可以發起代行」）：掛了會造出「查得到單一工作階段、卻查不到列表」的矛盾，而
	// 稽核角色本來就不該有發起代行的資格。
	//
	// ⚠️ audit-logs 是靜態段而**不是** `/delegated-operation-audit-logs`：ADM 的
	// catch-all BFF 逐 segment 全等比對 `delegated-operation` / `delegated-operations`
	// 才拒絕代理（bff-boundary.ts 的 isDelegatedOperationNamespace），單一 segment 的
	// 那個寫法兩者都不等 ⇒ 會被 catch-all 代理掉、繞過 parser／safeUpstreamError／
	// no-store，而且不會有東西轉紅。
	adminHandle(admin, http.MethodGet, "/delegated-operations", h.ListDelegatedOperations)
	adminHandle(admin, http.MethodGet, "/delegated-operations/audit-logs", h.ListAllDelegatedAuditLogs)

	// ── 授權列治理（7 支，#2444）：同樣是 ADM host 組 ────────────────────────
	//
	// 它們管的是「這家店可不可以被代行」，發生在任何工作階段之前 ⇒ 沒有 credential、
	// 不宣告 capability，掛 delegatedHandle 會要求一個當下還不存在的東西。
	//
	// ⚠️ 路徑第一段必須是 `delegated-operations`：ADM 那邊的 catch-all BFF 只認這個
	// 字面值來拒絕代理（bff-boundary.ts 的 isDelegatedOperationNamespace），改名等於
	// 開一條繞過 parser／safeUpstreamError／no-store 的路，而且不會有東西轉紅。
	//
	// 靜態段 `capabilities`／`authorizations` 與同層的 `:session_id` 並存是 gin 支援的
	// （`current`／`handoff`／`media` 三者早已如此），不會在 tree 註冊時 panic。
	adminHandle(admin, http.MethodGet, "/delegated-operations/capabilities", h.ListDelegationCapabilities)
	adminHandle(admin, http.MethodGet, "/delegated-operations/authorizations", h.ListDelegationAuthorizations)
	adminHandle(admin, http.MethodPost, "/delegated-operations/authorizations", h.GrantDelegationAuthorization)
	adminHandle(admin, http.MethodGet, "/delegated-operations/authorizations/:store_id", h.GetDelegationAuthorization)
	adminHandle(admin, http.MethodPut, "/delegated-operations/authorizations/:store_id", h.UpdateDelegationAuthorization)
	adminHandle(admin, http.MethodPost, "/delegated-operations/authorizations/:store_id/revoke", h.RevokeDelegationAuthorization)
	adminHandle(admin, http.MethodGet, "/delegated-operations/authorizations/:store_id/history", h.ListDelegationAuthorizationHistory)

	// ── 工作階段自述（3 支）：handler 自行解析 credential，含終態 ─────────────
	credentialHandle(adminNoAuth, http.MethodGet, "/delegated-operations/current", h.GetCurrentDelegatedOperation)
	credentialHandle(adminNoAuth, http.MethodDelete, "/delegated-operations/current", h.EndCurrentDelegatedOperation)
	credentialHandle(adminNoAuth, http.MethodGet, "/delegated-operations/current/summary", h.GetCurrentDelegatedSessionSummary)

	// ── S6 交接兌換（1 支）：四個 helper 一個都不掛 ──────────────────────────
	handoffHandle(adminNoAuth, http.MethodPost, "/delegated-operations/handoff/redeem", h.RedeemDelegationHandoff)

	// ── 媒體 upload session（2 支）：不驗 capability，但一律算活動 ────────────
	sessionOnlyHandle(adminNoAuth, http.MethodPost, "/delegated-operations/media/upload_sessions", deps, h.CreateDelegatedMediaUploadSession)
	sessionOnlyHandle(adminNoAuth, http.MethodPost, "/delegated-operations/media/upload_sessions/:session_id/complete", deps, h.CompleteDelegatedMediaUploadSession)

	// ── 20 支代行資源端點：capability 為必填參數，宣告＝掛載合一 ──────────────
	sellers := "/sellers/:store_id"

	// storefront（4）
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/storefront", authz.AdminCapStorefrontRead, deps, h.DelegatedStorefrontUnavailable)
	delegatedHandle(adminNoAuth, http.MethodPatch, sellers+"/storefront", authz.AdminCapStorefrontWrite, deps, h.DelegatedStorefrontUnavailable)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/storefront/logo", authz.AdminCapStorefrontWrite, deps, h.DelegatedStorefrontUnavailable)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/storefront/cover", authz.AdminCapStorefrontWrite, deps, h.DelegatedStorefrontUnavailable)

	// products（7）。batch-unpublish 的 capability 是 products.publish，不是任何
	// 「批次」專用值——批次與逐筆做的是同一件事，只有跳數不同。
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/products", authz.AdminCapProductsRead, deps, h.ListDelegatedProducts)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products", authz.AdminCapProductsWrite, deps, h.CreateDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products/batch-unpublish", authz.AdminCapProductsPublish, deps, h.BatchUnpublishDelegatedProducts)
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/products/:product_id", authz.AdminCapProductsRead, deps, h.GetDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPatch, sellers+"/products/:product_id", authz.AdminCapProductsWrite, deps, h.UpdateDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products/:product_id/publish", authz.AdminCapProductsPublish, deps, h.PublishDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products/:product_id/unpublish", authz.AdminCapProductsPublish, deps, h.UnpublishDelegatedProduct)

	// posts（6）
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/posts", authz.AdminCapPostsRead, deps, h.ListDelegatedPosts)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/posts", authz.AdminCapPostsWrite, deps, h.CreateDelegatedPost)
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/posts/:post_id", authz.AdminCapPostsRead, deps, h.GetDelegatedPost)
	delegatedHandle(adminNoAuth, http.MethodPatch, sellers+"/posts/:post_id", authz.AdminCapPostsWrite, deps, h.UpdateDelegatedPost)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/posts/:post_id/publish", authz.AdminCapPostsPublish, deps, h.PublishDelegatedPost)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/posts/:post_id/unpublish", authz.AdminCapPostsPublish, deps, h.UnpublishDelegatedPost)

	// orders（3，全部唯讀）。recipient-contact 是獨立的 PII 揭露動作，仍然只吃
	// orders.read——它必落稽核的性質由 order-service 斷言 audit_log_id 非空承擔，
	// 不是靠第十個 capability。
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/orders", authz.AdminCapOrdersRead, deps, h.ListDelegatedOrders)
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/orders/:order_no", authz.AdminCapOrdersRead, deps, h.GetDelegatedOrder)
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/orders/:order_no/recipient-contact", authz.AdminCapOrdersRead, deps, h.RevealDelegatedRecipientContact)
}

// handoffHandle mounts S6 with none of the four guards attached, and records it as a
// credential-only route so the boot whitelist still covers it.
//
// 掛任何一個都會讓交接永遠失敗：delegatedHandle／credentialHandle 都會要求一個它正要
// 去換的東西，adminHandle 則要求一個 www 上的瀏覽器沒有的東西。它的授權完全由
// handoff code 自己那四道防護承擔（單句 CAS、六個述詞、一次性、四種成因共用 9282），
// 全部落在 delegation 模組的 RedeemHandoff 內，一道都不得推給呼叫端。
func handoffHandle(g *gin.RouterGroup, method, relPath string, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	registerAdminNoAuth(method, full)
	middleware.RegisterDelegatedRoute(method, full)
	g.Handle(method, relPath, h)
}
