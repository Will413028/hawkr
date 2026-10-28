package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerCRMRoutes 配置 CRM 路由（/api/v1/app/sellers/me）
// 所有路由需要 JWT 認證且具備賣家權限
func registerCRMRoutes(app *gin.RouterGroup, h *handler.Holder) {
	// CRM 客戶經營：整組 CAP_MARKETING（含唯讀查詢，2026-07-10 裁決 CRM 查詢也鎖）。
	crm := app.Group("/sellers/me")
	crm.Use(middleware.JWTAuth())
	const cap = authz.CapMarketing
	{
		// 買家精準查詢（依手機號）
		sellerHandle(crm, "GET", CrmCustomerLookup, cap, h.LookupBuyer)

		// 買家基本資訊（不限 CRM 客戶，透過 buyer 模組查詢）
		sellerHandle(crm, "GET", CrmCustomerMeta, cap, h.GetBuyerMeta)

		// 客戶管理
		sellerHandle(crm, "POST", CrmCustomerSearchByTags, cap, h.SearchCustomersByTags)
		sellerHandle(crm, "GET", CrmCustomers, cap, h.GetCustomerList)
		sellerHandle(crm, "GET", CrmCustomerID, cap, h.GetCustomerStats)
		sellerHandle(crm, "PATCH", CrmCustomerID, cap, h.UpdateCustomerStatus)

		// 標籤管理
		sellerHandle(crm, "GET", CrmTags, cap, h.ListMerchantTags)
		sellerHandle(crm, "POST", CrmTags, cap, h.CreateSellerTag)
		sellerHandle(crm, "PATCH", CrmTagID, cap, h.UpdateSellerTag)
		sellerHandle(crm, "DELETE", CrmTagID, cap, h.DeleteMerchantTag)

		// 客戶訂單 & 優惠券
		sellerHandle(crm, "GET", CrmCustomerOrders, cap, h.GetCustomerOrders)
		sellerHandle(crm, "GET", CrmCustomerCoupons, cap, h.GetCustomerCoupons)

		// 客戶標籤
		sellerHandle(crm, "GET", CrmCustomerTags, cap, h.GetCustomerTags)
		sellerHandle(crm, "POST", CrmCustomerTags, cap, h.AddCustomerTags)
		sellerHandle(crm, "DELETE", CrmCustomerTagID, cap, h.RemoveCustomerTag)
	}
}
