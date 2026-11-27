package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

func registerAdminNonManagedRoutes(app, admin *gin.RouterGroup, h *handler.Holder) {
	asAdmin := admin.Group(AdminAfterSalesPath)
	{
		asAdmin.GET("", h.ListAfterSaleCasesForAdmin)
		asAdmin.GET(AdminAfterSalesAppeals, h.ListAppealsForAdmin)
		asAdmin.POST(AdminAfterSalesDecide, h.DecideAppealForAdmin)
		asAdmin.POST(AdminAfterSalesRetryPrintReturnLabel, h.RetryPrintReturnLabel)
		asAdmin.POST(AdminAfterSalesForceConfirmReceipt, h.ForceConfirmReceiptForAdmin)
		asAdmin.GET(AdminAfterSalesDetail, h.GetAppealDetailForAdmin)
	}
}

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
