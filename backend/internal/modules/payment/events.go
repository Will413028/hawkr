package payment

import "github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"

// PayoutAccountChanged 在同步 job 寫入收款帳戶快照的 transaction 內發布（每次同步都發布）：seller 以同 transaction 的訂閱者更新開通閘門讀的
// transfers_active 副本，並把 BankFingerprints 寫成防詐比對 hash（收款開通 ADR D12、D28）。BankFingerprints 是明文，訂閱者不保存、不寫 log。
type PayoutAccountChanged = model.PayoutAccountChanged

// PaymentChanged 在付款的快照寫入、狀態與上次不同的 transaction 內發布（同一個狀態的重複讀取與重複事件不重發）：訂單一方據此得知付款成功或取消，
// 並由它判斷訂單已不可付時發起自動退款（計畫 D5 修訂）。
type PaymentChanged = model.PaymentChanged
