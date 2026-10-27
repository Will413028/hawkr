package model

import (
	"time"
)

// DelegationTarget 是代行的店家：建立 session 當下的快照。
type DelegationTarget struct {
	StoreID         int64
	SellerAccountID int64
	StoreName       string
}

// DelegationActor 是代行的管理員。DisplayName 是 email：admins 沒有名稱欄。
type DelegationActor struct {
	AdminID     int64
	DisplayName string
}

// DelegationSession 是一次代行。不含 credential、handoff code 或任何 token。
type DelegationSession struct {
	// ID 過渡期是數字（IDENTITY，同帳號；見 schema.sql）。
	ID            int64
	DisplayCode   string
	Actor         DelegationActor
	Target        DelegationTarget
	Reason        DelegationReason
	ReasonLabel   string
	CaseReference *string
	Capabilities  []Capability
	IssuedAt      time.Time
	// ExpiresAt 是 hard 到期，建立後只在主動結束時縮短。
	ExpiresAt time.Time
	EndedAt   *time.Time
	// Status 是讀取當下推導的狀態。
	Status DelegationStatus
	// Revision 只在主動結束與撤銷時加一。
	Revision         int32
	RevokedAt        *time.Time
	RevokedByAdminID *int64
	// RevokedByDisplayName 是撤銷者的 email，只有清單與摘要會解析；其他地方是 nil。
	RevokedByDisplayName *string
	RevokeReason         *string
}

// DelegationPreflightInput 是 preflight 的輸入。
type DelegationPreflightInput struct {
	AdminID int64
	StoreID int64
}

// DelegationPreflight 是「這位管理員此刻能不能代行這家店」的答案。不允許時 ReasonOptions 與 Capabilities 仍然
// 非空（前端在看 Allowed 之前就檢查它們）：Capabilities 是沒有阻擋時會核發的集合，不授予任何東西。
type DelegationPreflight struct {
	Allowed bool
	// Denial 與 DeniedLabel 是不允許的原因與給操作者的那一句；Allowed 時是零值。
	Denial      DelegationDenial
	DeniedLabel string
	// Target 在還沒讀到店家時（管理員沒有開通、店家不存在）只有 StoreID，TargetKnown 是 false。
	Target                DelegationTarget
	TargetKnown           bool
	Actor                 DelegationActor
	ReasonOptions         []DelegationReasonOption
	Capabilities          []Capability
	TTL                   time.Duration
	IdleTimeout           time.Duration
	CaseReferenceRequired bool
}

// StartDelegationInput 是建立代行的輸入。
type StartDelegationInput struct {
	AdminID int64
	// AdminDisplayName 是 gateway 帶來的顯示名；空的時候用管理員的 email。
	AdminDisplayName string
	StoreID          int64
	Reason           DelegationReason
	CaseReference    *string
	Capabilities     []Capability
	TTLSeconds       int32
	// IdempotencyKey 以 (管理員, key) 去重：同一把 key 重放回同一個 session。
	IdempotencyKey string
	RequestID      string
	CorrelationID  string
	UserAgent      string
}

// StartedDelegation 是建立（或重放）的結果。HandoffCode 是一次性的兌換碼，重放時會換一個新的。
type StartedDelegation struct {
	HandoffCode string
	Session     DelegationSession
}

// RedeemedDelegation 是兌換 handoff code 的結果，也是 credential 明文的唯一出口。
type RedeemedDelegation struct {
	Credential string
	Session    DelegationSession
}

// AdminEligibilityChange 是管理員代行資格的開通或撤回。
type AdminEligibilityChange struct {
	AdminID         int64
	OperatorAdminID int64
	Enabled         bool
}

// RevokeDelegationInput 是治理的撤銷。
type RevokeDelegationInput struct {
	SessionID int64
	// AdminID 是執行撤銷的管理員，可以不是代行的人。
	AdminID int64
	// Reason 是 1–200 字的理由。
	Reason string
}

// RevokeStoreSessionsInput 是授權撤回時撤銷店家進行中代行的輸入。
type RevokeStoreSessionsInput struct {
	StoreID int64
	AdminID int64
	Now     time.Time
}

// AuditSessionInput 是稽核時讀取代行狀態的輸入。
type AuditSessionInput struct {
	ID     int64
	Origin AuditOrigin
	Now    time.Time
}
