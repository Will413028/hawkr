package proxy

import (
	"net/url"
	"strings"
	"testing"
)

// mustParseTarget 必須在上游位址解析失敗時 panic，而不是回一個 nil target。
//
// 背景（#2813）：原本寫成 `targetURL, _ := url.Parse(...)`，error 被丟掉之後 nil
// 不會在啟動時被任何東西發現，而是在該路由的每一個請求上於 httputil.ReverseProxy
// 內部 nil dereference，被 gin Recovery() 接成 500 + 空 body（沒有 envelope、沒有
// traceId、ErrorHandler 也不會進）。這支測試守住「配置壞掉要在開機就炸」。
func TestMustParseTargetPanicsOnUnparsableUpstream(t *testing.T) {
	// port 帶非數字且不含 "//"，url.Parse 會判 "invalid port" 而回 err + nil URL。
	// 這正是把 k8s service-link 注入的值誤當成 port 時可能出現的形狀。
	cfg := ServiceConfig{Name: "broken-service", Host: "broken-service", Port: "tcp"}

	defer func() {
		p := recover()
		if p == nil {
			t.Fatal("上游位址無法解析時必須 panic，實際沒有 panic —— " +
				"error 又被吞掉了，會退化成每請求 500 空 body（#2813）")
		}
		msg, ok := p.(string)
		if !ok {
			t.Fatalf("panic 值應為 string，實際 %T", p)
		}
		// panic 訊息必須帶足排障線索：是哪個 service、實際的 host/port 是什麼。
		// #2813 的整輪成本就花在「日誌裡沒有任何指向配置的線索」。
		for _, want := range []string{"broken-service", "tcp", "port="} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic 訊息缺少排障線索 %q：%s", want, msg)
			}
		}
	}()

	mustParseTarget("broken", cfg)
}

// 正常配置必須回一個可用的 target，且不得 panic。
func TestMustParseTargetAcceptsValidUpstream(t *testing.T) {
	cfg := ServiceConfig{Name: "media-service", Host: "media-service", Port: "9936"}

	got := mustParseTarget("media", cfg)
	if got == nil {
		t.Fatal("正常配置不該回 nil")
	}
	if got.Scheme != "http" || got.Host != "media-service:9936" {
		t.Errorf("target 解析錯誤：scheme=%q host=%q", got.Scheme, got.Host)
	}
}

// 固化 #2813 的真正 root cause：k8s service-link 注入的 `tcp://IP:PORT` 形式會讓
// url.Parse 失敗，於是被丟掉的 error 變成 nil target，變成每請求 500 空 body。
//
// 這個值怎麼來的：k8s 對 Service `seller-service` 自動注入 `SELLER_SERVICE_PORT`
// （service-link），而 gateway 自己也要讀同名的 `SELLER_SERVICE_PORT` 當 HTTP port。
// cm/gateway-config 只定義了 SELLER_SERVICE_HOST 與 SELLER_SERVICE_GRPC_PORT，
// **沒有定義 SELLER_SERVICE_PORT**，於是 service-link 的 `tcp://10.43.144.51:10021`
// 生效（dev pod 的 /proc/1/environ 實測值）。
//
// ⚠️ 驗證這件事時有一個真實踩過的陷阱：**這個 parse 的成敗取決於 module 的 go
// directive**。Go 1.26 收緊了 URL 的 host/port 驗證，而 go directive 會透過 GODEBUG
// 決定套用新舊哪一套語意 —— 同一個字串，在 `go 1.26.x` 的 module 裡 parse 失敗，
// 在 `go 1.24.x` 的 module 裡卻會成功。#2813 調查時就是先在一個 go1.24 的臨時 module
// 裡重現，得到「parse 得過」的相反結論，白繞一圈。要驗這種行為，必須在**本 module
// 內**跑（也就是這支測試所在的位置），不能另開一個小程式。
//
// 同一個 pod 內同樣是 tcp:// 形式的還有 ACCOUNT_SERVICE_PORT／BUYER_SERVICE_PORT／
// SOCIAL_SERVICE_PORT。它們今天沒有 CreateProxy 路由在用，所以不會被 parse；
// 一旦有人加了，mustParseTarget 會在**開機**就炸並印出實際值，而不是留下一個
// 沒有線索的 500。
func TestServiceLinkStyleUpstreamFailsToParse(t *testing.T) {
	const port = "tcp://10.43.144.51:10021" // dev pod /proc/1/environ 實測值
	cfg := ServiceConfig{Name: "seller-service", Host: "seller-service", Port: port}

	// 先釘住底層 url.Parse 的行為，這是整個 bug 的起點。
	u, err := url.Parse("http://" + cfg.Host + ":" + cfg.Port)
	if err == nil {
		t.Fatalf("前提改變了：service-link 形式的值現在 parse 成功了 (host=%q)。"+
			"若 Go 放寬了 URL host 驗證、或本 module 的 go directive 被降版，"+
			"#2813 的歸因就要重新評估", u.Host)
	}
	if u != nil {
		t.Errorf("url.Parse 失敗時應回 nil URL，實際 %#v", u)
	}

	// 而 mustParseTarget 必須把它變成開機期 panic，不是請求期 nil deref。
	defer func() {
		if p := recover(); p == nil {
			t.Fatal("service-link 形式的上游位址必須在開機就 panic")
		}
	}()
	mustParseTarget("seller", cfg)
}
