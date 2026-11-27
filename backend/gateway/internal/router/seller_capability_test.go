package router

import (
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

var expectedSellerRoutes = []routeCap{
	{"POST", "/api/v1/app/categories", authz.CapProductManage},
	{"GET", "/api/v1/app/coupon_templates", authz.CapMarketing},
	{"POST", "/api/v1/app/coupon_templates", authz.CapMarketing},
	{"GET", "/api/v1/app/coupon_templates/:template_id", authz.CapMarketing},
	{"GET", "/api/v1/app/categories/tree", authz.CapProductManage},
	{"GET", "/api/v1/app/hashtags", authz.CapProductManage},
	{"POST", "/api/v1/app/orders/:order_no/seller_cancellations", authz.CapOrderContract},
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
	{"GET", "/api/v1/app/seller/invoices/complaints", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/seller/invoices/complaints/:complaint_id", authz.CapOrderFulfill},
	{"POST", "/api/v1/app/seller/invoices/complaints/:complaint_id/responses", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/seller/invoices/documents", authz.CapInvoiceManage},
	{"GET", "/api/v1/app/seller/invoices/documents/:document_id", authz.CapInvoiceManage},
	{"GET", "/api/v1/app/seller/invoices/documents/:document_id/file/content", authz.CapInvoiceManage},
	{"POST", "/api/v1/app/seller/invoices/documents/:document_id/notification_resends", authz.CapOrderFulfill},
	{"GET", "/api/v1/app/seller/invoices/jobs/:job_id", authz.CapInvoiceManage},
	{"POST", "/api/v1/app/seller/invoices/jobs/:job_id/external_issuances", authz.CapInvoiceTaxRemedy},
	{"GET", "/api/v1/app/seller/invoices/orders/:order_no/external_issuances", authz.CapInvoiceTaxRemedy},
	{"GET", "/api/v1/app/seller/invoices/overview", authz.CapInvoiceManage},
	{"GET", "/api/v1/app/seller/invoices/provider_account", authz.CapInvoiceManage},
	{"POST", "/api/v1/app/seller/invoices/provider_account/activations", authz.CapInvoiceManage},
	{"PUT", "/api/v1/app/seller/invoices/provider_account/credentials", authz.CapInvoiceManage},
	{"POST", "/api/v1/app/seller/invoices/provider_account/rotations", authz.CapInvoiceManage},
	{"POST", "/api/v1/app/seller/invoices/provider_account/verifications", authz.CapInvoiceManage},
	{"GET", "/api/v1/app/seller/invoices/receipt_authorization", authz.CapInvoiceManage},
	{"POST", "/api/v1/app/seller/invoices/receipt_authorization/confirmations", authz.CapInvoiceManage},
	{"GET", "/api/v1/app/seller/invoices/received_documents", authz.CapInvoiceManage},
	{"GET", "/api/v1/app/seller/invoices/self_issue_attestations", authz.CapInvoiceTaxRemedy},
	{"POST", "/api/v1/app/seller/invoices/self_issue_attestations", authz.CapInvoiceTaxRemedy},
	{"GET", "/api/v1/app/seller/invoices/tax_profile", authz.CapInvoiceTaxRemedy},
	{"PUT", "/api/v1/app/seller/invoices/tax_profile/draft", authz.CapInvoiceTaxRemedy},
	{"POST", "/api/v1/app/seller/invoices/tax_profile/extension_requests", authz.CapInvoiceTaxRemedy},
	{"POST", "/api/v1/app/seller/invoices/tax_profile/submissions", authz.CapInvoiceTaxRemedy},
}

func buildFullRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	docs := handler.NewDocs(config.DocsConfig{Username: "u", Password: "p", SignKey: "k"})
	SetupRoutes(r, &handler.Holder{Docs: docs})
	return r
}

// The live registry must equal the 06-sourced expected table exactly.
func TestSellerCapabilityRegistryMatchesMatrix(t *testing.T) {
	buildFullRouter(t)

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
	SetupRoutes(r, &handler.Holder{})
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
