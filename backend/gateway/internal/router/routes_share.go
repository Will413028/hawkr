package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerShareRoutes 配置分享連結路由
// GET /api/v1/app/share_url?type={post|product|seller}&id={id}
func registerShareRoutes(app *gin.RouterGroup, h *handler.Holder) {
	app.GET(ShareURLPath, middleware.JWTAuth(), h.GetShareURL)
}
