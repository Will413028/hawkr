package model

import (
	"time"
	"uuid"
)

// AuthorizationSource 是授權的依據（憑什麼授權，不是授權給誰）。ticket 指客服對話、通話紀錄或營運簽核這類可以回查
// 的內部紀錄（沒有工單系統）。值等於 DB 的 CHECK。
type AuthorizationSource string

const (
	AuthorizationContract       AuthorizationSource = "contract"
	AuthorizationTicket         AuthorizationSource = "ticket"
	AuthorizationPlatformPolicy AuthorizationSource = "platform_policy"
)

func (s AuthorizationSource) Valid() bool {
	return s == AuthorizationContract || s == AuthorizationTicket || s == AuthorizationPlatformPolicy
}

// AuthorizationAction 是治理歷史的動作。GRANT 與 REINSTATE 是同一個動作的兩個分支，由 DB 實際走到哪一邊決定，
// 不是呼叫方指定的。值等於 DB 的 CHECK。
type AuthorizationAction string

const (
	AuthorizationGrant     AuthorizationAction = "GRANT"
	AuthorizationReinstate AuthorizationAction = "REINSTATE"
	AuthorizationUpdate    AuthorizationAction = "UPDATE"
	AuthorizationRevoke    AuthorizationAction = "REVOKE"
)

// Label 是動作的顯示名（原本在 CodeMeta 的 DelegationAuthorizationAction）；不認得的回傳原值。
func (a AuthorizationAction) Label() string {
	switch a {
	case AuthorizationGrant:
		return "開通授權"
	case AuthorizationReinstate:
		return "重新開通授權"
	case AuthorizationUpdate:
		return "調整授權範圍"
	case AuthorizationRevoke:
		return "撤回授權"
	}
	return string(a)
}

// AuthorizationStatusFilter 是治理總覽的篩選：全部、有效、無效三個值，是列的完整二分（{有效、撤回、到期} 那種分類
// 會漏掉生效時間在未來的列）。
type AuthorizationStatusFilter string

const (
	AuthorizationAny         AuthorizationStatusFilter = ""
	AuthorizationEffective   AuthorizationStatusFilter = "effective"
	AuthorizationIneffective AuthorizationStatusFilter = "ineffective"
)

// GrantOutcome 是開通實際走到的分支。
type GrantOutcome string

const (
	GrantCreated    GrantOutcome = "created"
	GrantReinstated GrantOutcome = "reinstated"
)

// DelegationAuthorization 是一家店的代行授權。管理員的顯示名（email）在讀取時解析：它回答「現在是誰負責」，要跟著
// 改名；治理歷史反過來存寫入當下的快照。
type DelegationAuthorization struct {
	ID                    uuid.UUID
	StoreID               int64
	Source                AuthorizationSource
	SourceReference       string
	Capabilities          []Capability
	EffectiveFrom         time.Time
	EffectiveUntil        *time.Time
	ApprovedByAdminID     int64
	ApprovedByDisplayName string
	ApprovedAt            time.Time
	RevokedAt             *time.Time
	RevokedByAdminID      *int64
	// RevokedByDisplayName 在沒有撤回時是 nil（不是空字串：空的名字與沒有撤回者是兩回事）。
	RevokedByDisplayName *string
	RevokeReason         *string
	// Effective 是讀取當下評估的有效性（前端不得自己由三個時間欄推導）。
	Effective bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AuthorizationAuditEntry 是一列治理歷史（寫入當下的快照，只追加）。
type AuthorizationAuditEntry struct {
	ID              uuid.UUID
	AuthorizationID uuid.UUID
	StoreID         int64
	Action          AuthorizationAction
	ActorAdminID    int64
	// ActorDisplayName 是寫入當下管理員的 email：稽核是歷史事實，之後改了 email 不回溯。
	ActorDisplayName string
	OperationReason  string
	// Source、SourceReference 在 REVOKE 是 nil（撤回不改它們）。
	Source          *AuthorizationSource
	SourceReference *string
	// CapabilitiesBefore 為 nil 表示這列授權之前不存在（只有 GRANT）。
	CapabilitiesBefore   []Capability
	CapabilitiesAfter    []Capability
	EffectiveUntilBefore *time.Time
	EffectiveUntilAfter  *time.Time
	CreatedAt            time.Time
}

// DelegationAuthorizationFilter 是治理總覽的篩選。
type DelegationAuthorizationFilter struct {
	Status AuthorizationStatusFilter
	// StoreID 為 nil 時不篩選。
	StoreID *int64
}

type ListDelegationAuthorizationsInput struct {
	AdminID int64
	Filter  DelegationAuthorizationFilter
	Paging  DelegationPaging
}

type DelegationAuthorizationHistoryInput struct {
	AdminID int64
	StoreID int64
	Paging  DelegationPaging
}

// DelegationAuthorizationPage 是治理總覽的一頁。
type DelegationAuthorizationPage struct {
	Authorizations []DelegationAuthorization
	Total          int64
	Paging         DelegationPaging
}

// DelegationAuthorizationPanel 是單店的授權面板。
type DelegationAuthorizationPanel struct {
	// Exists 為 false 表示這家店從來沒有開通（正常的狀態，不是錯誤），此時 Authorization 是零值。
	Exists        bool
	Authorization DelegationAuthorization
	// ActiveSessions 是這家店此刻進行中的代行數：撤回的確認框靠它（撤回會連帶中止它們）。與 Exists 正交：撤回過的店
	// 仍可能有殘留的 session。
	ActiveSessions int64
}

// DelegationAuthorizationChange 是開通與調整共用的輸入：兩者只差在 CAS 的述詞，形狀不同會逼前端維護兩套表單。每個欄位
// 都是「授權變成什麼」（全量取代）。
type DelegationAuthorizationChange struct {
	AdminID         int64
	StoreID         int64
	Source          AuthorizationSource
	SourceReference string
	Capabilities    []Capability
	// EffectiveUntil 為 nil 是無期限；重新開通時不沿用舊值。
	EffectiveUntil *time.Time
	// OperationReason 是這一次為什麼按下按鈕（1–200 字，只寫進歷史）。
	OperationReason string
	IdempotencyKey  string
	RequestID       string
	CorrelationID   string
}

// GrantedAuthorization 是開通的結果。
type GrantedAuthorization struct {
	Authorization DelegationAuthorization
	Outcome       GrantOutcome
}

// DelegationAuthorizationRevocation 是撤回授權的輸入。
type DelegationAuthorizationRevocation struct {
	AdminID int64
	StoreID int64
	// Reason 是 1–200 字的撤回理由（撤回的三欄同生同滅）。
	Reason         string
	IdempotencyKey string
	RequestID      string
	CorrelationID  string
}

// RevokedAuthorization 是撤回的結果。
type RevokedAuthorization struct {
	Authorization DelegationAuthorization
	// AlreadyRevoked 表示抵達時已經撤回：沒有改任何欄位、沒有寫歷史、沒有中止 session。
	AlreadyRevoked bool
	// RevokedSessions 是這次連帶中止的 session（沒有時是空的 slice，不是 nil）。
	RevokedSessions []int64
}

// AuthorizationHistoryPage 是單店治理歷史的一頁。
type AuthorizationHistoryPage struct {
	Entries []AuthorizationAuditEntry
	Total   int64
	Paging  DelegationPaging
}
