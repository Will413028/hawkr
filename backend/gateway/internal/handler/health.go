package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthSelf checks gateway's own liveness
// @Summary      Gateway 存活檢查
// @Description  僅確認 Gateway 本身是否存活，不依賴下游服務
// @Tags         健康檢查 (Health)
// @Produce      json
// @Success      200  {object}  object{status=string}
// @Router       /health [get]
func (h *Holder) HealthSelf(c *gin.Context) {
	// 唯一登記在案的 envelope 例外（10 檔 §2「Envelope 例外清單」第 1 條）：
	// k8s liveness/readiness probe 讀的是 probe 形狀 {"status":"ok"}，不是業務 envelope。
	// 這條路徑沒有業務碼可回、也沒有前端消費者，套上 envelope 只會讓探針判讀多一層。
	c.JSON(http.StatusOK, gin.H{"status": "ok"}) //nolint:gocritic // 10 檔 §2 envelope 例外清單第 1 條：health probe 形狀
}
