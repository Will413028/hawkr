package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// registerShippingRoutes 配置物流相關路由
// /api/v1/app/shipments — 賣家安排出貨（Action Endpoint）
// /api/v1/app/shipping_onboardings — 賣家物流建店開通
func registerShippingRoutes(app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {
	// 出貨/物流處理：整組 CAP_ORDER_FULFILL（處理既有已付款訂單 → 白名單，停權仍可用）。
	shipments := app.Group(ShipmentsPath)
	shipments.Use(middleware.JWTAuth())
	{
		// 批次超商出貨單列印（不含 :order_no，放在 path param 路由之前）
		sellerHandle(shipments, "POST", ShipmentsBatchPrintCvsLabel, authz.CapOrderFulfill, h.BatchPrintCvsLabel)
		// 批次宅配取號 PDF（不含 :order_no，放在 path param 路由之前）
		sellerHandle(shipments, "POST", ShipmentsBatchGenerateHomeLabel, authz.CapOrderFulfill, h.BatchGenerateHomeLabel)
		// §5.21 配送異常賣家通知（static prefix，必須在 :order_no 路由之前註冊）
		sellerHandle(shipments, "GET", ShipmentsAnomaliesPath, authz.CapOrderFulfill, h.ListSellerShipmentAnomalies)
		sellerHandle(shipments, "GET", ShipmentsAnomaliesStats, authz.CapOrderFulfill, h.GetSellerShipmentAnomalyStats)
		sellerHandle(shipments, "POST", ShipmentsAnomalyAcknowledge, authz.CapOrderFulfill, h.AcknowledgeShipmentAnomaly)
		sellerHandle(shipments, "POST", ShipmentsAnomalyIgnore, authz.CapOrderFulfill, h.IgnoreShipmentAnomaly)
		// 超商出貨單列印（單筆）
		sellerHandle(shipments, "POST", "/:order_no"+ShipmentsPrintCvsLabel, authz.CapOrderFulfill, h.PrintCvsLabel)
		// 產宅配託運單 PDF
		sellerHandle(shipments, "POST", "/:order_no"+ShipmentsGenerateHomeLabel, authz.CapOrderFulfill, h.GenerateHomeLabel)
		// 呼叫黑貓預約取件
		sellerHandle(shipments, "POST", "/:order_no"+ShipmentsCallTcat, authz.CapOrderFulfill, h.CallTcat)
		// 超商確認出貨
		sellerHandle(shipments, "POST", "/:order_no/confirm", authz.CapOrderFulfill, h.ConfirmCvsShipment)
		// 賣家查看訂單物流追蹤（即時同步 PayUni，見 06 §5.7.2）
		sellerHandle(shipments, "GET", "/:order_no/tracking", authz.CapOrderFulfill, h.GetSellerShipmentTracking)
	}

	// 物流建店開通：建立與切換啟用/停用要 CAP_SHIPPING_SETUP（前置條件 KYC 核准，由單體推導）；
	// 唯讀（列出、查詢）留在 CAP_PRODUCT_MANAGE。
	onboardings := app.Group(ShippingOnboardingsPath)
	onboardings.Use(middleware.JWTAuth())
	{
		// 超商 C2C 建店
		sellerHandle(onboardings, "POST", ShippingOnboardingsCvs, authz.CapShippingSetup, h.OnboardCvsShipping)
		// 宅配建店
		sellerHandle(onboardings, "POST", ShippingOnboardingsHomeDelivery, authz.CapShippingSetup, h.OnboardHomeDeliveryShipping)
		// 列出建店設定
		sellerHandle(onboardings, "GET", "", authz.CapProductManage, h.ListShippingOnboardings)
		// 查詢建店詳情
		sellerHandle(onboardings, "GET", ShippingOnboardingID, authz.CapProductManage, h.GetShippingOnboarding)
		// 切換建店設定啟用/停用
		sellerHandle(onboardings, "POST", ShippingOnboardingToggle, authz.CapShippingSetup, h.ToggleShippingOnboarding)
	}

	// 賣家配送方式查詢（商品刊登用，查 4 槽位配送可用性 → CAP_PRODUCT_MANAGE）
	sellerHandleAuthed(app, "GET", ShippingMethodsPath, authz.CapProductManage, h.ListSellerShippingMethods)

	// 超商門市地圖（買家/賣家共用，需 JWT）— JWTAuth-only 共用工具，不掛能力層
	app.POST(StoreMapSessionsPath, middleware.JWTAuth(), h.GetStoreMapParams)

	// 可出貨工作天查詢（賣家選出貨日期 → CAP_ORDER_FULFILL 白名單）
	sellerHandleAuthed(app, "GET", ShippingBusinessDaysPath, authz.CapOrderFulfill, h.ListBusinessDays)

	// 買家常用門市
	storePickups := app.Group(StorePickupsPath)
	storePickups.Use(middleware.JWTAuth(), requireBuyer())
	{
		storePickups.POST("", h.AddBuyerStorePickup)
		storePickups.GET("", h.ListBuyerStorePickups)
		storePickups.PUT(StorePickupID, h.UpdateBuyerStorePickup)
		storePickups.DELETE(StorePickupID, h.DeleteBuyerStorePickup)
	}

	// Admin — 假日管理
	holidays := admin.Group(HolidaysPath)
	{
		holidays.POST(HolidaysImport, h.ImportHolidays)
		holidays.POST(HolidaysImportContent, h.ImportHolidaysFromContent)
		holidays.GET(HolidaysCoverage, h.GetHolidayCoverage)
	}

	// Admin — 異常查詢（§29 Admin 異常訂單管理）
	// admin group 已含 JWTAuth + requireAdmin
	// §5.21 Admin 配送異常監控（static path，必須在 /shipments/:id 之前註冊）
	admin.GET(AdminShipmentsAnomaliesPath, h.ListShipmentAnomaliesForAdmin)
	// issue #1327 Admin 配送異常處置（static prefix，必須在 /shipments/:id 之前註冊）
	admin.POST(AdminShipmentsAnomalyAcknowledge, h.AcknowledgeShipmentAnomalyForAdmin)
	admin.POST(AdminShipmentsAnomalyIgnore, h.IgnoreShipmentAnomalyForAdmin)
	admin.GET(AdminShipmentsPath, h.ListShipmentsForAdmin)
	admin.GET(AdminShipmentDetail, h.GetShipmentForAdmin)
	admin.GET(AdminSellerShipmentSettingsPath, h.ListSellerShipmentSettingsForAdmin)

}

// requireBuyer 檢查使用者是否涵蓋買家權限（accountType code=1 或 3）
// 測試環境由 SKIP_ROLE_CHECK 環境變數控制跳過檢查
func requireBuyer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.SkipRoleCheck {
			c.Next()
			return
		}
		accountType, _ := c.Get("accountType")
		code, _ := accountType.(int)
		if !middleware.IsBuyer(code) {
			response.Forbidden(c, "buyer access required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// registerStoreMapReturnRoutes 配置超商門市地圖回調路由（頂層路由，無需 JWT）
// GET /store_map/redirect?id={opaqueId} — 供 APP WebView 開啟，自動 Form POST 到 PayUni
// #1394：以 opaque id 向 shipping-service 取回 canonical 表單 → html/template 安全 render
// POST /store_map/return — PayUni 門市地圖選店完成後以 Form POST 導回
func registerStoreMapReturnRoutes(r *gin.Engine, h *handler.Holder) {
	r.GET(StoreMapRedirectPath, h.StoreMapRedirect)
	r.POST(StoreMapReturnPath, h.StoreMapReturn)
}
