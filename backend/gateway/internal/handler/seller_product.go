package handler

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GalleryItem represents an image in the gallery（賣家 / Admin 管理用，含 assetId + objectKey）。
// assetId 供賣家編輯只調順序時 round-trip 保留既有圖。
// 公開端（匿名 PDP / 公開搜尋）用的是另一個型別 PublicGalleryItem（無 assetId，見 02 設計書 §2.2.2），
// 不再靠「共用本型別 + 執行期清空欄位」達成——那會讓 swagger 宣告與實際輸出對不上。
// GalleryItem 是結構化圖片集的單筆（賣家 / Admin 端，含 assetId 供編輯 round-trip）。
// 圖片欄用 imageKey/imageUrl 而非 objectKey/url：任何「帶一張圖」的物件對外都是這一組
// 名字（商品卡、購物車項、貼文商品、gallery 條目），前端才寫得出單一取圖函式。
type GalleryItem struct {
	AssetID  string `json:"assetId,omitempty"`
	ImageKey string `json:"imageKey,omitempty"`
	ImageURL string `json:"imageUrl"` // 完整 URL（Gateway 由 imageKey 組；49 §6.1）
}

// SellerProductItem represents a seller's product in the list
// 對應新 proto 的 ProductListItem（UUID ID、CodeMeta Status）
type SellerProductItem struct {
	ID           string        `json:"id"` // UUID
	Name         string        `json:"name"`
	Rating       string        `json:"rating,omitempty"`
	Status       string        `json:"status"` // CodeMeta: 4001-4004
	CategoryID   string        `json:"categoryId,omitempty"`
	CategoryName string        `json:"categoryName,omitempty"`
	Origin       string        `json:"origin,omitempty"`
	Gallery      []GalleryItem `json:"gallery,omitempty"`
	PriceMin     string        `json:"priceMin,omitempty"`     // 最低實際售價（新台幣元，數字字串。彙總自 SKU effective_price = salePrice ?? price）
	PriceMax     string        `json:"priceMax,omitempty"`     // 最高實際售價（新台幣元）
	ListPriceMin string        `json:"listPriceMin,omitempty"` // 最低原價（新台幣元，給前端 strike-through 顯示）
	ListPriceMax string        `json:"listPriceMax,omitempty"` // 最高原價（新台幣元）
	SalePriceMin string        `json:"salePriceMin,omitempty"` // 最低特價（新台幣元，皆無特價時為 "0"，不是空字串）
	SalePriceMax string        `json:"salePriceMax,omitempty"` // 最高特價（新台幣元）
	CreatedAt    string        `json:"createdAt,omitempty"`
	OwnerID      string        `json:"ownerId,omitempty"`
	Hashtags     []HashtagItem `json:"hashtags,omitempty"` // Hashtag 列表（02 §3.7-§3.9）
	// 反正規化欄位（02 §3.2 / §5.13a / §5.14；本期新增，issue B-192 / B-193）
	Stock           string                   `json:"stock"`                     // 即時庫存（總和），數字字串
	SalesCount      string                   `json:"salesCount"`                // 累計銷量（付款成功時 +N），數字字串
	FavoriteCount   string                   `json:"favoriteCount"`             // 收藏人次，數字字串
	DeliveryTemp    []string                 `json:"deliveryTemp,omitempty"`    // 配送溫層聯集（ShippingTemperature value）
	ShippingMethods []ShippingMethodResponse `json:"shippingMethods,omitempty"` // 運送方式聯集（含免運門檻）
	// 圖片成對欄位（49 §6.1）：key=SSOT、url=Gateway 組
	ImageKey  string   `json:"imageKey,omitempty"`  // 第一張圖 objectKey
	ImageURL  string   `json:"imageUrl,omitempty"`  // 第一張圖顯示 URL
	ImageKeys []string `json:"imageKeys,omitempty"` // 所有圖片 objectKey
	ImageUrls []string `json:"imageUrls,omitempty"` // 所有圖片顯示 URL
}

// HashtagItem 是 hashtag 條目（gateway 層共用結構，對應 proto HashtagItem）
// productCount 為字串型別（33-商品 Hashtag 模組詳細設計書 §5.4；對外數字一律 string，
// 見 docs/開發規範/Java 微服務開發規範.md §2）
type HashtagItem struct {
	Name         string `json:"name"`                  // canonical 名稱（lower）
	DisplayName  string `json:"displayName,omitempty"` // 顯示名稱（保留原大小寫）；auto-complete 場景為空字串
	ProductCount string `json:"productCount"`          // 引用數量（hashtag.product_count 即時值，字串型別）
}

// toHashtagItems 將 proto 的 hashtag list 轉為 gateway 的 HashtagItem slice。
func toHashtagItems(protoItems []*pb.HashtagItem) []HashtagItem {
	if len(protoItems) == 0 {
		return nil
	}
	out := make([]HashtagItem, 0, len(protoItems))
	for _, h := range protoItems {
		if h == nil {
			continue
		}
		out = append(out, HashtagItem{
			Name:         h.Name,
			DisplayName:  h.DisplayName,
			ProductCount: h.ProductCount,
		})
	}
	return out
}

// toCertificationItems converts proto Certification list to gateway DTO（商品卡 / 詳情共用）。
func toCertificationItems(protoItems []*pb.Certification) []CertificationItem {
	if len(protoItems) == 0 {
		return nil
	}
	out := make([]CertificationItem, 0, len(protoItems))
	for _, c := range protoItems {
		if c == nil {
			continue
		}
		out = append(out, CertificationItem{
			Name:   c.Name,
			CertNo: c.CertNo,
		})
	}
	return out
}

// SellerProductListData contains products and pagination for merchant
type SellerProductListData struct {
	Total    string              `json:"total"`
	Page     string              `json:"page"`
	PageSize string              `json:"pageSize"`
	Products []SellerProductItem `json:"records"`
}

// CreateProductImageInput represents image input for creating product
//
// 只收 stagedToken（新圖）或 assetId（既有圖）——product-service 組 gallery 時就只認這兩者
// （ProductService.processImages / processImageUpdate）。曾有一個 objectKey 欄位，從 Gateway
// 一路傳到 Java DTO 卻全程無人讀取，已於 2026-08-05 連同 proto 欄位一起移除。
type CreateProductImageInput struct {
	ID          string `json:"id,omitempty"`          // 前端排序用
	StagedToken string `json:"stagedToken,omitempty"` // 新增圖片用
	AssetID     string `json:"assetId,omitempty"`     // 既有圖片用
}

// CreateSkuInput represents SKU input for creating product.
//
// 必填集合以 product-service 的實際行為為準（dev 實測）：skuName / unit / price / stockQty /
// weight / lengthCm / widthCm / heightCm 缺一即 400。這些 binding tag 只有在父欄位帶了
// `dive` 時才會生效——CreateProductRequest.Skus 就是為此加上 dive 的。
type CreateSkuInput struct {
	SkuName         string                      `json:"skuName" binding:"required"`
	Weight          string                      `json:"weight" binding:"required"`
	Unit            string                      `json:"unit" binding:"required"`
	Price           string                      `json:"price" binding:"required"` // 原價（list price，新台幣元，數字字串）
	SalePrice       string                      `json:"salePrice,omitempty"`      // 特價（新台幣元，空字串=無特價，須 < price 且 > 0）
	StockQty        string                      `json:"stockQty" binding:"required"`
	LengthCm        string                      `json:"lengthCm" binding:"required"`
	WidthCm         string                      `json:"widthCm" binding:"required"`
	HeightCm        string                      `json:"heightCm" binding:"required"`
	ShippingMethods []CreateShippingMethodInput `json:"shippingMethods,omitempty"` // 運送方式（SKU 層級）
}

// UpdateSkuInput represents SKU input for updating a product.
//
// 與建立時的必填集合**不同**，所以不共用 CreateSkuInput（§2：每個 endpoint 唯一 DTO）：
// 更新走 PATCH 語意，skuName / unit / price / stockQty 省略 = 不變更（dev 實測：只帶
// id + 材積可成功）；但 weight 與三個材積只要該 SKU 出現在陣列裡就必填（省略會被
// product-service 讀成 0 並回 400「商品重量 必須大於 0」）。
//
// 整個 skus 欄位省略 = 完全不動 SKU（不是清空）。
type UpdateSkuInput struct {
	ID              string                      `json:"id,omitempty"` // 既有 SKU 必帶；空值視為新增
	SkuName         string                      `json:"skuName,omitempty"`
	Weight          string                      `json:"weight" binding:"required"`
	Unit            string                      `json:"unit,omitempty"`
	Price           string                      `json:"price,omitempty"`
	SalePrice       string                      `json:"salePrice,omitempty"`
	StockQty        string                      `json:"stockQty,omitempty"`
	LengthCm        string                      `json:"lengthCm" binding:"required"`
	WidthCm         string                      `json:"widthCm" binding:"required"`
	HeightCm        string                      `json:"heightCm" binding:"required"`
	ShippingMethods []CreateShippingMethodInput `json:"shippingMethods,omitempty"`
	// ExpectedVersion 是這筆 SKU 的樂觀鎖 token（#2545），取自讀取時的 skus[].version。
	//
	// 省略 = 不比對（維持舊 client 的既有行為，不是回歸）。帶了才擋得住這件事：
	// 表單開著的期間有人下單扣了庫存，送出時 client 帶的是開表單那一刻的 stockQty，
	// 後端原樣寫回 ⇒ 賣掉的庫存被復活，HTTP 200、無任何訊號。
	// product 層的 expectedVersion 對它失明——扣庫存刻意不 bump product.version
	// （56 §5.4 F41 第 11 列），所以這把鎖必須掛在 SKU 自己身上。
	ExpectedVersion string `json:"expectedVersion,omitempty"`
}

// CreateShippingMethodInput 運送方式輸入結構
type CreateShippingMethodInput struct {
	Type          string `json:"type"`
	Temperature   string `json:"temperature"`
	FreeThreshold string `json:"freeThreshold"`
	FeePayer      string `json:"feePayer"`
}

// CreateCertificationInput represents certification input
type CreateCertificationInput struct {
	Name   string `json:"name"`
	CertNo string `json:"certNo"`
}

// ProductDescriptionInput 商品描述輸入結構（圖文混排）
type ProductDescriptionInput struct {
	Version string                  `json:"version,omitempty"`
	Blocks  []DescriptionBlockInput `json:"blocks,omitempty"`
}

// DescriptionBlockInput 描述區塊輸入結構
type DescriptionBlockInput struct {
	Type string `json:"type"`         // 區塊類型：text / table / list / image
	ID   string `json:"id,omitempty"` // 區塊識別碼
	// 文字類型欄位
	Runs    []InlineRunDTO         `json:"runs,omitempty"`    // 行內 runs（inline-run 模型）
	Content string                 `json:"content,omitempty"` // legacy 純文字
	Style   map[string]interface{} `json:"style,omitempty"`   // 樣式物件（Gateway body 使用 style，轉 proto 時序列化為 style_json）
	// 表格類型欄位
	TableJson string `json:"tableJson,omitempty"` // 表格資料 JSON 字串
	// 清單類型欄位
	ListJson string `json:"listJson,omitempty"` // 清單資料 JSON 字串
	// 圖片類型欄位
	StagedToken string `json:"stagedToken,omitempty"` // 新增圖片用
	AssetID     string `json:"assetId,omitempty"`     // 既有圖片用
	ImageKey    string `json:"imageKey,omitempty"`    // 物件儲存路徑（49 §6.1）
	Width       string `json:"width,omitempty"`       // 圖片寬度（數字字串）
	Height      string `json:"height,omitempty"`      // 圖片高度（數字字串）
	Alt         string `json:"alt,omitempty"`         // 替代文字
}

// CreateProductRequest is the request body for creating a product
// 對應新 proto 的 CreateProductRequest
type CreateProductRequest struct {
	Description    *ProductDescriptionInput  `json:"description,omitempty"`
	IdempotencyKey string                    `json:"idempotencyKey"`
	Name           string                    `json:"name" binding:"required"`
	CategoryID     string                    `json:"categoryId" binding:"required"`
	Origin         string                    `json:"origin,omitempty"`
	Images         []CreateProductImageInput `json:"images,omitempty"`
	// `dive` 不可省略：沒有它，CreateSkuInput 上的 binding tag 全部是死標籤（gin 不會進到
	// slice 元素內做驗證）。`min=1` 也不可省略：validator 對 slice 的 `required` 只檢查
	// 非 nil，`"skus": []` 會照樣通過——零 SKU 商品會建成上架但不可購買
	// （priceMin=0 / stock=0）。product-service 端另有同樣的收斂點檢查。
	Skus []CreateSkuInput `json:"skus" binding:"required,min=1,dive"`
	// max=10 是筆數上限（擋一次送一萬筆的 DoS），**不是值域驗證**：值域非法一律走
	// filterCertificationInputs 剔除該筆並回 warning，不得回 400（19 §6.2.1／§6.2.2）。
	// 代行路徑的 validateProductArrays 早就是 10，賣家路徑原本無上限，這裡補齊。
	// ⚠️ 不得在此加 dive 或元素值域 tag——那會產生 §6.2.2 明令禁止的 400。
	Certifications []CreateCertificationInput `json:"certifications,omitempty" binding:"omitempty,max=10"`
	Hashtags       []string                   `json:"hashtags,omitempty"` // Hashtag 字串列表（任意大小寫，server normalize；上限 10 個）
}

// SellerProductMutationData 是賣家建立／更新商品的回應 data（19 §9.5）：商品詳情的每個
// 欄位照舊平鋪在 data 之下，另外並列一個 certificationWarnings。
//
// 用內嵌而不是多包一層 product：既有前端讀的是 data.id / data.name，多包一層是破壞性變更。
type SellerProductMutationData struct {
	ProductDetailItem
	// 本次未被採用的認證輸入。無警告時是空陣列不是省略——省略會讓前端分不出
	// 「後端沒實作」與「這次沒有警告」（19 §9.5）。
	CertificationWarnings []CertificationWarningOutput `json:"certificationWarnings"`
}

// UpdateProductRequest is the request body for updating a product
// 對應新 proto 的 UpdateProductRequest
type UpdateProductRequest struct {
	Description    *ProductDescriptionInput  `json:"description,omitempty"`
	IdempotencyKey string                    `json:"idempotencyKey,omitempty"`
	Name           string                    `json:"name,omitempty"`
	CategoryID     string                    `json:"categoryId,omitempty"`
	Origin         string                    `json:"origin,omitempty"`
	Status         string                    `json:"status,omitempty"`
	Images         []CreateProductImageInput `json:"images,omitempty"`
	// 省略 skus = 不動 SKU；有帶則逐項驗證（`dive`），必填集合見 UpdateSkuInput。
	Skus []UpdateSkuInput `json:"skus,omitempty" binding:"omitempty,dive"`
	// 上限與禁令同 CreateProductRequest.Certifications。
	Certifications []CreateCertificationInput `json:"certifications,omitempty" binding:"omitempty,max=10"`
	Hashtags       []string                   `json:"hashtags,omitempty"`       // Hashtag 字串列表
	UpdateHashtags bool                       `json:"updateHashtags,omitempty"` // true=本次以 hashtags 全量取代；false/未設=不變更
}

// convertDescriptionInputToProto 將 ProductDescriptionInput 轉換為 pb.ProductDescription
// Gateway body 使用 style 物件，轉換為 Proto 的 style_json 字串
//
// objectKey 走 resolveInputObjectKey 正規化（49 號 §25 / 10 檔 §25：前端可控輸入只收 objectKey）：
// 剝殼本平台顯示 URL、拒絕外來域名與非 public/ 前綴。商品描述圖是公開資產，前綴一律 public/。
// 不做這層會讓完整 URL 直接落進 products.description 的 objectKey——product-service 只在
// stagedToken finalize 成功時覆寫該欄位（沒有 stagedToken 的 block 原樣落庫），髒值一旦
// 落庫就會隨「載入→再送出」自我複製。
func convertDescriptionInputToProto(mediaBase string, input *ProductDescriptionInput) (*pb.ProductDescription, error) {
	if input == nil {
		return nil, nil
	}

	blocks := make([]*pb.DescriptionBlock, 0, len(input.Blocks))
	for _, b := range input.Blocks {
		objectKey, err := resolveInputObjectKey(mediaBase, b.ImageKey, "", "public/")
		if err != nil {
			return nil, err
		}

		block := &pb.DescriptionBlock{
			Type:        b.Type,
			Id:          b.ID,
			Content:     b.Content,
			TableJson:   b.TableJson,
			ListJson:    b.ListJson,
			StagedToken: b.StagedToken,
			AssetId:     b.AssetID,
			ObjectKey:   objectKey,
			Width:       b.Width,
			Height:      b.Height,
			Alt:         b.Alt,
		}

		// 行內 runs（inline-run 模型）
		if len(b.Runs) > 0 {
			runs := make([]*pb.InlineRun, 0, len(b.Runs))
			for _, r := range b.Runs {
				runs = append(runs, &pb.InlineRun{
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

		// 將 style 物件序列化為 style_json 字串
		if len(b.Style) > 0 {
			styleBytes, err := json.Marshal(b.Style)
			if err == nil {
				block.StyleJson = string(styleBytes)
			}
		}

		blocks = append(blocks, block)
	}

	return &pb.ProductDescription{
		Version: input.Version,
		Blocks:  blocks,
	}, nil
}

// ListSellerProducts 取得商家商品列表
// @Summary      取得商家商品列表
// @Description  取得當前登入商家的商品列表，支援狀態篩選、分類篩選、關鍵字搜尋與分頁
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status      query  string  false  "商品狀態（CodeMeta: 4001=草稿, 4002=審核中, 4003=未上架, 4004=已上架）"
// @Param        page        query  string  false  "頁碼（預設為 1）"
// @Param        page_size   query  string  false  "每頁筆數（預設為 20）"
// @Param        keyword     query  string  false  "搜尋關鍵字"
// @Param        category_id query  string  false  "第二層分類 ID（預設為 0，表示全部分類）"
// @Success      200  {object}  object{code=int,data=SellerProductListData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/products [get]
func (h *Holder) ListSellerProducts(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	page := c.DefaultQuery("page", defaultPageStr)
	size := c.DefaultQuery("page_size", defaultPageSizeStr)
	status := c.DefaultQuery("status", "")
	categoryID := c.DefaultQuery("category_id", "0")
	keyword := c.DefaultQuery("keyword", "")
	hashtags := parseHashtagsQuery(c.DefaultQuery("hashtags", ""))
	matchAll := strings.EqualFold(c.DefaultQuery("match", "any"), "all")

	resp, err := h.Product.ListSellerProductsWithHashtags(c.Request.Context(), storeID, page, size, status, categoryID, keyword, hashtags, matchAll)
	if err != nil {
		zap.S().Infof("Failed to get merchant products: %v", err)
		response.GrpcDomainErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	base := h.MediaBaseURL
	products := make([]SellerProductItem, 0, len(resp.Products))
	for _, p := range resp.Products {
		// gallery objectKey 為 SSOT，顯示 URL 由 Gateway 組（49 §6.6）
		gallery := make([]GalleryItem, 0, len(p.Gallery))
		for _, g := range p.Gallery {
			gallery = append(gallery, GalleryItem{
				AssetID:  g.AssetId,
				ImageKey: g.ObjectKey,
				ImageURL: mediaurl.BuildURL(base, g.ObjectKey),
			})
		}

		shippingMethods := make([]ShippingMethodResponse, 0, len(p.ShippingMethods))
		for _, sm := range p.ShippingMethods {
			shippingMethods = append(shippingMethods, ShippingMethodResponse{
				Type:          sm.Type,
				Temperature:   sm.Temperature,
				FreeThreshold: sm.FreeThreshold,
				FeePayer:      sm.FeePayer,
			})
		}

		products = append(products, SellerProductItem{
			ID:              p.Id,
			Name:            p.Name,
			Rating:          p.Rating,
			Status:          p.Status,
			CategoryID:      p.CategoryId,
			CategoryName:    p.CategoryName,
			Origin:          p.Origin,
			Gallery:         gallery,
			PriceMin:        p.PriceMin,
			PriceMax:        p.PriceMax,
			ListPriceMin:    p.ListPriceMin,
			ListPriceMax:    p.ListPriceMax,
			SalePriceMin:    p.SalePriceMin,
			SalePriceMax:    p.SalePriceMax,
			CreatedAt:       p.CreatedAt,
			OwnerID:         p.StoreId,
			Hashtags:        toHashtagItems(p.Hashtags),
			Stock:           p.StockQty,
			SalesCount:      p.SalesCount,
			FavoriteCount:   p.FavoriteCount,
			DeliveryTemp:    p.DeliveryTemp,
			ShippingMethods: shippingMethods,
			ImageKey:        p.ImageKey,
			ImageURL:        mediaurl.BuildURL(base, p.ImageKey),
			ImageKeys:       p.ImageKeys,
			ImageUrls:       buildMediaURLs(base, p.ImageKeys),
		})
	}

	response.OK(c, SellerProductListData{
		Products: products,
		Total:    resp.Total,
		Page:     resp.Page,
		PageSize: resp.PageSize,
	})
}

// CreateSellerProduct 建立商品
// @Summary      建立商品
// @Description  為當前登入商家建立新商品，包含名稱、分類、描述、圖片、規格、產地、認證等資訊
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CreateProductRequest  true  "建立商品請求"
// @Success      200  {object}  object{code=int,data=SellerProductMutationData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /app/products [post]
func (h *Holder) CreateSellerProduct(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// 驗證冪等鍵必填
	if req.IdempotencyKey == "" {
		response.BadRequest(c, "idempotencyKey 為必填欄位")
		return
	}
	idempotencyKey := req.IdempotencyKey

	// 轉換圖片
	images := make([]*pb.ProductImage, 0, len(req.Images))
	for _, img := range req.Images {
		images = append(images, &pb.ProductImage{
			Id:          img.ID,
			StagedToken: img.StagedToken,
			AssetId:     img.AssetID,
		})
	}

	// 轉換 SKU（配送資訊在 SKU 層級）
	skus := make([]*pb.ProductSku, 0, len(req.Skus))
	for _, v := range req.Skus {
		// 轉換 SKU 層級的運送方式
		skuShippingMethods := make([]*pb.ShippingMethod, 0, len(v.ShippingMethods))
		for _, sm := range v.ShippingMethods {
			skuShippingMethods = append(skuShippingMethods, &pb.ShippingMethod{
				Type:          sm.Type,
				Temperature:   sm.Temperature,
				FreeThreshold: sm.FreeThreshold,
				FeePayer:      sm.FeePayer,
			})
		}
		skus = append(skus, &pb.ProductSku{
			// 建立時不帶 Id：SKU 主鍵由 product-service 產生，前端送來的 id 一律無意義。
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
		})
	}

	// 轉換認證，接著在送進 gRPC 之前剔除非法的那幾筆（19 §6.2.1 第 ① 層）。
	// 剔除不回 4xx：非法值在舊版 App 的 AI 上架流程上可達，改成 400 會當場弄壞它。
	certifications := make([]*pb.Certification, 0, len(req.Certifications))
	for _, cert := range req.Certifications {
		certifications = append(certifications, &pb.Certification{
			Name:   cert.Name,
			CertNo: cert.CertNo,
		})
	}
	certifications, certWarnings := filterCertificationInputs(c, certifications, "", int64(storeID))

	description, err := convertDescriptionInputToProto(h.MediaBaseURL, req.Description)
	if err != nil {
		response.BadRequest(c, "invalid description image: "+err.Error())
		return
	}

	grpcReq := &pb.CreateProductRequest{
		IdempotencyKey: idempotencyKey,
		Name:           req.Name,
		CategoryId:     req.CategoryID,
		Description:    description,
		Images:         images,
		Skus:           skus,
		StoreId:        int64(storeID),
		Origin:         req.Origin,
		Certifications: certifications,
		Hashtags:       req.Hashtags,
	}

	resp, err := h.Product.CreateProduct(c.Request.Context(), grpcReq)
	if err != nil {
		zap.S().Infof("Failed to create product: %v", err)
		response.GrpcDomainErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, SellerProductMutationData{
		ProductDetailItem:     convertToProductDetail(h.MediaBaseURL, resp.Data),
		CertificationWarnings: mergeCertificationWarnings(certWarnings, resp.GetCertificationWarnings()),
	})
}

// UpdateSellerProduct 更新商品
// @Summary      更新商品
// @Description  更新指定商品的資訊，包含名稱、分類、描述、圖片、規格、產地、認證、狀態等欄位
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        pid      path  string              true  "商品 ID（UUID）"
// @Param        request  body  UpdateProductRequest true  "更新商品請求"
// @Success      200  {object}  object{code=int,data=SellerProductMutationData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/products/{id} [put]
func (h *Holder) UpdateSellerProduct(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	productID := c.Param("product_id")
	if productID == "" {
		productID = c.Param("id") // app 路由使用 :id
	}
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// 驗證冪等鍵必填
	if req.IdempotencyKey == "" {
		response.BadRequest(c, "idempotencyKey 為必填欄位")
		return
	}
	idempotencyKey := req.IdempotencyKey

	// 轉換圖片
	images := make([]*pb.ProductImage, 0, len(req.Images))
	for _, img := range req.Images {
		images = append(images, &pb.ProductImage{
			Id:          img.ID,
			StagedToken: img.StagedToken,
			AssetId:     img.AssetID,
		})
	}

	// 轉換 SKU（配送資訊在 SKU 層級）
	skus := make([]*pb.ProductSku, 0, len(req.Skus))
	for _, v := range req.Skus {
		// 轉換 SKU 層級的運送方式
		skuShippingMethods := make([]*pb.ShippingMethod, 0, len(v.ShippingMethods))
		for _, sm := range v.ShippingMethods {
			skuShippingMethods = append(skuShippingMethods, &pb.ShippingMethod{
				Type:          sm.Type,
				Temperature:   sm.Temperature,
				FreeThreshold: sm.FreeThreshold,
				FeePayer:      sm.FeePayer,
			})
		}
		skus = append(skus, &pb.ProductSku{
			Id:              v.ID,
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
			// SKU 層樂觀鎖 token（#2545）。漏了透傳＝欄位在契約上存在但永遠是空字串，
			// product-service 一律不比對，鎖形同不存在。
			ExpectedVersion: v.ExpectedVersion,
		})
	}

	// 轉換認證，接著剔除非法的那幾筆（19 §6.2.1 第 ① 層；理由同 CreateSellerProduct）。
	// ⚠️ 剔除只影響送下去的內容，不影響 UpdateCertifications 旗標：一次「只送了一筆非法
	// 認證」的更新仍然是「本次要全量取代」，全部剔光的結果就是清空——那是賣家送出的
	// 內容如實反映，不是靜默失敗（warning 會告訴他那一筆沒被採用）。
	certifications := make([]*pb.Certification, 0, len(req.Certifications))
	for _, cert := range req.Certifications {
		certifications = append(certifications, &pb.Certification{
			Name:   cert.Name,
			CertNo: cert.CertNo,
		})
	}
	certifications, certWarnings := filterCertificationInputs(c, certifications, productID, int64(storeID))

	description, err := convertDescriptionInputToProto(h.MediaBaseURL, req.Description)
	if err != nil {
		response.BadRequest(c, "invalid description image: "+err.Error())
		return
	}

	grpcReq := &pb.UpdateProductRequest{
		IdempotencyKey: idempotencyKey,
		ProductId:      productID,
		StoreId:        int64(storeID),
		Name:           req.Name,
		CategoryId:     req.CategoryID,
		Description:    description,
		Images:         images,
		Skus:           skus,
		Origin:         req.Origin,
		Certifications: certifications,
		// body 有帶 certifications（含明確帶空陣列）＝這次要全量取代。判斷用 req 而不是
		// 上面那個 make 出來的 slice：後者恆非 nil，分不出「沒帶」與「帶空的」。
		// 沒有這個旗標時賣家清不掉自己的認證標章——送出成功、畫面重整後標章又回來。
		UpdateCertifications: req.Certifications != nil,
		Status:               req.Status,
		Hashtags:             req.Hashtags,
		UpdateHashtags:       req.UpdateHashtags,
	}

	resp, err := h.Product.UpdateProduct(c.Request.Context(), grpcReq)
	if err != nil {
		zap.S().Infof("Failed to update product: %v", err)
		response.GrpcDomainErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, SellerProductMutationData{
		ProductDetailItem:     convertToProductDetail(h.MediaBaseURL, resp.Data),
		CertificationWarnings: mergeCertificationWarnings(certWarnings, resp.GetCertificationWarnings()),
	})
}

// GetSellerProductDetail 取得商品編輯詳情
// @Summary      取得商品編輯詳情
// @Description  取得指定商品的完整詳情，供商家編輯使用。僅限商品擁有者存取
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        pid  path  string  true  "商品 ID（UUID）"
// @Success      200  {object}  object{code=int,data=ProductDetailItem}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/products/{id} [get]
func (h *Holder) GetSellerProductDetail(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Forbidden(c, "seller access required")
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	productID := c.Param("product_id")
	if productID == "" {
		productID = c.Param("id") // app 路由使用 :id
	}
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	resp, err := h.Product.GetProduct(c.Request.Context(), productID, int64(storeID))
	if err != nil {
		zap.S().Infof("Failed to get product detail: %v", err)
		response.GrpcDomainErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	if resp.Data.CreatedBy != strconv.FormatInt(int64(storeID), 10) {
		response.Forbidden(c, "not authorized to access this product")
		return
	}

	response.OK(c, convertToProductDetail(h.MediaBaseURL, resp.Data))
}

// DeleteSellerProduct 賣家軟刪除商品
// @Summary      刪除商品（軟刪除）
// @Description  賣家刪除自己的商品，僅草稿(status=4001)或未上架(status=4003)狀態可刪除，操作為軟刪除（設定 deleted_at）
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "商品 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/products/{id} [delete]
func (h *Holder) DeleteSellerProduct(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Forbidden(c, "seller access required")
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	productID := c.Param("id")
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	resp, err := h.Product.DeleteProduct(c.Request.Context(), productID, storeID)
	if err != nil {
		zap.S().Infof("Failed to delete product: %v", err)
		response.GrpcDomainErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, nil)
}

// UpdateSellerProductStatusRequest 賣家更新商品狀態請求
type UpdateSellerProductStatusRequest struct {
	Status string `json:"status" binding:"required"` // 目標狀態碼 (4003=下架, 4004=上架)
}

// UpdateSellerProductStatus 賣家更新商品狀態（上架/下架）
// @Summary      賣家更新商品狀態
// @Description  賣家更新自己商品的狀態（上架/下架）。允許轉換：草稿→上架、上架→下架、下架→上架。審核中的商品不可操作
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                               true  "商品 ID"
// @Param        request  body      UpdateSellerProductStatusRequest      true  "更新商品狀態請求（status: 4003=下架, 4004=上架）"
// @Success      200      {object}  object{code=int,message=string}
// @Failure      400      {object}  object{code=int,message=string}
// @Failure      403      {object}  object{code=int,message=string}
// @Failure      404      {object}  object{code=int,message=string}
// @Router       /app/products/{id}/status [put]
func (h *Holder) UpdateSellerProductStatus(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Forbidden(c, "seller access required")
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	productID := c.Param("id")
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	var req UpdateSellerProductStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Product.UpdateSellerProductStatus(c.Request.Context(), productID, storeID, req.Status)
	if err != nil {
		response.GrpcDomainErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, nil)
}
