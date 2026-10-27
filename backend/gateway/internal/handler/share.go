package handler

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
// @Param        type  query  string  true  "資源類型（post, product, seller）"
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
	case "seller":
		h.getSellerShareURL(c, resourceID)
	default:
		response.BadRequest(c, "invalid type: must be post, product, or seller")
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

	// 分享網址對外一律用 store_code（2026-07-23 起）；resolve 失敗（seller-
	// service 暫時不可達等）時退回原始 store_id——分享連結仍可用（會被網站
	// canonical redirect 導到 store_code 版），不因為這一步失敗就整支請求 503。
	urlID := sellerID
	if sid, sidErr := strconv.ParseInt(sellerID, 10, 64); sidErr == nil {
		if sellerResp, sErr := h.Seller.GetSellerByStoreID(c.Request.Context(), sid); sErr == nil && sellerResp.Code == 0 && sellerResp.Data != nil && sellerResp.Data.StoreCode != "" {
			urlID = sellerResp.Data.StoreCode
		}
	}

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

func (h *Holder) getSellerShareURL(c *gin.Context, id string) {
	sellerID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid seller id")
		return
	}

	resp, err := h.Seller.GetSellerByStoreID(c.Request.Context(), sellerID)
	if err != nil {
		response.GrpcErr(c, "seller", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	s := resp.Data
	title := s.Name

	var parts []string
	if s.City != "" || s.District != "" {
		parts = append(parts, s.City+s.District)
	}
	parts = append(parts, fmt.Sprintf("%.1f 評分", s.Rating))
	description := strings.Join(parts, " | ")

	// s.LogoKey / s.CoverKey 為 objectKey（49 §6.6）；OG 預覽需完整 URL，由 Gateway 組。
	var imageURL string
	if s.LogoKey != "" {
		imageURL = mediaurl.BuildURL(h.MediaBaseURL, s.LogoKey)
	} else if s.CoverKey != "" {
		imageURL = mediaurl.BuildURL(h.MediaBaseURL, s.CoverKey)
	}

	// s.StoreCode 理論上不應為空，但缺值時退回賣家自身 id，避免產生
	// 結尾為 /sellers/ 的殘缺網址（對稱 getPostShareURL 的同款防呆）。
	urlID := s.StoreCode
	if urlID == "" {
		urlID = strconv.FormatInt(sellerID, 10)
	}

	response.OK(c, ShareURLData{
		URL:         fmt.Sprintf("%s/sellers/%s", websiteBaseURL, urlID),
		Title:       title,
		Description: description,
		ImageURL:    imageURL,
	})
}

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
