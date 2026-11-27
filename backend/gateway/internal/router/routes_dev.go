package router

import (
	"os"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

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
