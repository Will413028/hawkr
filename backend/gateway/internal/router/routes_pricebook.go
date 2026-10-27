package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerPricebookRoutes 配置賣家市場行情查詢路由（P6+P7+#2786 categoryId 解析）。
// 2026-08-02 原裁決是唯讀市場資訊、非賣家專屬資料，只掛 JWTAuth 不掛
// capability——2026-08-27 推翻：改採「其餘全鎖含純唯讀查看」原則（46 §3.4
// dashboard/CRM 查詢/報表同款裁決），收斂進 CAP_MARKETING（BLOCK-if-suspended，
// 停權賣家看不到市場行情）。掛在同一個 SellersPath 底下不變。
func registerPricebookRoutes(app *gin.RouterGroup, h *handler.Holder) {
	personalized := app.Group(SellersPath)
	personalized.Use(middleware.PrivateNoStore(), middleware.JWTAuth())
	sellerHandle(personalized, "GET", PricebookOverview, authz.CapMarketing, h.GetMarketOverview)
	sellerHandle(personalized, "POST", PricebookWatchTargets, authz.CapMarketing, h.CreateMarketWatchTarget)
	sellerHandle(personalized, "DELETE", PricebookWatchTarget, authz.CapMarketing, h.DeleteMarketWatchTarget)
	pricebook := app.Group(SellersPath)
	pricebook.Use(middleware.JWTAuth())
	{
		sellerHandle(pricebook, "GET", PricebookCropCatalogSearch, authz.CapMarketing, h.SearchCropCatalog)
		sellerHandle(pricebook, "GET", PricebookObservedPrice, authz.CapMarketing, h.GetObservedPriceHistory)
		sellerHandle(pricebook, "GET", PricebookForecastCurve, authz.CapMarketing, h.GetForecastCurve)
		sellerHandle(pricebook, "GET", PricebookTrendSummary, authz.CapMarketing, h.GetTrendSummary)
		sellerHandle(pricebook, "GET", PricebookSpatialSurface, authz.CapMarketing, h.GetSpatialSurface)
		sellerHandle(pricebook, "GET", PricebookRegionalComparison, authz.CapMarketing, h.GetRegionalComparison)
		sellerHandle(pricebook, "GET", PricebookCategoryResolution, authz.CapMarketing, h.ResolveCategoryMapping)
		sellerHandle(pricebook, "GET", PricebookCategoryPriceRollup, authz.CapMarketing, h.GetCategoryPriceRollup)
	}
}
