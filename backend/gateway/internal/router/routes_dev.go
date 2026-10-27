package router

import (
	"os"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

// registerDevRoutes 開發工具路由（無需認證，僅 local/dev 環境註冊）
// APP_ENV 未設定視為 production，fail closed 不註冊（issue #1397）。
func registerDevRoutes(public *gin.RouterGroup, h *handler.Holder) {
	switch os.Getenv("APP_ENV") {
	case "local", "dev", "development":
		caches := public.Group(CachesPath)
		{
			caches.POST(CachesFlush, h.FlushAllCaches)
			caches.POST(CachesFlushService, h.FlushServiceCache)
		}
	}
}
