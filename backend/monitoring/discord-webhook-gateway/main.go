package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync/atomic"
)

// Alertmanager webhook → Discord Webhook bridge
// 接收 Alertmanager POST，轉為 Discord Embed 訊息

type alertmanagerPayload struct {
	Status string  `json:"status"` // "firing" or "resolved"
	Alerts []alert `json:"alerts"`
}

type alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type discordWebhook struct {
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds,omitempty"`
}

type discordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// routeCounters 是每條路由的轉發計數。刻意不引入 prometheus/client_golang——
// 這個服務的 go.mod 至今零依賴，而 exposition format 的 counter 是三行文字，
// 為了三行文字換來一整棵依賴樹不划算。
type routeCounters struct {
	received atomic.Int64
	failed   atomic.Int64
}

// metricsHandler 手寫 Prometheus exposition format。
//
// 為什麼這個服務需要 /metrics：它是告警鏈的**最後一跳**。在此之前，
// sendDiscordWebhook 失敗只會 log.Printf 一行然後回 500 給 Alertmanager
// ——Alertmanager 會重試，但沒有任何東西會告訴人「告警送不出去」。
// 而這正是最不能靜默的一種失效：它發生的時候，所有其他告警也送不出去。
//
// 另外它本身沒有 up series（2026-09-10 實測），所以 ServiceDown 也蓋不到它。
func metricsHandler(routes []string, counters map[string]*routeCounters) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var b strings.Builder
		b.WriteString("# HELP discord_webhook_gateway_alerts_received_total Alertmanager POST 進來的次數（依路由）\n")
		b.WriteString("# TYPE discord_webhook_gateway_alerts_received_total counter\n")
		for _, path := range routes {
			fmt.Fprintf(&b, "discord_webhook_gateway_alerts_received_total{route=%q} %d\n",
				path, counters[path].received.Load())
		}
		b.WriteString("# HELP discord_webhook_gateway_forward_failures_total 轉發到 Discord 失敗的次數（依路由）\n")
		b.WriteString("# TYPE discord_webhook_gateway_forward_failures_total counter\n")
		for _, path := range routes {
			fmt.Fprintf(&b, "discord_webhook_gateway_forward_failures_total{route=%q} %d\n",
				path, counters[path].failed.Load())
		}
		b.WriteString("# HELP discord_webhook_gateway_routes_enabled 已註冊的路由數（未設 webhook URL 的路由不註冊）\n")
		b.WriteString("# TYPE discord_webhook_gateway_routes_enabled gauge\n")
		fmt.Fprintf(&b, "discord_webhook_gateway_routes_enabled %d\n", len(routes))
		_, _ = io.WriteString(w, b.String())
	}
}

func main() {
	// 每個路徑對應一個 Discord 頻道 webhook（皆由 env 注入，密鑰不入版控）。
	// 依告警性質分流：/alert = infra 系統告警（monitor-alert 既有頻道）；business 告警再依 env 分 dev/prod 頻道。
	// Alertmanager 依 label（channel / env）路由到對應路徑（見 alertmanager/config.yml）。
	routes := map[string]string{
		"/alert":            os.Getenv("DISCORD_WEBHOOK_URL"),            // infra 系統告警 monitor-alert（預設，必填）
		"/alert/dev-alert":  os.Getenv("DISCORD_WEBHOOK_URL_DEV_ALERT"),  // dev 業務告警頻道 dev-警報
		"/alert/prod-alert": os.Getenv("DISCORD_WEBHOOK_URL_PROD_ALERT"), // prod 業務告警頻道 prod-警報
	}
	if routes["/alert"] == "" {
		log.Fatal("DISCORD_WEBHOOK_URL is required")
	}

	counters := make(map[string]*routeCounters, len(routes))
	enabled := make([]string, 0, len(routes))
	for path, url := range routes {
		if url == "" {
			// 未設定該頻道 webhook → 不註冊該路徑（Alertmanager 若指向未註冊路徑會收到 404，於部署時可察覺）
			log.Printf("route %s disabled (webhook url not set)", path)
			continue
		}
		c := &routeCounters{}
		counters[path] = c
		enabled = append(enabled, path)
		http.HandleFunc(path, makeAlertHandler(url, c))
		log.Printf("route %s enabled", path)
	}
	// 路由順序決定 /metrics 的輸出順序；map 迭代無序，排過才不會每次 scrape 都換行序。
	sort.Strings(enabled)
	http.HandleFunc("/metrics", metricsHandler(enabled, counters))

	log.Println("Discord webhook gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// makeAlertHandler 回傳一個把 Alertmanager POST 轉發到指定 Discord webhook 的 handler。
func makeAlertHandler(webhookURL string, c *routeCounters) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.received.Add(1)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}

		var payload alertmanagerPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		webhook := buildDiscordWebhook(payload)
		if err := sendDiscordWebhook(webhookURL, webhook); err != nil {
			c.failed.Add(1)
			log.Printf("Discord webhook failed: %v", err)
			http.Error(w, "notify failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func buildDiscordWebhook(p alertmanagerPayload) discordWebhook {
	// 紅色=觸發, 綠色=已恢復
	color := 0xED4245 // 紅色
	title := fmt.Sprintf("🔥 FIRING — %d alert(s)", len(p.Alerts))
	if p.Status == "resolved" {
		color = 0x57F287 // 綠色
		title = fmt.Sprintf("✅ RESOLVED — %d alert(s)", len(p.Alerts))
	}

	fields := make([]discordEmbedField, 0, len(p.Alerts))
	for _, a := range p.Alerts {
		env := a.Labels["env"]
		name := a.Labels["alertname"]
		severity := a.Labels["severity"]
		summary := a.Annotations["summary"]

		value := fmt.Sprintf("**Severity:** %s", severity)
		if summary != "" {
			value += fmt.Sprintf("\n%s", summary)
		}

		fields = append(fields, discordEmbedField{
			Name:   fmt.Sprintf("[%s] %s", strings.ToUpper(env), name),
			Value:  value,
			Inline: false,
		})
	}

	return discordWebhook{
		Embeds: []discordEmbed{
			{
				Title:  title,
				Color:  color,
				Fields: fields,
			},
		},
	}
}

func sendDiscordWebhook(webhookURL string, payload discordWebhook) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		// *url.Error.Error() 會把完整 webhook URL（含 path 裡的 token）原樣印進
		// 訊息——net/http 的 stripPassword 只處理 userinfo，對 path 完全不起作用。
		// .Err 是真正的網路層錯誤，不含 URL。
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return fmt.Errorf("discord webhook request: %w", uerr.Err)
		}
		return fmt.Errorf("discord webhook request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Discord 回傳 204 No Content 表示成功
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
