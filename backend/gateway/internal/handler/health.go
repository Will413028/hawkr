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
	c.JSON(http.StatusOK, gin.H{"status": "ok"}) //nolint:gocritic // 10 檔 §2 envelope 例外清單第 1 條：health probe 形狀
}
