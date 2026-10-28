package router

// ==================== Namespace 路徑 ====================

const (
	// API 基礎路徑
	APIV1Path = "/api/v1"

	// 三大 Namespace（依 API 設計規範）
	PublicPath = "/public" // 匿名可存取
	AppPath    = "/app"    // 需要 JWT 認證
	AdminPath  = "/admin"  // 管理員專用
)

// ==================== Payment（/app/payments）====================

const (
	PaymentsPath         = "/payments"
	PaymentsTradeNo      = "/:trade_no"
	PaymentsTradeHistory = "/orders/:order_id/history"
	PaymentsStatusSyncs  = "/:trade_no/status_syncs"
	PaymentsRefundNo     = "/refunds/:refund_no"
	PaymentsTradeRefunds = "/:trade_no/refunds"
	// Apple Pay on Web merchant validation（05 號設計書 §5.11）
	PaymentsApplePayValidateMerchant = "/apple_pay/validate_merchant"
)

// ==================== PayUni Webhook / 付款回調（頂層路由，不在 /api/v1 下）====================

const (
	WebhookPayUniNotify  = "/webhook/payuni/notify"
	ServicePayuniWebhook = "payuni-webhook"
	PaymentReturnPath    = "/payment/return"
	PaymentRedirectPath  = "/payment/redirect"
)

// ==================== 超商門市地圖（/app/store_map_sessions + 頂層回調）====================

const (
	StoreMapSessionsPath = "/store_map_sessions"
	StoreMapRedirectPath = "/store_map/redirect"
	StoreMapReturnPath   = "/store_map/return"
)

// ==================== 買家常用門市（/app/store_pickups）====================

const (
	StorePickupsPath = "/store_pickups"
	StorePickupID    = "/:store_pickup_id"
)

// ==================== Product（/public/products + /app/products）====================

const (
	ProductsPath            = "/products"
	HashtagsPath            = "/hashtags"              // 02 §5.12 hashtag auto-complete
	OriginsSummaryPath      = "/origins/summary"       // 02 §5.13 公開產地聚合
	SellerCitiesSummaryPath = "/seller-cities/summary" // 商家註冊所在地完整聚合
	ProductID               = "/:id"
	ProductBrief            = "/:id/brief"
	ProductReviews          = "/:id/reviews"
	ProductSkus             = "/:id/skus"
	ProductRating           = "/:id/rating"
	ProductRelated          = "/:id/related"

	// Aliases with 's' suffix for compatibility
	ProductsIDPath  = "/:id"
	ProductsBrief   = "/:id/brief"
	ProductsReviews = "/:id/reviews"
	ProductsRating  = "/:id/rating"
	ProductsRelated = "/:id/related"
	ProductsView    = "/:id/view"

	// 賣家商品狀態
	ProductStatus = "/:id/status"

	// 認證驗證
	CertificationVerify = "/certifications/verify"

	// 商品認證申請（19 號設計書 §9.1）。掛在 /app/products 之下：認證是商品的附屬資源，
	// 歸屬與權限完全跟著商品走（requireSeller(CapProductManage)）。
	ProductCertificationApplications        = "/:id/certification_applications"
	ProductCertificationApplicationWithdraw = "/:id/certification_applications/:application_id/withdraw"
)

// ==================== Category（/public/categories + /app/categories）====================

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

// ==================== Review（/app/reviews）====================

const (
	ReviewsPath = "/reviews"
	ReviewID    = "/:id"
)

// SellerReviewReply 賣家回覆評論（#1174）— 掛 /app/sellers group（requireSeller 保護）
const SellerReviewReply = "/reviews/:id/reply"

// ==================== Order（/app/orders）====================

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

// ==================== Shipments（/app/shipments）====================

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

// ==================== Shipping Onboardings（/app/shipping_onboardings）====================

const (
	ShippingOnboardingsPath         = "/shipping_onboardings"
	ShippingOnboardingsCvs          = "/cvs"
	ShippingOnboardingsHomeDelivery = "/home_delivery"
	ShippingOnboardingID            = "/:onboarding_id"
	ShippingOnboardingToggle        = "/:onboarding_id/toggle"
)

// ==================== 賣家配送方式（/app/shipping_methods）====================

const (
	ShippingMethodsPath = "/shipping_methods"
)

// ==================== 假日管理（/admin/holidays + /app/shipping_business_days）====================

const (
	HolidaysPath             = "/holidays"
	HolidaysImport           = "/import"
	HolidaysImportContent    = "/import_content"
	HolidaysCoverage         = "/coverage"
	ShippingBusinessDaysPath = "/shipping_business_days"
)

// ==================== Cart（/public/carts + /app/carts）====================

const (
	CartsPath        = "/carts"
	CartsItems       = "/items"
	CartsItemQty     = "/items/:sku_id/quantity"
	CartsItemBySkuID = "/items/:sku_id"
	CartsMerge       = "/merge"
)

// ==================== Media（/public/media + /app/media）====================

const (
	MediaPath           = "/media"
	MediaUploadSessions = "/upload_sessions"
	MediaUploadComplete = "/upload_sessions/:id/complete"
	MediaFinalize       = "/finalize"
	MediaSecureUpload   = "/secure_upload/:id"
	MediaSecureDownload = "/secure_download/:asset_id"
	MediaSecurePreview  = "/secure_preview/:session_id"
)

// ==================== 結帳（/app/checkout）====================

const (
	CheckoutPath        = "/checkout"
	CheckoutValidate    = "/validate"
	CheckoutShippingFee = "/shipping_fee"
	CheckoutPreview     = "/preview"
	CheckoutConfirm     = "/confirm"
)

// ==================== 配送報價（/app/shipping-quotes）====================

const (
	ShippingQuotesPath = "/shipping_quotes"
)

// ==================== Admin Product ====================

const (
	AdminProductsPath  = "/products"
	AdminProducts      = "/products" // Alias
	AdminProductDetail = "/products/:id"
	AdminProductStatus = "/products/:id/status"
	AdminProductCache  = "/products/cache"
)

// ==================== Admin Product Certification（19 號設計書 §9.2 / §9.3）====================

// ⚠️ 路徑刻意是 product_certification_applications 而不是 certifications：
// /admin/certifications* 曾經是商家實名認證（KYC，18 號設計書）的後台入口，兩者是不同的東西；KYC 審核已搬進單體
// （SellerAdminService），這個前綴不再使用，見 TestLegacyKYCAdminRoutesStayDeleted。
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

// ==================== Admin Shipping（§29 + §5.21 配送異常）====================

const (
	AdminShipmentsPath              = "/shipments"
	AdminShipmentDetail             = "/shipments/:id"
	AdminSellerShipmentSettingsPath = "/seller_shipment_settings"
	AdminShipmentsAnomaliesPath     = "/shipments/anomalies" // §5.21 Admin 配送異常監控
	// issue #1327 Admin 配送異常處置（static prefix，必須在 /shipments/:id 之前註冊）
	AdminShipmentsAnomalyAcknowledge = "/shipments/anomalies/:shipment_id/:anomaly_type/acknowledge"
	AdminShipmentsAnomalyIgnore      = "/shipments/anomalies/:shipment_id/:anomaly_type/ignore"
)

// ==================== 物流財務 ====================

const (
	// 賣家面：物流財務（V9 的九張表；提示詞 W6-3「Logistics Finance」十一個維度）。
	SellerLogisticsFinancePath     = "/logistics_finance"
	SellerLogisticsReceivablesPath = "/logistics_finance/receivables"
	SellerLogisticsLedgerPath      = "/logistics_finance/ledger"
)

const (
	// Admin 面：物流財務維運（應收、收款、對帳、調整）。
	AdminLogisticsReceivablesPath    = "/logistics/receivables"
	AdminLogisticsReceivableDetail   = "/logistics/receivables/:receivable_no"
	AdminLogisticsCollectionsPath    = "/logistics/collections"
	AdminLogisticsAdjustmentsPath    = "/logistics/adjustments"
	AdminProviderStatementsPath      = "/logistics/provider_statements"
	AdminProviderStatementDetail     = "/logistics/provider_statements/:statement_no"
	AdminLogisticsExceptionCasesPath = "/logistics/exception_cases"

	// Admin 面：單一例外案件詳情與責任判定 maker-checker（65 §6.1.1 A-2／A-4／A-5）。
	//
	// 🔴 以 case_no 定位而不是 case_id：case_no 是對外業務單號（uq_exc_no），ADM 佇列與
	// 稽核紀錄引用的都是它；case_id 是 TSID 內部主鍵。形狀與
	// /logistics/receivables/:receivable_no 的既有慣例一致。
	//
	// 🔴 覆核端點是 resolutions 底下的**子資源** approvals 而不是
	// `/resolutions/:resolution_id/review`（動詞）：一次覆核就是對那張提案新增一筆
	// 批核紀錄，資源化之後後續波次要加「第二關覆核」只是同一個 collection 再加一筆，
	// 不必再開一支動詞端點（API 設計規範的 Action 形狀留給真的無法資源化者）。
	AdminLogisticsExceptionCaseDetail     = "/logistics/exception_cases/:case_no"
	AdminLogisticsCaseResolutionsPath     = "/logistics/exception_cases/:case_no/resolutions"
	AdminLogisticsCaseResolutionApprovals = "/logistics/exception_cases/:case_no/resolutions/:resolution_id/approvals"
)

// ==================== Admin Payment（§29）====================

const (
	AdminPaymentTransactionsPath  = "/payment_transactions"
	AdminPaymentTransactionDetail = "/payment_transactions/:trade_no"
	AdminPaymentRefundsPath       = "/payment_refunds"
	AdminPaymentRefundDetail      = "/payment_refunds/:refund_no"
	AdminGrantsPath               = "/grants"
)

// ==================== 搜尋相關 ====================

const (
	SearchPath     = "/search"
	SearchProducts = "/products"
	SearchSuggest  = "/suggest"
)

// 商家相關
const (
	SellersPath        = "/sellers"
	SellersIDPath      = "/sellers/:id"
	SellersStats       = "/sellers/:id/stats"
	SellersRating      = "/sellers/:id/rating"
	SellersProducts    = "/sellers/:id/products"
	SellersPosts       = "/sellers/:id/posts"
	SellersRecommended = "/sellers/recommended"
	SellersForum       = "/sellers/:id/forum"
	ForumPath          = "/sellers/forum"
	ForumIDReplies     = "/:id/replies"
	ForumIDReply       = "/:id/reply"
	ForumID            = "/:id"
)

const (
	PostsPath       = "/posts"
	PostsIDPath     = "/posts/:id"
	PostsComments   = "/posts/:id/comments"
	PostsLikeCount  = "/posts/:id/like_count"
	PostsShare      = "/posts/:id/share"
	PostsLikeStatus = "/posts/:id/like_status"

	// Group 內使用（不含 /posts 前綴，避免雙重嵌套）
	PostIDRel        = "/:id"
	PostCommentsRel  = "/:id/comments"
	PostLikeCountRel = "/:id/like_count"
	PostShareRel     = "/:id/share"
	PostLikeRel      = "/:id/like"
)

// ==================== 評論相關 ====================

const (
	CommentsPath    = "/comments/:id"
	CommentsReplies = "/comments/:id/replies"
	CommentsLike    = "/comments/:id/like"
)

// ==================== 買家相關（/app/buyers/me）====================

const (
	BuyersPath = "/buyers/me"
)

// ==================== 推薦（/public/recommendations）====================

const (
	RecommendationsPath = "/recommendations"
)

// ==================== 優惠券 Coupon ====================

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

// 物流 wildcard 死代理 /app/shipping/*path 已於能力層 PR 刪除（46 §3.7）：
// shipping-service 無 HTTP 面且 port 錯配，為 404 死路由；刪除後 engine.Routes()
// 即完整攻擊面，boot 斷言才成立。

// ==================== 售後服務相關 ====================

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
	OrderRefundPage                      = "/:order_no/refund_page"
	AtmRefundReturnPath                  = "/atm-refund/return"
)

// ==================== 電子發票 Invoice（58 號設計書 §6.1）====================
//
// 路徑與 path param 一律 snake_case：gin tree 同層不允許不同名 wildcard（`:storeId` 與
// 既有 `:store_id` 同段註冊即 panic），且 metric 的 path label 取自 c.FullPath，寫錯
// 名字會讓 PromQL 恆零命中。

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

// ==================== 分享（/app/share_url）====================

const (
	ShareURLPath = "/share_url"
)

// ==================== 地址搜尋與驗證（/public/address）====================

const (
	AddressPath        = "/address"
	AddressSuggestions = "/suggestions"
	AddressValidate    = "/validate"
)

// ==================== AI 相關 ====================

// AIServicePath 死代理 /app/ai/*path 已於能力層 PR 刪除（46 §3.7）：CreateProxy
// Director 只 strip /api/v1，path 前綴不匹配 ai-service 一律 404。
const (
	AIStylesPath = "/ai_styles"
	AIQuotasPath = "/ai_quotas"
)

// ==================== Hawkr 幫找報價單邀請（doc 55 §7）====================
//
// 賣家端掛在既有的 /app/sellers/me 命名空間下（同 /app/sellers/me/assistant）；
// 站外賣家的免登入確認掛 /api/v1/public。兩條路徑與設計書 §7 的
// `POST /sellers/me/quote-invites/{id}/confirm`／`POST /public/quote-invites/confirm`
// 逐字對齊。
const ()

// ==================== #2538 賣家回價（/app/sellers/me/procurement）====================
//
// `/procurement/*` 沿用 kebab-case（同既有 rfq-price-previews／rfq-publications），
// 不新造第三種命名。Action endpoint 依 API 設計規範 §九 `POST /{resource}/{id}/{action}`。
//
// 站外業者那半邊（/public/vendor_portal 的四支寫入）刻意不在這裡：wire 形狀已凍結，
// 但啟用仍 gated on E2 的 Google email step-up，先掛上等於以放寬驗證作為預設修法。
const (
	SellerProcurementPath          = "/sellers/me/procurement"
	SellerProcurementPublications  = "/publications"
	SellerProcurementPublicationID = "/publications/:publication_id"
	SellerProcurementResponseDraft = "/publications/:publication_id/response-draft"
	SellerProcurementResponses     = "/publications/:publication_id/seller-responses"
	SellerProcurementWithdraw      = "/publications/:publication_id/seller-responses/:seller_response_id/withdraw"
	SellerProcurementDecline       = "/publications/:publication_id/decline"
)

// ==================== 站外業者 Vendor Portal（#2539 E1，public/vendor_portal）====================

const (
	VendorPortalPath = "/vendor_portal"
)

// ==================== 開發工具（/public/caches）====================

const (
	CachesPath         = "/caches"
	CachesFlush        = "/flush"
	CachesFlushService = "/flush/:service"
)

// ==================== Webhook 模擬器（/dev/webhook-simulator）====================

const (
	WebhookSimulatorPath       = "/dev/webhook-simulator"
	WebhookSimulatorSendPath   = "/dev/webhook-simulator/send"
	WebhookSimulatorLookupPath = "/dev/webhook-simulator/lookup"
)

// ==================== CRM（/app/sellers/me）====================

const (
	CrmCustomers            = "/customers"
	CrmCustomerID           = "/customers/:buyerId"
	CrmCustomerTags         = "/customers/:buyerId/tags"
	CrmCustomerTagID        = "/customers/:buyerId/tags/:tagId"
	CrmCustomerOrders       = "/customers/:buyerId/orders"
	CrmCustomerCoupons      = "/customers/:buyerId/coupons"
	CrmCustomerMeta         = "/customers/:buyerId/meta"
	CrmCustomerLookup       = "/customers/lookup"
	CrmCustomerSearchByTags = "/customers/search-by-tags"
	CrmTags                 = "/tags"
	CrmTagID                = "/tags/:tagId"
)

// ==================== 聊天相關 ====================

const (
	ChatPath                 = "/chat"
	ChatWS                   = "/ws"
	ChatWSTicket             = "/ws-ticket"
	ChatMessages             = "/messages"
	ChatHistory              = "/history/:targetId"
	ChatConversations        = "/conversations"
	ChatUnread               = "/unread"
	ChatReadReceipts         = "/messages"
	ChatConversationReceipts = "/conversations/:targetId"
	ChatAllReceipts          = "/conversations"

	// 商家管理相關（相對路徑，在 /app/sellers 群組內使用）
	SellerPosts         = "/posts"
	SellerPostID        = "/posts/:id"
	SellerPostPublish   = "/posts/:id/publish"
	SellerPostUnpublish = "/posts/:id/unpublish"
	SellerPostReports   = "/posts/:id/reports"
	SellerMePosts       = "/me/posts"

	// Service names
	ServiceProduct    = "product"
	ServiceOrder      = "order"
	ServicePayment    = "payment"
	ServiceShipping   = "shipping"
	ServiceAftersales = "aftersales"
	ServiceMedia      = "media"
	ServiceCart       = "cart"
	ServiceCoupon     = "coupon"
	// ServiceInvoice 指向 invoice-service 的檔案串流面（INVOICE_SERVICE_FILE_PORT
	// 10042），供三條 PDF 二進位子路由反代；與 gRPC 面（10041）無關。
	ServiceInvoice = "invoice"
)
