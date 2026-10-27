package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// DelegatedStorefrontUnavailable 是 /admin/sellers/:store_id/storefront* 四條路由的 handler：店面資料已在單體的 seller
// 模組，代行店面（A10 搬進單體）完成前固定回 503，不讀也不寫舊 seller-service。路由與代行授權鏈（session、capability、
// 稽核）照舊，被授權鏈拒絕的請求仍是原本的結果。
func (h *Holder) DelegatedStorefrontUnavailable(c *gin.Context) {
	response.ServiceUnavailable(c, "storefront")
}
