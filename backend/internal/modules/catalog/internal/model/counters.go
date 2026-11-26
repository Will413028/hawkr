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
