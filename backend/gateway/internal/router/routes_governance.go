package router

import (
	"time"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerGovernanceRoutes — 商家治理 / 食安通報 / 申訴流程（B-169 / issue #722）
//
// 兩個 namespace（後台的通報與案件走單體的 Connect，不在這裡）：
//
//	/public/food_safety_reports                                POST   公開無登入食安通報（雙層 IP rate limit）
//	/public/sellers/:id/violations/public                      GET    公開賣家頁查違規（D8 cache）
//
//	/app/violation_reports                                     POST   登入端買家/賣家檢舉
//	/app/violation_reports/me                                  GET    我提交的檢舉
func registerGovernanceRoutes(public, app *gin.RouterGroup, h *handler.Holder) {
	// ----- Public（無 JWT；雙層 rate limit，B-169 / B2） -----
	//
	// 全域 tiered limiter 答的是「這個來源有沒有在打 gateway」，對匿名寫入端點是錯的
	// 問題：它的 moderate 額度用於瀏覽很合理，用於「建立一筆會通知賣家、進 FDA 流程的
	// 通報」則等同不設防。所以這裡疊一層路由專屬的窄額度（#2295 ①）。
	publicFoodSafety := public.Group("/food_safety_reports")
	publicFoodSafety.Use(middleware.DualWindowRateLimit(
		"food_safety_report",
		5, 10*time.Minute, // burst：連續灌單
		10, time.Hour, // sustained：整小時的總量
	))
	{
		publicFoodSafety.POST("", h.SubmitFoodSafetyReport)
	}
	publicSellers := public.Group("/sellers")
	{
		// 用 :id 對齊 routes_post.go / constants.go SellersIDPath；
		// 同 /api/v1/public/sellers 前綴下 gin tree 不允許 wildcard 名稱不同
		// （:sellerId vs :id 會 panic at startup — 見 dev #25507857877）。
		publicSellers.GET("/:id/violations/public", h.GetPublicSellerViolations)
	}

	// ----- App（buyer/seller JWT auth） -----
	appReports := app.Group("/violation_reports")
	appReports.Use(middleware.JWTAuth())
	{
		appReports.POST("", h.SubmitViolationReport)
		appReports.GET("/me", h.ListMyViolationReports)
	}
}
