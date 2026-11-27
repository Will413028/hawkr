package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

func registerInvoiceRoutes(app, admin *gin.RouterGroup, h *handler.Holder) {
	buyerProfiles := app.Group(BuyerInvoiceProfilesPath)
	buyerProfiles.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), middleware.RequireIdempotencyKey())
	{
		buyerProfiles.GET("", h.ListBuyerInvoiceProfiles)
		buyerProfiles.POST("", h.CreateBuyerInvoiceProfile)
		buyerProfiles.DELETE(InvoiceProfileID, h.DeleteBuyerInvoiceProfile)
		buyerProfiles.PUT(InvoiceProfileID, h.UpdateBuyerInvoiceProfile)
		buyerProfiles.POST(InvoiceProfileDefault, h.SetDefaultBuyerInvoiceProfile)
	}

	orderInvoices := app.Group(OrdersPath)
	orderInvoices.Use(middleware.PrivateNoStore(), middleware.JWTAuth())
	{
		orderInvoices.GET(OrderInvoice, h.GetOrderInvoice)
		orderInvoices.GET(OrderInvoiceComplaints, h.ListOrderInvoiceComplaints)
		orderInvoices.POST(OrderInvoiceComplaints, middleware.RequireIdempotencyKey(), h.CreateInvoiceComplaint)
		orderInvoices.GET(OrderInvoiceComplaintID, h.GetInvoiceComplaint)
		orderInvoices.POST(OrderInvoiceComplaintRejections, middleware.RequireIdempotencyKey(), h.RejectInvoiceComplaintResponse)
		orderInvoices.POST(OrderInvoiceComplaintReopenings, middleware.RequireIdempotencyKey(), h.ReopenInvoiceComplaint)
		orderInvoices.POST(OrderInvoiceComplaintWithdrawals, middleware.RequireIdempotencyKey(), h.WithdrawInvoiceComplaint)
		orderInvoices.POST(OrderInvoiceCorrections, middleware.RequireIdempotencyKey(), h.CreateInvoiceCorrection)
		orderInvoices.GET(OrderInvoiceCorrectionID, h.GetInvoiceCorrection)
		orderInvoices.GET(OrderInvoiceDocument, h.GetOrderInvoiceDocument)
		orderInvoices.GET(OrderInvoiceDocumentFile, h.GetOrderInvoiceDocumentFile)

		orderInvoices.GET(OrderInvoiceDocumentContent,
			middleware.InjectBuyerIDHeader(h.BuyerID), invoicePDFProxy())
	}

	buyerInvoices := app.Group(BuyerInvoicesPath)
	buyerInvoices.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), middleware.RequireIdempotencyKey())
	{
		buyerInvoices.GET(BuyerInvoiceDocuments, h.ListBuyerInvoiceDocuments)
		buyerInvoices.GET(BuyerInvoiceDocumentID, h.GetBuyerInvoiceDocument)
		buyerInvoices.GET(BuyerInvoiceDocumentFile, h.GetBuyerInvoiceDocumentFile)
		buyerInvoices.POST(BuyerInvoiceDocumentResend, h.ResendBuyerInvoiceNotification)
	}

	sellerInvoices := app.Group(SellerInvoicesPath)
	sellerInvoices.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), middleware.RequireIdempotencyKey())
	{
		sellerHandle(sellerInvoices, "GET", SellerInvoiceComplaints, authz.CapOrderFulfill, h.ListSellerInvoiceComplaints)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceComplaintID, authz.CapOrderFulfill, h.GetSellerInvoiceComplaint)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceComplaintResponses, authz.CapOrderFulfill, h.RespondSellerInvoiceComplaint)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceDocuments, authz.CapInvoiceManage, h.ListSellerInvoiceDocuments)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceDocumentID, authz.CapInvoiceManage, h.GetSellerInvoiceDocument)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceDocumentResend, authz.CapOrderFulfill, h.ResendSellerInvoiceNotification)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceJobID, authz.CapInvoiceManage, h.GetSellerInvoiceJob)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceJobExternalIssues, authz.CapInvoiceTaxRemedy, h.ReportExternalIssuance)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceOrderExternalIssues, authz.CapInvoiceTaxRemedy, h.ListOrderExternalIssuances)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceOverview, authz.CapInvoiceManage, h.GetSellerInvoiceOverview)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceProviderAccount, authz.CapInvoiceManage, h.GetSellerProviderAccount)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceProviderActivate, authz.CapInvoiceManage, h.ActivateSellerProviderAccount)
		sellerHandle(sellerInvoices, "PUT", SellerInvoiceProviderCreds, authz.CapInvoiceManage, h.PutSellerProviderCredentials)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceProviderRotate, authz.CapInvoiceManage, h.RotateSellerProviderCredentials)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceProviderVerify, authz.CapInvoiceManage, h.VerifySellerProviderAccount)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceReceiptAuth, authz.CapInvoiceManage, h.GetSellerReceiptAuthorization)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceReceiptAuthConfirm, authz.CapInvoiceManage, h.ConfirmSellerReceiptAuthorization)
		sellerHandle(sellerInvoices, "GET", SellerReceivedInvoiceDocuments, authz.CapInvoiceManage, h.ListSellerReceivedInvoiceDocuments)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceAttestations, authz.CapInvoiceTaxRemedy, h.GetSelfIssueAttestation)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceAttestations, authz.CapInvoiceTaxRemedy, h.AcceptSelfIssueAttestation)
		sellerHandle(sellerInvoices, "GET", SellerInvoiceTaxProfile, authz.CapInvoiceTaxRemedy, h.GetSellerTaxProfile)
		sellerHandle(sellerInvoices, "PUT", SellerInvoiceTaxProfileDraft, authz.CapInvoiceTaxRemedy, h.SaveSellerTaxProfileDraft)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceTaxExtensions, authz.CapInvoiceTaxRemedy, h.RequestSellerTaxExtension)
		sellerHandle(sellerInvoices, "POST", SellerInvoiceTaxSubmissions, authz.CapInvoiceTaxRemedy, h.SubmitSellerTaxProfile)

		sellerHandle(sellerInvoices, "GET", SellerInvoiceDocumentFileContent, authz.CapInvoiceManage,
			gin.HandlerFunc(func(c *gin.Context) {
				middleware.InjectStoreIDHeader(h.Store)(c)
				if c.IsAborted() {
					return
				}
				invoicePDFProxy()(c)
			}))
	}

	adminInvoices := admin.Group(AdminInvoicesPath)
	adminInvoices.Use(middleware.RequireIdempotencyKey())
	{
		adminInvoices.GET(AdminInvoiceAuditLogs, h.ListInvoiceAuditLogsForAdmin)
		adminInvoices.GET(AdminInvoiceComplaints, h.ListInvoiceComplaintsForAdmin)
		adminInvoices.GET(AdminInvoiceComplaintID, h.GetInvoiceComplaintForAdmin)
		adminInvoices.POST(AdminInvoiceComplaintActions, h.ActOnInvoiceComplaintForAdmin)
		adminInvoices.GET(AdminInvoiceDocuments, h.ListInvoiceDocumentsForAdmin)
		adminInvoices.GET(AdminInvoiceDocumentID, h.GetInvoiceDocumentForAdmin)
		adminInvoices.POST(AdminInvoiceDocumentResend, h.ResendInvoiceNotificationForAdmin)
		adminInvoices.POST(AdminInvoiceDocumentResume, h.ResumeInvoiceDocumentForAdmin)
		adminInvoices.POST(AdminInvoiceIssuances, h.CreateManualIssuanceForAdmin)
		adminInvoices.GET(AdminInvoiceJobs, h.ListInvoiceJobsForAdmin)
		adminInvoices.GET(AdminInvoiceJobID, h.GetInvoiceJobForAdmin)
		adminInvoices.POST(AdminInvoiceJobCancellations, h.CancelInvoiceJobForAdmin)
		adminInvoices.POST(AdminInvoiceJobManualResults, h.ReportManualResultForAdmin)
		adminInvoices.POST(AdminInvoiceJobManualUnlocks, h.RequestManualUnlockForAdmin)
		adminInvoices.POST(AdminInvoiceJobUnlockApproval, h.ApproveManualUnlockForAdmin)
		adminInvoices.GET(AdminInvoiceUnlockEvidenceContent, invoicePDFProxy())
		adminInvoices.POST(AdminInvoiceJobRetries, h.RetryInvoiceJobForAdmin)
		adminInvoices.GET(AdminInvoiceOverview, h.GetInvoiceOverviewForAdmin)
		adminInvoices.GET(AdminInvoiceProviderAccounts, h.ListProviderAccountsForAdmin)
		adminInvoices.GET(AdminInvoiceProviderAccountID, h.GetProviderAccountForAdmin)
		adminInvoices.GET(AdminInvoiceProviderCutovers, h.ListProviderCutoversForAdmin)
		adminInvoices.POST(AdminInvoiceProviderDeactivate, h.DeactivateProviderAccountForAdmin)
		adminInvoices.POST(AdminInvoiceProviderHealth, h.HealthCheckProviderAccountForAdmin)
		adminInvoices.POST(AdminInvoiceProviderReactivations, h.ReactivateProviderAccountForAdmin)
		adminInvoices.POST(AdminInvoiceProviderRevocations, h.RevokeProviderAccountForAdmin)
		adminInvoices.GET(AdminInvoiceTaxReviews, h.ListTaxReviewsForAdmin)
		adminInvoices.GET(AdminInvoiceTaxReviewID, h.GetTaxReviewForAdmin)
		adminInvoices.POST(AdminInvoiceTaxDecisions, h.DecideTaxReviewForAdmin)
		adminInvoices.POST(AdminInvoiceTaxExtensions, h.DecideTaxExtensionForAdmin)

		adminInvoices.GET(AdminInvoiceDocumentContent, invoicePDFProxy())
		adminInvoices.GET(AdminInvoiceTaxDocumentContent, invoicePDFProxy())
	}
}

func invoicePDFProxy() gin.HandlerFunc {
	return proxy.CreateProxy(ServiceInvoice, proxy.FailClientFacing)
}
