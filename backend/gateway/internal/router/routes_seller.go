package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerSellerRoutes(public, app *gin.RouterGroup, h *handler.Holder) {

	pubSellers := public.Group(SellersPath)
	pubSellers.Use(middleware.OptionalJWTAuth())
	{
		pubSellers.GET("/:id/products", h.GetSellerProducts)
	}
}
