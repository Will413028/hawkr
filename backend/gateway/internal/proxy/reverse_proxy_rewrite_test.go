package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// #2808：Director（Go 1.26 deprecated）遷移到 Rewrite。
//
// 這兩支測試釘住的是**轉發後下游實際收到什麼**，不是「有沒有呼叫某個 API」——
// Rewrite 與 Director 的差異全部發生在 ServeHTTP 內部（前綴 strip 的目標欄位從
// req.URL.Path 變成 r.Out.URL.Path、Host 從 req.Host 變成 r.Out.Host、
// X-Forwarded-* 從自動附加變成必須顯式 SetXForwarded），任何一項寫錯都是
// 編譯得過、單看程式碼也看不出來，只有真的打一次才會現形。
//
// 留成正式測試而非臨時檔的理由：遷移前 CreateProxy 的路徑改寫**零測試覆蓋**，
// 而它是 media 上傳／下載／預覽與賣家 PATCH 的唯一轉發路徑，改錯等於那些路由
// 全部 404 在下游。

// startFakeUpstream 起一個假的下游服務，並把 services[serviceName] 暫時指向它。
// 回傳「最後一次收到的請求」的取用函式；t.Cleanup 負責還原 services 與關閉 server。
func startFakeUpstream(t *testing.T, serviceName string) func() *http.Request {
	t.Helper()

	var got *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		got.Host = r.Host // Clone 不保證帶走 Host，顯式複製
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "upstream-ok")
	}))
	t.Cleanup(upstream.Close)

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}
	host, port := u.Hostname(), u.Port()

	original, existed := services[serviceName]
	services[serviceName] = ServiceConfig{Name: "fake-" + serviceName, Host: host, Port: port}
	t.Cleanup(func() {
		if existed {
			services[serviceName] = original
		} else {
			delete(services, serviceName)
		}
	})

	return func() *http.Request { return got }
}

// startGateway 把 handler 掛在 method+pattern 上，用 httptest 起真的 HTTP server
// （需要真的 server 才有 RemoteAddr，SetXForwarded 才有東西可填）。
func startGateway(t *testing.T, method, pattern string, h gin.HandlerFunc) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Handle(method, pattern, h)
	gw := httptest.NewServer(engine)
	t.Cleanup(gw.Close)
	return gw
}

// CreateProxy：/api/v1 前綴必須被 strip，Host 必須改成上游的 host:port。
// 這是 media 三條路由（secure_upload / secure_download / secure_preview）與
// 賣家 PATCH /app/sellers/:id 實際依賴的行為。
func TestCreateProxy_StripsAPIPrefixAndSetsHost(t *testing.T) {
	lastRequest := startFakeUpstream(t, "media")
	upstreamHost := services["media"].Host + ":" + services["media"].Port

	gw := startGateway(t, http.MethodGet,
		"/api/v1/app/media/secure_download/:asset_id",
		CreateProxy("media", FailClientFacing))

	// 帶一個偽造的 X-Forwarded-For，順便釘住「覆寫而非附加」的語意。
	req, err := http.NewRequest(http.MethodGet,
		gw.URL+"/api/v1/app/media/secure_download/abc123?token=t1&preset=thumb", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("打 gateway 失敗：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK || string(body) != "upstream-ok" {
		t.Fatalf("請求沒有被轉發到上游：status=%d body=%q", resp.StatusCode, string(body))
	}

	got := lastRequest()
	if got == nil {
		t.Fatal("上游沒收到任何請求")
	}

	// ① 前綴 strip：下游期望 /app/media/... 而不是 /api/v1/app/media/...
	if want := "/app/media/secure_download/abc123"; got.URL.Path != want {
		t.Errorf("上游收到的 path = %q, want %q（/api/v1 前綴未被 strip ⇒ 下游 404）",
			got.URL.Path, want)
	}
	if strings.HasPrefix(got.URL.Path, "/api") {
		t.Errorf("上游收到的 path 仍帶 /api 前綴：%q", got.URL.Path)
	}

	// ② Host header 指向上游，不是 gateway 自己
	if got.Host != upstreamHost {
		t.Errorf("上游收到的 Host = %q, want %q", got.Host, upstreamHost)
	}

	// ③ query string 原樣保留
	if q := got.URL.Query(); q.Get("token") != "t1" || q.Get("preset") != "thumb" {
		t.Errorf("query 遺失或被改寫：%q", got.URL.RawQuery)
	}

	// ④ SetXForwarded 有生效，而且是**覆寫**：client 自稱的 1.2.3.4 不得留下
	//    （ServeHTTP 在 Rewrite 前就把 client 送來的 X-Forwarded-* 刪光了）。
	xff := got.Header.Get("X-Forwarded-For")
	if xff == "" {
		t.Error("X-Forwarded-For 為空 —— Rewrite 不會自動帶，漏了 SetXForwarded()")
	}
	if strings.Contains(xff, "1.2.3.4") {
		t.Errorf("X-Forwarded-For = %q 仍含 client 自稱的值；SetXForwarded 應為覆寫", xff)
	}
	if got.Header.Get("X-Forwarded-Proto") != "http" {
		t.Errorf("X-Forwarded-Proto = %q, want http", got.Header.Get("X-Forwarded-Proto"))
	}
}

// CreateProxyWithPathRewrite：fromPath → toPath。
// 唯一消費者是 PayUni webhook（/webhook/payuni/notify → /webhooks/payuni/notify），
// 改錯 ＝ 第三方通知全部打不到 payuni-webhook-service。
func TestCreateProxyWithPathRewrite_RewritesPathAndSetsHost(t *testing.T) {
	lastRequest := startFakeUpstream(t, "payuni-webhook")
	upstreamHost := services["payuni-webhook"].Host + ":" + services["payuni-webhook"].Port

	gw := startGateway(t, http.MethodPost, "/webhook/payuni/notify",
		CreateProxyWithPathRewrite("payuni-webhook",
			"/webhook/payuni/notify", "/webhooks/payuni/notify", FailThirdPartyWebhook))

	resp, err := http.Post(gw.URL+"/webhook/payuni/notify",
		"application/x-www-form-urlencoded", strings.NewReader("MerID=HAWK1&EncryptInfo=xx"))
	if err != nil {
		t.Fatalf("打 gateway 失敗：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.ReadAll(resp.Body)

	got := lastRequest()
	if got == nil {
		t.Fatal("上游沒收到任何請求")
	}
	if want := "/webhooks/payuni/notify"; got.URL.Path != want {
		t.Errorf("上游收到的 path = %q, want %q（路徑改寫未生效）", got.URL.Path, want)
	}
	if got.Host != upstreamHost {
		t.Errorf("上游收到的 Host = %q, want %q", got.Host, upstreamHost)
	}
	if got.Header.Get("X-Forwarded-For") == "" {
		t.Error("X-Forwarded-For 為空 —— 漏了 SetXForwarded()")
	}
}
