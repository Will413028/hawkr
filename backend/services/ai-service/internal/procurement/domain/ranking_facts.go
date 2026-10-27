package domain

import "github.com/Will413028/hawkr/go-pkg/identity"

// SellerRankingFact is ai-service's local, eventually-consistent snapshot of a
// store's membership tier and this month's completed-order count — read by
// candidate_platform_lane.go, never queried cross-service on the hot path.
// A missing entry for a store (BatchGet's returned map has no key for it)
// means "no facts yet" — treat as tier 0, no exposure cap (fail-open).
type SellerRankingFact struct {
	StoreID                identity.StoreID
	TierRank               int
	MonthlyOrderCap        *int // nil = unknown / not yet set by a membership event — do not exclude
	MonthlyCompletedOrders int
	// CountPeriod 是 MonthlyCompletedOrders 所屬的月份（YYYY-MM），由
	// order.completed consumer 依事件的 completed_at 寫入。
	//
	// 讀端必須連同這個欄位一起判斷才能做曝光排除：寫端是「下一筆訂單進來時才
	// 順帶歸零」（UpsertSellerRankingOrderCount 的
	// `CASE WHEN count_period = $2 THEN +1 ELSE 1`），沒有排程器會在月初主動把
	// 每一列歸零。因此某店在 M 月撞到上限、M+1 月完全沒有成交時，這一列會原封不動
	// 停在 M 月的計數上——只看 MonthlyCompletedOrders 會把上個月的舊帳當成本月，
	// 讓它在 M+1 月（乃至往後每一個沒有成交的月份）持續被排除。
	// 期間不等於當月時，這個計數就不代表本月，不得用來排除。
	CountPeriod string
}
