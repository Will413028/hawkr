package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerOrderRoutes 配置訂單相關路由
// /api/v1/app/orders — 買家 + 賣家訂單（透過 ?view=selling 切換視角）
// /api/v1/admin/orders — 管理員訂單
func registerOrderRoutes(app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 買家 + 賣家訂單（/app/orders）
	// 買家：GET /orders, POST /orders, GET /orders/:order_no
	// 賣家：GET /orders?view=selling, POST /orders/:order_no/ship
	// ------------------------------------------
	orders := app.Group(OrdersPath)
	orders.Use(middleware.JWTAuth())
	{
		// 查詢訂單列表（買家 / 賣家視角由 ?view=selling 區分）
		// ?view=selling → 賣家面走 CAP_ORDER_FULFILL 能力檢查；其他 view → 買家面放行。
		orders.GET("", sellerViewGuard(authz.CapOrderFulfill), h.GetMyOrders)
		// 買家訂單各狀態筆數 rollup（03 §2.1 SSOT）— static 路徑必須先於 :order_no 註冊；
		// 賣家視角（view=seller/selling）由 handler 回 400 拒絕，故不掛 sellerViewGuard。
		orders.GET(OrderStatusCounts, h.GetMyOrderStatusCounts)
		// 建立訂單（買家）
		orders.POST("", h.CreateOrder)
		// 查詢訂單詳情（買家 / 賣家由 ?view=seller 區分）
		orders.GET(OrderNo, sellerViewGuard(authz.CapOrderFulfill), h.GetOrder)
		// 取消訂單（買家）
		orders.POST(OrderCancellations, h.CancelOrder)
		// 確認收貨（買家）
		orders.POST(OrderConfirmReceipt, h.ConfirmReceipt)
		// 查看訂單物流追蹤（買家）— 即時同步 PayUni，呼叫 shipping-service（見 06 §5.7.1）
		orders.GET(OrderLogistics, h.GetOrderLogistics)
		// 出貨（賣家）已由 Shipping Service 提供，見 routes_shipping.go
	}

	// ------------------------------------------
	// 管理員訂單（/admin/orders）
	// ------------------------------------------
	admin.GET(OrdersPath, h.SearchOrders)
	// §29 O1 異常列表（static 必須先於 :order_no 註冊）
	admin.GET(OrdersPath+"/abnormal", h.ListAbnormalOrdersForAdmin)
	admin.GET(OrdersPath+OrderNo, h.GetAdminOrderDetail)
	// 收斂規範 §8.4 撥款重發（人工介入閉環）
	admin.POST(OrdersPath+OrderRetryGrant, h.RetryGrant)
}
