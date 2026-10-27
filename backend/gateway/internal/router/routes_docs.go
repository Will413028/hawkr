package router

import (
	"os"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

// registerDocsRoutes 文件檢視器路由
//
// 僅在本機開發環境啟用，且 DOCS_USERNAME、DOCS_PASSWORD、DOCS_SIGN_KEY 都設定時才註冊；
// 任一為空就不註冊，請求落到 404。檔案來源是工作目錄下的 docs/（handler.docsBaseDir），而那份文件
// 只有 docker-compose-local.yml 的 `./docs:/app/docs` bind mount 會供給——container image
// 本身不含 docs/（見 .dockerignore 與 gateway/Dockerfile）。
//
// 所以這裡刻意**不含 "dev"**：k8s 的 dev 叢集跑的是 image、沒有 docs/，註冊了也只會對每個
// 檔案回 404。與其留一條看得到卻永遠失敗的路由，不如不註冊。
func registerDocsRoutes(r *gin.Engine, d *handler.Docs) {
	if !d.Enabled() {
		return
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// 僅允許本機開發環境
	switch env {
	case "local", "development":
		// POST 登入（獨立路由，不走 wildcard）
		r.POST("/docs/login", d.LoginPost)

		// GET /docs/* — 由 DocsViewer 內部判斷 login/logout/文件
		// DocsAuthMiddleware 會攔截未登���的請求（但放行 /login 路徑）
		r.GET("/docs/*filepath", d.AuthMiddleware(), d.Viewer)
	}
}
