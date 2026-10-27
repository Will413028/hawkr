package buyer

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// ProductInfo 是商品顯示所需的資料。
type ProductInfo struct {
	ID        int64
	StoreID   int64
	Name      string
	ImageKey  string
	MinPrice  int64
	MaxPrice  int64
	Rating    *float32
	Available bool
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
	Cards(context.Context, []int64, bool) (map[int64]ProductInfo, error)
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
