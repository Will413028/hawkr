package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/gin-gonic/gin"
)

// 代行資源端點的真實形狀：路徑第二段 sellers/:store_id 與同段的非代行路由共用，所以「是不是代行」
// 只有登錄制答得準。adminPlainRoute 是測試自己掛的非代行示意路由。
const (
	delegatedProductsRoute = "/api/v1/admin/sellers/:store_id/products"
	delegatedProductsURL   = "/api/v1/admin/sellers/2685914682/products"
	adminPlainRoute        = "/api/v1/admin/sellers/:store_id/plain"
	adminPlainURL          = "/api/v1/admin/sellers/2685914682/plain"
)

// rlEngineDelegated mounts one delegated route and one plain admin route that share
// the /sellers/:store_id/ segment, plus a status knob so the auth-failure axis can be
// driven directly.
func rlEngineDelegated(limiter *ratelimit.Limiter, policy middleware.RateLimitPolicy, status *int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TieredRateLimiter(limiter, policy))
	h := func(c *gin.Context) { c.String(*status, "") }
	r.GET(delegatedProductsRoute, h)
	r.POST(adminPlainRoute, h)
	return r
}

func envelopeCode(t *testing.T, body []byte) int32 {
	t.Helper()
	var env struct {
		Code int32 `json:"code"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("回應不是合法 envelope：%v（body=%s）", err, body)
	}
	return env.Code
}

func doRL(r *gin.Engine, method, url string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	req.RemoteAddr = "203.0.113.77:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 56 號 §6.1.0 行 2317：代行沿用既有 tierDefault，而「9273 即該既有 tier 觸發時回傳的
// 業務碼」。實作原本一路送裸 429，於是 §6.5.1 登錄的 9273 在整個 repo 沒有 emitter，
// 前端為它寫的「倒數後安全重試、不得重複 mutation」分支永遠不會被觸發。
//
// ⚠️ HTTP 仍必須是 429：bizhttp.go 對 9273 登錄的就是 429，改動的只有業務碼。
func TestRateLimiter_DelegatedRouteReturnsDelegationRateLimited(t *testing.T) {
	middleware.RegisterDelegatedRoute(http.MethodGet, delegatedProductsRoute)

	limiter, _ := newLimiter(t, 1000)
	status := http.StatusOK
	policy := middleware.RateLimitPolicy{
		ModeratePerWindow:    1000,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		DefaultPerWindow: 2,
		Window:           time.Minute, FallbackCapacity: 1000,
	}
	r := rlEngineDelegated(limiter, policy, &status)

	for i := 1; i <= 2; i++ {
		if w := doRL(r, http.MethodGet, delegatedProductsURL); w.Code != http.StatusOK {
			t.Fatalf("第 %d 次請求 = %d，額度內應為 200", i, w.Code)
		}
	}
	w := doRL(r, http.MethodGet, delegatedProductsURL)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("超額後 HTTP = %d, want 429", w.Code)
	}
	if got := envelopeCode(t, w.Body.Bytes()); got != bizcode.DelegationRateLimited {
		t.Errorf("業務碼 = %d, want %d（DELEGATION_RATE_LIMITED，§6.1.0 行 2317）",
			got, bizcode.DelegationRateLimited)
	}
}

// 反向：同一段路徑下的非代行管理端點不得被順手改碼。9273 的語意是「代行工作階段被
// 節流」，發到一支與代行無關的端點上會讓前端誤判成代行狀態出了事。
func TestRateLimiter_NonDelegatedAdminRouteKeepsPlain429(t *testing.T) {
	middleware.RegisterDelegatedRoute(http.MethodGet, delegatedProductsRoute)

	limiter, _ := newLimiter(t, 1000)
	status := http.StatusOK
	policy := middleware.RateLimitPolicy{
		ModeratePerWindow:    1000,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		DefaultPerWindow: 1,
		Window:           time.Minute, FallbackCapacity: 1000,
	}
	r := rlEngineDelegated(limiter, policy, &status)

	if w := doRL(r, http.MethodPost, adminPlainURL); w.Code != http.StatusOK {
		t.Fatalf("第 1 次請求 = %d，額度內應為 200", w.Code)
	}
	w := doRL(r, http.MethodPost, adminPlainURL)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("超額後 HTTP = %d, want 429", w.Code)
	}
	if got := envelopeCode(t, w.Body.Bytes()); got != 429 {
		t.Errorf("業務碼 = %d, want 429 —— /sellers/:store_id/ 這一段同時住著代行與非代行"+
			"端點，前綴式判斷會把 /plain 一起改掉", got)
	}
}

// auth-failure 軸與 tier 額度是兩個不同的軸，但對呼叫端的意思相同（被節流了，倒數後
// 再來）。代行路由上兩條 429 分支共用 9273，否則同一組端點的同一個 HTTP status 會帶
// 兩種業務碼，其中一種還沒登錄。要分辨成因用 message 與 block log 的 reason 欄。
func TestRateLimiter_DelegatedAuthFailureBranchAlsoReturns9273(t *testing.T) {
	middleware.RegisterDelegatedRoute(http.MethodGet, delegatedProductsRoute)

	limiter, _ := newLimiter(t, 1000)
	status := http.StatusUnauthorized
	policy := middleware.RateLimitPolicy{
		ModeratePerWindow:    1000,
		ModerateSSRPerWindow: 1000, ModerateCIPerWindow: 1000,
		DefaultPerWindow:     1000,
		AuthFailurePerWindow: 2,
		Window:               time.Minute, FallbackCapacity: 1000,
	}
	r := rlEngineDelegated(limiter, policy, &status)

	// 工作區憑證失效後仍在輪詢的形狀：連續 401 把 auth-failure 計數推到門檻。
	for i := 1; i <= 2; i++ {
		if w := doRL(r, http.MethodGet, delegatedProductsURL); w.Code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次請求 = %d，門檻內應原樣回 401", i, w.Code)
		}
	}
	w := doRL(r, http.MethodGet, delegatedProductsURL)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("超過 auth-failure 門檻後 HTTP = %d, want 429", w.Code)
	}
	if got := envelopeCode(t, w.Body.Bytes()); got != bizcode.DelegationRateLimited {
		t.Errorf("業務碼 = %d, want %d", got, bizcode.DelegationRateLimited)
	}
}

// 未匹配任何路由（404，FullPath 為空）不得被判成代行——那是 raw URL 判斷才會犯的錯。
func TestIsDelegatedRoute_UnmatchedRouteIsNotDelegated(t *testing.T) {
	middleware.RegisterDelegatedRoute(http.MethodGet, delegatedProductsRoute)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, delegatedProductsURL, nil)
	if middleware.IsDelegatedRoute(c) {
		t.Error("未經路由匹配的請求（FullPath 為空）被判成代行路由")
	}
}
