package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/gin-gonic/gin"
)

// UpdateProductStatusRequest 管理員更新商品狀態請求
type UpdateProductStatusRequest struct {
	Status string `json:"status" binding:"required"` // 目標狀態碼 (4003=下架, 4004=上架)
}

// SearchAdminProducts 管理員查詢商品列表（不限賣家、不限狀態）
// GET /api/v1/admin/products
// @Summary      管理員查詢商品列表
// @Description  管理員查詢所有商品，可依關鍵字、賣家、狀態篩選，不限賣家與狀態
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        keyword      query     string  false  "搜尋關鍵字"
// @Param        owner_id     query     string  false  "賣家帳號 ID（數字字串；會解析為現任 store surrogate 過濾）"
// @Param        status       query     string  false  "商品狀態（4001=草稿, 4002=審核中, 4003=未上架, 4004=已上架）"
// @Param        category_id  query     string  false  "第二層分類 ID"
// @Param        page         query     string  false  "頁碼（預設 1）"
// @Param        page_size    query     string  false  "每頁筆數（預設 10）"
// @Success      200          {object}  object{code=int,data=object{records=[]ProductItem,total=string,page=string,pageSize=string}}
// @Failure      400          {object}  object{code=int,message=string}
// @Failure      403          {object}  object{code=int,message=string}
// @Router       /admin/products [get]
func (h *Holder) SearchAdminProducts(c *gin.Context) {
	page := c.DefaultQuery("page", defaultPageStr)
	size := c.DefaultQuery("page_size", defaultPageSizeStr)
	status := c.DefaultQuery("status", "")
	ownerID, _ := strconv.ParseInt(c.DefaultQuery("owner_id", "0"), 10, 64)
	categoryID := c.DefaultQuery("category_id", "0")
	keyword := c.DefaultQuery("keyword", "")

	if ownerID != 0 {
		resolved, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(ownerID))
		if err != nil {
			if errors.Is(err, storeid.ErrNoCurrentStore) {
				// 該帳號目前沒有現任店（已刪除未重開）→ 沒有「目前」商品可言，
				// 回空清單而非 403：admin 查詢容許查到「沒店」的帳號。
				response.Page(c, []ProductItem{}, "0", page, size)
				return
			}
			response.GrpcErr(c, "seller", err)
			return
		}
		ownerID = int64(resolved)
	}

	// Hashtag 精確篩選（02 §4.1.3 / §3.9）
	var hashtags []string
	if raw := c.DefaultQuery("hashtags", ""); raw != "" {
		for _, h := range strings.Split(raw, ",") {
			if t := strings.TrimSpace(h); t != "" {
				hashtags = append(hashtags, t)
			}
		}
	}
	matchAll := strings.EqualFold(c.DefaultQuery("match", "any"), "all")

	resp, err := h.Product.SearchAdminProductsWithHashtags(c.Request.Context(), page, size, status, ownerID, categoryID, keyword, hashtags, matchAll)
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
		products = append(products, ProductItem{
			ID:           p.Id,
			Name:         p.Name,
			Rating:       p.Rating,
			Status:       p.Status,
			CategoryID:   p.CategoryId,
			CategoryName: p.CategoryName,
			Origin:       p.Origin,
			ImageKey:     p.ImageKey,
			ImageURL:     mediaurl.BuildURL(base, p.ImageKey),
			ImageKeys:    p.ImageKeys,
			ImageUrls:    buildMediaURLs(base, p.ImageKeys),
			PriceMin:     p.PriceMin,
			PriceMax:     p.PriceMax,
			CreatedAt:    p.CreatedAt,
			ReviewCount:  p.ReviewCount,
			StoreID:      p.StoreId,
			Hashtags:     toHashtagItems(p.Hashtags),
			// 反正規化欄位（02 §3.2 / §5.14；本期新增，與 /public/products / /app/products 對齊）
			Stock:           p.StockQty,
			SalesCount:      p.SalesCount,
			FavoriteCount:   p.FavoriteCount,
			DeliveryTemp:    nilSafeSlice(p.DeliveryTemp),
			ShippingMethods: toShippingMethodResponses(p.ShippingMethods),
		})
	}

	response.Page(c, products, resp.Total, resp.Page, resp.PageSize)
}

// GetAdminProductDetail 管理員取得商品詳情（不限擁有者、不限狀態）
// GET /api/v1/admin/products/:id
// @Summary      管理員取得商品詳情
// @Description  管理員取得任意商品的完整詳情，不檢查擁有者，不限狀態
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "商品 ID（TSID 數字字串）"
// @Success      200  {object}  object{code=int,data=ProductDetailItem}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/products/{id} [get]
func (h *Holder) GetAdminProductDetail(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	resp, err := h.Product.GetProductForAdmin(c.Request.Context(), productID)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, convertToProductDetail(h.MediaBaseURL, resp.Data))
}

// UpdateAdminProductStatus 管理員更新商品狀態（上架/下架）
// PUT /api/v1/admin/products/:id/status
// @Summary      管理員更新商品狀態
// @Description  管理員更新指定商品的狀態（上架 / 下架）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                           true  "商品 ID"
// @Param        request  body      UpdateProductStatusRequest  true  "更新商品狀態請求（status: 4003=下架, 4004=上架）"
// @Success      200      {object}  object{code=int,data=object}
// @Failure      400      {object}  object{code=int,message=string}
// @Failure      403      {object}  object{code=int,message=string}
// @Failure      404      {object}  object{code=int,message=string}
// @Router       /admin/products/{id}/status [put]
func (h *Holder) UpdateAdminProductStatus(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	var req UpdateProductStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Product.UpdateProductStatus(c.Request.Context(), productID, req.Status)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, convertToProductDetail(h.MediaBaseURL, resp.Data))
}
