package coupon

import "github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"

// VoucherExpiring 在券進入到期提醒窗時與提醒紀錄同一個 transaction 內發布，每張券至多一次，VoucherID 是冪等鍵；由 notification 模組訂閱（A8-3）。
type VoucherExpiring = model.VoucherExpiring

// ExpiringHandler 是 VoucherExpiring 必須有的 handler 名稱：訂閱者以 events.SubscribeNamed 登記，沒有時到期提醒失敗而不是默默略過。
const ExpiringHandler = model.ExpiringHandler
