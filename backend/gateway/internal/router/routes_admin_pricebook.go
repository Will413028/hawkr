package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerAdminPricebookRoutes(admin *gin.RouterGroup, h *handler.Holder) {
	m := admin.Group("/pricebook/category-mapping")
	m.POST("", h.UpsertCategoryMapping)
	m.GET("", h.ListCategoryMappingWorklist)
}
