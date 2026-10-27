package handler

import (
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
)

// SearchPostTagsResponse 對應 /api/v1/public/tags/suggest 回應 data。
type SearchPostTagsResponse struct {
	Items []PostTagSuggestItem `json:"items"`
}

// PostTagSuggestItem 單一建議項目。
type PostTagSuggestItem struct {
	Name      string `json:"name"`
	PostCount string `json:"postCount"`
}

// SearchPostTags 貼文 hashtag 自動補全（spec 34 §5.1）。
//
// @Summary      貼文 Hashtag 自動補全
// @Description  依 prefix 模糊查詢貼文 hashtag；mode 支援 prefix（預設）/ contain。
// @Tags         PostHashtag
// @Accept       json
// @Produce      json
// @Param        q      query  string  false  "搜尋字串（不含 #；server normalize）"
// @Param        mode   query  string  false  "搜尋模式：prefix（預設）/ contain"
// @Param        limit  query  int     false  "上限（預設 10，最大 20）"
// @Success      200  {object}  object{code=int,data=SearchPostTagsResponse}
// @Router       /public/tags/suggest [get]
func (h *Holder) SearchPostTags(c *gin.Context) {
	q := c.DefaultQuery("q", "")
	modeStr := strings.ToLower(c.DefaultQuery("mode", "prefix"))
	limit, err := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 32)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	mode := pb.HashtagSearchMode_HASHTAG_SEARCH_PREFIX
	if modeStr == "contain" {
		mode = pb.HashtagSearchMode_HASHTAG_SEARCH_CONTAIN
	}

	resp, err := h.Social.SearchHashtags(c.Request.Context(), q, mode, int32(limit))
	if err != nil {
		response.GrpcDomainErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	items := make([]PostTagSuggestItem, 0, len(resp.Data))
	for _, it := range resp.Data {
		if it == nil {
			continue
		}
		items = append(items, PostTagSuggestItem{
			Name:      it.Name,
			PostCount: it.PostCount,
		})
	}
	response.OK(c, SearchPostTagsResponse{Items: items})
}

// TrendingPostTagsResponse 對應 /api/v1/public/tags/trending 回應 data。
type TrendingPostTagsResponse struct {
	Items []TrendingPostTagItem `json:"items"`
}

// TrendingPostTagItem 單一熱門項目。
type TrendingPostTagItem struct {
	Hashtag string `json:"hashtag"`
	Count   string `json:"count"`
}

// GetTrendingPostTags 貼文熱門 hashtag（spec 34 §5.2）。
//
// @Summary      貼文熱門 Hashtag
// @Description  日期區間 distinct post 數降冪；start_from / end_to 兩端可空。
// @Tags         PostHashtag
// @Accept       json
// @Produce      json
// @Param        start_from  query  string  false  "YYYY-MM-DD"
// @Param        end_to      query  string  false  "YYYY-MM-DD"
// @Param        limit       query  int     false  "上限（預設 20，最大 50）"
// @Success      200  {object}  object{code=int,data=TrendingPostTagsResponse}
// @Router       /public/tags/trending [get]
func (h *Holder) GetTrendingPostTags(c *gin.Context) {
	startFrom := strings.TrimSpace(c.DefaultQuery("start_from", ""))
	endTo := strings.TrimSpace(c.DefaultQuery("end_to", ""))
	limit, err := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 32)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	resp, err := h.Social.GetTrendingHashtags(c.Request.Context(), startFrom, endTo, int32(limit))
	if err != nil {
		response.GrpcDomainErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	items := make([]TrendingPostTagItem, 0, len(resp.Data))
	for _, it := range resp.Data {
		if it == nil {
			continue
		}
		items = append(items, TrendingPostTagItem{
			Hashtag: it.Hashtag,
			Count:   it.Count,
		})
	}
	response.OK(c, TrendingPostTagsResponse{Items: items})
}
