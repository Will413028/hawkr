package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerInfraRoutes(r *gin.Engine, h *handler.Holder) {
	r.GET("/.well-known/apple-developer-merchantid-domain-association.txt", h.AppleDomainVerification)
}

func registerHealthRoutes(r gin.IRouter, h *handler.Holder) {
	r.GET("/health", h.HealthSelf)
}
