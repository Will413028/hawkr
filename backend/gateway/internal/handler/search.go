package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProductHitData represents a product in search results.
// Mirrors product-service ProductListItem so frontend deserialization
// of /public/search/products is identical to /public/products.
//
// JSON key 一律 camelCase（§2 API 設計規範：URL/Query 用 snake_case、JSON body 用
// camelCase）。這裡曾是全站唯一一組 snake_case body key，使同一張商品卡在
// /public/products 讀 categoryId、在 /public/search/products 得改讀 category_id ——
// 「與 /public/products 相同」的設計意圖在 key 名這層從未兌現。
type ProductHitData struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Rating       string              `json:"rating,omitempty"`
	Status       string              `json:"status,omitempty"`
	CategoryID   string              `json:"categoryId,omitempty"`
	CategoryName string              `json:"categoryName,omitempty"`
	Origin       string              `json:"origin,omitempty"`
	Gallery      []PublicGalleryItem `json:"gallery,omitempty"` // 公開端形狀（無 assetId；02 §2.2.2）
	PriceMin     string              `json:"priceMin,omitempty"`
	PriceMax     string              `json:"priceMax,omitempty"`
	CreatedAt    string              `json:"createdAt,omitempty"`
	ReviewCount  string              `json:"reviewCount,omitempty"`
	StoreID      string              `json:"storeId,omitempty"`
	ImageKey     string              `json:"imageKey,omitempty"`  // 主圖 objectKey（SSOT；49 §6.1）
	ImageURL     string              `json:"imageUrl,omitempty"`  // 主圖顯示 URL（Gateway 組）
	ImageKeys    []string            `json:"imageKeys,omitempty"` // 所有圖片 objectKey（SSOT）
	ImageUrls    []string            `json:"imageUrls,omitempty"` // 所有圖片顯示 URL（Gateway 組）

	// 反正規化欄位（02 §3.2 / §5.14；本期新增，與 /public/products 對稱）
	Stock           string                   `json:"stock"`
	SalesCount      string                   `json:"salesCount,omitempty"`
	FavoriteCount   string                   `json:"favoriteCount"`
	DeliveryTemp    []string                 `json:"deliveryTemp,omitempty"`
	ShippingMethods []ShippingMethodResponse `json:"shippingMethods,omitempty"`

	// 商家資訊（與 /public/products 對稱：seller 物件 + flat sellerName/avatar）
	Seller        *SellerInfoItem `json:"seller,omitempty"`
	SellerName    string          `json:"sellerName,omitempty"`
	SellerLogoKey string          `json:"sellerLogoKey,omitempty"` // 商家 logo objectKey（flat 便利欄；49 §6.1）
	SellerLogoUrl string          `json:"sellerLogoUrl,omitempty"` // 商家 logo 顯示 URL（flat）

	// 當前登入買家對此商品是否有 ACTIVE 補貨通知訂閱（issue #1005 / B-203，39 §5.5b）
	// 匿名訪客或 RPC 失敗時為 false（fail-open）
	IsRestockSubscribed bool `json:"isRestockSubscribed"`
}

// SearchResultData contains paginated search results.
type SearchResultData struct {
	// 分頁 key 用 records（§16），四個計數一律 string（§4）——product-service 那頭
	// 本來就以 string 過線，是 Gateway 自己把它降級成 JSON number 的。
	Products   []ProductHitData `json:"records"`
	Total      string           `json:"total"`
	Page       string           `json:"page"`
	PageSize   string           `json:"pageSize"`
	TotalPages string           `json:"totalPages"`
}

// mapSortToProduct maps buyer-app sort options to product-service sort values.
// Unknown sort_by values fall through and let product-service default to
// created_at DESC (matches the previous behaviour where rating_desc /
// sales_desc / distance silently degraded to default ordering inside the
// retired search-service).
func mapSortToProduct(sortBy, sortOrder string) string {
	switch sortBy {
	case "price":
		if sortOrder == "asc" {
			return "price_min"
		}
		return "price_max"
	case "price_asc":
		return "price_min"
	case "price_desc":
		return "price_max"
	case "created_at":
		return "created_at"
	default:
		return ""
	}
}

// SearchProductsViaES forwards product search to product-service.
//
// Originally backed by a dedicated search-service (Elasticsearch-style gRPC).
// Retired in favour of product-service.GetPublicProductList because no
// frontend used brand_id, price_min/max, facets, highlights or score; only
// keyword + category + sort were ever consumed.
//
// @Summary      商品搜尋
// @Description  關鍵字 + 分類 + 排序商品搜尋（轉 product-service.GetPublicProductList）
// @Tags         搜尋 (Search)
// @Produce      json
// @Param        q            query  string   false  "搜尋關鍵字（別名 keyword）"
// @Param        keyword      query  string   false  "搜尋關鍵字（別名 q）"
// @Param        category_id  query  string   false  "分類 ID"
// @Param        sort_by      query  string   false  "排序欄位（price / price_asc / price_desc / created_at；其他值退化為 created_at DESC）"
// @Param        sort_order   query  string   false  "排序方向（asc 或 desc，預設 desc）"
// @Param        page         query  int      false  "頁碼（預設為 1）"
// @Param        page_size    query  int      false  "每頁筆數（預設為 20）"
// @Success      200  {object}  object{code=int,data=SearchResultData}
// @Router /public/search/products [get]
func (h *Holder) SearchProductsViaES(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		keyword = c.Query("q")
	}
	categoryID := c.DefaultQuery("category_id", "0")
	page := c.DefaultQuery("page", defaultPageStr)
	size := c.DefaultQuery("page_size", defaultPageSizeStr)
	sort := mapSortToProduct(c.Query("sort_by"), c.DefaultQuery("sort_order", "desc"))

	resp, err := h.Product.GetPublicProductList(c.Request.Context(), categoryID, keyword, page, size, sort)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	result := buildSearchResult(h.MediaBaseURL, resp)

	// isRestockSubscribed aggregation (issue #1005 / B-203, 39 §5.5b)
	// buyer_id-first：這裡沒有其他地方已解析過 buyerID，自己做一次 soft-fail 解析
	// （同 product.go GetProductList 的理由：ErrNoCurrentBuyer 等同無 stint，略過
	// 聚合、不阻斷搜尋結果）。
	accountID := getOptionalAccountID(c)
	if accountID != 0 && len(result.Products) > 0 {
		ids := make([]string, 0, len(result.Products))
		idx := make(map[string]int, len(result.Products))
		for i, p := range result.Products {
			ids = append(ids, p.ID)
			idx[p.ID] = i
		}
		if buyerID, berr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID)); berr != nil {
			pkglogger.Debug(c.Request.Context(), "Failed to resolve buyer id for restock subscription aggregation", zap.Error(berr))
		} else {
			h.applyRestockSubscribed(c.Request.Context(), int64(buyerID), ids, func(pid string, subscribed bool) {
				if i, ok := idx[pid]; ok {
					result.Products[i].IsRestockSubscribed = subscribed
				}
			})
		}
	}

	response.OK(c, result)
}

// GetSearchSuggestions returns keyword auto-complete suggestions.
//
// Implementation: query product-service.GetPublicProductList with the prefix
// as keyword and return distinct product names. Uses substring match
// (`name ILIKE %prefix%`) — wider than prefix match, acceptable for
// autocomplete UX.
//
// @Summary      搜尋建議
// @Description  依輸入前綴回傳商品名稱建議
// @Tags         搜尋 (Search)
// @Produce      json
// @Param        q      query  string  false  "搜尋前綴（別名 prefix）"
// @Param        prefix query  string  false  "搜尋前綴（別名 q）"
// @Param        limit  query  int     false  "回傳筆數上限（預設為 10）"
// @Success      200  {object}  object{code=int,data=object{suggestions=[]string}}
// @Router /public/search/suggest [get]
func (h *Holder) GetSearchSuggestions(c *gin.Context) {
	prefix := c.Query("q")
	if prefix == "" {
		prefix = c.Query("prefix")
	}
	if prefix == "" {
		response.OK(c, map[string]interface{}{"suggestions": []string{}})
		return
	}
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)), 10, 32)
	if limit <= 0 {
		limit = defaultLimit
	}

	resp, err := h.Product.GetPublicProductList(c.Request.Context(), "0", prefix,
		"1", strconv.FormatInt(limit, 10), "")
	if err != nil {
		pkglogger.Debug(c.Request.Context(), "Suggestions: product list failed", zap.Error(err))
		response.OK(c, map[string]interface{}{"suggestions": []string{}})
		return
	}

	suggestions := make([]string, 0, len(resp.Products))
	seen := make(map[string]struct{}, len(resp.Products))
	for _, p := range resp.Products {
		if p.Name == "" {
			continue
		}
		if _, ok := seen[p.Name]; ok {
			continue
		}
		seen[p.Name] = struct{}{}
		suggestions = append(suggestions, p.Name)
	}

	response.OK(c, map[string]interface{}{"suggestions": suggestions})
}

// buildSearchResult converts product-service GetPublicProductList response
// into the SearchResultData shape /public/search/products clients expect.
func buildSearchResult(base string, resp *pb.GetProductListResponse) SearchResultData {
	products := make([]ProductHitData, 0, len(resp.Products))
	for _, p := range resp.Products {
		item := ProductHitData{
			ID:              p.Id,
			Name:            p.Name,
			Rating:          p.Rating,
			Status:          p.Status,
			CategoryID:      p.CategoryId,
			CategoryName:    p.CategoryName,
			Origin:          p.Origin,
			PriceMin:        p.PriceMin,
			PriceMax:        p.PriceMax,
			CreatedAt:       p.CreatedAt,
			ReviewCount:     p.ReviewCount,
			StoreID:         p.StoreId,
			ImageKey:        p.ImageKey,
			ImageURL:        mediaurl.BuildURL(base, p.ImageKey),
			ImageKeys:       p.ImageKeys,
			ImageUrls:       buildMediaURLs(base, p.ImageKeys),
			Stock:           p.StockQty,
			SalesCount:      p.SalesCount,
			FavoriteCount:   p.FavoriteCount,
			DeliveryTemp:    nilSafeSlice(p.DeliveryTemp),
			ShippingMethods: toShippingMethodResponses(p.ShippingMethods),
		}
		if len(p.Gallery) > 0 {
			gallery := make([]PublicGalleryItem, 0, len(p.Gallery))
			for _, g := range p.Gallery {
				// 公開搜尋（/public/search/products 匿名可達）無編輯 round-trip 場景，故不吐 assetId
				// （內部資產識別碼，02 §2.2.2）；imageKey 保留供前端組 preset URL——它是 public/ 前綴的
				// 公開路徑，且同一筆結果的 imageKeys 已列出全部 objectKey，隱藏它只會讓縮圖被迫載原圖。
				gallery = append(gallery, PublicGalleryItem{
					ImageKey: g.ObjectKey,
					ImageURL: mediaurl.BuildURL(base, g.ObjectKey), // 49 §6.6：Gateway 由 objectKey 組
				})
			}
			item.Gallery = gallery
		}

		// 商家資訊（與 /public/products 對稱；同一支 RPC 已回 seller，補映射避免搜尋結果缺 sellerName）
		if p.Seller != nil {
			item.Seller = buildSellerInfoItem(base, p.Seller)
			item.SellerName, item.SellerLogoKey, item.SellerLogoUrl = flatSellerFields(item.Seller)
		}

		products = append(products, item)
	}

	total, _ := strconv.ParseInt(resp.Total, 10, 64)
	page64, _ := strconv.ParseInt(resp.Page, 10, 32)
	pageSize64, _ := strconv.ParseInt(resp.PageSize, 10, 32)
	var totalPages int64
	if pageSize64 > 0 {
		totalPages = (total + pageSize64 - 1) / pageSize64
	}

	return SearchResultData{
		Products:   products,
		Total:      strconv.FormatInt(total, 10),
		Page:       strconv.FormatInt(page64, 10),
		PageSize:   strconv.FormatInt(pageSize64, 10),
		TotalPages: strconv.FormatInt(totalPages, 10),
	}
}
