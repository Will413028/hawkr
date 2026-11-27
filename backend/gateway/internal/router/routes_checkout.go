package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerCheckoutRoutes(app *gin.RouterGroup, h *handler.Holder) {
	checkout := app.Group(CheckoutPath)
	checkout.Use(middleware.JWTAuth())
	{
		checkout.POST(CheckoutValidate, h.CheckoutValidate)
		checkout.POST(CheckoutShippingFee, h.CheckoutShippingFee)
		checkout.POST(CheckoutPreview, h.CheckoutPreview)
		checkout.POST(CheckoutConfirm, h.CheckoutConfirm)
	}
}
