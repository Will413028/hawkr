package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerModerationRoutes — 內容檢舉與使用者封鎖（B-135 / Apple Guideline 1.2 / Issue #645）
//
// /app/reports          POST   buyer/seller 提交檢舉
// /app/reports/me       GET    查我送出的檢舉
// /app/blocks/:userId   POST   封鎖使用者
// /app/blocks/:userId   DELETE 解封
// /app/blocks           GET    列我的封鎖名單
// /admin/reports        GET    admin 列 reports
// /admin/reports/:id    GET    admin 看單筆 report
// /admin/reports/:id/resolve POST admin 處理 report
// /admin/reports/:id/reopen POST admin 重開已處理的檢舉（申訴）
func registerModerationRoutes(app, admin *gin.RouterGroup, h *handler.Holder) {
	appReports := app.Group("/reports")
	appReports.Use(middleware.JWTAuth())
	{
		appReports.POST("", h.SubmitReport)
		appReports.GET("/me", h.ListMyReports)
	}

	appBlocks := app.Group("/blocks")
	appBlocks.Use(middleware.JWTAuth())
	{
		appBlocks.POST("/:userId", h.BlockUser)
		appBlocks.DELETE("/:userId", h.UnblockUser)
		appBlocks.GET("", h.ListBlocks)
	}

	adminReports := admin.Group("/reports")
	{
		adminReports.GET("", h.AdminListReports)
		adminReports.GET("/:id", h.AdminGetReport)
		adminReports.POST("/:id/resolve", h.AdminResolveReport)
		adminReports.POST("/:id/reopen", h.AdminReopenReport)
	}
}
