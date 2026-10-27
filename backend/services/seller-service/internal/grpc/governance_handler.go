package grpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// B-169 Governance gRPC handlers (issue #722).
//
// 所有 method follow seller-service 既有「不回 grpc.Status，回 proto code field」風格。

// ========== helpers ==========

// errToCode 把 pkgerrors.Error / nil 轉 proto code。0 = success，其他依 pkgerrors 分類。
func errToCode(err error) (int32, string) {
	if err == nil {
		return 0, ""
	}
	if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
		return int32(code), err.Error()
	}
	return int32(pkgerrors.CodeUnknown), err.Error()
}

func tsMs(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func tsMsPtr(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return tsMs(*t)
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func int64Deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// formatID renders a TSID int64 id as its decimal string. Per Proto 管理規範
// §6.1 ids cross the HTTP edge as strings to avoid JS number precision loss.
func formatID(v int64) string {
	return strconv.FormatInt(v, 10)
}

// optID renders an optional id; nil → "" so an absent id stays falsy on the wire.
func optID(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

// ========== Domain → Proto mappers ==========

func toProtoSellerViolationReport(r *domain.SellerViolationReport) *pb.SellerViolationReport {
	if r == nil {
		return nil
	}
	return &pb.SellerViolationReport{
		Id:                     formatID(r.ID),
		TargetSellerId:         formatID(r.TargetSellerID),
		TargetProductId:        optID(r.TargetProductID),
		TargetPostId:           optID(r.TargetPostID),
		TargetOrderNo:          strDeref(r.TargetOrderNo),
		ReporterAccountId:      formatID(r.ReporterAccountID),
		Category:               string(r.Category),
		Severity:               string(r.Severity),
		Description:            r.Description,
		EvidenceAssetIds:       r.EvidenceAssetIDs,
		Status:                 string(r.Status),
		InvestigatingAtMs:      tsMsPtr(r.InvestigatingAt),
		ResolvedAtMs:           tsMsPtr(r.ResolvedAt),
		AdminResolutionSummary: strDeref(r.AdminResolutionSummary),
		CreatedAtMs:            tsMs(r.CreatedAt),
	}
}

func toProtoFoodSafetyReport(r *domain.FoodSafetyReport) *pb.FoodSafetyReport {
	if r == nil {
		return nil
	}
	out := &pb.FoodSafetyReport{
		Id:                     formatID(r.ID),
		TargetSellerId:         optID(r.TargetSellerID),
		TargetProductId:        optID(r.TargetProductID),
		TargetOrderNo:          strDeref(r.TargetOrderNo),
		ReporterPhone:          r.ReporterPhone,
		ReporterEmail:          strDeref(r.ReporterEmail),
		ReporterName:           strDeref(r.ReporterName),
		Description:            r.Description,
		EvidenceAssetIds:       r.EvidenceAssetIDs,
		Status:                 string(r.Status),
		AcknowledgedAtMs:       tsMsPtr(r.AcknowledgedAt),
		AcknowledgedByAdminId:  optID(r.AcknowledgedByAdminID),
		InvestigatingAtMs:      tsMsPtr(r.InvestigatingAt),
		ResolvedAtMs:           tsMsPtr(r.ResolvedAt),
		AdminResolutionSummary: strDeref(r.AdminResolutionSummary),
		FdaReportedAtMs:        tsMsPtr(r.FDAReportedAt),
		FdaReferenceNo:         strDeref(r.FDAReferenceNo),
		CreatedAtMs:            tsMs(r.CreatedAt),
	}
	if r.FDAReportChannel != nil {
		out.FdaReportChannel = string(*r.FDAReportChannel)
	}
	return out
}

func toProtoSellerViolation(v *domain.SellerViolation) *pb.SellerViolation {
	if v == nil {
		return nil
	}
	out := &pb.SellerViolation{
		Id:                       formatID(v.ID),
		SellerId:                 formatID(v.SellerID),
		SourceSellerReportId:     optID(v.SourceSellerReportID),
		SourceFoodSafetyReportId: optID(v.SourceFoodSafetyReportID),
		Category:                 string(v.Category),
		Severity:                 string(v.Severity),
		PenaltyType:              string(v.PenaltyType),
		PenaltyAmountCents:       int64Deref(v.PenaltyAmountCents),
		Description:              v.Description,
		PublicSummary:            strDeref(v.PublicSummary),
		EffectiveAtMs:            tsMs(v.EffectiveAt),
		ExpiresAtMs:              tsMsPtr(v.ExpiresAt),
		Status:                   string(v.Status),
		RevokedAtMs:              tsMsPtr(v.RevokedAt),
		Appealed:                 v.Appealed,
		AppealReason:             strDeref(v.AppealReason),
		AppealEvidenceAssetIds:   v.AppealEvidenceAssetIDs,
		AppealSubmittedAtMs:      tsMsPtr(v.AppealSubmittedAt),
		AppealDecidedAtMs:        tsMsPtr(v.AppealDecidedAt),
		AppealDecisionReason:     strDeref(v.AppealDecisionReason),
		AppealDecidedByAdminId:   optID(v.AppealDecidedByAdminID),
		CreatedByAdminId:         formatID(v.CreatedByAdminID),
		CreatedAtMs:              tsMs(v.CreatedAt),
	}
	if v.RevokedReason != nil {
		out.RevokedReason = string(*v.RevokedReason)
	}
	if v.AppealStatus != nil {
		out.AppealStatus = string(*v.AppealStatus)
	}
	return out
}

func toProtoPublicSellerViolation(v *domain.SellerViolation) *pb.PublicSellerViolation {
	return &pb.PublicSellerViolation{
		Id:            formatID(v.ID),
		Category:      string(v.Category),
		Severity:      string(v.Severity),
		PenaltyType:   string(v.PenaltyType),
		PublicSummary: strDeref(v.PublicSummary),
		EffectiveAtMs: tsMs(v.EffectiveAt),
		ExpiresAtMs:   tsMsPtr(v.ExpiresAt),
	}
}

// ========== Proto → Domain helpers ==========

func optInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// parseInt64 converts a §4 string id to int64 at the gRPC boundary; non-numeric
// (incl. empty) → 0, which the usecase treats as not-found.
func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// optParseInt64 parses an optional §4 string id; empty / non-numeric → nil.
func optParseInt64(s string) *int64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func optString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func optTimeFromMs(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms)
	return &t
}

// ========== Reporter side ==========

func (s *SellerServer) CreateViolationReport(ctx context.Context, req *pb.CreateViolationReportRequest) (*pb.CreateViolationReportResponse, error) {
	r := &domain.SellerViolationReport{
		TargetSellerID:    parseInt64(req.TargetSellerId),
		TargetProductID:   optParseInt64(req.TargetProductId),
		TargetPostID:      optParseInt64(req.TargetPostId),
		TargetOrderNo:     optString(req.TargetOrderNo),
		ReporterAccountID: req.ReporterAccountId,
		Category:          domain.ViolationCategory(req.Category),
		Severity:          domain.ViolationSeverity(req.Severity),
		Description:       req.Description,
		EvidenceAssetIDs:  req.EvidenceAssetIds,
	}
	created, err := s.violationReportUC.CreateLoggedInReport(ctx, r)
	if err != nil {
		// 409 duplicate path 仍回 既有 report_id (created != nil)；其他錯誤 created 可能 nil。
		code, msg := errToCode(err)
		resp := &pb.CreateViolationReportResponse{Code: code, Message: msg}
		if created != nil {
			resp.ReportId = formatID(created.ID)
			resp.Duplicate = true
		}
		return resp, nil
	}
	return &pb.CreateViolationReportResponse{ReportId: formatID(created.ID)}, nil
}

func (s *SellerServer) ListMyViolationReports(ctx context.Context, req *pb.ListMyViolationReportsRequest) (*pb.ListMyViolationReportsResponse, error) {
	since := time.Time{}
	if req.SinceMs > 0 {
		since = time.UnixMilli(req.SinceMs)
	}
	reports, err := s.violationReportUC.ListMyReports(ctx, req.ReporterAccountId, since, req.Limit, req.Offset)
	if err != nil {
		c, m := errToCode(err)
		return &pb.ListMyViolationReportsResponse{Code: c, Message: m}, nil
	}
	out := make([]*pb.SellerViolationReport, 0, len(reports))
	for _, r := range reports {
		out = append(out, toProtoSellerViolationReport(r))
	}
	return &pb.ListMyViolationReportsResponse{Reports: out}, nil
}

func (s *SellerServer) CreateFoodSafetyReport(ctx context.Context, req *pb.CreateFoodSafetyReportRequest) (*pb.CreateFoodSafetyReportResponse, error) {
	if !s.foodSafetyEnabled {
		return &pb.CreateFoodSafetyReportResponse{Code: int32(pkgerrors.CodeUnavailable), Message: "food_safety_endpoint_enabled is false"}, nil
	}
	r := &domain.FoodSafetyReport{
		TargetSellerID:   optParseInt64(req.TargetSellerId),
		TargetProductID:  optParseInt64(req.TargetProductId),
		TargetOrderNo:    optString(req.TargetOrderNo),
		ReporterPhone:    req.ReporterPhone,
		ReporterEmail:    optString(req.ReporterEmail),
		ReporterName:     optString(req.ReporterName),
		Description:      req.Description,
		EvidenceAssetIDs: req.EvidenceAssetIds,
	}
	created, ackEstAt, err := s.foodSafetyReportUC.CreatePublicReport(ctx, r)
	if err != nil {
		c, m := errToCode(err)
		return &pb.CreateFoodSafetyReportResponse{Code: c, Message: m}, nil
	}
	return &pb.CreateFoodSafetyReportResponse{
		ReportId:       formatID(created.ID),
		AckEstimatedAt: tsMs(ackEstAt),
	}, nil
}

// ========== Seller side ==========

func (s *SellerServer) SellerListMyViolations(ctx context.Context, req *pb.SellerListMyViolationsRequest) (*pb.SellerListMyViolationsResponse, error) {
	violations, err := s.violationUC.SellerListMyViolations(ctx, req.SellerId, req.Limit, req.Offset)
	if err != nil {
		c, m := errToCode(err)
		return &pb.SellerListMyViolationsResponse{Code: c, Message: m}, nil
	}
	out := make([]*pb.SellerViolation, 0, len(violations))
	for _, v := range violations {
		out = append(out, toProtoSellerViolation(v))
	}
	return &pb.SellerListMyViolationsResponse{Violations: out}, nil
}

func (s *SellerServer) SellerSubmitAppeal(ctx context.Context, req *pb.SellerSubmitAppealRequest) (*pb.SellerSubmitAppealResponse, error) {
	v, err := s.violationUC.SellerSubmitAppeal(ctx, parseInt64(req.ViolationId), req.SellerId, req.AppealReason, req.AppealEvidenceAssetIds)
	if err != nil {
		c, m := errToCode(err)
		return &pb.SellerSubmitAppealResponse{Code: c, Message: m}, nil
	}
	return &pb.SellerSubmitAppealResponse{Violation: toProtoSellerViolation(v)}, nil
}

func (s *SellerServer) SellerGetAppeal(ctx context.Context, req *pb.SellerGetAppealRequest) (*pb.SellerGetAppealResponse, error) {
	v, err := s.violationUC.SellerGetAppeal(ctx, parseInt64(req.ViolationId), req.SellerId)
	if err != nil {
		c, m := errToCode(err)
		return &pb.SellerGetAppealResponse{Code: c, Message: m}, nil
	}
	return &pb.SellerGetAppealResponse{Violation: toProtoSellerViolation(v)}, nil
}

// ========== Public ==========

func (s *SellerServer) GetPublicSellerViolations(ctx context.Context, req *pb.GetPublicSellerViolationsRequest) (*pb.GetPublicSellerViolationsResponse, error) {
	violations, err := s.violationUC.GetPublicViolations(ctx, parseInt64(req.SellerId))
	if err != nil {
		c, m := errToCode(err)
		return &pb.GetPublicSellerViolationsResponse{Code: c, Message: m}, nil
	}
	out := make([]*pb.PublicSellerViolation, 0, len(violations))
	for _, v := range violations {
		out = append(out, toProtoPublicSellerViolation(v))
	}
	return &pb.GetPublicSellerViolationsResponse{Violations: out}, nil
}

// ========== Admin: List ==========

func (s *SellerServer) AdminListViolationReports(ctx context.Context, req *pb.AdminListViolationReportsRequest) (*pb.AdminListViolationReportsResponse, error) {
	var sev *domain.ViolationSeverity
	if req.Severity != "" {
		v := domain.ViolationSeverity(req.Severity)
		sev = &v
	}
	var st *domain.ReportStatus
	if req.Status != "" {
		v := domain.ReportStatus(req.Status)
		st = &v
	}
	since := optTimeFromMs(req.SinceMs)
	rows, err := s.violationUC.AdminUnionListReports(ctx, req.Source, sev, st, since, req.Limit, req.Offset)
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminListViolationReportsResponse{Code: c, Message: m}, nil
	}
	out := make([]*pb.UnionReportRow, 0, len(rows))
	for _, r := range rows {
		row := &pb.UnionReportRow{
			Source:      r.Source,
			Severity:    string(r.Severity),
			CreatedAtMs: tsMs(r.CreatedAt),
		}
		if r.SellerReport != nil {
			row.SellerReport = toProtoSellerViolationReport(r.SellerReport)
		}
		if r.FoodSafety != nil {
			row.FoodSafety = toProtoFoodSafetyReport(r.FoodSafety)
		}
		out = append(out, row)
	}
	return &pb.AdminListViolationReportsResponse{Rows: out}, nil
}

// ========== Admin: Patch Status ==========

func (s *SellerServer) AdminPatchSellerViolationReportStatus(ctx context.Context, req *pb.AdminPatchReportStatusRequest) (*pb.AdminPatchReportStatusResponse, error) {
	summary := optString(req.AdminResolutionSummary)
	r, err := s.violationReportUC.AdminPatchStatus(ctx, parseInt64(req.ReportId), domain.ReportStatus(req.Status), summary)
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminPatchReportStatusResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminPatchReportStatusResponse{SellerReport: toProtoSellerViolationReport(r)}, nil
}

func (s *SellerServer) AdminPatchFoodSafetyReportStatus(ctx context.Context, req *pb.AdminPatchReportStatusRequest) (*pb.AdminPatchReportStatusResponse, error) {
	summary := optString(req.AdminResolutionSummary)
	r, err := s.foodSafetyReportUC.AdminPatchStatus(ctx, parseInt64(req.ReportId), domain.ReportStatus(req.Status), summary)
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminPatchReportStatusResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminPatchReportStatusResponse{FoodSafety: toProtoFoodSafetyReport(r)}, nil
}

// ========== Admin: Acknowledge / FDA / NotifySeller ==========

func (s *SellerServer) AdminAcknowledgeFoodSafetyReport(ctx context.Context, req *pb.AdminAcknowledgeRequest) (*pb.AdminAcknowledgeResponse, error) {
	r, err := s.foodSafetyReportUC.AdminAcknowledge(ctx, parseInt64(req.ReportId), req.AdminId)
	if err != nil {
		c, m := errToCode(err)
		resp := &pb.AdminAcknowledgeResponse{Code: c, Message: m}
		// 409 idempotent friendly：r 仍含既有 ack 資訊
		if r != nil {
			resp.Report = toProtoFoodSafetyReport(r)
			resp.AckLatencyMs = r.AckLatency().Milliseconds()
			resp.SlaMet = r.IsAckSLAMet()
		}
		return resp, nil
	}
	return &pb.AdminAcknowledgeResponse{
		Report:       toProtoFoodSafetyReport(r),
		AckLatencyMs: r.AckLatency().Milliseconds(),
		SlaMet:       r.IsAckSLAMet(),
	}, nil
}

func (s *SellerServer) AdminGenerateFDAReport(ctx context.Context, req *pb.AdminGenerateFDAReportRequest) (*pb.AdminGenerateFDAReportResponse, error) {
	md, err := s.foodSafetyReportUC.AdminGenerateFDAExport(ctx, parseInt64(req.ReportId))
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminGenerateFDAReportResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminGenerateFDAReportResponse{Markdown: md}, nil
}

func (s *SellerServer) AdminMarkFDAReported(ctx context.Context, req *pb.AdminMarkFDAReportedRequest) (*pb.AdminMarkFDAReportedResponse, error) {
	r, err := s.foodSafetyReportUC.AdminMarkFDAReported(ctx, parseInt64(req.ReportId), domain.FDAReportChannel(req.FdaReportChannel), req.FdaReferenceNo)
	if err != nil {
		c, m := errToCode(err)
		resp := &pb.AdminMarkFDAReportedResponse{Code: c, Message: m}
		if r != nil {
			resp.Report = toProtoFoodSafetyReport(r)
		}
		return resp, nil
	}
	return &pb.AdminMarkFDAReportedResponse{Report: toProtoFoodSafetyReport(r)}, nil
}

func (s *SellerServer) AdminNotifySeller(ctx context.Context, req *pb.AdminNotifySellerRequest) (*pb.AdminNotifySellerResponse, error) {
	logRow, err := s.foodSafetyReportUC.AdminNotifySeller(ctx, parseInt64(req.ReportId), req.AdminId, req.TemplateKey)
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminNotifySellerResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminNotifySellerResponse{LogId: formatID(logRow.ID)}, nil
}

// ========== Admin: Violations CRUD + Appeal Decide ==========

func (s *SellerServer) AdminCreateViolation(ctx context.Context, req *pb.AdminCreateViolationRequest) (*pb.AdminCreateViolationResponse, error) {
	in := &domain.SellerViolation{
		SellerID:                 parseInt64(req.SellerId),
		SourceSellerReportID:     optParseInt64(req.SourceSellerReportId),
		SourceFoodSafetyReportID: optParseInt64(req.SourceFoodSafetyReportId),
		Category:                 domain.ViolationCategory(req.Category),
		Severity:                 domain.ViolationSeverity(req.Severity),
		PenaltyType:              domain.PenaltyType(req.PenaltyType),
		PenaltyAmountCents:       optInt64(req.PenaltyAmountCents),
		Description:              req.Description,
		PublicSummary:            optString(req.PublicSummary),
		ExpiresAt:                optTimeFromMs(req.ExpiresAtMs),
		CreatedByAdminID:         req.CreatedByAdminId,
	}
	result, err := s.violationUC.AdminCreateViolation(ctx, in)
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminCreateViolationResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminCreateViolationResponse{
		Violation:             toProtoSellerViolation(result.Violation),
		SeverityAutoEscalated: result.SeverityAutoEscalated,
		OriginalSeverity:      string(result.OriginalSeverity),
		AppliedSeverity:       string(result.AppliedSeverity),
	}, nil
}

// AdminGetViolation 對應 spec：admin 決議申訴前查看 violation 詳情（issue #1516）。
// 回應含 appeal_evidence_asset_ids（真 asset_id，非 staged_token），不在此層組 URL
// （見 mapper.go IdCardFrontAssetId 等 secure asset 皆回傳原始 asset_id）；後台沒有通用的
// 機敏檔下載，佐證檔的讀取由擁有者端點授權。
func (s *SellerServer) AdminGetViolation(ctx context.Context, req *pb.AdminGetViolationRequest) (*pb.AdminGetViolationResponse, error) {
	v, err := s.violationUC.AdminGetViolation(ctx, parseInt64(req.Id))
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminGetViolationResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminGetViolationResponse{Violation: toProtoSellerViolation(v)}, nil
}

func (s *SellerServer) AdminUpdateViolation(ctx context.Context, req *pb.AdminUpdateViolationRequest) (*pb.AdminUpdateViolationResponse, error) {
	v, err := s.violationUC.AdminUpdateMutable(ctx, parseInt64(req.Id), req.Description, optString(req.PublicSummary), optTimeFromMs(req.ExpiresAtMs))
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminUpdateViolationResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminUpdateViolationResponse{Violation: toProtoSellerViolation(v)}, nil
}

func (s *SellerServer) AdminRevokeViolation(ctx context.Context, req *pb.AdminRevokeViolationRequest) (*pb.AdminRevokeViolationResponse, error) {
	reason := domain.RevokedReason(req.RevokedReason)
	if reason == "" {
		reason = domain.RevokedReasonOther
	}
	v, err := s.violationUC.AdminRevoke(ctx, parseInt64(req.Id), reason, req.AdminId)
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminRevokeViolationResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminRevokeViolationResponse{Violation: toProtoSellerViolation(v)}, nil
}

func (s *SellerServer) AdminDecideAppeal(ctx context.Context, req *pb.AdminDecideAppealRequest) (*pb.AdminDecideAppealResponse, error) {
	v, err := s.violationUC.AdminDecideAppeal(ctx, parseInt64(req.ViolationId), req.AdminId, req.Accept, req.DecisionReason)
	if err != nil {
		c, m := errToCode(err)
		return &pb.AdminDecideAppealResponse{Code: c, Message: m}, nil
	}
	return &pb.AdminDecideAppealResponse{Violation: toProtoSellerViolation(v)}, nil
}

// ========== silence unused import (placeholder for future imports) ==========

var _ = errors.New
