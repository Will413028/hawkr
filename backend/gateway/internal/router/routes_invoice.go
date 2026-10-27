package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

// registerInvoiceRoutes 掛載 58 號《電子發票服務》§6.1 的全部對外 REST 路由。
//
// 四條路徑前綴：
//   - /api/v1/app/buyers/me/invoice_profiles  買家常用發票資料（端點 1–5）
//   - /api/v1/app/orders/:order_no/invoice*   訂單層級的買家發票（端點 8–15、63–65 ＋ PDF）
//   - /api/v1/app/invoices/documents          買家發票中心 B09／B05（端點 68–71）
//   - /api/v1/app/seller/invoices             賣家面（端點 16–36、62、62R ＋ PDF）
//   - /api/v1/admin/invoices                  ADM（端點 37–61、66–67 ＋ PDF）
//
// **ADM 不走代行**：`invoice` 是 56 號代行路由的 boot-time 路徑禁字
// （seller_route_assert.go 的 delegatedPathForbiddenWords），但那個檢查的作用域是
// `authz.RegisteredAdmin` **代行路由註冊表**、不是路徑前綴 ⇒ 這裡用一般
// `adminInvoices.GET(...)` 掛在已帶 JWTAuth + requireAdmin 的 admin group 上不受影響。
// 反過來說：把任何一支改成 delegatedHandle 會讓 assertRouteCoverage 直接 panic，
// **那是刻意的**，不要為了「代行也想看發票」去繞過它。
//
// 賣家面每一條（含 PDF 反代子路由）都走 sellerHandle(g, method, path, cap, h)：宣告＝
// 掛載合一。裸掛 sellerInvoices.GET(...) 會被 boot 斷言擋下（該前綴已列入
// strictSellerPrefixes），那是這個模組唯一會自己叫的保護。
//
// Cache-Control：四個群組都先掛 middleware.PrivateNoStore()。發票含買受人統編、載具
// 識別碼、抬頭與金額，被共享快取留下就是 PII 外洩。
func registerInvoiceRoutes(app, admin *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 買家常用發票資料（/app/buyers/me/invoice_profiles，端點 1–5）
	// ------------------------------------------
	buyerProfiles := app.Group(BuyerInvoiceProfilesPath)
	buyerProfiles.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), middleware.RequireIdempotencyKey())
	{
		buyerProfiles.GET("", h.ListBuyerInvoiceProfiles)
		buyerProfiles.POST("", h.CreateBuyerInvoiceProfile)
		buyerProfiles.DELETE(InvoiceProfileID, h.DeleteBuyerInvoiceProfile)
		buyerProfiles.PUT(InvoiceProfileID, h.UpdateBuyerInvoiceProfile)
		buyerProfiles.POST(InvoiceProfileDefault, h.SetDefaultBuyerInvoiceProfile)
	}

	// ------------------------------------------
	// 訂單層級的買家發票（/app/orders，端點 8–15、63–65 ＋ PDF 子路由）
	// ------------------------------------------
	// 自建群組而不是掛回既有的 orders 群組：這裡要在認證之前掛 PrivateNoStore，而
	// 既有群組的 .Use 已經是 JWTAuth。gin 的 middleware 是註冊當下複製進該路由的鏈，
	// 所以同前綴的第二個群組不會影響既有訂單／售後路由（routes_aftersales.go 已是
	// 同一個作法）。
	//
	// ⚠️ 這個群組**不可**整組 .Use(RequireIdempotencyKey)：那等於改了訂單／售後既有
	// 路由的契約，它們會全部 400。端點 11／13／63／64／65 逐條掛。
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

		// 買家 PDF 二進位子路由：反代到 invoice-service 10042。營運鍵必須在 proxy 之前
		// 補注入——CreateProxy 只注入 x-account-id／x-account-type，x-buyer-id 只存在於
		// gRPC 路徑的 metadata_interceptor。
		orderInvoices.GET(OrderInvoiceDocumentContent,
			middleware.InjectBuyerIDHeader(h.BuyerID), invoicePDFProxy())
	}

	// ------------------------------------------
	// 買家發票中心（/app/invoices/documents，端點 68–71）
	// ------------------------------------------
	// 與 order-scoped 的入口並存而非取代：平台→買家方向的發票不掛在任何訂單底下，
	// order-scoped 的路徑取不到它（§2.3 方向三）。
	buyerInvoices := app.Group(BuyerInvoicesPath)
	buyerInvoices.Use(middleware.PrivateNoStore(), middleware.JWTAuth(), middleware.RequireIdempotencyKey())
	{
		buyerInvoices.GET(BuyerInvoiceDocuments, h.ListBuyerInvoiceDocuments)
		buyerInvoices.GET(BuyerInvoiceDocumentID, h.GetBuyerInvoiceDocument)
		buyerInvoices.GET(BuyerInvoiceDocumentFile, h.GetBuyerInvoiceDocumentFile)
		buyerInvoices.POST(BuyerInvoiceDocumentResend, h.ResendBuyerInvoiceNotification)
	}

	// ------------------------------------------
	// 賣家面（/app/seller/invoices，端點 16–36、62、62R ＋ PDF 子路由）
	// ------------------------------------------
	// capability 歸屬逐條照 §8.1.6(b)：CapInvoiceTaxRemedy 8 支（救濟管道，停權白名單）
	// ＋ CapInvoiceManage 13 支（擴張型與唯讀查看，BLOCK）＋ CapOrderFulfill 4 支
	// （履行既有訂單義務，既有白名單 token）。
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

		// 賣家 PDF 子路由與端點 31 掛同一個 CapInvoiceManage：掛成別的 cap 會出現
		// 「看不到明細卻下載得到 PDF」。
		sellerHandle(sellerInvoices, "GET", SellerInvoiceDocumentFileContent, authz.CapInvoiceManage,
			gin.HandlerFunc(func(c *gin.Context) {
				middleware.InjectStoreIDHeader(h.Store)(c)
				if c.IsAborted() {
					return
				}
				invoicePDFProxy()(c)
			}))
	}

	// ------------------------------------------
	// ADM（/admin/invoices，端點 37–61、66–67 ＋ PDF 子路由）
	// ------------------------------------------
	// admin group 在 routes.go 已掛 JWTAuth + requireAdmin，子路由不再掛。
	adminInvoices := admin.Group(AdminInvoicesPath)
	// PrivateNoStore 不在這裡：它必須排在認證之前，而認證掛在父群組 admin 上
	// ⇒ 已移到 routes.go 的 admin.Use(...)（§6.1.0(11)）。留在這裡會是第二個真相，
	// 而且對 401／403 無效。
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
		// A4 解鎖證據下載（#3108）。走 invoicePDFProxy 的同一條反代——
		// 它回的是二進位，不能走 gRPC envelope。
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

		// ADM PDF 子路由不注入營運鍵：admin 沒有 ownership 概念。
		adminInvoices.GET(AdminInvoiceDocumentContent, invoicePDFProxy())
		// 稅籍佐證檔：授權述詞（store 的稅務資格在該位置有佐證）由 invoice-service 持有，
		// 取代已刪除的通用 admin 機敏下載。
		adminInvoices.GET(AdminInvoiceTaxDocumentContent, invoicePDFProxy())
	}
}

// invoicePDFProxy 是三條 PDF 二進位子路由共用的反代 handler（58 §6.1.4）。
//
// 目標是 invoice-service 的**檔案串流面**（INVOICE_SERVICE_FILE_PORT 10042），不是
// gRPC 面：bytes 不該經 gRPC，而擁有者述詞（order_no × buyer_id × document_id）不在
// media ⇒ media 的 secure_download 代用不了（它的授權述詞等價於「只有上傳者本人」，
// 而發票 PDF 由 invoice-service 以服務身分上傳、合法讀者卻有三個）。
//
// 失敗模式 FailClientFacing（424）：消費者是我們自己的前端，不是第三方送信端。
func invoicePDFProxy() gin.HandlerFunc {
	return proxy.CreateProxy(ServiceInvoice, proxy.FailClientFacing)
}
