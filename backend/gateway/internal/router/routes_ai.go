package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerAIRoutes configures AI-related routes.
func registerAIRoutes(app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {
	// AI 風格列表（App）——回應帶 weeklyUsage／weeklyLimit，是由 JWT 解出的 storeID
	// 查出來的額度，屬個人化回應；且是 GET，預設可被快取，所以逐條掛 PrivateNoStore。
	app.GET(AIStylesPath, middleware.PrivateNoStore(), middleware.JWTAuth(), h.ProxyListAiStyles)

	// AI 圖片轉換（App）——刻意不掛：POST 回應沒有明確 freshness 標頭時本來就不會被
	// 共享快取存下（RFC 9111 §4.4），加了只是噪音。
	app.POST("/image_transformations", middleware.JWTAuth(), h.ProxyTransformImage)

	// Admin — AI 風格管理。以下兩組 admin 端點刻意不掛 PrivateNoStore：它們是後台
	// 管理資料而非使用者個人化回應，且 admin namespace 的快取政策應該整組一次決定，
	// 不在這張安全票裡零星處理（#2447 BE-01A 的範圍是 AI 助手與採購）。
	adminStyles := admin.Group(AIStylesPath)
	{
		adminStyles.GET("", h.ProxyListAiStylesAdmin)
		adminStyles.POST("", h.ProxyCreateAiStyle)
		adminStyles.PUT("/:id", h.ProxyUpdateAiStyle)
		adminStyles.DELETE("/:id", h.ProxyDeleteAiStyle)
	}

	// Admin — 帳號額度管理
	adminQuotas := admin.Group(AIQuotasPath)
	{
		adminQuotas.GET("/:store_id", h.ProxyGetAiQuota)
		adminQuotas.PUT("/:store_id", h.ProxyUpdateAiQuota)
		adminQuotas.DELETE("/:store_id", h.ProxyDeleteAiQuota)
	}

	// 以下三組（賣家 AI 助手／買家 AI 助手／Hawkr 幫找採購）的回應一律 private, no-store：
	// 內容逐一綁定登入者身分，且 PrivateNoStore 刻意排在 JWTAuth 之前，讓 401／403 這種
	// 早退回應也帶到 header（#2447 BE-01A，判準見 API 設計規範「回應快取政策」）。

	// 賣家 AI 助手：整組 CAP_MARKETING（AI 助理即使唯讀也鎖，2026-07-10 裁決）。
	sellerAI := app.Group("/sellers/me/assistant")
	sellerAI.Use(middleware.PrivateNoStore(), middleware.JWTAuth())
	{
		sellerHandle(sellerAI, "POST", "/chat", authz.CapMarketing, h.ProxyAIAssistantSSE)
		sellerHandle(sellerAI, "POST", "/orders", authz.CapMarketing, h.ProxySellerOrdersSSE)
		sellerHandle(sellerAI, "POST", "/inventory", authz.CapMarketing, h.ProxySellerInventorySSE)
		sellerHandle(sellerAI, "POST", "/payment", authz.CapMarketing, h.ProxySellerPaymentSSE)
		sellerHandle(sellerAI, "POST", "/price", authz.CapMarketing, h.ProxySellerPriceSSE)
		sellerHandle(sellerAI, "POST", "/product-autofill", authz.CapMarketing, h.ProxySellerProductAutofill)
		sellerHandle(sellerAI, "POST", "/product-listing/chat", authz.CapMarketing, h.ProxySellerProductListingSSE)
		sellerHandle(sellerAI, "GET", "/sessions/latest", authz.CapMarketing, h.ProxySellerLatestSession)
	}

	// 買家 AI 助手
	buyerAI := app.Group("/buyers/me/assistant")
	buyerAI.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), requireBuyer())
	{
		buyerAI.POST("/orders", h.ProxyBuyerOrdersSSE)
		buyerAI.POST("/tracking", h.ProxyBuyerTrackingSSE)
		buyerAI.POST("/return-policy", h.ProxyBuyerReturnPolicySSE)
		buyerAI.POST("/seasonal", h.ProxyBuyerSeasonalSSE)
		buyerAI.POST("/seller-match", h.ProxyBuyerSellerMatchSSE)
		buyerAI.POST("/chat", h.ProxyBuyerAssistantSSE)
		buyerAI.GET("/sessions/latest", h.ProxyBuyerLatestSession)
	}

	// Hawkr 幫找 — 採購需求（turns 有 JSON 與 SSE 兩版；其餘端點是 JSON）
	procurement := app.Group("/buyers/me/procurement")
	procurement.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), requireBuyer())
	{
		procurement.POST("/turns", h.ProxyProcurementTurn)
		procurement.POST("/turns/stream", h.ProxyProcurementTurnStream)
		procurement.GET("/requests/:id/snapshot", h.ProxyProcurementSnapshot)
		procurement.POST("/requests/:id/candidate-searches", h.ProxyProcurementCandidateSearch)
		procurement.GET("/requests/:id/candidate-searches/:runId", h.ProxyProcurementCandidateSearchGet)
		procurement.POST("/requests/:id/selections", h.ProxyProcurementSelection)
		procurement.POST("/requests/:id/answers", h.ProxyProcurementAnswer)
		procurement.POST("/requests/:id/rewinds", h.ProxyProcurementRewind)
		procurement.POST("/requests/:id/rfq-price-previews", h.ProxyProcurementRfqPricePreview)
		procurement.POST("/requests/:id/rfq-confirmations", h.ProxyProcurementRfqConfirmation)
		procurement.POST("/requests/:id/rfq-publications/preview", h.ProxyProcurementRfqPublicationPreview)
		procurement.POST("/requests/:id/rfq-publications", h.ProxyProcurementRfqPublicationExecute)
		procurement.GET("/conversations", h.ProxyProcurementConversations)
		procurement.GET("/conversations/:id/messages", h.ProxyProcurementConversationMessages)
		// #2538 買家面：比較清單、shortlist（冪等、零 side effect，不需冪等鍵）、
		// 接受 exact version（Idempotency-Key header）。
		procurement.GET("/requests/:id/seller-responses", h.ProxyBuyerProcurementSellerResponses)
		procurement.PUT("/requests/:id/shortlist/:seller_response_id/versions/:seller_response_version", h.ProxyBuyerProcurementShortlistPut)
		procurement.DELETE("/requests/:id/shortlist/:seller_response_id/versions/:seller_response_version", h.ProxyBuyerProcurementShortlistDelete)
		procurement.POST("/requests/:id/buyer-quote-acceptances", h.ProxyBuyerProcurementQuoteAcceptance)
		procurement.GET("/conversations/:id/runtime-snapshot", h.ProxyProcurementConversationRuntimeSnapshot)
		procurement.PATCH("/conversations/:id", h.ProxyProcurementConversationRename)
		procurement.DELETE("/conversations/:id", h.ProxyProcurementConversationDelete)
		// Run ledger 事件重播／即時訂閱（Task 12）。刻意不掛在 /requests/:id 底下：
		// run 本身跨越多個 request（SubmitAnswer／Rewind 也會產生 TURN run），
		// 跟 /conversations 一樣是 procurement group 的頂層路由。
		procurement.GET("/runs/:runId/events", h.ProxyProcurementRunEvents)
		// Run 取消／恢復（Task 14，doc57 §5.3/§5.4/§5.5）。同一組頂層 /runs/:runId
		// 路由，理由同上——取消／恢復都不是對某個 request 的操作。
		procurement.POST("/runs/:runId/cancel", h.ProxyProcurementRunCancel)
		procurement.POST("/runs/:runId/resume", h.ProxyProcurementRunResume)
	}
}
