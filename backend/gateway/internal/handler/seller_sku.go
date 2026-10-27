package handler

import (
	"net/http"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

// SellerSkuListResponse 商品 SKU 列表回應
type SellerSkuListResponse struct {
	Skus  []SellerSkuResponse `json:"skus"`
	Total int                 `json:"total"`
}

// SellerSkuResponse 單一 SKU 回應
type SellerSkuResponse struct {
	Id        string `json:"id"`
	ProductId string `json:"productId"`
	SkuName   string `json:"skuName"`
	Weight    string `json:"weight"`
	Unit      string `json:"unit"`
	Price     string `json:"price"`
	StockQty  string `json:"stockQty"`
	LengthCm  string `json:"lengthCm"`
	WidthCm   string `json:"widthCm"`
	HeightCm  string `json:"heightCm"`
}

// ==================== 對外 API ====================

// ListSellerSkus lists all SKUs for a product (public route：viewer identity
// only, never an owner claim — anonymous/buyer  只用來讓
// product-service 判斷 selfOwned，非上架中商品一律走現有存取控制)。
// GET /api/v1/products/:id/skus
// @Summary      取得商品 SKU 列表
// @Description  取得指定商品的所有 SKU，包含價格、庫存、運送方式等資訊
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Param        id  path      string  true  "商品 ID"
// @Success      200  {object}  object{code=int,data=SellerSkuListResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/products/{id}/skus [get]
func (h *Holder) ListSellerSkus(c *gin.Context) {
	productID := skuProductID(c)
	if productID == "" {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	accountID := getProductAccountID(c)
	if accountID != 0 && isSellerRole(c) {
		if resolved, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(accountID)); err == nil {
			accountID = int64(resolved)
		}
	}

	resp, err := h.Product.GetProduct(c.Request.Context(), productID, accountID)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	renderSellerSkuList(c, resp)
}

// GetSellerProductSkus 取得商品 SKU 列表（賣家管理用途，僅限商品擁有者存取）
// @Summary      取得商品 SKU 列表（賣家管理）
// @Description  取得指定商品的所有 SKU，包含價格、庫存、運送方式等資訊。僅限商品擁有者存取
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "商品 ID"
// @Success      200  {object}  object{code=int,data=SellerSkuListResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/products/{id}/skus [get]
func (h *Holder) GetSellerProductSkus(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Forbidden(c, "seller access required")
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	productID := skuProductID(c)
	if productID == "" {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	resp, err := h.Product.GetProduct(c.Request.Context(), productID, int64(storeID))
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.Code != 0 {
		response.Err(c, http.StatusBadRequest, resp.Code, resp.Message)
		return
	}
	if resp.Data.CreatedBy != strconv.FormatInt(int64(storeID), 10) {
		response.Forbidden(c, "not authorized to access this product")
		return
	}

	renderSellerSkuList(c, resp)
}

// skuProductID 從 :id 或 :product_id path 參數取得商品 ID（相容不同路由命名）。
func skuProductID(c *gin.Context) string {
	productID := c.Param("product_id")
	if productID == "" {
		productID = c.Param("id")
	}
	return productID
}

func renderSellerSkuList(c *gin.Context, resp *pb.GetProductResponse) {
	if resp.Code != 0 {
		response.Err(c, http.StatusBadRequest, resp.Code, resp.Message)
		return
	}
	skus := make([]gin.H, 0, len(resp.Data.Skus))
	for _, v := range resp.Data.Skus {
		skus = append(skus, gin.H{
			"id":        v.Id,         // UUID
			"productId": resp.Data.Id, // 商品 UUID
			"skuName":   v.SkuName,    // SKU 名稱
			"weight":    v.Weight,     // 重量
			"unit":      v.Unit,       // 單位
			"price":     v.Price,      // 價格
			"stockQty":  v.StockQty,   // 庫存
			"lengthCm":  v.LengthCm,   // 長
			"widthCm":   v.WidthCm,    // 寬
			"heightCm":  v.HeightCm,   // 高
		})
	}

	response.OK(c, gin.H{
		"skus":  skus,
		"total": len(skus),
	})
}
