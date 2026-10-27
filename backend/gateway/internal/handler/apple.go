package handler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// AppleDomainVerification serves the Apple Pay domain verification file.
// @Summary      Apple Pay 網域驗證
// @Description  提供 Apple Pay 網域驗證檔案，用於 Apple Developer 驗證
// @Tags         基礎設施 (Infra)
// @Produce      plain
// @Success      200  {string}  string  "驗證檔案內容"
// @Failure      404
// @Router       /.well-known/apple-developer-merchantid-domain-association.txt [get]
func (h *Holder) AppleDomainVerification(c *gin.Context) {
	content := os.Getenv("APPLE_DOMAIN_VERIFICATION")
	if content == "" {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}
