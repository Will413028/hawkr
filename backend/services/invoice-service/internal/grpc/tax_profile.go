package grpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SaveSellerTaxProfileDraft 儲存稅務資格草稿（端點 18 ＝ T01 ＋ 草稿存檔）。
func (s *InvoiceServer) SaveSellerTaxProfileDraft(
	ctx context.Context, req *pb.SaveSellerTaxProfileDraftRequest,
) (*pb.SellerTaxProfileResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	version, err := parseExpectedVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	id := req.GetTaxIdentity()

	view, err := s.taxProfiles.SaveDraft(ctx, usecase.SaveTaxDraftInput{
		StoreID:              storeID,
		ExpectedVersion:      version,
		EntityType:           id.GetEntityType(),
		TaxID:                id.GetTaxId(),
		LegalName:            id.GetLegalName(),
		ResponsibleName:      id.GetResponsibleName(),
		AddressCity:          id.GetAddressCity(),
		AddressDistrict:      id.GetAddressDistrict(),
		AddressDetail:        id.GetAddressDetail(),
		ClaimedStatus:        id.GetClaimedStatus(),
		EvidenceObjectKeys:   req.GetEvidenceObjectKeys(),
		EvidenceStagedTokens: req.GetEvidenceStagedTokens(),
		SavedByAccountID:     req.GetSavedByAccountId(),
	})
	if err != nil {
		return s.taxProfileErr(ctx, "invoice.tax_profile.save_draft.failed", err)
	}
	return &pb.SellerTaxProfileResponse{
		Code: 0, Message: msgSuccess, Data: s.toSellerTaxProfileData(ctx, view),
	}, nil
}

// SubmitSellerTaxProfile 送出稅務資格審核（端點 19 ＝ T02，必要時先走 T01 重申）。
//
// accepted_terms=false（含未帶）⇒ **9325**，且一筆都不寫。9325 走 envelope 不走
// gRPC error：它不是故障，而且前端要靠它把賣家導去條款（§6.1.7 端點 19 失敗分支）。
func (s *InvoiceServer) SubmitSellerTaxProfile(
	ctx context.Context, req *pb.SubmitSellerTaxProfileRequest,
) (*pb.SellerTaxProfileResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	version, err := parseExpectedVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}

	res, err := s.taxProfiles.Submit(ctx, usecase.SubmitTaxProfileInput{
		StoreID: storeID, ExpectedVersion: version,
		AcceptedTerms:       req.GetAcceptedTerms(),
		AcceptedByAccountID: req.GetAcceptedByAccountId(),
	})
	if err != nil {
		return s.taxProfileErr(ctx, "invoice.tax_profile.submit.failed", err)
	}
	if res.CooldownBlocked {
		return &pb.SellerTaxProfileResponse{
			Code:    bizcode.SellerTaxReapplyCooldown,
			Message: bizcode.Message(bizcode.SellerTaxReapplyCooldown),
			Data:    s.toSellerTaxProfileData(ctx, res.View),
		}, nil
	}
	return &pb.SellerTaxProfileResponse{
		Code: 0, Message: msgSuccess, Data: s.toSellerTaxProfileData(ctx, res.View),
	}, nil
}

// RequestSellerTaxExtension 賣家申請延長待審期限（端點 20）。
//
// ⚠️ 它**不改任何狀態**：真正的延長是 ADM 的 T11（端點 41）。回應仍是完整單筆，
// 前端據此覆蓋 query cache（extensionCount 不變是預期的，不是沒生效）。
func (s *InvoiceServer) RequestSellerTaxExtension(
	ctx context.Context, req *pb.RequestSellerTaxExtensionRequest,
) (*pb.SellerTaxProfileResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	version, err := parseExpectedVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}

	view, err := s.taxProfiles.RequestExtension(ctx, usecase.RequestTaxExtensionInput{
		StoreID: storeID, ExpectedVersion: version, Reason: req.GetReason(),
		RequestedByAccountID: req.GetRequestedByAccountId(),
	})
	if err != nil {
		return s.taxProfileErr(ctx, "invoice.tax_profile.request_extension.failed", err)
	}
	return &pb.SellerTaxProfileResponse{
		Code: 0, Message: msgSuccess, Data: s.toSellerTaxProfileData(ctx, view),
	}, nil
}

// AcceptSelfIssueAttestation 接受自行開票責任條款（端點 22 ＝ T04）。
//
// accepted_by_account_id 是**登入 principal**（§4 例外①，Gateway 注入）：接受條款是
// 自然人的意思表示，稽核要能回答「是誰按的」。它可以缺席（0）而不是錯誤——賣家換手後
// store_id 會指向新任賣家，而條款是前任那個自然人簽的，缺席時只是留不下那個事實。
func (s *InvoiceServer) AcceptSelfIssueAttestation(
	ctx context.Context, req *pb.AcceptSelfIssueAttestationRequest,
) (*pb.SelfIssueAttestationResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}

	view, err := s.taxProfiles.AcceptAttestation(ctx, usecase.AcceptAttestationInput{
		StoreID:             storeID,
		AcceptedByAccountID: req.GetAcceptedByAccountId(),
		TermsVersion:        req.GetTermsVersion(),
		Accepted:            req.GetAccepted(),
	})
	if err != nil {
		if st, ok := taxInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.SelfIssueAttestationResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.tax_profile.accept_attestation.failed", err)
	}
	return &pb.SelfIssueAttestationResponse{
		Code: 0, Message: msgSuccess, Data: toSelfIssueAttestation(view.Profile),
	}, nil
}

// DecideTaxReviewForAdmin 是 ADM 的稅務審核裁決（端點 40 ＝ T05／T07／T08／T09／T13／T14／T16）。
func (s *InvoiceServer) DecideTaxReviewForAdmin(
	ctx context.Context, req *pb.DecideTaxReviewForAdminRequest,
) (*pb.DecideTaxReviewForAdminResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	version, err := parseExpectedVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	validUntil, err := parseOptionalISO8601(req.GetApprovalValidUntil())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "approval_valid_until: %v", err)
	}

	res, err := s.taxProfiles.Decide(ctx, usecase.TaxDecisionInput{
		ActorAccountID:       actor,
		StoreID:              storeID,
		Decision:             req.GetDecision(),
		Reason:               req.GetReason(),
		ApprovalValidUntil:   validUntil,
		Confirmed:            req.GetConfirmed(),
		ExpectedVersion:      version,
		ApprovalBasisKey:     req.GetApprovalBasisKey(),
		ApprovalBasisAssetID: req.GetApprovalBasisAssetId(),
	})
	if err != nil {
		return s.taxDecisionErr(ctx, "invoice.tax_profile.decide.failed", err)
	}
	return &pb.DecideTaxReviewForAdminResponse{
		Code: 0, Message: msgSuccess, Data: toTaxDecisionData(res),
	}, nil
}

// DecideTaxExtensionForAdmin 是 ADM 的延長裁決（端點 41 ＝ T11）。
func (s *InvoiceServer) DecideTaxExtensionForAdmin(
	ctx context.Context, req *pb.DecideTaxExtensionForAdminRequest,
) (*pb.DecideTaxReviewForAdminResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	version, err := parseExpectedVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	days, err := parseOptionalCount(req.GetExtendDays())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "extend_days: %v", err)
	}

	res, err := s.taxProfiles.DecideExtension(ctx, usecase.TaxExtensionDecisionInput{
		ActorAccountID:  actor,
		StoreID:         storeID,
		Decision:        req.GetDecision(),
		ExtendDays:      days,
		Reason:          req.GetReason(),
		ExpectedVersion: version,
	})
	if err != nil {
		return s.taxDecisionErr(ctx, "invoice.tax_profile.decide_extension.failed", err)
	}
	return &pb.DecideTaxReviewForAdminResponse{
		Code: 0, Message: msgSuccess, Data: toTaxDecisionData(res),
	}, nil
}

func parseExpectedVersion(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, status.Errorf(codes.InvalidArgument, "expected_version: must be a non-negative integer")
	}
	return v, nil
}

func parseOptionalCount(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		return 0, errNonPositiveID
	}
	return v, nil
}

func parseOptionalISO8601(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	utc := t.UTC()
	return &utc, nil
}

func toTaxDecisionData(res usecase.TaxDecisionResult) *pb.DecideTaxReviewForAdminData {
	return &pb.DecideTaxReviewForAdminData{
		StoreId:            fmtID(res.StoreID),
		ReviewStatus:       res.ReviewStatus,
		Version:            fmtInt64(res.Version),
		AuditLogId:         fmtID(res.AuditLogID),
		RevokedEffectiveAt: fmtTime(res.RevokedEffectiveAt),
	}
}

func (s *InvoiceServer) taxProfileErr(ctx context.Context, event string, err error) (*pb.SellerTaxProfileResponse, error) {
	if st, ok := taxInvalidArgument(err); ok {
		return nil, st
	}
	if code, msg, ok := bizOf(err); ok {
		return &pb.SellerTaxProfileResponse{Code: code, Message: msg}, nil
	}
	return nil, s.internal(ctx, event, err)
}

func (s *InvoiceServer) taxDecisionErr(ctx context.Context, event string, err error) (*pb.DecideTaxReviewForAdminResponse, error) {
	if st, ok := taxInvalidArgument(err); ok {
		return nil, st
	}
	if code, msg, ok := bizOf(err); ok {
		return &pb.DecideTaxReviewForAdminResponse{Code: code, Message: msg}, nil
	}
	return nil, s.internal(ctx, event, err)
}

func taxInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrTaxEvidenceRequired),
		errors.Is(err, usecase.ErrTaxEvidenceTooMany),
		errors.Is(err, usecase.ErrTaxEvidenceNotOwned),
		errors.Is(err, usecase.ErrTaxIdentityIncomplete),
		errors.Is(err, usecase.ErrTaxReasonRequired),
		errors.Is(err, usecase.ErrTaxReasonTooLong),
		errors.Is(err, usecase.ErrTaxTermsVersionInvalid),
		errors.Is(err, usecase.ErrTaxTermsVersionStale),
		errors.Is(err, usecase.ErrTaxTermsNotAccepted),
		errors.Is(err, usecase.ErrTaxDecisionUnknown),
		errors.Is(err, usecase.ErrTaxConfirmRequired),
		errors.Is(err, usecase.ErrTaxExtendDaysInvalid),
		errors.Is(err, usecase.ErrTaxValidUntilInvalid),
		errors.Is(err, usecase.ErrTaxLegalNameTooLong),
		errors.Is(err, usecase.ErrTaxIDInvalid),
		errors.Is(err, usecase.ErrTaxEntityTypeInvalid),
		errors.Is(err, usecase.ErrTaxResponsibleNameTooLong),
		errors.Is(err, usecase.ErrTaxAddressPartTooLong),
		errors.Is(err, usecase.ErrTaxApprovalBasisIncomplete),
		errors.Is(err, usecase.ErrTaxApprovalBasisNotExempt),
		errors.Is(err, usecase.ErrTaxApprovalBasisNotOwned),
		errors.Is(err, usecase.ErrTaxApprovalBasisAssetInvalid):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}
