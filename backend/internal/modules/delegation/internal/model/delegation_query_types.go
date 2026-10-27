package model

import (
	"time"
)

// AuditLogFilter 是稽核清單的篩選：SessionID、AdminID、StoreID 為 nil 時不篩選，其他的零值不篩選。
type AuditLogFilter struct {
	SessionID    *int64
	AdminID      *int64
	StoreID      *int64
	Action       AuditAction
	ResourceType AuditResourceType
	Result       AuditResult
	From         *time.Time
	To           *time.Time
}

// AuditLogPage 是稽核清單的一頁，新到舊。
type AuditLogPage struct {
	Logs   []AuditLog
	Total  int64
	Paging DelegationPaging
}

func ValidateRange(from, to *time.Time) error {
	if from != nil && to != nil && from.After(*to) {
		return Deny(DenialInvalidCommand, "dateFrom 不得晚於 dateTo")
	}
	return nil
}

// DelegationSummary 是一次代行的結束摘要。
type DelegationSummary struct {
	// Session 帶撤銷者的顯示名。
	Session DelegationSession
	// Duration 是實際代行的時間：右端取結束、閒置逾時的最後活動、hard 到期與現在之中最早的一個。
	Duration time.Duration
	// OperationCount 是成功的操作數；RiskOperationCount 是其中高風險的；PendingAuditCount 是還沒確認的。
	OperationCount     int64
	RiskOperationCount int64
	PendingAuditCount  int64
	// NotifiedSeller 表示至少一列已經排入給賣家的通知（事實，不是意圖）。
	NotifiedSeller bool
	// FulfillmentActionCount、FinanceActionCount 結構上恆為 0；不是 0 表示三層封鎖被打穿（P0 事故）。
	FulfillmentActionCount int64
	FinanceActionCount     int64
	// AuditRetentionDays 是畫面「保留原則」一行顯示的天數。
	AuditRetentionDays int
}

// DelegationSessionFilter 是跨 session 治理清單的篩選：nil 或空字串不篩選。沒有狀態的篩選（expired 是讀取時推導的）。
type DelegationSessionFilter struct {
	AdminID     *int64
	StoreID     *int64
	DisplayCode string
	// From、To 比對 issued_at（對外顯示的是它）。
	From *time.Time
	To   *time.Time
}

// DelegationSessionPage 是治理清單的一頁，新到舊。
type DelegationSessionPage struct {
	Sessions []DelegationSession
	Total    int64
	Paging   DelegationPaging
}
