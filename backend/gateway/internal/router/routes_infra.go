package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

// registerInfraRoutes 配置基礎設施路由。
//
// 目前只有 Apple Pay 網域驗證檔。曾記載的「Swagger、DB 初始化」兩者都不在這裡：
// Gateway 沒有註冊任何 Swagger UI 路由（生成物只餵 Apidog，10 檔 §16），
// 文件檢視器 /docs/* 在 registerDocsRoutes（僅 local/dev）。
//
// Prometheus /metrics 不掛在這裡：它跑在獨立 internal listener（見
// infrastructure/di.go 的 startMetrics），不經公開 Gateway Ingress。
func registerInfraRoutes(r *gin.Engine, h *handler.Holder) {
	// ==========================================
	// Apple Pay 網域驗證
	// ==========================================
	r.GET("/.well-known/apple-developer-merchantid-domain-association.txt", h.AppleDomainVerification)
}

// registerHealthRoutes 配置健康檢查路由
func registerHealthRoutes(r gin.IRouter, h *handler.Holder) {
	r.GET("/health", h.HealthSelf)
}
