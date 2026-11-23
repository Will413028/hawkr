package coupon

import "github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/qualify"

// 模組擁有的 port（形狀由 coupon 定義，見 internal/qualify）。資格判定讀不到資料時回錯誤，不降級（BD28）。

// Buyers 是買家註冊時間的 port：ALL 資格（活動建立之前註冊的買家）用。B7-3 由 identity 模組的 API 實作。
type Buyers = qualify.Buyers

// Members 是「買家在該店有消費紀錄」的 port：MEMBER 資格用。B7-4 由 coupon 持有、由訂單完成事件餵入的投影實作
// （spike 2 不成立時改同步 port，由 legacy/orderservice 實作）。
type Members = qualify.Members
