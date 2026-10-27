package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// invoiceAdminPageSizeStr 是 ADM 佇列類端點（tax_reviews／jobs／complaints／
// audit_logs／documents）的預設 page_size，買賣家面沿用 defaultPageSizeStr（10）。
// §6.1.0(9)。
const invoiceAdminPageSizeStr = "20"

// DeleteBuyerInvoiceProfileData 是端點 4 的 data。
//
// proto 的 DeleteBuyerInvoiceProfileResponse 把 deleted 掛在 envelope 頂層（沒有
// data 欄），而對外契約要求業務資料一律在 data 之下 ⇒ Gateway 在這裡重建形狀。
type DeleteBuyerInvoiceProfileData struct {
	Deleted bool `json:"deleted"`
}

// invoicePaging 解析並驗證分頁 query（§6.1.0(9)）。
//
// ⚠️ 逾越上限一律 400，**不夾住**：夾住之後回應的 pageSize 與請求不符，前端據此算
// 總頁數會算錯，而且沒有任何訊號。
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

// invoiceBoolQuery 讀 "true"/"false" 的布林 query；未帶視為 false，其餘值 400。
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
