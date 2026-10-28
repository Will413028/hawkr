package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

type routeCap struct {
	method string
	path   string
	cap    authz.Capability
}

// expectedSellerRoutes is the authoritative capability assignment for every
// requireSeller(cap)-gated route, cross-referenced to the 06 功能矩陣 終判 and 46
// §3.4 的 capability↔類別映射表 (source: .claude/tmp/suspension/06-功能矩陣.md).
// The test asserts the live authz registry equals this set exactly — drift in either
// direction (a mount changing cap, or a new gated route missing from this table)
// fails CI. Deviations from 06's raw lean are the 46 §3.4 authority overriding it:
//   - me/reviews* : 06 leans ALLOW-ALWAYS (唯讀分析), but §3.4 裁決
//     "報表/唯讀查看也鎖" → CAP_MARKETING (BLOCK).
//   - all CRM 查詢 (customers*, orders, coupons): 06 leans ALLOW-ORDER/ALLOW for some,
//     but §3.4 CAP_MARKETING covers "CRM 寫入+查詢" → BLOCK.
//
// /app/coupon_templates 建立/列表/詳情（CAP_MARKETING）掛 requireSeller。
// Revoke=CONTENT_CONTRACT 白名單不 deny，不掛 guard 故不在 registry。
var expectedSellerRoutes = []routeCap{
	{"POST", "/api/v1/app/categories", authz.CapProductManage},
	{"GET", "/api/v1/app/coupon_templates", authz.CapMarketing},
	{"POST", "/api/v1/app/coupon_templates", authz.CapMarketing},
	{"GET", "/api/v1/app/coupon_templates/:template_id", authz.CapMarketing},
	{"GET", "/api/v1/app/categories/tree", authz.CapProductManage},
	{"GET", "/api/v1/app/hashtags", authz.CapProductManage},
	{"POST", "/api/v1/app/orders/:order_no/seller_cancellations", authz.CapOrderContract},
	{"GET", "/api/v1/app/product_import_jobs", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs", authz.CapProductManage},
	{"GET", "/api/v1/app/product_import_jobs/:import_job_id", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/cancel", authz.CapProductManage},
	{"GET", "/api/v1/app/product_import_jobs/:import_job_id/category_confirmation_tokens", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/commit", authz.CapProductManage},
	{"PUT", "/api/v1/app/product_import_jobs/:import_job_id/csv", authz.CapProductManage},
	{"GET", "/api/v1/app/product_import_jobs/:import_job_id/items", authz.CapProductManage},
	{"GET", "/api/v1/app/product_import_jobs/:import_job_id/items/:import_item_id", authz.CapProductManage},
	{"PUT", "/api/v1/app/product_import_jobs/:import_job_id/items/:import_item_id", authz.CapProductManage},
	{"GET", "/api/v1/app/product_import_jobs/:import_job_id/items/:import_item_id/preview", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/items/batch_update_category", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/items/batch_update_shipping", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/items/batch_update_target_status", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/items/remove", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/items/restore", authz.CapProductManage},
	{"POST", "/api/v1/app/product_import_jobs/:import_job_id/items/retry_assets", authz.CapProductManage},
	{"GET", "/api/v1/app/product_import_jobs/:import_job_id/preview_listing", authz.CapProductManage},
	{"GET", "/api/v1/app/products", authz.CapProductManage},
	{"POST", "/api/v1/app/products", authz.CapProductManage},
	{"DELETE", "/api/v1/app/products/:id", authz.CapProductManage},
	{"GET", "/api/v1/app/products/:id", authz.CapProductManage},
	{"PUT", "/api/v1/app/products/:id", authz.CapProductManage},
	{"GET", "/api/v1/app/products/:id/certification_applications", authz.CapProductManage},
	{"POST", "/api/v1/app/products/:id/certification_applications", authz.CapProductManage},
	{"POST", "/api/v1/app/products/:id/certification_applications/:application_id/withdraw", authz.CapProductManage},
	{"GET", "/api/v1/app/products/:id/skus", authz.CapProductManage},
	{"PUT", "/api/v1/app/products/:id/status", authz.CapProductManage},
	{"GET", "/api/v1/app/products/certifications/verify", authz.CapProductManage},
	{"GET", "/api/v1/app/seller/after_sales", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/seller/after_sales/:case_no", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/seller/after_sales/:case_no/arbitration", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/seller/after_sales/:case_no/confirm_receipt", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/seller/after_sales/:case_no/resolve", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/sellers/me/assistant/chat", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/assistant/inventory", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/assistant/orders", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/assistant/payment", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/assistant/product-autofill", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/assistant/product-listing/chat", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/assistant/sessions/latest", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/customers", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/customers/:buyerId", authz.CapMarketing},
	{"PATCH", "/api/v1/app/sellers/me/customers/:buyerId", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/customers/:buyerId/coupons", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/customers/:buyerId/meta", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/customers/:buyerId/orders", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/customers/:buyerId/tags", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/customers/:buyerId/tags", authz.CapMarketing},
	{"DELETE", "/api/v1/app/sellers/me/customers/:buyerId/tags/:tagId", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/customers/lookup", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/customers/search-by-tags", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/notification-preferences", authz.CapNotification},
	{"PUT", "/api/v1/app/sellers/me/notification-preferences", authz.CapNotification},
	{"GET", "/api/v1/app/sellers/me/posts", authz.CapMarketing},
	// Hawkr 幫找 — 賣家確認報價單邀請（doc 55 §5/§7）。承接一筆新生意＝擴張型操作，
	// 停權賣家不該做得到，所以是 BLOCK 類的 CAP_MARKETING，不是白名單的
	// CAP_ORDER_CONTRACT（那是「收縮型」，例如取消出貨）。同一個 AI 採購功能的賣家
	// 面（sellers/me/assistant 整組）本來就掛 CAP_MARKETING。
	// #2538 賣家回價：與上一列同一個 capability，理由同 routes_procurement_quote.go
	// 的群組註解——CapMarketing 不在停權白名單，停權賣家送不出正式回價。
	{"GET", "/api/v1/app/sellers/me/procurement/publications", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/procurement/publications/:publication_id", authz.CapMarketing},
	{"PUT", "/api/v1/app/sellers/me/procurement/publications/:publication_id/response-draft", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/procurement/publications/:publication_id/seller-responses", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/procurement/publications/:publication_id/seller-responses/:seller_response_id/withdraw", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/procurement/publications/:publication_id/decline", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/reviews", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/reviews/stats", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/me/tags", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/me/tags", authz.CapMarketing},
	{"DELETE", "/api/v1/app/sellers/me/tags/:tagId", authz.CapMarketing},
	{"PATCH", "/api/v1/app/sellers/me/tags/:tagId", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/posts", authz.CapMarketing},
	{"DELETE", "/api/v1/app/sellers/posts/:id", authz.CapContentContract},
	{"PUT", "/api/v1/app/sellers/posts/:id", authz.CapMarketing},
	{"GET", "/api/v1/app/sellers/posts/:id", authz.CapMarketing},
	// 發布＝把內容推向買家（擴張型 → BLOCK）；下架只減不增（收縮型 → 白名單，與同組
	// 的 DELETE 同級），兩者刻意不同 capability。
	{"POST", "/api/v1/app/sellers/posts/:id/publish", authz.CapMarketing},
	{"POST", "/api/v1/app/sellers/posts/:id/unpublish", authz.CapContentContract},
	// 賣家市場行情查詢（P6+P7+#2786 categoryId 解析）：8/2 原裁決是唯讀市場資訊、
	// 非賣家專屬資料，只掛 JWTAuth 不掛 capability——2026-08-27 推翻，改採「其餘
	// 全鎖含純唯讀查看」原則（46 §3.4 dashboard/CRM 查詢/報表同款裁決）收斂進
	// CAP_MARKETING（BLOCK-if-suspended，停權賣家不可見）。
	{"POST", "/api/v1/app/sellers/reviews/:id/reply", authz.CapMarketing},
	{"POST", "/api/v1/app/shipments/:order_no/call_tcat", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/shipments/:order_no/confirm", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/shipments/:order_no/generate_home_label", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/shipments/:order_no/print_cvs_label", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/shipments/:order_no/tracking", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/shipments/anomalies", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/shipments/anomalies/:shipment_id/:anomaly_type/acknowledge", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/shipments/anomalies/:shipment_id/:anomaly_type/ignore", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/shipments/anomalies/stats", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/shipments/batch_generate_home_label", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/shipments/batch_print_cvs_label", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/shipping_business_days", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/shipping_methods", authz.CapProductManage},
	{"GET", "/api/v1/app/shipping_onboardings", authz.CapProductManage},
	{"GET", "/api/v1/app/shipping_onboardings/:onboarding_id", authz.CapProductManage},
	{"POST", "/api/v1/app/shipping_onboardings/:onboarding_id/toggle", authz.CapShippingSetup},
	{"POST", "/api/v1/app/shipping_onboardings/cvs", authz.CapShippingSetup},
	{"POST", "/api/v1/app/shipping_onboardings/home_delivery", authz.CapShippingSetup},
	{"POST", "/api/v1/app/shipping_quotes", authz.CapProductManage},
	// ---- 70 號《賣家物流財務》賣家面（W6-3 Logistics Finance）----
	// CAP_ORDER_FULFILL，與 /shipments 群組同一類：物流費是「處理既有
	// 已付款訂單」這條軸上的東西。⚠️ 停權白名單（CapOrderFulfill 在停權白名單）
	// 是刻意的——停權賣家仍必須看得到自己欠平台多少，否則他連要不要付清都無從判斷，
	// 而那筆錢平台是真的要收的（§24 分類原則的「救濟管道」那一格）。
	// 全部唯讀：本波不開任何寫入端點。
	{"GET", "/api/v1/app/logistics_finance", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/logistics_finance/ledger", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/logistics_finance/receivables", authz.CapOrderFulfill},
	// ---- 58 號《電子發票服務》賣家面（§8.1.6(b) 逐條裁決表）----
	// 25 條 ＝ CapInvoiceTaxRemedy 8（救濟管道，停權白名單）＋ CapInvoiceManage 13
	// （擴張型與唯讀查看，BLOCK）＋ CapOrderFulfill 4（履行既有訂單義務）。
	// ⚠️ 設計書 §6.1.0(4) 與 §8.1.6(b) 的加總寫的是 24／12，那兩處是在端點 72
	// （S12 賣家進項發票，CapInvoiceManage）加入 §6.1.0(15) 端點總表之前寫的，
	// 沒有回灌。以端點總表為準 ⇒ 25／13。
	{"GET", "/api/v1/app/seller/invoices/complaints", authz.CapOrderFulfill},                                   // 端點 34
	{"GET", "/api/v1/app/seller/invoices/complaints/:complaint_id", authz.CapOrderFulfill},                     // 端點 35
	{"POST", "/api/v1/app/seller/invoices/complaints/:complaint_id/responses", authz.CapOrderFulfill},          // 端點 36
	{"GET", "/api/v1/app/seller/invoices/documents", authz.CapInvoiceManage},                                   // 端點 30
	{"GET", "/api/v1/app/seller/invoices/documents/:document_id", authz.CapInvoiceManage},                      // 端點 31
	{"GET", "/api/v1/app/seller/invoices/documents/:document_id/file/content", authz.CapInvoiceManage},         // 端點 PDF
	{"POST", "/api/v1/app/seller/invoices/documents/:document_id/notification_resends", authz.CapOrderFulfill}, // 端點 32
	{"GET", "/api/v1/app/seller/invoices/jobs/:job_id", authz.CapInvoiceManage},                                // 端點 33
	{"POST", "/api/v1/app/seller/invoices/jobs/:job_id/external_issuances", authz.CapInvoiceTaxRemedy},         // 端點 62
	{"GET", "/api/v1/app/seller/invoices/orders/:order_no/external_issuances", authz.CapInvoiceTaxRemedy},      // 端點 62R
	{"GET", "/api/v1/app/seller/invoices/overview", authz.CapInvoiceManage},                                    // 端點 16
	{"GET", "/api/v1/app/seller/invoices/provider_account", authz.CapInvoiceManage},                            // 端點 23
	{"POST", "/api/v1/app/seller/invoices/provider_account/activations", authz.CapInvoiceManage},               // 端點 26
	{"PUT", "/api/v1/app/seller/invoices/provider_account/credentials", authz.CapInvoiceManage},                // 端點 24
	{"POST", "/api/v1/app/seller/invoices/provider_account/rotations", authz.CapInvoiceManage},                 // 端點 27
	{"POST", "/api/v1/app/seller/invoices/provider_account/verifications", authz.CapInvoiceManage},             // 端點 25
	{"GET", "/api/v1/app/seller/invoices/receipt_authorization", authz.CapInvoiceManage},                       // 端點 28
	{"POST", "/api/v1/app/seller/invoices/receipt_authorization/confirmations", authz.CapInvoiceManage},        // 端點 29
	{"GET", "/api/v1/app/seller/invoices/received_documents", authz.CapInvoiceManage},                          // 端點 72
	{"GET", "/api/v1/app/seller/invoices/self_issue_attestations", authz.CapInvoiceTaxRemedy},                  // 端點 21
	{"POST", "/api/v1/app/seller/invoices/self_issue_attestations", authz.CapInvoiceTaxRemedy},                 // 端點 22
	{"GET", "/api/v1/app/seller/invoices/tax_profile", authz.CapInvoiceTaxRemedy},                              // 端點 17
	{"PUT", "/api/v1/app/seller/invoices/tax_profile/draft", authz.CapInvoiceTaxRemedy},                        // 端點 18
	{"POST", "/api/v1/app/seller/invoices/tax_profile/extension_requests", authz.CapInvoiceTaxRemedy},          // 端點 20
	{"POST", "/api/v1/app/seller/invoices/tax_profile/submissions", authz.CapInvoiceTaxRemedy},                 // 端點 19
}

// buildFullRouter builds the real route table (exercising the boot assertion).
func buildFullRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 文件檢視器要有設定才會註冊；規格涵蓋所有環境的路由聯集。
	docs := handler.NewDocs(config.DocsConfig{Username: "u", Password: "p", SignKey: "k"})
	SetupRoutes(r, &handler.Holder{Docs: docs}) // panics if the boot assertion fails
	return r
}

// The live registry must equal the 06-sourced expected table exactly.
func TestSellerCapabilityRegistryMatchesMatrix(t *testing.T) {
	buildFullRouter(t) // populates authz registry

	want := map[string]authz.Capability{}
	for _, rc := range expectedSellerRoutes {
		want[rc.method+" "+rc.path] = rc.cap
	}
	got := map[string]authz.Capability{}
	for _, rr := range authz.Registered() {
		got[rr.Method+" "+rr.Path] = rr.Cap
	}

	for k, wc := range want {
		if gc, ok := got[k]; !ok {
			t.Errorf("expected gated route not declared: %s (want %s)", k, wc)
		} else if gc != wc {
			t.Errorf("capability mismatch for %s: got %s, want %s (06 終判)", k, gc, wc)
		}
	}
	for k, gc := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("undocumented gated route %s [%s] — add to expected table + verify against 06 matrix", k, gc)
		}
	}
}

// The boot assertion must pass on the real, fully-assembled router: no wildcard
// under /app, no registry drift, every strict-prefix route declared.
func TestBootAssertionPassesOnRealRouter(t *testing.T) {
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("boot assertion panicked on the real router: %v", p)
		}
	}()
	buildFullRouter(t)
}

// No wildcard catch-all may exist under /api/v1/app (§3.7): the two dead proxies
// were removed so engine.Routes() is the complete attack surface.
func TestNoWildcardUnderApp(t *testing.T) {
	r := buildFullRouter(t)
	for _, ri := range r.Routes() {
		if underPrefix(ri.Path, appPrefix) && strings.Contains(ri.Path, "/*") {
			t.Errorf("wildcard route under /app must not exist: %s %s", ri.Method, ri.Path)
		}
	}
}

// The 3 ?view= routes are guarded by sellerViewGuard (not requireSeller), so they
// are mounted but intentionally NOT in the requireSeller registry.
func TestViewGuardRoutesMountedButNotInRegistry(t *testing.T) {
	r := buildFullRouter(t)
	viewRoutes := []string{
		"GET /api/v1/app/orders",
		"GET /api/v1/app/orders/:order_no",
		"GET /api/v1/app/coupons",
	}
	mounted := map[string]bool{}
	for _, ri := range r.Routes() {
		mounted[ri.Method+" "+ri.Path] = true
	}
	for _, vr := range viewRoutes {
		if !mounted[vr] {
			t.Errorf("view-guard route not mounted: %s", vr)
		}
		parts := strings.SplitN(vr, " ", 2)
		if _, ok := authz.LookupRoute(parts[0], parts[1]); ok {
			t.Errorf("view-guard route %s must NOT be in requireSeller registry (uses sellerViewGuard)", vr)
		}
	}
}

// The boot assertion must actually bite: a new seller route under a strict prefix
// that forgot requireSeller(cap) → panic fail-fast (46 §3.8-2b, 驗收 #5).
func TestBootAssertionPanicsOnUndeclaredSellerRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{}) // real router (passes)
	// A bare seller-area route under the strict /products prefix, WITHOUT requireSeller.
	r.GET(appPrefix+"/products/rogue_undeclared", func(c *gin.Context) {})

	defer func() {
		p := recover()
		if p == nil {
			t.Fatal("expected boot assertion to panic on an undeclared seller-area route")
		}
		if !strings.Contains(strings.ToLower(toStr(p)), "not declared") {
			t.Fatalf("panic did not mention the undeclared route: %v", p)
		}
	}()
	assertRouteCoverage(r)
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return ""
}

// The owner's single-post read is behind JWT authentication: with no
// credentials it is rejected before the capability check or the handler runs.
func TestSellerPostDetailRequiresAuthentication(t *testing.T) {
	r := buildFullRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/sellers/posts/77", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/v1/app/sellers/posts/77 with no credentials: status = %d, want 401", w.Code)
	}
}
