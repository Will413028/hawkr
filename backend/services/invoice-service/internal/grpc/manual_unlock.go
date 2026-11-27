package grpc

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


// RequestManualUnlockForAdmin 是 ADM 申請人工出口解鎖（端點 54）。
func (s *InvoiceServer) RequestManualUnlockForAdmin(
	ctx context.Context, req *pb.RequestManualUnlockForAdminRequest,
) (*pb.ManualUnlockResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	jobID, err := parseID(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "job_id: %v", err)
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}
	checkedAt, err := parseISO8601(req.GetProviderCheckedAt())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "provider_checked_at: %v", err)
	}

	view, err := s.adminJobs.RequestManualUnlock(ctx, usecase.RequestManualUnlockInput{
		ActorAccountID:        actor,
		JobID:                 jobID,
		ProviderCheckEvidence: req.GetProviderCheckEvidence(),
		ProviderCheckedAt:     checkedAt,
		Reason:                req.GetReason(),
		ExpectedVersion:       version,
		IdempotencyKey:        req.GetIdempotencyKey(),
		CancelEvidenceKind:      req.GetCancelEvidenceKind(),
		CancelEvidenceObjectKey: req.GetCancelEvidenceObjectKey(),
		CancelEvidenceStagedToken: req.GetCancelEvidenceStagedToken(),
	})
	if err != nil {
		return s.manualUnlockError(ctx, "invoice.admin.manual_unlock.request.failed", err)
	}

	if view.NeedsSecondApproval {
		return &pb.ManualUnlockResponse{
			Code:    bizcode.InvoiceManualUnlockNeedsSecondApproval,
			Message: bizcode.Message(bizcode.InvoiceManualUnlockNeedsSecondApproval),
			Data:    toManualUnlockInfo(view),
		}, nil
	}
	return &pb.ManualUnlockResponse{Code: 0, Message: msgSuccess, Data: toManualUnlockInfo(view)}, nil
}

// ApproveManualUnlockForAdmin 是 ADM 覆核人工出口解鎖（端點 55）。
//
// approved=false 同樣是**成功回應**（code 0，data.status = REJECTED）：駁回是一個正常的
// 業務結果，不是錯誤。把它回成 4xx 會讓 ADM 前端把「我剛剛駁回了」顯示成「駁回失敗」。
func (s *InvoiceServer) ApproveManualUnlockForAdmin(
	ctx context.Context, req *pb.ApproveManualUnlockForAdminRequest,
) (*pb.ManualUnlockResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	jobID, err := parseID(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "job_id: %v", err)
	}
	unlockID, err := parseID(req.GetUnlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unlock_id: %v", err)
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}

	view, err := s.adminJobs.ApproveManualUnlock(ctx, usecase.ApproveManualUnlockInput{
		ActorAccountID:  actor,
		JobID:           jobID,
		UnlockID:        unlockID,
		Approved:        req.GetApproved(),
		Reason:          req.GetReason(),
		ExpectedVersion: version,
		IdempotencyKey:  req.GetIdempotencyKey(),
	})
	if err != nil {
		return s.manualUnlockError(ctx, "invoice.admin.manual_unlock.approve.failed", err)
	}
	return &pb.ManualUnlockResponse{Code: 0, Message: msgSuccess, Data: toManualUnlockInfo(view)}, nil
}

// ListProviderCutoversForAdmin 是 ADM 自動開立切點歷史（端點 46）。
func (s *InvoiceServer) ListProviderCutoversForAdmin(
	ctx context.Context, req *pb.ListProviderCutoversForAdminRequest,
) (*pb.ListProviderCutoversForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListProviderCutoversForAdmin(ctx, usecase.AdminProviderCutoversInput{
		StoreID:  storeID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListProviderCutoversForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_provider_cutovers.failed", err)
	}

	records := make([]*pb.ProviderCutoverItem, 0, len(out.Records))
	for _, row := range out.Records {
		records = append(records, &pb.ProviderCutoverItem{
			Version:   fmtInt64(row.AccountVersion),
			CutoverAt: fmtTimeValue(row.CutoverAt),
			DeactivatedAt:      fmtTime(row.DeactivatedAt),
			Mode:               row.Mode,
			CreatedByAccountId: fmtInt64(row.CreatedByAccountID),
			CreatedAt:          fmtTimeValue(row.CreatedAt),
			Reason:             str(row.Reason),
		})
	}
	return &pb.ListProviderCutoversForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListProviderCutoversForAdminData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

func toManualUnlockInfo(v usecase.ManualUnlockView) *pb.ManualUnlockInfo {
	return &pb.ManualUnlockInfo{
		UnlockId:                fmtInt64(v.UnlockID),
		Status:                  v.Status,
		RequestedByAccountId:    fmtInt64(v.RequestedByAccountID),
		ApprovedByAccountId:     fmtIDPtr(v.ApprovedByAccountID),
		ProviderCheckEvidence:   v.ProviderCheckEvidence,
		ProviderCheckedAt:       fmtTimeValue(v.ProviderCheckedAt),
		RequestedAt:             fmtTimeValue(v.RequestedAt),
		ExpiresAt:               fmtTimeValue(v.ExpiresAt),
		CancelEvidenceKind:      str(v.CancelEvidenceKind),
		CancelEvidenceObjectKey: str(v.CancelEvidenceObjectKey),
	}
}

func (s *InvoiceServer) manualUnlockError(ctx context.Context, event string, err error) (*pb.ManualUnlockResponse, error) {
	if st, ok := unlockInvalidArgument(err); ok {
		return nil, st
	}
	if code, msg, ok := bizOf(err); ok {
		return &pb.ManualUnlockResponse{Code: code, Message: msg}, nil
	}
	return nil, s.internal(ctx, event, err)
}

func unlockInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrUnlockEvidenceKindRequired),
		errors.Is(err, usecase.ErrUnlockEvidenceKindInvalid),
		errors.Is(err, usecase.ErrUnlockEvidenceKindUnwanted),
		errors.Is(err, usecase.ErrUnlockEvidenceKeyOrphan),
		errors.Is(err, usecase.ErrUnlockEvidenceRequired),
		errors.Is(err, usecase.ErrUnlockEvidenceTooLong),
		errors.Is(err, usecase.ErrUnlockCheckedAtRequired),
		errors.Is(err, usecase.ErrUnlockCheckedAtFuture),
		errors.Is(err, usecase.ErrJobReasonRequired),
		errors.Is(err, usecase.ErrJobReasonTooLong):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}
