package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RecordProductView records a product view event (anonymous web traffic).
// @Summary      記錄商品瀏覽（匿名）
// @Description  網頁端在商品詳情頁 mount 時呼叫，記錄一次匿名瀏覽事件；失敗時靜默忽略
// @Tags         商品 (Product)
// @Produce      json
// @Param        id   path      string  true  "商品 ID"
// @Success      200  {object}  object{code=int}
// @Router       /public/products/{id}/view [post]
func (h *Holder) RecordProductView(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		response.OK(c, nil)
		return
	}

	if _, err := h.Seller.RecordProductView(c.Request.Context(), productID); err != nil {
		pkglogger.Warn(c.Request.Context(), "product_view: failed to record view", zap.String("product_id", productID), zap.Error(err))
	}

	response.OK(c, nil)
}
