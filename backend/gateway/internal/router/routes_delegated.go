package router

import (
	"net/http"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerDelegatedOperationRoutes(admin, adminNoAuth *gin.RouterGroup, h *handler.Holder) {
	deps := newDelegationDeps(h)

	adminHandle(admin, http.MethodGet, "/sellers/:store_id/delegated-operation/preflight", h.GetDelegatedOperationPreflight)
	adminHandle(admin, http.MethodPost, "/delegated-operations", h.CreateDelegatedOperation)
	adminHandle(admin, http.MethodPost, "/delegated-operations/:session_id/revoke", h.RevokeDelegatedOperation)
	adminHandle(admin, http.MethodGet, "/delegated-operations/:session_id/audit-logs", h.ListDelegatedAuditLogs)
	adminHandle(admin, http.MethodGet, "/delegated-operations/:session_id/summary", h.GetDelegatedSessionSummary)

	adminHandle(admin, http.MethodGet, "/delegated-operations", h.ListDelegatedOperations)
	adminHandle(admin, http.MethodGet, "/delegated-operations/audit-logs", h.ListAllDelegatedAuditLogs)

	adminHandle(admin, http.MethodGet, "/delegated-operations/capabilities", h.ListDelegationCapabilities)
	adminHandle(admin, http.MethodGet, "/delegated-operations/authorizations", h.ListDelegationAuthorizations)
	adminHandle(admin, http.MethodPost, "/delegated-operations/authorizations", h.GrantDelegationAuthorization)
	adminHandle(admin, http.MethodGet, "/delegated-operations/authorizations/:store_id", h.GetDelegationAuthorization)
	adminHandle(admin, http.MethodPut, "/delegated-operations/authorizations/:store_id", h.UpdateDelegationAuthorization)
	adminHandle(admin, http.MethodPost, "/delegated-operations/authorizations/:store_id/revoke", h.RevokeDelegationAuthorization)
	adminHandle(admin, http.MethodGet, "/delegated-operations/authorizations/:store_id/history", h.ListDelegationAuthorizationHistory)

	credentialHandle(adminNoAuth, http.MethodGet, "/delegated-operations/current", h.GetCurrentDelegatedOperation)
	credentialHandle(adminNoAuth, http.MethodDelete, "/delegated-operations/current", h.EndCurrentDelegatedOperation)
	credentialHandle(adminNoAuth, http.MethodGet, "/delegated-operations/current/summary", h.GetCurrentDelegatedSessionSummary)

	handoffHandle(adminNoAuth, http.MethodPost, "/delegated-operations/handoff/redeem", h.RedeemDelegationHandoff)

	sellers := "/sellers/:store_id"

	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/storefront", authz.AdminCapStorefrontRead, deps, h.DelegatedStorefrontUnavailable)
	delegatedHandle(adminNoAuth, http.MethodPatch, sellers+"/storefront", authz.AdminCapStorefrontWrite, deps, h.DelegatedStorefrontUnavailable)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/storefront/logo", authz.AdminCapStorefrontWrite, deps, h.DelegatedStorefrontUnavailable)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/storefront/cover", authz.AdminCapStorefrontWrite, deps, h.DelegatedStorefrontUnavailable)

	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/products", authz.AdminCapProductsRead, deps, h.ListDelegatedProducts)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products", authz.AdminCapProductsWrite, deps, h.CreateDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products/batch-unpublish", authz.AdminCapProductsPublish, deps, h.BatchUnpublishDelegatedProducts)
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/products/:product_id", authz.AdminCapProductsRead, deps, h.GetDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPatch, sellers+"/products/:product_id", authz.AdminCapProductsWrite, deps, h.UpdateDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products/:product_id/publish", authz.AdminCapProductsPublish, deps, h.PublishDelegatedProduct)
	delegatedHandle(adminNoAuth, http.MethodPost, sellers+"/products/:product_id/unpublish", authz.AdminCapProductsPublish, deps, h.UnpublishDelegatedProduct)

	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/orders", authz.AdminCapOrdersRead, deps, h.ListDelegatedOrders)
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/orders/:order_no", authz.AdminCapOrdersRead, deps, h.GetDelegatedOrder)
	delegatedHandle(adminNoAuth, http.MethodGet, sellers+"/orders/:order_no/recipient-contact", authz.AdminCapOrdersRead, deps, h.RevealDelegatedRecipientContact)
}

func handoffHandle(g *gin.RouterGroup, method, relPath string, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	registerAdminNoAuth(method, full)
	middleware.RegisterDelegatedRoute(method, full)
	g.Handle(method, relPath, h)
}
