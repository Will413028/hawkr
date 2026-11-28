package buyer

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// ProductInfo 是商品顯示所需的資料。
type ProductInfo struct {
	ID        uuid.UUID
	StoreID   int64
	Name      string
	ImageKey  string
	MinPrice  int64
	MaxPrice  int64
	Rating    *float32
	Available bool
	Visible   bool
}

// MerchantSummary 是單一店家的累計訂單金額。
type MerchantSummary struct {
	StoreID  int64
	Spent    int64
	Refunded int64
}

// SpendingRecord 是單筆訂單的顯示資料。
type SpendingRecord struct {
	OrderNo       string
	OrderTime     time.Time
	Amount        int64
	Refunded      int64
	CouponApplied bool
}

// SpendingPage 是舊訂單讀取端的一頁資料。
type SpendingPage struct {
	Records []SpendingRecord
	Total   int64
}

// Products 以一個批次取得商品顯示資料；缺少的 ID 表示已刪除。
type Products interface {
	Cards(context.Context, []uuid.UUID, bool) (map[uuid.UUID]ProductInfo, error)
}

// RestockSku 是補貨訂閱要確認的規格（SKU）現況：它所屬的商品，與現在有沒有貨（可購買：商品上架、沒刪除、庫存大於零）。
type RestockSku struct {
	ProductID uuid.UUID
	InStock   bool
}

// Skus 讀補貨訂閱用的規格現況；規格不存在、已刪除或商品還是草稿時 found 為假（catalog 的商品與規格在 buyer 之上，由 cmd/hawkr 組裝）。
type Skus interface {
	RestockSku(ctx context.Context, skuID uuid.UUID) (sku RestockSku, found bool, err error)
}

// Stores 是 seller 模組給買家端的店家讀取（*seller.Module 直接滿足），ADR 2026-10-03-public-store-identity-and-visibility：
// StoreCards 回傳曾經公開過的店家的卡片，從沒公開過與不存在的不在回傳裡；PublicStores 只回可以公開曝光的店家（追蹤用，
// 帶擁有者帳號供自己的店的檢查）。
type Stores interface {
	StoreCards(ctx context.Context, storeIDs []int64) ([]seller.StoreCard, error)
	PublicStores(ctx context.Context, storeIDs []int64) ([]seller.Store, error)
}

// Orders 提供買家在店家的累計消費及消費紀錄。
type Orders interface {
	MerchantSummary(context.Context, int64) ([]MerchantSummary, error)
	SpendingRecords(context.Context, int64, int64, int, int) (SpendingPage, error)
}

// CRM 提供買家在店家的 VIP 狀態。
type CRM interface {
	VIPStatuses(context.Context, int64, []int64) (map[int64]bool, error)
}
