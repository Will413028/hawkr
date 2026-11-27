package model

import (
	"context"
	"time"
	"uuid"
)

// SalesItem 是一筆訂單裡一個商品的銷量。同一個商品出現多次（不同 SKU）時累加。
type SalesItem struct {
	ProductID uuid.UUID
	Quantity  int32
}

// MaxSalesItems 是一次銷量累計的品項數上限（檢查輸入的合理範圍，不是業務限制）。
const MaxSalesItems = 1000

// RestockedSKU 是一個 SKU 由不可買變可買時，通知用的最新快照。Name 與 ImageKey 是 job 執行當下的商品名稱與主圖 key（沒有主圖為空）；
// OccurredAt 是 job 重新確認可買的時間。
type RestockedSKU struct {
	ProductID  uuid.UUID
	SKUID      uuid.UUID
	StoreID    int64
	Name       string
	ImageKey   string
	OccurredAt time.Time
}

// RestockNotifier 是 catalog 對補貨通知下游（buyer，B4-4 接線）的依賴。兩個方法都在 transaction 外由 job 呼叫，要能重複呼叫：
// 訂閱是一次性的，通知過就不再有，所以重送是 no-op。
type RestockNotifier interface {
	// NotifyRestocked 通知訂閱了這個 SKU 的買家。
	NotifyRestocked(ctx context.Context, sku RestockedSKU) error
	// PurgeSKUSubscriptions 清掉這些 SKU 的訂閱（商品或 SKU 刪除後）。
	PurgeSKUSubscriptions(ctx context.Context, skuIDs []uuid.UUID) error
}

// MaxBatchIDs 是一次批次讀取的 ID 數上限（檢查輸入的合理範圍，不是業務限制）；超過回 *InvalidInputError。
const MaxBatchIDs = 200

// SkuSnapshot 是一個 SKU 與它的商品此刻的內容（結帳與購物車用，D20：不版本化，訂單自己保存當時的快照）。
// 已刪除與下架的 SKU 也讀得到（訂單與購物車仍指向它），Active 是呼叫端判斷可不可以買的依據：商品 ACTIVE、商品與 SKU 都沒刪除；庫存另看 StockQty。
// 配送方式、重量與尺寸不在 catalog（D10），所以不在快照裡。
type SkuSnapshot struct {
	SkuID, ProductID uuid.UUID
	StoreID          int64
	ProductTitle     string
	// SkuName 是選項值以 " / " 串接（沒有選項軸的預設 SKU 為空字串）。
	SkuName string
	// CoverKey 是主圖庫第一張的 object key，沒有圖是空字串。
	CoverKey string
	// Price 是實際成交單價；CompareAtPrice 是原價（有特價才有值，大於 Price）。
	Price          int64
	CompareAtPrice *int64
	StockQty       int32
	Status         ProductStatus
	// Deleted 為真表示商品或 SKU 已刪除。
	Deleted bool
	Active  bool
}

// Rating 是商品目前的評分平均與評價則數，與商品所屬的店家（LockProduct 回傳；StoreID 供呼叫方排店面頁的快取失效）。
type Rating struct {
	StoreID int64
	Average float64
	Count   int32
}
