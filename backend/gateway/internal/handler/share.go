package handler

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

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

// websitePostPageSize 必須與 hawkr-website/lib/api.ts fetchSellerPosts 的 page_size 一致。
// 改動時兩邊要同步，否則分享連結的 ?page=N 會指到錯誤的頁面。
const websitePostPageSize = 10

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
// @Param        type  query  string  true  "資源類型（post, product）"
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
	case "post":
		h.getPostShareURL(c, resourceID)
	case "product":
		h.getProductShareURL(c, resourceID)
	default:
		response.BadRequest(c, "invalid type: must be post or product")
	}
}

func (h *Holder) getPostShareURL(c *gin.Context, id string) {
	resp, err := h.Social.GetPostDetail(c.Request.Context(), 0, id)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	p := resp.Data
	title := fmt.Sprintf("%s 的貼文", p.SellerName)
	description := truncate(p.Content, 100)

	// p.ImageKeys 值為 objectKey（49 §6.2）；OG 預覽需完整 URL，由 Gateway 組。
	var imageURL string
	if len(p.ImageKeys) > 0 {
		imageURL = mediaurl.BuildURL(h.MediaBaseURL, p.ImageKeys[0])
	} else if p.SellerLogoKey != "" {
		imageURL = mediaurl.BuildURL(h.MediaBaseURL, p.SellerLogoKey)
	}

	// p.SellerId 是 string TSID（§6 Track 1 B5）；GetPostList sellerID 與分享 URL 皆用 string。
	sellerID := p.SellerId

	// 分享網址對外一律用 store_code；解析不到時回錯誤，不退回 store_id。
	sid, parseErr := strconv.ParseInt(sellerID, 10, 64)
	if parseErr != nil {
		// 非數字的 SellerId 是上游資料壞掉，不是客戶端找不到資源。
		response.Err(c, http.StatusInternalServerError, 500, "invalid seller id from upstream")
		return
	}
	sellerResp, sErr := h.Seller.GetSellerByStoreID(c.Request.Context(), sid)
	if sErr != nil {
		response.GrpcErr(c, "seller", sErr)
		return
	}
	if sellerResp.Code != 0 {
		response.GrpcResult(c, sellerResp.Code, sellerResp.Message, nil)
		return
	}
	if sellerResp.Data == nil || sellerResp.Data.StoreCode == "" {
		response.NotFound(c, "seller store code not found")
		return
	}
	urlID := sellerResp.Data.StoreCode

	// 找出貼文在賣家列表中的位置，推算網站頁碼。
	// GetPostList 的順序需與網站一致（預期皆為 created_at DESC）。
	// 查不到時退回 page=1，至少能連到貼文列表頁面。
	page := 1
	listResp, listErr := h.Social.GetPostList(c.Request.Context(), 0, sellerID, 1, 500, "")
	if listErr == nil && listResp != nil && listResp.Code == 0 {
		for i, item := range listResp.Data {
			if item.Id == id {
				page = i/websitePostPageSize + 1
				break
			}
		}
	}

	response.OK(c, ShareURLData{
		URL:         fmt.Sprintf("%s/sellers/%s/posts?page=%d#post-%s", websiteBaseURL, urlID, page, id),
		Title:       title,
		Description: description,
		ImageURL:    imageURL,
	})
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

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
