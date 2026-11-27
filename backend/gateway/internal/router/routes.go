package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// SetupRoutes 配置所有 API 路由
// API 設計：RESTful 風格，三大 Namespace: /public, /app, /admin
func SetupRoutes(r *gin.Engine, h *handler.Holder) {
	registerInfraRoutes(r, h)

	v1 := r.Group(APIV1Path)
	registerHealthRoutes(v1, h)

	public := v1.Group(PublicPath)
	app := v1.Group(AppPath)
	admin := v1.Group(AdminPath)
	admin.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), requireAdmin())

	adminNoAuth := v1.Group(AdminPath)

	admin.Use(middleware.StripDelegationReservedHeaders())
	adminNoAuth.Use(middleware.StripDelegationReservedHeaders())

	registerProductRoutes(public, app, admin, h)
	registerOrderRoutes(app, admin, h)
	registerCheckoutRoutes(app, h)
	registerCartRoutes(public, app, h)
	registerShippingRoutes(app, admin, h)

	registerStoreMapReturnRoutes(r, h)

	registerInvoiceRoutes(app, admin, h)
	registerSearchRoutes(public, h)
	registerSellerRoutes(public, app, h)
	registerBuyerRoutes(app, h)
	registerCouponRoutes(app, admin, h)
	registerAfterSalesRoutes(app, r, h)
	registerAdminNonManagedRoutes(app, admin, h)
	registerShareRoutes(app, h)
	registerDevRoutes(public, h)

	registerDelegatedOperationRoutes(admin, adminNoAuth, h)

	registerDocsRoutes(r, h.Docs)

	assertRouteCoverage(r)
}
