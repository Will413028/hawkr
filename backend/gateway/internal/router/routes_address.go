package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerAddressRoutes 地址搜尋與驗證（proxy 至外部地址服務）
func registerAddressRoutes(app *gin.RouterGroup, h *handler.Holder) {
	address := app.Group(AddressPath)
	address.Use(middleware.JWTAuth())
	{
		address.GET(AddressSuggestions, h.AddressSuggestions)
		address.GET(AddressValidate, h.AddressValidate)
	}
}
