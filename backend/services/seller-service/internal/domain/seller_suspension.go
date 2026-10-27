package domain

import (
	"context"
	"time"
)

// Admin 手動停權（不服勸導）— 32 §13.2 / 44 §13.2。與 governance 停權
// （seller_violations, penalty_type='full_suspension'）並存但獨立：僅 admin 手動解除，
// 無 expires_at、不進 cron，precedence 高於 suspended_governance（32 §3.5）。

// SellerSuspensionStatus is the lifecycle of one suspension episode row
// (active / lifted — no "expired": admin suspension never auto-expires).
type SellerSuspensionStatus string

const (
	SellerSuspensionActive SellerSuspensionStatus = "active"
	SellerSuspensionLifted SellerSuspensionStatus = "lifted"
)

// SellerSuspensionReason mirrors CodeMeta 業務狀態碼.md SellerSuspensionReason
// (three values, 46 §5.1 已定裁決：不含 FOOD_SAFETY，該原因走 32 號治理路徑)。
//
// issue #1534：全鏈路（DB `seller_suspensions.reason_code` / domain / Redis
// marker / MQ payload / gRPC 傳輸）統一為 CodeMeta **Value 數字碼**——與全
// repo 其他 CodeMeta enum 的「DB 存 Value」慣例一致；#1514 時期「DB 存 Name、
// 輸出邊界 Name→Value 轉換、輸入端 Name/Value 雙認」的過渡層已整層移除。
// 存量資料由 data migration（db/migrations/*_1534_*.sql）一次轉換。
const (
	SellerSuspensionReasonPolicyViolation        = "9601" // POLICY_VIOLATION（違反平台規範）
	SellerSuspensionReasonRepeatedWarningIgnored = "9602" // REPEATED_WARNING_IGNORED（屢勸不改）
	SellerSuspensionReasonOther                  = "9603" // OTHER（其他）
)

// IsValidSellerSuspensionReason validates against the closed CodeMeta enum
// (Go 服務無 CodeMetaValidator，此為等效的 allow-list 檢查 — §8 Java-only 工具的 Go 對應)。
// 只認 Value 數字碼；#1514 的 Name 字串輸入相容已於 #1534 移除。
func IsValidSellerSuspensionReason(code string) bool {
	switch code {
	case SellerSuspensionReasonPolicyViolation, SellerSuspensionReasonRepeatedWarningIgnored, SellerSuspensionReasonOther:
		return true
	default:
		return false
	}
}

// sellerSuspensionReasonLabels mirrors CodeMeta 業務狀態碼.md
// SellerSuspensionReason 的 Value→Label 對照。
//
// Go 服務尚未接線 `internal/platform/codemeta/code_meta.go`
// 這份 codegen 產物（該檔屬於單體 module，seller-service 的 module import 不到）——
// seller-service 是唯一消費此值的 Go 服務，此處手動維護一份小型對照表是務實
// 折衷，非重複實作 SSOT（來源仍是 CodeMeta .md）；待「Go 服務接 CodeMeta
// codegen」的更大範圍工作完成後應改為呼叫生成產物，屆時此表可整段移除。
var sellerSuspensionReasonLabels = map[string]string{
	SellerSuspensionReasonPolicyViolation:        "違反平台規範",
	SellerSuspensionReasonRepeatedWarningIgnored: "屢勸不改",
	SellerSuspensionReasonOther:                  "其他",
}

// SellerSuspensionReasonLabel returns the human-readable CodeMeta label for
// a Value (e.g. "9601" → "違反平台規範"), used to enrich the seller.suspended
// MQ payload (issue #1514) so notification-service's DB-driven template
// engine — which only does literal `#{field}` substitution, no lookup —
// doesn't render a bare numeric code to the seller. Unrecognized input
// returns "".
func SellerSuspensionReasonLabel(value string) string {
	return sellerSuspensionReasonLabels[value]
}

type SellerSuspension struct {
	ID                 int64
	AccountID          int64
	ReasonCode         string
	ReasonDetail       string
	Status             SellerSuspensionStatus
	SuspendedByAdminID int64
	SuspendedAt        time.Time
	LiftedByAdminID    *int64
	LiftedAt           *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IsActive returns whether this suspension episode is currently in effect.
func (s *SellerSuspension) IsActive() bool { return s.Status == SellerSuspensionActive }

// SellerSuspensionRepository 對應 seller_suspensions table。
type SellerSuspensionRepository interface {
	// Insert creates a new active suspension row inside the caller's transaction.
	// Returns *ErrDuplicate when uq_seller_suspensions_active rejects a second
	// concurrent active suspension for the same account (23505) — caller maps
	// this to the idempotent "already suspended" response (32 §13.4).
	Insert(ctx context.Context, s *SellerSuspension) error

	// GetActiveByAccountID returns the current active suspension, or nil if none.
	GetActiveByAccountID(ctx context.Context, accountID int64) (*SellerSuspension, error)

	// HasActiveByAccountID is the cheap existence check used by
	// recomputeSellerStatus (32 §13.3).
	HasActiveByAccountID(ctx context.Context, accountID int64) (bool, error)

	// GetByID returns a suspension row by its TSID (any status).
	GetByID(ctx context.Context, id int64) (*SellerSuspension, error)

	// Lift transitions status active→lifted (only-if condition status='active').
	// Returns nil (no error) when the row was already lifted — caller re-fetches
	// via GetByID to build the idempotent "already lifted" response.
	Lift(ctx context.Context, id, liftedByAdminID int64, now time.Time) (*SellerSuspension, error)

	// ListByAccountID returns suspension history (all statuses), newest first.
	ListByAccountID(ctx context.Context, accountID int64, limit, offset int32) ([]*SellerSuspension, error)
}
