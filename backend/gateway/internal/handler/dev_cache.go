package handler

import (
	"net/http"

	"github.com/Will413028/hawkr/gateway/internal/cache"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// FlushAllCaches 清除所有服務的 Redis 快取
// @Summary      清除所有服務快取
// @Description  清除 product/cart/coupon 所有 Redis 快取（開發用，無需驗證，僅 local/dev 環境註冊）
// @Tags         開發工具 (Dev)
// @Produce      json
// @Success      200  {object}  object{code=int,message=string,data=object{product=int,cart=int,coupon=int,total=int}}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/caches/flush [post]
func (h *Holder) FlushAllCaches(c *gin.Context) {
	result, err := cache.FlushAll(c.Request.Context())
	if err != nil {
		response.Err(c, http.StatusServiceUnavailable, 503, err.Error())
		return
	}

	var total int64
	for _, count := range result {
		total += count
	}
	result["total"] = total

	response.OK(c, result)
}

// FlushServiceCache 清除指定服務的 Redis 快取
// @Summary      清除指定服務快取
// @Description  依服務名稱清除對應 Redis 快取（可用值：product, cart, coupon, address, seller, favorite）
// @Tags         開發工具 (Dev)
// @Produce      json
// @Param        service  path  string  true  "服務名稱（product/cart/coupon/address/seller/favorite）"
// @Success      200  {object}  object{code=int,message=string,data=object{total=int}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/caches/flush/{service} [post]
func (h *Holder) FlushServiceCache(c *gin.Context) {
	service := c.Param("service")

	count, err := cache.FlushService(c.Request.Context(), service)
	if err != nil {
		statusCode := http.StatusServiceUnavailable
		errCode := int32(503)
		if count == 0 && err.Error() != "Redis 未連線" {
			statusCode = http.StatusBadRequest
			errCode = 400
		}
		response.Err(c, statusCode, errCode, err.Error())
		return
	}

	response.OK(c, gin.H{
		service: count,
		"total": count,
	})
}
