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

// 本檔是 A4 受控人工出口的雙人解鎖（§6.1.14 端點 54／55）與 A09 自動開立切點歷史
// （§6.1.13 端點 46）的 Layer 5。
//
// 與 admin_write.go 同三條規則（業務錯誤走 envelope、帶 data 的非零碼必須真的帶 data、
// 長度／必填／格式回 InvalidArgument），另加一條只有本檔有的：
//
//	🔴 **9344 是 preserveData 碼，HTTP 恆 200**（§6.1.0(13)）。端點 54 撞到「已經有一個
//	   等待覆核的請求」時回 9344 ＋ 既有那一列的 unlockId，讓 ADM 前端直接導去覆核對話框。
//	   回 code 0 會讓 ADM 以為自己剛剛發起了一個新請求（實際上什麼都沒建立），
//	   而回 4xx 則拿不到那個 unlockId ⇒ A4 走不下去。

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
		// J28 證據。空字串＝未帶（與本 message 其餘選填欄位同慣例）；
		// 「該帶而沒帶」由 usecase 依 job 狀態判，不在這一層。
		CancelEvidenceKind:      req.GetCancelEvidenceKind(),
		CancelEvidenceObjectKey: req.GetCancelEvidenceObjectKey(),
		// #3108：ADM 上傳證據檔後拿到的 token，與 objectKey 二擇一（usecase 擋兩者並存）。
		CancelEvidenceStagedToken: req.GetCancelEvidenceStagedToken(),
	})
	if err != nil {
		return s.manualUnlockError(ctx, "invoice.admin.manual_unlock.request.failed", err)
	}

	// 9344 走 preserveData：code 非零但 data 必須帶（unlockId 是 ADM 導頁的唯一線索）。
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
			// 空字串 ＝ 至今仍有效（proto3 沒有 null）。前端據此區分「現行切點」與歷史列。
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

// toManualUnlockInfo 把 usecase 的視圖折成 proto。
//
// Status 已在 usecase 投影過（逾期 ⇒ EXPIRED），本層不再判一次：判兩次就有兩個真相，
// 而它們用的 now 不同。
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

// manualUnlockError 把 usecase 的錯誤分成三類：契約違規（400）、業務碼（envelope）、故障。
func (s *InvoiceServer) manualUnlockError(ctx context.Context, event string, err error) (*pb.ManualUnlockResponse, error) {
	if st, ok := unlockInvalidArgument(err); ok {
		return nil, st
	}
	if code, msg, ok := bizOf(err); ok {
		return &pb.ManualUnlockResponse{Code: code, Message: msg}, nil
	}
	return nil, s.internal(ctx, event, err)
}

// unlockInvalidArgument 把本流程的型別邊界錯誤映成 400。
//
// 它們刻意不是業務碼：沒有任何 9300 段的碼描述「查證說明太長」或「查證時間在未來」，
// 借一個近似的碼會讓前端把格式問題顯示成別的意思。
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
