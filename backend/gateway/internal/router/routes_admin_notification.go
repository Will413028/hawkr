package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerAdminNotificationTemplateRoutes(admin *gin.RouterGroup, h *handler.Holder) {
	tmpl := admin.Group("/notification-templates")
	tmpl.POST("/reload", h.AdminReloadTemplates)
	tmpl.GET("", h.AdminListTemplates)
	tmpl.GET("/:id", h.AdminGetTemplate)
	// POST "" / PUT "/:id" 已移除（issue #1176 SSOT 政策）：template 變更走 Atlas migration
	tmpl.PATCH("/:id/enabled", h.AdminUpdateTemplateEnabled)
	tmpl.POST("/:id/preview", h.AdminPreviewTemplate)
	tmpl.POST("/:id/test-send", h.AdminTestSendTemplate)

	// System announcement broadcast
	admin.POST("/notifications/broadcast", h.AdminBroadcastAnnouncement)
}
