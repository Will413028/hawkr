package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

const invoiceAdminPageSizeStr = "20"

// DeleteBuyerInvoiceProfileData 是端點 4 的 data。
//
// proto 的 DeleteBuyerInvoiceProfileResponse 把 deleted 掛在 envelope 頂層（沒有
// data 欄），而對外契約要求業務資料一律在 data 之下 ⇒ Gateway 在這裡重建形狀。
type DeleteBuyerInvoiceProfileData struct {
	Deleted bool `json:"deleted"`
}

func invoicePaging(c *gin.Context, defaultSize string) (string, string, bool) {
	page := c.DefaultQuery("page", defaultPageStr)
	size := c.DefaultQuery("page_size", defaultSize)
	if n, err := strconv.Atoi(page); err != nil || n < 1 {
		response.BadRequest(c, "page 必須是 ≥ 1 的數字字串")
		return "", "", false
	}
	if n, err := strconv.Atoi(size); err != nil || n < 1 || n > maxPageSize {
		response.BadRequest(c, "page_size 必須是 1–"+strconv.Itoa(maxPageSize)+" 的數字字串")
		return "", "", false
	}
	return page, size, true
}

func invoiceBoolQuery(c *gin.Context, key string) (bool, bool) {
	raw := c.Query(key)
	switch raw {
	case "":
		return false, true
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		response.BadRequest(c, key+" 必須是 true 或 false")
		return false, false
	}
}
