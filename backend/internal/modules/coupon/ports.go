package coupon

import (
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/admin"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/qualify"
)

// 模組擁有的 port（形狀由 coupon 定義，見 internal/qualify）。資格判定讀不到資料時回錯誤，不降級（BD28）。

// Stores 是「店家目前是否停權」的 port：賣家發行的活動在發行店家停權期間，領券與鎖券回 COUPON_STORE_SUSPENDED；
// 平台（admin）發行的活動不受影響，Redeem、Release 不查（D8 修訂）。組裝點由 seller 模組的 `Store(ctx, id)`（`Suspended()`）接上。
type Stores = qualify.Stores

// Buyers 是「這些買家存在」的 port：後台建立 DESIGNATED 活動與指定發放寫入前檢查名單。Existing 回傳 ids 之中存在的買家。組裝點由 buyer 模組的 `BatchMeta` 接上。
type Buyers = admin.Buyers

// StoreDirectory 是「店家存在、是否已關閉」的 port：後台建立限店活動寫入前檢查。組裝點由 seller 模組的 `Store(ctx, id)` 接上。
type StoreDirectory = admin.StoreDirectory
