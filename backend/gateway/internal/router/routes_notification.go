package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerNotificationRoutes(app *gin.RouterGroup, h *handler.Holder) {
	notif := app.Group("/notifications")
	notif.Use(middleware.JWTAuth())

	// Device token management
	notif.POST("/device-token", h.RegisterDeviceToken)
	notif.PUT("/device-token", h.RefreshDeviceToken)
	notif.DELETE("/device-token", h.RemoveDeviceToken)

	// Inbox
	notif.GET("", h.ListNotifications)
	notif.GET("/unread-count", h.GetUnreadCount)
	notif.PATCH("/:id/read", h.MarkNotificationAsRead)
	notif.PATCH("/read-all", h.MarkAllNotificationsAsRead)
	notif.DELETE("", h.DeleteNotifications)

	// 41 號設計書：通知 SSE 流（per-user，僅 web 訂閱）
	notif.GET("/stream", h.StreamNotifications)
}
