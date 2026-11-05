package payment

import "github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"

// PayoutAccountChanged 在同步 job 寫入收款帳戶快照的 transaction 內發布（每次同步都發布）：seller 以同 transaction 的訂閱者更新開通閘門讀的
// transfers_active 副本，並把 BankFingerprints 寫成防詐比對 hash（Stripe 計畫 D12、D28）。BankFingerprints 是明文，訂閱者不保存、不寫 log。
type PayoutAccountChanged = model.PayoutAccountChanged
