// Package catalog 是 catalog 模組的公開 API：商品、規格、庫存與平台分類。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package catalog

import (
	"context"
	"embed"
	"io/fs"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/product"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/stock"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/taxonomy"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

type (
	// StockItem 是一個 SKU 的扣減或歸還量。
	StockItem = model.StockItem
	// Category 是平台分類樹的一個節點。
	Category = model.Category
	// InsufficientStockError 表示庫存不足，整次扣減沒有生效。
	InsufficientStockError = model.InsufficientStockError
	// InvalidInputError 指出商品輸入的哪個欄位為什麼不合法；errors.Is(err, ErrInvalidInput) 成立。
	InvalidInputError = model.InvalidInputError
	// ProductStatus 是商品的狀態：DRAFT、ACTIVE、ARCHIVED。
	ProductStatus = model.ProductStatus
	// ProductInput 是建立與修改商品的完整內容。
	ProductInput = model.ProductInput
	// Product 是商品的完整內容。
	Product = model.Product
	// ProductSummary 是列表的一列。
	ProductSummary = model.ProductSummary
	// ProductPage 是列表的一頁。
	ProductPage = model.ProductPage
	// ListCursor 是列表 keyset 分頁的游標。
	ListCursor = model.ListCursor
	// Image 是一張圖片的引用：新圖給 Token，現有的圖給 AssetID。
	Image = model.Image
	// Block 是描述的一個區塊（段落、圖片、清單、表格）。
	Block = model.Block
	// BlockKind 是描述區塊的種類。
	BlockKind = model.BlockKind
	// Axis 是一個選項軸。
	Axis = model.Axis
	// SkuInput 是一個 SKU 的輸入。
	SkuInput = model.SkuInput
	// Sku 是商品的一個 SKU。
	Sku = model.Sku
	// Hashtag 是商品的一個標籤。
	Hashtag = model.Hashtag
	// Media 是 catalog 對 media 的依賴（*media.Module 與 *media.Fake 都滿足）。
	Media = model.Media
)

// 商品狀態與描述區塊的種類。
const (
	StatusDraft    = model.StatusDraft
	StatusActive   = model.StatusActive
	StatusArchived = model.StatusArchived

	BlockParagraph = model.BlockParagraph
	BlockImage     = model.BlockImage
	BlockList      = model.BlockList
	BlockTable     = model.BlockTable
)

// 庫存操作的錯誤；用 errors.Is 判斷。
var (
	ErrInvalidStockInput       = model.ErrInvalidStockInput
	ErrSkuNotFound             = model.ErrSkuNotFound
	ErrIdempotencyKeyReused    = model.ErrIdempotencyKeyReused
	ErrDeductionNotFound       = model.ErrDeductionNotFound
	ErrRestoreExceedsDeduction = model.ErrRestoreExceedsDeduction
)

// 商品讀寫的錯誤；用 errors.Is 判斷。
var (
	ErrProductNotFound = model.ErrProductNotFound
	// ErrActiveRequiresGalleryImage 見 model.ErrActiveRequiresGalleryImage。
	ErrActiveRequiresGalleryImage = model.ErrActiveRequiresGalleryImage
	ErrInvalidInput               = model.ErrInvalidInput
)

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB *db.DB
	// NewID 產生實體的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now func() time.Time
	// Media 是 media 模組；新圖的 Confirm 與退場資產的 MarkDeleted 在商品寫入的 transaction 內呼叫。
	Media Media
}

// Module 是 catalog 模組。
type Module struct {
	stock    *stock.Service
	taxonomy *taxonomy.Service
	product  *product.Service
}

// New 建立模組。
func New(d Deps) *Module {
	return &Module{
		stock:    stock.New(stock.Deps{DB: d.DB, NewID: d.NewID, Now: d.Now}),
		taxonomy: taxonomy.New(taxonomy.Deps{DB: d.DB}),
		product:  product.New(product.Deps{DB: d.DB, NewID: d.NewID, Now: d.Now, Media: d.Media}),
	}
}

// DeductStock 以冪等鍵 key 扣減庫存，與呼叫方的 transaction 同進退（ctx 沒有 transaction 時自己開一個）。
// 同一個 key 重送且內容相同不再扣減；任何一個 SKU 庫存不足整次不生效，回 *InsufficientStockError。
// 在呼叫方的 transaction 內失敗時，呼叫方必須讓整個 transaction 回滾，不能吞掉錯誤繼續。
func (m *Module) DeductStock(ctx context.Context, key string, items []StockItem) error {
	return m.stock.Deduct(ctx, key, items)
}

// RestoreStock 依 deductKey 指的原扣減歸還庫存，同一個 key 重送不再歸還。items 為空表示歸還剩下的全部；有給時累計不得超過原扣減量
// （超過回 ErrRestoreExceedsDeduction）。transaction 規則同 DeductStock。
func (m *Module) RestoreStock(ctx context.Context, key, deductKey string, items []StockItem) error {
	return m.stock.Restore(ctx, key, deductKey, items)
}

// AdjustStock 是賣家以變動量 delta（有號、不為 0）調整自己店家某個 SKU 的庫存，記為 ADJUST 操作；storeID 由呼叫端推導。
// SKU 不屬於該店家或已刪除回 ErrSkuNotFound；調整後不得小於 0（回 *InsufficientStockError）。同一個 key 重送且內容相同不再調整。
// transaction 規則同 DeductStock。
func (m *Module) AdjustStock(ctx context.Context, storeID int64, key string, skuID uuid.UUID, delta int32) error {
	return m.stock.Adjust(ctx, storeID, key, skuID, delta)
}

// Categories 回傳整棵平台分類樹，頂層在前，同層依 position。
func (m *Module) Categories(ctx context.Context) ([]Category, error) { return m.taxonomy.List(ctx) }

// IsLeafCategory 回報分類是否存在且可掛商品（葉節點）。
func (m *Module) IsLeafCategory(ctx context.Context, id uuid.UUID) (bool, error) {
	return m.taxonomy.RequireLeaf(ctx, id)
}

// CreateProduct 建立一個草稿商品。storeID 是呼叫端從登入身分推導的店家（不信任 client 送的店家）；uploader 是新圖 staged token 的上傳者。
// 圖片的 Confirm 在建立的 transaction 內，失敗整筆回滾。輸入不合法回 *InvalidInputError。
func (m *Module) CreateProduct(ctx context.Context, storeID int64, uploader media.Uploader, in ProductInput) (Product, error) {
	return m.product.Create(ctx, storeID, uploader, in)
}

// UpdateProduct 整體取代商品的內容，狀態不變（ACTIVE 的商品輸入沒有主圖回 ErrActiveRequiresGalleryImage）；SKU 以選項值的組合辨識（組合還在就更新價格，庫存不動）。商品不是這家店的或已刪除回 ErrProductNotFound。
func (m *Module) UpdateProduct(ctx context.Context, storeID int64, uploader media.Uploader, productID uuid.UUID, in ProductInput) (Product, error) {
	return m.product.Update(ctx, storeID, uploader, productID, in)
}

// SetProductStatus 切換商品狀態：DRAFT、ACTIVE、ARCHIVED 任兩個都能互切，切成目前狀態是 no-op。切成 ACTIVE 時商品沒有主圖回 ErrActiveRequiresGalleryImage；
// first_published_at 只在第一次 ACTIVE 時填，之後不清除。
func (m *Module) SetProductStatus(ctx context.Context, storeID int64, productID uuid.UUID, to ProductStatus) error {
	return m.product.SetStatus(ctx, storeID, productID, to)
}

// DeleteProduct 刪除商品（設 deleted_at，不可還原，列保留）。
func (m *Module) DeleteProduct(ctx context.Context, storeID int64, productID uuid.UUID) error {
	return m.product.Delete(ctx, storeID, productID)
}

// Product 讀店家自己的商品（任何狀態，沒刪除）。
func (m *Module) Product(ctx context.Context, storeID int64, productID uuid.UUID) (Product, error) {
	return m.product.Get(ctx, storeID, productID)
}

// PublicProduct 讀公開可見的商品：ACTIVE、沒刪除、店家在 at 可曝光；否則 ErrProductNotFound。at 是請求的 clock。
func (m *Module) PublicProduct(ctx context.Context, productID uuid.UUID, at time.Time) (Product, error) {
	return m.product.GetPublic(ctx, productID, at)
}

// ListSellerProducts 是賣家清單：店家自己的所有沒刪除的商品，不看狀態也不看曝光，最新建立的在前。
func (m *Module) ListSellerProducts(ctx context.Context, storeID int64, after *ListCursor, size int32) (ProductPage, error) {
	return m.product.ListSeller(ctx, storeID, after, size)
}

// ListPublicProducts 是公開列表：只含 ACTIVE、沒刪除、店家在 at 可曝光（seller.pub_exposable_stores）的商品，最新上架的在前；categoryID 非 nil 時只列該分類。
func (m *Module) ListPublicProducts(ctx context.Context, at time.Time, categoryID *uuid.UUID, after *ListCursor, size int32) (ProductPage, error) {
	return m.product.ListPublic(ctx, at, categoryID, after, size)
}
