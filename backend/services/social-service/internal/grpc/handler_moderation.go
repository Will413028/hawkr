package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/pagination"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxEvidenceKeys = 5

// ==================== B-135: Reporting & Blocking ====================

func (s *ContentServer) SubmitReport(ctx context.Context, req *pb.SubmitReportRequest) (*pb.SubmitReportResponse, error) {
	targetID := parseInt64(req.TargetId)
	if req.ReporterId == 0 || targetID == 0 {
		return nil, status.Error(codes.InvalidArgument, "reporter_id and target_id are required")
	}
	if len(req.EvidenceKeys) > maxEvidenceKeys {
		return nil, status.Errorf(codes.InvalidArgument, "max %d evidence keys", maxEvidenceKeys)
	}
	// proto 的空 repeated 是 nil slice，直接 Marshal 會落庫成 JSON `null`，
	// 與 schema.sql 的 `evidence JSONB NOT NULL DEFAULT '[]'` 語意不符
	// （消費端拿 null 得多寫一條分支）。統一正規化成 []。
	evidenceKeys := req.EvidenceKeys
	if evidenceKeys == nil {
		evidenceKeys = []string{}
	}
	evidenceJSON, err := json.Marshal(evidenceKeys)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal evidence: %v", err)
	}
	report, err := s.reportUC.SubmitReport(ctx, domain.CreateReportParams{
		ReporterID:  req.ReporterId,
		TargetType:  req.TargetType,
		TargetID:    targetID,
		ReasonCode:  req.ReasonCode,
		Description: req.Description,
		Evidence:    evidenceJSON,
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidTargetType), errors.Is(err, usecase.ErrInvalidReason):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrReportRateLimited):
			// Expected user behavior, not a service error — don't log.
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		default:
			s.logger.Error("SubmitReport failed", zap.Error(err))
			return nil, status.Error(codes.Internal, "internal error")
		}
	}
	return &pb.SubmitReportResponse{ReportId: strconv.FormatInt(report.ID, 10), Status: report.Status}, nil
}

func (s *ContentServer) ListMyReports(ctx context.Context, req *pb.ListMyReportsRequest) (*pb.ListMyReportsResponse, error) {
	if req.ReporterId == 0 {
		return nil, status.Error(codes.InvalidArgument, "reporter_id is required")
	}
	limit, offset := pageToLimitOffset(req.Page, req.PageSize)
	rows, err := s.reportUC.ListMyReports(ctx, req.ReporterId, limit, offset)
	if err != nil {
		s.logger.Error("ListMyReports failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	out := make([]*pb.Report, len(rows))
	for i, r := range rows {
		out[i] = reportToProto(r)
	}
	return &pb.ListMyReportsResponse{Reports: out}, nil
}

func (s *ContentServer) BlockUser(ctx context.Context, req *pb.BlockUserRequest) (*pb.BlockUserResponse, error) {
	blockedID := parseInt64(req.BlockedId)
	if req.BlockerId == 0 || blockedID == 0 {
		return nil, status.Error(codes.InvalidArgument, "blocker_id and blocked_id are required")
	}
	err := s.blockUC.BlockUser(ctx, req.BlockerId, blockedID)
	switch {
	case err == nil:
		return &pb.BlockUserResponse{}, nil
	case errors.Is(err, usecase.ErrCannotBlockSelf):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, usecase.ErrUnresolvedBlockPrincipal):
		// #2789 fail-closed：至少一端不是現存的 buyer_id / store_id，自我封鎖守門
		// 無從判定。這是請求層的事實（id 不屬於任何身分空間），不是服務故障，
		// 所以走 400 而非 500；usecase 已留 warn log，此處不重複記。
		return nil, status.Error(codes.InvalidArgument, err.Error())
	default:
		s.logger.Error("BlockUser failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}
}

func (s *ContentServer) UnblockUser(ctx context.Context, req *pb.UnblockUserRequest) (*pb.UnblockUserResponse, error) {
	blockedID := parseInt64(req.BlockedId)
	if req.BlockerId == 0 || blockedID == 0 {
		return nil, status.Error(codes.InvalidArgument, "blocker_id and blocked_id are required")
	}
	if err := s.blockUC.UnblockUser(ctx, req.BlockerId, blockedID); err != nil {
		s.logger.Error("UnblockUser failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &pb.UnblockUserResponse{}, nil
}

func (s *ContentServer) ListBlocks(ctx context.Context, req *pb.ListBlocksRequest) (*pb.ListBlocksResponse, error) {
	if req.BlockerId == 0 {
		return nil, status.Error(codes.InvalidArgument, "blocker_id is required")
	}
	limit, offset := pageToLimitOffset(req.Page, req.PageSize)
	rows, err := s.blockUC.ListBlocks(ctx, req.BlockerId, limit, offset)
	if err != nil {
		s.logger.Error("ListBlocks failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	out := make([]*pb.UserBlock, len(rows))
	for i, b := range rows {
		var ms int64
		if b.CreateTime.Valid {
			ms = b.CreateTime.Time.UnixMilli()
		}
		out[i] = &pb.UserBlock{
			Id:           strconv.FormatInt(b.ID, 10),
			BlockedId:    strconv.FormatInt(b.BlockedID, 10),
			CreateTimeMs: ms,
		}
	}
	return &pb.ListBlocksResponse{Blocks: out}, nil
}

func (s *ContentServer) AdminListReports(ctx context.Context, req *pb.AdminListReportsRequest) (*pb.AdminListReportsResponse, error) {
	limit, offset := pageToLimitOffset(req.Page, req.PageSize)
	rows, err := s.reportUC.AdminList(ctx, req.Status, req.TargetType, req.ReasonCode, limit, offset)
	if err != nil {
		s.logger.Error("AdminListReports failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	out := make([]*pb.Report, len(rows))
	for i, r := range rows {
		pbReport := reportToProto(r)
		if r.TargetOwnerID != nil {
			if count, err := s.reportUC.CountRecentActionsByOwner(ctx, *r.TargetOwnerID); err == nil {
				pbReport.RecentActionCount = strconv.FormatInt(count, 10)
			}
		}
		out[i] = pbReport
	}
	return &pb.AdminListReportsResponse{Reports: out}, nil
}

func (s *ContentServer) AdminGetReport(ctx context.Context, req *pb.AdminGetReportRequest) (*pb.AdminGetReportResponse, error) {
	reportID := parseInt64(req.ReportId)
	if reportID == 0 {
		return nil, status.Error(codes.InvalidArgument, "report_id is required")
	}
	row, err := s.reportUC.AdminGet(ctx, reportID)
	if err != nil {
		s.logger.Error("AdminGetReport failed", zap.Error(err))
		return nil, status.Error(codes.NotFound, "report not found")
	}
	return &pb.AdminGetReportResponse{Report: reportToProto(row)}, nil
}

func (s *ContentServer) AdminResolveReport(ctx context.Context, req *pb.AdminResolveReportRequest) (*pb.AdminResolveReportResponse, error) {
	reportID := parseInt64(req.ReportId)
	if reportID == 0 || req.AdminId == 0 || req.Action == "" {
		return nil, status.Error(codes.InvalidArgument, "report_id, admin_id, action are required")
	}
	// HIDE_TARGET / BAN_USER side-effect dispatch to other services
	// (review-service / im-service / account-service) is task 7.x.
	// Here we just transition the report state.
	row, err := s.reportUC.AdminResolve(ctx, reportID, req.Action, req.AdminId, req.Note)
	switch {
	case err == nil:
		return &pb.AdminResolveReportResponse{Report: reportToProto(row)}, nil
	case errors.Is(err, usecase.ErrAlreadyResolved):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, usecase.ErrUnknownTargetOwner):
		// Not an internal failure and not retryable-by-retrying: this report
		// simply has nobody to ban (REVIEW / CHAT_MESSAGE, or a legacy row).
		// FailedPrecondition so the admin UI shows the reason instead of a
		// generic 500 the operator would read as "the platform is broken".
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	default:
		s.logger.Error("AdminResolveReport failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}
}

func (s *ContentServer) AdminReopenReport(ctx context.Context, req *pb.AdminReopenReportRequest) (*pb.AdminReopenReportResponse, error) {
	reportID := parseInt64(req.ReportId)
	if reportID == 0 || req.AdminId == 0 {
		return nil, status.Error(codes.InvalidArgument, "report_id and admin_id are required")
	}
	row, err := s.reportUC.Reopen(ctx, reportID, req.AdminId, req.Note)
	if err != nil {
		s.logger.Error("AdminReopenReport failed", zap.Error(err))
		return nil, status.Error(codes.NotFound, "report not found or not in a reopenable state")
	}
	return &pb.AdminReopenReportResponse{Report: reportToProto(row)}, nil
}

// ==================== helpers ====================

func pageToLimitOffset(page, pageSize int32) (int64, int64) {
	_, pageSize, offset := pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxModerationPageSize)
	return int64(pageSize), int64(offset)
}

func reportToProto(r *gen.Report) *pb.Report {
	out := &pb.Report{
		Id:         strconv.FormatInt(r.ID, 10),
		ReporterId: strconv.FormatInt(r.ReporterID, 10),
		TargetType: r.TargetType,
		TargetId:   strconv.FormatInt(r.TargetID, 10),
		ReasonCode: r.ReasonCode,
		Status:     r.Status,
	}
	if r.Description != nil {
		out.Description = *r.Description
	}
	if r.ResolvedAction != nil {
		out.ResolvedAction = *r.ResolvedAction
	}
	if r.ResolvedBy != nil {
		out.ResolvedBy = *r.ResolvedBy
	}
	if r.ResolvedAt != nil {
		out.ResolvedAtMs = r.ResolvedAt.UnixMilli()
	}
	if r.CreateTime.Valid {
		out.CreateTimeMs = r.CreateTime.Time.UnixMilli()
	}
	return out
}
