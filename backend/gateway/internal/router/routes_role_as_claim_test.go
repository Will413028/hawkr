package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// A buyer+seller account in "seller" view (單體解析 X-Active-Role 後送 X-Login-Role:2) must pass BOTH
// requireSeller and requireBuyer (authz checks the role set), and downstream
// routes must receive X-Account-Id=909 (ownership unaffected by active role) —
// the structural #832 fix: no role-mismatch 401/403.
func TestRoleAsClaim_DualRoleSession_PassesAllGates(t *testing.T) {
	prevSkip := middleware.SkipRoleCheck
	middleware.SkipRoleCheck = false
	t.Cleanup(func() { middleware.SkipRoleCheck = prevSkip })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	app := r.Group(APIV1Path).Group(AppPath)

	echo := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"accountId": c.Request.Header.Get("X-Account-Id"),
			"loginRole": c.Request.Header.Get("X-Login-Role"),
		})
	}
	app.GET("/seller-only", middleware.JWTAuth(), requireSeller(authz.CapOrderFulfill), echo)
	app.GET("/buyer-only", middleware.JWTAuth(), requireBuyer(), echo)
	app.GET("/payment-like", middleware.JWTAuth(), echo)

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
		req.Header.Set("X-Account-Id", "909")
		req.Header.Set("X-Account-Type", "3")
		req.Header.Set("X-Login-Role", "2")
		req.Header.Set("X-Store-Id", "4242")
		req.Header.Set("X-Seller-Capabilities", string(authz.CapOrderFulfill))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := get("/app/seller-only"); w.Code != http.StatusOK {
		t.Fatalf("seller-only: dual-role account must pass, got %d: %s", w.Code, w.Body.String())
	}
	bw := get("/app/buyer-only")
	if bw.Code != http.StatusOK {
		t.Fatalf("buyer-only: dual-role account in seller view must STILL pass (role-as-claim), got %d: %s", bw.Code, bw.Body.String())
	}
	pw := get("/app/payment-like")
	if pw.Code != http.StatusOK {
		t.Fatalf("payment-like: expected 200, got %d", pw.Code)
	}
	if !bytes.Contains(pw.Body.Bytes(), []byte(`"accountId":"909"`)) {
		t.Fatalf("downstream X-Account-Id must be 909 regardless of active role, got %s", pw.Body.String())
	}
	if !bytes.Contains(pw.Body.Bytes(), []byte(`"loginRole":"2"`)) {
		t.Fatalf("downstream X-Login-Role must reflect the seller view (2), got %s", pw.Body.String())
	}
}
