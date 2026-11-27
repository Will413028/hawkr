package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerCartRoutes(public *gin.RouterGroup, app *gin.RouterGroup, h *handler.Holder) {
	carts := public.Group(CartsPath)
	carts.Use(middleware.OptionalJWTAuth())
	{
		carts.GET("", h.GetCartList)
		carts.POST(CartsItems, h.AddToCart)
		carts.PUT(CartsItemQty, h.SetItemQuantity)
		carts.DELETE(CartsItemBySkuID, h.DeleteCartItem)
	}

	appCarts := app.Group(CartsPath)
	appCarts.Use(middleware.JWTAuth())
	{
		appCarts.POST(CartsMerge, h.MergeCart)
	}
}
