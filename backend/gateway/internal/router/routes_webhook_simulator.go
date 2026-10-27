package router

import (
	"os"

	"github.com/Will413028/hawkr/gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

// registerWebhookSimulatorRoutes 配置 Webhook 模擬器路由
// 代理到 payuni-webhook-service，無需 JWT（開發測試工具）
//
// L3 defense in depth：僅在 local/dev/development 環境註冊路由，其他環境（含 prod）
// 完全不註冊 → 自然回 404（攻擊者也無法 enumerate 出端點存在）。
// fail-secure default = production，避免新環境漏注入 APP_ENV 被當作 dev。
// 對齊 routes_docs.go 既有 pattern。
// FailClientFacing：模擬器本身是我們自己用瀏覽器開的開發工具（不是第三方送信端），
// 上游不可達時要拿得到 envelope，見 proxy.UpstreamFailureStatus。
func registerWebhookSimulatorRoutes(r *gin.Engine) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}

	switch env {
	case "local", "dev", "development":
		r.GET(WebhookSimulatorPath, proxyToPath(ServicePayuniWebhook, "/simulator", proxy.FailClientFacing))
		r.POST(WebhookSimulatorSendPath, proxyToPath(ServicePayuniWebhook, "/simulator/send", proxy.FailClientFacing))
		r.GET(WebhookSimulatorLookupPath, proxyToPath(ServicePayuniWebhook, "/simulator/lookup", proxy.FailClientFacing))
	}
}
