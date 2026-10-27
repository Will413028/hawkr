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

// buildMediaURLs 將一組 objectKey 逐一組為顯示 URL（49 §6.1/§6.6：Gateway 唯一組裝點）。
// keys 為空回傳空 slice（omitempty 生效）；單一 http 值由 mediaurl.BuildURL passthrough。
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

// productSortAllowed 合法 ProductSort Value 集合（35 設計書）。
// 與 common-core ProductSort enum 對應；後端做白名單驗證，避免靜默 fallback。
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

// nilSafeSlice 把 nil slice 換成 empty slice，避免 JSON 輸出 null（前端契約：deliveryTemp 必為陣列）。
func nilSafeSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// parseCSVQuery 通用：把 query string 「a,b , c」拆為 ["a","b","c"]，去重 + 修整 whitespace；空字串回 nil
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

// parseCategoryIDsQuery 解析 ?category_id=... 支援 CSV 與重複 param 兩種形式：
//
//	?category_id=1,5         → ["1","5"]
//	?category_id=1&category_id=5 → ["1","5"]
//	?category_id=1,0,5       → ["1","5"]（過濾 "0"=全部）
//
// 空字串 / 純 "0" / 全為空白 → nil（不篩選）；自動 trim + 去重。
// 上限校驗（50 個）在 product-service 入口執行，回 INVALID_ARGUMENT。
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

// parseHashtagsQuery 把 query string 「a,b , c」拆為 ["a","b","c"]，去重 + 修整 whitespace；空字串回 nil
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

// getProductAccountID 從 context 獲取 accountId（用於產品存取權限判斷）
// 如果用戶未登入則返回 0
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

// toShippingMethodResponses 將 proto ShippingMethod 列表轉為 ShippingMethodResponse 陣列
// 供 list / detail 共用，避免重複構造。
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

	// sort 合法值驗證（35 設計書）；不送（空字串）等同 LATEST
	if sort != "" && !isValidProductSort(sort) {
		response.BadRequest(c, "invalid sort value")
		return
	}

	// Hashtag 精確篩選（02 §3.9 / §5.3）
	hashtags := parseHashtagsQuery(c.DefaultQuery("hashtags", ""))
	matchAll := strings.EqualFold(c.DefaultQuery("match", "any"), "all")

	// 02 §5.3.4 新增 server-side filter
	deliveryTemp := parseCSVQuery(c.DefaultQuery("delivery_temp", ""))
	priceMin := strings.TrimSpace(c.DefaultQuery("price_min", ""))
	priceMax := strings.TrimSpace(c.DefaultQuery("price_max", ""))
	storeIDs := parseCSVQuery(c.DefaultQuery("store_ids", ""))
	sellerCityCode := strings.TrimSpace(c.DefaultQuery("seller_city_code", ""))
	certificationTypes := parseCSVQuery(c.DefaultQuery("certification_types", ""))
	followedProductOnly := strings.EqualFold(c.DefaultQuery("followed_product_only", ""), "true")
	followedSellerOnly := strings.EqualFold(c.DefaultQuery("followed_seller_only", ""), "true")

	accountID := getProductAccountID(c)

	// followed_* filter 必須有目前有效的 buyer stint；resolveBuyerID 內部同時
	// 涵蓋「未登入」（401）與「登入但無 buyer stint」（403）兩種情況，比舊版
	// 只檢查 accountID != 0 更嚴格——沒有 buyer profile 的帳號本來就不可能有
	// 收藏/追蹤。未啟用任一 filter 時不呼叫 resolveBuyerID，避免多一次 Redis
	// 往返（一般公開瀏覽維持原本行為，ViewerBuyerID 為 0）。
	var viewerBuyerID int64
	if followedProductOnly || followedSellerOnly {
		buyerID, ok := h.resolveBuyerID(c)
		if !ok {
			return
		}
		viewerBuyerID = int64(buyerID)
	}

	// 公開 API：由 Product Service 強制只返回 ACTIVE 狀態；
	// viewerBuyerID 由 gateway 解析後傳入，只作為 ViewerBuyerID（不會變成 owner filter）
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

		// 商家資訊（含 city / district / address）
		if p.Seller != nil {
			item.Seller = buildSellerInfoItem(base, p.Seller)
			item.SellerName, item.SellerLogoKey, item.SellerLogoUrl = flatSellerFields(item.Seller)
		}

		products = append(products, item)
	}

	// Aggregate isFavorited + isRestockSubscribed: only if user is logged in and there are products
	if accountID != 0 && len(products) > 0 {
		productIDStrs := make([]string, 0, len(products))
		for _, p := range products {
			productIDStrs = append(productIDStrs, p.ID)
		}

		if len(productIDStrs) > 0 {
			// 收藏、補貨訂閱都是 buyer_id 資源，共用同一次 soft-fail 解析（不能用
			// h.resolveBuyerID(c)，它會直接寫 401/403 回 c，PDP 聚合失敗不該阻斷主回應）。
			// ErrNoCurrentBuyer 併入下面既有的「err != nil → Debug + 整塊略過」skip
			// path，不另開分支；下方 applyRestockSubscribed 重複使用同一個 buyerID，
			// 解析失敗時 buyerID 是零值，helper 自己的 buyerID<=0 guard 會 no-op，
			// 不需要再判斷一次 berr。
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

			// isRestockSubscribed aggregation (issue #1005 / B-203, 39 §5.5b)
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

// buildSellerInfoItem 是 pb.SellerInfo → SellerInfoItem 的**唯一**映射點。
//
// 每個吐 `seller` 物件的端點（商品列表 / 商品詳情 / 市集 / 搜尋 / 賣家店內列表）
// 都必須走這裡：日後 SellerInfoItem 新增欄位只要改這一個函式，不必再去獵五份
// 手抄副本。這正是本函式存在的理由——過去五處各自手抄，欄位覆蓋長期不一致：
// 賣家店內列表與市集漏了 city/district/address（前端 readDisplayOrigin 會讀
// seller.city 當產地 fallback，缺了商品卡產地就空白），store_code 也是事後
// 逐處補釘（commit a2419f271「2 missed SellerInfoItem sites」即同一類事故）。
//
// s 為 nil 回傳 nil，呼叫端直接指派即可（`seller` 欄位帶 omitempty，nil 時省略）。
// proto 空字串一律收斂成 nil 指標而非空字串指標，讓 omitempty 真的把欄位拿掉，
// 而不是吐出 `"city": ""` 這種前端得再判一次的雜訊。
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

// flatSellerFields 取出商品卡 DTO（ProductItem / ProductHitData）在 nested
// `seller` 之外另外攤平的三個便利欄（sellerName / sellerLogoKey / sellerLogoUrl）。
//
// 為什麼不併進 buildSellerInfoItem：這三欄不屬於 SellerInfoItem，而是掛在**容器**
// 上，而容器有兩種不同型別（ProductItem 與 ProductHitData），無法由一個回傳
// *SellerInfoItem 的函式代為填寫。拆成這支 helper 至少讓「攤平值怎麼來的」只有
// 一份定義——攤平欄恆等於 nested 值，不容許兩者說法不一致。
//
// 注意 PublicProductDetailItem（商品詳情）沒有這三欄，故詳情端點只呼叫
// buildSellerInfoItem 不呼叫本函式；那是容器型別本身的差異，不是漏抄。
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
	productID := c.Param("id") // UUID string

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
	// 公開 PDP 一律使用 publication contract；只有賣家身分保留既有的 owner preview，
	// 避免登入買家因走私有 GetProduct 而把 TEMPORARILY_UNAVAILABLE 誤判成 404。
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

	// Aggregate isFavorited + isRestockSubscribed: only if user is logged in.
	// 兩個 best-effort 聚合，失敗不阻斷 PDP 回應（02 §5.13b / §5.5.1）。
	// syncutil.Group 平行兩個 gRPC（不 WithContext — 任一失敗不應 cancel 對方；
	// panic 會被 recover 並記錄，不會讓整個 gateway process crash）。
	if buyerAccountID != 0 && detail.ID != "" {
		if _, err := strconv.ParseInt(detail.ID, 10, 64); err == nil {
			ctx := c.Request.Context()
			var (
				favorited     bool
				subscribedMap map[string]bool
			)
			// 收藏、補貨訂閱都是 buyer_id 資源，共用同一次 soft-fail 解析（同
			// GetProductList 的理由：失敗不阻斷 PDP）。解析失敗（含 ErrNoCurrentBuyer）
			// 兩個 goroutine 一起略過，不各自再解析一次。
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
	productID := c.Param("id") // UUID string

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
	// 刻意不組 Seller（也不設 StoreID）：本端點回的是「商品基本資訊」，
	// 沒有任何客戶端讀賣家欄位。2026-07-31 實測四個前端 repo（website / admin /
	// buyer app / seller app），全站唯一呼叫者是 buyer app 的
	// features/product/data/product_repository.dart:206 getSkuImageUrl()，它只讀
	// url / imageUrl / data 三個鍵，而且該方法本身無人呼叫（dead code）。
	// 上游其實帶得出來（GetPublicProduct 走 ProductDetail，seller 已由
	// product-service 填充），所以要補只是兩行 buildSellerInfoItem——
	// **觸發條件**：哪天有客戶端要在這支端點顯示店名 / 店家 logo / 產地，
	// 就照 §SellerInfoItem 覆蓋矩陣補上並把本端點納入 seller_info_matrix_test.go。
	// 在那之前補 = 白白多打一次 seller batch RPC 並放大回應體，不做。
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

	// isRestockSubscribed aggregation (issue #1005 / B-203, 39 §5.5b)
	// buyer_id-first：這裡沒有其他地方已解析過 buyerID，比照 GetProductList 的
	// favorites soft-fail 解析自己做一次；ErrNoCurrentBuyer 等同「無 stint」，
	// 略過聚合、不阻斷本端點。
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
	productID := c.Param("id") // UUID string

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
	productID := c.Param("id") // UUID string

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
	// 刻意不組 Seller（也不設 StoreID）：2026-07-31 實測四個前端 repo
	// （website / admin / buyer app / seller app）**無任何客戶端呼叫本端點**——
	// 官網的「相關商品」是自己用 /public/search/products 併「同分類補齊」實作的
	// （app/api/public/search/products/route.ts），不走這裡。
	// 上游帶得出來：product-service getRelatedProducts 用 publicApi(true) 查詢，
	// ProductService.java:1085 的公開分支會 batch 填 seller meta，
	// 故 ProductListItem.seller 有值，是 Gateway 這裡沒往下傳。
	// **觸發條件**：哪天有客戶端真的改用本端點渲染商品卡，就得補 Seller——
	// 前端商品卡的產地是 `origin || seller.city`（website
	// lib/market/listFilters.ts readDisplayOrigin），缺 city 產地會空白。
	// 屆時用 buildSellerInfoItem + flatSellerFields 兩行補上，並把本端點納入
	// seller_info_matrix_test.go 的覆蓋矩陣。在那之前補 = 為零消費者放大回應體。
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

	// isRestockSubscribed aggregation (issue #1005 / B-203, 39 §5.5b)
	// buyer_id-first：沒有其他地方已解析過 buyerID，自己做一次 soft-fail 解析
	// （同 GetProductInfoByID／GetProductList 的理由）。
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

// convertDescriptionProtoToResponse 將 proto ProductDescription 轉換為 ProductDescriptionResponse
// Proto 的 style_json 字串反序列化為 style 物件給前端
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
			ImageURL:  mediaurl.BuildURL(base, b.ObjectKey), // 49 §6.6：Gateway 由 objectKey 組顯示 URL
			Width:     b.Width,
			Height:    b.Height,
			Alt:       b.Alt,
		}

		// 行內 runs（inline-run 模型）
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

		// 將 style_json 字串反序列化為 style 物件
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

// toPublicDescription 把賣家形狀的描述轉成公開形狀（逐欄搬過去，就是不搬 assetId）。
//
// 從 ProductDescriptionResponse 轉而不是從 proto 直接轉，是為了讓 runs 展開與 style_json
// 反序列化只留 convertDescriptionProtoToResponse 一份——複製一份解析邏輯出來，兩邊遲早分岔。
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

// convertToProductDetail converts proto ProductDetail to handler ProductDetailItem
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
		// 轉換 SKU 層級的運送方式
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

	// 配送資訊（聚合自所有 SKU）
	shippingMethods := make([]ShippingMethodResponse, 0, len(data.ShippingMethods))
	for _, sm := range data.ShippingMethods {
		shippingMethods = append(shippingMethods, ShippingMethodResponse{
			Type:          sm.Type,
			Temperature:   sm.Temperature,
			FreeThreshold: sm.FreeThreshold,
			FeePayer:      sm.FeePayer,
		})
	}

	// 結構化圖片集（含 assetId），供賣家編輯只調順序時 round-trip 保留既有圖，
	// 與 list 端（SellerProductItem.Gallery）對稱。公開端不吐 assetId，作法是另立
	// PublicGalleryItem 型別（非執行期清欄位），見 convertToPublicProductDetail 與 02 §2.2.2。
	gallery := make([]GalleryItem, 0, len(data.Gallery))
	for _, g := range data.Gallery {
		gallery = append(gallery, GalleryItem{
			AssetID:  g.AssetId,
			ImageKey: g.ObjectKey,
			ImageURL: mediaurl.BuildURL(base, g.ObjectKey), // 49 §6.6：Gateway 由 objectKey 組
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

// convertToPublicProductDetail converts proto ProductDetail to PublicProductDetailItem（含 seller + reviews）
func convertToPublicProductDetail(base string, data *pb.ProductDetail) PublicProductDetailItem {
	detail := convertToProductDetail(base, data)

	// 公開端（匿名 PDP）不吐的只有 assetId（媒體資產 TSID＝內部識別碼），作法是換一組
	// 公開型別而非清欄位——型別上沒有那個欄位，swagger 才不會宣告出永不出現的東西。
	// objectKey 保留：
	// 它是 public/ 前綴的公開物件路徑，且同一份回應的 imageKeys 已列出全部 gallery objectKey，
	// 清空它保護不到任何東西，卻讓前端只能載 /i/original/（描述內嵌圖的 objectKey 更不在
	// imageKeys 裡，清掉即永久無法壓縮）。見 02 設計書 §2.2.2。
	publicGallery := make([]PublicGalleryItem, 0, len(detail.Gallery))
	for _, g := range detail.Gallery {
		publicGallery = append(publicGallery, PublicGalleryItem{ImageKey: g.ImageKey, ImageURL: g.ImageURL})
	}
	// 商品描述區塊（富文本內嵌圖）走同一條界線：型別上就沒有 assetId，objectKey / url 保留供
	// 顯示與 preset 壓縮（單層 Blocks，無巢狀）。
	publicDescription := toPublicDescription(detail.Description)
	// 內嵌的 ProductDetailItem.Gallery / .Description 被外層同名欄位遮蔽、不會序列化；
	// 一併清空避免有人誤用它們。
	detail.Gallery = nil
	detail.Description = nil
	// SKU 的樂觀鎖版本（#2545）只給寫得到這筆 SKU 的人（賣家／代行／admin）。
	// 匿名 PDP 用不到它，而它是一個**寫入次數計數器**——扣庫存與訂單取消都會 +1，
	// 公開等於把每個規格的成交／取消事件次數（比 salesCount 更細）攤在匿名端。
	// 這裡用清欄位而非另立公開型別（上面 assetId 的作法），因為 SkuItem 是 public／app／
	// admin 三個 namespace 共用的葉節點：換型別要連帶動到三條路徑的回應形狀，
	// 而本欄有 omitempty ⇒ 清掉之後公開回應裡根本不會出現這個鍵。
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

	// 商家資訊（含 city / district / address，list 與 detail 對稱）
	// PublicProductDetailItem 沒有攤平便利欄（見 flatSellerFields 註解），故只組 nested。
	result.Seller = buildSellerInfoItem(base, data.Seller)

	// 評論預覽
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
				// 該帳號目前沒有現任店 → 沒有對應的 list bucket 可清，
				// 呼應 SearchAdminProducts 對同一情境回空結果的處理，視為清除成功（0 筆）。
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

// ---- 認證驗證 DTO 區塊 ----

// VerifyCertificationResponse 認證驗證回應
type VerifyCertificationResponse struct {
	Exists bool `json:"exists" example:"true"` // 認證編號是否存在
}

// certNoMaxLength 認證編號長度上限。
//
// certifications 存成 JSONB（product-service V1__init_product_schema.sql），沒有可比照的
// 欄位長度；repo 內最長的實際樣本是 18 碼（seed data 的 HACCP-2026-DP-0047），可線上驗證的
// 三型最長 16 碼，取 64 留餘裕，同時讓送往農業部 API 的查詢字串長度有界。
const certNoMaxLength = 64

// certNoPattern 認證編號合法字元：英數 + 連字號 + 底線。
//
// 農業部三支端點沒有公開的編號格式規範，docs/開發規範/ 也沒有記載，因此取保守通用集合；
// 實際樣本（設計書 19 §3/§7.1 的 1-009-300022、seed data 的 1-008-131005 / TAP-2025-0032 /
// CAS-2025-1234）都落在這個集合內。這一組字元全屬 RFC 3986 unreserved character。
//
// 擋在這裡的理由：product-service 的 GovCertificationService.buildGovApiUrl 把 certNo
// 原字未編碼串進農業部 API 的查詢字串，後面緊接著 &api_key=<真實金鑰>。放行 URI 保留字元
// 等於讓任何呼叫端反覆把整條含金鑰的 URL 逼進上游的例外訊息。
//
// ⚠️ 規則與 Java 側 GovCertificationService.isValidCertNo 必須逐字一致，改動時兩邊一起改。
//
// ⚠️ 適用範圍限「可線上驗證的三型」（verifiableCertTypes），advisory 查驗端點與商品
// 寫入路徑的 filter 都是如此。3704/3705/3706 走 unverifiableCertNoValid 的放寬規則——
// 拿這個 pattern 去擋它們會把 `ISO 22000:2018` 這種真實編號靜默剔除（19 §6.2.9）。
var certNoPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,` + strconv.Itoa(certNoMaxLength) + `}$`)

// verifiableCertTypes 是可線上驗證的 CertificationType value（3701 ORGANIC / 3702 TAP /
// 3703 CAS），與 product-service 的 CertificationTypeRules.isVerifiable 同一組。
//
// 只有這三型的 cert_no 會被 product-service 串進農業部 API 的查詢字串，所以也只有這三型
// 需要套 certNoPattern。3704/3705/3706（HACCP/ISO/GMP）在 product-service 端會先短路成
// 「暫不支援線上驗證」（3003），cert_no 根本不會離開系統；對它們套白名單零安全效益，卻會
// 把 `ISO 22000:2018` 這種該類型的常見合法編號誤判成 400。
var verifiableCertTypes = map[string]struct{}{
	"3701": {},
	"3702": {},
	"3703": {},
}

// ---- 認證輸入 filter（19 號設計書 §6.2 / §6.2.1）----

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

// 認證未被採用的原因（19 §9.5）。Gateway 只產出前兩個；CONCURRENT_CONFLICT 由
// product-service 在收斂點判定後經 certification_warnings 帶回來。
const (
	certWarningInvalidType   = "INVALID_TYPE"
	certWarningInvalidCertNo = "INVALID_CERT_NO"
)

// certificationInputTypes 是 certifications 輸入的合法 type 值域（CertificationType
// value 3701–3706）。值取自 pkg/codemeta 的生成常數而不是重打一次字面值——CodeMeta 是
// SSOT，在這裡寫死 "3701" 就是憑空多出來的第二份。
//
// ⚠️ 與 verifiableCertTypes 是兩件事：那個回答「這一型能不能線上查驗」（只有前三型），
// 這個回答「這一型存不存在」。拿 verifiableCertTypes 當值域會把合法的 HACCP / ISO / GMP
// 認證整批剔除。
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

// certificationInputRejection 回傳這一筆認證輸入被剔除的原因；合法則回兩個空字串。
//
// 🔴 certNo 的規則**依 type 分兩類**（19 §6.2.9）。一套規則走天下會靜默吃掉真實編號：
// `ISO 22000:2018` 含冒號、HACCP／GMP 的編號常含空格與斜線，而嚴格白名單只收
// `[A-Za-z0-9_-]` ⇒ 賣家填的是對的編號，卻被剔除、送出成功、什麼都沒發生、零訊號。
// 那正是這份設計要防的失效形狀本身。
//
//	3701 / 3702 / 3703（可線上驗證）
//	  嚴格 certNoPattern。這個值會被 product-service 原字未編碼組進農業部 API 的查詢
//	  字串（後面緊接著 &api_key=<真實金鑰>），格式由對方決定不是由我們決定；而且它與
//	  Java 的 GovCertificationService.isValidCertNo 是**同一條規則的拷貝**，不得分岔。
//
//	3704 / 3705 / 3706（不支援線上驗證）
//	  放寬：trim 後非空、≤ certNoMaxLength 字、禁控制字元與角括號，其餘字元允許。
//	  它們不會被組進任何外部 URL，格式由 ADM 人眼判斷。這與 advisory 查驗端點
//	  （VerifyCertification）對這三型本來就不套白名單的既有行為一致——同一個理由。
//
// ⚠️ 放寬**不等於不驗**：控制字元與 `<` `>` 仍然擋，那是防注入與防髒資料進 ADM 佇列，
// 與「格式由誰決定」無關。
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

// unverifiableCertNoValid 是 3704／3705／3706 的放寬規則（19 §6.2.9）。
//
// 判在 trim 之後：尾端換行是 CSV／試算表貼上來的常態，把它判成非法等於用一個排版字元
// 靜默吃掉一張真的證書。⚠️ 判什麼就得送什麼——normalizedCertNo 負責讓送下游的值與這裡
// 驗過的值是同一個。
//
// 長度算的是**字元不是位元組**：這三型的編號可能含中文（例：「食品GMP認證字第○○號」），
// 用 len() 會讓 22 個中文字就超過 64 而被剔除。可驗證三型不受影響——它們的字元集全是
// ASCII，兩種算法等值。
func unverifiableCertNoValid(certNo string) bool {
	trimmed := strings.TrimSpace(certNo)
	if trimmed == "" {
		return false
	}
	if utf8.RuneCountInString(trimmed) > certNoMaxLength {
		return false
	}
	for _, r := range trimmed {
		// 角括號單獨列出而不是靠某個字元集白名單：白名單正是這條放寬規則要拿掉的東西，
		// 而這兩個字元擋的是「值被當成標記語言渲染」，與格式由誰決定無關。
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return false
		}
	}
	return true
}

// normalizedCertNo 回傳真正要送下游的 certNo。
//
// 存在的理由只有一個：**驗過的值與送出的值必須是同一個**。放寬那三型驗的是 trim 後的
// 結果，若送出的仍是原值，一個從試算表貼進來的尾端換行就會通過驗證然後原樣落進申請表
// 與 ADM 佇列——驗證看起來有做，擋下來的東西卻進去了。
//
// 可驗證三型原樣回：certNoPattern 本來就不容許任何空白，走到這裡的值已經是乾淨的，
// 在這裡多做一次 trim 只會讓「Gateway 與 Java 是同一條規則」這句話變得不精確。
func normalizedCertNo(certType, certNo string) string {
	if _, verifiable := verifiableCertTypes[certType]; verifiable {
		return certNo
	}
	return strings.TrimSpace(certNo)
}

// certificationInputRef 是「一筆認證輸入」在判定時唯一需要的兩個值。
//
// 抽這個中介型別是因為兩條寫入路徑的 DTO 不同（賣家 pb.Certification、代行
// DelegatedProductCertification，19 §6.2 的表），而判定規則必須只有一份。
type certificationInputRef struct {
	Type string
	No   string
}

// certificationInputDecision 對一組認證輸入逐筆判定，回傳「第 i 筆留不留」的遮罩與
// 對應 warning，並為每一筆被剔除的輸入記一行 warn log（19 §6.2.1 第 ① 層）。
//
// 🔴 為什麼是 handler 內的 filter 而不是 binding tag：本 repo 的 `binding:` 只有一條
// 出口——ShouldBindJSON 失敗 → response.BadRequest ⇒ 400，而 `dive` 讓元素 tag 生效的
// 意思正是「元素違規時 bind 失敗」。AI 上架助手 patch 進來的 certNo 會繞過畫面驗證原樣
// 送出，後端現況照收 200；改成 400 會讓**已上架 App** 的 AI 上架流程當場壞掉（19
// §6.2.2）。所以非法輸入一律「剔除該筆 ＋ warn log」，回應維持 200。
// ✅ 筆數上限不受此限：binding:"max=N" 擋的是 DoS 不是值域，超量回 400 是正確行為。
//
// 呼叫時機固定是 **bind 之後、組 gRPC request 之前**：被剔掉的那一筆不得離開 Gateway。
// log 欄位刻意不含 certNo（19 §6.2.1 只要求 productId／storeId／type／原因）。
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

// filterCertificationInputs 是賣家路徑（W1／W2）的 filter：剔除非法輸入後回傳可以送進
// gRPC 的那幾筆，以及要回給前端的 warning。
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
		// 送出 normalizedCertNo 而不是原值：驗過的值與送出的值必須是同一個
		// （見 normalizedCertNo 的註解）。可驗證三型這裡是 no-op。
		kept = append(kept, &pb.Certification{
			Name:   cert.GetName(),
			CertNo: normalizedCertNo(cert.GetName(), cert.GetCertNo()),
		})
	}
	return kept, warnings
}

// mergeCertificationWarnings 併入 product-service 回傳的 warning（CONCURRENT_CONFLICT）。
// Gateway 側剔除的排前面：那是操作者自己送錯的，先看到比較合理。
//
// 回傳恆非 nil——json 序列化成 [] 而不是 null，19 §9.5 的硬性要求。
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
	// 字元白名單擋在送進 gRPC 之前：非法字元不得繼續往下游送。
	// 只套用在可線上驗證的三型——其餘類型的 cert_no 不會被串進政府 API URL，擋了不會多安全，
	// 只會把該類型的合法編號從 3003（暫不支援線上驗證）改判成 400。
	if _, verifiable := verifiableCertTypes[certType]; verifiable && !certNoPattern.MatchString(certNo) {
		response.BadRequest(c, "認證編號格式不正確，僅接受英數字、連字號與底線")
		return
	}

	resp, err := h.Product.VerifyCertification(c.Request.Context(), certNo, certType)
	if err != nil {
		pkglogger.Error(c.Request.Context(), "VerifyCertification gRPC error", zap.Error(err))
		// 透傳 gRPC status description（含農業部 API 失敗的具體訊息）
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
