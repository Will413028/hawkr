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

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/counter"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/product"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/restock"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/stock"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/taxonomy"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
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
	// HashtagCount 是一個商品標籤與它的可見商品數。
	HashtagCount = model.HashtagCount
	// ProductSort 是商品列表與搜尋的排序。
	ProductSort = model.ProductSort
	// StockStatus 是賣家清單與計數依庫存的篩選。
	StockStatus = model.StockStatus
	// PublicFilter 是公開列表與搜尋的篩選。
	PublicFilter = model.PublicFilter
	// SellerFilter 是賣家清單與計數的篩選。
	SellerFilter = model.SellerFilter
	// AdminFilter 是後台列表的篩選。
	AdminFilter = model.AdminFilter
	// StockMismatchError 表示設定庫存時的預期值與現值不符。
	StockMismatchError = model.StockMismatchError
	// Media 是 catalog 對 media 的依賴（*media.Module 與 *media.Fake 都滿足）。
	Media = model.Media
	// SkuState 是店家自己的一個 SKU 此刻的內容與可購買性（含已刪除的）。
	SkuState = model.SkuState
	// SalesItem 是一筆訂單裡一個商品的銷量。
	SalesItem = model.SalesItem
	// Rating 是商品目前的評分平均與評價則數，加上所屬店家 ID（LockProduct 回傳）。
	Rating = model.Rating
	// RestockedSKU 是 SKU 由不可買變可買時，通知用的最新快照。
	RestockedSKU = model.RestockedSKU
	// SkuSnapshot 是一個 SKU 與它的商品此刻的內容（結帳快照與購物車資訊共用）。
	SkuSnapshot = model.SkuSnapshot
	// ProductWritten 是商品寫入事件共用的欄位（見 internal/model/events.go）。
	ProductWritten = model.ProductWritten
	// ProductCreated、ProductUpdated、ProductPublished、ProductUnpublished、ProductPublishRepeated、ProductUnpublishRepeated 是單一商品的寫入事件，
	// ProductsUnpublished 是一次批次下架的事件（UnpublishedItem 是其中一項）；在商品寫入的 transaction 內發布，Actor 非 nil 時是代行寫入（A7-D27）。
	ProductCreated           = model.ProductCreated
	ProductUpdated           = model.ProductUpdated
	ProductPublished         = model.ProductPublished
	ProductUnpublished       = model.ProductUnpublished
	ProductPublishRepeated   = model.ProductPublishRepeated
	ProductUnpublishRepeated = model.ProductUnpublishRepeated
	ProductsUnpublished      = model.ProductsUnpublished
	UnpublishedItem          = model.UnpublishedItem
	// VersionedProduct 是批次操作的一項：商品與讀取時拿到的版本。
	VersionedProduct = model.VersionedProduct
	// BatchVersionConflictError 是批次下架有版本衝突時的錯誤，列出全部衝突的商品；errors.Is(err, ErrVersionConflict) 成立。
	BatchVersionConflictError = model.BatchVersionConflictError
	// BatchNotPublishedError 是代行批次下架含草稿時的錯誤，列出全部草稿商品；errors.Is(err, ErrNotPublished) 成立。
	BatchNotPublishedError = model.BatchNotPublishedError
	// RestockNotifier 是 catalog 宣告、補貨通知下游（buyer，B4-4 接線）實作的 port。
	RestockNotifier = model.RestockNotifier
)

// 商品狀態與描述區塊的種類。
const (
	// MaxBatchIDs 是批次讀取一次的 ID 數上限。
	MaxBatchIDs = model.MaxBatchIDs

	StatusDraft    = model.StatusDraft
	StatusActive   = model.StatusActive
	StatusArchived = model.StatusArchived

	SortLatest    = model.SortLatest
	SortPriceAsc  = model.SortPriceAsc
	SortPriceDesc = model.SortPriceDesc
	SortSales     = model.SortSales
	SortRating    = model.SortRating
	SortRelevance = model.SortRelevance

	StockLow        = model.StockLow
	StockOutOfStock = model.StockOutOfStock

	// DefaultLowStockThreshold 是商品沒有自己的低庫存門檻時的平台預設值；MaxLowStockThreshold 是商品門檻的上限。
	DefaultLowStockThreshold = model.DefaultLowStockThreshold
	MaxLowStockThreshold     = model.MaxLowStockThreshold

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
	// ErrVersionConflict 表示依據的版本（etag）已過時，沒有寫入。
	ErrVersionConflict = model.ErrVersionConflict
	// ErrCreateReplayed 見 model.ErrCreateReplayed：代行建立的 request_id 在這家店已用過。
	ErrCreateReplayed = model.ErrCreateReplayed
	// ErrNotPublished 見 model.ErrNotPublished：代行不能下架草稿。
	ErrNotPublished = model.ErrNotPublished
)

// InvalidInputError.Reason 的值：欄位為什麼不合法。
const (
	ReasonRequired   = model.ReasonRequired
	ReasonTooLong    = model.ReasonTooLong
	ReasonTooMany    = model.ReasonTooMany
	ReasonDuplicate  = model.ReasonDuplicate
	ReasonUnknown    = model.ReasonUnknown
	ReasonOutOfRange = model.ReasonOutOfRange
	ReasonInvalid    = model.ReasonInvalid
)

// Etag 是商品版本對外的 etag；ParseEtag 還原它（不合法回 ErrInvalidInput）。etag 只涵蓋賣家可編輯的內容與狀態：庫存、銷量、收藏數、評分不改它。
func Etag(version int64) string { return model.Etag(version) }

// ParseEtag 見 Etag。
func ParseEtag(etag string) (int64, error) { return model.ParseEtag(etag) }

// SuggestedSafetyStock 是依最近 8 週每週銷量（舊到新）建議的安全庫存：備貨兩週的平均需求、下限 5，沒有銷量是 10。
func SuggestedSafetyStock(weeklySales []int32) int32 { return model.SuggestedSafetyStock(weeklySales) }

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB *db.DB
	// NewID 產生實體的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now func() time.Time
	// Media 是 media 模組；新圖的 Confirm 與退場資產的 MarkDeleted 在商品寫入的 transaction 內呼叫。
	Media Media
	// Events 是寫入事件的 bus（ProductCreated 等在商品寫入的 transaction 內發布）；必要的，沒給就 panic。
	Events *events.Bus
	// Restock 是補貨通知的下游：catalog 的週期 worker 在 transaction 外呼叫它（B4-4 接 buyer，之前由組裝點給替身）。
	Restock RestockNotifier
}

// Module 是 catalog 模組。
type Module struct {
	stock    *stock.Service
	taxonomy *taxonomy.Service
	product  *product.Service
	counter  *counter.Service
	restock  *restock.Service
}

// New 建立模組；Restock 是必要的（缺了，補貨與清訂閱的 worker 會在執行時 nil 解參考），沒給就 panic，和 coupon.New 一樣在組裝時失敗。
func New(d Deps) *Module {
	if d.Restock == nil {
		panic("catalog：Restock 是必要的")
	}
	if d.Events == nil {
		panic("catalog：Events 是必要的")
	}
	rs := restock.New(restock.Deps{DB: d.DB, Now: d.Now, Notifier: d.Restock})
	return &Module{
		stock:    stock.New(stock.Deps{DB: d.DB, NewID: d.NewID, Now: d.Now, Restock: rs}),
		taxonomy: taxonomy.New(taxonomy.Deps{DB: d.DB}),
		product:  product.New(product.Deps{DB: d.DB, NewID: d.NewID, Now: d.Now, Media: d.Media, Events: d.Events, Restock: rs}),
		counter:  counter.New(counter.Deps{DB: d.DB, Now: d.Now}),
		restock:  rs,
	}
}

// AddWorkers 登記補貨佇列週期 job 的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) { restock.AddWorkers(workers, m.restock) }

// PeriodicJobs 回傳補貨佇列的排程（每 restock.Interval 處理一輪到期的補貨與清訂閱列）；組裝點在 B4-6 接線。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return restock.PeriodicJobs() }

// LockProduct 在呼叫方的 transaction 內鎖住商品列（FOR NO KEY UPDATE，不看 deleted_at）並回目前的評分與則數，讓之後的 SetRating 與其他寫入者序列化；
// review 的評分 worker 先呼叫它、再讀自己的統計（鎖順序 catalog → review，D3），並以回傳值略過沒有變的寫入。
// ctx 沒有 transaction 是呼叫方的程式錯誤，回一般 error（與 identity.LockAccount 同）；商品不存在回 ErrProductNotFound。
func (m *Module) LockProduct(ctx context.Context, productID uuid.UUID) (Rating, error) {
	return m.counter.LockProduct(ctx, productID)
}

// SetRating 把商品的評分平均（0 到 5）與評價則數設成絕對值，由 review 的評分 worker 在鎖住商品列（LockProduct）的 transaction 內呼叫（D3）：沒有版本欄，並行的呼叫靠商品列的行鎖序列化，後提交的絕對值勝出；
// 不改 etag。count 為 0 時 average 必為 0；不合法回 *InvalidInputError，商品不存在回 ErrProductNotFound（已刪除的商品仍更新）。
func (m *Module) SetRating(ctx context.Context, productID uuid.UUID, average float64, count int32) error {
	return m.counter.SetRating(ctx, productID, average, count)
}

// RecordSales 把一筆訂單的銷量累計到各商品（同一個商品多筆先加總），同一個 orderNo 只累計一次，重送回 nil 且不再累計。
// 必要條件：Java order 以 MQ 送出事件，會重送，直到 ordering 搬進單體（B8）；到時 sales_orders 去重表退場。與呼叫方的 transaction 同進退（ctx 沒有時自己開一個），任何商品不存在整筆回滾、回 ErrProductNotFound。
// 在呼叫方的 transaction 內回 error（含 ErrProductNotFound）時，呼叫方必須回滾那個 transaction：這裡不開 savepoint，已寫入的訂單登記仍在 transaction 裡，繼續 commit 會留下登記了但沒累計的訂單，之後的重送被去重擋下。未知商品是否當毒訊息丟棄由 B4-4 的消費端決定。
func (m *Module) RecordSales(ctx context.Context, orderNo string, items []SalesItem) error {
	return m.counter.RecordSales(ctx, orderNo, items)
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
// SKU 不屬於該店家或已刪除回 ErrSkuNotFound；調整後不得小於 0（回 *InsufficientStockError）。同一個 key（賣家的 request_id）重送且內容相同不再調整，同時在途也只生效一次，內容不同回 ErrIdempotencyKeyReused。
// 不改商品的 etag。transaction 規則同 DeductStock。
func (m *Module) AdjustStock(ctx context.Context, storeID int64, key string, skuID uuid.UUID, delta int32) error {
	return m.stock.Adjust(ctx, storeID, key, skuID, delta)
}

// SetStock 是賣家把自己店家某個 SKU 的庫存設成絕對值 quantity，expected 是呼叫者看到的現值：現值不等於 expected 回 *StockMismatchError（帶現值），不寫入。
// 成功時帳本記一列 SET 操作與差額 movement；重送與錯誤語意同 AdjustStock。不改商品的 etag。
func (m *Module) SetStock(ctx context.Context, storeID int64, key string, skuID uuid.UUID, quantity, expected int32) error {
	return m.stock.SetQuantity(ctx, storeID, key, skuID, quantity, expected)
}

// ProductCards 批次讀商品卡片（沒刪除的商品，缺席的 ID 表示不存在或已刪除）：includeInactive 為假時只回 ACTIVE 且店家在 at 可曝光的（公開可見），為真時任何狀態都回（直接連結仍可瀏覽）。
// ids 最多 MaxBatchIDs 個，超過或含零值回 *InvalidInputError。
func (m *Module) ProductCards(ctx context.Context, at time.Time, ids []uuid.UUID, includeInactive bool) (map[uuid.UUID]ProductSummary, error) {
	return m.product.Cards(ctx, at, ids, includeInactive)
}

// ProductStoreIDs 批次讀商品所屬店家：沒刪除的商品，任何狀態（含草稿與下架）、不看店家曝光；缺席的 ID 表示不存在或已刪除。輸入限制同 ProductCards。
func (m *Module) ProductStoreIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	return m.product.StoreIDs(ctx, ids)
}

// StoresWithPublicProducts 回至少有一件公開商品（ACTIVE、沒刪除）且店家在 at 可曝光的店家 ID，升序。
func (m *Module) StoresWithPublicProducts(ctx context.Context, at time.Time) ([]int64, error) {
	return m.product.StoresWithPublicProducts(ctx, at)
}

// CheckoutSnapshot 是結帳用的 SKU 快照（D20，不版本化：訂單自己保存當時讀到的內容）：不過濾狀態與刪除，已下架與已刪除的 SKU 也回，
// 由 Active 判斷可不可以買、StockQty 看庫存；缺席的 SKU 表示不存在。配送方式、重量與尺寸不在 catalog（D10）。輸入限制同 ProductCards。
func (m *Module) CheckoutSnapshot(ctx context.Context, skuIDs []uuid.UUID) (map[uuid.UUID]SkuSnapshot, error) {
	return m.product.SkuSnapshots(ctx, skuIDs)
}

// SkuCartInfo 是購物車列表與加入購物車用的 SKU 即時資訊，內容與判斷同 CheckoutSnapshot（兩個呼叫端各自的入口，同一句查詢）。
func (m *Module) SkuCartInfo(ctx context.Context, skuIDs []uuid.UUID) (map[uuid.UUID]SkuSnapshot, error) {
	return m.product.SkuSnapshots(ctx, skuIDs)
}

// SkuPurchasable 回報 SKU 此刻是否可買（D26）：有庫存、SKU 與商品未刪、商品 ACTIVE、店家可曝光（與補貨通知同一個判斷）；SKU 不存在回 false。
func (m *Module) SkuPurchasable(ctx context.Context, skuID uuid.UUID) (bool, error) {
	return m.restock.Purchasable(ctx, skuID)
}

// AddFavorite 把商品的收藏數加一，由 buyer 在收藏列同一個 transaction 內呼叫（D3）：與呼叫方的 transaction 同進退（ctx 沒有時單句寫入）。商品不存在或已刪除回 ErrProductNotFound。
func (m *Module) AddFavorite(ctx context.Context, productID uuid.UUID) error {
	return m.product.AddFavorite(ctx, productID)
}

// RemoveFavorite 把商品的收藏數減一，下限 0，transaction 規則同 AddFavorite；已刪除的商品照減，商品不存在回 ErrProductNotFound。
func (m *Module) RemoveFavorite(ctx context.Context, productID uuid.UUID) error {
	return m.product.RemoveFavorite(ctx, productID)
}

// Categories 回傳整棵平台分類樹，頂層在前，同層依 position。
func (m *Module) Categories(ctx context.Context) ([]Category, error) { return m.taxonomy.List(ctx) }

// IsLeafCategory 回報分類是否存在且可掛商品（葉節點）。
func (m *Module) IsLeafCategory(ctx context.Context, id uuid.UUID) (bool, error) {
	return m.taxonomy.RequireLeaf(ctx, id)
}

// CreateProduct 建立一個草稿商品。actor 是代行者（明確的參數，A7-D27；店主自己的寫入傳 nil）：成功的寫入在同一個 transaction 內發 ProductCreated 並帶著它，代行的稽核訂閱者同 transaction 寫入稽核；重送（沒有寫入）不發事件。storeID 是呼叫端從登入身分推導的店家（不信任 client 送的店家）；uploader 是新圖 staged token 的上傳者；
// requestID 是冪等鍵（範圍是店家，永久保存）：同一家店以同一個鍵重送回第一次建立的商品目前的樣子，不重複建立（同時在途也只建立一個），內容不同回 ErrIdempotencyKeyReused。
// 圖片的 Confirm 在建立的 transaction 內，失敗整筆回滾。輸入不合法回 *InvalidInputError。
func (m *Module) CreateProduct(ctx context.Context, storeID int64, uploader media.Uploader, requestID uuid.UUID, in ProductInput, actor *delegatedactor.Actor) (Product, error) {
	return m.product.Create(ctx, storeID, uploader, requestID, in, actor)
}

// UpdateProduct 整體取代商品的內容（actor 同 CreateProduct，發 ProductUpdated），狀態不變（ACTIVE 的商品輸入沒有主圖回 ErrActiveRequiresGalleryImage）；SKU 以選項值的組合辨識（組合還在就更新價格，庫存不動）。
// version 是讀取時拿到的版本（Product.Version，對外是 Etag），必填；與目前的不符回 ErrVersionConflict、不寫入。商品不是這家店的或已刪除回 ErrProductNotFound。
func (m *Module) UpdateProduct(ctx context.Context, storeID int64, uploader media.Uploader, productID uuid.UUID, version int64, in ProductInput, actor *delegatedactor.Actor) (Product, error) {
	return m.product.Update(ctx, storeID, uploader, productID, version, in, actor)
}

// SetProductStatus 切換商品狀態（actor 同 CreateProduct；事件依目標狀態是 ProductPublished 或 ProductUnpublished，已經在目標狀態時是 *Repeated，目標 DRAFT 沒有事件也不接受 actor）：DRAFT、ACTIVE、ARCHIVED 任兩個都能互切，切成目前狀態是 no-op（不改版本）。切成 ACTIVE 時商品沒有主圖回 ErrActiveRequiresGalleryImage；
// first_published_at 只在第一次 ACTIVE 時填，之後不清除。version 必填，不符回 ErrVersionConflict。代行者不能下架草稿（回 ErrNotPublished）。回傳寫入後（no-op 時是目前）的商品，在寫入的 transaction 內讀出。
func (m *Module) SetProductStatus(ctx context.Context, storeID int64, productID uuid.UUID, version int64, to ProductStatus, actor *delegatedactor.Actor) (Product, error) {
	return m.product.SetStatus(ctx, storeID, productID, version, to, actor)
}

// UnpublishProducts 原子地下架多個商品（D29）：一個 transaction、一個事件，全部下架或全部不動；任一項版本不符整批回 *BatchVersionConflictError（列出全部衝突的商品），
// 已經下架的項目視為成功（兩個版本都是目前版本）。商品不是這家店的回 ErrProductNotFound。actor 同 SetProductStatus；代行者的批次裡有草稿，整批回 *BatchNotPublishedError（ErrNotPublished）。
// 回傳每一項寫入後的結果（依商品 ID 由小到大），與事件同一份，呼叫方據它組回應、不必再讀商品。
func (m *Module) UnpublishProducts(ctx context.Context, storeID int64, items []VersionedProduct, actor *delegatedactor.Actor) ([]UnpublishedItem, error) {
	return m.product.UnpublishMany(ctx, storeID, items, actor)
}

// DeleteProduct 刪除商品（設 deleted_at，不可還原，列保留）。version 選填：0 不比對，非 0 要與目前版本相符，否則回 ErrVersionConflict。
func (m *Module) DeleteProduct(ctx context.Context, storeID int64, productID uuid.UUID, version int64) error {
	return m.product.Delete(ctx, storeID, productID, version)
}

// Product 讀店家自己的商品（任何狀態，沒刪除）。
func (m *Module) Product(ctx context.Context, storeID int64, productID uuid.UUID) (Product, error) {
	return m.product.Get(ctx, storeID, productID)
}

// OwnSku 讀店家自己的 SKU 此刻的內容與可購買性，含已刪除的 SKU 與商品；SKU 不存在或不是這家店的回 ErrSkuNotFound。庫存調整成功後 Connect 層用它回該 SKU 調整後的內容：已生效的調整不會因為並行刪除或重送而回失敗。
func (m *Module) OwnSku(ctx context.Context, storeID int64, skuID uuid.UUID) (SkuState, error) {
	return m.product.OwnSku(ctx, storeID, skuID)
}

// PublicProduct 讀公開可見的商品：ACTIVE 或 ARCHIVED（下架的商品頁保留，Status 讓呼叫端決定可不可以買）、沒刪除、店家在 at 可曝光；草稿與其他情況 ErrProductNotFound。at 是請求的 clock。
func (m *Module) PublicProduct(ctx context.Context, productID uuid.UUID, at time.Time) (Product, error) {
	return m.product.GetPublic(ctx, productID, at)
}

// AdminProduct 讀任何店家的商品（任何狀態，沒刪除），後台唯讀用；不存在回 ErrProductNotFound。
func (m *Module) AdminProduct(ctx context.Context, productID uuid.UUID) (Product, error) {
	return m.product.GetAdmin(ctx, productID)
}

// ListSellerProducts 是賣家清單：店家自己的所有沒刪除的商品，不看曝光，最新建立的在前；可依狀態、庫存（只比對 ACTIVE）與關鍵字（比對 searchable_text）篩選。
func (m *Module) ListSellerProducts(ctx context.Context, storeID int64, f SellerFilter, after *ListCursor, size int32) (ProductPage, error) {
	return m.product.ListSeller(ctx, storeID, f, after, size)
}

// CountSellerProducts 回店家自己符合篩選的商品數（低庫存徽章用 Status ACTIVE、StockStatus StockLow）。低庫存以 SKU 判斷：沒刪除的 SKU 庫存大於 0 且小於或等於商品的門檻
// （沒設用 DefaultLowStockThreshold）；售完（有 SKU 庫存為 0）是另一個篩選。
func (m *Module) CountSellerProducts(ctx context.Context, storeID int64, f SellerFilter) (int32, error) {
	return m.product.CountSeller(ctx, storeID, f)
}

// ListAdminProducts 是後台列表：全站沒刪除的商品（任何狀態），最新建立的在前，回符合篩選的總數。
func (m *Module) ListAdminProducts(ctx context.Context, f AdminFilter, after *ListCursor, size int32) (ProductPage, int32, error) {
	return m.product.ListAdmin(ctx, f, after, size)
}

// ListPublicProducts 是公開列表與搜尋：只含 ACTIVE、沒刪除、店家在 at 可曝光（seller.pub_exposable_stores）的商品。sort 空字串是 SortLatest；SortRelevance 要有 f.Query。
// 游標是 (排序鍵, ID)，翻頁不重不漏。
func (m *Module) ListPublicProducts(ctx context.Context, at time.Time, f PublicFilter, sort ProductSort, after *ListCursor, size int32) (ProductPage, error) {
	return m.product.ListPublic(ctx, at, f, sort, after, size)
}

// ListPublicProductsOfStores 是指定店家的公開商品，最新上架在前（buyer 的「追蹤中」動態用）。storeIDs 為空回空頁（不是不篩選）。
func (m *Module) ListPublicProductsOfStores(ctx context.Context, at time.Time, storeIDs []int64, after *ListCursor, size int32) (ProductPage, error) {
	if len(storeIDs) == 0 {
		return ProductPage{Items: []ProductSummary{}}, nil
	}
	return m.product.ListPublic(ctx, at, PublicFilter{StoreIDs: storeIDs}, SortLatest, after, size)
}

// SuggestProductNames 是搜尋框的商品名稱建議（可見商品，相關度高的在前，最多 limit 筆：預設 10、上限 20）。
func (m *Module) SuggestProductNames(ctx context.Context, at time.Time, query string, limit int32) ([]string, error) {
	return m.product.SuggestNames(ctx, at, query, limit)
}

// SuggestHashtags 是標籤建議（可見商品數多的在前，預設 10、上限 20）。
func (m *Module) SuggestHashtags(ctx context.Context, at time.Time, query string, contains bool, limit int32) ([]HashtagCount, error) {
	return m.product.SuggestHashtags(ctx, at, query, contains, limit)
}

// TrendingHashtags 是熱門標籤（預設 20、上限 50）。
func (m *Module) TrendingHashtags(ctx context.Context, at time.Time, limit int32) ([]HashtagCount, error) {
	return m.product.TrendingHashtags(ctx, at, limit)
}
