package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerShareRoutes(app *gin.RouterGroup, h *handler.Holder) {
	app.GET(ShareURLPath, middleware.JWTAuth(), h.GetShareURL)
}
