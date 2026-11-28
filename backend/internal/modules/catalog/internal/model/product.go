package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
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

// MaxListPageSize 是列表單頁筆數的上限。
const MaxListPageSize int32 = 100

// 寫入與讀取商品的錯誤；用 errors.Is 判斷。
var (
	// ErrProductNotFound 表示商品不存在、已刪除，或不屬於這家店。
	ErrProductNotFound = errors.New("catalog: product not found")
	// ErrActiveRequiresGalleryImage 表示 ACTIVE 的商品必須至少有一張主圖：切成 ACTIVE 或寫入 ACTIVE 商品的內容時主圖是空的。
	ErrActiveRequiresGalleryImage = errors.New("catalog: an active product needs at least one gallery image")
	// ErrInvalidInput 表示輸入不合法；具體的欄位與原因在 *InvalidInputError。
	ErrInvalidInput = errors.New("catalog: invalid input")
	// ErrVersionConflict 表示呼叫端依據的版本（etag）已經過時：與目前的不符，沒有寫入。
	ErrVersionConflict = errors.New("catalog: product version conflict")
	ErrCreateReplayed  = errors.New("catalog: request_id already used for a product of this store")
	ErrNotPublished    = errors.New("catalog: a draft product cannot be unpublished by delegation")
)

// DefaultLowStockThreshold 是商品沒有自己的低庫存門檻時用的平台預設值；MaxLowStockThreshold 是商品門檻的上限（products_low_stock_check）。
const (
	DefaultLowStockThreshold int32 = 5
	MaxLowStockThreshold     int32 = 9999
)

// Etag 是商品版本對外的 etag（client 視為不透明字串）；版本只在賣家可編輯的內容與狀態改變時加一。
func Etag(version int64) string { return strconv.FormatInt(version, 10) }

// ParseEtag 把 etag 還原成版本；空字串、不是正整數都回 ErrInvalidInput。
func ParseEtag(etag string) (int64, error) {
	v, err := strconv.ParseInt(etag, 10, 64)
	if err != nil || v < 1 || Etag(v) != etag {
		return 0, Invalid("Etag", ReasonInvalid)
	}
	return v, nil
}

// ProductSort 是商品列表與搜尋的排序。
type ProductSort string

// 排序：LATEST 是首次上架時間遞減，PRICE_ASC 依最低價，PRICE_DESC 依最高價，SALES 銷量，RATING 評分，RELEVANCE 相關度（要有關鍵字）。
const (
	SortLatest    ProductSort = "LATEST"
	SortPriceAsc  ProductSort = "PRICE_ASC"
	SortPriceDesc ProductSort = "PRICE_DESC"
	SortSales     ProductSort = "SALES"
	SortRating    ProductSort = "RATING"
	SortRelevance ProductSort = "RELEVANCE"
)

// StockStatus 是賣家清單與計數依庫存的篩選；空字串是不篩選。兩個值都只比對 ACTIVE 的商品。
type StockStatus string

// 庫存篩選：LOW 是至少一個 SKU 庫存大於 0 且小於或等於商品的門檻；OUT_OF_STOCK 是至少一個 SKU 庫存是 0。
const (
	StockLow        StockStatus = "LOW"
	StockOutOfStock StockStatus = "OUT_OF_STOCK"
)

// PublicFilter 是公開列表與搜尋的篩選，全部選填、之間是 AND；空的欄位不篩選。
// CategoryIDs 可給頂層（含底下的葉節點）或葉節點；Hashtags 是已正規化的名稱，HashtagAll 為真時要全部都有。
type PublicFilter struct {
	CategoryIDs []uuid.UUID
	PriceMin    *int64
	PriceMax    *int64
	Hashtags    []string
	HashtagAll  bool
	StoreIDs    []int64
	// Query 是關鍵字（去頭尾空白後空的是不篩選）。
	Query string
}

// SellerFilter 是賣家清單與計數的篩選；空值不篩選。
type SellerFilter struct {
	Status      ProductStatus
	StockStatus StockStatus
	Query       string
}

// AdminFilter 是後台列表的篩選；空值不篩選。
type AdminFilter struct {
	StoreID *int64
	Query   string
}

// StockMismatchError 表示設定庫存時的預期值與現值不符，沒有寫入；Current 是呼叫當下看到的現值。
type StockMismatchError struct {
	SkuID   uuid.UUID
	Current int32
}

func (e *StockMismatchError) Error() string {
	return fmt.Sprintf("catalog: stock of sku %s is %d, not the expected quantity", e.SkuID, e.Current)
}

// SuggestedSafetyStock 是依最近銷售建議的安全庫存：weeklySales 是最近 SalesWindowWeeks 週每週的銷量（舊到新，可以較短，缺的週算 0）；
// 較長的輸入只取最後 SalesWindowWeeks 週（尾端窗口，與除數一致）。備貨兩週的平均需求，下限 5；沒有銷量時是 10。規則沿用賣家中心庫存頁（website/features/seller-center/inventory/InventoryPage.tsx）。
func SuggestedSafetyStock(weeklySales []int32) int32 {
	weeklySales = weeklySales[max(0, len(weeklySales)-SalesWindowWeeks):]
	var sum int64
	for _, w := range weeklySales {
		sum += int64(w)
	}
	if sum <= 0 {
		return SafetyStockNoSales
	}
	n := (sum*SafeCoverWeeks + SalesWindowWeeks - 1) / SalesWindowWeeks
	return int32(min(max(n, int64(SafetyStockFloor)), int64(math.MaxInt32)))
}

// 建議安全庫存的參數。
const (
	SalesWindowWeeks         = 8
	SafeCoverWeeks           = 2
	SafetyStockFloor   int32 = 5
	SafetyStockNoSales int32 = 10
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

// SkuState 是店家自己的一個 SKU 此刻的內容與可購買性（ACTIVE、SKU 與商品都沒刪除、庫存大於 0），已刪除的 SKU 也讀得到：
// 庫存調整已經 commit 之後並行刪除，或重送時 SKU 已被刪除，回應仍要照實回報已生效的結果。
type SkuState struct {
	Sku      Sku
	Sellable bool
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
	// LowStockThreshold 是低庫存提醒門檻（0 到 MaxLowStockThreshold）；nil 是用平台預設值。
	LowStockThreshold *int32
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
	// LowStockThreshold 是賣家設的門檻，nil 是用平台預設值。
	LowStockThreshold *int32
	// Version 是樂觀鎖的版本（對外是 Etag(Version)）：只在賣家可編輯的內容與狀態改變時加一，庫存與計數不改它。
	Version int64
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
	// TotalStock 是沒刪除 SKU 的庫存總和（上限 int32）。
	TotalStock        int32
	LowStockThreshold *int32
	Version           int64
}

// ListCursor 是 keyset 分頁的游標：Key 是排序鍵（賣家清單與後台是建立時間的微秒數，公開列表依 ProductSort 決定），同鍵依 ID 遞減。
// 游標的編碼與跟請求參數的綁定在對外層。
type ListCursor struct {
	Key int64
	ID  uuid.UUID
}

// ProductPage 是列表的一頁；Next 為 nil 表示沒有下一頁。
type ProductPage struct {
	Items []ProductSummary
	Next  *ListCursor
}

// HashtagCount 是一個商品標籤與它在範圍內的可見商品數；Name 是正規化名稱，Display 是最常用的輸入樣子。
type HashtagCount struct {
	Name         string
	Display      string
	ProductCount int64
}
