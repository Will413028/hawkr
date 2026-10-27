package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerReviewRoutes 配置商品評論相關路由
// pubProducts: /api/v1/public/products group（來自 registerProductRoutes）
func registerReviewRoutes(pubProducts *gin.RouterGroup, app *gin.RouterGroup, h *handler.Holder) {
	// --------------------------------------------------
	// Public — 匿名可讀
	// GET /api/v1/public/products/:id/reviews
	// --------------------------------------------------
	pubProducts.GET(ProductsReviews, middleware.OptionalJWTAuth(), h.GetProductReviews)

	// --------------------------------------------------
	// App — 評論（需要 JWT）
	// POST /api/v1/app/reviews       買家提交評論
	// GET  /api/v1/app/reviews/:id   取得單一評論
	// --------------------------------------------------
	appReviews := app.Group(ReviewsPath)
	appReviews.Use(middleware.JWTAuth())
	{
		appReviews.POST("", h.SubmitOrderItemReview)
		appReviews.GET("/:id", h.GetReview)
	}

	// --------------------------------------------------
	// App — 我的評論（需要 JWT）
	// GET /api/v1/app/me/reviews   買家評論歷史
	// --------------------------------------------------
	appMe := app.Group("/me")
	appMe.Use(middleware.JWTAuth())
	{
		appMe.GET("/reviews", h.ListMyOrderReviews)
	}
}
