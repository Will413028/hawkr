package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// registerAdminNonManagedRoutes 配置非負責微服務的管理員路由
// 注意：商品管理 / 訂單管理 / 退款審核 / 媒體管理 已遷移至各自的 register 函式
func registerAdminNonManagedRoutes(app, admin *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 管理員操作（admin 群組已含 JWTAuth + requireAdmin）
	// ------------------------------------------
	admin.GET("/posts", h.AdminListPosts)
	admin.PATCH("/posts/:id", h.AdminModeratePost)

	// §10 申訴與平台裁決（Admin） + §5.3.6 人工重試 7-11 退貨列印 + §29 異常列表
	asAdmin := admin.Group(AdminAfterSalesPath)
	{
		// §29 A1 異常列表（必須在 /:case_no 之前註冊，讓 /appeals 靜態路由先於 param 匹配）
		asAdmin.GET("", h.ListAfterSaleCasesForAdmin)
		asAdmin.GET(AdminAfterSalesAppeals, h.ListAppealsForAdmin)
		asAdmin.POST(AdminAfterSalesDecide, h.DecideAppealForAdmin)
		asAdmin.POST(AdminAfterSalesRetryPrintReturnLabel, h.RetryPrintReturnLabel)
		asAdmin.POST(AdminAfterSalesForceConfirmReceipt, h.ForceConfirmReceiptForAdmin) // §5.3.12
		asAdmin.GET(AdminAfterSalesDetail, h.GetAppealDetailForAdmin)                   // §10.6.6
	}
}

// requireAdmin 檢查使用者是否為管理員（accountType code=4）
// 測試環境由 SKIP_ROLE_CHECK 環境變數控制跳過檢查
func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.SkipRoleCheck {
			c.Next()
			return
		}

		accountType, _ := c.Get("accountType")
		code, _ := accountType.(int)
		if !middleware.IsAdmin(code) {
			response.Forbidden(c, "admin access required")
			c.Abort()
			return
		}

		c.Next()
	}
}
