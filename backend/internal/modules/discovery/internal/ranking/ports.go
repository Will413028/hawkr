package ranking

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/discovery/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 以下是 discovery 擁有的 port（消費端定義形狀）。上游搬進單體之前由 cmd/hawkr 以 internal/legacy 的 client 實作，
// ordering、review、catalog 搬進來後換成它們的公開 API。任何一個 port 失敗都回 error：重算整次失敗、保留上一份快照，
// 不把失敗當成空資料。

// Orders 是訂單統計的 port。
type Orders interface {
	// StoreOrderStats 回傳近 days 天每家有成交或賣家取消的店的統計；沒有出現的店代表兩者都是 0。
	StoreOrderStats(ctx context.Context, days int) ([]model.OrderStats, error)
}

// Products 是商品的 port。
type Products interface {
	// StoresWithPublicProducts 回傳至少有一個公開上架商品的店家 ID 全集。失敗必須回 error，不可降級成空集合
	// （空集合代表全平台沒有公開商品）。
	StoresWithPublicProducts(ctx context.Context) ([]int64, error)
}

// MaxRatingBatch 是 Reviews.StoreRatingStats 一次最多接受的店家數（review-service ListStoreRatingStats 的上限）；
// 呼叫端（重算）自己分批。
const MaxRatingBatch = 100

// Reviews 是評價統計的 port。
type Reviews interface {
	// StoreRatingStats 批次回傳 storeIDs（最多 MaxRatingBatch 個）的評價數與平均評分。回傳只含有評價的店，沒有出現的店
	// 沒有評價；錯誤不推定為沒有評價，回傳了沒有詢問的店家是違反契約。
	StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]model.RatingStats, error)
}

// Stores 是店家資格的 port，由 seller 模組實作（*seller.Module 直接滿足）。
type Stores interface {
	// ExposableStores 從 ids（最多 seller.MaxBatchSize 個）挑出可以公開曝光的店家，附公開時間（PublishedAt，seller 的 first_exposed_at；還沒記下的是讀取當下）。
	ExposableStores(ctx context.Context, ids []int64) ([]seller.ExposableStore, error)
}

// OrderStats 與 RatingStats 是 port 的回傳型別。
type (
	OrderStats  = model.OrderStats
	RatingStats = model.RatingStats
)
