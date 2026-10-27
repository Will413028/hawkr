package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerLogisticsFinanceRoutes 配置物流財務（線 B）路由。
//
// 賣家面 /api/v1/app/logistics_finance/*   —— 賣家看自己被收了多少物流費
// Admin 面 /api/v1/admin/logistics/*        —— 應收 / 收款 / 調整 / provider 對帳 / 例外案件
//
// 🔴 賣家面整組掛 CAP_ORDER_FULFILL（46 號能力層）：物流費是「處理既有已付款訂單」
// 這條軸上的東西，與出貨、物流追蹤同一個 capability——停權賣家仍必須看得到自己欠多少，
// 否則他連要不要付清都無從判斷（而那筆錢平台是真的要收的）。
// ⚠️ 這是**沿用**既有賣家物流路由的 capability，不是為本波新造一個 token：
// routes_shipping.go 的整組出貨／物流追蹤用的就是它。
//
// 🔴 **應收與收款軸全部是 GET**——應收金額與其狀態只能由送達事件與 scheduler 推進，
// 在那條軸上開寫入會是一條繞過 idempotency key 與 lease 的第二路徑（CLAUDE.md §22）。
//
// 🔴 **例外是責任判定軸的兩條 POST**（65 §6.1.1 A-4／A-5，WP2B）。它們不碰
// logistics_receivables 一個欄位：寫的是 fulfillment_exception_cases 與
// fulfillment_case_resolutions，而那兩張表的推進本來就只能由人做（誰該負責這次履約
// 失敗，沒有任何事件算得出來）。沒有這兩條端點，責任判定案件永遠沒有人推得走，
// 訂單會卡在 SHIPPING_ABNORMAL(35)、買家的錢無限期卡住，而全鏈路沒有任何錯誤訊息。
// ⚠️ 兩條都掛 RequireIdempotencyKey（65 §6.1.4 對 A-1～A-16 的逐字要求）：maker 重送
// 會多開一張提案、checker 重送會多一次覆核，而兩者都是**不可回復**的稽核事實。
func registerLogisticsFinanceRoutes(app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {
	// ── 賣家面 ────────────────────────────────────────────────────────────
	// ⚠️ 註冊序：/logistics_finance/receivables 與 /ledger 是 static path，與
	// /logistics_finance 不衝突（gin 對同層 static segment 不會誤配），但仍依
	// 「先具體後一般」的既有慣例排列。
	finance := app.Group(SellerLogisticsFinancePath)
	finance.Use(middleware.JWTAuth())
	{
		// 十一個維度的總覽
		sellerHandle(finance, "GET", "", authz.CapOrderFulfill, h.GetSellerLogisticsFinanceSummary)
	}
	sellerHandleAuthed(app, "GET", SellerLogisticsReceivablesPath, authz.CapOrderFulfill,
		h.ListSellerLogisticsReceivables)
	sellerHandleAuthed(app, "GET", SellerLogisticsLedgerPath, authz.CapOrderFulfill,
		h.ListSellerLogisticsLedger)

	// ── Admin 面 ─────────────────────────────────────────────────────────
	// admin group 在 SetupRoutes 已掛 PrivateNoStore + JWTAuth + requireAdmin。
	// ⚠️ static prefix 必須排在 path param 路由之前（gin 的既有陷阱），所以
	// /logistics/provider_statements 列表寫在 /:statement_no 之前。
	admin.GET(AdminLogisticsReceivablesPath, h.ListLogisticsReceivablesForAdmin)
	admin.GET(AdminLogisticsReceivableDetail, h.GetLogisticsReceivableForAdmin)
	admin.GET(AdminLogisticsCollectionsPath, h.ListLogisticsCollectionsForAdmin)
	admin.GET(AdminLogisticsAdjustmentsPath, h.ListLogisticsAdjustmentsForAdmin)
	admin.GET(AdminProviderStatementsPath, h.ListProviderStatementsForAdmin)
	admin.GET(AdminProviderStatementDetail, h.GetProviderStatementForAdmin)
	admin.GET(AdminLogisticsExceptionCasesPath, h.ListFulfillmentExceptionCasesForAdmin)
	// ⚠️ 詳情（path param）必須排在上面那條 static 列表之後，與本群組既有的
	// receivables／provider_statements 兩組同樣的理由。
	admin.GET(AdminLogisticsExceptionCaseDetail, h.GetFulfillmentExceptionCaseForAdmin)

	// ── 責任判定 maker-checker（A-4 提案／A-5 覆核）──────────────────────────
	// 🔴 RequireIdempotencyKey 逐條掛在註冊參數上而不是整組 .Use()：這個 admin group
	// 承載的是全平台的 admin 路由，整組掛會改掉每一條既有寫入端點的契約。
	admin.POST(AdminLogisticsCaseResolutionsPath, middleware.RequireIdempotencyKey(),
		h.ProposeResponsibilityResolutionForAdmin)
	admin.POST(AdminLogisticsCaseResolutionApprovals, middleware.RequireIdempotencyKey(),
		h.ReviewResponsibilityResolutionForAdmin)
}
