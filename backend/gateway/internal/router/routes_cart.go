package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerCartRoutes 配置購物車相關路由
// /api/v1/public/carts — 匿名可存取（OptionalJWT）
// /api/v1/app/carts — 需要 JWT（合併購物車）
func registerCartRoutes(public *gin.RouterGroup, app *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 匿名可存取（登入 / 訪客皆可）
	// ------------------------------------------
	carts := public.Group(CartsPath)
	carts.Use(middleware.OptionalJWTAuth())
	{
		// 查詢購物車列表
		carts.GET("", h.GetCartList)
		// 加入購物車
		carts.POST(CartsItems, h.AddToCart)
		// 設定商品數量（0 = 刪除）
		carts.PUT(CartsItemQty, h.SetItemQuantity)
		// 刪除購物車商品
		carts.DELETE(CartsItemBySkuID, h.DeleteCartItem)
	}

	// ------------------------------------------
	// 合併購物車（登入後，必須驗證身分）
	// ------------------------------------------
	appCarts := app.Group(CartsPath)
	appCarts.Use(middleware.JWTAuth())
	{
		appCarts.POST(CartsMerge, h.MergeCart)
	}
}
