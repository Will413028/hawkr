package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product_import"
	"github.com/gin-gonic/gin"
)

// ==================== DTO（入參） ====================

// CreateImportJobMultipartForm 建立匯入任務 multipart 欄位（純文件，非實際 binding 結構）
//
// 由於 CSV 透過 `multipart/form-data` 上傳，欄位以 `c.PostForm` / `c.FormFile` 取出，
// 此 struct 僅用於 Swagger 自動文件展示，handler 不會反序列化。
type CreateImportJobMultipartForm struct {
	Filename     string `form:"filename"`                // 原始檔名（不傳則用 multipart header 內 filename）
	TargetStatus string `form:"target_status"`           // ACTIVE | DRAFT | PENDING_REVIEW（job 級預設）
	File         string `form:"file" swaggertype:"file"` // CSV binary
}

// ReuploadImportJobCsvMultipartForm 重傳 CSV multipart 欄位
type ReuploadImportJobCsvMultipartForm struct {
	Filename string `form:"filename"`
	File     string `form:"file" swaggertype:"file"`
}

// CommitImportJobRequest 確認提交請求
type CommitImportJobRequest struct {
	RetryFailedOnly bool     `json:"retryFailedOnly"` // true = 只重試 COMMIT_FAILED 的 item
	IncludeItemIds  []string `json:"includeItemIds"`  // 空 = 全部 READY/COMMIT_FAILED
}

// UpdateImportItemRequest 編輯單一 item 請求（採 *_set 模式：設定 set=true 才更新該欄位）
type UpdateImportItemRequest struct {
	ExpectedVersion string `json:"expectedVersion" binding:"required"` // 樂觀鎖（數字字串）

	NameSet bool   `json:"nameSet"`
	Name    string `json:"name"`

	CategoryIdSet bool   `json:"categoryIdSet"`
	CategoryId    string `json:"categoryId"`

	OriginSet bool   `json:"originSet"`
	Origin    string `json:"origin"`

	TargetStatusSet bool   `json:"targetStatusSet"`
	TargetStatus    string `json:"targetStatus"` // ACTIVE | DRAFT | PENDING_REVIEW

	DescriptionSet bool   `json:"descriptionSet"`
	Description    string `json:"description"` // JSON 字串

	HashtagsSet bool     `json:"hashtagsSet"`
	Hashtags    []string `json:"hashtags"`

	CertificationsSet bool                    `json:"certificationsSet"`
	Certifications    []CertificationInputDTO `json:"certifications"`

	GalleryImagesSet bool                      `json:"galleryImagesSet"`
	GalleryImages    []ImportItemImageInputDTO `json:"galleryImages"` // 新圖片列表（順序），index 0 為封面；URL / stagedToken 二擇一

	SkusSet bool                `json:"skusSet"`
	Skus    []UpdateSkuInputDTO `json:"skus"`

	AllowDuplicateNameSet bool `json:"allowDuplicateNameSet"`
	AllowDuplicateName    bool `json:"allowDuplicateName"` // 賣家勾選「允許與既有商品同名」
}

// UpdateSkuInputDTO 編輯時的 SKU 結構
type UpdateSkuInputDTO struct {
	SkuId           string                   `json:"skuId"` // 空 = 新增；非空 = 更新
	SkuName         string                   `json:"skuName"`
	Price           string                   `json:"price"`
	SalePrice       string                   `json:"salePrice"`
	StockQty        string                   `json:"stockQty"`
	Weight          string                   `json:"weight"`
	Unit            string                   `json:"unit"`
	LengthCm        string                   `json:"lengthCm"`
	WidthCm         string                   `json:"widthCm"`
	HeightCm        string                   `json:"heightCm"`
	ShippingMethods []ShippingMethodInputDTO `json:"shippingMethods"`
	Delete          bool                     `json:"delete"` // true = 刪除此 SKU
}

// ShippingMethodInputDTO 編輯 SKU 用的運送方式（sku 層無 freeThreshold）
type ShippingMethodInputDTO struct {
	Method      string `json:"method"`      // ShippingMethod value
	Temperature string `json:"temperature"` // ShippingTemperature value
}

// CertificationInputDTO 認證輸入
type CertificationInputDTO struct {
	Name   string `json:"name"`   // CertificationType value（ORGANIC / TAP / ...）
	CertNo string `json:"certNo"` // 證書號碼
}

// ImportItemImageInputDTO 圖片輸入（sourceUrl / stagedToken 二擇一）
//   - sourceUrl 用於外部 URL，後端走 AssetDownloadWorker 派工下載
//   - stagedToken 用於賣家本地上傳（Media CompleteUploadSession 取得），後端走 Media BindStagedAsset 同步綁定
type ImportItemImageInputDTO struct {
	SourceUrl   string `json:"sourceUrl"`
	StagedToken string `json:"stagedToken"`
}

// RemoveImportItemsRequest 批次移除請求
type RemoveImportItemsRequest struct {
	ImportItemIds []string `json:"importItemIds" binding:"required"`
}

// RestoreImportItemsRequest 批次還原請求（結構同 Remove）
type RestoreImportItemsRequest struct {
	ImportItemIds []string `json:"importItemIds" binding:"required"`
}

// RetryImportItemAssetsRequest 重試 FAILED 圖片請求
type RetryImportItemAssetsRequest struct {
	AssetIds []string `json:"assetIds"` // 空 = 重試所有 FAILED
}

// BatchUpdateItemsShippingRequest 批次設定運送
type BatchUpdateItemsShippingRequest struct {
	ImportItemIds   []string                      `json:"importItemIds"` // 空 + applyAll=true 表全 job
	ApplyAll        bool                          `json:"applyAll"`
	ShippingMethods []BatchShippingMethodInputDTO `json:"shippingMethods" binding:"required"`
}

// BatchShippingMethodInputDTO 批次設定運送的單一方式（含 freeThreshold）
type BatchShippingMethodInputDTO struct {
	Type          string `json:"type"`          // ShippingMethod value
	Temperature   string `json:"temperature"`   // ShippingTemperature value
	FreeThreshold string `json:"freeThreshold"` // 空 = 不免運；"0" = 永遠免運；正整數 = 滿 X 元免運
}

// BatchUpdateItemsTargetStatusRequest 批次套用上架狀態
type BatchUpdateItemsTargetStatusRequest struct {
	ImportItemIds []string `json:"importItemIds"`
	ApplyAll      bool     `json:"applyAll"`
	TargetStatus  string   `json:"targetStatus" binding:"required"` // ACTIVE | DRAFT | PENDING_REVIEW（不接受 INACTIVE）
}

// BatchUpdateItemsCategoryRequest 批次套用商品類別（需帶 confirmation_token）
type BatchUpdateItemsCategoryRequest struct {
	ImportItemIds     []string `json:"importItemIds"`
	ApplyAll          bool     `json:"applyAll"`
	CategoryId        string   `json:"categoryId" binding:"required"`        // 葉節點 id
	ConfirmationToken string   `json:"confirmationToken" binding:"required"` // 從 GetCategoryConfirmationToken 取得
}

// ==================== DTO（回應） ====================

// ValidationErrorResponse CSV 驗證錯誤
type ValidationErrorResponse struct {
	CsvLine   string `json:"csvLine"`            // 行號，"0" 表 file-level
	Field     string `json:"field"`              // CSV 欄位名，可空
	ErrorType string `json:"errorType"`          // ImportItemErrorType value
	Message   string `json:"message"`            // 賣家可讀繁中訊息
	RawValue  string `json:"rawValue,omitempty"` // 原始值（debug，可截斷）
}

// CreateImportJobResponseData CreateImportJob 回應 data
type CreateImportJobResponseData struct {
	ImportJobId      string                    `json:"importJobId"`
	Status           string                    `json:"status"`           // PARSING | CSV_INVALID | dedup 命中時為既有 job 當前 status
	DedupHit         bool                      `json:"dedupHit"`         // true = 同 hash 已存在，回既有 job
	ValidationErrors []ValidationErrorResponse `json:"validationErrors"` // CSV_INVALID 才有值
}

// ReuploadImportJobCsvResponseData ReuploadImportJobCsv 回應 data
type ReuploadImportJobCsvResponseData struct {
	ImportJobId      string                    `json:"importJobId"`
	Status           string                    `json:"status"` // PARSING | CSV_INVALID
	ValidationErrors []ValidationErrorResponse `json:"validationErrors"`
}

// ImportJobBriefResponse 任務列表單筆
type ImportJobBriefResponse struct {
	ImportJobId    string `json:"importJobId"`
	Status         string `json:"status"` // ImportJobStatus value
	SourceFilename string `json:"sourceFilename"`
	TotalItems     string `json:"totalItems"`
	CommittedCount string `json:"committedCount"`
	FailedCount    string `json:"failedCount"`
	PendingCount   string `json:"pendingCount"`
	CreatedAt      string `json:"createdAt"`
	ExpiresAt      string `json:"expiresAt"`
}

// ImportJobCountersResponse 任務計數
type ImportJobCountersResponse struct {
	ItemsTotal             string `json:"itemsTotal"`
	ItemsParsed            string `json:"itemsParsed"`
	ItemsAssetDownloading  string `json:"itemsAssetDownloading"`
	ItemsReady             string `json:"itemsReady"`
	ItemsNeedsFix          string `json:"itemsNeedsFix"`
	ItemsRemoved           string `json:"itemsRemoved"`
	ItemsCommitting        string `json:"itemsCommitting"`
	ItemsCommitted         string `json:"itemsCommitted"`
	ItemsCommitFailed      string `json:"itemsCommitFailed"`
	ItemsWithWarnings      string `json:"itemsWithWarnings"`
	ItemsDuplicateExisting string `json:"itemsDuplicateExisting"`

	AssetsTotal       string `json:"assetsTotal"`
	AssetsPending     string `json:"assetsPending"`
	AssetsDownloading string `json:"assetsDownloading"`
	AssetsUploaded    string `json:"assetsUploaded"`
	AssetsFailed      string `json:"assetsFailed"`
	AssetsLinked      string `json:"assetsLinked"`
}

// ImportJobProgressResponse 任務進度
type ImportJobProgressResponse struct {
	OverallPercentage       string `json:"overallPercentage"` // 0~100
	ParsePercentage         string `json:"parsePercentage"`
	AssetDownloadPercentage string `json:"assetDownloadPercentage"`
	CommitPercentage        string `json:"commitPercentage"`
	ParseCompleted          bool   `json:"parseCompleted"`
	AssetDownloadCompleted  bool   `json:"assetDownloadCompleted"`
	CommitCompleted         bool   `json:"commitCompleted"`
}

// ImportJobDetailResponse 任務詳情
type ImportJobDetailResponse struct {
	ImportJobId      string                     `json:"importJobId"`
	Status           string                     `json:"status"`
	SourceFilename   string                     `json:"sourceFilename"`
	TotalRows        string                     `json:"totalRows"`
	TotalItems       string                     `json:"totalItems"`
	Counters         *ImportJobCountersResponse `json:"counters"`
	TargetStatus     string                     `json:"targetStatus"`
	ParseError       string                     `json:"parseError,omitempty"` // PARSE_FAILED 才有值
	CreatedAt        string                     `json:"createdAt"`
	UpdatedAt        string                     `json:"updatedAt"`
	ExpiresAt        string                     `json:"expiresAt"`
	CommittedAt      string                     `json:"committedAt,omitempty"`
	CancelledAt      string                     `json:"cancelledAt,omitempty"`
	Progress         *ImportJobProgressResponse `json:"progress"`
	ValidationErrors []ValidationErrorResponse  `json:"validationErrors,omitempty"` // CSV_INVALID 才有值
}

// CancelImportJobResponseData CancelImportJob 回應 data
type CancelImportJobResponseData struct {
	Status string `json:"status"` // 應為 CANCELLED
}

// CommitImportJobResponseData CommitImportJob 回應 data
type CommitImportJobResponseData struct {
	Status         string                     `json:"status"`                   // 應為 COMMITTING
	QueuedCount    string                     `json:"queuedCount"`              // 排入 commit queue 的 item 數
	CommitOutcome  string                     `json:"commitOutcome"`            // QUEUED / PARTIAL_SUCCESS / ALL_SKIPPED
	SkippedItemIds []string                   `json:"skippedItemIds,omitempty"` // 被自動剔除的 item id（partial-success）
	SkipReasons    []CommitSkipReasonResponse `json:"skipReasons,omitempty"`    // 與 skippedItemIds 1:1（依設計書 §4.2.5）
}

// CommitSkipReasonResponse 單筆跳過原因
type CommitSkipReasonResponse struct {
	ImportItemId  string `json:"importItemId"`
	ReasonCode    string `json:"reasonCode"`    // BLOCKING_ERROR / ALREADY_COMMITTED / REMOVED / NOT_READY
	ReasonMessage string `json:"reasonMessage"` // 中文短描述
}

// ImportItemErrorResponse Item 級錯誤/警告
type ImportItemErrorResponse struct {
	ErrorType string `json:"errorType"` // ImportItemErrorType value
	Severity  string `json:"severity"`  // ERROR | WARNING
	Field     string `json:"field,omitempty"`
	Message   string `json:"message"`
	CsvLine   string `json:"csvLine,omitempty"`
}

// CertificationDTOResponse 認證
type CertificationDTOResponse struct {
	Name   string `json:"name"`
	CertNo string `json:"certNo"`
}

// ImportShippingMethodResponse 運送方式（import sku 層；與 product.ShippingMethodResponse 區別：
// import 階段尚未確定 fee_payer / free_threshold，僅展示 method+temperature 配對）
type ImportShippingMethodResponse struct {
	Method      string `json:"method"`
	Temperature string `json:"temperature"`
}

// ImportItemSkuResponse SKU
type ImportItemSkuResponse struct {
	SkuId           string                         `json:"skuId"`
	CsvLine         string                         `json:"csvLine"`
	SkuName         string                         `json:"skuName"`
	Price           string                         `json:"price"`
	SalePrice       string                         `json:"salePrice"`
	StockQty        string                         `json:"stockQty"`
	Weight          string                         `json:"weight"`
	Unit            string                         `json:"unit"`
	LengthCm        string                         `json:"lengthCm"`
	WidthCm         string                         `json:"widthCm"`
	HeightCm        string                         `json:"heightCm"`
	ShippingMethods []ImportShippingMethodResponse `json:"shippingMethods"`
}

// ImportItemAssetResponse 圖片資產
type ImportItemAssetResponse struct {
	AssetId       string `json:"assetId"`
	Usage         string `json:"usage"` // GALLERY | DESCRIPTION
	SortOrder     string `json:"sortOrder"`
	BlockIndex    string `json:"blockIndex,omitempty"`
	SourceUrl     string `json:"sourceUrl"`
	Status        string `json:"status"` // ImportAssetLocalStatus value
	PreviewUrl    string `json:"previewUrl,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
	FailureDetail string `json:"failureDetail,omitempty"`
	RetryCount    string `json:"retryCount"`
}

// ImportItemBriefResponse Item 列表單筆
type ImportItemBriefResponse struct {
	ImportItemId             string `json:"importItemId"`
	ProductKey               string `json:"productKey"`
	Status                   string `json:"status"`
	Name                     string `json:"name"`
	CategoryId               string `json:"categoryId"` // 葉節點 id（數字字串）；編輯頁 form load 唯一來源；尚未通過 ValidateLeafCategoryInternal 時為空字串
	CatL3Name                string `json:"catL3Name"`
	SkuCount                 string `json:"skuCount"`
	PriceMin                 string `json:"priceMin"`
	PriceMax                 string `json:"priceMax"`
	FirstImagePreviewUrl     string `json:"firstImagePreviewUrl"` // presigned/原站 URL，非平台媒體 URL（見 GalleryItemResponse）
	BlockingErrorCount       string `json:"blockingErrorCount"`
	WarningCount             string `json:"warningCount"`
	CommitFailedReason       string `json:"commitFailedReason,omitempty"`
	FormalProductId          string `json:"formalProductId,omitempty"`
	FormalProductStatus      string `json:"formalProductStatus,omitempty"`      // COMMITTED 後正式商品 status code
	FormalProductStatusLabel string `json:"formalProductStatusLabel,omitempty"` // 中文標籤
	// #2819：ProductPublicationState（4011~4014）。4014 = 正式商品已被軟刪。
	// 與 FormalProductStatus 正交：那個答「能不能賣」，本欄答「還在不在」。
	FormalProductPublicationState string `json:"formalProductPublicationState,omitempty"`
	// #2819：true = 我方查不到該商品狀態（RPC 失敗或回應無此列）。
	// 🔴 與 publicationState=4014 是**兩件事**：4014 是「確定已移除」，本欄是「不知道」。
	// 不帶 omitempty —— false 是有意義的答案（我們查過了、沒問題），欄位消失會讓
	// 前端無法區分「查過沒問題」與「後端還沒接這個欄位」。
	FormalProductStatusUnavailable bool   `json:"formalProductStatusUnavailable"`
	NeedsFix                       bool   `json:"needsFix"`
	HasDuplicateExisting           bool   `json:"hasDuplicateExisting"`
	ExistingProductId              string `json:"existingProductId,omitempty"`          // 同名既有商品 id
	ExistingProductName            string `json:"existingProductName,omitempty"`        // 同名既有商品 name（DuplicateNameWarning 渲染）
	ExistingProductStatusLabel     string `json:"existingProductStatusLabel,omitempty"` // 同名既有商品 status 中文標籤
	CommitRevision                 string `json:"commitRevision"`
	Version                        string `json:"version"` // 樂觀鎖 entity version；前端編輯時必須帶回 expectedVersion；新建初始 "0"
}

// ImportItemDetailResponse Item 詳情
type ImportItemDetailResponse struct {
	Brief             ImportItemBriefResponse    `json:"brief"`
	DescriptionJson   string                     `json:"descriptionJson"`
	Skus              []ImportItemSkuResponse    `json:"skus"`
	GalleryAssets     []ImportItemAssetResponse  `json:"galleryAssets"`
	DescriptionAssets []ImportItemAssetResponse  `json:"descriptionAssets"`
	Errors            []ImportItemErrorResponse  `json:"errors"`
	Warnings          []ImportItemErrorResponse  `json:"warnings"`
	CommitRevision    string                     `json:"commitRevision"`
	Origin            string                     `json:"origin"`
	Hashtags          []string                   `json:"hashtags"`
	Certifications    []CertificationDTOResponse `json:"certifications"`
	TargetStatus      string                     `json:"targetStatus"`
}

// GalleryItemResponse Preview 用畫廊單筆。
//
// 欄名刻意**不叫** imageKey/imageUrl（正式商品的那組）：這裡的值是 presigned
// previewUrl 或原站 sourceUrl，不是平台媒體 URL，前端不可把它丟進 buildImageUrl，
// 也組不出 preset 縮圖。名字要說實話，否則共用取圖函式會靜默拿到不可用的 URL。
type GalleryItemResponse struct {
	PreviewUrl string `json:"previewUrl"`
	SortOrder  string `json:"sortOrder"`
}

// ProductDetailPresentationResponse 匯入預覽的商品詳情展示結構。
//
// 非媒體欄位對齊正式 ProductDetail（ProductDetailItem），**媒體欄位不相容**：
// 這裡只有 gallery[].previewUrl，沒有 imageKey／imageKeys／gallery[].assetId。
// 原因：匯入中的圖片落在 tmp/product-import/ 前綴（40 號 §5.1），image Worker 只服務
// public/ 前綴（49 §6.6），因此預覽只能吐 presigned previewUrl 或原站 sourceUrl
// （product-import ProductImportServiceImpl#buildGallery），沒有可組 URL 的 objectKey。
// 前端不可把本結構當正式 ProductDetail 共用同一組媒體讀取邏輯。
type ProductDetailPresentationResponse struct {
	Id             string                     `json:"id"` // 預覽時為空
	Name           string                     `json:"name"`
	Rating         string                     `json:"rating"` // 預覽固定 "0"
	Status         string                     `json:"status"`
	CategoryId     string                     `json:"categoryId"`
	Origin         string                     `json:"origin"`
	Description    string                     `json:"description"`
	Gallery        []GalleryItemResponse      `json:"gallery"`
	Certifications []CertificationDTOResponse `json:"certifications"`
	PriceMin       string                     `json:"priceMin"`
	PriceMax       string                     `json:"priceMax"`
	Skus           []ImportItemSkuResponse    `json:"skus"`
	CatL1Id        string                     `json:"catL1Id"`
	CatL1Name      string                     `json:"catL1Name"`
	CatL2Id        string                     `json:"catL2Id"`
	CatL2Name      string                     `json:"catL2Name"`
	CatL3Id        string                     `json:"catL3Id"`
	CatL3Name      string                     `json:"catL3Name"`
}

// ImportItemPreviewResponse 單品預覽回應
type ImportItemPreviewResponse struct {
	ImportJobId      string                            `json:"importJobId"`
	ImportItemId     string                            `json:"importItemId"`
	ImportStatus     string                            `json:"importStatus"`
	ValidationErrors []ImportItemErrorResponse         `json:"validationErrors"`
	Warnings         []ImportItemErrorResponse         `json:"warnings"`
	FormalProductId  string                            `json:"formalProductId,omitempty"`
	PreviewData      ProductDetailPresentationResponse `json:"previewData"`
}

// ProductListPresentationResponse 匯入預覽的商品列表展示結構。
//
// 非媒體欄位對齊正式 ProductListItem，**媒體欄位不相容**：只有 imagePreviewUrl 與
// gallery[].previewUrl，沒有 imageKey／imageKeys（理由同 ProductDetailPresentationResponse）。
type ProductListPresentationResponse struct {
	Id              string                `json:"id"`
	Name            string                `json:"name"`
	Rating          string                `json:"rating"` // 固定 "0"
	CategoryId      string                `json:"categoryId"`
	CategoryName    string                `json:"categoryName"` // = cat_l3_name
	Origin          string                `json:"origin"`
	Gallery         []GalleryItemResponse `json:"gallery"`
	PriceMin        string                `json:"priceMin"`
	PriceMax        string                `json:"priceMax"`
	ReviewCount     string                `json:"reviewCount"` // 固定 "0"
	SkuCount        string                `json:"skuCount"`
	ImagePreviewUrl string                `json:"imagePreviewUrl"` // 同 GalleryItemResponse：presigned/原站 URL，非平台媒體 URL
}

// ImportItemPreviewListingRecordResponse 商城模擬列表單筆
type ImportItemPreviewListingRecordResponse struct {
	ImportItemId string                          `json:"importItemId"`
	ImportStatus string                          `json:"importStatus"`
	PreviewData  ProductListPresentationResponse `json:"previewData"`
}

// ImportJobPreviewListingResponse 商城模擬列表回應
type ImportJobPreviewListingResponse struct {
	ImportJobId   string                                   `json:"importJobId"`
	TotalEligible string                                   `json:"totalEligible"`
	TotalNeedsFix string                                   `json:"totalNeedsFix"`
	TotalPending  string                                   `json:"totalPending"`
	TotalRemoved  string                                   `json:"totalRemoved"`
	Records       []ImportItemPreviewListingRecordResponse `json:"records"`
	Total         string                                   `json:"total"`
	Page          string                                   `json:"page"`
	PageSize      string                                   `json:"pageSize"`
}

// RemoveOrRestoreItemsResponseData Remove / Restore 共用
type RemoveOrRestoreItemsResponseData struct {
	AffectedCount string `json:"affectedCount"`
}

// RetryImportItemAssetsResponseData 重試圖片回應
type RetryImportItemAssetsResponseData struct {
	QueuedCount string `json:"queuedCount"`
}

// BatchUpdateItemsShippingResponseData 批次運送回應
type BatchUpdateItemsShippingResponseData struct {
	AffectedItems string `json:"affectedItems"`
	AffectedSkus  string `json:"affectedSkus"`
}

// BatchUpdateItemsTargetStatusResponseData 批次上架狀態回應
type BatchUpdateItemsTargetStatusResponseData struct {
	AffectedItems string `json:"affectedItems"`
}

// BatchUpdateItemsCategoryResponseData 批次分類回應
type BatchUpdateItemsCategoryResponseData struct {
	AffectedItems string `json:"affectedItems"`
	CatL1Id       string `json:"catL1Id"`
	CatL1Name     string `json:"catL1Name"`
	CatL2Id       string `json:"catL2Id"`
	CatL2Name     string `json:"catL2Name"`
	CatL3Id       string `json:"catL3Id"`
	CatL3Name     string `json:"catL3Name"`
}

// CategoryConfirmationTokenResponse 分類二次確認 token 回應
type CategoryConfirmationTokenResponse struct {
	ConfirmationToken string `json:"confirmationToken"` // 不透明 UUID v4 字串
	ExpiresAt         string `json:"expiresAt"`         // ISO 8601 UTC（TTL 5 分鐘）
	CatL1Id           string `json:"catL1Id"`
	CatL1Name         string `json:"catL1Name"`
	CatL2Id           string `json:"catL2Id"`
	CatL2Name         string `json:"catL2Name"`
	CatL3Id           string `json:"catL3Id"`
	CatL3Name         string `json:"catL3Name"`
}

// ==================== 轉換函式 ====================

func convertValidationErrors(list []*pb.ValidationError) []ValidationErrorResponse {
	result := make([]ValidationErrorResponse, 0, len(list))
	for _, e := range list {
		if e == nil {
			continue
		}
		result = append(result, ValidationErrorResponse{
			CsvLine:   e.GetCsvLine(),
			Field:     e.GetField(),
			ErrorType: e.GetErrorType(),
			Message:   e.GetMessage(),
			RawValue:  e.GetRawValue(),
		})
	}
	return result
}

func convertImportJobBrief(b *pb.ImportJobBrief) ImportJobBriefResponse {
	if b == nil {
		return ImportJobBriefResponse{}
	}
	return ImportJobBriefResponse{
		ImportJobId:    b.GetImportJobId(),
		Status:         b.GetStatus(),
		SourceFilename: b.GetSourceFilename(),
		TotalItems:     b.GetTotalItems(),
		CommittedCount: b.GetCommittedCount(),
		FailedCount:    b.GetFailedCount(),
		PendingCount:   b.GetPendingCount(),
		CreatedAt:      b.GetCreatedAt(),
		ExpiresAt:      b.GetExpiresAt(),
	}
}

func convertImportJobBriefs(list []*pb.ImportJobBrief) []ImportJobBriefResponse {
	result := make([]ImportJobBriefResponse, 0, len(list))
	for _, b := range list {
		result = append(result, convertImportJobBrief(b))
	}
	return result
}

func convertImportJobCounters(c *pb.ImportJobCounters) *ImportJobCountersResponse {
	if c == nil {
		return nil
	}
	return &ImportJobCountersResponse{
		ItemsTotal:             c.GetItemsTotal(),
		ItemsParsed:            c.GetItemsParsed(),
		ItemsAssetDownloading:  c.GetItemsAssetDownloading(),
		ItemsReady:             c.GetItemsReady(),
		ItemsNeedsFix:          c.GetItemsNeedsFix(),
		ItemsRemoved:           c.GetItemsRemoved(),
		ItemsCommitting:        c.GetItemsCommitting(),
		ItemsCommitted:         c.GetItemsCommitted(),
		ItemsCommitFailed:      c.GetItemsCommitFailed(),
		ItemsWithWarnings:      c.GetItemsWithWarnings(),
		ItemsDuplicateExisting: c.GetItemsDuplicateExisting(),
		AssetsTotal:            c.GetAssetsTotal(),
		AssetsPending:          c.GetAssetsPending(),
		AssetsDownloading:      c.GetAssetsDownloading(),
		AssetsUploaded:         c.GetAssetsUploaded(),
		AssetsFailed:           c.GetAssetsFailed(),
		AssetsLinked:           c.GetAssetsLinked(),
	}
}

func convertImportJobProgress(p *pb.ImportJobProgress) *ImportJobProgressResponse {
	if p == nil {
		return nil
	}
	return &ImportJobProgressResponse{
		OverallPercentage:       p.GetOverallPercentage(),
		ParsePercentage:         p.GetParsePercentage(),
		AssetDownloadPercentage: p.GetAssetDownloadPercentage(),
		CommitPercentage:        p.GetCommitPercentage(),
		ParseCompleted:          p.GetParseCompleted(),
		AssetDownloadCompleted:  p.GetAssetDownloadCompleted(),
		CommitCompleted:         p.GetCommitCompleted(),
	}
}

func convertImportJobDetail(d *pb.ImportJobDetail) ImportJobDetailResponse {
	if d == nil {
		return ImportJobDetailResponse{}
	}
	return ImportJobDetailResponse{
		ImportJobId:      d.GetImportJobId(),
		Status:           d.GetStatus(),
		SourceFilename:   d.GetSourceFilename(),
		TotalRows:        d.GetTotalRows(),
		TotalItems:       d.GetTotalItems(),
		Counters:         convertImportJobCounters(d.GetCounters()),
		TargetStatus:     d.GetTargetStatus(),
		ParseError:       d.GetParseError(),
		CreatedAt:        d.GetCreatedAt(),
		UpdatedAt:        d.GetUpdatedAt(),
		ExpiresAt:        d.GetExpiresAt(),
		CommittedAt:      d.GetCommittedAt(),
		CancelledAt:      d.GetCancelledAt(),
		Progress:         convertImportJobProgress(d.GetProgress()),
		ValidationErrors: convertValidationErrors(d.GetValidationErrors()),
	}
}

func convertItemErrors(list []*pb.ImportItemErrorDto) []ImportItemErrorResponse {
	result := make([]ImportItemErrorResponse, 0, len(list))
	for _, e := range list {
		if e == nil {
			continue
		}
		result = append(result, ImportItemErrorResponse{
			ErrorType: e.GetErrorType(),
			Severity:  e.GetSeverity(),
			Field:     e.GetField(),
			Message:   e.GetMessage(),
			CsvLine:   e.GetCsvLine(),
		})
	}
	return result
}

func convertCertifications(list []*pb.CertificationDto) []CertificationDTOResponse {
	result := make([]CertificationDTOResponse, 0, len(list))
	for _, c := range list {
		if c == nil {
			continue
		}
		result = append(result, CertificationDTOResponse{
			Name:   c.GetName(),
			CertNo: c.GetCertNo(),
		})
	}
	return result
}

func convertShippingMethods(list []*pb.ShippingMethodDto) []ImportShippingMethodResponse {
	result := make([]ImportShippingMethodResponse, 0, len(list))
	for _, m := range list {
		if m == nil {
			continue
		}
		result = append(result, ImportShippingMethodResponse{
			Method:      m.GetMethod(),
			Temperature: m.GetTemperature(),
		})
	}
	return result
}

func convertImportSkus(list []*pb.ImportItemSkuDto) []ImportItemSkuResponse {
	result := make([]ImportItemSkuResponse, 0, len(list))
	for _, s := range list {
		if s == nil {
			continue
		}
		result = append(result, ImportItemSkuResponse{
			SkuId:           s.GetSkuId(),
			CsvLine:         s.GetCsvLine(),
			SkuName:         s.GetSkuName(),
			Price:           s.GetPrice(),
			SalePrice:       s.GetSalePrice(),
			StockQty:        s.GetStockQty(),
			Weight:          s.GetWeight(),
			Unit:            s.GetUnit(),
			LengthCm:        s.GetLengthCm(),
			WidthCm:         s.GetWidthCm(),
			HeightCm:        s.GetHeightCm(),
			ShippingMethods: convertShippingMethods(s.GetShippingMethods()),
		})
	}
	return result
}

func convertImportAssets(list []*pb.ImportItemAssetDto) []ImportItemAssetResponse {
	result := make([]ImportItemAssetResponse, 0, len(list))
	for _, a := range list {
		if a == nil {
			continue
		}
		result = append(result, ImportItemAssetResponse{
			AssetId:       a.GetAssetId(),
			Usage:         a.GetUsage(),
			SortOrder:     a.GetSortOrder(),
			BlockIndex:    a.GetBlockIndex(),
			SourceUrl:     a.GetSourceUrl(),
			Status:        a.GetStatus(),
			PreviewUrl:    a.GetPreviewUrl(),
			FailureReason: a.GetFailureReason(),
			FailureDetail: a.GetFailureDetail(),
			RetryCount:    a.GetRetryCount(),
		})
	}
	return result
}

func convertImportItemBrief(b *pb.ImportItemBrief) ImportItemBriefResponse {
	if b == nil {
		return ImportItemBriefResponse{}
	}
	return ImportItemBriefResponse{
		ImportItemId:                   b.GetImportItemId(),
		ProductKey:                     b.GetProductKey(),
		Status:                         b.GetStatus(),
		Name:                           b.GetName(),
		CategoryId:                     b.GetCategoryId(),
		CatL3Name:                      b.GetCatL3Name(),
		SkuCount:                       b.GetSkuCount(),
		PriceMin:                       b.GetPriceMin(),
		PriceMax:                       b.GetPriceMax(),
		FirstImagePreviewUrl:           b.GetFirstImageUrl(),
		BlockingErrorCount:             b.GetBlockingErrorCount(),
		WarningCount:                   b.GetWarningCount(),
		CommitFailedReason:             b.GetCommitFailedReason(),
		FormalProductId:                b.GetFormalProductId(),
		FormalProductStatus:            b.GetFormalProductStatus(),
		FormalProductStatusLabel:       b.GetFormalProductStatusLabel(),
		FormalProductPublicationState:  b.GetFormalProductPublicationState(),
		FormalProductStatusUnavailable: b.GetFormalProductStatusUnavailable(),
		NeedsFix:                       b.GetNeedsFix(),
		HasDuplicateExisting:           b.GetHasDuplicateExisting(),
		ExistingProductId:              b.GetExistingProductId(),
		ExistingProductName:            b.GetExistingProductName(),
		ExistingProductStatusLabel:     b.GetExistingProductStatusLabel(),
		CommitRevision:                 b.GetCommitRevision(),
		Version:                        b.GetVersion(),
	}
}

func convertImportItemBriefs(list []*pb.ImportItemBrief) []ImportItemBriefResponse {
	result := make([]ImportItemBriefResponse, 0, len(list))
	for _, b := range list {
		result = append(result, convertImportItemBrief(b))
	}
	return result
}

func convertImportItemDetail(d *pb.ImportItemDetail) ImportItemDetailResponse {
	if d == nil {
		return ImportItemDetailResponse{}
	}
	return ImportItemDetailResponse{
		Brief:             convertImportItemBrief(d.GetBrief()),
		DescriptionJson:   d.GetDescriptionJson(),
		Skus:              convertImportSkus(d.GetSkus()),
		GalleryAssets:     convertImportAssets(d.GetGalleryAssets()),
		DescriptionAssets: convertImportAssets(d.GetDescriptionAssets()),
		Errors:            convertItemErrors(d.GetErrors()),
		Warnings:          convertItemErrors(d.GetWarnings()),
		CommitRevision:    d.GetCommitRevision(),
		Origin:            d.GetOrigin(),
		Hashtags:          d.GetHashtags(),
		Certifications:    convertCertifications(d.GetCertifications()),
		TargetStatus:      d.GetTargetStatus(),
	}
}

func convertGallery(list []*pb.GalleryItem) []GalleryItemResponse {
	result := make([]GalleryItemResponse, 0, len(list))
	for _, g := range list {
		if g == nil {
			continue
		}
		result = append(result, GalleryItemResponse{
			PreviewUrl: g.GetUrl(),
			SortOrder:  g.GetSortOrder(),
		})
	}
	return result
}

func convertProductDetailPresentation(p *pb.ProductDetailPresentation) ProductDetailPresentationResponse {
	if p == nil {
		return ProductDetailPresentationResponse{}
	}
	return ProductDetailPresentationResponse{
		Id:             p.GetId(),
		Name:           p.GetName(),
		Rating:         p.GetRating(),
		Status:         p.GetStatus(),
		CategoryId:     p.GetCategoryId(),
		Origin:         p.GetOrigin(),
		Description:    p.GetDescription(),
		Gallery:        convertGallery(p.GetGallery()),
		Certifications: convertCertifications(p.GetCertifications()),
		PriceMin:       p.GetPriceMin(),
		PriceMax:       p.GetPriceMax(),
		Skus:           convertImportSkus(p.GetSkus()),
		CatL1Id:        p.GetCatL1Id(),
		CatL1Name:      p.GetCatL1Name(),
		CatL2Id:        p.GetCatL2Id(),
		CatL2Name:      p.GetCatL2Name(),
		CatL3Id:        p.GetCatL3Id(),
		CatL3Name:      p.GetCatL3Name(),
	}
}

func convertProductListPresentation(p *pb.ProductListPresentation) ProductListPresentationResponse {
	if p == nil {
		return ProductListPresentationResponse{}
	}
	return ProductListPresentationResponse{
		Id:              p.GetId(),
		Name:            p.GetName(),
		Rating:          p.GetRating(),
		CategoryId:      p.GetCategoryId(),
		CategoryName:    p.GetCategoryName(),
		Origin:          p.GetOrigin(),
		Gallery:         convertGallery(p.GetGallery()),
		PriceMin:        p.GetPriceMin(),
		PriceMax:        p.GetPriceMax(),
		ReviewCount:     p.GetReviewCount(),
		SkuCount:        p.GetSkuCount(),
		ImagePreviewUrl: p.GetImageUrl(),
	}
}

func convertPreviewListingRecords(list []*pb.ImportItemPreviewListingRecord) []ImportItemPreviewListingRecordResponse {
	result := make([]ImportItemPreviewListingRecordResponse, 0, len(list))
	for _, r := range list {
		if r == nil {
			continue
		}
		result = append(result, ImportItemPreviewListingRecordResponse{
			ImportItemId: r.GetImportItemId(),
			ImportStatus: r.GetImportStatus(),
			PreviewData:  convertProductListPresentation(r.GetPreviewData()),
		})
	}
	return result
}

func toPbCertifications(list []CertificationInputDTO) []*pb.CertificationDto {
	result := make([]*pb.CertificationDto, 0, len(list))
	for _, c := range list {
		result = append(result, &pb.CertificationDto{Name: c.Name, CertNo: c.CertNo})
	}
	return result
}

func toPbShippingMethods(list []ShippingMethodInputDTO) []*pb.ShippingMethodDto {
	result := make([]*pb.ShippingMethodDto, 0, len(list))
	for _, m := range list {
		result = append(result, &pb.ShippingMethodDto{Method: m.Method, Temperature: m.Temperature})
	}
	return result
}

func toPbGalleryImages(list []ImportItemImageInputDTO) []*pb.ImportItemImageInput {
	if len(list) == 0 {
		return nil
	}
	out := make([]*pb.ImportItemImageInput, 0, len(list))
	for _, img := range list {
		out = append(out, &pb.ImportItemImageInput{
			SourceUrl:   img.SourceUrl,
			StagedToken: img.StagedToken,
		})
	}
	return out
}

func toPbSkus(list []UpdateSkuInputDTO) []*pb.UpdateSkuInput {
	result := make([]*pb.UpdateSkuInput, 0, len(list))
	for _, s := range list {
		result = append(result, &pb.UpdateSkuInput{
			SkuId:           s.SkuId,
			SkuName:         s.SkuName,
			Price:           s.Price,
			SalePrice:       s.SalePrice,
			StockQty:        s.StockQty,
			Weight:          s.Weight,
			Unit:            s.Unit,
			LengthCm:        s.LengthCm,
			WidthCm:         s.WidthCm,
			HeightCm:        s.HeightCm,
			ShippingMethods: toPbShippingMethods(s.ShippingMethods),
			Delete:          s.Delete,
		})
	}
	return result
}

func toPbBatchShippingMethods(list []BatchShippingMethodInputDTO) []*pb.ShippingMethodInput {
	result := make([]*pb.ShippingMethodInput, 0, len(list))
	for _, m := range list {
		result = append(result, &pb.ShippingMethodInput{
			Type:          m.Type,
			Temperature:   m.Temperature,
			FreeThreshold: m.FreeThreshold,
		})
	}
	return result
}

// ==================== Handler（Job 級） ====================

// CreateImportJob 建立匯入任務
//
// @Summary      建立商品匯入任務
// @Description  賣家上傳 CSV 建立匯入任務，含 L1（檔案結構）+ L2（必填/enum/數字/連動）同步驗證。
// @Description  L1 失敗回 400 不建任務；L2 失敗回 200 + status=CSV_INVALID + validationErrors；
// @Description  L2 通過回 200 + status=PARSING。
// @Description  冪等規則：**同一份 CSV 內容**（SHA256(content) + store_id，24 小時內）會回既有 job 並帶 dedup_hit=true。
// @Description  冪等鍵由後端從內容算出，呼叫端不需要、也無法自帶——重試時送同樣的檔案即可。
// @Tags         商品匯入 (Product Import)
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        filename         formData  string  false  "原始檔名（不傳則用 multipart header 內 filename）"
// @Param        target_status    formData  string  false  "job 級預設上架狀態（ACTIVE / DRAFT / PENDING_REVIEW）"
// @Param        file             formData  file    true   "CSV 檔案 binary（上限見配置）"
// @Success      200  {object}  object{code=int,message=string,data=CreateImportJobResponseData}
// @Failure      400  {object}  object{code=int,message=string,validationErrors=[]ValidationErrorResponse}  "L1 檔案結構錯誤"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}  "PRODUCT_IMPORT_RATE_LIMITED 或同檔名鎖衝突"
// @Router       /app/product_import_jobs [post]
func (h *Holder) CreateImportJob(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		response.BadRequest(c, "failed to open uploaded file")
		return
	}
	defer func() { _ = src.Close() }()
	content, err := io.ReadAll(src)
	if err != nil {
		response.BadRequest(c, "failed to read uploaded file")
		return
	}

	filename := c.PostForm("filename")
	if filename == "" {
		filename = fileHeader.Filename
	}
	targetStatus := c.PostForm("target_status")

	resp, err := h.ProductImport.CreateImportJob(c.Request.Context(), &pb.CreateImportJobRequest{
		StoreId:      int64(storeID),
		Filename:     filename,
		Content:      content,
		TargetStatus: targetStatus,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, CreateImportJobResponseData{
		ImportJobId:      resp.GetImportJobId(),
		Status:           resp.GetStatus(),
		DedupHit:         resp.GetDedupHit(),
		ValidationErrors: convertValidationErrors(resp.GetValidationErrors()),
	})
}

// ReuploadImportJobCsv 重傳 CSV
//
// @Summary      重傳 CSV（CSV_INVALID 狀態才可）
// @Description  任務狀態為 CSV_INVALID 時重傳 CSV，重跑 L1+L2 驗證。SHA256 dedup check 跳過。
// @Tags         商品匯入 (Product Import)
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path      string  true  "匯入任務 ID"
// @Param        filename       formData  string  false "原始檔名"
// @Param        file           formData  file    true  "新 CSV 檔案"
// @Success      200  {object}  object{code=int,message=string,data=ReuploadImportJobCsvResponseData}
// @Failure      400  {object}  object{code=int,message=string}  "L1 檔案結構錯誤"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}  "job 非 CSV_INVALID 狀態"
// @Router       /app/product_import_jobs/{import_job_id}/csv [put]
func (h *Holder) ReuploadImportJobCsv(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		response.BadRequest(c, "failed to open uploaded file")
		return
	}
	defer func() { _ = src.Close() }()
	content, err := io.ReadAll(src)
	if err != nil {
		response.BadRequest(c, "failed to read uploaded file")
		return
	}

	filename := c.PostForm("filename")
	if filename == "" {
		filename = fileHeader.Filename
	}

	resp, err := h.ProductImport.ReuploadImportJobCsv(c.Request.Context(), &pb.ReuploadImportJobCsvRequest{
		StoreId:     int64(storeID),
		ImportJobId: importJobID,
		Filename:    filename,
		Content:     content,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, ReuploadImportJobCsvResponseData{
		ImportJobId:      resp.GetImportJobId(),
		Status:           resp.GetStatus(),
		ValidationErrors: convertValidationErrors(resp.GetValidationErrors()),
	})
}

// ListImportJobs 查詢匯入任務列表
//
// @Summary      查詢匯入任務列表
// @Description  賣家查詢自己的匯入任務列表，支援分頁與狀態多值篩選。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status     query     []string  false  "ImportJobStatus value（可帶多次）"  collectionFormat(multi)
// @Param        page       query     string    false  "頁碼（預設 1）"
// @Param        page_size  query     string    false  "每頁筆數（預設 10）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]ImportJobBriefResponse,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs [get]
func (h *Holder) ListImportJobs(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", defaultPageSizeStr)
	status := c.QueryArray("status")

	resp, err := h.ProductImport.ListImportJobs(c.Request.Context(), &pb.ListImportJobsRequest{
		StoreId:  int64(storeID),
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, response.PageData{
		Records:  convertImportJobBriefs(resp.GetRecords()),
		Total:    resp.GetTotal(),
		Page:     resp.GetPage(),
		PageSize: resp.GetPageSize(),
	})
}

// GetImportJob 取得任務詳情
//
// @Summary      取得匯入任務詳情
// @Description  回傳含 counters、progress；CSV_INVALID 時含 validationErrors。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string  true  "匯入任務 ID"
// @Success      200  {object}  object{code=int,message=string,data=ImportJobDetailResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id} [get]
func (h *Holder) GetImportJob(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	resp, err := h.ProductImport.GetImportJob(c.Request.Context(), &pb.GetImportJobRequest{
		StoreId:     int64(storeID),
		ImportJobId: importJobID,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, convertImportJobDetail(resp.GetData()))
}

// GetImportJobPreviewListing 商城模擬列表
//
// @Summary      商城模擬列表（買家視角預覽 job 內商品）
// @Description  以買家視角預覽此匯入任務的商品列表，固定按 first_csv_line ASC，scope by job。
// @Description  預設只顯示 status IN (READY, COMMIT_FAILED) AND blocking_error_count=0 的 item；
// @Description  include_pending=true 才加入 ASSET_DOWNLOADING 的 item。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id    path   string  true   "匯入任務 ID"
// @Param        include_pending  query  string  false  "true 才顯示 ASSET_DOWNLOADING 的 item（預設 false）"
// @Param        page             query  string  false  "頁碼（預設 1）"
// @Param        page_size        query  string  false  "每頁筆數（預設 20）"
// @Success      200  {object}  object{code=int,message=string,data=ImportJobPreviewListingResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/preview_listing [get]
func (h *Holder) GetImportJobPreviewListing(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", "20")
	includePending, _ := strconv.ParseBool(c.DefaultQuery("include_pending", "false"))

	resp, err := h.ProductImport.GetImportJobPreviewListing(c.Request.Context(), &pb.GetImportJobPreviewListingRequest{
		StoreId:        int64(storeID),
		ImportJobId:    importJobID,
		Page:           page,
		PageSize:       pageSize,
		IncludePending: includePending,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}

	data := resp.GetData()
	out := ImportJobPreviewListingResponse{
		ImportJobId:   data.GetImportJobId(),
		TotalEligible: data.GetTotalEligible(),
		TotalNeedsFix: data.GetTotalNeedsFix(),
		TotalPending:  data.GetTotalPending(),
		TotalRemoved:  data.GetTotalRemoved(),
		Records:       convertPreviewListingRecords(data.GetRecords()),
		Total:         data.GetTotal(),
		Page:          data.GetPage(),
		PageSize:      data.GetPageSize(),
	}
	response.GrpcResult(c, resp.Code, resp.Message, out)
}

// CancelImportJob 取消任務
//
// @Summary      取消匯入任務
// @Description  賣家主動取消任務（CREATED / PARSING / READY 狀態才可），轉為 CANCELLED 終態。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string  true  "匯入任務 ID"
// @Success      200  {object}  object{code=int,message=string,data=CancelImportJobResponseData}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}  "job 狀態不支援取消"
// @Router       /app/product_import_jobs/{import_job_id}/cancel [post]
func (h *Holder) CancelImportJob(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	resp, err := h.ProductImport.CancelImportJob(c.Request.Context(), &pb.CancelImportJobRequest{
		StoreId:     int64(storeID),
		ImportJobId: importJobID,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, CancelImportJobResponseData{Status: resp.GetStatus()})
}

// CommitImportJob 確認提交
//
// @Summary      確認提交（排入 commit queue）
// @Description  將 READY / COMMIT_FAILED 的 item 排入 commit queue，逐筆建立正式 product。
// @Description  retryFailedOnly=true 只重試 COMMIT_FAILED；includeItemIds 為空 = 全部 eligible。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string                   true  "匯入任務 ID"
// @Param        request        body  CommitImportJobRequest   true  "提交內容"
// @Success      200  {object}  object{code=int,message=string,data=CommitImportJobResponseData}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}  "job 狀態不支援 commit"
// @Router       /app/product_import_jobs/{import_job_id}/commit [post]
func (h *Holder) CommitImportJob(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	var req CommitImportJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 空 body 允許（全部 eligible 預設）
		req = CommitImportJobRequest{}
	}

	resp, err := h.ProductImport.CommitImportJob(c.Request.Context(), &pb.CommitImportJobRequest{
		StoreId:         int64(storeID),
		ImportJobId:     importJobID,
		RetryFailedOnly: req.RetryFailedOnly,
		IncludeItemIds:  req.IncludeItemIds,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	skipReasons := make([]CommitSkipReasonResponse, 0, len(resp.GetSkipReasons()))
	for _, r := range resp.GetSkipReasons() {
		if r == nil {
			continue
		}
		skipReasons = append(skipReasons, CommitSkipReasonResponse{
			ImportItemId:  r.GetImportItemId(),
			ReasonCode:    r.GetReasonCode(),
			ReasonMessage: r.GetReasonMessage(),
		})
	}
	response.GrpcResult(c, resp.Code, resp.Message, CommitImportJobResponseData{
		Status:         resp.GetStatus(),
		QueuedCount:    resp.GetQueuedCount(),
		CommitOutcome:  resp.GetCommitOutcome().String(),
		SkippedItemIds: resp.GetSkippedItemIds(),
		SkipReasons:    skipReasons,
	})
}

// ==================== Handler（Item 級） ====================

// ListImportItems 查詢任務內 items
//
// @Summary      查詢任務內 items
// @Description  支援 status 多值、only_with_errors、include_removed 篩選 + 分頁。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id     path   string    true   "匯入任務 ID"
// @Param        status            query  []string  false  "ImportItemStatus value（可帶多次）"  collectionFormat(multi)
// @Param        only_with_errors  query  string    false  "true 只回 blocking_error_count > 0 的 item"
// @Param        include_removed   query  string    false  "true 包含 REMOVED（預設 false）"
// @Param        page              query  string    false  "頁碼（預設 1）"
// @Param        page_size         query  string    false  "每頁筆數（預設 10）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]ImportItemBriefResponse,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items [get]
func (h *Holder) ListImportItems(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", defaultPageSizeStr)
	status := c.QueryArray("status")
	onlyWithErrors, _ := strconv.ParseBool(c.DefaultQuery("only_with_errors", "false"))
	includeRemoved, _ := strconv.ParseBool(c.DefaultQuery("include_removed", "false"))

	resp, err := h.ProductImport.ListImportItems(c.Request.Context(), &pb.ListImportItemsRequest{
		StoreId:        int64(storeID),
		ImportJobId:    importJobID,
		Status:         status,
		OnlyWithErrors: onlyWithErrors,
		IncludeRemoved: includeRemoved,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, response.PageData{
		Records:  convertImportItemBriefs(resp.GetRecords()),
		Total:    resp.GetTotal(),
		Page:     resp.GetPage(),
		PageSize: resp.GetPageSize(),
	})
}

// GetImportItem 取得單一 item 詳情
//
// @Summary      取得單一 item 詳情
// @Description  含 SKU、圖片、認證、錯誤/警告、target_status 等完整資料。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id   path  string  true  "匯入任務 ID"
// @Param        import_item_id  path  string  true  "Item ID"
// @Success      200  {object}  object{code=int,message=string,data=ImportItemDetailResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/{import_item_id} [get]
func (h *Holder) GetImportItem(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	importItemID := c.Param("import_item_id")
	if importJobID == "" || importItemID == "" {
		response.BadRequest(c, "import_job_id and import_item_id are required")
		return
	}

	resp, err := h.ProductImport.GetImportItem(c.Request.Context(), &pb.GetImportItemRequest{
		StoreId:      int64(storeID),
		ImportJobId:  importJobID,
		ImportItemId: importItemID,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, convertImportItemDetail(resp.GetData()))
}

// GetImportItemPreview 取得單一 item 商城模擬詳情
//
// @Summary      取得單一 item 商城模擬詳情
// @Description  以買家視角預覽單一 item。previewData 的非媒體欄位對齊正式 ProductDetail；
// @Description  媒體欄位不相容——只有 gallery[].url（presigned／原站 URL），無 imageKey／imageKeys／objectKey／assetId。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id   path  string  true  "匯入任務 ID"
// @Param        import_item_id  path  string  true  "Item ID"
// @Success      200  {object}  object{code=int,message=string,data=ImportItemPreviewResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/{import_item_id}/preview [get]
func (h *Holder) GetImportItemPreview(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	importItemID := c.Param("import_item_id")
	if importJobID == "" || importItemID == "" {
		response.BadRequest(c, "import_job_id and import_item_id are required")
		return
	}

	resp, err := h.ProductImport.GetImportItemPreview(c.Request.Context(), &pb.GetImportItemPreviewRequest{
		StoreId:      int64(storeID),
		ImportJobId:  importJobID,
		ImportItemId: importItemID,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}

	data := resp.GetData()
	out := ImportItemPreviewResponse{
		ImportJobId:      data.GetImportJobId(),
		ImportItemId:     data.GetImportItemId(),
		ImportStatus:     data.GetImportStatus(),
		ValidationErrors: convertItemErrors(data.GetValidationErrors()),
		Warnings:         convertItemErrors(data.GetWarnings()),
		FormalProductId:  data.GetFormalProductId(),
		PreviewData:      convertProductDetailPresentation(data.GetPreviewData()),
	}
	response.GrpcResult(c, resp.Code, resp.Message, out)
}

// UpdateImportItem 編輯單一 item
//
// @Summary      編輯單一 item
// @Description  採 *_set 模式：set=true 才更新該欄位。expectedVersion 樂觀鎖必填。
// @Description  galleryUrls[0] 為封面；allowDuplicateName=true 允許與既有商品同名。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id   path  string                  true  "匯入任務 ID"
// @Param        import_item_id  path  string                  true  "Item ID"
// @Param        request         body  UpdateImportItemRequest true  "編輯內容（採 *_set 模式）"
// @Success      200  {object}  object{code=int,message=string,data=ImportItemDetailResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}  "樂觀鎖衝突或 job 狀態不支援"
// @Router       /app/product_import_jobs/{import_job_id}/items/{import_item_id} [put]
func (h *Holder) UpdateImportItem(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	importItemID := c.Param("import_item_id")
	if importJobID == "" || importItemID == "" {
		response.BadRequest(c, "import_job_id and import_item_id are required")
		return
	}

	var req UpdateImportItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.ProductImport.UpdateImportItem(c.Request.Context(), &pb.UpdateImportItemRequest{
		StoreId:               int64(storeID),
		ImportJobId:           importJobID,
		ImportItemId:          importItemID,
		ExpectedVersion:       req.ExpectedVersion,
		NameSet:               req.NameSet,
		Name:                  req.Name,
		CategoryIdSet:         req.CategoryIdSet,
		CategoryId:            req.CategoryId,
		OriginSet:             req.OriginSet,
		Origin:                req.Origin,
		TargetStatusSet:       req.TargetStatusSet,
		TargetStatus:          req.TargetStatus,
		DescriptionSet:        req.DescriptionSet,
		Description:           req.Description,
		HashtagsSet:           req.HashtagsSet,
		Hashtags:              req.Hashtags,
		CertificationsSet:     req.CertificationsSet,
		Certifications:        toPbCertifications(req.Certifications),
		GalleryImagesSet:      req.GalleryImagesSet,
		GalleryImages:         toPbGalleryImages(req.GalleryImages),
		SkusSet:               req.SkusSet,
		Skus:                  toPbSkus(req.Skus),
		AllowDuplicateNameSet: req.AllowDuplicateNameSet,
		AllowDuplicateName:    req.AllowDuplicateName,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, convertImportItemDetail(resp.GetData()))
}

// RemoveImportItems 批次移除 items（軟刪）
//
// @Summary      批次移除 items（軟刪）
// @Description  將指定 item 設為 REMOVED 狀態，不影響其他 item；commit 時跳過。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string                    true  "匯入任務 ID"
// @Param        request        body  RemoveImportItemsRequest  true  "要移除的 item id 列表"
// @Success      200  {object}  object{code=int,message=string,data=RemoveOrRestoreItemsResponseData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/remove [post]
func (h *Holder) RemoveImportItems(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	var req RemoveImportItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.ProductImport.RemoveImportItems(c.Request.Context(), &pb.RemoveImportItemsRequest{
		StoreId:       int64(storeID),
		ImportJobId:   importJobID,
		ImportItemIds: req.ImportItemIds,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, RemoveOrRestoreItemsResponseData{AffectedCount: resp.GetAffectedCount()})
}

// RestoreImportItems 批次還原 items
//
// @Summary      批次還原 items
// @Description  將 REMOVED 的 item 還原至原狀態（READY / COMMIT_FAILED）。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string                     true  "匯入任務 ID"
// @Param        request        body  RestoreImportItemsRequest  true  "要還原的 item id 列表"
// @Success      200  {object}  object{code=int,message=string,data=RemoveOrRestoreItemsResponseData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/restore [post]
func (h *Holder) RestoreImportItems(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	var req RestoreImportItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.ProductImport.RestoreImportItems(c.Request.Context(), &pb.RestoreImportItemsRequest{
		StoreId:       int64(storeID),
		ImportJobId:   importJobID,
		ImportItemIds: req.ImportItemIds,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, RemoveOrRestoreItemsResponseData{AffectedCount: resp.GetAffectedCount()})
}

// RetryImportItemAssets 重試 FAILED 圖片
//
// @Summary      重試 FAILED 圖片
// @Description  重試指定 item 的 FAILED 資產；assetIds 為空 = 重試所有 FAILED。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id   path  string                        true  "匯入任務 ID"
// @Param        import_item_id  path  string                        true  "Item ID（query 形式：由 body 內傳遞）"
// @Param        request         body  RetryImportItemAssetsRequest  true  "重試內容（assetIds 為空 = 全部 FAILED）"
// @Success      200  {object}  object{code=int,message=string,data=RetryImportItemAssetsResponseData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/retry_assets [post]
func (h *Holder) RetryImportItemAssets(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	var req struct {
		ImportItemId string   `json:"importItemId" binding:"required"`
		AssetIds     []string `json:"assetIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.ProductImport.RetryImportItemAssets(c.Request.Context(), &pb.RetryImportItemAssetsRequest{
		StoreId:      int64(storeID),
		ImportJobId:  importJobID,
		ImportItemId: req.ImportItemId,
		AssetIds:     req.AssetIds,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, RetryImportItemAssetsResponseData{QueuedCount: resp.GetQueuedCount()})
}

// ==================== Handler（批次操作） ====================

// BatchUpdateItemsShipping 批次設定運送
//
// @Summary      批次設定運送（覆寫所選 items 所有 SKU）
// @Description  套用 shipping × dimension/weight/temp 政策後覆寫；applyAll=true 表全 job eligible items。
// @Description  free_threshold 有值（含 "0"） → fee_payer=SELLER；空 → fee_payer=BUYER。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string                            true  "匯入任務 ID"
// @Param        request        body  BatchUpdateItemsShippingRequest   true  "運送方式覆寫內容"
// @Success      200  {object}  object{code=int,message=string,data=BatchUpdateItemsShippingResponseData}
// @Failure      400  {object}  object{code=int,message=string}  "shipping method 未開通 / 政策降級無可用 type"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/batch_update_shipping [post]
func (h *Holder) BatchUpdateItemsShipping(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	var req BatchUpdateItemsShippingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.ProductImport.BatchUpdateItemsShipping(c.Request.Context(), &pb.BatchUpdateItemsShippingRequest{
		StoreId:         int64(storeID),
		ImportJobId:     importJobID,
		ImportItemIds:   req.ImportItemIds,
		ApplyAll:        req.ApplyAll,
		ShippingMethods: toPbBatchShippingMethods(req.ShippingMethods),
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, BatchUpdateItemsShippingResponseData{
		AffectedItems: resp.GetAffectedItems(),
		AffectedSkus:  resp.GetAffectedSkus(),
	})
}

// BatchUpdateItemsTargetStatus 批次套用上架狀態
//
// @Summary      批次套用上架狀態
// @Description  套用 target_status（ACTIVE / DRAFT / PENDING_REVIEW，不接受 INACTIVE）。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string                                  true  "匯入任務 ID"
// @Param        request        body  BatchUpdateItemsTargetStatusRequest     true  "上架狀態套用內容"
// @Success      200  {object}  object{code=int,message=string,data=BatchUpdateItemsTargetStatusResponseData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/batch_update_target_status [post]
func (h *Holder) BatchUpdateItemsTargetStatus(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	var req BatchUpdateItemsTargetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.ProductImport.BatchUpdateItemsTargetStatus(c.Request.Context(), &pb.BatchUpdateItemsTargetStatusRequest{
		StoreId:       int64(storeID),
		ImportJobId:   importJobID,
		ImportItemIds: req.ImportItemIds,
		ApplyAll:      req.ApplyAll,
		TargetStatus:  req.TargetStatus,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, BatchUpdateItemsTargetStatusResponseData{
		AffectedItems: resp.GetAffectedItems(),
	})
}

// BatchUpdateItemsCategory 批次套用商品類別
//
// @Summary      批次套用商品類別（含二次確認語意）
// @Description  必須帶 confirmationToken（先呼叫 GET .../category_confirmation_tokens 取得），
// @Description  伺服端 GETDEL Redis key 單次使用；category_id 必須是葉節點。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path  string                            true  "匯入任務 ID"
// @Param        request        body  BatchUpdateItemsCategoryRequest   true  "分類套用內容（含 confirmationToken）"
// @Success      200  {object}  object{code=int,message=string,data=BatchUpdateItemsCategoryResponseData}
// @Failure      400  {object}  object{code=int,message=string}  "PRODUCT_IMPORT_CATEGORY_INVALID / TOKEN_INVALID"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/product_import_jobs/{import_job_id}/items/batch_update_category [post]
func (h *Holder) BatchUpdateItemsCategory(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}

	var req BatchUpdateItemsCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.ProductImport.BatchUpdateItemsCategory(c.Request.Context(), &pb.BatchUpdateItemsCategoryRequest{
		StoreId:           int64(storeID),
		ImportJobId:       importJobID,
		ImportItemIds:     req.ImportItemIds,
		ApplyAll:          req.ApplyAll,
		CategoryId:        req.CategoryId,
		ConfirmationToken: req.ConfirmationToken,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, BatchUpdateItemsCategoryResponseData{
		AffectedItems: resp.GetAffectedItems(),
		CatL1Id:       resp.GetCatL1Id(),
		CatL1Name:     resp.GetCatL1Name(),
		CatL2Id:       resp.GetCatL2Id(),
		CatL2Name:     resp.GetCatL2Name(),
		CatL3Id:       resp.GetCatL3Id(),
		CatL3Name:     resp.GetCatL3Name(),
	})
}

// GetCategoryConfirmationToken 發給前端「分類二次確認 token」
//
// @Summary      發給前端分類二次確認 token
// @Description  前端打開分類選擇 modal、渲染完路徑後呼叫；提交 BatchUpdateItemsCategory 時必須帶回。
// @Description  Redis 落 token，TTL 5 分鐘、單次使用。
// @Tags         商品匯入 (Product Import)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        import_job_id  path   string  true   "匯入任務 ID"
// @Param        category_id    query  string  true   "葉節點分類 id"
// @Success      200  {object}  object{code=int,message=string,data=CategoryConfirmationTokenResponse}
// @Failure      400  {object}  object{code=int,message=string}  "PRODUCT_IMPORT_CATEGORY_INVALID"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "PRODUCT_IMPORT_JOB_NOT_FOUND / WRONG_STATUS"
// @Router       /app/product_import_jobs/{import_job_id}/category_confirmation_tokens [get]
func (h *Holder) GetCategoryConfirmationToken(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}
	importJobID := c.Param("import_job_id")
	if importJobID == "" {
		response.BadRequest(c, "import_job_id is required")
		return
	}
	categoryID := c.Query("category_id")
	if categoryID == "" {
		response.BadRequest(c, "category_id is required")
		return
	}

	resp, err := h.ProductImport.GetCategoryConfirmationToken(c.Request.Context(), &pb.GetCategoryConfirmationTokenRequest{
		StoreId:     int64(storeID),
		ImportJobId: importJobID,
		CategoryId:  categoryID,
	})
	if err != nil {
		response.GrpcErr(c, "product-import", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, CategoryConfirmationTokenResponse{
		ConfirmationToken: resp.GetConfirmationToken(),
		ExpiresAt:         resp.GetExpiresAt(),
		CatL1Id:           resp.GetCatL1Id(),
		CatL1Name:         resp.GetCatL1Name(),
		CatL2Id:           resp.GetCatL2Id(),
		CatL2Name:         resp.GetCatL2Name(),
		CatL3Id:           resp.GetCatL3Id(),
		CatL3Name:         resp.GetCatL3Name(),
	})
}

// 保留 swagger 文件 anchor，避免被 staticcheck 標未使用
var _ = http.StatusOK
