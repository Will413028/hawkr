package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

// registerMediaRoutes 配置媒體相關路由
// 依設計書 3.1，Media Service 提供以下外部端點：
// - gRPC: CreateUploadSession, CompleteUploadSession
// - HTTP Proxy: secure_upload, secure_preview, secure_download（流式傳輸）
// - 沒有通用的 admin 機敏下載或預覽：機敏檔由擁有者各自授權（例如 invoice 的稅籍佐證端點）
func registerMediaRoutes(public *gin.RouterGroup, app *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 需要認證的媒體操作
	// ------------------------------------------
	appMedia := app.Group(MediaPath)
	appMedia.Use(middleware.JWTAuth())
	{
		// 上傳會話（gRPC）
		appMedia.POST(MediaUploadSessions, h.CreateUploadSession)
		appMedia.POST(MediaUploadComplete, h.CompleteUploadSession)

		// 機敏檔案操作（HTTP Proxy 至 Media Service）。
		// FailClientFacing：消費者是 web／admin／App 自己，上游不可達要讓它們拿得到
		// envelope（5xx 的 body 會被 Cloudflare 換掉），見 proxy.UpstreamFailureStatus。
		appMedia.PUT(MediaSecureUpload, proxy.CreateProxy(ServiceMedia, proxy.FailClientFacing))
		appMedia.GET(MediaSecureDownload, proxy.CreateProxy(ServiceMedia, proxy.FailClientFacing))
		appMedia.GET(MediaSecurePreview, proxy.CreateProxy(ServiceMedia, proxy.FailClientFacing))
	}
}
