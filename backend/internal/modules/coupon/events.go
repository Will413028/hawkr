package coupon

import "github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"

// VoucherExpiring 在券進入到期提醒窗時於同一個 transaction 內發布，每張券至多一次；由 notification 模組訂閱（A8-3）。
type VoucherExpiring = model.VoucherExpiring
