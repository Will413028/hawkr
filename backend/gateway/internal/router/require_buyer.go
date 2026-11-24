package router

import (
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// requireBuyer 檢查使用者是否涵蓋買家權限（accountType code=1 或 3）
// 測試環境由 SKIP_ROLE_CHECK 環境變數控制跳過檢查
func requireBuyer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.SkipRoleCheck {
			c.Next()
			return
		}
		accountType, _ := c.Get("accountType")
		code, _ := accountType.(int)
		if !middleware.IsBuyer(code) {
			response.Forbidden(c, "buyer access required")
			c.Abort()
			return
		}
		c.Next()
	}
}
