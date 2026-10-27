package domain

import (
	"errors"
	"time"
)

// B-169 Seller Violation Report — 登入端商家違規檢舉。
// 對應 schema/schema.sql 的 seller_violation_reports table。

// ViolationCategory 商家違規類別。對應 issue #722 的 7 種 enum。
type ViolationCategory string

const (
	ViolationCategoryFoodSafety        ViolationCategory = "food_safety"
	ViolationCategoryFakeCertification ViolationCategory = "fake_certification"
	ViolationCategoryMisleadingInfo    ViolationCategory = "misleading_info"
	ViolationCategoryCounterfeit       ViolationCategory = "counterfeit"
	ViolationCategoryUnauthorizedSale  ViolationCategory = "unauthorized_sale"
	ViolationCategoryFraud             ViolationCategory = "fraud"
	ViolationCategoryOther             ViolationCategory = "other"
)

// ValidViolationCategories 所有合法 category 值。validation 用。
var ValidViolationCategories = map[ViolationCategory]struct{}{
	ViolationCategoryFoodSafety:        {},
	ViolationCategoryFakeCertification: {},
	ViolationCategoryMisleadingInfo:    {},
	ViolationCategoryCounterfeit:       {},
	ViolationCategoryUnauthorizedSale:  {},
	ViolationCategoryFraud:             {},
	ViolationCategoryOther:             {},
}

// IsValid 檢查 category 字串是否合法。
func (c ViolationCategory) IsValid() bool {
	_, ok := ValidViolationCategories[c]
	return ok
}

// ViolationSeverity 違規嚴重度。
type ViolationSeverity string

const (
	ViolationSeverityLow      ViolationSeverity = "low"
	ViolationSeverityMedium   ViolationSeverity = "medium"
	ViolationSeverityHigh     ViolationSeverity = "high"
	ViolationSeverityCritical ViolationSeverity = "critical"
)

// ValidViolationSeverities 所有合法 severity 值。
var ValidViolationSeverities = map[ViolationSeverity]struct{}{
	ViolationSeverityLow:      {},
	ViolationSeverityMedium:   {},
	ViolationSeverityHigh:     {},
	ViolationSeverityCritical: {},
}

// IsValid 檢查 severity 字串是否合法。
func (s ViolationSeverity) IsValid() bool {
	_, ok := ValidViolationSeverities[s]
	return ok
}

// SeverityRank 用於排序：critical 最前。
func (s ViolationSeverity) Rank() int {
	switch s {
	case ViolationSeverityCritical:
		return 0
	case ViolationSeverityHigh:
		return 1
	case ViolationSeverityMedium:
		return 2
	case ViolationSeverityLow:
		return 3
	default:
		return 4
	}
}

// ReportStatus 檢舉處理狀態。seller_violation_reports 與 food_safety_reports 共用此 enum 語意。
type ReportStatus string

const (
	ReportStatusPending       ReportStatus = "pending"
	ReportStatusInvestigating ReportStatus = "investigating"
	ReportStatusVerified      ReportStatus = "verified"
	ReportStatusDismissed     ReportStatus = "dismissed"
)

// ValidReportStatuses 所有合法 status 值。
var ValidReportStatuses = map[ReportStatus]struct{}{
	ReportStatusPending:       {},
	ReportStatusInvestigating: {},
	ReportStatusVerified:      {},
	ReportStatusDismissed:     {},
}

// IsValid 檢查 status 字串是否合法。
func (s ReportStatus) IsValid() bool {
	_, ok := ValidReportStatuses[s]
	return ok
}

// IsTerminal 是否為終態（不可再 transition）。
func (s ReportStatus) IsTerminal() bool {
	return s == ReportStatusVerified || s == ReportStatusDismissed
}

// CanTransitionTo 檢查狀態推進是否合法。
//
// 合法路徑：
//   - pending → investigating
//   - pending → dismissed（admin 直接駁回）
//   - investigating → verified
//   - investigating → dismissed
//
// 終態（verified / dismissed）不可改 — 需走 violation 申訴決議流程。
func (s ReportStatus) CanTransitionTo(next ReportStatus) bool {
	switch s {
	case ReportStatusPending:
		return next == ReportStatusInvestigating || next == ReportStatusDismissed
	case ReportStatusInvestigating:
		return next == ReportStatusVerified || next == ReportStatusDismissed
	default:
		return false
	}
}

type SellerViolationReport struct {
	CreatedAt              time.Time
	UpdatedAt              time.Time
	InvestigatingAt        *time.Time
	ResolvedAt             *time.Time
	AdminResolutionSummary *string

	// targets — at least target_seller_id required
	TargetProductID  *int64
	TargetPostID     *int64
	TargetOrderNo    *string
	EvidenceAssetIDs []string // media-service asset_id 列表（DB 存 jsonb）

	Description string
	Category    ViolationCategory
	Severity    ViolationSeverity
	Status      ReportStatus

	ID                int64
	TargetSellerID    int64
	ReporterAccountID int64
}

// Validate 檢查必填欄位與 enum 合法性。
func (r *SellerViolationReport) Validate() error {
	if r.TargetSellerID == 0 {
		return errors.New("target_seller_id is required")
	}
	if r.ReporterAccountID == 0 {
		return errors.New("reporter_account_id is required (anonymous reports use food_safety_reports)")
	}
	if !r.Category.IsValid() {
		return errors.New("invalid category")
	}
	if !r.Severity.IsValid() {
		return errors.New("invalid severity")
	}
	if !r.Status.IsValid() {
		return errors.New("invalid status")
	}
	if r.Description == "" {
		return errors.New("description is required")
	}
	return nil
}

// SellerViolationReportListFilter admin list filter（pagination + filters）。
type SellerViolationReportListFilter struct {
	Status   *ReportStatus
	Severity *ViolationSeverity
	Since    *time.Time
	Limit    int32
	Offset   int32
}
