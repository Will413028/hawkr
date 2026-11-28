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

func startFakeUpstream(t *testing.T, serviceName string) func() *http.Request {
	t.Helper()

	var got *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		got.Host = r.Host
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
// 這是發票 PDF 串流路由與
// 賣家 PATCH /app/sellers/:id 實際依賴的行為。
func TestCreateProxy_StripsAPIPrefixAndSetsHost(t *testing.T) {
	lastRequest := startFakeUpstream(t, "invoice")
	upstreamHost := services["invoice"].Host + ":" + services["invoice"].Port

	gw := startGateway(t, http.MethodGet,
		"/api/v1/app/invoices/documents/:asset_id",
		CreateProxy("invoice", FailClientFacing))

	req, err := http.NewRequest(http.MethodGet,
		gw.URL+"/api/v1/app/invoices/documents/abc123?token=t1&preset=thumb", nil)
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

	if want := "/app/invoices/documents/abc123"; got.URL.Path != want {
		t.Errorf("上游收到的 path = %q, want %q（/api/v1 前綴未被 strip ⇒ 下游 404）",
			got.URL.Path, want)
	}
	if strings.HasPrefix(got.URL.Path, "/api") {
		t.Errorf("上游收到的 path 仍帶 /api 前綴：%q", got.URL.Path)
	}

	if got.Host != upstreamHost {
		t.Errorf("上游收到的 Host = %q, want %q", got.Host, upstreamHost)
	}

	if q := got.URL.Query(); q.Get("token") != "t1" || q.Get("preset") != "thumb" {
		t.Errorf("query 遺失或被改寫：%q", got.URL.RawQuery)
	}

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
