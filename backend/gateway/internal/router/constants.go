package router

const (
	// API 基礎路徑
	APIV1Path = "/api/v1"

	// 三大 Namespace（依 API 設計規範）
	PublicPath = "/public" // 匿名可存取
	AppPath    = "/app"    // 需要 JWT 認證
	AdminPath  = "/admin"  // 管理員專用
)

const (
	StoreMapSessionsPath = "/store_map_sessions"
	StoreMapRedirectPath = "/store_map/redirect"
	StoreMapReturnPath   = "/store_map/return"
)

const (
	StorePickupsPath = "/store_pickups"
	StorePickupID    = "/:store_pickup_id"
)

const (
	ProductsPath            = "/products"
	HashtagsPath            = "/hashtags"              // 02 §5.12 hashtag auto-complete
	OriginsSummaryPath      = "/origins/summary"       // 02 §5.13 公開產地聚合
	SellerCitiesSummaryPath = "/seller-cities/summary" // 商家註冊所在地完整聚合
	ProductID               = "/:id"
	ProductBrief            = "/:id/brief"
	ProductSkus             = "/:id/skus"
	ProductRelated          = "/:id/related"

	// Aliases with 's' suffix for compatibility
	ProductsIDPath  = "/:id"
	ProductsBrief   = "/:id/brief"
	ProductsRating  = "/:id/rating"
	ProductsRelated = "/:id/related"

	// 賣家商品狀態
	ProductStatus = "/:id/status"

	// 認證驗證
	CertificationVerify = "/certifications/verify"

	// 商品認證申請（19 號設計書 §9.1）。掛在 /app/products 之下：認證是商品的附屬資源，
	// 歸屬與權限完全跟著商品走（requireSeller(CapProductManage)）。
	ProductCertificationApplications        = "/:id/certification_applications"
	ProductCertificationApplicationWithdraw = "/:id/certification_applications/:application_id/withdraw"
)

const (
	CategoriesPath         = "/categories"
	CategoriesTree         = "/tree"
	CategoryID             = "/:id"
	CategoryPathSeg        = "/:id/path"
	CategoryChildren       = "/:id/children"
	CategoryProducts       = "/:id/products"
	CategoryRestore        = "/:id/restore"
	ProductsBatchChangeCat = "/products/batch-change-category"
)

const (
	OrdersPath               = "/orders"
	OrderStatusCounts        = "/status_counts"
	OrderNo                  = "/:order_no"
	OrderCancellations       = "/:order_no/cancellations"
	OrderConfirmReceipt      = "/:order_no/confirm_receipt"
	OrderLogistics           = "/:order_no/logistics"
	OrderSellerCancellations = "/:order_no/seller_cancellations"
	OrderRetryGrant          = "/:order_no/retry_grant"
)

const (
	ShipmentsPath                   = "/shipments"
	ShipmentsPrintCvsLabel          = "/print_cvs_label"
	ShipmentsBatchPrintCvsLabel     = "/batch_print_cvs_label"
	ShipmentsGenerateHomeLabel      = "/generate_home_label"
	ShipmentsBatchGenerateHomeLabel = "/batch_generate_home_label"
	ShipmentsCallTcat               = "/call_tcat"

	// §5.21 配送異常賣家通知
	ShipmentsAnomaliesPath      = "/anomalies"
	ShipmentsAnomaliesStats     = "/anomalies/stats"
	ShipmentsAnomalyAcknowledge = "/anomalies/:shipment_id/:anomaly_type/acknowledge"
	ShipmentsAnomalyIgnore      = "/anomalies/:shipment_id/:anomaly_type/ignore"
)

const (
	ShippingOnboardingsPath         = "/shipping_onboardings"
	ShippingOnboardingsCvs          = "/cvs"
	ShippingOnboardingsHomeDelivery = "/home_delivery"
	ShippingOnboardingID            = "/:onboarding_id"
	ShippingOnboardingToggle        = "/:onboarding_id/toggle"
)

const (
	ShippingMethodsPath = "/shipping_methods"
)

const (
	ShippingBusinessDaysPath = "/shipping_business_days"
)

const (
	CartsPath        = "/carts"
	CartsItems       = "/items"
	CartsItemQty     = "/items/:sku_id/quantity"
	CartsItemBySkuID = "/items/:sku_id"
	CartsMerge       = "/merge"
)

const (
	CheckoutPath        = "/checkout"
	CheckoutValidate    = "/validate"
	CheckoutShippingFee = "/shipping_fee"
	CheckoutPreview     = "/preview"
	CheckoutConfirm     = "/confirm"
)

const (
	ShippingQuotesPath = "/shipping_quotes"
)

const (
	AdminProductsPath  = "/products"
	AdminProducts      = "/products" // Alias
	AdminProductDetail = "/products/:id"
	AdminProductStatus = "/products/:id/status"
	AdminProductCache  = "/products/cache"
)

// ⚠️ 路徑刻意是 product_certification_applications 而不是 certifications：
// /admin/certifications* 是商家實名認證（KYC，`docs/modules/seller.md` §13）的前綴，兩者是不同的東西；KYC 審核在單體
// （SellerAdminService），gateway 不註冊這個前綴，見 TestLegacyKYCAdminRoutesStayDeleted。
const (
	AdminProductCertificationApplications      = "/product_certification_applications"
	AdminProductCertificationApplicationDetail = "/product_certification_applications/:id"
	AdminProductCertificationAuditLogs         = "/product_certification_applications/:id/audit_logs"
	AdminProductCertificationApprove           = "/product_certification_applications/:id/approve"
	AdminProductCertificationReject            = "/product_certification_applications/:id/reject"
	AdminProductCertificationRevoke            = "/product_certification_applications/:id/revoke"
	// reverify 一律 POST：它有副作用（打政府 API 消耗配額並寫 DB），GET 會被瀏覽器／
	// 代理／prefetch 重放（19 §9.3）。
	AdminProductCertificationReverify = "/product_certification_applications/:id/reverify"
)

const (
	AdminShipmentsPath              = "/shipments"
	AdminShipmentDetail             = "/shipments/:id"
	AdminSellerShipmentSettingsPath = "/seller_shipment_settings"
	AdminShipmentsAnomaliesPath     = "/shipments/anomalies" // §5.21 Admin 配送異常監控
	// issue #1327 Admin 配送異常處置（static prefix，必須在 /shipments/:id 之前註冊）
	AdminShipmentsAnomalyAcknowledge = "/shipments/anomalies/:shipment_id/:anomaly_type/acknowledge"
	AdminShipmentsAnomalyIgnore      = "/shipments/anomalies/:shipment_id/:anomaly_type/ignore"
)

const (
	SearchPath     = "/search"
	SearchProducts = "/products"
	SearchSuggest  = "/suggest"
)

// 商家相關
const (
	SellersPath     = "/sellers"
	SellersIDPath   = "/sellers/:id"
	SellersStats    = "/sellers/:id/stats"
	SellersProducts = "/sellers/:id/products"
)

const (
	BuyersPath = "/buyers/me"
)

const (
	// 共用 Resource
	CouponsPath         = "/coupons"
	CouponTemplatesPath = "/coupon_templates"
	CouponQuotasPath    = "/coupon_quotas"

	// App — Buyer
	CouponsAvailable = "/available"
	CouponsReceive   = "/:template_id/receive"
	CouponsClaim     = "/:template_id/claim"

	// App — Seller
	CouponTemplateID     = "/:template_id"
	CouponTemplateRevoke = "/:template_id/revoke"

	// Admin — Template
	AdminCouponTemplatePublish    = "/:template_id/publish"
	AdminCouponTemplateDisable    = "/:template_id/disable"
	AdminCouponTemplateIssue      = "/:template_id/issue"
	AdminCouponTemplateDistribute = "/:template_id/distribute"

	// Admin — Instance
	AdminCouponCompensations = "/compensations"
	AdminCouponVoid          = "/:instance_id/void"

	// Admin — Quota
	AdminCouponQuotaSeller = "/:store_id"
)

const (
	AfterSalesPath                       = "/after_sales"
	AfterSalesCaseNo                     = "/:case_no"
	AfterSalesSupplement                 = "/:case_no/supplement"
	AfterSalesCancel                     = "/:case_no/cancel"
	AfterSalesAppeal                     = "/:case_no/appeal"      // §10 買家提交申訴
	AfterSalesArbitration                = "/:case_no/arbitration" // §10 買家 / 賣家查裁決
	SellerAfterSalesPath                 = "/seller/after_sales"
	SellerAfterSalesResolve              = "/:case_no/resolve"
	SellerAfterSalesConfirm              = "/:case_no/confirm_receipt"
	AdminAfterSalesPath                  = "/after_sales"
	AdminAfterSalesAppeals               = "/appeals"                           // §10 Admin 待裁決列表
	AdminAfterSalesDecide                = "/:case_no/decide"                   // §10 Admin 裁決
	AdminAfterSalesDetail                = "/:case_no"                          // §10.6.6 Admin 單案詳情
	AdminAfterSalesRetryPrintReturnLabel = "/:case_no/retry_print_return_label" // §5.3.6 admin/ops 人工重試 7-11 退貨列印
	AdminAfterSalesForceConfirmReceipt   = "/:case_no/force_confirm_receipt"    // §5.3.12 Admin 強制確認收貨
)

const (
	// 群組根路徑
	BuyerInvoiceProfilesPath = "/buyers/me/invoice_profiles" // app group，端點 1–5
	BuyerInvoicesPath        = "/invoices"                   // app group，端點 68–71（買家發票中心 B09／B05）
	SellerInvoicesPath       = "/seller/invoices"            // app group，端點 16–36、62 ＋ 62R
	AdminInvoicesPath        = "/invoices"                   // admin group，端點 37–61、66–67

	// 端點相對路徑（值 ＝ §6.1.0(15) 的 Path 去掉群組前綴）
	InvoiceProfileID                 = "/:profile_id"                                            // 端點 4
	InvoiceProfileDefault            = "/:profile_id/default"                                    // 端點 5
	OrderInvoice                     = "/:order_no/invoice"                                      // 端點 8
	OrderInvoiceComplaints           = "/:order_no/invoice_complaints"                           // 端點 14
	OrderInvoiceComplaintID          = "/:order_no/invoice_complaints/:complaint_id"             // 端點 15
	OrderInvoiceComplaintRejections  = "/:order_no/invoice_complaints/:complaint_id/rejections"  // 端點 63
	OrderInvoiceComplaintReopenings  = "/:order_no/invoice_complaints/:complaint_id/reopenings"  // 端點 65
	OrderInvoiceComplaintWithdrawals = "/:order_no/invoice_complaints/:complaint_id/withdrawals" // 端點 64
	OrderInvoiceCorrections          = "/:order_no/invoice_corrections"                          // 端點 11
	OrderInvoiceCorrectionID         = "/:order_no/invoice_corrections/:correction_id"           // 端點 12
	OrderInvoiceDocument             = "/:order_no/invoice_documents/:document_id"               // 端點 9
	OrderInvoiceDocumentFile         = "/:order_no/invoice_documents/:document_id/file"          // 端點 10
	BuyerInvoiceDocuments            = "/documents"                                              // 端點 68
	BuyerInvoiceDocumentID           = "/documents/:document_id"                                 // 端點 69
	BuyerInvoiceDocumentFile         = "/documents/:document_id/file"                            // 端點 70
	BuyerInvoiceDocumentResend       = "/documents/:document_id/notification_resends"            // 端點 71
	SellerInvoiceComplaints          = "/complaints"                                             // 端點 34
	SellerInvoiceComplaintID         = "/complaints/:complaint_id"                               // 端點 35
	SellerInvoiceComplaintResponses  = "/complaints/:complaint_id/responses"                     // 端點 36
	SellerInvoiceDocuments           = "/documents"                                              // 端點 30
	SellerInvoiceDocumentID          = "/documents/:document_id"                                 // 端點 31
	SellerInvoiceDocumentResend      = "/documents/:document_id/notification_resends"            // 端點 32
	SellerInvoiceJobID               = "/jobs/:job_id"                                           // 端點 33
	SellerInvoiceJobExternalIssues   = "/jobs/:job_id/external_issuances"                        // 端點 62
	SellerInvoiceOrderExternalIssues = "/orders/:order_no/external_issuances"                    // 端點 62R
	SellerInvoiceOverview            = "/overview"                                               // 端點 16
	SellerInvoiceProviderAccount     = "/provider_account"                                       // 端點 23
	SellerInvoiceProviderActivate    = "/provider_account/activations"                           // 端點 26
	SellerInvoiceProviderCreds       = "/provider_account/credentials"                           // 端點 24
	SellerInvoiceProviderRotate      = "/provider_account/rotations"                             // 端點 27
	SellerInvoiceProviderVerify      = "/provider_account/verifications"                         // 端點 25
	SellerInvoiceReceiptAuth         = "/receipt_authorization"                                  // 端點 28
	SellerInvoiceReceiptAuthConfirm  = "/receipt_authorization/confirmations"                    // 端點 29
	SellerReceivedInvoiceDocuments   = "/received_documents"                                     // 端點 72
	SellerInvoiceAttestations        = "/self_issue_attestations"                                // 端點 21
	SellerInvoiceTaxProfile          = "/tax_profile"                                            // 端點 17
	SellerInvoiceTaxProfileDraft     = "/tax_profile/draft"                                      // 端點 18
	SellerInvoiceTaxExtensions       = "/tax_profile/extension_requests"                         // 端點 20
	SellerInvoiceTaxSubmissions      = "/tax_profile/submissions"                                // 端點 19
	AdminInvoiceAuditLogs            = "/audit_logs"                                             // 端點 61
	AdminInvoiceComplaints           = "/complaints"                                             // 端點 58
	AdminInvoiceComplaintID          = "/complaints/:complaint_id"                               // 端點 59
	AdminInvoiceComplaintActions     = "/complaints/:complaint_id/actions"                       // 端點 60
	AdminInvoiceDocuments            = "/documents"                                              // 端點 47
	AdminInvoiceDocumentID           = "/documents/:document_id"                                 // 端點 48
	AdminInvoiceDocumentResend       = "/documents/:document_id/notification_resends"            // 端點 49
	// 端點 73：把停在人工出口、且零 operation 的文件送回自動線。掛在 document 上而不是
	// job 上——那些文件根本沒有 job（見 usecase.ResumeDocumentFromManual 的紅字）。
	AdminInvoiceDocumentResume    = "/documents/:document_id/manual_resolutions"        // 端點 73
	AdminInvoiceIssuances         = "/issuances"                                        // 端點 57
	AdminInvoiceJobs              = "/jobs"                                             // 端點 50
	AdminInvoiceJobID             = "/jobs/:job_id"                                     // 端點 51
	AdminInvoiceJobCancellations  = "/jobs/:job_id/cancellations"                       // 端點 53
	AdminInvoiceJobManualResults  = "/jobs/:job_id/manual_results"                      // 端點 56
	AdminInvoiceJobManualUnlocks  = "/jobs/:job_id/manual_unlocks"                      // 端點 54
	AdminInvoiceJobUnlockApproval = "/jobs/:job_id/manual_unlocks/:unlock_id/approvals" // 端點 55
	// A4 解鎖證據的二進位子路由（#3108）。與端點 51 的 JSON 詳情分開：
	// 這一條回檔案本身，走 invoice-service 的 file server（10042）而不是 gRPC。
	AdminInvoiceUnlockEvidenceContent = "/jobs/:job_id/manual_unlocks/:unlock_id/evidence/content"
	AdminInvoiceJobRetries            = "/jobs/:job_id/retries"                      // 端點 52
	AdminInvoiceOverview              = "/overview"                                  // 端點 37
	AdminInvoiceProviderAccounts      = "/provider_accounts"                         // 端點 42
	AdminInvoiceProviderAccountID     = "/provider_accounts/:store_id"               // 端點 43
	AdminInvoiceProviderCutovers      = "/provider_accounts/:store_id/cutovers"      // 端點 46
	AdminInvoiceProviderDeactivate    = "/provider_accounts/:store_id/deactivations" // 端點 45
	AdminInvoiceProviderHealth        = "/provider_accounts/:store_id/health_checks" // 端點 44
	AdminInvoiceProviderReactivations = "/provider_accounts/:store_id/reactivations" // 端點 66
	AdminInvoiceProviderRevocations   = "/provider_accounts/:store_id/revocations"   // 端點 67
	AdminInvoiceTaxReviews            = "/tax_reviews"                               // 端點 38
	AdminInvoiceTaxReviewID           = "/tax_reviews/:store_id"                     // 端點 39
	AdminInvoiceTaxDecisions          = "/tax_reviews/:store_id/decisions"           // 端點 40
	AdminInvoiceTaxExtensions         = "/tax_reviews/:store_id/extensions"          // 端點 41

	// PDF 二進位子路由（不計入 72 支端點，但各自是一條真實路由）
	OrderInvoiceDocumentContent      = "/:order_no/invoice_documents/:document_id/file/content" // 買家 PDF 二進位子路由（§6.1.4）
	SellerInvoiceDocumentFileContent = "/documents/:document_id/file/content"                   // 賣家 PDF 二進位子路由（§6.1.4）
	AdminInvoiceDocumentContent      = "/documents/:document_id/file/content"                   // ADM PDF 二進位子路由（§6.1.4）
	AdminInvoiceTaxDocumentContent   = "/tax_reviews/:store_id/documents/:document_id/content"  // ADM 稅籍佐證檔二進位子路由（plan 8f-1）
)

const (
	ShareURLPath = "/share_url"
)

const (
	CachesPath         = "/caches"
	CachesFlush        = "/flush"
	CachesFlushService = "/flush/:service"
)

const (

	// Service names
	ServiceProduct    = "product"
	ServiceOrder      = "order"
	ServiceShipping   = "shipping"
	ServiceAftersales = "aftersales"
	ServiceCart       = "cart"
	ServiceCoupon     = "coupon"
	// ServiceInvoice 指向 invoice-service 的檔案串流面（INVOICE_SERVICE_FILE_PORT
	// 10042），供三條 PDF 二進位子路由反代；與 gRPC 面（10041）無關。
	ServiceInvoice = "invoice"
)
