package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// 能力集合是單體帶來的輸入（seller 模組推導）；測試以明確的清單當輸入，不在 gateway 重算。
var (
	suspendedCaps = []authz.Capability{
		authz.CapAccountSelfRead, authz.CapOrderFulfill, authz.CapOrderContract, authz.CapChatSend,
		authz.CapChatRead, authz.CapNotification, authz.CapContentContract, authz.CapAccountClose, authz.CapInvoiceTaxRemedy,
	}
	pendingDeletionCaps = []authz.Capability{
		authz.CapAccountSelfRead, authz.CapOrderFulfill, authz.CapOrderContract, authz.CapChatSend,
		authz.CapChatRead, authz.CapNotification, authz.CapContentContract, authz.CapInvoiceTaxRemedy,
	}
)

func capsForReason(reason string) []authz.Capability {
	if reason != "" {
		return suspendedCaps
	}
	return authz.AllCapabilities
}

func TestSellerGuardBehavior(t *testing.T) {
	middleware.SetRedisClient(nil)
	reason := ""

	prevSkip := middleware.SkipRoleCheck
	middleware.SkipRoleCheck = false // exercise the real role + suspension checks
	t.Cleanup(func() { middleware.SkipRoleCheck = prevSkip })

	gin.SetMode(gin.TestMode)
	asIdentity := func(accountID int64, accountType int) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("accountId", accountID)
			c.Set("accountType", accountType)
			c.Set("sellerSuspended", reason)
			c.Set("sellerCapabilities", capsForReason(reason))
			c.Next()
		}
	}
	echo := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"suspHeader": c.Request.Header.Get(headerSellerSuspended)})
	}
	build := func(accountID int64, accountType int) *gin.Engine {
		r := gin.New()
		r.Use(asIdentity(accountID, accountType))
		r.GET("/block", requireSeller(authz.CapProductManage), echo)  // BLOCK-if-suspended cap
		r.GET("/white", requireSeller(authz.CapOrderFulfill), echo)   // 白名單 cap (skips Redis)
		r.GET("/view", sellerViewGuard(authz.CapProductManage), echo) // BLOCK cap to prove deny-path delegation
		r.GET("/inject", injectSellerSuspension(), echo)
		return r
	}
	do := func(r *gin.Engine, target string, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	const sellerID = int64(2683382637)
	seller := build(sellerID, middleware.AccountTypeSeller)
	reason = ""
	if w := do(seller, "/block", nil); w.Code != http.StatusOK {
		t.Fatalf("active /block: want 200, got %d %s", w.Code, w.Body)
	}
	if w := do(seller, "/white", nil); w.Code != http.StatusOK {
		t.Fatalf("active /white: want 200, got %d", w.Code)
	}
	if w := do(seller, "/view?view=selling", nil); w.Code != http.StatusOK {
		t.Fatalf("active view=selling: want 200, got %d", w.Code)
	}
	if w := do(seller, "/inject", nil); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "9601") {
		t.Fatalf("active /inject: want 200 with empty suspHeader, got %d %s", w.Code, w.Body)
	}
	reason = "9601"
	if w := do(seller, "/block", nil); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "9240") {
		t.Fatalf("suspended /block: want 409 with code 9240, got %d %s", w.Code, w.Body)
	}
	if w := do(seller, "/white", nil); w.Code != http.StatusOK {
		t.Fatalf("suspended /white (whitelist): want 200, got %d", w.Code)
	}
	if w := do(seller, "/view?view=selling", nil); w.Code != http.StatusConflict {
		t.Fatalf("suspended view=selling (BLOCK cap): want 409, got %d", w.Code)
	}
	if w := do(seller, "/view?view=buyer", nil); w.Code != http.StatusOK {
		t.Fatalf("suspended view=buyer: must bypass guard, want 200, got %d", w.Code)
	}
	if w := do(seller, "/inject", map[string]string{headerSellerSuspended: "SPOOF"}); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), "9601") || strings.Contains(w.Body.String(), "SPOOF") {
		t.Fatalf("suspended /inject: want reason 9601 with spoof stripped, got %s", w.Body)
	}
	buyer := build(int64(2608240571), middleware.AccountTypeBuyer)
	if w := do(buyer, "/block", nil); w.Code != http.StatusForbidden {
		t.Fatalf("buyer /block: want 403, got %d", w.Code)
	}
	if w := do(buyer, "/view?view=buyer", nil); w.Code != http.StatusOK {
		t.Fatalf("buyer view=buyer: want 200, got %d", w.Code)
	}

}
func TestSellerSuspensionOnlyGuard(t *testing.T) {
	middleware.SetRedisClient(nil)
	reason := ""

	prevSkip := middleware.SkipRoleCheck
	middleware.SkipRoleCheck = false
	t.Cleanup(func() { middleware.SkipRoleCheck = prevSkip })

	gin.SetMode(gin.TestMode)

	const (
		onboardingLogin = int64(111) // accountType=1（重新登入前）, has seller record
		suspendedLogin  = int64(222)
		pureBuyer       = int64(333) // no seller record at all
	)
	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"reached": "handler"}) }
	build := func(accountID int64, accountType int) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("accountId", accountID)
			c.Set("accountType", accountType)
			c.Set("sellerSuspended", reason)
			c.Set("sellerCapabilities", capsForReason(reason))
			c.Next()
		})
		r.PUT("/me", requireSeller(authz.CapStorefrontWrite), ok)
		r.POST("/coupon_templates", requireSeller(authz.CapMarketing), ok)
		r.POST("/coupon_templates/:id/revoke", ok)
		return r
	}
	do := func(r *gin.Engine, method, target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	reason = ""
	onb := build(onboardingLogin, middleware.AccountTypeBuyer)
	if w := do(onb, http.MethodPut, "/me"); w.Code != http.StatusForbidden {
		t.Fatalf("no seller role PUT /me: want 403, got %d %s", w.Code, w.Body)
	}
	reason = "9601"
	sus := build(suspendedLogin, middleware.AccountTypeSeller)
	if w := do(sus, http.MethodPut, "/me"); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "9240") {
		t.Fatalf("suspended PUT /me: want 409+9240, got %d %s", w.Code, w.Body)
	}
	if w := do(sus, http.MethodPost, "/coupon_templates"); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "9240") {
		t.Fatalf("suspended 建券: want 409+9240, got %d %s", w.Code, w.Body)
	}
	if w := do(sus, http.MethodPost, "/coupon_templates/42/revoke"); w.Code != http.StatusOK {
		t.Fatalf("suspended Revoke: 收縮型必放行, want 200, got %d", w.Code)
	}
	buyer := build(pureBuyer, middleware.AccountTypeBuyer)
	if w := do(buyer, http.MethodPut, "/me"); w.Code != http.StatusForbidden {
		t.Fatalf("pure buyer PUT /me: want 403, got %d %s", w.Code, w.Body)
	}

}
func TestSuspensionMetadataPropagation(t *testing.T) {
	middleware.SetRedisClient(nil)
	reason := ""

	prevSkip := middleware.SkipRoleCheck
	middleware.SkipRoleCheck = false
	t.Cleanup(func() { middleware.SkipRoleCheck = prevSkip })

	gin.SetMode(gin.TestMode)
	const sellerID = int64(777)
	var gotMD metadata.MD
	interceptor := grpcclient.AuthMetadataInterceptor()
	capturingHandler := func(c *gin.Context) {
		gotMD = nil
		_ = interceptor(c.Request.Context(), "/im.IMService/SendMessage", nil, nil, nil,
			func(ctx context.Context, method string, req, reply interface{},
				cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				gotMD, _ = metadata.FromOutgoingContext(ctx)
				return nil
			})
		c.JSON(http.StatusOK, gin.H{"header": c.Request.Header.Get(headerSellerSuspended)})
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accountId", sellerID)
		c.Set("accountType", middleware.AccountTypeSeller)
		c.Set("sellerSuspended", reason)
		c.Set("sellerCapabilities", capsForReason(reason))
		c.Next()
	})
	r.POST("/chat/messages", injectSellerSuspension(), capturingHandler)

	send := func(spoof string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/chat/messages", nil)
		if spoof != "" {
			req.Header.Set(headerSellerSuspended, spoof)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	reason = ""
	send("")
	if v := gotMD.Get("x-seller-suspended"); len(v) != 0 {
		t.Fatalf("no suspension: metadata must NOT carry x-seller-suspended, got %v", v)
	}
	w := send("FORGED_REASON")
	if v := gotMD.Get("x-seller-suspended"); len(v) != 0 {
		t.Fatalf("spoofed header must be stripped, metadata got %v", v)
	}
	if strings.Contains(w.Body.String(), "FORGED_REASON") {
		t.Fatalf("spoofed header must not survive to handler, body %s", w.Body)
	}
	reason = "9601"
	send("")
	if v := gotMD.Get("x-seller-suspended"); len(v) != 1 || v[0] != "9601" {
		t.Fatalf("suspended: metadata must carry x-seller-suspended=9601, got %v", v)
	}
	send("FORGED_REASON")
	if v := gotMD.Get("x-seller-suspended"); len(v) != 1 || v[0] != "9601" {
		t.Fatalf("suspended+spoof: only middleware value may propagate, got %v", v)
	}
}

func TestRequireSellerChecksOnlyCapabilityMembership(t *testing.T) {
	prevSkip := middleware.SkipRoleCheck
	middleware.SkipRoleCheck = false
	t.Cleanup(func() { middleware.SkipRoleCheck = prevSkip })

	gin.SetMode(gin.TestMode)
	var (
		caps      []authz.Capability
		suspended string
	)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accountType", middleware.AccountTypeSeller)
		c.Set("sellerSuspended", suspended)
		if caps != nil {
			c.Set("sellerCapabilities", caps)
		}
		c.Next()
	})
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/business", requireSeller(authz.CapProductManage), ok)
	r.GET("/fulfill", requireSeller(authz.CapOrderFulfill), ok)
	r.GET("/shipping-setup", requireSeller(authz.CapShippingSetup), ok)
	do := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	caps, suspended = pendingDeletionCaps, ""
	if w := do("/business"); w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "9240") {
		t.Fatalf("等待刪除 經營類：want 403 且不是 9240，got %d %s", w.Code, w.Body)
	}
	if w := do("/fulfill"); w.Code != http.StatusOK {
		t.Fatalf("等待刪除 履約類：want 200, got %d %s", w.Code, w.Body)
	}

	caps, suspended = suspendedCaps, "SUSPENDED_GOVERNANCE"
	if w := do("/business"); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "9240") {
		t.Fatalf("停權 經營類：want 409+9240, got %d %s", w.Code, w.Body)
	}

	caps, suspended = nil, ""
	if w := do("/fulfill"); w.Code != http.StatusForbidden {
		t.Fatalf("沒有能力集合：want 403（fail-closed），got %d %s", w.Code, w.Body)
	}

	// 能力集合是唯一依據：即使帶了停權原因，集合有這個能力就放行。
	caps, suspended = authz.AllCapabilities, "9601"
	if w := do("/business"); w.Code != http.StatusOK {
		t.Fatalf("集合含 cap：want 200, got %d %s", w.Code, w.Body)
	}
}

// 物流建店要 CAP_SHIPPING_SETUP：有 CAP_PRODUCT_MANAGE 但單體沒有授予它（KYC 流程未核准）時被擋下，回 403（沒有停權，
// 不是 9240）；授予之後放行。
func TestShippingSetupIsDeniedWithoutItsOwnCapability(t *testing.T) {
	prevSkip := middleware.SkipRoleCheck
	middleware.SkipRoleCheck = false
	t.Cleanup(func() { middleware.SkipRoleCheck = prevSkip })

	gin.SetMode(gin.TestMode)
	var caps []authz.Capability
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accountType", middleware.AccountTypeSeller)
		c.Set("sellerSuspended", "")
		c.Set("sellerCapabilities", caps)
		c.Next()
	})
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/read", requireSeller(authz.CapProductManage), ok)
	r.GET("/setup", requireSeller(authz.CapShippingSetup), ok)
	do := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	caps = []authz.Capability{authz.CapProductManage, authz.CapOrderFulfill}
	if w := do("/setup"); w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "9240") {
		t.Fatalf("KYC 未核准：want 403 且不是 9240，got %d %s", w.Code, w.Body)
	}
	if w := do("/read"); w.Code != http.StatusOK {
		t.Fatalf("唯讀仍是 CAP_PRODUCT_MANAGE：want 200, got %d %s", w.Code, w.Body)
	}
	caps = append(caps, authz.CapShippingSetup)
	if w := do("/setup"); w.Code != http.StatusOK {
		t.Fatalf("已授予：want 200, got %d %s", w.Code, w.Body)
	}
}
