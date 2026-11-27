package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerSearchRoutes(public *gin.RouterGroup, h *handler.Holder) {
	search := public.Group(SearchPath)
	search.Use(middleware.OptionalJWTAuth())
	{
		search.GET(SearchProducts, h.SearchProductsViaES)
		search.GET(SearchSuggest, h.GetSearchSuggestions)
	}
}
