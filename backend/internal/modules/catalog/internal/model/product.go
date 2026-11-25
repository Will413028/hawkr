package model

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

// ProductStatus 是商品的狀態。空字串與未知值都不是有效狀態。
type ProductStatus string

// 商品狀態。
const (
	StatusDraft    ProductStatus = "DRAFT"
	StatusActive   ProductStatus = "ACTIVE"
	StatusArchived ProductStatus = "ARCHIVED"
)

// Valid 回報 s 是否是三個狀態之一。
func (s ProductStatus) Valid() bool {
	return s == StatusDraft || s == StatusActive || s == StatusArchived
}

// CanTransition 回報賣家能否把狀態從 s 切到 to：草稿上架、上架後下架、下架可再上架。同狀態不是切換。
func (s ProductStatus) CanTransition(to ProductStatus) bool {
	switch s {
	case StatusDraft:
		return to == StatusActive
	case StatusActive:
		return to == StatusArchived
	case StatusArchived:
		return to == StatusActive
	}
	return false
}

// 輸入與結構上限。圖片與區塊的上限沒有明定數字：表格與清單照 seller-app 編輯器的常數（50 列、10 欄、每格 200 字），其餘為待確認的值，
// 集中在這裡以便調整。
const (
	MaxTitleRunes              = 100
	MaxAxes                    = 2
	MaxAxisNameRunes           = 20
	MaxOptionValueRunes        = 40
	MaxValuesPerAxis           = 50
	MaxSkus                    = 100
	MaxGalleryImages           = 9
	MaxDescriptionImages       = 20
	MaxDescriptionBlocks       = 100
	MaxParagraphRunes          = 5000
	MaxListItems               = 50
	MaxListItemRunes           = 200
	MaxTableRows               = 50
	MaxTableCols               = 10
	MaxTableCellRunes          = 200
	MaxAltRunes                = 200
	MaxListPageSize      int32 = 100
)

// 寫入與讀取商品的錯誤；用 errors.Is 判斷。
var (
	// ErrProductNotFound 表示商品不存在、已刪除，或不屬於這家店。
	ErrProductNotFound = errors.New("catalog: product not found")
	// ErrInvalidTransition 表示商品狀態不能這樣切換。
	ErrInvalidTransition = errors.New("catalog: invalid status transition")
	// ErrInvalidInput 表示輸入不合法；具體的欄位與原因在 *InvalidInputError。
	ErrInvalidInput = errors.New("catalog: invalid input")
)

// 輸入不合法的原因。
const (
	ReasonRequired   = "REQUIRED"
	ReasonTooLong    = "TOO_LONG"
	ReasonTooMany    = "TOO_MANY"
	ReasonDuplicate  = "DUPLICATE"
	ReasonUnknown    = "UNKNOWN"
	ReasonOutOfRange = "OUT_OF_RANGE"
	ReasonInvalid    = "INVALID"
)

// InvalidInputError 指出哪個欄位為什麼不合法。Field 是輸入欄位的路徑，例如 "Skus[2].Price"；Reason 是 Reason* 常數之一；
// 若是 hashtag 違規，Cause 是 platform/hashtag 的 sentinel 錯誤。使用者面文字由呼叫端（對外層的 handler）決定。
type InvalidInputError struct {
	Field  string
	Reason string
	Cause  error
}

func (e *InvalidInputError) Error() string {
	return fmt.Sprintf("catalog: invalid input: %s: %s", e.Field, e.Reason)
}

// Is 讓 errors.Is(err, ErrInvalidInput) 成立。
func (e *InvalidInputError) Is(target error) bool { return target == ErrInvalidInput }

// Unwrap 回傳 Cause。
func (e *InvalidInputError) Unwrap() error { return e.Cause }

// Invalid 建立 *InvalidInputError。
func Invalid(field, reason string) *InvalidInputError {
	return &InvalidInputError{Field: field, Reason: reason}
}

// Media 是 catalog 對 media 的依賴：圖片在寫入的 transaction 內 Confirm，被換掉或移除的資產 MarkDeleted。兩個方法的 ctx 都必須帶 transaction。
type Media interface {
	Confirm(ctx context.Context, uploader media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error)
	MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error
}

// Image 是一張圖片的引用。輸入時給 Token（新上傳的 staged token）或 AssetID（商品現有的圖）其中一個；讀取時 AssetID 與 ObjectKey 有值。
type Image struct {
	Token     string
	AssetID   uuid.UUID
	ObjectKey string
	Alt       string
}

// BlockKind 是描述區塊的種類。
type BlockKind string

// 描述區塊的種類。
const (
	BlockParagraph BlockKind = "paragraph"
	BlockImage     BlockKind = "image"
	BlockList      BlockKind = "list"
	BlockTable     BlockKind = "table"
)

// Block 是描述的一個區塊；依 Kind 只有對應的欄位有值：段落 Text（純文字）、圖片 Image、項目清單 Items、表格 Rows（列的儲存格）。
type Block struct {
	Kind  BlockKind
	Text  string
	Image *Image
	Items []string
	Rows  [][]string
}

// Axis 是一個選項軸：名稱與有序的值。
type Axis struct {
	Name   string
	Values []string
}

// SkuInput 是一個 SKU 的輸入：以選項值的組合辨識（沒有軸的商品兩個都空）。組合已存在時更新價格，庫存只在新組合時作為初始值（庫存只由 stock 改）。
type SkuInput struct {
	Option1, Option2 string
	Price            int64
	CompareAtPrice   *int64
	StockQty         int32
}

// Sku 是商品的一個 SKU。Option1、Option2 空字串表示沒有該軸。
type Sku struct {
	ID               uuid.UUID
	Option1, Option2 string
	Price            int64
	CompareAtPrice   *int64
	StockQty         int32
}

// Hashtag 是商品的一個標籤。
type Hashtag struct{ Name, Display string }

// ProductInput 是建立與修改商品的完整內容（修改是整體取代；狀態不在這裡，用 SetStatus）。
type ProductInput struct {
	Title       string
	CategoryID  uuid.UUID
	Description []Block
	Gallery     []Image
	Axes        []Axis
	Skus        []SkuInput
	Hashtags    []string
}

// Product 是商品的完整內容。
type Product struct {
	ID               uuid.UUID
	StoreID          int64
	CategoryID       uuid.UUID
	Title            string
	Status           ProductStatus
	FirstPublishedAt *time.Time
	MinPrice         int64
	MaxPrice         int64
	SalesCount       int32
	FavoriteCount    int32
	RatingAverage    float64
	ReviewCount      int32
	Description      []Block
	Gallery          []Image
	Axes             []Axis
	Skus             []Sku
	Hashtags         []Hashtag
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ProductSummary 是列表的一列。CoverKey 是主圖庫第一張的 object key，沒有圖是空字串。
type ProductSummary struct {
	ID               uuid.UUID
	StoreID          int64
	CategoryID       uuid.UUID
	Title            string
	Status           ProductStatus
	FirstPublishedAt *time.Time
	MinPrice         int64
	MaxPrice         int64
	SalesCount       int32
	FavoriteCount    int32
	RatingAverage    float64
	ReviewCount      int32
	CoverKey         string
	CreatedAt        time.Time
}

// ListCursor 是 keyset 分頁的游標：賣家清單的 At 是建立時間，公開列表的 At 是首次上架時間；同時間依 ID 遞減。
type ListCursor struct {
	At time.Time
	ID uuid.UUID
}

// ProductPage 是列表的一頁；Next 為 nil 表示沒有下一頁。
type ProductPage struct {
	Items []ProductSummary
	Next  *ListCursor
}
