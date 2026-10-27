package postgres

import (
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

// B-169 Governance — shared helpers between report / violation / notification_log repositories.

// marshalAssetIDs converts []string to jsonb-compatible []byte.
// Empty / nil slice maps to nil (DB NULL), matching the repository JSON policy.
func marshalAssetIDs(ids []string) []byte {
	if len(ids) == 0 {
		return nil
	}
	b, _ := json.Marshal(ids)
	return b
}

// unmarshalAssetIDs converts jsonb []byte back to []string. nil / empty → nil.
// Tolerant: invalid jsonb returns nil (B-169 evidence_asset_ids is admin-controlled, but defensive).
func unmarshalAssetIDs(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	var ids []string
	if err := json.Unmarshal(b, &ids); err != nil {
		return nil
	}
	return ids
}

// isUniqueViolation 偵測 PG SQLState 23505 + 回傳 constraint name。
// 用於 D13 calendar-day partial unique 與 B1 ISO-week partial unique 的 application 捕捉路徑。
func isUniqueViolation(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// ========== to-domain mappers ==========

func toDomainSellerViolationReport(row gen.SellerViolationReport) *domain.SellerViolationReport {
	return &domain.SellerViolationReport{
		ID:                     row.ID,
		TargetSellerID:         row.TargetSellerID,
		TargetProductID:        row.TargetProductID,
		TargetPostID:           row.TargetPostID,
		TargetOrderNo:          row.TargetOrderNo,
		ReporterAccountID:      row.ReporterAccountID,
		Category:               domain.ViolationCategory(row.Category),
		Severity:               domain.ViolationSeverity(row.Severity),
		Description:            row.Description,
		EvidenceAssetIDs:       unmarshalAssetIDs(row.EvidenceAssetIds),
		Status:                 domain.ReportStatus(row.Status),
		InvestigatingAt:        row.InvestigatingAt,
		ResolvedAt:             row.ResolvedAt,
		AdminResolutionSummary: row.AdminResolutionSummary,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
	}
}

func toDomainFoodSafetyReport(row gen.FoodSafetyReport) *domain.FoodSafetyReport {
	r := &domain.FoodSafetyReport{
		ID:                     row.ID,
		TargetSellerID:         row.TargetSellerID,
		TargetProductID:        row.TargetProductID,
		TargetOrderNo:          row.TargetOrderNo,
		ReporterPhone:          row.ReporterPhone,
		ReporterEmail:          row.ReporterEmail,
		ReporterName:           row.ReporterName,
		Description:            row.Description,
		EvidenceAssetIDs:       unmarshalAssetIDs(row.EvidenceAssetIds),
		Status:                 domain.ReportStatus(row.Status),
		AcknowledgedAt:         row.AcknowledgedAt,
		AcknowledgedByAdminID:  row.AcknowledgedByAdminID,
		InvestigatingAt:        row.InvestigatingAt,
		ResolvedAt:             row.ResolvedAt,
		AdminResolutionSummary: row.AdminResolutionSummary,
		FDAReportedAt:          row.FdaReportedAt,
		FDAReferenceNo:         row.FdaReferenceNo,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
	}
	if ch := stringValue(row.FdaReportChannel); ch != "" {
		c := domain.FDAReportChannel(ch)
		r.FDAReportChannel = &c
	}
	return r
}

func toDomainSellerViolation(row gen.SellerViolation) *domain.SellerViolation {
	v := &domain.SellerViolation{
		ID:                       row.ID,
		SellerID:                 row.SellerID,
		SourceSellerReportID:     row.SourceSellerReportID,
		SourceFoodSafetyReportID: row.SourceFoodSafetyReportID,
		Category:                 domain.ViolationCategory(row.Category),
		Severity:                 domain.ViolationSeverity(row.Severity),
		PenaltyType:              domain.PenaltyType(row.PenaltyType),
		PenaltyAmountCents:       row.PenaltyAmountCents,
		Description:              row.Description,
		PublicSummary:            row.PublicSummary,
		EffectiveAt:              row.EffectiveAt.Time,
		ExpiresAt:                row.ExpiresAt,
		Status:                   domain.ViolationStatus(row.Status),
		RevokedAt:                row.RevokedAt,
		Appealed:                 row.Appealed,
		AppealReason:             row.AppealReason,
		AppealEvidenceAssetIDs:   unmarshalAssetIDs(row.AppealEvidenceAssetIds),
		AppealSubmittedAt:        row.AppealSubmittedAt,
		AppealDecidedAt:          row.AppealDecidedAt,
		AppealDecisionReason:     row.AppealDecisionReason,
		AppealDecidedByAdminID:   row.AppealDecidedByAdminID,
		CreatedByAdminID:         row.CreatedByAdminID,
		CreatedAt:                row.CreatedAt.Time,
		UpdatedAt:                row.UpdatedAt.Time,
	}
	if rr := stringValue(row.RevokedReason); rr != "" {
		reason := domain.RevokedReason(rr)
		v.RevokedReason = &reason
	}
	if as := stringValue(row.AppealStatus); as != "" {
		s := domain.AppealStatus(as)
		v.AppealStatus = &s
	}
	return v
}

func toDomainNotificationLog(row gen.SellerNotificationLog) *domain.SellerNotificationLog {
	return &domain.SellerNotificationLog{
		ID:                row.ID,
		SourceReportID:    row.SourceReportID,
		SourceReportTable: row.SourceReportTable,
		TemplateKey:       row.TemplateKey,
		SellerID:          row.SellerID,
		SentByAdminID:     row.SentByAdminID,
		SentAt:            row.SentAt.Time,
		MessagePayload:    row.MessagePayload,
	}
}
