package handler

import (
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
)

// SearchHashtagsResponse 對應 /api/v1/public/hashtags 與 /api/v1/app/hashtags 回應 data。
type SearchHashtagsResponse struct {
	Items []HashtagItem `json:"items"`
}

// SearchHashtags hashtag 自動補全（02 §5.12）。
//
// @Summary      Hashtag 自動補全
// @Description  依 prefix 模糊查詢 hashtag，僅回傳 product_count > 0；mode 支援 prefix（預設）/ contain（IG 風格）。
// @Tags         Hashtag
// @Accept       json
// @Produce      json
// @Param        prefix  query  string  false  "輸入文字（任意大小寫，server normalize 後查詢；空字串回空陣列）"
// @Param        mode    query  string  false  "搜尋模式：prefix（預設）/ contain"
// @Param        limit   query  string  false  "上限（預設 10，最大 50）"
// @Success      200  {object}  object{code=int,data=SearchHashtagsResponse}
// @Router       /public/hashtags [get]
func (h *Holder) SearchHashtags(c *gin.Context) {
	prefix := c.DefaultQuery("prefix", "")
	modeStr := strings.ToLower(c.DefaultQuery("mode", "prefix"))
	limitStr := c.DefaultQuery("limit", "10")

	mode := pb.HashtagSearchMode_HASHTAG_SEARCH_PREFIX
	if modeStr == "contain" {
		mode = pb.HashtagSearchMode_HASHTAG_SEARCH_CONTAIN
	}

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	resp, err := h.Product.SearchHashtags(c.Request.Context(), prefix, mode, int32(limit))
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, SearchHashtagsResponse{
		Items: toHashtagItems(resp.Items),
	})
}

// PopularHashtagItem 對應 GetPopularHashtagsResponse.records 條目。
// 33-商品 Hashtag 模組詳細設計書 §5.2.5 / §6.2。
type PopularHashtagItem struct {
	Hashtag string `json:"hashtag"` // 區間內最多 ACTIVE 商品使用的原始大小寫版本
	Count   string `json:"count"`   // 區間內 distinct ACTIVE 商品數（對外數字一律 string，見 docs/開發規範/Java 微服務開發規範.md §2）
}

// GetPopularHashtagsResponse 對應 /api/v1/public/hashtags/popular 回應 data。
type GetPopularHashtagsResponse struct {
	Records []PopularHashtagItem `json:"records"`
}

// GetPopularHashtags 區間熱門 hashtag 查詢（33-商品 Hashtag 模組詳細設計書 §5.2）。
//
// @Summary      取得區間熱門 Hashtag
// @Description  依日期區間查詢熱門 hashtag 排行，依被掛上的 distinct ACTIVE 商品數降冪排序。
// @Description  startFrom / endTo 皆可空，預設近 1 個月；兩端日期皆 inclusive；區間最大 1 年。
// @Tags         Hashtag
// @Accept       json
// @Produce      json
// @Param        startFrom  query  string  false  "區間下界（YYYY-MM-DD，含當天）"
// @Param        endTo      query  string  false  "區間上界（YYYY-MM-DD，含當天）"
// @Param        limit      query  string  false  "回傳筆數上限（預設 50，最大 50）"
// @Success      200  {object}  object{code=int,data=GetPopularHashtagsResponse}
// @Router       /public/hashtags/popular [get]
func (h *Holder) GetPopularHashtags(c *gin.Context) {
	startFrom := strings.TrimSpace(c.DefaultQuery("startFrom", ""))
	endTo := strings.TrimSpace(c.DefaultQuery("endTo", ""))
	limitStr := c.DefaultQuery("limit", "50")

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 50 {
		limit = 50
	}

	resp, err := h.Product.GetPopularHashtags(c.Request.Context(), startFrom, endTo, int32(limit))
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	records := make([]PopularHashtagItem, 0, len(resp.Records))
	for _, r := range resp.Records {
		if r == nil {
			continue
		}
		records = append(records, PopularHashtagItem{
			Hashtag: r.Hashtag,
			Count:   r.Count,
		})
	}

	response.OK(c, GetPopularHashtagsResponse{
		Records: records,
	})
}
