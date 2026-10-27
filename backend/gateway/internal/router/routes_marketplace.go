package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerMarketplaceRoutes 配置 Marketplace 聚合 API 路由
func registerMarketplaceRoutes(public *gin.RouterGroup, h *handler.Holder) {
	// GET /api/v1/public/marketplace — 聚合推薦商家 + 最新貼文 + 熱門商品
	// OptionalJWTAuth: 有帶 token 時解析 userID，讓 latest_posts 回傳 is_liked 狀態
	public.GET("/marketplace", middleware.OptionalJWTAuth(), h.GetMarketplace)
}
