package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerOrderRoutes(app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {
	orders := app.Group(OrdersPath)
	orders.Use(middleware.JWTAuth())
	{
		orders.GET("", sellerViewGuard(authz.CapOrderFulfill), h.GetMyOrders)
		orders.GET(OrderStatusCounts, h.GetMyOrderStatusCounts)
		orders.POST("", h.CreateOrder)
		orders.GET(OrderNo, sellerViewGuard(authz.CapOrderFulfill), h.GetOrder)
		orders.POST(OrderCancellations, h.CancelOrder)
		orders.POST(OrderConfirmReceipt, h.ConfirmReceipt)
		orders.GET(OrderLogistics, h.GetOrderLogistics)
	}

	admin.GET(OrdersPath, h.SearchOrders)
	admin.GET(OrdersPath+"/abnormal", h.ListAbnormalOrdersForAdmin)
	admin.GET(OrdersPath+OrderNo, h.GetAdminOrderDetail)
	admin.POST(OrdersPath+OrderRetryGrant, h.RetryGrant)
}
