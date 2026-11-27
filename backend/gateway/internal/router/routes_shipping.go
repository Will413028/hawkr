package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

func registerShippingRoutes(app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {
	shipments := app.Group(ShipmentsPath)
	shipments.Use(middleware.JWTAuth())
	{
		sellerHandle(shipments, "POST", ShipmentsBatchPrintCvsLabel, authz.CapOrderFulfill, h.BatchPrintCvsLabel)
		sellerHandle(shipments, "POST", ShipmentsBatchGenerateHomeLabel, authz.CapOrderFulfill, h.BatchGenerateHomeLabel)
		sellerHandle(shipments, "GET", ShipmentsAnomaliesPath, authz.CapOrderFulfill, h.ListSellerShipmentAnomalies)
		sellerHandle(shipments, "GET", ShipmentsAnomaliesStats, authz.CapOrderFulfill, h.GetSellerShipmentAnomalyStats)
		sellerHandle(shipments, "POST", ShipmentsAnomalyAcknowledge, authz.CapOrderFulfill, h.AcknowledgeShipmentAnomaly)
		sellerHandle(shipments, "POST", ShipmentsAnomalyIgnore, authz.CapOrderFulfill, h.IgnoreShipmentAnomaly)
		sellerHandle(shipments, "POST", "/:order_no"+ShipmentsPrintCvsLabel, authz.CapOrderFulfill, h.PrintCvsLabel)
		sellerHandle(shipments, "POST", "/:order_no"+ShipmentsGenerateHomeLabel, authz.CapOrderFulfill, h.GenerateHomeLabel)
		sellerHandle(shipments, "POST", "/:order_no"+ShipmentsCallTcat, authz.CapOrderFulfill, h.CallTcat)
		sellerHandle(shipments, "POST", "/:order_no/confirm", authz.CapOrderFulfill, h.ConfirmCvsShipment)
		sellerHandle(shipments, "GET", "/:order_no/tracking", authz.CapOrderFulfill, h.GetSellerShipmentTracking)
	}

	onboardings := app.Group(ShippingOnboardingsPath)
	onboardings.Use(middleware.JWTAuth())
	{
		sellerHandle(onboardings, "POST", ShippingOnboardingsCvs, authz.CapShippingSetup, h.OnboardCvsShipping)
		sellerHandle(onboardings, "POST", ShippingOnboardingsHomeDelivery, authz.CapShippingSetup, h.OnboardHomeDeliveryShipping)
		sellerHandle(onboardings, "GET", "", authz.CapProductManage, h.ListShippingOnboardings)
		sellerHandle(onboardings, "GET", ShippingOnboardingID, authz.CapProductManage, h.GetShippingOnboarding)
		sellerHandle(onboardings, "POST", ShippingOnboardingToggle, authz.CapShippingSetup, h.ToggleShippingOnboarding)
	}

	sellerHandleAuthed(app, "GET", ShippingMethodsPath, authz.CapProductManage, h.ListSellerShippingMethods)

	app.POST(StoreMapSessionsPath, middleware.JWTAuth(), h.GetStoreMapParams)

	sellerHandleAuthed(app, "GET", ShippingBusinessDaysPath, authz.CapOrderFulfill, h.ListBusinessDays)

	storePickups := app.Group(StorePickupsPath)
	storePickups.Use(middleware.JWTAuth(), requireBuyer())
	{
		storePickups.POST("", h.AddBuyerStorePickup)
		storePickups.GET("", h.ListBuyerStorePickups)
		storePickups.PUT(StorePickupID, h.UpdateBuyerStorePickup)
		storePickups.DELETE(StorePickupID, h.DeleteBuyerStorePickup)
	}

	admin.GET(AdminShipmentsAnomaliesPath, h.ListShipmentAnomaliesForAdmin)
	admin.POST(AdminShipmentsAnomalyAcknowledge, h.AcknowledgeShipmentAnomalyForAdmin)
	admin.POST(AdminShipmentsAnomalyIgnore, h.IgnoreShipmentAnomalyForAdmin)
	admin.GET(AdminShipmentsPath, h.ListShipmentsForAdmin)
	admin.GET(AdminShipmentDetail, h.GetShipmentForAdmin)
	admin.GET(AdminSellerShipmentSettingsPath, h.ListSellerShipmentSettingsForAdmin)

}

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

func registerStoreMapReturnRoutes(r *gin.Engine, h *handler.Holder) {
	r.GET(StoreMapRedirectPath, h.StoreMapRedirect)
	r.POST(StoreMapReturnPath, h.StoreMapReturn)
}
