package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	gwcodemeta "github.com/Will413028/hawkr/gateway/internal/codemeta"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

// DelegatedProductListItem is one row of GET .../products.
//
// 不含 rating／reviewCount／favoriteCount（買家側社群數據，不是賣家自主維護欄位）、
// 不含 imageUrl／imageUrls（媒體欄位只回 objectKey）、不含 publicationState／
// purchasable（列表不畫買家視角預覽；詳情 DTO 帶這兩欄，理由見 DelegatedProductDetail）。
type DelegatedProductListItem struct {
	ProductID    string `json:"productId"`
	Version      string `json:"version"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	StatusLabel  string `json:"statusLabel"`
	PriceMin     string `json:"priceMin"`
	PriceMax     string `json:"priceMax"`
	StockQty     string `json:"stockQty"`
	SalesCount   string `json:"salesCount"`
	ImageKey     string `json:"imageKey"`
	CategoryName string `json:"categoryName"`
}

// DelegatedProductImage is one element of the detail DTO's images[].
//
// assetId 必須有：PATCH 的 images 是整份替換，要保留既有圖只能以
// {"assetId":"…"} 回送；沒有它，每一次「只改價格」都會把賣家的商品圖片清空，
// 而請求完全合法、expectedVersion 相符、回 code: 0。
type DelegatedProductImage struct {
	AssetID  string `json:"assetId"`
	ImageKey string `json:"imageKey"`
}

// DelegatedProductSku mirrors the PATCH body's sku element field-for-field, so the
// GET → edit → PATCH round trip lines up. Note spec ↔ 下游 ProductSku.sku_name：
// 欄名不同，這是最容易接錯的一欄。
type DelegatedProductSku struct {
	SkuID           string                   `json:"skuId"`
	Spec            string                   `json:"spec"`
	Price           string                   `json:"price"`
	SalePrice       string                   `json:"salePrice"`
	StockQty        string                   `json:"stockQty"`
	Unit            string                   `json:"unit"`
	Weight          string                   `json:"weight"`
	LengthCm        string                   `json:"lengthCm"`
	WidthCm         string                   `json:"widthCm"`
	HeightCm        string                   `json:"heightCm"`
	ShippingMethods []ShippingMethodResponse `json:"shippingMethods"`
}

// DelegatedProductCertification is {name, certNo}.
type DelegatedProductCertification struct {
	Name   string `json:"name"`
	CertNo string `json:"certNo"`
}

// DelegatedProductDescription is the delegated view of the block document.
//
// 與賣家自助端的 ProductDescriptionResponse 分開一個型別，理由與 §8.1.5 原則 5 同一
// 條：媒體欄位只回 objectKey。既有 DTO 的 block 帶了 imageUrl 孿生欄，沿用它等於在
// 代行回應裡塞一個由 Gateway 組好的完整網址，而域名的唯一來源是 MEDIA_BASE_URL、
// URL 組裝的唯一入口在前端 buildImageUrl(key, preset)。
//
// 型別分離而不是執行期清空：清空會讓宣告出一個實際不出現的欄位，前端照著接必拿
// undefined。
type DelegatedProductDescription struct {
	Version string                      `json:"version"`
	Blocks  []DelegatedDescriptionBlock `json:"blocks"`
}

// DelegatedDescriptionBlock 帶 assetId（賣家編輯 round-trip 需要）但不帶 imageUrl。
type DelegatedDescriptionBlock struct {
	Type      string                 `json:"type"`
	ID        string                 `json:"id,omitempty"`
	Runs      []InlineRunDTO         `json:"runs,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Style     map[string]interface{} `json:"style,omitempty"`
	TableJson string                 `json:"tableJson,omitempty"`
	ListJson  string                 `json:"listJson,omitempty"`
	AssetID   string                 `json:"assetId,omitempty"`
	ImageKey  string                 `json:"imageKey,omitempty"`
	Width     string                 `json:"width,omitempty"`
	Height    string                 `json:"height,omitempty"`
	Alt       string                 `json:"alt,omitempty"`
}

// DelegatedProductDetail is GET .../products/{productId}.
//
// publicationState／purchasable 曾以「公開端的衍生旗標」為由排除在代行投影外，那個判斷
// 現在不成立了：56 號 §2.2 out-of-scope 第 13 列定版，代行工作區的買家視角預覽必須以本
// 端點回傳的 DTO **自繪** —— 不新增預覽端點、也不得改打 /api/v1/public/*（草稿在公開路徑
// 會被 publish_status 過濾掉，預覽會靜默空白而不是報錯）。而這兩欄正是「買家看不看得到、
// 買不買得到」的判定依據，少了它們自繪就無從判起。
//
// ⚠️ purchasable 是 bool 且**不得**加 omitempty：前端的判定寫成 `purchasable !== false`
// （hawkr-website features/product/deriveDetailLayoutProps.ts:391），欄位一旦消失就
// 恆為 true —— 草稿商品的預覽會顯示成「可以購買」，而且不報錯、不轉紅。
type DelegatedProductDetail struct {
	ProductID        string                          `json:"productId"`
	Version          string                          `json:"version"`
	Name             string                          `json:"name"`
	Status           string                          `json:"status"`
	StatusLabel      string                          `json:"statusLabel"`
	PublicationState string                          `json:"publicationState"`
	Purchasable      bool                            `json:"purchasable"`
	CategoryID       string                          `json:"categoryId"`
	CategoryName     string                          `json:"categoryName"`
	Description      *DelegatedProductDescription    `json:"description"`
	Images           []DelegatedProductImage         `json:"images"`
	Origin           string                          `json:"origin"`
	Certifications   []DelegatedProductCertification `json:"certifications"`
	Hashtags         []string                        `json:"hashtags"`
	Skus             []DelegatedProductSku           `json:"skus"`
	PriceMin         string                          `json:"priceMin"`
	PriceMax         string                          `json:"priceMax"`
	StockQty         string                          `json:"stockQty"`
	SalesCount       string                          `json:"salesCount"`
	CreatedAt        string                          `json:"createdAt"`
	UpdatedAt        string                          `json:"updatedAt"`
}

// DelegatedProductImageInput is stagedToken XOR assetId.
type DelegatedProductImageInput struct {
	StagedToken string `json:"stagedToken"`
	AssetID     string `json:"assetId"`
}

// DelegatedShippingMethodInput 沒有溫層：閘道一律補常溫。
type DelegatedShippingMethodInput struct {
	Type          string `json:"type"`
	FreeThreshold string `json:"freeThreshold"`
	FeePayer      string `json:"feePayer"`
}

// DelegatedProductSkuInput is one element of skus[].
type DelegatedProductSkuInput struct {
	SkuID           string                         `json:"skuId"`
	Spec            *string                        `json:"spec"`
	Price           *string                        `json:"price"`
	SalePrice       *string                        `json:"salePrice"`
	StockQty        *string                        `json:"stockQty"`
	Unit            *string                        `json:"unit"`
	Weight          *string                        `json:"weight"`
	LengthCm        *string                        `json:"lengthCm"`
	WidthCm         *string                        `json:"widthCm"`
	HeightCm        *string                        `json:"heightCm"`
	ShippingMethods []DelegatedShippingMethodInput `json:"shippingMethods"`
}

// DelegatedProductCreateRequest is POST .../products.
//
// 沒有 status 欄位：建立一律落 4003（未上架），上架只能走 /publish 明確命令。
// DisallowUnknownFields 讓一個帶了 status 的 body 直接 9260，而不是靜默忽略——後者會
// 讓前端以為自己送出的上架指令生效了。
type DelegatedProductCreateRequest struct {
	OperationReason string                          `json:"operationReason"`
	NotifySeller    *bool                           `json:"notifySeller"`
	Name            string                          `json:"name"`
	CategoryID      string                          `json:"categoryId"`
	Description     *ProductDescriptionInput        `json:"description"`
	Images          []DelegatedProductImageInput    `json:"images"`
	Skus            []DelegatedProductSkuInput      `json:"skus"`
	Origin          string                          `json:"origin"`
	Certifications  []DelegatedProductCertification `json:"certifications"`
	Hashtags        []string                        `json:"hashtags"`
}

// DelegatedProductUpdateRequest is PATCH .../products/{productId}.
type DelegatedProductUpdateRequest struct {
	ExpectedVersion string                          `json:"expectedVersion"`
	OperationReason string                          `json:"operationReason"`
	NotifySeller    *bool                           `json:"notifySeller"`
	Name            *string                         `json:"name"`
	CategoryID      *string                         `json:"categoryId"`
	Description     *ProductDescriptionInput        `json:"description"`
	Images          []DelegatedProductImageInput    `json:"images"`
	Skus            []DelegatedProductSkuInput      `json:"skus"`
	Origin          *string                         `json:"origin"`
	Certifications  []DelegatedProductCertification `json:"certifications"`
	Hashtags        []string                        `json:"hashtags"`
	// RemovedSkuIDs 是刪除規格的**唯一**表達方式。「沒出現在 skus 裡」不是刪除：
	// 那會讓「只改一個 SKU 的庫存」變成「刪掉其他所有規格」，而請求完全合法、
	// expectedVersion 相符、回 code: 0。
	RemovedSkuIDs []string `json:"removedSkuIds"`
}

// DelegatedProductStatusRequest is the /publish and /unpublish body.
type DelegatedProductStatusRequest struct {
	ExpectedVersion string `json:"expectedVersion"`
	OperationReason string `json:"operationReason"`
	NotifySeller    *bool  `json:"notifySeller"`
}

// DelegatedBatchUnpublishItem is one element of the batch request.
type DelegatedBatchUnpublishItem struct {
	ProductID       string `json:"productId"`
	ExpectedVersion string `json:"expectedVersion"`
}

// DelegatedBatchUnpublishRequest is POST .../products/batch-unpublish.
//
// notifySeller 是整批一個旗標（與 operationReason 同形），但會被逐項寫進全部 N 個
// intent item —— notify_seller 掛在每一項上，而 proto3 分不出「未設定」與 false。
type DelegatedBatchUnpublishRequest struct {
	OperationReason string                        `json:"operationReason"`
	NotifySeller    *bool                         `json:"notifySeller"`
	Items           []DelegatedBatchUnpublishItem `json:"items"`
}

// DelegatedBatchResultItem is one element of the batch response.
type DelegatedBatchResultItem struct {
	ProductID  string `json:"productId"`
	OK         bool   `json:"ok"`
	Code       int32  `json:"code"`
	Version    string `json:"version"`
	AuditLogID string `json:"auditLogId"`
}

// DelegatedBatchUnpublishData is the batch response payload. The top-level envelope
// code stays 0 even when some elements failed: the front end discards data on any
// non-zero code, which would take every auditLogId with it.
type DelegatedBatchUnpublishData struct {
	Requested string                     `json:"requested"`
	Succeeded string                     `json:"succeeded"`
	Failed    string                     `json:"failed"`
	Results   []DelegatedBatchResultItem `json:"results"`
}

// ==================== handlers ====================

// ListDelegatedProducts handles GET .../products.
func (h *Holder) ListDelegatedProducts(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, false)
	if !ok {
		return
	}
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	keyword, ok := delegatedKeyword(c)
	if !ok {
		return
	}
	if h.Product == nil {
		denyDelegated(c, upstreamDenial())
		return
	}

	orch := h.orchestrator()
	resp, err := h.Product.GetProductListForDelegation(c.Request.Context(), &productpb.GetProductListRequest{
		// owner_store_id 是「呼叫者自己的 store」語意；store_id（field 9）是公開頁
		// 「指定任意賣家」語意，兩者不可混用。
		OwnerStoreId: req.StoreID,
		Keyword:      keyword,
		Status:       c.Query("status"),
		Page:         strconv.FormatInt(int64(page), 10),
		PageSize:     strconv.FormatInt(int64(pageSize), 10),
		Delegation:   orch.Context(delegation.Spec{Session: req.Session}, authz.AdminCapProductsRead, ""),
	}, h.Delegation.GrpcDeadlineDomainReadMs)
	if err != nil {
		denyDelegated(c, denialFromGrpcErr(err))
		return
	}
	if resp.GetCode() != 0 {
		h.recordProductReadDenial(c, orch, req, "", resp.GetCode())
		denyDelegated(c, delegationDenialFromCode(resp.GetCode(), resp.GetMessage()))
		return
	}

	records := make([]DelegatedProductListItem, 0, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		records = append(records, DelegatedProductListItem{
			ProductID:    p.GetId(),
			Version:      p.GetVersion(),
			Name:         p.GetName(),
			Status:       p.GetStatus(),
			StatusLabel:  codemeta.ProductStatusLabel(p.GetStatus()),
			PriceMin:     p.GetPriceMin(),
			PriceMax:     p.GetPriceMax(),
			StockQty:     p.GetStockQty(),
			SalesCount:   p.GetSalesCount(),
			ImageKey:     p.GetImageKey(),
			CategoryName: p.GetCategoryName(),
		})
	}
	response.Page(c, records, resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

// GetDelegatedProduct handles GET .../products/{productId}.
func (h *Holder) GetDelegatedProduct(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, false)
	if !ok {
		return
	}
	productID := c.Param("product_id")
	orch := h.orchestrator()
	res, denial := h.readProduct(c.Request.Context(), req.StoreID, productID,
		orch.Context(delegation.Spec{Session: req.Session}, authz.AdminCapProductsRead, ""))
	if denial != nil {
		h.recordProductReadDenial(c, orch, req, productID, denial.Code)
		denyDelegated(c, denial)
		return
	}
	response.OK(c, res.Payload)
}

// CreateDelegatedProduct handles POST .../products.
func (h *Holder) CreateDelegatedProduct(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	var body DelegatedProductCreateRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}
	if !validateProductName(c, &body.Name) ||
		!validateProductCategoryID(c, &body.CategoryID) ||
		!validateProductDescription(c, body.Description) ||
		!validateProductImages(c, body.Images) ||
		!validateProductSkus(c, body.Skus, true) ||
		!validateProductArrays(c, body.Origin, body.Certifications, body.Hashtags) {
		return
	}
	// 值域非法的認證在這裡剔除（19 §6.2.1 第 ① 層）。建立時還沒有 productId。
	filteredCerts, localCertWarnings := filterDelegatedCertifications(c, body.Certifications, "", req.StoreID)
	body.Certifications = filteredCerts
	// 收斂點判定出來的 CONCURRENT_CONFLICT 只出現在 gRPC 回應裡，用一個宣告在
	// closure 外的變數接住。用賦值不用 append：Write 有可能被重跑，append 會讓
	// 同一批 warning 出現兩次。
	var upstreamCertWarnings []*productpb.CertificationWarning

	description, err := convertDescriptionInputToProto(h.MediaBaseURL, body.Description)
	if err != nil {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品描述的圖片欄位不正確")
		return
	}

	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:      req.Session,
		Action:       "PRODUCT_CREATE",
		ResourceType: "product",
		ReadCap:      authz.AdminCapProductsRead,
		WriteCap:     authz.AdminCapProductsWrite,
		// 全表比較。建立類沒有 before，每一欄都是「— → 值」，收窄反而會漏記剛建出來
		// 的內容。
		SnapshotFields: nil,
		// 建立類沒有 pre-image，名稱只能從請求本身取。不填的話這一列的
		// resource_name 是空的，而 resource_id 要等 confirm 才補上 —— 稽核主控台的
		// 「目標資源」欄因此指不回任何一件商品，正是建立這個動作最需要交代的東西。
		// 取請求值也比取寫入後的名稱誠實：寫入失敗的那一列同樣記得下「當時要建的是誰」。
		ResourceName:    body.Name,
		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		// ReadBefore 為 nil：建立類沒有 before 狀態，不是「有一個空的」。
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			images := make([]*productpb.ProductImage, 0, len(body.Images))
			for _, img := range body.Images {
				images = append(images, &productpb.ProductImage{StagedToken: img.StagedToken, AssetId: img.AssetID})
			}
			skus := make([]*productpb.ProductSku, 0, len(body.Skus))
			for _, s := range body.Skus {
				skus = append(skus, newProductSkuProto(s, nil))
			}
			resp, err := h.Product.CreateProductForDelegation(ctx, &productpb.CreateProductRequest{
				IdempotencyKey: req.IdempotencyKey,
				Name:           body.Name,
				CategoryId:     body.CategoryID,
				Description:    description,
				Images:         images,
				Skus:           skus,
				StoreId:        req.StoreID,
				Origin:         body.Origin,
				Certifications: toCertificationProtos(body.Certifications),
				// status 明確填 4003 未上架。
				//
				// 不是 4001：DRAFT 的語意是「僅建立者可查看」，而代行建立的商品從第一刻
				// 起就該歸賣家看見與接手 —— 一個賣家看不到的「草稿」正是代行不該製造的
				// 隱藏狀態。4003 的語意涵蓋「從未上架」，且 INACTIVE→ACTIVE 是賣家自己
				// 就走得通的既有轉換（ProductService.validateSellerStatusTransition），
				// 賣家不必等代行者回來才能把它上架。
				//
				// ⚠️ 這一行不能省。product-service 的 ProductEntityConverter.toEntity 在
				// **未指定 status 時預設落地 ACTIVE(4004)**（ProductService.createProduct:222
				// 的註解明載），所以「不填 = 不上架」是錯的：本機實打坐實，代行建立出來的
				// 商品直接是 4004、買家立刻看得到。
				//
				// 那是一個**繞過 capability** 的洞：建立只要 products.write，而讓商品對買家
				// 可見本來只有 products.publish 做得到。少了這一行，一個沒有上架權的工作
				// 階段可以用「建立商品」把商品直接上架，稽核列上留下的卻是 PRODUCT_CREATE。
				Status:     strconv.Itoa(codemeta.ProductStatusInactive),
				Hashtags:   body.Hashtags,
				Delegation: dctx,
			}, h.Delegation.GrpcDeadlineDomainWriteMs)
			if err != nil {
				return nil, denialFromGrpcErr(err)
			}
			if resp.GetCode() != 0 {
				return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
			}
			upstreamCertWarnings = resp.GetCertificationWarnings()
			return h.productResource(resp.GetData()), nil
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	respondDelegatedProductMutation(c, result, mergeCertificationWarnings(localCertWarnings, upstreamCertWarnings))
}

// UpdateDelegatedProduct handles PATCH .../products/{productId}.
func (h *Holder) UpdateDelegatedProduct(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	productID := c.Param("product_id")
	var body DelegatedProductUpdateRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	if !validateExpectedVersion(c, body.ExpectedVersion) {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}
	if !validateProductName(c, body.Name) ||
		!validateProductCategoryID(c, body.CategoryID) ||
		!validateProductDescription(c, body.Description) ||
		!validateProductImages(c, body.Images) ||
		!validateProductSkus(c, body.Skus, false) ||
		!validateRemovedSkuIDs(c, body.RemovedSkuIDs, body.Skus) {
		return
	}
	origin := ""
	if body.Origin != nil {
		origin = *body.Origin
	}
	if !validateProductArrays(c, origin, body.Certifications, body.Hashtags) {
		return
	}
	// 只過濾「這一次送進來的」認證，不碰 before 快照的回填值：既有列可能是本期之前
	// 落庫的舊值，把它們剔掉等於用一次無關的更新順手清掉賣家的標章。
	filteredCerts, localCertWarnings := filterDelegatedCertifications(c, body.Certifications, productID, req.StoreID)
	body.Certifications = filteredCerts
	// 理由同 CreateDelegatedProduct：收斂點的 CONCURRENT_CONFLICT 只在 gRPC 回應裡。
	var upstreamCertWarnings []*productpb.CertificationWarning

	description, err := convertDescriptionInputToProto(h.MediaBaseURL, body.Description)
	if err != nil {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品描述的圖片欄位不正確")
		return
	}

	var before *DelegatedProductDetail
	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:      req.Session,
		Action:       "PRODUCT_UPDATE",
		ResourceType: "product",
		ReadCap:      authz.AdminCapProductsRead,
		WriteCap:     authz.AdminCapProductsWrite,
		// 全表比較：PATCH 可以動到規則表裡幾乎每一個可編輯欄，而 UpdateProduct 回的是
		// 完整 ProductDetail，兩側鍵集合對稱。
		SnapshotFields:  nil,
		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		ResourceID:      productID,
		ReadBefore: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			res, denial := h.readProduct(ctx, req.StoreID, productID, dctx)
			if denial != nil {
				return nil, denial
			}
			detail := res.Payload.(DelegatedProductDetail)
			before = &detail
			return res, nil
		},
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			// assetId 的越權比對對象是 orchestrator 於 intent 之前讀到的**原始資源
			// 物件**，不是持久化的 before_snapshot：images 進快照時被投影成數量而非
			// assetId 陣列，拿快照比對會永遠不命中，這條防線會靜默失效。
			if denial := verifyImageAssetIDs(body.Images, before); denial != nil {
				return nil, denial
			}
			if denial := verifySkuOwnership(body.Skus, body.RemovedSkuIDs, before); denial != nil {
				return nil, denial
			}
			// 未送的欄位一律以 before 快照回填後整份送下游。UpdateProductRequest 的
			// 純量欄位是值型別，空字串在 product-service 是「覆寫成空」而不是「保留」
			// ⇒ 只改一個名稱會把產地一起清掉，而 expectedVersion 相符、回 code: 0，
			// 破壞完全靜默（本機實打坐實：origin 從「台南善化」被清成空字串）。
			//
			// 陣列同理但語意不同：images／certifications 是整份替換，未送就必須把
			// 原內容回送；skus 未送＝不動（product-service 對空清單直接略過）；
			// hashtags 靠 update_hashtags 旗標表達「這次不動」。
			images := make([]*productpb.ProductImage, 0, len(body.Images))
			if body.Images != nil {
				for _, img := range body.Images {
					images = append(images, &productpb.ProductImage{StagedToken: img.StagedToken, AssetId: img.AssetID})
				}
			} else if before != nil {
				for _, img := range before.Images {
					images = append(images, &productpb.ProductImage{AssetId: img.AssetID})
				}
			}
			skus := make([]*productpb.ProductSku, 0, len(body.Skus))
			for _, s := range body.Skus {
				skus = append(skus, newProductSkuProto(s, findBeforeSku(before, s.SkuID)))
			}
			certifications := body.Certifications
			// body 有給（含明確給空陣列）＝這次要以它全量取代；沒給才回填 before 保持原狀。
			// 少了這個旗標，「移除唯一一張認證」會送出空陣列而 product-service 分不出
			// 「空」與「未提供」⇒ 靜默不動、稽核卻記 SUCCEEDED（見 product.proto
			// update_certifications 的註解）。
			updateCertifications := body.Certifications != nil
			if certifications == nil && before != nil {
				certifications = before.Certifications
			}
			updateReq := &productpb.UpdateProductRequest{
				IdempotencyKey:       req.IdempotencyKey,
				ProductId:            productID,
				StoreId:              req.StoreID,
				Description:          description,
				Images:               images,
				Skus:                 skus,
				Certifications:       toCertificationProtos(certifications),
				UpdateCertifications: updateCertifications,
				// status 不填：發布狀態只能經 /publish、/unpublish 改。
				ExpectedVersion: body.ExpectedVersion,
				Delegation:      dctx,
			}
			if before != nil {
				updateReq.Name = before.Name
				updateReq.CategoryId = before.CategoryID
				updateReq.Origin = before.Origin
			}
			if body.Name != nil {
				updateReq.Name = *body.Name
			}
			if body.CategoryID != nil {
				updateReq.CategoryId = *body.CategoryID
			}
			if body.Origin != nil {
				updateReq.Origin = *body.Origin
			}
			if body.Hashtags != nil {
				updateReq.Hashtags = body.Hashtags
				updateReq.UpdateHashtags = true
			}
			resp, err := h.Product.UpdateProductForDelegation(ctx, updateReq, h.Delegation.GrpcDeadlineDomainWriteMs)
			if err != nil {
				return nil, denialFromGrpcErr(err)
			}
			if resp.GetCode() != 0 {
				return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
			}
			upstreamCertWarnings = resp.GetCertificationWarnings()
			return h.productResource(resp.GetData()), nil
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	respondDelegatedProductMutation(c, result, mergeCertificationWarnings(localCertWarnings, upstreamCertWarnings))
}

// PublishDelegatedProduct handles POST .../products/{productId}/publish.
func (h *Holder) PublishDelegatedProduct(c *gin.Context) {
	h.changeDelegatedProductStatus(c, "PRODUCT_PUBLISH", strconv.Itoa(codemeta.ProductStatusActive))
}

// UnpublishDelegatedProduct handles POST .../products/{productId}/unpublish.
func (h *Holder) UnpublishDelegatedProduct(c *gin.Context) {
	h.changeDelegatedProductStatus(c, "PRODUCT_UNPUBLISH", strconv.Itoa(codemeta.ProductStatusInactive))
}

func (h *Holder) changeDelegatedProductStatus(c *gin.Context, action, targetStatus string) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	productID := c.Param("product_id")
	var body DelegatedProductStatusRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	if !validateExpectedVersion(c, body.ExpectedVersion) {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}

	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:      req.Session,
		Action:       action,
		ResourceType: "product",
		ReadCap:      authz.AdminCapProductsRead,
		WriteCap:     authz.AdminCapProductsPublish,
		// 只比 status。/publish 與 /unpublish 的 body 只有 expectedVersion／
		// operationReason／notifySeller，內容欄一律動不到（§6.1.8），所以這一列的
		// changedFields 依契約只會有 status（§6.1.11）。
		//
		// 不收窄的後果不是理論值：UpdateSellerProductStatus 回的 ProductDetail 曾經
		// 不帶 hashtags，於是每一次上下架都虛報一列「主題標籤被清空」，而 DB 裡的
		// product_hashtag 一列未動。上游那個缺口已補（ProductService.java），但這裡
		// 不靠它——領域服務回什麼形狀，不該決定稽核記什麼。
		SnapshotFields:  delegation.ScopeProductStatus(),
		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		ResourceID:      productID,
		ReadBefore: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			return h.readProduct(ctx, req.StoreID, productID, dctx)
		},
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			resp, err := h.Product.UpdateSellerProductStatusForDelegation(ctx, &productpb.UpdateSellerProductStatusRequest{
				ProductId:       productID,
				StoreId:         req.StoreID,
				Status:          targetStatus,
				ExpectedVersion: body.ExpectedVersion,
				IdempotencyKey:  req.IdempotencyKey,
				Delegation:      dctx,
			}, h.Delegation.GrpcDeadlineDomainWriteMs)
			if err != nil {
				return nil, denialFromGrpcErr(err)
			}
			if resp.GetCode() != 0 {
				return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
			}
			return h.productResource(resp.GetData()), nil
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	respondDelegatedMutation(c, result)
}

// BatchUnpublishDelegatedProducts handles POST .../products/batch-unpublish.
func (h *Holder) BatchUnpublishDelegatedProducts(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	var body DelegatedBatchUnpublishRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}
	// items 上限與分頁的 page_size 上限必須由同一個常數提供，不得各寫各的字面值。
	if len(body.Items) < 1 || len(body.Items) > maxPageSize {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "items 需為 1–50 筆")
		return
	}
	seen := map[string]struct{}{}
	items := make([]delegation.BatchItem, 0, len(body.Items))
	for _, it := range body.Items {
		if !delegatedVersionRE.MatchString(it.ProductID) || !delegatedVersionRE.MatchString(it.ExpectedVersion) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "items 的 productId 或 expectedVersion 格式不正確")
			return
		}
		if _, dup := seen[it.ProductID]; dup {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "items 的 productId 不可重複")
			return
		}
		seen[it.ProductID] = struct{}{}
		items = append(items, delegation.BatchItem{ResourceID: it.ProductID, ExpectedVersion: it.ExpectedVersion})
	}

	orch := h.orchestrator()
	results, denial := orch.OrchestrateBatch(c.Request.Context(), delegation.BatchSpec{
		Session:      req.Session,
		Action:       "PRODUCT_BATCH_UNPUBLISH",
		ResourceType: "product",
		ReadCap:      authz.AdminCapProductsRead,
		WriteCap:     authz.AdminCapProductsPublish,

		// 只比 status，理由同單筆下架。批次這一側更需要它：AfterFields 是手寫的衍生
		// 投影（批次回應只有新版本、沒有資源），一旦 ReadBatch 哪天改成回完整資源，
		// 兩側就會不對稱。
		SnapshotFields: delegation.ScopeProductStatus(),

		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		Items:           items,
		ReadBatch: func(ctx context.Context, dctx *commonpb.DelegationContext) (map[string]*delegation.Resource, *delegation.Denial) {
			ids := make([]string, 0, len(items))
			for _, it := range items {
				ids = append(ids, it.ResourceID)
			}
			resp, err := h.Product.BatchGetSellerProductsForDelegation(ctx, &productpb.BatchGetSellerProductsForDelegationRequest{
				StoreId:    req.StoreID,
				ProductIds: ids,
				Delegation: dctx,
			}, h.Delegation.GrpcDeadlineBatchDomainReadMs)
			if err != nil {
				return nil, denialFromGrpcErr(err)
			}
			if resp.GetCode() != 0 {
				return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
			}
			out := map[string]*delegation.Resource{}
			for _, snap := range resp.GetResults() {
				if !snap.GetFound() {
					continue
				}
				out[snap.GetProductId()] = batchProductResource(h, snap)
			}
			return out, nil
		},
		WriteBatch: func(ctx context.Context, dctx *commonpb.DelegationContext, writeItems []delegation.BatchWriteItem) ([]delegation.BatchWriteResult, *delegation.Denial) {
			protoItems := make([]*productpb.BatchProductStatusItem, 0, len(writeItems))
			for _, wi := range writeItems {
				protoItems = append(protoItems, &productpb.BatchProductStatusItem{
					ProductId:       wi.ResourceID,
					ExpectedVersion: wi.ExpectedVersion,
					// audit_log_id 由 Gateway 填、領域服務原樣回音：稽核列在
					// delegation 模組，領域服務拿不到這個值。
					AuditLogId: wi.AuditLogID,
				})
			}
			resp, err := h.Product.BatchUpdateSellerProductStatusForDelegation(ctx, &productpb.BatchUpdateSellerProductStatusRequest{
				StoreId:        req.StoreID,
				Status:         strconv.Itoa(codemeta.ProductStatusInactive),
				Items:          protoItems,
				IdempotencyKey: req.IdempotencyKey,
				Delegation:     dctx,
			}, h.Delegation.GrpcDeadlineBatchDomainWriteMs)
			if err != nil {
				return nil, denialFromGrpcErr(err)
			}
			if resp.GetCode() != 0 {
				return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
			}
			out := make([]delegation.BatchWriteResult, 0, len(resp.GetResults()))
			for _, r := range resp.GetResults() {
				out = append(out, delegation.BatchWriteResult{
					ResourceID: r.GetProductId(),
					OK:         r.GetOk(),
					Code:       mapDomainItemCode(r.GetCode()),
					Version:    r.GetVersion(),
				})
			}
			return out, nil
		},
		AfterFields: func(_ map[string]any) map[string]any {
			return map[string]any{"status": strconv.Itoa(codemeta.ProductStatusInactive)}
		},
	})
	if denial != nil {
		// 整批在逐項執行**之前**就被拒（沒有 results 可回）。9276 是這個位置的兜底
		// 碼，不是部分失敗的表達。
		response.BizStatusData(c, denial.Code, denial.Message, nil)
		return
	}

	data := DelegatedBatchUnpublishData{
		Requested: strconv.Itoa(len(results)),
		Results:   make([]DelegatedBatchResultItem, 0, len(results)),
	}
	succeeded, failed := 0, 0
	for _, r := range results {
		if r.OK {
			succeeded++
		} else {
			failed++
		}
		data.Results = append(data.Results, DelegatedBatchResultItem{
			ProductID:  r.ResourceID,
			OK:         r.OK,
			Code:       r.Code,
			Version:    r.Version,
			AuditLogID: r.AuditLogID,
		})
	}
	data.Succeeded = strconv.Itoa(succeeded)
	data.Failed = strconv.Itoa(failed)
	response.OK(c, data)
}

// ==================== 讀取與轉換 ====================

func (h *Holder) readProduct(ctx context.Context, storeID int64, productID string, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
	if h.Product == nil {
		return nil, upstreamDenial()
	}
	resp, err := h.Product.GetProductForDelegation(ctx, &productpb.GetProductRequest{
		ProductId:  productID,
		StoreId:    storeID,
		Delegation: dctx,
	}, h.Delegation.GrpcDeadlineDomainReadMs)
	if err != nil {
		return nil, denialFromGrpcErr(err)
	}
	if resp.GetCode() != 0 {
		return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
	}
	return h.productResource(resp.GetData()), nil
}

func (h *Holder) recordProductReadDenial(c *gin.Context, orch *delegation.Orchestrator, req *delegatedRequest, productID string, code int32) {
	if code != bizcode.DelegationResourceNotOwned {
		return
	}
	orch.RecordReadDenial(c.Request.Context(), req.readDenial("product", productID, code))
}

// batchProductResource 把批次 before 快照的一項轉成 orchestrator 要的 Resource。
//
// 走的是**與單筆路徑同一支** productResource()，這是刻意的：before_snapshot 的欄位集合
// 與格式化語意只准存在一處。批次曾經在這裡自己組 `{"status": …}`，於是
// PRODUCT_BATCH_UNPUBLISH 的 before_snapshot 只有一個鍵，而同一個操作的單筆版本
// （PRODUCT_UNPUBLISH）存的是完整 12 欄——批次正是「一次動很多件」的高風險操作，舉證素材
// 反而最少。⚠️ changed_fields 不受影響：批次的比較範圍仍是 ScopeProductStatus()，收窄的是
// Diff 不是 Project（見 delegation/diff.go 的 SnapshotFields 註解）。
//
// detail 缺席時退回舊形狀而不是照樣投影：productResource(nil) 會產出一整組空字串，稽核列
// 會記下「名稱是空的、產地是空的」這種從未成立的事實，比只記 status 更糟。這條路徑只在滾動
// 部署的窗口內走得到（Gateway 已更新、product-service 還沒）。
func batchProductResource(h *Holder, snap *productpb.BatchProductSnapshot) *delegation.Resource {
	if snap.GetDetail() != nil {
		return h.productResource(snap.GetDetail())
	}
	return &delegation.Resource{
		ID:      snap.GetProductId(),
		Name:    snap.GetName(),
		Version: snap.GetVersion(),
		Fields:  map[string]any{"status": snap.GetStatus()},
	}
}

func (h *Holder) productResource(detail *productpb.ProductDetail) *delegation.Resource {
	images := make([]DelegatedProductImage, 0, len(detail.GetGallery()))
	for _, g := range detail.GetGallery() {
		images = append(images, DelegatedProductImage{AssetID: g.GetAssetId(), ImageKey: g.GetObjectKey()})
	}
	hashtags := make([]string, 0, len(detail.GetHashtags()))
	for _, tag := range detail.GetHashtags() {
		hashtags = append(hashtags, tag.GetName())
	}
	certs := make([]DelegatedProductCertification, 0, len(detail.GetCertifications()))
	certNames := make([]string, 0, len(detail.GetCertifications()))
	for _, cert := range detail.GetCertifications() {
		certs = append(certs, DelegatedProductCertification{Name: cert.GetName(), CertNo: cert.GetCertNo()})
		certNames = append(certNames, cert.GetName())
	}
	skus := make([]DelegatedProductSku, 0, len(detail.GetSkus()))
	for _, s := range detail.GetSkus() {
		skus = append(skus, DelegatedProductSku{
			SkuID:           s.GetId(),
			Spec:            s.GetSkuName(),
			Price:           s.GetPrice(),
			SalePrice:       s.GetSalePrice(),
			StockQty:        s.GetStockQty(),
			Unit:            s.GetUnit(),
			Weight:          s.GetWeight(),
			LengthCm:        s.GetLengthCm(),
			WidthCm:         s.GetWidthCm(),
			HeightCm:        s.GetHeightCm(),
			ShippingMethods: toShippingMethodResponses(s.GetShippingMethods()),
		})
	}

	dto := DelegatedProductDetail{
		ProductID:   detail.GetId(),
		Version:     detail.GetVersion(),
		Name:        detail.GetName(),
		Status:      detail.GetStatus(),
		StatusLabel: codemeta.ProductStatusLabel(detail.GetStatus()),
		// 上游一律填這兩欄（ProductEntityConverter.toDetailDTO 由 ProductPublicationPolicy
		// 推導，非可選欄位），投影原樣帶出來即可——它們是預覽自繪的判定依據，不是可省的裝飾。
		PublicationState: detail.GetPublicationState(),
		Purchasable:      detail.GetPurchasable(),
		CategoryID:       detail.GetCategoryLv3Id(),
		CategoryName:     detail.GetCategoryName(),
		Description:      delegatedDescriptionDTO(detail.GetDescription()),
		Images:           images,
		Origin:           detail.GetOrigin(),
		Certifications:   certs,
		Hashtags:         hashtags,
		Skus:             skus,
		PriceMin:         detail.GetPriceMin(),
		PriceMax:         detail.GetPriceMax(),
		StockQty:         detail.GetStockQty(),
		SalesCount:       detail.GetSalesCount(),
		CreatedAt:        detail.GetCreatedAt(),
		UpdatedAt:        detail.GetUpdatedAt(),
	}

	imageKey := ""
	if len(images) > 0 {
		imageKey = images[0].ImageKey
	}
	return &delegation.Resource{
		ID:       dto.ProductID,
		Name:     dto.Name,
		ImageKey: imageKey,
		Version:  dto.Version,
		Fields: map[string]any{
			"name":         dto.Name,
			"categoryName": dto.CategoryName,
			"description":  descriptionPlainText(dto.Description),
			"origin":       dto.Origin,
			"status":       dto.Status,
			"priceMin":     dto.PriceMin,
			"priceMax":     dto.PriceMax,
			"stockQty":     dto.StockQty,
			// skus／images 投影成數量而非內容：內容差異會讓單列輕易破 KB 級，而
			// UI 三欄表要的是「規格從 2 項變 3 項」這種可讀事實。
			"skus":           strconv.Itoa(len(skus)),
			"images":         strconv.Itoa(len(images)),
			"hashtags":       joinForDisplay(hashtags),
			"certifications": joinForDisplay(certNames),
		},
		Payload: dto,
	}
}

// descriptionPlainText flattens the block document into one scalar for the
// projection: the formatter layer only handles scalars, which is why seven of them
// are enough.
func descriptionPlainText(desc *DelegatedProductDescription) string {
	if desc == nil {
		return ""
	}
	var b strings.Builder
	for _, block := range desc.Blocks {
		if block.Content != "" {
			b.WriteString(block.Content)
		}
		for _, run := range block.Runs {
			b.WriteString(run.Text)
		}
	}
	return b.String()
}

// filterDelegatedCertifications 是代行路徑（W3／W4）的認證輸入 filter（19 §6.2 的表）。
//
// 代行走的是另一個 DTO，但判定規則與賣家路徑共用 certificationInputDecision——
// 值域規則只有一份，這裡只負責把遮罩套回代行自己的 slice。
// 剔除仍然不回 4xx：validateProductArrays 只擋筆數，值域一律剔除該筆＋warn log。
func filterDelegatedCertifications(c *gin.Context, certs []DelegatedProductCertification, productID string, storeID int64) ([]DelegatedProductCertification, []CertificationWarningOutput) {
	refs := make([]certificationInputRef, 0, len(certs))
	for _, cert := range certs {
		refs = append(refs, certificationInputRef{Type: cert.Name, No: cert.CertNo})
	}
	keep, warnings := certificationInputDecision(c, refs, productID, storeID)
	// nil 進 nil 出：代行更新用 `body.Certifications != nil` 區分「未提供」與「清空」，
	// 把 nil 換成空 slice 會讓「這次不動認證」變成「清空全部認證」。
	if certs == nil {
		return nil, warnings
	}
	kept := make([]DelegatedProductCertification, 0, len(certs))
	for i, cert := range certs {
		if !keep[i] {
			continue
		}
		// 理由同賣家路徑：驗過的值與送出的值必須是同一個（normalizedCertNo）。
		cert.CertNo = normalizedCertNo(cert.Name, cert.CertNo)
		kept = append(kept, cert)
	}
	return kept, warnings
}

func toCertificationProtos(certs []DelegatedProductCertification) []*productpb.Certification {
	out := make([]*productpb.Certification, 0, len(certs))
	for _, c := range certs {
		out = append(out, &productpb.Certification{Name: c.Name, CertNo: c.CertNo})
	}
	return out
}

// newProductSkuProto merges one request element over its pre-image. Fields absent
// from the element keep the existing value — the same partial-update rule the scalar
// fields follow.
func newProductSkuProto(in DelegatedProductSkuInput, before *DelegatedProductSku) *productpb.ProductSku {
	sku := &productpb.ProductSku{Id: in.SkuID}
	if before != nil {
		sku.SkuName = before.Spec
		sku.Price = before.Price
		sku.SalePrice = before.SalePrice
		sku.StockQty = before.StockQty
		sku.Unit = before.Unit
		sku.Weight = before.Weight
		sku.LengthCm = before.LengthCm
		sku.WidthCm = before.WidthCm
		sku.HeightCm = before.HeightCm
		sku.ShippingMethods = toShippingMethodProtos(before.ShippingMethods)
	}
	if in.Spec != nil {
		sku.SkuName = *in.Spec
	}
	if in.Price != nil {
		sku.Price = *in.Price
	}
	if in.SalePrice != nil {
		sku.SalePrice = *in.SalePrice
	}
	if in.StockQty != nil {
		sku.StockQty = *in.StockQty
	}
	if in.Unit != nil {
		sku.Unit = *in.Unit
	}
	if in.Weight != nil {
		sku.Weight = *in.Weight
	}
	if in.LengthCm != nil {
		sku.LengthCm = *in.LengthCm
	}
	if in.WidthCm != nil {
		sku.WidthCm = *in.WidthCm
	}
	if in.HeightCm != nil {
		sku.HeightCm = *in.HeightCm
	}
	if in.ShippingMethods != nil {
		sku.ShippingMethods = toShippingMethodProtosFromInput(in.ShippingMethods)
	}
	return sku
}

func toShippingMethodProtos(methods []ShippingMethodResponse) []*productpb.ShippingMethod {
	out := make([]*productpb.ShippingMethod, 0, len(methods))
	for _, m := range methods {
		out = append(out, shippingMethodProto(m.Type, m.FreeThreshold, m.FeePayer))
	}
	return out
}

func toShippingMethodProtosFromInput(methods []DelegatedShippingMethodInput) []*productpb.ShippingMethod {
	out := make([]*productpb.ShippingMethod, 0, len(methods))
	for _, m := range methods {
		out = append(out, shippingMethodProto(m.Type, m.FreeThreshold, m.FeePayer))
	}
	return out
}

func shippingMethodProto(methodType, freeThreshold, feePayer string) *productpb.ShippingMethod {
	return &productpb.ShippingMethod{
		Type:          methodType,
		Temperature:   ambientShippingTemperature,
		FreeThreshold: freeThreshold,
		FeePayer:      feePayer,
	}
}

func findBeforeSku(before *DelegatedProductDetail, skuID string) *DelegatedProductSku {
	if before == nil || skuID == "" {
		return nil
	}
	for i := range before.Skus {
		if before.Skus[i].SkuID == skuID {
			return &before.Skus[i]
		}
	}
	return nil
}

// verifyImageAssetIDs rejects an assetId that was not in the product's own gallery.
// 9275 / 403, and the whole request is refused rather than partially applied.
func verifyImageAssetIDs(images []DelegatedProductImageInput, before *DelegatedProductDetail) *delegation.Denial {
	if len(images) == 0 {
		return nil
	}
	owned := map[string]struct{}{}
	if before != nil {
		for _, img := range before.Images {
			owned[img.AssetID] = struct{}{}
		}
	}
	for _, img := range images {
		if img.AssetID == "" {
			continue
		}
		if _, ok := owned[img.AssetID]; !ok {
			return delegation.Deny(bizcode.DelegationResourceNotOwned, "圖片不屬於本商品")
		}
	}
	return nil
}

// verifySkuOwnership rejects a skuId that belongs to another product (or another
// store). Cross-product SKU edits refuse the entire request.
func verifySkuOwnership(skus []DelegatedProductSkuInput, removed []string, before *DelegatedProductDetail) *delegation.Denial {
	owned := map[string]struct{}{}
	if before != nil {
		for _, s := range before.Skus {
			owned[s.SkuID] = struct{}{}
		}
	}
	for _, s := range skus {
		if s.SkuID == "" {
			continue
		}
		if _, ok := owned[s.SkuID]; !ok {
			return delegation.Deny(bizcode.DelegationResourceNotOwned, "規格不屬於本商品")
		}
	}
	for _, id := range removed {
		if _, ok := owned[id]; !ok {
			return delegation.Deny(bizcode.DelegationResourceNotOwned, "規格不屬於本商品")
		}
	}
	// 刪到零個 SKU：商品至少保留一個規格。
	if before != nil && len(removed) > 0 {
		remaining := len(before.Skus) - len(removed)
		for _, s := range skus {
			if s.SkuID == "" {
				remaining++
			}
		}
		if remaining < 1 {
			return delegation.Deny(bizcode.DelegationInvalidCommand, "商品至少需保留一個規格")
		}
	}
	return nil
}

// ==================== 值域 ====================

func validateProductName(c *gin.Context, name *string) bool {
	if name == nil {
		return true
	}
	trimmed := strings.TrimSpace(*name)
	// 上限 255（DB 欄位定義）。產品需求畫的 60 字計數器不是任何一層的實際上限，
	// 照稿設限會讓合法商品名在前端就被擋下。
	if trimmed == "" || len([]rune(trimmed)) > 255 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品名稱需為 1–255 字")
		return false
	}
	return true
}

func validateProductCategoryID(c *gin.Context, categoryID *string) bool {
	if categoryID == nil {
		return true
	}
	if !delegatedVersionRE.MatchString(*categoryID) {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "categoryId 格式不正確")
		return false
	}
	return true
}

func validateProductDescription(c *gin.Context, desc *ProductDescriptionInput) bool {
	if desc == nil {
		return true
	}
	if len(desc.Blocks) > 100 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品介紹的區塊數不得超過 100")
		return false
	}
	for _, b := range desc.Blocks {
		switch b.Type {
		case "text", "table", "list", "image":
		default:
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品介紹的區塊類型不合法")
			return false
		}
	}
	return true
}

func validateProductImages(c *gin.Context, images []DelegatedProductImageInput) bool {
	if len(images) > 10 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品圖片最多 10 張")
		return false
	}
	for _, img := range images {
		hasToken := img.StagedToken != ""
		hasAsset := img.AssetID != ""
		if hasToken == hasAsset {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品圖片必須是 stagedToken 或 assetId 二選一")
			return false
		}
		if hasToken && len(img.StagedToken) > 4096 {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "stagedToken 不正確")
			return false
		}
		if hasAsset && !delegatedVersionRE.MatchString(img.AssetID) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "assetId 格式不正確")
			return false
		}
	}
	return true
}

func validateProductSkus(c *gin.Context, skus []DelegatedProductSkuInput, required bool) bool {
	if required && len(skus) == 0 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "至少需要一個商品規格")
		return false
	}
	if len(skus) > 20 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品規格最多 20 個")
		return false
	}
	for _, s := range skus {
		if s.SkuID != "" && !delegatedVersionRE.MatchString(s.SkuID) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "skuId 格式不正確")
			return false
		}
		if required && (s.Spec == nil || s.Price == nil || s.StockQty == nil || s.Unit == nil) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "商品規格缺少必填欄位")
			return false
		}
		if s.Spec != nil {
			trimmed := strings.TrimSpace(*s.Spec)
			if trimmed == "" || len([]rune(trimmed)) > 100 {
				response.BizStatus(c, bizcode.DelegationInvalidCommand, "規格名稱需為 1–100 字")
				return false
			}
		}
		if s.Price != nil && !positiveDecimal(*s.Price) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "規格售價必須大於 0")
			return false
		}
		if s.SalePrice != nil && *s.SalePrice != "" && !positiveDecimal(*s.SalePrice) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "規格特價必須大於 0")
			return false
		}
		if s.StockQty != nil && !nonNegativeInteger(*s.StockQty) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "規格庫存必須是 0 或正整數")
			return false
		}
		if s.Unit != nil && !gwcodemeta.IsValidWeightUnit(*s.Unit) {
			// unit 存的是 WeightUnit 的 Value（3301／3302／3303），不是 Code、不是
			// 中文。髒值落庫會讓商品上架成功，到結帳鏈才由 WeightUnit.fromValue()
			// 拋例外。
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "規格單位不在合法值域內")
			return false
		}
	}
	return true
}

func validateRemovedSkuIDs(c *gin.Context, removed []string, skus []DelegatedProductSkuInput) bool {
	if len(removed) == 0 {
		return true
	}
	inSkus := map[string]struct{}{}
	for _, s := range skus {
		if s.SkuID != "" {
			inSkus[s.SkuID] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	for _, id := range removed {
		if !delegatedVersionRE.MatchString(id) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "removedSkuIds 格式不正確")
			return false
		}
		if _, dup := seen[id]; dup {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "removedSkuIds 不可重複")
			return false
		}
		seen[id] = struct{}{}
		if _, both := inSkus[id]; both {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "同一個 skuId 不可同時出現在 skus 與 removedSkuIds")
			return false
		}
	}
	// 下游 UpdateProductRequest 目前沒有承載刪除的欄位（product-service 全平台都還
	// 沒有刪除 SKU 的路徑）。接受這個欄位卻送不下去就是靜默失效——請求合法、
	// 回 code: 0、規格一個也沒被刪掉，而操作者以為刪掉了。明確拒絕，直到下游補上。
	response.BizStatus(c, bizcode.DelegationInvalidCommand, "刪除商品規格尚未開放")
	return false
}

func validateProductArrays(c *gin.Context, origin string, certs []DelegatedProductCertification, hashtags []string) bool {
	if len([]rune(origin)) > 255 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "產地超過長度上限")
		return false
	}
	if len(certs) > 10 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "產品驗證最多 10 個")
		return false
	}
	// Gateway 只擋筆數；正規化與重複判定由 product-service 權威。
	if len(hashtags) > 10 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "主題標籤最多 10 個")
		return false
	}
	return true
}

func positiveDecimal(s string) bool {
	if s == "" {
		return false
	}
	f, err := strconv.ParseFloat(s, 64)
	return err == nil && f > 0
}

func nonNegativeInteger(s string) bool {
	if s == "" {
		return false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && n >= 0
}

// delegatedDescriptionDTO projects the block document without the imageUrl twin.
func delegatedDescriptionDTO(desc *productpb.ProductDescription) *DelegatedProductDescription {
	if desc == nil {
		return nil
	}
	blocks := make([]DelegatedDescriptionBlock, 0, len(desc.GetBlocks()))
	for _, b := range desc.GetBlocks() {
		block := DelegatedDescriptionBlock{
			Type:      b.GetType(),
			ID:        b.GetId(),
			Content:   b.GetContent(),
			TableJson: b.GetTableJson(),
			ListJson:  b.GetListJson(),
			AssetID:   b.GetAssetId(),
			ImageKey:  b.GetObjectKey(),
			Width:     b.GetWidth(),
			Height:    b.GetHeight(),
			Alt:       b.GetAlt(),
		}
		if len(b.GetRuns()) > 0 {
			runs := make([]InlineRunDTO, 0, len(b.GetRuns()))
			for _, r := range b.GetRuns() {
				runs = append(runs, InlineRunDTO{
					Text:      r.GetText(),
					Bold:      r.GetBold(),
					Italic:    r.GetItalic(),
					Underline: r.GetUnderline(),
					Color:     r.GetColor(),
					Href:      r.GetHref(),
				})
			}
			block.Runs = runs
		}
		if raw := b.GetStyleJson(); raw != "" {
			var style map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &style); err == nil {
				block.Style = style
			}
		}
		blocks = append(blocks, block)
	}
	return &DelegatedProductDescription{Version: desc.GetVersion(), Blocks: blocks}
}
