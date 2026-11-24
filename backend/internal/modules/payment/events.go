package payment

import "github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"

// PayoutAccountChanged 在同步 job 寫入收款帳戶快照的 transaction 內發布（每次同步都發布）：seller 以同 transaction 的訂閱者更新開通閘門讀的
// transfers_active 副本，並把 BankFingerprints 寫成防詐比對 hash（收款開通 ADR D12、D28）。BankFingerprints 是明文，訂閱者不保存、不寫 log。
type PayoutAccountChanged = model.PayoutAccountChanged

// PaymentChanged 在付款的快照寫入、狀態與上次不同的 transaction 內發布（同一個狀態的重複讀取與重複事件不重發）：訂單一方據此得知付款成功或取消，
// 並由它判斷訂單已不可付時發起自動退款（計畫 D5 修訂）。
type PaymentChanged = model.PaymentChanged

// MoneyAnomaly 在搬錢的物件（付款、轉帳、退款）進入 ANOMALY 的 transaction 內發布（也包含 provider 已搬錢、本地列卻不在可接收狀態的孤兒結果）：告警與後台待辦據此通知，
// 人工核對 provider 後結案（ADR D68）。
type MoneyAnomaly = model.MoneyAnomaly

// RefundChanged 在退款進入終局（SUCCEEDED、FAILED）的 transaction 內發布：訂單與售後一方據此得知退款結果。
type RefundChanged = model.RefundChanged

// AutoRefundCompleted 在自動退款成功的 transaction 內發布（計畫 D5），供通知模組與後台使用。
type AutoRefundCompleted = model.AutoRefundCompleted

// PayoutDeadlineApproaching 由 payment.deadline_scan 在待補條件的最早期限進入 7 天窗口後發布（每次掃描對每個帳戶一次；期限與檔位相同的重複發布由消費者去重，D23），供通知模組提醒賣家補件。
type PayoutDeadlineApproaching = model.PayoutDeadlineApproaching

// DeadlineTier 是 PayoutDeadlineApproaching 的檔位。
type DeadlineTier = model.DeadlineTier

const (
	// DeadlineSevenDays 是期限前超過一天的檔位。
	DeadlineSevenDays = model.DeadlineSevenDays
	// DeadlineOneDay 是期限前最後一天的檔位。
	DeadlineOneDay = model.DeadlineOneDay
)
