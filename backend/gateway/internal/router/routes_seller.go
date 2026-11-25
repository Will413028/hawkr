package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerSellerRoutes 配置商家相關路由
// /api/v1/public/sellers — 公開商家資訊（匿名可存取）
// /api/v1/app/sellers — 商家操作（需要 JWT）
func registerSellerRoutes(public, app *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 公開商家資訊
	// ------------------------------------------

	pubSellers := public.Group(SellersPath)
	pubSellers.Use(middleware.OptionalJWTAuth())
	{
		pubSellers.GET("/:id/products", h.GetSellerProducts)
	}
}
