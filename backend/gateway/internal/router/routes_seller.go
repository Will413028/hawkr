package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerSellerRoutes 配置商家相關路由
// /api/v1/public/sellers — 公開商家資訊（匿名可存取）
// /api/v1/app/sellers — 商家操作（需要 JWT）
func registerSellerRoutes(public, app *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 公開商家資訊
	// ------------------------------------------
	// 推薦商家（必須在 /:id 之前，避免被 /:id 攔截）
	public.GET(SellersRecommended, h.GetRecommendedSellers)

	pubSellers := public.Group(SellersPath)
	pubSellers.Use(middleware.OptionalJWTAuth())
	{
		pubSellers.GET("/:id", h.GetSellerInfo)
		pubSellers.GET("/:id/stats", h.GetSellerStats)
		pubSellers.GET("/:id/rating", h.GetSellerRating)
		pubSellers.GET("/:id/products", h.GetSellerProducts)
		pubSellers.GET("/:id/reviews", h.GetPublicSellerReviews)
		pubSellers.GET("/:id/reviews/stats", h.GetPublicSellerReviewStats)
	}

	// ------------------------------------------
	// 商家管理（需要 JWT + 賣家角色 + 能力層）
	// capability 粒度決定掛載粒度：本群組混 CAP_MARKETING（dashboard/報表/評論回覆）
	// 與 CAP_NOTIFICATION（通知偏好，白名單），故 per-route 掛 requireSeller(cap)。
	// ------------------------------------------
	sellerMgmt := app.Group(SellersPath)
	sellerMgmt.Use(middleware.JWTAuth())
	{
		// 訂單商品評論（賣家查看＋公開回覆＝經營）
		sellerHandle(sellerMgmt, "GET", "/me/reviews", authz.CapMarketing, h.GetSellerOrderReviews)
		sellerHandle(sellerMgmt, "GET", "/me/reviews/stats", authz.CapMarketing, h.GetSellerOrderReviewStats)
		sellerHandle(sellerMgmt, "POST", SellerReviewReply, authz.CapMarketing, h.ReplyReview)

		// 通知偏好設定（設定自己收不收通知，非經營行為 → 白名單）
		sellerHandle(sellerMgmt, "GET", "/me/notification-preferences", authz.CapNotification, h.GetNotificationPreferences)
		sellerHandle(sellerMgmt, "PUT", "/me/notification-preferences", authz.CapNotification, h.UpdateNotificationPreference)
	}
}
