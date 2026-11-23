package coupon

import "github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/qualify"

// 模組擁有的 port（形狀由 coupon 定義，見 internal/qualify）。資格判定讀不到資料時回錯誤，不降級（BD28）。

// Stores 是「店家目前是否停權」的 port：賣家發行的活動在發行店家停權期間，領券與鎖券回 COUPON_STORE_SUSPENDED；
// 平台（admin）發行的活動不受影響，Redeem、Release 不查（D8 修訂）。組裝點由 seller 模組的 `Store(ctx, id)`（`Suspended()`）接上。
type Stores = qualify.Stores

// Members 是「買家在該店有消費紀錄」的 port：MEMBER 資格用。B7-4 由 coupon 持有、由訂單完成事件餵入的投影實作
// （spike 2 不成立時改同步 port，由 legacy/orderservice 實作）。
type Members = qualify.Members
