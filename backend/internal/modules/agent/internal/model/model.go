// Package model 是 agent 模組的內部型別：代碼值、回傳型別、錯誤與期限常數。
package model

import (
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// VerificationMethod 是 session 驗證業者持有邀請信箱的方式。儲存的代碼是 UPPER_SNAKE，DB 的 CHECK 與這裡的值集合相同
// （TestChecksMatchGo 綁在一起）。
type VerificationMethod string

const (
	// VerificationGoogle 是 Google 登入：入口經 identity 驗證 Google token，模組只收已驗證的 email。
	VerificationGoogle VerificationMethod = "GOOGLE"
)

// VerificationMethodValues 回傳全部驗證方式。
func VerificationMethodValues() []VerificationMethod { return []VerificationMethod{VerificationGoogle} }

// Valid 回傳是不是已知的驗證方式。
func (m VerificationMethod) Valid() bool { return slices.Contains(VerificationMethodValues(), m) }

// RevokeReason 是邀請被撤銷的原因。儲存的代碼是 UPPER_SNAKE，與 CHECK 的值集合相同。
type RevokeReason string

const (
	// RevokeProspectArchived 是 prospect 封存。合併不撤銷：邀請改掛贏家（ADR「合併時的邀請」）。
	RevokeProspectArchived RevokeReason = "PROSPECT_ARCHIVED"
)

// RevokeReasonValues 回傳全部撤銷原因。
func RevokeReasonValues() []RevokeReason {
	return []RevokeReason{RevokeProspectArchived}
}

// 期限與上限（ADR 2026-10-03-vendor-portal-invite-scoped-access「決策」）。
const (
	// SessionIdleTTL 是 session 的 idle 期限，每次有效的使用滑動（最多節流到 TouchInterval 一次）。
	SessionIdleTTL = 24 * time.Hour
	// SessionAbsoluteTTL 是 session 的絕對期限；實際上再被邀請的到期時間截斷，session 不會比邀請活得久。
	SessionAbsoluteTTL = 7 * 24 * time.Hour
	// TouchInterval 是滑動 idle 期限的節流：距上次看到不到這麼久，不寫入。
	TouchInterval = 5 * time.Minute
	// MaxActiveSessions 是一張邀請同時有效的 session 上限，超過時先撤銷最舊的未驗證 session，全部都已驗證才撤銷最舊的；本次新發的不在撤銷之列。
	MaxActiveSessions = 5
	// MaxInviteLifetime 是 EnsureInvite 接受的到期時間上限（從現在起算）。
	MaxInviteLifetime = 180 * 24 * time.Hour
	// SessionRetention 是 session 過了絕對期限之後保留的期間。
	SessionRetention = 7 * 24 * time.Hour
	// InviteRetention 是邀請到期或撤銷之後保留的期間（到期才刪除 invited_email）。
	InviteRetention = 90 * 24 * time.Hour
)

// 給 notification 的 Ensure／Mark 兩段式的結果（A8 之前成立）。用 errors.Is 判斷；之後的 legacy gRPC adapter 依分類映射。
var (
	// ErrInviteAlreadyDelivered 表示這張邀請的 secret 已經標記寄出：不得再換 secret、再寄一封。
	ErrInviteAlreadyDelivered = apperr.AlreadyExists("邀請的連結已經寄出")
	// ErrInviteRevoked 表示這張邀請已撤銷（prospect 封存）：不寄。
	ErrInviteRevoked = apperr.FailedPrecondition("邀請已撤銷")
	// ErrProspectNotActive 表示 prospect 不是進行中（封存、被合併或已轉成店家）：不建立也不換 secret。
	ErrProspectNotActive = apperr.FailedPrecondition("業者不是進行中，不能發出邀請")
	// ErrInviteSecretSuperseded 表示 Mark 帶的 secret 已不是目前的 secret（Ensure 在這之間換過）：標記不生效，寄出的連結
	// 已經失效，呼叫方要重新 Ensure 再寄。
	ErrInviteSecretSuperseded = apperr.VersionConflict("邀請的 secret 已被換掉")
	// ErrNoDeliverableEmail 表示 prospect 沒有收得到信的 email：不寄。
	ErrNoDeliverableEmail = apperr.FailedPrecondition("業者沒有可寄送的 email")
)

// EnsureInviteInput 是建立或重取邀請的輸入。ProspectID 屬於 seller，RfqRecipientID 屬於 ai-service（A9 之前不驗證存在）。
type EnsureInviteInput struct {
	ProspectID, RfqRecipientID int64
	ExpiresAt                  time.Time
}

// IssuedInvite 是 EnsureInvite 的結果；Secret 是明文，只在這裡出現一次，由呼叫方放進信件。
type IssuedInvite struct {
	InviteID     uuid.UUID
	Secret       string
	InvitedEmail string
}

// InvitePreview 是邀請的預覽（不產生副作用）。
type InvitePreview struct {
	// InviteID 與 RfqRecipientID 只給單體入口判斷「請求帶的 session 是不是這張邀請的」；不進對外的 API。
	InviteID       uuid.UUID
	RfqRecipientID int64
	MaskedEmail    string
	ExpiresAt      time.Time
}

// IssuedSession 是兌換邀請的結果；Token 是明文，只在這裡出現一次。
type IssuedSession struct {
	Token                            string
	InviteID                         uuid.UUID
	RfqRecipientID                   int64
	IdleExpiresAt, AbsoluteExpiresAt time.Time
}

// Verification 是 session 的驗證紀錄：方式、驗證的 email 與時間。
type Verification struct {
	Method VerificationMethod
	Email  string
	At     time.Time
}

// SessionInfo 是有效 session 的內容；Verification 是 nil 表示還沒驗證。
type SessionInfo struct {
	SessionID, InviteID uuid.UUID
	RfqRecipientID      int64
	Verification        *Verification
}

// PurgeResult 是清理刪除的列數。
type PurgeResult struct{ Sessions, Invites int64 }
