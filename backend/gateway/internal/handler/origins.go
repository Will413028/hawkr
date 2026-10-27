package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// OriginSummaryItem 對應 GetPublicOriginsSummary 回應條目（02 §5.13）。
type OriginSummaryItem struct {
	Origin       string `json:"origin"`       // 產地字串（如「台灣」「日本」）
	ProductCount string `json:"productCount"` // 該產地下 ACTIVE 商品數（對外數字一律 string，見 docs/開發規範/Java 微服務開發規範.md §2）
}

// OriginsSummaryResponse /public/origins/summary 回應 data。
type OriginsSummaryResponse struct {
	Records []OriginSummaryItem `json:"records"`
}

// GetOriginsSummary 公開商品產地聚合（02 §5.13）。
//
// @Summary      公開商品產地聚合
// @Description  依當前 ACTIVE 商品 group by origin，回傳產地與商品數列表；不含未填產地（origin IS NULL / 空字串）。
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Param        category_id  query  string  false  "分類 ID（支援第一/二/三層，空或 0 = 全部分類，後端自動偵測層級）"
// @Param        limit        query  string  false  "上限（預設 50，最大 100；超過視為 100；最少 1）"
// @Success      200  {object}  response.Envelope{data=OriginsSummaryResponse}
// @Failure      400  {object}  response.Envelope
// @Router       /public/origins/summary [get]
func (h *Holder) GetOriginsSummary(c *gin.Context) {
	categoryID := c.DefaultQuery("category_id", "")
	limitStr := c.DefaultQuery("limit", "50")

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	resp, err := h.Product.GetPublicOriginsSummary(c.Request.Context(), categoryID, int32(limit))
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	records := make([]OriginSummaryItem, 0, len(resp.Records))
	for _, r := range resp.Records {
		records = append(records, OriginSummaryItem{
			Origin:       r.Origin,
			ProductCount: r.ProductCount,
		})
	}

	response.OK(c, OriginsSummaryResponse{
		Records: records,
	})
}
