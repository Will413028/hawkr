package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

// SetupRoutes 配置所有 API 路由
// API 設計：RESTful 風格，三大 Namespace: /public, /app, /admin
func SetupRoutes(r *gin.Engine, h *handler.Holder) {
	// 基礎設施路由 (swagger, metrics)
	registerInfraRoutes(r, h)

	// API v1 路由
	v1 := r.Group(APIV1Path)
	registerHealthRoutes(v1, h)

	// 三大 Namespace
	public := v1.Group(PublicPath) // /api/v1/public — 匿名可存取
	app := v1.Group(AppPath)       // /api/v1/app — 需要 JWT 認證
	admin := v1.Group(AdminPath)   // /api/v1/admin — 管理員專用
	// PrivateNoStore 必須排在 JWTAuth／requireAdmin **之前**：gin 的中介層鏈依註冊序
	// 執行，排在之後的話 401／403 abort 時它根本不會跑，那兩種回應就漏掉 Cache-Control。
	// 58 §6.1.0(11) 逐字寫了這條，而 2026-08-29 的獨立驗收實測到 /admin/invoices/** 的
	// 401 確實 Cache-Control 為空（三個 app 群組都對，只有 ADM 這組踩到）。
	// 掛在父群組而不是發票那一組：admin 的回應本來就沒有一個該被快取，這是 admin 的
	// 性質不是發票的需求。自己設 Cache-Control 的 handler 仍然會覆蓋它（本中介層在
	// c.Next() 之前設，handler 後設者勝）。
	admin.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), requireAdmin())

	// A31（56 號）：/api/v1/admin 前綴不再蘊含 admin 身分。這個 sibling group 不掛
	// JWTAuth／requireAdmin，身分改由 operation credential 承擔——工作區在
	// hawkr-website 上，那裡沒有 ADM session（host-only cookie 送不到）。
	// 誰能掛在這上面由 boot 斷言第 4 條的 26 支具名白名單機械守住。
	adminNoAuth := v1.Group(AdminPath)

	// 五個保留 header 一律在群組入口刪掉。目前沒有任何 handler 讀它們，這正是要現在
	// 刪的理由：applyIdentity「一律用 Set 覆蓋」的慣例對新增的 header 不適用，只要
	// 日後有人加了讀取點，外部就能偽造一個代行身分。
	admin.Use(middleware.StripDelegationReservedHeaders())
	adminNoAuth.Use(middleware.StripDelegationReservedHeaders())

	// 負責微服務路由（payment, media, product, order, cart）
	pubProducts := registerProductRoutes(public, app, admin, h)
	registerReviewRoutes(pubProducts, app, h)
	registerPaymentRoutes(app, admin, h)
	registerOrderRoutes(app, admin, h)
	registerCheckoutRoutes(app, h)
	registerCartRoutes(public, app, h)
	registerMediaRoutes(public, app, h)
	registerShippingRoutes(app, admin, h)
	registerLogisticsFinanceRoutes(app, admin, h)

	// PayUni Webhook（頂層路由，外部回調無需 JWT）
	registerPayUniWebhookRoute(r)

	// 付款結果回調頁面（頂層路由，PayUni 導回無需 JWT）
	registerPaymentReturnRoute(r, h)

	// PAYUNi 付款頁重導向（頂層路由，供 WebView 開啟無需 JWT）
	registerPaymentRedirectRoute(r, h)

	// 超商門市地圖回調（頂層路由，PayUni 導回 + WebView 開啟無需 JWT）
	registerStoreMapReturnRoutes(r, h)

	registerPostRoutes(public, app, h)
	registerModerationRoutes(app, admin, h)
	registerGovernanceRoutes(public, app, h)
	registerInvoiceRoutes(app, admin, h) // 58 號電子發票（置於 governance 之後）
	registerSearchRoutes(public, h)
	registerSellerRoutes(public, app, h)
	registerBuyerRoutes(public, app, h)
	registerCouponRoutes(app, admin, h)
	registerAfterSalesRoutes(app, r, h)
	registerProductImportRoutes(app, h)
	registerPricebookRoutes(app, h)
	registerChatRoutes(app, h)
	registerAdminNonManagedRoutes(app, admin, h)
	registerCRMRoutes(app, h)
	registerAIRoutes(app, admin, h)
	registerProcurementQuoteRoutes(app, h)
	registerVendorPortalRoutes(public, h)
	registerShareRoutes(app, h)
	registerNotificationRoutes(app, h)
	registerAdminNotificationTemplateRoutes(admin, h)
	registerAdminPricebookRoutes(admin, h)
	registerAddressRoutes(app, h)
	registerDevRoutes(public, h)
	registerMarketplaceRoutes(public, h)

	// ADM 代行營運（56 號）：5 支走 ADM session、26 支走 operation credential。
	registerDelegatedOperationRoutes(admin, adminNoAuth, h)

	// Webhook 模擬器（代理到 payuni-webhook-service，開發測試工具）
	registerWebhookSimulatorRoutes(r)

	// 文件檢視器（僅 local/dev 環境）
	registerDocsRoutes(r, h.Docs)

	// 能力層 deny-by-default boot 斷言（46 §3.8 賣家側 a–c、56 §8.1.2 代行側 d–g）：
	// 漏宣告／禁字／殘留 wildcard／registry 漂移／略過 requireAdmin 卻不在白名單
	// → panic fail-fast，不帶病上線（部署即炸）。
	assertRouteCoverage(r)
}

// proxyToPath 建立代理，轉發到特定的後端路徑。
// onUpstreamFailure 依這條路由的消費者類別宣告，見 proxy.UpstreamFailureStatus。
func proxyToPath(serviceName, targetPath string, onUpstreamFailure proxy.UpstreamFailureStatus) gin.HandlerFunc {
	// 在**註冊時**建好 handler，不要每個請求重建一次：CreateProxy 內的上游位址解析
	// 失敗時會 panic（見 proxy.mustParseTarget），放在請求路徑上等於把「開機就炸」
	// 退化回「每個請求 500」——那正是 #2813 要消滅的形狀。順帶省掉每請求配置一個
	// httputil.ReverseProxy 的開銷。
	h := proxy.CreateProxy(serviceName, onUpstreamFailure)
	return func(c *gin.Context) {
		// 覆寫請求路徑
		c.Request.URL.Path = targetPath
		h(c)
	}
}
