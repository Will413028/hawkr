package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerAfterSalesRoutes(app *gin.RouterGroup, r *gin.Engine, h *handler.Holder) {
	afterSales := app.Group(AfterSalesPath)
	afterSales.Use(middleware.JWTAuth())
	{
		afterSales.POST("", h.CreateAfterSaleCase)
		afterSales.GET("", h.GetMyAfterSaleCases)
		afterSales.GET(AfterSalesCaseNo, h.GetAfterSaleCaseDetail)
		afterSales.POST(AfterSalesSupplement, h.SupplementEvidence)
		afterSales.POST(AfterSalesCancel, h.CancelAfterSaleCase)
		afterSales.POST(AfterSalesAppeal, h.SubmitAppeal)
		afterSales.GET(AfterSalesArbitration, h.GetArbitration)
	}

	sellerAfterSales := app.Group(SellerAfterSalesPath)
	sellerAfterSales.Use(middleware.JWTAuth())
	{
		sellerHandle(sellerAfterSales, "GET", "", authz.CapOrderFulfill, h.GetSellerAfterSaleCases)
		sellerHandle(sellerAfterSales, "GET", AfterSalesCaseNo, authz.CapOrderFulfill, h.GetSellerAfterSaleCaseDetail)
		sellerHandle(sellerAfterSales, "POST", SellerAfterSalesResolve, authz.CapOrderFulfill, h.ResolveAfterSaleCase)
		sellerHandle(sellerAfterSales, "POST", SellerAfterSalesConfirm, authz.CapOrderFulfill, h.ConfirmReturnReceipt)
		sellerHandle(sellerAfterSales, "GET", AfterSalesArbitration, authz.CapOrderFulfill, h.GetSellerArbitration)
	}

	orders := app.Group(OrdersPath)
	orders.Use(middleware.JWTAuth())
	{
		sellerHandle(orders, "POST", OrderSellerCancellations, authz.CapOrderContract, h.CancelShipment)
	}
}
