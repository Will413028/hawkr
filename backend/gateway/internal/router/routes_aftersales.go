package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerAfterSalesRoutes 配置售後服務相關路由
// /api/v1/app/after_sales — 買家售後
// /api/v1/app/seller/after_sales — 賣家售後
// /api/v1/app/orders/:order_no/refund_page — ATM 退款頁面（通用）
// /api/v1/app/orders/:order_no/seller_cancellations — 取消出貨（遷移自 Order）
// /atm-refund/return — ATM 退款頁面完成回調（頂層路由）
func registerAfterSalesRoutes(app *gin.RouterGroup, r *gin.Engine, h *handler.Holder) {
	// ------------------------------------------
	// 買家售後（/app/after_sales）
	// ------------------------------------------
	afterSales := app.Group(AfterSalesPath)
	afterSales.Use(middleware.JWTAuth())
	{
		afterSales.POST("", h.CreateAfterSaleCase)
		afterSales.GET("", h.GetMyAfterSaleCases)
		afterSales.GET(AfterSalesCaseNo, h.GetAfterSaleCaseDetail)
		afterSales.POST(AfterSalesSupplement, h.SupplementEvidence)
		afterSales.POST(AfterSalesCancel, h.CancelAfterSaleCase)
		// §10 申訴與平台裁決
		afterSales.POST(AfterSalesAppeal, h.SubmitAppeal)
		afterSales.GET(AfterSalesArbitration, h.GetArbitration)
	}

	// ------------------------------------------
	// 賣家售後（/app/seller/after_sales）
	// ------------------------------------------
	// 賣家售後：整組 CAP_ORDER_FULFILL（處理既有已付款訂單 → 白名單，停權仍可用）。
	sellerAfterSales := app.Group(SellerAfterSalesPath)
	sellerAfterSales.Use(middleware.JWTAuth())
	{
		sellerHandle(sellerAfterSales, "GET", "", authz.CapOrderFulfill, h.GetSellerAfterSaleCases)
		sellerHandle(sellerAfterSales, "GET", AfterSalesCaseNo, authz.CapOrderFulfill, h.GetSellerAfterSaleCaseDetail)
		sellerHandle(sellerAfterSales, "POST", SellerAfterSalesResolve, authz.CapOrderFulfill, h.ResolveAfterSaleCase)
		sellerHandle(sellerAfterSales, "POST", SellerAfterSalesConfirm, authz.CapOrderFulfill, h.ConfirmReturnReceipt)
		// §10 賣家 read-only 查裁決
		sellerHandle(sellerAfterSales, "GET", AfterSalesArbitration, authz.CapOrderFulfill, h.GetSellerArbitration)
	}

	// ------------------------------------------
	// 訂單層級路由（掛在 /app/orders 群組下）
	// ------------------------------------------
	orders := app.Group(OrdersPath)
	orders.Use(middleware.JWTAuth())
	{
		// ATM 退款頁面（買家/賣家通用）
		orders.POST(OrderRefundPage, h.GetRefundPageUrl)
		// 取消出貨（賣家，遷移自 Order Service）— 收縮型操作 CAP_ORDER_CONTRACT（白名單）
		sellerHandle(orders, "POST", OrderSellerCancellations, authz.CapOrderContract, h.CancelShipment)
	}

	// ------------------------------------------
	// ATM 退款頁面完成回調（頂層路由，PayUni Form POST 導回，無需 JWT）
	// ------------------------------------------
	r.POST(AtmRefundReturnPath, h.AtmRefundReturn)
}
