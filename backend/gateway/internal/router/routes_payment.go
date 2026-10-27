package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

// registerPaymentRoutes 配置支付相關路由
// /api/v1/app/payments — 需要 JWT 認證
// /api/v1/admin/payment_transactions|payment_refunds|grants — Admin 異常查詢（§29）
func registerPaymentRoutes(app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {
	payments := app.Group(PaymentsPath)
	payments.Use(middleware.JWTAuth())
	{
		// 建立支付（RESTful：POST /payments）
		payments.POST("", h.CreatePayment)
		// 查詢支付詳情
		payments.GET(PaymentsTradeNo, h.GetPayment)
		// §2A.3.6 / §2A.6 查詢付款歷程（含 CLOSED_BY_SWITCH / 重複付款副 row）
		payments.GET(PaymentsTradeHistory, h.GetPaymentHistory)
		// 同步支付狀態（呼叫 PayUni Query API）
		payments.POST(PaymentsStatusSyncs, h.SyncPaymentStatus)
		// 關閉交易
		payments.PATCH(PaymentsTradeNo, h.ClosePayment)
		// 退款查詢（買家無法直接建立退款——退款一律走售後申請 → aftersales internal call）
		payments.GET(PaymentsRefundNo, h.GetRefund)
		payments.GET(PaymentsTradeRefunds, h.GetRefundsByTradeNo)
		// Apple Pay on Web merchant validation（05 號設計書 §5.11）
		payments.POST(PaymentsApplePayValidateMerchant, h.ValidateApplePayMerchant)
	}

	// Admin 異常查詢（§29 Admin 異常訂單管理）
	// admin group 已含 JWTAuth + requireAdmin
	admin.GET(AdminPaymentTransactionsPath, h.ListPaymentTransactionsForAdmin)
	admin.GET(AdminPaymentTransactionDetail, h.GetPaymentTransactionForAdmin)
	admin.GET(AdminPaymentRefundsPath, h.ListRefundsForAdmin)
	admin.GET(AdminPaymentRefundDetail, h.GetRefundForAdmin)
	admin.GET(AdminGrantsPath, h.ListGrantsForAdmin)
}

// registerPayUniWebhookRoute 配置 PayUni Webhook 路由
// POST /webhook/payuni/notify — 外部 PayUni 服務回調，無需 JWT
// 代理至 payuni-webhook-service（統一解密 + 分發至各微服務 MQ）
// 路徑改寫：/webhook/payuni/notify → /webhooks/payuni/notify（webhook-service 期望的路徑）
func registerPayUniWebhookRoute(r *gin.Engine) {
	// ⚠️ FailThirdPartyWebhook（502）是**唯一一條刻意維持 5xx** 的代理路由，不要跟著
	// 其他路由改成 424。消費者是 PayUni 而不是我們的前端：5xx 對它是「等一下重送」，
	// 4xx 是「別再送了」，而 10 號設計書 §4.5.3／§4.5.5.D 的整套失敗回收（T2 失敗
	// → replay 路徑）就是靠重送觸發的。完整理由見 proxy.UpstreamFailureStatus。
	r.POST(WebhookPayUniNotify, proxy.CreateProxyWithPathRewrite(
		ServicePayuniWebhook, "/webhook/payuni/notify", "/webhooks/payuni/notify",
		proxy.FailThirdPartyWebhook))
}

// registerPaymentReturnRoute 配置付款結果回調頁面路由
// GET /payment/return?tradeNo={tradeNo} — 手動查詢付款狀態（備用）
// POST /payment/return — PayUni UPP 付款完成導回，Gateway 提取加密參數 → Payment gRPC 解密 + 更新 DB → 302 redirect Deep Link
func registerPaymentReturnRoute(r *gin.Engine, h *handler.Holder) {
	r.GET(PaymentReturnPath, h.PaymentReturn)
	r.POST(PaymentReturnPath, h.PaymentReturnPost)
}

// registerPaymentRedirectRoute 配置 PAYUNi 付款頁重導向路由
// GET /payment/redirect?id={opaqueId} — 供 APP WebView 開啟，無需 JWT
// #1394：以 opaque id 向 payment-service 取回 canonical 表單 → html/template 安全 render
func registerPaymentRedirectRoute(r *gin.Engine, h *handler.Holder) {
	r.GET(PaymentRedirectPath, h.RedirectToPayUni)
}
