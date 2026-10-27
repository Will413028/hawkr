package gatewayproxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type callerKey struct{}

func noVendorSession(*http.Request, int64) (agent.SessionInfo, bool, error) {
	return agent.SessionInfo{}, false, nil
}

func callerOf(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	return c, ok
}

// start 起一個假的 gateway 與代理；代理前面加一層把 caller 放進 ctx（入口的認證中介層做的事）。
func start(t *testing.T, gateway http.Handler, caller *Caller) *httptest.Server {
	t.Helper()
	gw := httptest.NewServer(gateway)
	t.Cleanup(gw.Close)
	p, err := New(gw.URL, callerOf, noVendorSession, emptyBusiness{}, emptyBusiness{}, sdktrace.NewTracerProvider(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if caller != nil {
			r = r.WithContext(context.WithValue(r.Context(), callerKey{}, *caller))
		}
		p.ServeHTTP(w, r)
	}))
	t.Cleanup(front.Close)
	return front
}

func echoHeaders(w http.ResponseWriter, r *http.Request) {
	for _, k := range []string{HeaderAccountID, HeaderAccountType, HeaderLoginRole, "X-Active-Role", "Traceparent", "Origin", "X-Forwarded-For"} {
		_, _ = fmt.Fprintf(w, "%s=%s\n", k, r.Header.Get(k))
	}
	_, _ = fmt.Fprintf(w, "path=%s?%s\n", r.URL.Path, r.URL.RawQuery)
}

func get(t *testing.T, url string, header map[string]string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// X-Login-Role 由代理從 client 的 X-Active-Role 提示加上帳號實際的角色推導（舊 REST 的工作區提示，Connect 不讀）；
// client 送來的 X-Login-Role、X-Active-Role 都不會原樣轉出。
func TestIdentityHeadersComeOnlyFromTheEntry(t *testing.T) {
	seller := identity.Principal{Kind: identity.PrincipalAccount, ID: 7, Roles: []identity.Role{identity.RoleSeller}}
	cases := []struct {
		name   string
		caller *Caller
		hint   string
		want   []string
	}{
		{"訪客：client 偽造的身分全部刪掉（#1859）", nil, "SELLER",
			[]string{"X-Account-Id=\n", "X-Account-Type=\n", "X-Login-Role=\n", "X-Active-Role=\n"}},
		// 買家位元由 legacy adapter 合成（W5）：identity 只有 SELLER，舊契約的 account_type 仍是買家 1 加賣家 2。
		{"賣家（也是買家）、提示賣家 → 2", &Caller{Principal: seller}, "SELLER",
			[]string{"X-Account-Id=7\n", "X-Account-Type=3\n", "X-Login-Role=2\n", "X-Active-Role=\n"}},
		{"賣家（也是買家）、沒有提示 → 1", &Caller{Principal: seller}, "",
			[]string{"X-Account-Id=7\n", "X-Account-Type=3\n", "X-Login-Role=1\n", "X-Active-Role=\n"}},
		{"沒有另外申請角色的帳號、提示賣家仍是買家 → 1", &Caller{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: 8}}, "SELLER",
			[]string{"X-Account-Id=8\n", "X-Account-Type=1\n", "X-Login-Role=1\n", "X-Active-Role=\n"}},
		{"管理員：不送視角，只送 4", &Caller{Principal: identity.Principal{Kind: identity.PrincipalAdmin, ID: 7}}, "SELLER",
			[]string{"X-Account-Id=7\n", "X-Account-Type=4\n", "X-Login-Role=4\n", "X-Active-Role=\n"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			forged := map[string]string{HeaderAccountID: "1", HeaderAccountType: "4", HeaderLoginRole: "4", "Origin": "https://web.example"}
			if c.hint != "" {
				forged["X-Active-Role"] = c.hint
			}
			front := start(t, http.HandlerFunc(echoHeaders), c.caller)
			body := get(t, front.URL+"/api/v1/app/buyers/me?x=1", forged)
			for _, w := range c.want {
				if !strings.Contains(body, w) {
					t.Errorf("gateway 收到的 header 沒有 %q：\n%s", w, body)
				}
			}
			if !strings.Contains(body, "path=/api/v1/app/buyers/me?x=1") || !strings.Contains(body, "Origin=https://web.example") {
				t.Errorf("path、query 或 Origin 沒有原樣轉過去：\n%s", body)
			}
			if !strings.Contains(body, "Traceparent=00-") {
				t.Errorf("沒有注入 traceparent：\n%s", body)
			}
		})
	}
}

// SSE 每一筆都要馬上送到 client，不能等緩衝滿或回應結束。
func TestServerSentEventsAreFlushedPerEvent(t *testing.T) {
	release := make(chan struct{})
	front := start(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
		_, _ = fmt.Fprint(w, "data: second\n\n")
	}), nil)
	defer close(release)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, front.URL+"/api/v1/app/notifications/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	got := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(resp.Body).ReadString('\n')
		got <- line
	}()
	select {
	case line := <-got:
		if line != "data: first\n" {
			t.Fatalf("第一筆 = %q", line)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("第一筆被緩衝住，沒有馬上送到")
	}
}

// WebSocket 的 upgrade 經過代理，兩邊可以雙向傳資料。
func TestWebSocketUpgradePassesThrough(t *testing.T) {
	front := start(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "not an upgrade", http.StatusBadRequest)
			return
		}
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = fmt.Fprint(buf, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = buf.Flush()
		line, _ := buf.ReadString('\n')
		_, _ = fmt.Fprint(buf, "echo:"+line)
		_ = buf.Flush()
	}), nil)

	conn, err := new(net.Dialer).DialContext(t.Context(), "tcp", strings.TrimPrefix(front.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprint(conn, "GET /api/v1/app/chat/ws?ticket=t HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(conn)
	resp, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade = %d", resp.StatusCode)
	}
	if _, err := fmt.Fprint(conn, "hello\n"); err != nil {
		t.Fatal(err)
	}
	line, err := r.ReadString('\n')
	if err != nil || line != "echo:hello\n" {
		t.Fatalf("upgrade 之後 = %q, %v", line, err)
	}
}

// 上傳的 multipart 原樣串流過去；大小由 gateway 自己的分級限制，代理不擋（這裡 3 MiB，超過單體 Connect 的 2 MiB）。
func TestMultipartBodiesStreamThrough(t *testing.T) {
	front := start(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		n, _ := io.Copy(io.Discard, f)
		_, _ = fmt.Fprint(w, n)
	}), nil)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "products.csv")
	_, _ = fw.Write(bytes.Repeat([]byte("a"), 3<<20))
	_ = mw.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, front.URL+"/api/v1/app/product_import_jobs", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(got) != fmt.Sprint(3<<20) {
		t.Fatalf("= %d %q", resp.StatusCode, got)
	}
}

func TestGatewayDownIsA503(t *testing.T) {
	p, err := New("http://127.0.0.1:1", callerOf, noVendorSession, emptyBusiness{}, emptyBusiness{}, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/public/products", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":503`) {
		t.Fatalf("= %d %s", rec.Code, rec.Body)
	}
}

func TestNewRejectsABadURL(t *testing.T) {
	if _, err := New("localhost:9900", callerOf, noVendorSession, emptyBusiness{}, emptyBusiness{}, nil, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("沒有 scheme 的網址應該失敗")
	}
}

type emptyBusiness struct{}

func (emptyBusiness) CurrentBuyerID(context.Context, int64) (int64, error) { return 0, nil }
func (emptyBusiness) CurrentStore(context.Context, int64) (seller.Store, error) {
	return storeIn(seller.Active), nil
}
