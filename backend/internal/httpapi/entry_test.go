package httpapi

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// fakeAuth 以 token 決定結果：good 是賣家（每個帳號都是買家），new 是這個請求剛建立的帳號（沒有另外申請的角色），admin 是管理員，
// dead 是無效的 token，denied 是不能登入後台的使用者，boom 是資料庫答不出來。
type fakeAuth struct{}

var entryNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func (fakeAuth) Authenticate(_ context.Context, token string) (identity.Authenticated, error) {
	switch token {
	case "good":
		return identity.Authenticated{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: 7, Roles: []identity.Role{identity.RoleSeller}}}, nil
	case "new":
		return identity.Authenticated{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: 8}}, nil
	case "admin":
		return identity.Authenticated{Principal: identity.Principal{Kind: identity.PrincipalAdmin, ID: 3}}, nil
	case "denied":
		return identity.Authenticated{}, apperr.PermissionDenied("這個 Google 帳號不能登入後台")
	case "denied-bare":
		return identity.Authenticated{}, apperr.PermissionDenied("")
	case "boom":
		return identity.Authenticated{}, errors.New("db down")
	}
	return identity.Authenticated{}, apperr.Unauthenticated("登入已失效")
}

// caller 回報 ctx 裡的呼叫者與限流身分，給中介層的測試看。
var caller = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	p, ok := rpc.PrincipalFrom(r.Context())
	_, _ = fmt.Fprintf(w, "principal=%v ok=%v subject=%s", p.ID, ok, rpc.SubjectFrom(r.Context()))
})

const testOrigin = "https://web.example"

func entry(t *testing.T, legacy http.Handler, services ...service) http.Handler {
	t.Helper()
	h, err := assemble(Deps{
		Identity: nil, Now: func() time.Time { return entryNow },
		Origins:    []string{testOrigin},
		OriginAuth: OriginAuth{Main: []byte("main-secret"), CI: []byte("ci-secret")},
		Legacy:     legacy, LegacyPrefixes: []string{"/api/v1/", "/payment/"},
		Logger: slog.New(slog.DiscardHandler),
	}, services)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// entryWithAuth 與 entry 相同，但認證用 fakeAuth（Deps.Identity 是具體型別，所以直接包 withSession 的那一層）。
func entryWithAuth(t *testing.T, legacy http.Handler) http.Handler {
	t.Helper()
	inner := http.NewServeMux()
	inner.Handle("/api/v1/", legacy)
	inner.Handle("/hawkr.identity.v1alpha1.IdentityAppService/", legacy)
	inner.Handle("/hawkr.buyer.v1alpha1.BuyerAppService/", legacy)
	return withSession(fakeAuth{}, slog.New(slog.DiscardHandler), inner)
}

func do(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestSessionMiddleware(t *testing.T) {
	h := entryWithAuth(t, caller)
	withCookie := func(token string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/app/buyers/me", nil)
		r.AddCookie(&http.Cookie{Name: rpc.SessionCookie, Value: token})
		return r
	}
	withBearer := func(token string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/app/buyers/me", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		return r
	}

	for _, r := range []*http.Request{withCookie("good"), withBearer("good")} {
		if rec := do(h, r); !strings.Contains(rec.Body.String(), "principal=7 ok=true") || rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("有效的 token = %q，Set-Cookie %q", rec.Body, rec.Header().Get("Set-Cookie"))
		}
	}
	// Connect 不讀 X-Active-Role（ADR 2026-10-07-connect-without-perspective）：帶不帶它，認證的結果都一樣。
	plain := do(h, withBearer("good")).Body.String()
	r := withBearer("good")
	r.Header.Set("X-Active-Role", "SELLER")
	if rec := do(h, r); rec.Body.String() != plain {
		t.Fatalf("帶 X-Active-Role 的結果 = %q，不帶是 %q", rec.Body, plain)
	}
	// 兩者都帶時以 Bearer 為準（後台的 BFF 帶 Bearer，瀏覽器的 cookie 可能也在）。
	r = withCookie("dead")
	r.Header.Set("Authorization", "Bearer good")
	if rec := do(h, r); !strings.Contains(rec.Body.String(), "ok=true") {
		t.Fatalf("Bearer 應該優先：%q", rec.Body)
	}

	// 無效的 token：以訪客繼續，不改 cookie。
	for _, r := range []*http.Request{withCookie("dead"), withBearer("dead")} {
		if rec := do(h, r); !strings.Contains(rec.Body.String(), "ok=false") || rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("無效的 token = %q，Set-Cookie %q", rec.Body, rec.Header().Get("Set-Cookie"))
		}
	}

	// #2511：憑證只從 session cookie 與 Authorization: Bearer 讀，query、token header、其他 scheme、舊的 cookie 都不讀。
	for _, r := range []*http.Request{
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/app/buyers/me?token=good", nil),
		func() *http.Request {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/app/buyers/me", nil)
			r.Header.Set("token", "good")
			r.Header.Set("Authorization", "good")
			r.AddCookie(&http.Cookie{Name: "__session", Value: "good"})
			return r
		}(),
	} {
		if rec := do(h, r); !strings.Contains(rec.Body.String(), "ok=false") {
			t.Fatalf("%s 被當成憑證：%q", r.URL, rec.Body)
		}
	}

	// 建帳事件已建立買家；入口不解析 buyer_id（buyer handler 自己查），角色 hint 不會觸發寫入。
	newBuyerRequest := func() *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hawkr.buyer.v1alpha1.BuyerAppService/GetProfile", nil)
		r.Header.Set("Authorization", "Bearer new")
		return r
	}
	if rec := do(h, newBuyerRequest()); !strings.Contains(rec.Body.String(), "principal=8 ok=true") {
		t.Fatalf("新帳號 = %q", rec.Body)
	}
	r = newBuyerRequest()
	r.Header.Set("X-Active-Role", "SELLER")
	do(h, r)
	do(h, withBearer("good")) // 不是新帳號：不在中介層建

	// 後台的使用者不是允許的帳號：回 403 與模組的文字，不以訪客繼續。
	rec := do(h, withBearer("denied"))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "不能登入後台") {
		t.Fatalf("REST 的 403 = %d %q", rec.Code, rec.Body)
	}
	r = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hawkr.identity.v1alpha1.IdentityAppService/GetSession", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer denied")
	if rec := do(h, r); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"code":"permission_denied"`) {
		t.Fatalf("Connect 的 403 = %d %q", rec.Code, rec.Body)
	}

	// 模組沒給文字時：REST 與 Connect 都用共用文案目錄的 PERMISSION_DENIED（傳輸層同一個 code 只有一份文字）。
	want := rpc.CommonText(t.Context(), connect.CodePermissionDenied).Text
	if want != "無權操作" {
		t.Fatalf("目錄的 PERMISSION_DENIED = %q", want)
	}
	if rec := do(h, withBearer("denied-bare")); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"message":"`+want+`"`) {
		t.Fatalf("REST 的 403（模組沒給文字）= %d %q", rec.Code, rec.Body)
	}
	r = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hawkr.identity.v1alpha1.IdentityAppService/GetSession", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer denied-bare")
	if rec := do(h, r); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("Connect 的 403（模組沒給文字）= %d %q", rec.Code, rec.Body)
	}

	// 資料庫答不出來時回 503，不當成訪客（否則 client 會把使用者登出）。
	rec = do(h, withCookie("boom"))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":503`) {
		t.Fatalf("REST 的 503 = %d %q", rec.Code, rec.Body)
	}
	r = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hawkr.identity.v1alpha1.IdentityAppService/GetSession", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer boom")
	rec = do(h, r)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":"unavailable"`) {
		t.Fatalf("Connect 的 503 = %d %q", rec.Code, rec.Body)
	}
}

func TestCORS(t *testing.T) {
	h := entry(t, caller)
	preflight := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/hawkr.identity.v1alpha1.IdentityAppService/GetSession", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "content-type,connect-protocol-version")
		return do(h, r)
	}
	rec := preflight(testOrigin)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != testOrigin ||
		rec.Header().Get("Access-Control-Allow-Credentials") != "true" ||
		!strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Connect-Protocol-Version") {
		t.Fatalf("允許的 origin 的 preflight = %d %v", rec.Code, rec.Header())
	}
	if rec := preflight("https://evil.example"); rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("其他 origin 的 preflight = %d %v", rec.Code, rec.Header())
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/public/products", nil)
	r.Header.Set("Origin", testOrigin)
	if rec := do(h, r); rec.Header().Get("Access-Control-Allow-Origin") != testOrigin || rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("跨來源的 GET = %v", rec.Header())
	}
}

func TestCSRF(t *testing.T) {
	h := entry(t, caller)
	post := func(path, site, origin string) int {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://api.example"+path, strings.NewReader("x"))
		if site != "" {
			r.Header.Set("Sec-Fetch-Site", site)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return do(h, r).Code
	}
	cases := []struct {
		name         string
		path         string
		site, origin string
		want         int
	}{
		{"別的網站以瀏覽器 POST", "/api/v1/app/orders", "cross-site", "https://evil.example", http.StatusForbidden},
		{"只有 Origin、不是自己的", "/api/v1/app/orders", "", "https://evil.example", http.StatusForbidden},
		{"自己的 web", "/api/v1/app/orders", "cross-site", testOrigin, http.StatusOK},
		{"App 與伺服器（沒有 Sec-Fetch-Site 與 Origin）", "/api/v1/app/orders", "", "", http.StatusOK},
		{"金流的瀏覽器回跳", "/payment/return", "cross-site", "https://payuni.example", http.StatusOK},
	}
	for _, c := range cases {
		if got := post(c.path, c.site, c.origin); got != c.want {
			t.Errorf("%s = %d，預期 %d", c.name, got, c.want)
		}
	}
}

func originAuth(key, p, ip, method, path string, ts time.Time) string {
	b, _ := json.Marshal(map[string]any{"v": 1, "p": p, "ip": ip, "ts": ts.Unix(), "m": method, "u": path})
	body := base64.RawURLEncoding.EncodeToString(b)
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte("hkorigin1:" + body))
	return body + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func TestRateLimitSubject(t *testing.T) {
	h := entry(t, caller)
	const path = "/api/v1/public/products"
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"沒有簽章：連線的 IP", "", "subject=v:192.0.2.1"},
		{"網頁的 SSR", originAuth("main-secret", "ssr", "", "GET", path, entryNow), "subject=s:ssr"},
		{"SSR 代訪客呼叫", originAuth("main-secret", "visitor", "203.0.113.9", "GET", path, entryNow), "subject=v:203.0.113.9"},
		{"CI", originAuth("ci-secret", "ci", "", "GET", path, entryNow), "subject=s:ci"},
		{"CI 的金鑰不能簽訪客", originAuth("ci-secret", "visitor", "203.0.113.9", "GET", path, entryNow), "subject=v:192.0.2.1"},
		{"簽章綁在別的路徑", originAuth("main-secret", "ssr", "", "GET", "/api/v1/other", entryNow), "subject=v:192.0.2.1"},
		{"過期", originAuth("main-secret", "ssr", "", "GET", path, entryNow.Add(-2*time.Minute)), "subject=v:192.0.2.1"},
		{"金鑰不對", originAuth("wrong", "ssr", "", "GET", path, entryNow), "subject=v:192.0.2.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if c.header != "" {
			r.Header.Set("X-HK-Origin-Auth", c.header)
		}
		if got := do(h, r).Body.String(); !strings.Contains(got, c.want) {
			t.Errorf("%s = %q，預期 %q", c.name, got, c.want)
		}
	}
}

func TestConnectIsRateLimitedByTier(t *testing.T) {
	if !slices.Equal(tiersOf("/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners"), []rateTier{tierPublic}) ||
		!slices.Equal(tiersOf("/hawkr.identity.v1alpha1.IdentityAppService/GetSession"), []rateTier{tierDefault}) ||
		!slices.Equal(tiersOf("/hawkr.admin.content.v1alpha1.ContentAdminService/ListBanners"), []rateTier{tierDefault}) {
		t.Fatal("級別對錯了")
	}
	if tierPublic.limit("s:ssr") != 12000 || tierPublic.limit("v:1.2.3.4") != 600 || tierDefault.limit("s:ssr") != 300 {
		t.Fatal("SSR 的額度不對")
	}

	const path = "/hawkr.identity.v1alpha1.IdentityAppService/"
	h := entry(t, nil, service{path, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })})
	call := func() *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path+"GetSession", strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		return do(h, r)
	}
	for i := range tierDefault.visitor {
		if rec := call(); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次 = %d", i+1, rec.Code)
		}
	}
	rec := call()
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"code":"resource_exhausted"`) ||
		!strings.Contains(rec.Body.String(), `"type":"google.rpc.RetryInfo"`) {
		t.Fatalf("超過額度 = %d %q", rec.Code, rec.Body)
	}
}

// 檔案回應（/api/files/）與 Connect 同一條入口鏈：經過限流（級別同後台的 Connect 方法），超過額度回 429 加 Retry-After，不是 Connect 的錯誤
// 格式，也不可快取；額度內的請求到達 handler。
func TestFileRoutesAreRateLimitedAndNeverFallThroughToTheLegacyGateway(t *testing.T) {
	const pattern = "GET /api/files/app/test/{id}"
	var served int
	legacy := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h, err := assemble(Deps{
		Now: func() time.Time { return entryNow }, Origins: []string{testOrigin}, OriginAuth: OriginAuth{Main: []byte("main-secret"), CI: []byte("ci-secret")},
		Legacy: legacy, LegacyPrefixes: []string{"/api/v1/"}, Logger: slog.New(slog.DiscardHandler),
	}, nil, fileRoute{pattern: pattern, handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { served++; w.WriteHeader(http.StatusOK) })})
	if err != nil {
		t.Fatal(err)
	}
	call := func() *httptest.ResponseRecorder {
		return do(h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/files/app/test/1", nil))
	}
	for i := range tierDefault.visitor {
		if rec := call(); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次 = %d", i+1, rec.Code)
		}
	}
	rec := call()
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || rec.Header().Get("Cache-Control") != "private, no-store" || served != tierDefault.visitor {
		t.Fatalf("超過額度 = %d retry-after=%q cache=%q served=%d", rec.Code, rec.Header().Get("Retry-After"), rec.Header().Get("Cache-Control"), served)
	}
	// 沒有註冊的 /api/files/ 路徑不會落到舊 gateway（前綴只有 /api/v1/ 等）。
	if rec := do(h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/files/app/unknown", nil)); rec.Code == http.StatusTeapot {
		t.Fatal("/api/files/ 不能轉給 gateway")
	}
}

// 轉給 gateway 的 WebSocket 與 SSE 經過入口的整條中介層（tracing、CORS、CSRF、session）之後仍然可以 upgrade、逐筆送出。
func TestLegacyStreamsSurviveTheMiddlewareChain(t *testing.T) {
	legacy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/app/chat/ws":
			conn, buf, err := http.NewResponseController(w).Hijack()
			if err != nil {
				http.Error(w, "hijack："+err.Error(), http.StatusInternalServerError)
				return
			}
			defer func() { _ = conn.Close() }()
			if _, err := fmt.Fprint(buf, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"); err != nil {
				t.Error(err)
				return
			}
			_ = buf.Flush()
		case "/api/v1/app/notifications/stream":
			_, _ = fmt.Fprint(w, "data: first\n\n")
			if err := http.NewResponseController(w).Flush(); err != nil {
				_, _ = fmt.Fprint(w, "flush："+err.Error())
			}
			<-r.Context().Done()
		}
	})
	srv := httptest.NewServer(entry(t, legacy))
	defer srv.Close()

	conn, err := new(net.Dialer).DialContext(t.Context(), "tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprint(conn, "GET /api/v1/app/chat/ws HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("WebSocket 的 upgrade = %v, %v", resp, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/app/notifications/stream", nil)
	sse, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sse.Body.Close() }()
	line, err := bufio.NewReader(sse.Body).ReadString('\n')
	if err != nil || line != "data: first\n" {
		t.Fatalf("SSE 的第一筆 = %q, %v", line, err)
	}
}

// 入口開新的 trace：client 帶來的 traceparent 不接續（L265），每個請求的 trace id 都不同。
func TestEntryStartsItsOwnTrace(t *testing.T) {
	h := entry(t, caller)
	const clientTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
	ids := map[string]bool{}
	for range 2 {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/public/products", nil)
		r.Header.Set("traceparent", "00-"+clientTrace+"-00f067aa0ba902b7-01")
		id := do(h, r).Header().Get("X-Trace-Id")
		if id == "" || id == clientTrace {
			t.Fatalf("X-Trace-Id = %q", id)
		}
		ids[id] = true
	}
	if len(ids) != 2 {
		t.Fatal("兩個請求的 trace id 相同")
	}
}

func TestAuthenticatedRateSubjectOverridesOriginAndIP(t *testing.T) {
	keys := OriginAuth{Main: []byte("main-secret")}
	h := withSession(fakeAuth{}, slog.New(slog.DiscardHandler), withSubject(keys, ClientIP{}, func() time.Time { return entryNow }, caller))
	for _, tc := range []struct{ token, want string }{{"good", "a:7"}, {"admin", "m:3"}, {"dead", "v:192.0.2.1"}} {
		r := httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/app/buyers/me", nil)
		r.RemoteAddr = "192.0.2.1:1234"
		r.Header.Set("Authorization", "Bearer "+tc.token)
		if tc.token != "dead" {
			payload, _ := json.Marshal(originAuthPayload{V: 1, P: "ssr", TS: entryNow.Unix(), M: r.Method, U: r.URL.Path})
			body := base64.RawURLEncoding.EncodeToString(payload)
			mac := hmac.New(sha256.New, keys.Main)
			mac.Write([]byte(originAuthDomain + body))
			r.Header.Set(originAuthHeader, body+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
		}
		if got := do(h, r).Body.String(); !strings.Contains(got, "subject="+tc.want) {
			t.Fatalf("%s: %s", tc.token, got)
		}
	}
}

// 連線端 IP 放進 ctx 的來源只有一個（ClientIP.of，與限流身分同一套）：訪客與已登入的請求都有；已登入時限流身分換成 a:<ID>，IP 仍在。
func TestClientIPIsInTheContextForGuestsAndSignedInRequests(t *testing.T) {
	var got string
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = rpc.ClientIPFrom(r.Context())
		_, _ = fmt.Fprintf(w, "subject=%s", rpc.SubjectFrom(r.Context()))
	})
	for name, c := range map[string]struct {
		clientIP ClientIP
		header   string
		token    string
		want     string
		subject  string
	}{
		"訪客，連線位址":                {ClientIP{}, "", "", "192.0.2.1", "v:192.0.2.1"},
		"已登入，連線位址":               {ClientIP{}, "", "good", "192.0.2.1", "a:7"},
		"訪客，CDN 的 header":        {ClientIP{Header: "CF-Connecting-IP"}, "198.51.100.4", "", "198.51.100.4", "v:198.51.100.4"},
		"已登入，CDN 的 header":       {ClientIP{Header: "CF-Connecting-IP"}, "198.51.100.4", "good", "198.51.100.4", "a:7"},
		"沒有設定 header 時不讀 header": {ClientIP{}, "198.51.100.4", "", "192.0.2.1", "v:192.0.2.1"},
	} {
		got = ""
		h := withSession(fakeAuth{}, slog.New(slog.DiscardHandler), withSubject(OriginAuth{}, c.clientIP, func() time.Time { return entryNow }, probe))
		r := httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/app/buyers/me", nil)
		r.RemoteAddr = "192.0.2.1:1234"
		if c.header != "" {
			r.Header.Set("CF-Connecting-IP", c.header)
		}
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
		body := do(h, r).Body.String()
		if got != c.want || !strings.Contains(body, "subject="+c.subject) {
			t.Errorf("%s：ip=%q body=%s", name, got, body)
		}
	}
}

// 每條 /api/files/admin/ 路由都要宣告目錄裡的權限，其他路由不得宣告權限；宣告不合法時 assemble 失敗（不能啟動）。
func TestFileRoutesMustDeclareAnAdminPermission(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	for name, route := range map[string]fileRoute{
		"後台路由沒有宣告權限":    {pattern: "GET /api/files/admin/x/{id}", handler: ok},
		"後台路由宣告不在目錄的權限": {pattern: "GET /api/files/admin/x/{id}", handler: ok, permission: "seller.no_such.get"},
		"非後台路由宣告了權限":    {pattern: "GET /api/files/app/x/{id}", handler: ok, permission: adminperm.SellerKYCDocumentsGet},
	} {
		if _, err := assemble(Deps{Logger: slog.New(slog.DiscardHandler)}, nil, route); err == nil {
			t.Errorf("%s：assemble 應該失敗", name)
		}
	}
	valid := fileRoute{pattern: "GET /api/files/admin/x/{id}", handler: ok, permission: adminperm.SellerKYCDocumentsGet}
	if _, err := assemble(Deps{Logger: slog.New(slog.DiscardHandler)}, nil, valid); err != nil {
		t.Fatalf("合法的宣告：%v", err)
	}
}
