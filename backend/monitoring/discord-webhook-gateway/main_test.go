package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSendDiscordWebhook_TransportErrorDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(nil)
	srv.Close() // 立即關閉，確定性觸發 dial 失敗

	webhookURL := srv.URL + "/api/webhooks/123456789/hunter2-secret-token"
	err := sendDiscordWebhook(webhookURL, discordWebhook{Content: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "hunter2-secret-token") {
		t.Errorf("error leaks webhook token via URL: %v", err)
	}
}

// metricsHandler 的價值全部來自「Discord 轉發失敗時那個計數真的會動」——
// 如果只驗它吐得出 exposition format，失敗計數永遠是 0 也會通過。
// 所以這裡走完整條 handler 路徑：先讓轉發成功一次、再讓它失敗一次，
// 斷言 received 與 failed 分別是 2 與 1。
func TestMetricsHandler_CountsForwardFailures(t *testing.T) {
	var fail atomic.Bool
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discord.Close()

	c := &routeCounters{}
	h := makeAlertHandler(discord.URL, c)
	post := func() int {
		body := `{"status":"firing","alerts":[{"status":"firing","labels":{"alertname":"X"},"annotations":{}}]}`
		req := httptest.NewRequest(http.MethodPost, "/alert", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec.Code
	}

	if got := post(); got != http.StatusOK {
		t.Fatalf("成功路徑應回 200，得到 %d", got)
	}
	fail.Store(true)
	if got := post(); got != http.StatusInternalServerError {
		t.Fatalf("失敗路徑應回 500，得到 %d", got)
	}

	rec := httptest.NewRecorder()
	metricsHandler([]string{"/alert"}, map[string]*routeCounters{"/alert": c})(
		rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type 應是 text/plain（Prometheus 才解析得了），得到 %q", ct)
	}
	out := rec.Body.String()
	for _, want := range []string{
		`discord_webhook_gateway_alerts_received_total{route="/alert"} 2`,
		`discord_webhook_gateway_forward_failures_total{route="/alert"} 1`,
		`discord_webhook_gateway_routes_enabled 1`,
		"# TYPE discord_webhook_gateway_forward_failures_total counter",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("/metrics 缺少 %q\n實際輸出:\n%s", want, out)
		}
	}
}

// 未註冊的路由不得出現在 /metrics —— 否則「這個頻道沒設定」會被誤讀成
// 「這個頻道零告警」，兩者在 dashboard 上長得一樣。
func TestMetricsHandler_OnlyEnabledRoutes(t *testing.T) {
	c := &routeCounters{}
	rec := httptest.NewRecorder()
	metricsHandler([]string{"/alert"}, map[string]*routeCounters{"/alert": c})(
		rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()
	if strings.Contains(out, "/alert/dev-alert") {
		t.Errorf("未註冊的路由不該出現在 /metrics:\n%s", out)
	}
	if !strings.Contains(out, "discord_webhook_gateway_routes_enabled 1") {
		t.Errorf("routes_enabled 應為 1:\n%s", out)
	}
}
