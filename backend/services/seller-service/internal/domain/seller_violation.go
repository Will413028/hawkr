package domain

import (
	"errors"
	"time"
)

// B-169 Seller Violation — admin 建立的違規處罰紀錄。
// status 兩值（D11 enforced / revoked），「目前是否生效」即時計算。

// PenaltyType 處罰類型。enum 全保留，application 層限 admin 在 Phase 1 只能選 warning + full_suspension（D4）。
type PenaltyType string

const (
	PenaltyTypeWarning           PenaltyType = "warning"
	PenaltyTypeFine              PenaltyType = "fine"
	PenaltyTypePartialSuspension PenaltyType = "partial_suspension"
	PenaltyTypeFullSuspension    PenaltyType = "full_suspension"
	PenaltyTypePermanentBan      PenaltyType = "permanent_ban"
	PenaltyTypeMandatoryTraining PenaltyType = "mandatory_training"
)

// AllPenaltyTypes 全部合法 enum 值（DB CHECK 接受）。
var AllPenaltyTypes = map[PenaltyType]struct{}{
	PenaltyTypeWarning:           {},
	PenaltyTypeFine:              {},
	PenaltyTypePartialSuspension: {},
	PenaltyTypeFullSuspension:    {},
	PenaltyTypePermanentBan:      {},
	PenaltyTypeMandatoryTraining: {},
}

// Phase1EnabledPenaltyTypes admin 端 Phase 1 允許選擇的 penalty_type。
// 其他 enum 值在 application 層拒絕（回 400），DB 仍接受（schema chk_sv_penalty_fields）。
var Phase1EnabledPenaltyTypes = map[PenaltyType]struct{}{
	PenaltyTypeWarning:        {},
	PenaltyTypeFullSuspension: {},
}

// IsValid 檢查 penalty_type 字串是否在 DB schema 接受範圍內。
func (p PenaltyType) IsValid() bool {
	_, ok := AllPenaltyTypes[p]
	return ok
}

// IsPhase1Enabled 檢查是否為 Phase 1 admin 可選的 penalty_type。
func (p PenaltyType) IsPhase1Enabled() bool {
	_, ok := Phase1EnabledPenaltyTypes[p]
	return ok
}

// ViolationStatus 違規生效狀態。D11：兩值，「目前是否生效」由 IsActive() 即時計算。
type ViolationStatus string

const (
	ViolationStatusEnforced ViolationStatus = "enforced"
	ViolationStatusRevoked  ViolationStatus = "revoked"
)

// ValidViolationStatuses 合法值集合。
var ValidViolationStatuses = map[ViolationStatus]struct{}{
	ViolationStatusEnforced: {},
	ViolationStatusRevoked:  {},
}

// IsValid 檢查 status 字串是否合法。
func (s ViolationStatus) IsValid() bool {
	_, ok := ValidViolationStatuses[s]
	return ok
}

// AppealStatus 申訴決議狀態。
type AppealStatus string

const (
	AppealStatusPending  AppealStatus = "pending"
	AppealStatusAccepted AppealStatus = "accepted"
	AppealStatusRejected AppealStatus = "rejected"
)

// ValidAppealStatuses 合法值集合。
var ValidAppealStatuses = map[AppealStatus]struct{}{
	AppealStatusPending:  {},
	AppealStatusAccepted: {},
	AppealStatusRejected: {},
}

// IsValid 檢查 appeal_status 字串是否合法。
func (s AppealStatus) IsValid() bool {
	_, ok := ValidAppealStatuses[s]
	return ok
}

// RevokedReason 撤銷原因 enum。
type RevokedReason string

const (
	RevokedReasonAppealAccepted   RevokedReason = "appeal_accepted"
	RevokedReasonAdminCorrection  RevokedReason = "admin_correction"
	RevokedReasonExternalDecision RevokedReason = "external_decision"
	RevokedReasonOther            RevokedReason = "other"
)

// AutoEscalateThresholdMediumCount D6: 近 90 天 medium 數 >= 此值 → 自動升級 high。
const AutoEscalateThresholdMediumCount = 3

// AutoEscalateLookbackDays D6: 自動升級 lookback window。
const AutoEscalateLookbackDays = 90

// SellerViolation admin 建立的違規處罰紀錄。
type SellerViolation struct {
	CreatedAt   time.Time
	UpdatedAt   time.Time
	EffectiveAt time.Time

	ExpiresAt *time.Time
	RevokedAt *time.Time

	// source 互斥（D16）
	SourceSellerReportID     *int64
	SourceFoodSafetyReportID *int64

	PenaltyAmountCents *int64
	PublicSummary      *string
	RevokedReason      *RevokedReason

	// 申訴內嵌欄位（D7）
	AppealReason           *string
	AppealEvidenceAssetIDs []string
	AppealStatus           *AppealStatus
	AppealSubmittedAt      *time.Time
	AppealDecidedAt        *time.Time
	AppealDecisionReason   *string
	AppealDecidedByAdminID *int64
	Description            string
	Category               ViolationCategory
	Severity               ViolationSeverity
	PenaltyType            PenaltyType
	Status                 ViolationStatus
	ID                     int64
	SellerID               int64
	CreatedByAdminID       int64
	Appealed               bool
}

// IsActive D11: 目前是否生效（計算屬性 — 不對應 DB 欄位）。
func (v *SellerViolation) IsActive() bool {
	if v.Status != ViolationStatusEnforced {
		return false
	}
	if v.RevokedAt != nil {
		return false
	}
	if v.ExpiresAt != nil && !v.ExpiresAt.After(timeNow()) {
		return false
	}
	return true
}

// IsActiveAt 檢查在指定時間點是否生效（測試 / cron worker 用，避免依賴 timeNow()）。
func (v *SellerViolation) IsActiveAt(t time.Time) bool {
	if v.Status != ViolationStatusEnforced {
		return false
	}
	if v.RevokedAt != nil {
		return false
	}
	if v.ExpiresAt != nil && !v.ExpiresAt.After(t) {
		return false
	}
	return true
}

// IsPubliclyVisible D8: 對外（賣家頁）是否可見。
//   - severity 必須 high 或 critical
//   - revoked_at IS NULL
//   - effective_at >= NOW() - 90d
//   - (expires_at IS NULL OR expires_at > NOW())
//   - public_summary IS NOT NULL（admin 必須撰寫公開摘要）
func (v *SellerViolation) IsPubliclyVisibleAt(t time.Time) bool {
	if v.Severity != ViolationSeverityHigh && v.Severity != ViolationSeverityCritical {
		return false
	}
	if v.RevokedAt != nil {
		return false
	}
	if v.EffectiveAt.Before(t.Add(-90 * 24 * time.Hour)) {
		return false
	}
	if v.ExpiresAt != nil && !v.ExpiresAt.After(t) {
		return false
	}
	if v.PublicSummary == nil || *v.PublicSummary == "" {
		return false
	}
	return true
}

// HasSource 是否有 source report 引用（admin 主動建立則 false）。
func (v *SellerViolation) HasSource() bool {
	return v.SourceSellerReportID != nil || v.SourceFoodSafetyReportID != nil
}

// Validate 一次性檢查 enum + 必填 + 互斥規則。對應 schema 的 chk_sv_penalty_fields / chk_sv_source_mutex。
func (v *SellerViolation) Validate() error {
	if v.SellerID == 0 {
		return errors.New("seller_id is required")
	}
	if v.CreatedByAdminID == 0 {
		return errors.New("created_by_admin_id is required")
	}
	if !v.Category.IsValid() {
		return errors.New("invalid category")
	}
	if !v.Severity.IsValid() {
		return errors.New("invalid severity")
	}
	if !v.PenaltyType.IsValid() {
		return errors.New("invalid penalty_type")
	}
	if !v.Status.IsValid() {
		return errors.New("invalid status")
	}
	if v.Description == "" {
		return errors.New("description is required")
	}
	// D16 source 互斥
	if v.SourceSellerReportID != nil && v.SourceFoodSafetyReportID != nil {
		return errors.New("at most one source report id allowed")
	}
	// D14 penalty 欄位互斥（與 schema chk_sv_penalty_fields 對齊）
	if err := v.validatePenaltyFields(); err != nil {
		return err
	}
	return nil
}

// validatePenaltyFields D14：依 penalty_type 檢查 penalty_amount_cents / expires_at 互斥。
func (v *SellerViolation) validatePenaltyFields() error {
	switch v.PenaltyType {
	case PenaltyTypeWarning:
		if v.PenaltyAmountCents != nil {
			return errors.New("penalty_amount_cents must be NULL for warning")
		}
		if v.ExpiresAt != nil {
			return errors.New("expires_at must be NULL for warning")
		}
	case PenaltyTypeFullSuspension:
		if v.PenaltyAmountCents != nil {
			return errors.New("penalty_amount_cents must be NULL for full_suspension")
		}
		if v.ExpiresAt == nil {
			return errors.New("expires_at is required for full_suspension")
		}
	case PenaltyTypeFine:
		if v.PenaltyAmountCents == nil {
			return errors.New("penalty_amount_cents is required for fine")
		}
		if v.ExpiresAt != nil {
			return errors.New("expires_at must be NULL for fine")
		}
	case PenaltyTypePartialSuspension:
		if v.PenaltyAmountCents != nil {
			return errors.New("penalty_amount_cents must be NULL for partial_suspension")
		}
		if v.ExpiresAt == nil {
			return errors.New("expires_at is required for partial_suspension")
		}
	case PenaltyTypePermanentBan, PenaltyTypeMandatoryTraining:
		// Phase 1 admin 不選；schema CHECK 也未強制細節，暫時不驗
	default:
		return errors.New("unsupported penalty_type")
	}
	return nil
}

// CanSubmitAppeal 賣家提交申訴前的檢查（限申訴一次、status 必須 enforced）。
func (v *SellerViolation) CanSubmitAppeal() error {
	if v.Status == ViolationStatusRevoked {
		return errors.New("cannot appeal a revoked violation")
	}
	if v.Appealed {
		return errors.New("appeal already submitted")
	}
	return nil
}

// CanDecideAppeal admin 決議申訴前檢查。
func (v *SellerViolation) CanDecideAppeal() error {
	if !v.Appealed {
		return errors.New("no appeal to decide")
	}
	if v.AppealStatus == nil || *v.AppealStatus != AppealStatusPending {
		return errors.New("appeal already decided")
	}
	return nil
}

// SellerViolationListFilter admin list filter。
type SellerViolationListFilter struct {
	SellerID     *int64
	Severity     *ViolationSeverity
	PenaltyType  *PenaltyType
	AppealStatus *AppealStatus
	Limit        int32
	Offset       int32
}

// timeNow 包裝 time.Now() 方便 test 替換。預設指向 time.Now。
var timeNow = time.Now
