package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerSearchRoutes 配置搜尋相關路由
// /api/v1/public/search — 匿名可存取（OptionalJWT）
func registerSearchRoutes(public *gin.RouterGroup, h *handler.Holder) {
	search := public.Group(SearchPath)
	search.Use(middleware.OptionalJWTAuth())
	{
		// 商品搜尋（product-service.GetPublicProductList）
		search.GET(SearchProducts, h.SearchProductsViaES)
		search.GET(SearchSuggest, h.GetSearchSuggestions)
	}
}
