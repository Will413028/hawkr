package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerBuyerRoutes retains paths owned by other modules while the buyer
// profile, address, favorite, follow, visit, and restock clients use Connect.
func registerBuyerRoutes(public, app *gin.RouterGroup, h *handler.Holder) {
	public.GET(RecommendationsPath, h.GetRecommendations)
	buyers := app.Group("/buyers/me")
	buyers.Use(middleware.JWTAuth())
	buyers.GET("/notification-preferences", h.GetBuyerNotificationPreferences)
	buyers.PUT("/notification-preferences", h.UpdateBuyerNotificationPreference)
}
