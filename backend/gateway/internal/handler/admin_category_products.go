package handler

import (
	"net/http"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/gin-gonic/gin"
)

// ListProductsByCategory
// @Summary 管理員取分類底下所有商品
// @Description 依分類層級自動選欄位查（L1 看 category_lv1、L2 看 category_lv2、L3 看 category），含分頁。
// @Tags 管理員 (Admin)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "分類 ID"
// @Param page query string false "頁碼（預設 1）"
// @Param page_size query string false "每頁筆數（預設 20）"
// @Success 200 {object} object{code=int,data=object{records=[]ProductItem,total=string,page=string,pageSize=string}}
// @Failure 404 {object} object{code=int,message=string} "分類不存在"
// @Router /admin/categories/{id}/products [get]
func (h *Holder) ListProductsByCategory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid category id")
		return
	}
	page := c.DefaultQuery("page", defaultPageStr)
	size := c.DefaultQuery("page_size", defaultPageSizeStr)

	resp, err := h.Product.ListProductsByCategoryForAdmin(c.Request.Context(), id, page, size)
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
			SkuNames:        p.SkuNames,
		}
		products = append(products, item)
	}

	response.Page(c, products, resp.Total, resp.Page, resp.PageSize)
}

// BatchChangeProductCategory
// @Summary 管理員批次更換商品分類（限 L3→L3）
// @Description 兩種輸入模式擇一：(productIds + targetCategoryId) 或 (sourceCategoryId + targetCategoryId)。後端強制連動更新 product 的 lv1/lv2 反正規化欄位。
// @Tags 管理員 (Admin)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body BatchChangeProductCategoryRequest true "批次更換請求"
// @Success 200 {object} object{code=int,data=object{affectedCount=string}} "更換成功"
// @Failure 400 {object} object{code=int,message=string} "層級錯誤或輸入互斥"
// @Router /admin/products/batch-change-category [post]
func (h *Holder) BatchChangeProductCategory(c *gin.Context) {
	updatedBy, ok := getAccountID(c)
	if !ok {
		response.Err(c, http.StatusUnauthorized, 401, "未授權")
		return
	}
	var req BatchChangeProductCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Product.BatchChangeProductCategoryForAdmin(
		c.Request.Context(), req.ProductIDs, req.SourceCategoryID, req.TargetCategoryID, updatedBy)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, map[string]interface{}{"affectedCount": resp.AffectedCount})
}
