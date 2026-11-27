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


type alertmanagerPayload struct {
	Status string  `json:"status"`
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

type routeCounters struct {
	received atomic.Int64
	failed   atomic.Int64
}

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
	routes := map[string]string{
		"/alert":            os.Getenv("DISCORD_WEBHOOK_URL"),
		"/alert/dev-alert":  os.Getenv("DISCORD_WEBHOOK_URL_DEV_ALERT"),
		"/alert/prod-alert": os.Getenv("DISCORD_WEBHOOK_URL_PROD_ALERT"),
	}
	if routes["/alert"] == "" {
		log.Fatal("DISCORD_WEBHOOK_URL is required")
	}

	counters := make(map[string]*routeCounters, len(routes))
	enabled := make([]string, 0, len(routes))
	for path, url := range routes {
		if url == "" {
			log.Printf("route %s disabled (webhook url not set)", path)
			continue
		}
		c := &routeCounters{}
		counters[path] = c
		enabled = append(enabled, path)
		http.HandleFunc(path, makeAlertHandler(url, c))
		log.Printf("route %s enabled", path)
	}
	sort.Strings(enabled)
	http.HandleFunc("/metrics", metricsHandler(enabled, counters))

	log.Println("Discord webhook gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

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
	color := 0xED4245
	title := fmt.Sprintf("🔥 FIRING — %d alert(s)", len(p.Alerts))
	if p.Status == "resolved" {
		color = 0x57F287
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
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return fmt.Errorf("discord webhook request: %w", uerr.Err)
		}
		return fmt.Errorf("discord webhook request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
