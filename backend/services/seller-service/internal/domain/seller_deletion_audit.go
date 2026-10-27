package domain

import "time"

// SellerDeletionAudit 是 admin override 未結訂單檢查刪除賣家的 append-only 稽核紀錄。
type SellerDeletionAudit struct {
	ID               int64
	AccountID        int64
	OperatorID       int64
	ActiveOrderCount int
	Reason           string
	CreatedAt        time.Time
}
