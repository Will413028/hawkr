package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerCouponRoutes 配置優惠券相關路由
// /api/v1/app/coupons — 買家：我的券、可領取、領取
// /api/v1/app/coupon_templates — 賣家：建立/查詢/詳情/收回模板
// /api/v1/admin/coupon_templates — 管理員：建立/上架/停用/發行模板
// /api/v1/admin/coupons — 管理員：補償券、作廢
// /api/v1/admin/coupon_quotas — 管理員：配額
func registerCouponRoutes(app, admin *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// App — Buyer（JWT 必填）
	// ------------------------------------------
	appCoupons := app.Group(CouponsPath)
	appCoupons.Use(middleware.JWTAuth())
	{
		// 取得優惠券列表（view 參數分流：buyer/seller）
		// ?view=seller → 賣家面走 CAP_ORDER_FULFILL 能力檢查（授權判斷移出 handler，
		// 46 §3.3 rule 2）；其他 view → 買家面放行。
		appCoupons.GET("", sellerViewGuard(authz.CapOrderFulfill), h.GetCoupons)
		appCoupons.GET(CouponsAvailable, h.ListAvailableCoupons)
		// 買家領取優惠券
		appCoupons.POST(CouponsClaim, h.ClaimCoupon)
	}

	// ------------------------------------------
	// App — Seller（JWT 必填）
	// 掛回 requireSeller：這三條原本因 accountType lag 走方案 b，與 /me 寫入同因，
	// 該 lag 已隨 roles_epoch ＋「申請即授予」消除（見 routes_seller.go）。
	// 建立/列表/詳情=CAP_MARKETING（06 §F BLOCK；唯讀也鎖）；
	// Revoke=CAP_CONTENT_CONTRACT 收縮型白名單，不 deny 故不掛。
	// ------------------------------------------
	appTemplates := app.Group(CouponTemplatesPath)
	appTemplates.Use(middleware.JWTAuth())
	{
		// 建立並發行優惠券模板
		sellerHandle(appTemplates, "POST", "", authz.CapMarketing, h.CreateCouponTemplate)
		// 查詢賣家模板列表
		sellerHandle(appTemplates, "GET", "", authz.CapMarketing, h.ListCouponTemplates)
		// 查詢模板詳情
		sellerHandle(appTemplates, "GET", CouponTemplateID, authz.CapMarketing, h.GetCouponTemplateDetail)
		// 收回模板（收縮型，停權仍可自清）
		appTemplates.POST(CouponTemplateRevoke, h.RevokeCouponTemplate)
	}

	// ------------------------------------------
	// Admin — Template（requireAdmin 已在 admin group 層級套用）
	// ------------------------------------------
	adminTemplates := admin.Group(CouponTemplatesPath)
	{
		// 建立模板（草稿）
		adminTemplates.POST("", h.AdminCreateCouponTemplate)
		// 上架模板
		adminTemplates.POST(AdminCouponTemplatePublish, h.AdminPublishCouponTemplate)
		// 停用模板
		adminTemplates.POST(AdminCouponTemplateDisable, h.AdminDisableCouponTemplate)
		// 發行優惠券
		adminTemplates.POST(AdminCouponTemplateIssue, h.AdminIssueCouponsHandler)
		// 代領模板
		adminTemplates.POST(AdminCouponTemplateDistribute, h.AdminDistributeTemplate)
	}

	// ------------------------------------------
	// Admin — Instance（補償券、作廢）
	// ------------------------------------------
	adminCoupons := admin.Group(CouponsPath)
	{
		// 發放補償券
		adminCoupons.POST(AdminCouponCompensations, h.AdminGrantCompensation)
		// 作廢優惠券
		adminCoupons.POST(AdminCouponVoid, h.AdminVoidCouponInstance)
	}

	// ------------------------------------------
	// Admin — Quota
	// ------------------------------------------
	adminQuotas := admin.Group(CouponQuotasPath)
	{
		// 更新賣家配額
		adminQuotas.PUT(AdminCouponQuotaSeller, h.AdminUpdateCouponQuota)
	}
}
