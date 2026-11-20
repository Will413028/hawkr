package handler

import (
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetSellerProducts 取得指定商家的商品列表（公開 API，僅 ACTIVE）
// @Summary      取得商家的商品列表
// @Description  根據商家帳號 ID 取得該商家上架中的商品列表，支援分類篩選、關鍵字搜尋與分頁
// @Tags         商家 (Seller)
// @Accept       json
// @Produce      json
// @Param        id           path   string  true   "店家 ID（store_id）"
// @Param        category_id  query  string  false  "分類 ID"
// @Param        keyword      query  string  false  "搜尋關鍵字"
// @Param        page         query  string  false  "頁碼（預設為 1）"
// @Param        page_size    query  string  false  "每頁筆數（預設為 20）"
// @Success      200  {object}  object{code=int,data=object{records=[]ProductItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/sellers/{id}/products [get]
func (h *Holder) GetSellerProducts(c *gin.Context) {
	storeID, ok := h.requireStore(c, c.Param("id"), gateExists)
	if !ok {
		return
	}

	categoryID := c.DefaultQuery("category_id", "0")
	keyword := c.DefaultQuery("keyword", "")
	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", defaultPageSizeStr)
	hashtags := parseHashtagsQuery(c.DefaultQuery("hashtags", ""))
	matchAll := strings.EqualFold(c.DefaultQuery("match", "any"), "all")

	resp, err := h.Product.GetPublicSellerProductListWithHashtags(c.Request.Context(), storeID, categoryID, keyword, page, pageSize, hashtags, matchAll)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	base := h.MediaBaseURL
	products := make([]ProductItem, 0, len(resp.Products))
	for _, p := range resp.Products {
		item := ProductItem{
			ID:              p.Id,
			Name:            p.Name,
			Rating:          p.Rating,
			Status:          p.Status,
			CategoryID:      p.CategoryId,
			CategoryName:    p.CategoryName,
			Origin:          p.Origin,
			ImageKey:        p.ImageKey,
			ImageURL:        mediaurl.BuildURL(base, p.ImageKey),
			ImageKeys:       p.ImageKeys,
			ImageUrls:       buildMediaURLs(base, p.ImageKeys),
			PriceMin:        p.PriceMin,
			PriceMax:        p.PriceMax,
			CreatedAt:       p.CreatedAt,
			ReviewCount:     p.ReviewCount,
			StoreID:         p.StoreId,
			CategoryLv2ID:   p.CategoryLv2Id,
			CategoryLv2Name: p.CategoryLv2Name,
			Hashtags:        toHashtagItems(p.Hashtags),
		}

		// 與 /public/products 對稱（同一支 GetProductList RPC）。本處歷來只手抄
		// name/logo/store_code，漏了 city/district/address——而前端商品卡的產地是
		// `origin || seller.city`，缺 city 時店內商品卡產地就空白；改走共用 builder
		// 一併補齊。攤平便利欄同理補上，與市集/搜尋的 ProductItem 保持同形。
		if p.Seller != nil {
			item.Seller = buildSellerInfoItem(base, p.Seller)
			item.SellerName, item.SellerLogoKey, item.SellerLogoUrl = flatSellerFields(item.Seller)
		}

		products = append(products, item)
	}

	// isRestockSubscribed aggregation (issue #1005 / B-203, 39 §5.5b)
	// buyer_id-first：這裡沒有其他地方已解析過 buyerID，自己做一次 soft-fail 解析
	// （同 product.go GetProductList 的理由）。
	accountID := getOptionalAccountID(c)
	if accountID != 0 && len(products) > 0 {
		ids := make([]string, 0, len(products))
		idx := make(map[string]int, len(products))
		for i, p := range products {
			ids = append(ids, p.ID)
			idx[p.ID] = i
		}
		if buyerID, berr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID)); berr != nil {
			pkglogger.Debug(c.Request.Context(), "Failed to resolve buyer id for restock subscription aggregation", zap.Error(berr))
		} else {
			h.applyRestockSubscribed(c.Request.Context(), int64(buyerID), ids, func(pid string, subscribed bool) {
				if i, ok := idx[pid]; ok {
					products[i].IsRestockSubscribed = subscribed
				}
			})
		}
	}

	response.Page(c, products, resp.Total, resp.Page, resp.PageSize)
}
