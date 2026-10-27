package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// The three counts are the arithmetic §6.1.1 / §13.1 G5 states: 40 endpoints total,
// of which 20 enter the delegated registry and 20 are named exceptions (six S
// endpoints, three summary/audit queries, two media upload sessions, the seven
// #2444 授權列治理端點, and the two §11.3 P1 跨工作階段查詢). 26 of the 40 skip
// requireAdmin(). Writing it as "registry 沒有的就略過" instead would let any future
// resource endpoint that forgot to register slip through with it.
func TestDelegatedRouteInventory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{})

	mounted := map[string]struct{}{}
	for _, ri := range r.Routes() {
		mounted[ri.Method+" "+ri.Path] = struct{}{}
	}

	if got := len(authz.RegisteredAdmin()); got != 20 {
		t.Errorf("delegatedHandle registry = %d 支，應為 20", got)
	}

	noAuth := registeredAdminNoAuth()
	if len(noAuth) != 26 {
		t.Errorf("credential-only 路由 = %d 支，A31 白名單恰為 26", len(noAuth))
	}
	if len(adminNoAuthWhitelist) != 26 {
		t.Errorf("adminNoAuthWhitelist = %d 條，恰為 26", len(adminNoAuthWhitelist))
	}
	if p := checkAdminNoAuthWhitelist(mounted, noAuth, adminNoAuthWhitelist); len(p) != 0 {
		t.Errorf("白名單雙向比對不通過：%v", p)
	}

	// 十四支 ADM host 端點必須留在 requireAdmin 那一組。#2444 那七支（授權列治理）
	// 掉進 credential-only 群組的後果比前五支更嚴重：那是「誰可以被代行」的寫入面，
	// 掛錯之後任何持有 operation credential 的人都能替自己擴權。§11.3 P1 那兩支同理，
	// 方向相反但一樣嚴重：它們列的是**全平台**的代行活動（其他 admin 的 actor、其他
	// 店家的 target），掛到 credential-only 就等於把跨店治理視野發給每一個代行憑證。
	admOnly := delegatedAdmHostRoutes()
	noAuthSet := map[string]struct{}{}
	for _, k := range noAuth {
		noAuthSet[k] = struct{}{}
	}
	for _, k := range admOnly {
		if _, ok := mounted[k]; !ok {
			t.Errorf("ADM host 端點未掛載：%s", k)
		}
		if _, ok := noAuthSet[k]; ok {
			t.Errorf("ADM host 端點被掛到 credential-only 群組：%s", k)
		}
	}

	if total := len(admOnly) + len(noAuth); total != 40 {
		t.Errorf("代行端點合計 %d 支，總表為 40", total)
	}
}

func TestDelegatedSessionSummaryRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		adminPrefix+"/delegated-operations/2701445580000001/summary", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("沒有管理員身分的摘要查詢 = %d，預期 401", w.Code)
	}
}

// delegatedAdmHostRoutes 是十四支 ADM host 端點。抽成函式讓限流登錄的那支測試共用同一
// 份清單——兩份手抄清單遲早只有一份被更新，而那時兩支測試都還是綠的。
func delegatedAdmHostRoutes() []string {
	return []string{
		"GET " + adminPrefix + "/sellers/:store_id/delegated-operation/preflight",
		"POST " + adminPrefix + "/delegated-operations",
		"POST " + adminPrefix + "/delegated-operations/:session_id/revoke",
		"GET " + adminPrefix + "/delegated-operations/:session_id/audit-logs",
		"GET " + adminPrefix + "/delegated-operations/:session_id/summary",
		"GET " + adminPrefix + "/delegated-operations",
		"GET " + adminPrefix + "/delegated-operations/audit-logs",
		"GET " + adminPrefix + "/delegated-operations/capabilities",
		"GET " + adminPrefix + "/delegated-operations/authorizations",
		"POST " + adminPrefix + "/delegated-operations/authorizations",
		"GET " + adminPrefix + "/delegated-operations/authorizations/:store_id",
		"PUT " + adminPrefix + "/delegated-operations/authorizations/:store_id",
		"POST " + adminPrefix + "/delegated-operations/authorizations/:store_id/revoke",
		"GET " + adminPrefix + "/delegated-operations/authorizations/:store_id/history",
	}
}

// 限流命中時要回代行專用的 9273（56 號 §6.1.0 行 2317），而限流跑在所有 handler 之前，
// 唯一問得到的問題是「c.FullPath() 是不是代行路由」。答案來自掛載時的登錄，本測試守住
// 「登錄 ≡ 掛載」——漏登錄的症狀是那一支端點被節流時回裸 429，前端拿到一個沒登錄的碼，
// 不會有任何東西轉紅。
//
// 前綴式判斷之所以不行也在這裡看得見：20 支資源端點掛在 /sellers/:store_id/ 這一段，
// 而同一段也可以掛非代行的端點，前綴規則分不出兩者，所以這裡逐條對照登錄與掛載。
func TestDelegatedRoutesAreAllRateLimitRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{})

	mounted := map[string]struct{}{}
	for _, ri := range r.Routes() {
		mounted[ri.Method+" "+ri.Path] = struct{}{}
	}

	want := map[string]struct{}{}
	for _, k := range delegatedAdmHostRoutes() {
		want[k] = struct{}{}
	}
	for _, k := range registeredAdminNoAuth() {
		want[k] = struct{}{}
	}
	if len(want) != 40 {
		t.Fatalf("代行端點合計 %d 支，總表為 40 —— 先修 TestDelegatedRouteInventory", len(want))
	}

	got := map[string]struct{}{}
	for _, k := range middleware.RegisteredDelegatedRoutes() {
		got[k] = struct{}{}
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("代行端點未登錄進限流判定：%s —— 它被節流時會回裸 429 而不是 9273", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("非代行端點被登錄成代行：%s —— 它被節流時會回 9273，前端會誤判成代行狀態出事", k)
		}
		if _, ok := mounted[k]; !ok {
			t.Errorf("登錄了一條沒有掛載的路由：%s", k)
		}
	}
}

// 註冊字串必須是 snake_case：`:storeId` 會與 routes_admin.go 既有的 `:store_id`
// 在同一路徑段衝突而讓 gin 啟動不了，而 metric 的 path label 取自 c.FullPath()。
func TestDelegatedRoutesUseSnakeCaseParams(t *testing.T) {
	for _, rr := range append(authz.RegisteredAdmin(), regsFrom(adminNoAuthWhitelist)...) {
		for _, seg := range strings.Split(rr.Path, "/") {
			if !strings.HasPrefix(seg, ":") {
				continue
			}
			if seg != strings.ToLower(seg) {
				t.Errorf("%s 的 param %s 不是 snake_case", rr.Path, seg)
			}
		}
	}
}

func regsFrom(entries []string) []authz.RegisteredAdminRoute {
	out := make([]authz.RegisteredAdminRoute, 0, len(entries))
	for _, e := range entries {
		if i := strings.IndexByte(e, ' '); i > 0 {
			out = append(out, authz.RegisteredAdminRoute{Method: e[:i], Path: e[i+1:]})
		}
	}
	return out
}

// ── 四條 boot 斷言的鑑別力：每一條都用違規輸入證明它會轉紅 ────────────────────

func TestBootCheckD_CapabilityDeclared(t *testing.T) {
	clean := make([]authz.RegisteredAdminRoute, 0, 2)
	clean = append(clean, authz.RegisteredAdminRoute{Method: "GET", Path: adminPrefix + "/sellers/:store_id/products", Cap: authz.AdminCapProductsRead})
	if p := checkAdminCapabilityDeclared(clean); len(p) != 0 {
		t.Fatalf("正常註冊表不該有 finding：%v", p)
	}

	// dirty must be a genuinely independent slice, not clean grown in place: this
	// test's whole point is comparing "clean" against "clean + one bad route", so
	// clean's backing array must stay untouched for the earlier assertion to mean
	// anything on a re-run.
	dirty := append([]authz.RegisteredAdminRoute{}, clean...)
	dirty = append(dirty, authz.RegisteredAdminRoute{Method: "POST", Path: adminPrefix + "/sellers/:store_id/posts", Cap: ""})
	p := checkAdminCapabilityDeclared(dirty)
	if len(p) != 1 {
		t.Fatalf("漏宣告 capability 應產生 1 個 finding，實得 %d：%v", len(p), p)
	}
	if !strings.Contains(p[0], "未宣告 capability") || !strings.Contains(p[0], "/posts") {
		t.Errorf("訊息未指出命中原因與路由：%s", p[0])
	}
}

func TestBootCheckE_ForbiddenWords(t *testing.T) {
	clean := make([]authz.RegisteredAdminRoute, 0, 2)
	clean = append(clean, authz.RegisteredAdminRoute{Method: "GET", Path: adminPrefix + "/sellers/:store_id/orders", Cap: authz.AdminCapOrdersRead})
	if p := checkAdminPathForbiddenWords(clean); len(p) != 0 {
		t.Fatalf("正常註冊表不該有 finding：%v", p)
	}

	// param 名必須是 :store_id——換名會先炸在 gin 的 wildcard 衝突而不是禁字斷言，
	// 給出一個看起來正確的假紅燈。
	dirty := append([]authz.RegisteredAdminRoute{}, clean...)
	dirty = append(dirty, authz.RegisteredAdminRoute{
		Method: "POST", Path: adminPrefix + "/sellers/:store_id/shipments", Cap: authz.AdminCapOrdersRead,
	})
	p := checkAdminPathForbiddenWords(dirty)
	if len(p) != 1 {
		t.Fatalf("禁字應產生 1 個 finding，實得 %d：%v", len(p), p)
	}
	if !strings.Contains(p[0], `"shipment"`) {
		t.Errorf("訊息未指出命中的是哪一個禁字：%s", p[0])
	}

	// 十個禁字每一個都要有鑑別力，不能只有第一個生效。
	for _, w := range delegatedPathForbiddenWords {
		route := []authz.RegisteredAdminRoute{{Method: "GET", Path: adminPrefix + "/sellers/:store_id/" + w + "s", Cap: authz.AdminCapOrdersRead}}
		if got := checkAdminPathForbiddenWords(route); len(got) == 0 {
			t.Errorf("禁字 %q 未被攔下", w)
		}
	}
}

func TestBootCheckF_NoWildcard(t *testing.T) {
	clean := make([]gin.RouteInfo, 0, 2)
	clean = append(clean, gin.RouteInfo{Method: "GET", Path: adminPrefix + "/sellers/:store_id/products"})
	if p := checkAdminNoWildcard(clean); len(p) != 0 {
		t.Fatalf("正常路由表不該有 finding：%v", p)
	}

	dirty := append([]gin.RouteInfo{}, clean...)
	dirty = append(dirty, gin.RouteInfo{Method: "GET", Path: adminPrefix + "/*path"})
	p := checkAdminNoWildcard(dirty)
	if len(p) != 1 {
		t.Fatalf("wildcard 應產生 1 個 finding，實得 %d：%v", len(p), p)
	}
	if !strings.Contains(p[0], "wildcard") || !strings.Contains(p[0], "攻擊面") {
		t.Errorf("訊息未指出命中原因：%s", p[0])
	}
}

func TestBootCheckG_NoAuthWhitelist(t *testing.T) {
	whitelist := []string{"GET " + adminPrefix + "/delegated-operations/current"}
	mounted := map[string]struct{}{whitelist[0]: {}}

	if p := checkAdminNoAuthWhitelist(mounted, whitelist, whitelist); len(p) != 0 {
		t.Fatalf("一致時不該有 finding：%v", p)
	}

	// 方向一：白名單以外多出一支略過 requireAdmin 的路由。
	rogue := "POST " + adminPrefix + "/sellers/:store_id/suspend"
	mountedRogue := map[string]struct{}{whitelist[0]: {}, rogue: {}}
	p := checkAdminNoAuthWhitelist(mountedRogue, append([]string{rogue}, whitelist...), whitelist)
	if len(p) != 1 || !strings.Contains(p[0], "未列入 A31 白名單") || !strings.Contains(p[0], "/suspend") {
		t.Fatalf("多出的 credential-only 路由未被指名：%v", p)
	}

	// 方向二：白名單內有一支沒被掛載。
	p = checkAdminNoAuthWhitelist(map[string]struct{}{}, nil, whitelist)
	if len(p) != 1 || !strings.Contains(p[0], "未被掛載") {
		t.Fatalf("白名單殘留項未被指名：%v", p)
	}
}

// 端到端：真實路由表 + 一支登記在 credential-only 群組卻不在白名單的路由 → panic。
// 直接操作 registry（同 package）而不是掛真路由，才不會把污染留給其他測試。
func TestAssertRouteCoveragePanicsOnRogueCredentialOnlyRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{}) // 乾淨的真實路由表：不 panic

	rogue := "POST " + adminPrefix + "/sellers/:store_id/payouts"
	adminNoAuthMu.Lock()
	adminNoAuthRegistry[rogue] = struct{}{}
	adminNoAuthMu.Unlock()
	defer func() {
		adminNoAuthMu.Lock()
		delete(adminNoAuthRegistry, rogue)
		adminNoAuthMu.Unlock()

		p := recover()
		if p == nil {
			t.Fatal("略過 requireAdmin 且不在白名單的路由必須讓 boot 斷言 panic")
		}
		if !strings.Contains(toStr(p), "未列入 A31 白名單") {
			t.Fatalf("panic 訊息未指出命中原因：%v", p)
		}
	}()
	assertRouteCoverage(r)
}
