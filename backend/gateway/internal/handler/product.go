package handler

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func buildMediaURLs(base string, keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	urls := make([]string, 0, len(keys))
	for _, k := range keys {
		urls = append(urls, mediaurl.BuildURL(base, k))
	}
	return urls
}

var productSortAllowed = map[string]struct{}{
	"LATEST":     {},
	"PRICE_ASC":  {},
	"PRICE_DESC": {},
	"RATING":     {},
	"SALES":      {},
}

func isValidProductSort(v string) bool {
	_, ok := productSortAllowed[v]
	return ok
}

func nilSafeSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func parseCSVQuery(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, h := range strings.Split(raw, ",") {
		t := strings.TrimSpace(h)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseCategoryIDsQuery(c *gin.Context) []string {
	raw := c.QueryArray("category_id")
	if len(raw) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, r := range raw {
		for _, p := range strings.Split(r, ",") {
			t := strings.TrimSpace(p)
			if t == "" || t == "0" {
				continue
			}
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseHashtagsQuery(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, h := range strings.Split(raw, ",") {
		t := strings.TrimSpace(h)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func getProductAccountID(c *gin.Context) int64 {
	accountIDRaw, exists := c.Get("accountId")
	if !exists {
		return 0
	}
	accountID, ok := accountIDRaw.(int64)
	if !ok {
		return 0
	}
	return accountID
}

func toShippingMethodResponses(in []*pb.ShippingMethod) []ShippingMethodResponse {
	if len(in) == 0 {
		return nil
	}
	out := make([]ShippingMethodResponse, 0, len(in))
	for _, sm := range in {
		if sm == nil {
			continue
		}
		out = append(out, ShippingMethodResponse{
			Type:          sm.Type,
			Temperature:   sm.Temperature,
			FreeThreshold: sm.FreeThreshold,
			FeePayer:      sm.FeePayer,
		})
	}
	return out
}

// ProductItem represents a product in the list
// 對應新 proto 的 ProductListItem
type ProductItem struct {
	Seller          *SellerInfoItem     `json:"seller,omitempty"`
	PriceMax        string              `json:"priceMax,omitempty"` // 最高實際售價（彙總自 SKU effective_price）
	ImageKey        string              `json:"imageKey,omitempty"` // 主圖 objectKey（SSOT；49 §6.1）
	ImageURL        string              `json:"imageUrl,omitempty"` // 主圖顯示 URL（Gateway 組）
	Name            string              `json:"name"`
	Origin          string              `json:"origin,omitempty"`
	Status          string              `json:"status"`
	CategoryID      string              `json:"categoryId,omitempty"`
	CategoryName    string              `json:"categoryName,omitempty"`
	CategoryLv2Name string              `json:"categoryLv2Name,omitempty"`
	PriceMin        string              `json:"priceMin,omitempty"`     // 最低實際售價
	ListPriceMin    string              `json:"listPriceMin,omitempty"` // 最低原價（給前端 strike-through）
	ListPriceMax    string              `json:"listPriceMax,omitempty"` // 最高原價
	SalePriceMin    string              `json:"salePriceMin,omitempty"` // 最低特價（皆無特價時為 "0"，不是空字串）
	SalePriceMax    string              `json:"salePriceMax,omitempty"` // 最高特價
	ID              string              `json:"id"`
	Rating          string              `json:"rating,omitempty"`
	CreatedAt       string              `json:"createdAt,omitempty"`
	ReviewCount     string              `json:"reviewCount,omitempty"`
	StoreID         string              `json:"storeId,omitempty"`
	SellerName      string              `json:"sellerName,omitempty"`    // 商家名稱（flat，供商品卡直接顯示）
	SellerLogoKey   string              `json:"sellerLogoKey,omitempty"` // 商家 logo objectKey（flat 便利欄，避免與 nested seller.logoKey 碰撞；49 §6.1）
	SellerLogoUrl   string              `json:"sellerLogoUrl,omitempty"` // 商家 logo 顯示 URL（flat，供商品卡直接顯示）
	SalesCount      string              `json:"salesCount,omitempty"`    // 累計銷量（35 設計書）
	CategoryLv2ID   string              `json:"categoryLv2Id,omitempty"`
	ImageKeys       []string            `json:"imageKeys,omitempty"` // 所有圖片 objectKey（SSOT；49 §6.1）
	ImageUrls       []string            `json:"imageUrls,omitempty"` // 所有圖片顯示 URL（Gateway 組）
	SkuNames        []string            `json:"skuNames,omitempty"`
	Hashtags        []HashtagItem       `json:"hashtags,omitempty"`       // 02 §3.7-§3.9
	Certifications  []CertificationItem `json:"certifications,omitempty"` // 商品認證（02 §5.3.4），供商品卡 badge
	DeliveryTemp    []string            `json:"deliveryTemp"`             // 配送溫層聯集（02 §5.3.6），無資料時為空陣列 []
	// 反正規化欄位（02 §3.2 / §5.13a / §5.14；本期新增，issue B-192 / B-193）
	Stock           string                   `json:"stock"`                     // 即時庫存（總和）
	FavoriteCount   string                   `json:"favoriteCount"`             // 收藏人次
	ShippingMethods []ShippingMethodResponse `json:"shippingMethods,omitempty"` // 運送方式聯集
	IsFavorited     bool                     `json:"isFavorited"`
	// 當前登入買家對此商品是否有 ACTIVE 補貨通知訂閱（issue #1005 / B-203，39 §5.5b）
	// 匿名訪客或 RPC 失敗時為 false（fail-open）
	IsRestockSubscribed bool `json:"isRestockSubscribed"`
}

// ProductListData contains products and pagination
type ProductListData struct {
	Total    string        `json:"total"`
	Page     string        `json:"page"`
	PageSize string        `json:"pageSize"`
	Products []ProductItem `json:"records"`
}

// GetProductList returns product list (public API)
// @Summary      取得商品列表
// @Description  公開 API，取得上架中（status=4004）的商品列表，支援分類篩選、關鍵字搜尋、排序、分頁與多種 server-side filter（02 §5.3.4）
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Param        category_id         query  string  false  "分類 ID（支援第一/二/三層；可多值聯集：CSV 如 `1,5` 或重複參數 `?category_id=1&category_id=5`；空 / 0 = 全部；上限 50 個；任一不存在回 400）"
// @Param        keyword             query  string  false  "搜尋關鍵字（多 token 空白分隔，OR 命中）"
// @Param        sort                query  string  false  "排序方式 ProductSort Value：LATEST=最新上架、PRICE_ASC=價格低到高、PRICE_DESC=價格高到低、RATING=評分最高、SALES=銷量最高（不送=LATEST）" Enums(LATEST, PRICE_ASC, PRICE_DESC, RATING, SALES)
// @Param        page                query  string  false  "頁碼（預設為 1）"
// @Param        page_size           query  string  false  "每頁筆數（預設為 20）"
// @Param        hashtags            query  string  false  "Hashtag 精確篩選（逗號分隔），server normalize 為 canonical 後比對"
// @Param        match               query  string  false  "Hashtag 多值匹配模式：any (default) / all" Enums(any, all)
// @Param        delivery_temp       query  string  false  "配送溫層篩選（逗號分隔 ShippingTemperature.value，如 3201,3202,3203）"
// @Param        price_min           query  string  false  "價格下限（新台幣元，整數字串）"
// @Param        price_max           query  string  false  "價格上限（新台幣元，整數字串）"
// @Param        store_ids              query  string  false  "多賣家篩選（逗號分隔的賣家 store_id）"
// @Param        seller_city_code       query  string  false  "商家註冊所在地 ISO 3166-2:TW（例如 TW-TPE）；與 store_ids 並存時由 product-service 取交集。商品產地請使用 origin。"
// @Param        certification_types    query  string  false  "商品認證篩選（逗號分隔 CertificationType value，命中任一即可）。合法值：3701=有機農產品、3702=產銷履歷、3703=CAS、3704=HACCP、3705=ISO、3706=GMP"
// @Param        followed_product_only  query  bool    false  "僅回當前登入使用者收藏的商品（favorites 表）；未登入回 401、登入但無目前有效 buyer 身分回 403、buyer service 不可用回 503"
// @Param        followed_seller_only   query  bool    false  "僅回當前登入使用者追蹤的賣家所有商品（seller_subscriptions 表）；未登入回 401、登入但無目前有效 buyer 身分回 403、buyer service 不可用回 503；與 store_ids 並存時取交集"
// @Success      200  {object}  response.Envelope{data=ProductListData}
// @Failure      400  {object}  response.Envelope
// @Failure      401  {object}  response.Envelope
// @Failure      403  {object}  response.Envelope
// @Failure      503  {object}  response.Envelope
// @Router       /public/products [get]
func (h *Holder) GetProductList(c *gin.Context) {
	categoryIDs := parseCategoryIDsQuery(c)
	keyword := c.DefaultQuery("keyword", "")
	page := c.DefaultQuery("page", defaultPageStr)
	size := c.DefaultQuery("page_size", defaultPageSizeStr)
	sort := strings.TrimSpace(c.DefaultQuery("sort", ""))

	if sort != "" && !isValidProductSort(sort) {
		response.BadRequest(c, "invalid sort value")
		return
	}

	hashtags := parseHashtagsQuery(c.DefaultQuery("hashtags", ""))
	matchAll := strings.EqualFold(c.DefaultQuery("match", "any"), "all")

	deliveryTemp := parseCSVQuery(c.DefaultQuery("delivery_temp", ""))
	priceMin := strings.TrimSpace(c.DefaultQuery("price_min", ""))
	priceMax := strings.TrimSpace(c.DefaultQuery("price_max", ""))
	storeIDs := parseCSVQuery(c.DefaultQuery("store_ids", ""))
	sellerCityCode := strings.TrimSpace(c.DefaultQuery("seller_city_code", ""))
	certificationTypes := parseCSVQuery(c.DefaultQuery("certification_types", ""))
	followedProductOnly := strings.EqualFold(c.DefaultQuery("followed_product_only", ""), "true")
	followedSellerOnly := strings.EqualFold(c.DefaultQuery("followed_seller_only", ""), "true")

	accountID := getProductAccountID(c)

	var viewerBuyerID int64
	if followedProductOnly || followedSellerOnly {
		buyerID, ok := h.resolveBuyerID(c)
		if !ok {
			return
		}
		viewerBuyerID = int64(buyerID)
	}

	resp, err := h.Product.GetPublicProductListFiltered(c.Request.Context(), grpc.PublicProductListFilters{
		CategoryIDs:         categoryIDs,
		Keyword:             keyword,
		Page:                page,
		PageSize:            size,
		Sort:                sort,
		Hashtags:            hashtags,
		MatchAll:            matchAll,
		DeliveryTemp:        deliveryTemp,
		PriceMin:            priceMin,
		PriceMax:            priceMax,
		StoreIDs:            storeIDs,
		SellerCityCode:      sellerCityCode,
		CertificationTypes:  certificationTypes,
		FollowedProductOnly: followedProductOnly,
		FollowedSellerOnly:  followedSellerOnly,
		ViewerBuyerID:       viewerBuyerID,
	})
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
			ListPriceMin:    p.ListPriceMin,
			ListPriceMax:    p.ListPriceMax,
			SalePriceMin:    p.SalePriceMin,
			SalePriceMax:    p.SalePriceMax,
			CreatedAt:       p.CreatedAt,
			ReviewCount:     p.ReviewCount,
			StoreID:         p.StoreId,
			SalesCount:      p.SalesCount,
			CategoryLv2ID:   p.CategoryLv2Id,
			CategoryLv2Name: p.CategoryLv2Name,
			SkuNames:        p.SkuNames,
			Hashtags:        toHashtagItems(p.Hashtags),
			Certifications:  toCertificationItems(p.Certifications),
			DeliveryTemp:    nilSafeSlice(p.DeliveryTemp),
			Stock:           p.StockQty,
			FavoriteCount:   p.FavoriteCount,
			ShippingMethods: toShippingMethodResponses(p.ShippingMethods),
		}

		if p.Seller != nil {
			item.Seller = buildSellerInfoItem(base, p.Seller)
			item.SellerName, item.SellerLogoKey, item.SellerLogoUrl = flatSellerFields(item.Seller)
		}

		products = append(products, item)
	}

	if accountID != 0 && len(products) > 0 {
		productIDStrs := make([]string, 0, len(products))
		for _, p := range products {
			productIDStrs = append(productIDStrs, p.ID)
		}

		if len(productIDStrs) > 0 {
			buyerID, berr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
			if berr != nil {
				pkglogger.Debug(c.Request.Context(), "Failed to resolve buyer id for favorites aggregation", zap.Error(berr))
			} else if favoritedIDs, err := h.Buyer.BatchCheckFavorites(c.Request.Context(), int64(buyerID), productIDStrs); err != nil {
				pkglogger.Debug(c.Request.Context(), "Failed to batch check favorites", zap.Error(err))
			} else {
				favSet := make(map[string]struct{}, len(favoritedIDs))
				for _, fid := range favoritedIDs {
					favSet[fid] = struct{}{}
				}
				for i := range products {
					if _, ok := favSet[products[i].ID]; ok {
						products[i].IsFavorited = true
					}
				}
			}

			subscribedIdx := make(map[string]int, len(products))
			for i := range products {
				subscribedIdx[products[i].ID] = i
			}
			h.applyRestockSubscribed(c.Request.Context(), int64(buyerID), productIDStrs, func(pid string, subscribed bool) {
				if i, ok := subscribedIdx[pid]; ok {
					products[i].IsRestockSubscribed = subscribed
				}
			})
		}
	}

	response.Page(c, products, resp.Total, resp.Page, resp.PageSize)
}

// ProductDescriptionResponse 商品描述回應結構（圖文混排）。
// 賣家 / Admin 端專用；公開端形狀見 PublicProductDescription。
type ProductDescriptionResponse struct {
	Version string                     `json:"version,omitempty"`
	Blocks  []DescriptionBlockResponse `json:"blocks,omitempty"`
}

// PublicProductDescription 公開端（匿名 PDP）的商品描述，blocks 走 PublicDescriptionBlock。
type PublicProductDescription struct {
	Version string                   `json:"version,omitempty"`
	Blocks  []PublicDescriptionBlock `json:"blocks,omitempty"`
}

// InlineRunDTO 行內片段（inline run；02 §3.6 inline-run 模型）。
// text block 的 runs 與 table cell 的 runs 皆用此型別；input / response 共用。
type InlineRunDTO struct {
	Text      string `json:"text"`
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
	Color     string `json:"color,omitempty"` // #RRGGBB
	Href      string `json:"href,omitempty"`  // http(s):// 絕對 URL
}

// DescriptionBlockResponse 描述區塊回應結構（賣家 / Admin 端）
type DescriptionBlockResponse struct {
	Type string `json:"type"`         // 區塊類型：text / table / list / image
	ID   string `json:"id,omitempty"` // 區塊識別碼
	// 文字類型欄位
	Runs    []InlineRunDTO         `json:"runs,omitempty"`    // 行內 runs（inline-run 模型）
	Content string                 `json:"content,omitempty"` // legacy 純文字
	Style   map[string]interface{} `json:"style,omitempty"`   // 樣式物件（從 proto style_json 反序列化）
	// 表格類型欄位
	TableJson string `json:"tableJson,omitempty"` // 表格資料 JSON 字串
	// 清單類型欄位
	ListJson string `json:"listJson,omitempty"` // 清單資料 JSON 字串
	// 圖片類型欄位
	AssetID  string `json:"assetId,omitempty"`  // 資產 ID
	ImageKey string `json:"imageKey,omitempty"` // 物件儲存路徑（49 §6.1：帶一張圖的物件一律 imageKey/imageUrl）
	ImageURL string `json:"imageUrl,omitempty"` // 完整網址（Gateway 由 imageKey 組）
	Width    string `json:"width,omitempty"`    // 圖片寬度（數字字串）
	Height   string `json:"height,omitempty"`   // 圖片高度（數字字串）
	Alt      string `json:"alt,omitempty"`      // 替代文字
}

// PublicDescriptionBlock 公開端（匿名 PDP）的描述區塊單筆。
//
// 與賣家 / Admin 端的 DescriptionBlockResponse 差在**不含 assetId**：assetId 是媒體資產
// TSID（內部識別碼），只有賣家編輯 round-trip 需要（02 §2.2.2）。objectKey 保留——描述
// 內嵌圖的 objectKey 不在 imageKeys 裡，清掉之後公開端的描述圖就永遠只能載 /i/original/，
// 而且救不回來。
//
// 為什麼不共用 DescriptionBlockResponse 再於執行期把 assetId 清空：那會讓 swagger 宣告出
// 一個公開端永不出現的欄位，前端照文件接必拿 undefined。型別分離後宣告＝實際輸出。
// ⚠️ 新增區塊欄位時兩個型別都要加，TestPublicDTOFieldParity 會擋漏掉的那一邊。
type PublicDescriptionBlock struct {
	Type string `json:"type"`         // 區塊類型：text / table / list / image
	ID   string `json:"id,omitempty"` // 區塊識別碼
	// 文字類型欄位
	Runs    []InlineRunDTO         `json:"runs,omitempty"`    // 行內 runs（inline-run 模型）
	Content string                 `json:"content,omitempty"` // legacy 純文字
	Style   map[string]interface{} `json:"style,omitempty"`   // 樣式物件（從 proto style_json 反序列化）
	// 表格類型欄位
	TableJson string `json:"tableJson,omitempty"` // 表格資料 JSON 字串
	// 清單類型欄位
	ListJson string `json:"listJson,omitempty"` // 清單資料 JSON 字串
	// 圖片類型欄位（無 assetId）
	ImageKey string `json:"imageKey,omitempty"` // 物件儲存路徑（public/ 前綴；前端據此組 preset URL）
	ImageURL string `json:"imageUrl,omitempty"` // 完整網址（Gateway 由 imageKey 組）
	Width    string `json:"width,omitempty"`    // 圖片寬度（數字字串）
	Height   string `json:"height,omitempty"`   // 圖片高度（數字字串）
	Alt      string `json:"alt,omitempty"`      // 替代文字
}

// ProductDetailItem represents product detail
// 對應新 proto 的 ProductDetail
type ProductDetailItem struct {
	// 圖文混排描述（含內嵌圖的 assetId，供賣家編輯 round-trip；公開端改用 PublicProductDescription，
	// 不含 assetId，見 02 §2.2.2）
	Description      *ProductDescriptionResponse `json:"description,omitempty"`
	Origin           string                      `json:"origin,omitempty"`
	ImageKey         string                      `json:"imageKey,omitempty"` // 主圖 objectKey（SSOT；49 §6.1）
	ImageURL         string                      `json:"imageUrl,omitempty"` // 主圖顯示 URL（Gateway 組）
	CategoryLv1Name  string                      `json:"categoryLv1Name,omitempty"`
	CategoryLv1ID    string                      `json:"categoryLv1Id,omitempty"`
	CategoryLv2Name  string                      `json:"categoryLv2Name,omitempty"`
	ID               string                      `json:"id"`
	CategoryLv2ID    string                      `json:"categoryLv2Id,omitempty"`
	Rating           string                      `json:"rating,omitempty"`
	Status           string                      `json:"status"`
	PublicationState string                      `json:"publicationState"`
	Purchasable      bool                        `json:"purchasable"`
	CategoryID       string                      `json:"categoryId,omitempty"`
	CategoryLv3Name  string                      `json:"categoryLv3Name,omitempty"`
	Name             string                      `json:"name"`
	UpdatedAt        string                      `json:"updatedAt,omitempty"`
	PriceMin         string                      `json:"priceMin,omitempty"`     // 最低實際售價
	PriceMax         string                      `json:"priceMax,omitempty"`     // 最高實際售價
	ListPriceMin     string                      `json:"listPriceMin,omitempty"` // 最低原價
	ListPriceMax     string                      `json:"listPriceMax,omitempty"` // 最高原價
	SalePriceMin     string                      `json:"salePriceMin,omitempty"` // 最低特價
	SalePriceMax     string                      `json:"salePriceMax,omitempty"` // 最高特價
	CreatedAt        string                      `json:"createdAt,omitempty"`
	CategoryName     string                      `json:"categoryName,omitempty"`
	CreatedBy        string                      `json:"createdBy,omitempty"`
	CategoryLv3ID    string                      `json:"categoryLv3Id,omitempty"`
	SalesCount       string                      `json:"salesCount,omitempty"` // 累計銷量（35 設計書）
	// 反正規化欄位（02 §3.2 / §5.14；本期新增）
	Stock           string                   `json:"stock"`         // 即時庫存（總和），對應 product.stock_qty
	FavoriteCount   string                   `json:"favoriteCount"` // 收藏人次，對應 product.favorite_count
	ReviewCount     string                   `json:"reviewCount"`   // 評論數，對應 product.review_count（#1156）
	Certifications  []CertificationItem      `json:"certifications,omitempty"`
	Gallery         []GalleryItem            `json:"gallery,omitempty"`   // 結構化圖片集（含 assetId，供賣家編輯 round-trip；公開端改用 PublicGalleryItem，不含 assetId，見 02 §2.2.2）
	ImageKeys       []string                 `json:"imageKeys,omitempty"` // 所有圖片 objectKey（SSOT；49 §6.1）
	ImageUrls       []string                 `json:"imageUrls,omitempty"` // 所有圖片顯示 URL（Gateway 組）
	ShippingMethods []ShippingMethodResponse `json:"shippingMethods,omitempty"`
	DeliveryTemp    []string                 `json:"deliveryTemp,omitempty"`
	Skus            []SkuItem                `json:"skus,omitempty"`
	Hashtags        []HashtagItem            `json:"hashtags,omitempty"` // 02 §3.7-§3.9
}

// PublicGalleryItem 公開端（匿名 PDP / 公開搜尋）的圖片集單筆。
//
// 與賣家 / Admin 端的 GalleryItem 差在**不含 assetId**：assetId 是媒體資產 TSID（內部識別碼），
// 只有賣家編輯 round-trip 需要（02 §2.2.2）。imageKey 保留——它是 public/ 前綴的公開物件路徑，
// image Worker 本來就對任何人服務，且同一份回應的 imageKeys 早已列出全部 gallery objectKey，
// 隱藏它不構成任何保護；反過來清掉它會讓公開端只能吃 /i/original/，商品描述內嵌圖
// （objectKey 不在 imageKeys 內）永遠無法走 preset 壓縮。
//
// 公開端與賣家端形狀本來就不同，各自一個型別，swagger 宣告才會等於執行期實際輸出。
type PublicGalleryItem struct {
	ImageKey string `json:"imageKey"` // 物件儲存路徑（public/ 前綴；前端據此組 preset URL）
	ImageURL string `json:"imageUrl"` // 完整 URL（Gateway 由 imageKey 組）
}

// PublicProductDetailItem 公開商品詳情（含 seller + reviews + isFavorited + isRestockSubscribed + selfOwned）
type PublicProductDetailItem struct {
	ProductDetailItem
	// 遮蔽內嵌 ProductDetailItem 的同名欄位（encoding/json：同名欄位取較淺層），
	// 公開端形狀不含 assetId。⚠️ 拿掉這兩個外層欄位任一個，內嵌的賣家形狀會立刻接手
	// 序列化並把 assetId 吐出去——TestGalleryAssetIDVisibility 的「公開 JSON 不得含
	// assetId」斷言會擋下（key 計數斷言擋的是另一種：外層在、但序列化出兩份同名 key）。
	Gallery             []PublicGalleryItem       `json:"gallery,omitempty"`
	Description         *PublicProductDescription `json:"description,omitempty"`
	Seller              *SellerInfoItem           `json:"seller,omitempty"`
	Reviews             []ReviewPreviewItem       `json:"reviews"`             // 空陣列不省略
	IsFavorited         bool                      `json:"isFavorited"`         // 是否已收藏（未登入一律 false）
	IsRestockSubscribed bool                      `json:"isRestockSubscribed"` // 是否已訂閱補貨通知（未登入一律 false；02 §5.13b / #942）
	SelfOwned           bool                      `json:"selfOwned"`           // 當前登入者是否為商品建立者（匿名一律 false，前端據此 disable 加入購物車按鈕；§2.2.1）
}

// SellerInfoItem 商家資訊
type SellerInfoItem struct {
	Name      *string `json:"name"`
	LogoKey   *string `json:"logoKey,omitempty"`   // 商家 Logo objectKey（SSOT；49 §6.1）
	LogoUrl   *string `json:"logoUrl,omitempty"`   // 商家 Logo 顯示 URL（Gateway 組）
	Address   *string `json:"address,omitempty"`   // 商家地址（city + district 合成）
	City      *string `json:"city,omitempty"`      // 城市（如「台北市」）
	District  *string `json:"district,omitempty"`  // 區域（如「大安區」）
	StoreCode *string `json:"storeCode,omitempty"` // 賣家短碼
	StoreID   string  `json:"storeId"`
}

func buildSellerInfoItem(base string, s *pb.SellerInfo) *SellerInfoItem {
	if s == nil {
		return nil
	}
	si := &SellerInfoItem{StoreID: s.StoreId}
	if s.Name != "" {
		name := s.Name
		si.Name = &name
	}
	if s.LogoKey != "" {
		logoKey := s.LogoKey
		logoURL := mediaurl.BuildURL(base, s.LogoKey)
		si.LogoKey = &logoKey
		si.LogoUrl = &logoURL
	}
	if s.Address != "" {
		address := s.Address
		si.Address = &address
	}
	if s.City != "" {
		city := s.City
		si.City = &city
	}
	if s.District != "" {
		district := s.District
		si.District = &district
	}
	if s.StoreCode != "" {
		storeCode := s.StoreCode
		si.StoreCode = &storeCode
	}
	return si
}

func flatSellerFields(si *SellerInfoItem) (name, logoKey, logoURL string) {
	if si == nil {
		return "", "", ""
	}
	if si.Name != nil {
		name = *si.Name
	}
	if si.LogoKey != nil {
		logoKey = *si.LogoKey
	}
	if si.LogoUrl != nil {
		logoURL = *si.LogoUrl
	}
	return name, logoKey, logoURL
}

// ReviewPreviewItem 評論預覽
type ReviewPreviewItem struct {
	ReviewID  string `json:"reviewId"`
	AccountID string `json:"accountId"`
	Rating    string `json:"rating"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// CertificationItem represents a certification
type CertificationItem struct {
	Name   string `json:"name"`
	CertNo string `json:"certNo"`
}

// SkuItem represents a product SKU
type SkuItem struct {
	ID              string                   `json:"id"` // UUID
	SkuName         string                   `json:"skuName"`
	Weight          string                   `json:"weight,omitempty"`
	Unit            string                   `json:"unit"`
	Price           string                   `json:"price"`               // 原價（list price，新台幣元，數字字串）
	SalePrice       string                   `json:"salePrice,omitempty"` // 特價（新台幣元，空字串=無特價）
	StockQty        string                   `json:"stockQty"`
	LengthCm        string                   `json:"lengthCm,omitempty"`
	WidthCm         string                   `json:"widthCm,omitempty"`
	HeightCm        string                   `json:"heightCm,omitempty"`
	ShippingMethods []ShippingMethodResponse `json:"shippingMethods,omitempty"` // 運送方式（SKU 層級）
	// Version 是這筆 SKU 的樂觀鎖版本（#2545），寫回時放進 skus[].expectedVersion。
	// 不下發它，PUT 端那把鎖就永遠沒有人拿得到鑰匙。
	Version string `json:"version,omitempty"`
}

// ShippingMethodResponse 運送方式回應結構
type ShippingMethodResponse struct {
	Type          string `json:"type"`
	Temperature   string `json:"temperature"`
	FreeThreshold string `json:"freeThreshold"`
	FeePayer      string `json:"feePayer"`
}

// GetProductDetailByID returns product detail (using :id param for v1 routes)
// 公開 API：登入時走 GetProduct（含 selfOwned 標記），匿名走 GetPublicProduct（有 Redis 快取）
// @Summary      取得商品詳情
// @Description  取得商品完整詳情（含商家資訊與評論預覽，登入時含 selfOwned 旗標），僅可查看上架中商品
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "商品 ID（UUID）"
// @Success      200  {object}  object{code=int,data=PublicProductDetailItem}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/products/{id} [get]
func (h *Holder) GetProductDetailByID(c *gin.Context) {
	productID := c.Param("id")

	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	accountID := getProductAccountID(c)
	buyerAccountID := accountID
	if accountID != 0 && isSellerRole(c) {
		if resolved, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(accountID)); err == nil {
			accountID = int64(resolved)
		}
	}
	var resp *pb.GetProductResponse
	var err error
	if isSellerRole(c) && accountID != 0 {
		resp, err = h.Product.GetProduct(c.Request.Context(), productID, accountID)
	} else {
		resp, err = h.Product.GetPublicProduct(c.Request.Context(), productID)
	}
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	data := resp.Data
	detail := convertToPublicProductDetail(h.MediaBaseURL, data)

	if buyerAccountID != 0 && detail.ID != "" {
		if _, err := strconv.ParseInt(detail.ID, 10, 64); err == nil {
			ctx := c.Request.Context()
			var (
				favorited     bool
				subscribedMap map[string]bool
			)
			buyerID, berr := h.BuyerID.CurrentBuyerID(ctx, identity.AccountID(buyerAccountID))
			g := syncutil.NewGroup(ctx)
			if berr != nil {
				pkglogger.Debug(ctx, "Failed to resolve buyer id for favorite/restock check", zap.String("productId", detail.ID), zap.Error(berr))
			} else {
				g.Go(func() error {
					f, ferr := h.Buyer.CheckFavorite(ctx, int64(buyerID), detail.ID)
					if ferr != nil {
						pkglogger.Debug(ctx, "Failed to check favorite for product", zap.String("productId", detail.ID), zap.Error(ferr))
						return nil
					}
					favorited = f
					return nil
				})
				g.Go(func() error {
					m, rerr := h.Buyer.BatchCheckRestockSubscriptions(ctx, int64(buyerID), []string{detail.ID})
					if rerr != nil {
						pkglogger.Debug(ctx, "Failed to check restock subscription", zap.String("productId", detail.ID), zap.Error(rerr))
						return nil
					}
					subscribedMap = m
					return nil
				})
			}
			_ = g.Wait()
			detail.IsFavorited = favorited
			if subscribedMap != nil {
				detail.IsRestockSubscribed = subscribedMap[detail.ID]
			}
		}
	}

	response.OK(c, detail)
}

// GetProductInfoByID returns basic product info (using :id param for v1 routes)
// 公開 API：呼叫 GetPublicProduct（有 Redis 快取，僅 ACTIVE）
// @Summary      取得商品基本資訊
// @Description  取得商品簡要資訊（不含規格、認證等詳細資料），僅可查看上架中商品
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "商品 ID（UUID）"
// @Success      200  {object}  object{code=int,data=ProductItem}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/products/{id}/brief [get]
func (h *Holder) GetProductInfoByID(c *gin.Context) {
	productID := c.Param("id")

	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	resp, err := h.Product.GetPublicProduct(c.Request.Context(), productID)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	p := resp.Data
	base := h.MediaBaseURL
	item := ProductItem{
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
		ListPriceMin: p.ListPriceMin,
		ListPriceMax: p.ListPriceMax,
		SalePriceMin: p.SalePriceMin,
		SalePriceMax: p.SalePriceMax,
		SalesCount:   p.SalesCount,
		CreatedAt:    p.CreatedAt,
	}

	accountID := getProductAccountID(c)
	if accountID != 0 {
		if buyerID, berr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID)); berr != nil {
			pkglogger.Debug(c.Request.Context(), "Failed to resolve buyer id for restock subscription aggregation", zap.Error(berr))
		} else {
			h.applyRestockSubscribed(c.Request.Context(), int64(buyerID), []string{p.Id}, func(_ string, subscribed bool) {
				item.IsRestockSubscribed = subscribed
			})
		}
	}

	response.OK(c, item)
}

// ProductRatingResponse represents product rating info
type ProductRatingResponse struct {
	Rating      string `json:"rating"`
	ReviewCount string `json:"reviewCount"`
}

// GetProductRating returns product rating
// 公開 API：呼叫 GetPublicProduct（有 Redis 快取，僅 ACTIVE）
// @Summary      取得商品評分
// @Description  取得指定商品的評分與評論數量
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "商品 ID（UUID）"
// @Success      200  {object}  object{code=int,data=ProductRatingResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/products/{id}/rating [get]
func (h *Holder) GetProductRating(c *gin.Context) {
	productID := c.Param("id")

	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	resp, err := h.Product.GetPublicProduct(c.Request.Context(), productID)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, ProductRatingResponse{
		Rating:      resp.Data.Rating,
		ReviewCount: resp.Data.ReviewCount,
	})
}

// GetRelatedProducts returns related products for a given product
// GET /api/v1/products/:id/related
// 公開 API：只返回 ACTIVE 商品
// @Summary      取得相關商品
// @Description  取得與指定商品同分類的相關商品列表（最多 8 筆，僅返回上架中商品）
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "商品 ID（UUID）"
// @Success      200  {object}  object{code=int,data=[]ProductItem}
// @Failure      400  {object}  object{code=int,message=string}
// @Router       /public/products/{id}/related [get]
func (h *Holder) GetRelatedProducts(c *gin.Context) {
	productID := c.Param("id")

	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	resp, err := h.Product.GetRelatedProducts(c.Request.Context(), productID)
	if err != nil {
		response.OK(c, []ProductItem{})
		return
	}

	if resp.Code != 0 {
		response.OK(c, []ProductItem{})
		return
	}

	base := h.MediaBaseURL
	products := make([]ProductItem, 0, len(resp.Products))
	for _, p := range resp.Products {
		products = append(products, ProductItem{
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
			ListPriceMin:    p.ListPriceMin,
			ListPriceMax:    p.ListPriceMax,
			SalePriceMin:    p.SalePriceMin,
			SalePriceMax:    p.SalePriceMax,
			SalesCount:      p.SalesCount,
			Stock:           p.StockQty,
			FavoriteCount:   p.FavoriteCount,
			ShippingMethods: toShippingMethodResponses(p.ShippingMethods),
			DeliveryTemp:    nilSafeSlice(p.DeliveryTemp),
		})
	}

	accountID := getProductAccountID(c)
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

	response.OK(c, products)
}

func convertDescriptionProtoToResponse(base string, desc *pb.ProductDescription) *ProductDescriptionResponse {
	if desc == nil {
		return nil
	}

	blocks := make([]DescriptionBlockResponse, 0, len(desc.Blocks))
	for _, b := range desc.Blocks {
		block := DescriptionBlockResponse{
			Type:      b.Type,
			ID:        b.Id,
			Content:   b.Content,
			TableJson: b.TableJson,
			ListJson:  b.ListJson,
			AssetID:   b.AssetId,
			ImageKey:  b.ObjectKey,
			ImageURL:  mediaurl.BuildURL(base, b.ObjectKey),
			Width:     b.Width,
			Height:    b.Height,
			Alt:       b.Alt,
		}

		if len(b.Runs) > 0 {
			runs := make([]InlineRunDTO, 0, len(b.Runs))
			for _, r := range b.Runs {
				runs = append(runs, InlineRunDTO{
					Text:      r.Text,
					Bold:      r.Bold,
					Italic:    r.Italic,
					Underline: r.Underline,
					Color:     r.Color,
					Href:      r.Href,
				})
			}
			block.Runs = runs
		}

		if b.StyleJson != "" {
			var style map[string]interface{}
			if err := json.Unmarshal([]byte(b.StyleJson), &style); err == nil {
				block.Style = style
			}
		}

		blocks = append(blocks, block)
	}

	return &ProductDescriptionResponse{
		Version: desc.Version,
		Blocks:  blocks,
	}
}

func toPublicDescription(desc *ProductDescriptionResponse) *PublicProductDescription {
	if desc == nil {
		return nil
	}

	blocks := make([]PublicDescriptionBlock, 0, len(desc.Blocks))
	for _, b := range desc.Blocks {
		blocks = append(blocks, PublicDescriptionBlock{
			Type:      b.Type,
			ID:        b.ID,
			Runs:      b.Runs,
			Content:   b.Content,
			Style:     b.Style,
			TableJson: b.TableJson,
			ListJson:  b.ListJson,
			ImageKey:  b.ImageKey,
			ImageURL:  b.ImageURL,
			Width:     b.Width,
			Height:    b.Height,
			Alt:       b.Alt,
		})
	}

	return &PublicProductDescription{
		Version: desc.Version,
		Blocks:  blocks,
	}
}

func convertToProductDetail(base string, data *pb.ProductDetail) ProductDetailItem {
	certifications := make([]CertificationItem, 0, len(data.Certifications))
	for _, cert := range data.Certifications {
		certifications = append(certifications, CertificationItem{
			Name:   cert.Name,
			CertNo: cert.CertNo,
		})
	}

	skus := make([]SkuItem, 0, len(data.Skus))
	for _, v := range data.Skus {
		skuShippingMethods := make([]ShippingMethodResponse, 0, len(v.ShippingMethods))
		for _, sm := range v.ShippingMethods {
			skuShippingMethods = append(skuShippingMethods, ShippingMethodResponse{
				Type:          sm.Type,
				Temperature:   sm.Temperature,
				FreeThreshold: sm.FreeThreshold,
				FeePayer:      sm.FeePayer,
			})
		}
		skus = append(skus, SkuItem{
			ID:              v.Id,
			SkuName:         v.SkuName,
			Weight:          v.Weight,
			Unit:            v.Unit,
			Price:           v.Price,
			SalePrice:       v.SalePrice,
			StockQty:        v.StockQty,
			LengthCm:        v.LengthCm,
			WidthCm:         v.WidthCm,
			HeightCm:        v.HeightCm,
			ShippingMethods: skuShippingMethods,
			Version:         v.Version,
		})
	}

	shippingMethods := make([]ShippingMethodResponse, 0, len(data.ShippingMethods))
	for _, sm := range data.ShippingMethods {
		shippingMethods = append(shippingMethods, ShippingMethodResponse{
			Type:          sm.Type,
			Temperature:   sm.Temperature,
			FreeThreshold: sm.FreeThreshold,
			FeePayer:      sm.FeePayer,
		})
	}

	gallery := make([]GalleryItem, 0, len(data.Gallery))
	for _, g := range data.Gallery {
		gallery = append(gallery, GalleryItem{
			AssetID:  g.AssetId,
			ImageKey: g.ObjectKey,
			ImageURL: mediaurl.BuildURL(base, g.ObjectKey),
		})
	}

	return ProductDetailItem{
		ID:               data.Id,
		Name:             data.Name,
		Rating:           data.Rating,
		Status:           data.Status,
		PublicationState: data.PublicationState,
		Purchasable:      data.Purchasable,
		CategoryID:       data.CategoryId,
		CategoryName:     data.CategoryName,
		Origin:           data.Origin,
		Description:      convertDescriptionProtoToResponse(base, data.Description),
		ImageKey:         data.ImageKey,
		ImageURL:         mediaurl.BuildURL(base, data.ImageKey),
		Gallery:          gallery,
		ImageKeys:        data.ImageKeys,
		ImageUrls:        buildMediaURLs(base, data.ImageKeys),
		Certifications:   certifications,
		PriceMin:         data.PriceMin,
		PriceMax:         data.PriceMax,
		ListPriceMin:     data.ListPriceMin,
		ListPriceMax:     data.ListPriceMax,
		SalePriceMin:     data.SalePriceMin,
		SalePriceMax:     data.SalePriceMax,
		Skus:             skus,
		DeliveryTemp:     data.DeliveryTemp,
		ShippingMethods:  shippingMethods,
		CreatedAt:        data.CreatedAt,
		UpdatedAt:        data.UpdatedAt,
		CreatedBy:        data.CreatedBy,
		CategoryLv3ID:    data.CategoryLv3Id,
		CategoryLv3Name:  data.CategoryLv3Name,
		CategoryLv2ID:    data.CategoryLv2Id,
		CategoryLv2Name:  data.CategoryLv2Name,
		CategoryLv1ID:    data.CategoryLv1Id,
		CategoryLv1Name:  data.CategoryLv1Name,
		SalesCount:       data.SalesCount,
		Stock:            data.StockQty,
		FavoriteCount:    data.FavoriteCount,
		ReviewCount:      data.ReviewCount,
		Hashtags:         toHashtagItems(data.Hashtags),
	}
}

func convertToPublicProductDetail(base string, data *pb.ProductDetail) PublicProductDetailItem {
	detail := convertToProductDetail(base, data)

	publicGallery := make([]PublicGalleryItem, 0, len(detail.Gallery))
	for _, g := range detail.Gallery {
		publicGallery = append(publicGallery, PublicGalleryItem{ImageKey: g.ImageKey, ImageURL: g.ImageURL})
	}
	publicDescription := toPublicDescription(detail.Description)
	detail.Gallery = nil
	detail.Description = nil
	for i := range detail.Skus {
		detail.Skus[i].Version = ""
	}

	result := PublicProductDetailItem{
		ProductDetailItem: detail,
		Gallery:           publicGallery,
		Description:       publicDescription,
		Reviews:           make([]ReviewPreviewItem, 0),
		SelfOwned:         data.SelfOwned,
	}

	result.Seller = buildSellerInfoItem(base, data.Seller)

	for _, r := range data.Reviews {
		result.Reviews = append(result.Reviews, ReviewPreviewItem{
			ReviewID:  r.ReviewId,
			AccountID: r.BuyerId,
			Rating:    r.Rating,
			Content:   r.Content,
			CreatedAt: r.CreatedAt,
		})
	}

	return result
}

// PurgeCacheRequest 清除快取請求
type PurgeCacheRequest struct {
	Scope     string `json:"scope" binding:"required"` // all/detail/list
	ProductID string `json:"productId,omitempty"`      // scope=detail 時必填
	AccountID int64  `json:"accountId,omitempty"`
}

// PurgeCacheResult 清除快取結果
type PurgeCacheResult struct {
	Scope       string `json:"scope"`
	PurgedKeys  string `json:"purgedKeys"`
	Description string `json:"description"`
}

// PurgeProductCache 清除商品快取
// POST /api/v1/products/cache/purge
// 僅限管理員使用（需要 JWT 認證）
// @Summary      清除商品快取
// @Description  管理員清除商品快取，支援清除全部、單一商品詳情或商品列表快取
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  PurgeCacheRequest  true  "清除快取請求（scope: all/detail/list）"
// @Success      200  {object}  object{code=int,message=string,data=PurgeCacheResult}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/products/cache [delete]
func (h *Holder) PurgeProductCache(c *gin.Context) {
	var req PurgeCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	if req.Scope == "list" && req.AccountID != 0 {
		resolved, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(req.AccountID))
		if err != nil {
			if errors.Is(err, storeid.ErrNoCurrentStore) {
				response.OK(c, &PurgeCacheResult{
					Scope:       req.Scope,
					PurgedKeys:  "0",
					Description: "帳號目前無現任店，無列表快取可清除",
				})
				return
			}
			response.GrpcErr(c, "seller", err)
			return
		}
		req.AccountID = int64(resolved)
	}

	resp, err := h.Product.PurgeProductCache(c.Request.Context(), req.Scope, req.ProductID, req.AccountID)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, &PurgeCacheResult{
		Scope:       resp.Data.Scope,
		PurgedKeys:  resp.Data.PurgedKeys,
		Description: resp.Data.Description,
	})
}

// VerifyCertificationResponse 認證驗證回應
type VerifyCertificationResponse struct {
	Exists bool `json:"exists" example:"true"` // 認證編號是否存在
}

const certNoMaxLength = 64

var certNoPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,` + strconv.Itoa(certNoMaxLength) + `}$`)

var verifiableCertTypes = map[string]struct{}{
	"3701": {},
	"3702": {},
	"3703": {},
}

// CertificationWarningOutput 認證未被採用的提示（每筆對應一個被剔除或被跳過的認證輸入）。
//
// 19 §9.5：無警告時回**空陣列**而不是省略欄位——省略會讓前端分不出「後端沒實作」與
// 「這次沒有警告」。它與 certifications 不重疊：後者是 approved-only 投影，本欄是這一次
// 沒被採用的輸入，前端不得把它當成投影的一部分渲染。
type CertificationWarningOutput struct {
	CertificationType string `json:"certificationType" example:"3701"`     // 原樣回送送進來的值，可能非法
	CertificateNo     string `json:"certificateNo" example:"1-009-300022"` // 同上
	Reason            string `json:"reason" example:"CONCURRENT_CONFLICT"` // INVALID_TYPE / INVALID_CERT_NO / CONCURRENT_CONFLICT
	Message           string `json:"message" example:"另一個視窗剛送出同類型的認證，這一筆未被採用，請重新儲存"`
}

const (
	certWarningInvalidType   = "INVALID_TYPE"
	certWarningInvalidCertNo = "INVALID_CERT_NO"
)

var certificationInputTypes = func() map[string]struct{} {
	values := []int{
		codemeta.CertificationTypeOrganic,
		codemeta.CertificationTypeTap,
		codemeta.CertificationTypeCas,
		codemeta.CertificationTypeHaccp,
		codemeta.CertificationTypeIso,
		codemeta.CertificationTypeGmp,
	}
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[strconv.Itoa(v)] = struct{}{}
	}
	return out
}()

func certificationInputRejection(certType, certNo string) (reason string, message string) {
	if _, ok := certificationInputTypes[certType]; !ok {
		return certWarningInvalidType, "認證類型格式不正確，這一筆未被採用"
	}
	if _, verifiable := verifiableCertTypes[certType]; verifiable {
		if !certNoPattern.MatchString(certNo) {
			return certWarningInvalidCertNo, "認證編號格式不正確（此類型僅接受英數字、連字號與底線，最長 64 字元），這一筆未被採用"
		}
		return "", ""
	}
	if !unverifiableCertNoValid(certNo) {
		return certWarningInvalidCertNo, "認證編號格式不正確（不可空白、最長 64 字元，且不得含控制字元或角括號），這一筆未被採用"
	}
	return "", ""
}

func unverifiableCertNoValid(certNo string) bool {
	trimmed := strings.TrimSpace(certNo)
	if trimmed == "" {
		return false
	}
	if utf8.RuneCountInString(trimmed) > certNoMaxLength {
		return false
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return false
		}
	}
	return true
}

func normalizedCertNo(certType, certNo string) string {
	if _, verifiable := verifiableCertTypes[certType]; verifiable {
		return certNo
	}
	return strings.TrimSpace(certNo)
}

type certificationInputRef struct {
	Type string
	No   string
}

func certificationInputDecision(c *gin.Context, inputs []certificationInputRef, productID string, storeID int64) ([]bool, []CertificationWarningOutput) {
	keep := make([]bool, len(inputs))
	warnings := make([]CertificationWarningOutput, 0)
	for i, in := range inputs {
		reason, message := certificationInputRejection(in.Type, in.No)
		if reason == "" {
			keep[i] = true
			continue
		}
		warnings = append(warnings, CertificationWarningOutput{
			CertificationType: in.Type,
			CertificateNo:     in.No,
			Reason:            reason,
			Message:           message,
		})
		pkglogger.Warn(c.Request.Context(), "certification input rejected",
			zap.String("product_id", productID),
			zap.Int64("store_id", storeID),
			zap.String("certification_type", in.Type),
			zap.String("reason", reason),
		)
	}
	return keep, warnings
}

func filterCertificationInputs(c *gin.Context, certs []*pb.Certification, productID string, storeID int64) ([]*pb.Certification, []CertificationWarningOutput) {
	compact := make([]*pb.Certification, 0, len(certs))
	for _, cert := range certs {
		if cert != nil {
			compact = append(compact, cert)
		}
	}
	refs := make([]certificationInputRef, 0, len(compact))
	for _, cert := range compact {
		refs = append(refs, certificationInputRef{Type: cert.GetName(), No: cert.GetCertNo()})
	}
	keep, warnings := certificationInputDecision(c, refs, productID, storeID)
	kept := make([]*pb.Certification, 0, len(compact))
	for i, cert := range compact {
		if !keep[i] {
			continue
		}
		kept = append(kept, &pb.Certification{
			Name:   cert.GetName(),
			CertNo: normalizedCertNo(cert.GetName(), cert.GetCertNo()),
		})
	}
	return kept, warnings
}

func mergeCertificationWarnings(local []CertificationWarningOutput, upstream []*pb.CertificationWarning) []CertificationWarningOutput {
	out := make([]CertificationWarningOutput, 0, len(local)+len(upstream))
	out = append(out, local...)
	for _, w := range upstream {
		if w == nil {
			continue
		}
		out = append(out, CertificationWarningOutput{
			CertificationType: w.GetCertificationType(),
			CertificateNo:     w.GetCertificateNo(),
			Reason:            w.GetReason(),
			Message:           w.GetMessage(),
		})
	}
	return out
}

// VerifyCertification 驗證認證編號是否存在於政府開放資料平台
// @Summary      驗證農業認證編號
// @Description  驗證賣家輸入的認證編號是否存在於農業部開放資料平台。type 為 CertificationType value：3701=有機農產品(ORGANIC)、3702=產銷履歷(TAP)、3703=CAS優良農產品(CAS)。3704/3705/3706 (HACCP/ISO/GMP) 暫不支援驗證。
// @Tags         商品 (Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        cert_no  query   string  true  "認證編號（可線上驗證的 3701/3702/3703 僅接受英數字、連字號與底線，最長 64 字元）"
// @Param        type     query   string  true  "認證類型 CertificationType value（3701=ORGANIC / 3702=TAP / 3703=CAS / 3704=HACCP / 3705=ISO / 3706=GMP）" Enums(3701, 3702, 3703, 3704, 3705, 3706)
// @Success      200  {object}  object{code=int,message=string,data=VerifyCertificationResponse}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      501  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/products/certifications/verify [get]
func (h *Holder) VerifyCertification(c *gin.Context) {
	certNo := c.Query("cert_no")
	certType := c.Query("type")

	if certNo == "" {
		response.BadRequest(c, "認證編號不可為空")
		return
	}
	if certType == "" {
		response.BadRequest(c, "認證類型不可為空")
		return
	}
	if _, verifiable := verifiableCertTypes[certType]; verifiable && !certNoPattern.MatchString(certNo) {
		response.BadRequest(c, "認證編號格式不正確，僅接受英數字、連字號與底線")
		return
	}

	resp, err := h.Product.VerifyCertification(c.Request.Context(), certNo, certType)
	if err != nil {
		pkglogger.Error(c.Request.Context(), "VerifyCertification gRPC error", zap.Error(err))
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, &VerifyCertificationResponse{
		Exists: resp.Exists,
	})
}
