package router

import (
	"os"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerDocsRoutes(r *gin.Engine, d *handler.Docs) {
	if !d.Enabled() {
		return
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	switch env {
	case "local", "development":
		r.POST("/docs/login", d.LoginPost)

		r.GET("/docs/*filepath", d.AuthMiddleware(), d.Viewer)
	}
}
