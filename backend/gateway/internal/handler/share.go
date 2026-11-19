package handler

import (
	"fmt"
	"os"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/gin-gonic/gin"
)

var websiteBaseURL = getWebsiteBaseURL()

func getWebsiteBaseURL() string {
	if v := os.Getenv("WEBSITE_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:3000"
}

// ShareURLData 分享連結回應
type ShareURLData struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
}

// GetShareURL 取得分享連結
// @Summary      取得分享連結
// @Description  根據類型與 ID 產生分享用網頁 URL 及 Open Graph 預覽資訊
// @Tags         分享 (Share)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        type  query  string  true  "資源類型（product）"
// @Param        id    query  string  true  "資源 ID"
// @Success      200  {object}  object{code=int,data=ShareURLData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/share_url [get]
func (h *Holder) GetShareURL(c *gin.Context) {
	resourceType := c.Query("type")
	resourceID := c.Query("id")

	if resourceType == "" || resourceID == "" {
		response.BadRequest(c, "type and id are required")
		return
	}

	switch resourceType {
	case "product":
		h.getProductShareURL(c, resourceID)
	default:
		response.BadRequest(c, "invalid type: must be product")
	}
}

func (h *Holder) getProductShareURL(c *gin.Context, id string) {
	resp, err := h.Product.GetPublicProduct(c.Request.Context(), id)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	p := resp.Data
	title := p.Name
	var description string
	if p.Seller != nil && p.Seller.Name != "" {
		if p.PriceMin != "" {
			description = fmt.Sprintf("NT$%s - %s", p.PriceMin, p.Seller.Name)
		} else {
			description = p.Seller.Name
		}
	} else if p.PriceMin != "" {
		description = fmt.Sprintf("NT$%s", p.PriceMin)
	}

	response.OK(c, ShareURLData{
		URL:         fmt.Sprintf("%s/products/%s", websiteBaseURL, p.Id),
		Title:       title,
		Description: description,
		ImageURL:    mediaurl.BuildURL(h.MediaBaseURL, p.ImageKey), // 49 §6.6：分享圖走 /i/original（S6），Gateway 由 objectKey 組
	})
}
