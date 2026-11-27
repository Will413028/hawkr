package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerCouponRoutes(app, admin *gin.RouterGroup, h *handler.Holder) {
	appCoupons := app.Group(CouponsPath)
	appCoupons.Use(middleware.JWTAuth())
	{
		appCoupons.GET("", sellerViewGuard(authz.CapOrderFulfill), h.GetCoupons)
		appCoupons.GET(CouponsAvailable, h.ListAvailableCoupons)
		appCoupons.POST(CouponsClaim, h.ClaimCoupon)
	}

	appTemplates := app.Group(CouponTemplatesPath)
	appTemplates.Use(middleware.JWTAuth())
	{
		sellerHandle(appTemplates, "POST", "", authz.CapMarketing, h.CreateCouponTemplate)
		sellerHandle(appTemplates, "GET", "", authz.CapMarketing, h.ListCouponTemplates)
		sellerHandle(appTemplates, "GET", CouponTemplateID, authz.CapMarketing, h.GetCouponTemplateDetail)
		appTemplates.POST(CouponTemplateRevoke, h.RevokeCouponTemplate)
	}

	adminTemplates := admin.Group(CouponTemplatesPath)
	{
		adminTemplates.POST("", h.AdminCreateCouponTemplate)
		adminTemplates.POST(AdminCouponTemplatePublish, h.AdminPublishCouponTemplate)
		adminTemplates.POST(AdminCouponTemplateDisable, h.AdminDisableCouponTemplate)
		adminTemplates.POST(AdminCouponTemplateIssue, h.AdminIssueCouponsHandler)
		adminTemplates.POST(AdminCouponTemplateDistribute, h.AdminDistributeTemplate)
	}

	adminCoupons := admin.Group(CouponsPath)
	{
		adminCoupons.POST(AdminCouponCompensations, h.AdminGrantCompensation)
		adminCoupons.POST(AdminCouponVoid, h.AdminVoidCouponInstance)
	}

	adminQuotas := admin.Group(CouponQuotasPath)
	{
		adminQuotas.PUT(AdminCouponQuotaSeller, h.AdminUpdateCouponQuota)
	}
}
