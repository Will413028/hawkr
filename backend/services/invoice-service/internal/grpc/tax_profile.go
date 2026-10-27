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

// 本檔是 M3 稅務資格狀態機寫入面的 Layer 5：賣家側 §6.1.7 的端點 18／19／20／22，
// ADM 側 §6.1.12 的端點 40／41。讀取面（端點 17／21／39）在 query.go。
//
// 四條規則：
//
//	① **業務錯誤走 envelope、故障走 gRPC error**。GetCode() 是 Gateway 的
//	   EnvelopeGuardInterceptor 唯一的判定依據——回 gRPC error 的那些會變成 5xx，
//	   而「你的狀態不允許送審」不是故障。
//	② **Tier A 的回應必須與「不存在」逐位元相同**：四支賣家 RPC 在越權（別人的
//	   store_id）與查無時都回同一個 9320 envelope，因為那個判定發生在
//	   `invoice_issuers.issuer_ref_id` 的 WHERE，本層沒有第二個分支可以回不同的東西。
//	③ **必填／長度／值域是契約違規不是業務結果** ⇒ 回 InvalidArgument（400），不借
//	   業務碼。§6.1.12 對 confirmed 缺席逐字要求 400 且**不得用 9348**（那是狀態不相容，
//	   語意不同：一個要再按一次確認、一個要換個決策）。
//	④ **9324 是唯一帶 data 的非零碼**（端點 19 的 7 天冷卻，preserveData／HTTP 200）：
//	   前端要靠 data.reapplyCooldownUntil 顯示倒數。少了 data，賣家只看到一句紅字而
//	   不知道什麼時候可以再試。
//
// ⚠️ **端點 18 沒有「非零碼 ＋ data」的出口**：gateway 的 SaveSellerTaxProfileDraft
// handler 無條件走 response.OK（不看 resp.GetCode()）⇒ preserveData 類的碼在那條路上
// 會被寫成 code 0。所以 SaveDraft 的冷卻分支回 9321 而不是 9324——它是端點 18 碼表
// （9321／9322）之內的值，且冷卻到期時刻本來就在 S02 讀得到的 reapplyCooldownUntil 上。

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
		// 裁決 C：條款勾選與送審是同一個動作。未勾 ⇒ 9325（envelope，HTTP 409），
		// 不是 InvalidArgument——賣家的下一步是「去讀條款並勾選」，前端要據此
		// 就地彈出條款連結（58A §S02 的 9325 列）。
		AcceptedTerms: req.GetAcceptedTerms(),
		// 簽署的自然人。**不 requirePrincipal**：它缺席時要記成「查不到簽署人」
		// 而不是讓整支送審 InvalidArgument——舊版 client 還在線時那會直接鎖死送審。
		// Gateway 已在入口擋掉無 account 的請求（401），這裡是縱深防禦不是唯一防線。
		AcceptedByAccountID: req.GetAcceptedByAccountId(),
	})
	if err != nil {
		return s.taxProfileErr(ctx, "invoice.tax_profile.submit.failed", err)
	}
	if res.CooldownBlocked {
		// 9324 ＋ 當前單筆。code 非零但 data 必須在（envelopeCodePreserveData，HTTP 200）。
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
		ActorAccountID:     actor,
		StoreID:            storeID,
		Decision:           req.GetDecision(),
		Reason:             req.GetReason(),
		ApprovalValidUntil: validUntil,
		Confirmed:          req.GetConfirmed(),
		ExpectedVersion:    version,
		// §6.1.9：核定免用統一發票時由 ADM 指名收據代製授權的核定依據文件。
		// 兩欄同進同出、且必須是該賣家自己上傳的那幾份之一——驗證在 usecase。
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
	// extend_days 缺席 ⇒ 0，由 usecase 折成預設的 30（§6.1.12 逐字）。
	// 在這裡就折會讓「沒帶」與「帶了 30」在 usecase 端不可分辨，而預設值日後要改時
	// 會有兩處。
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

// ---------------------------------------------------------------------------
// 型別邊界與錯誤映射
// ---------------------------------------------------------------------------

// parseExpectedVersion 解析樂觀鎖版本。
//
// 🔴 空字串 ⇒ 0 **而不是錯誤**：新建的 seller_tax_profiles 版本就是 0（DDL DEFAULT），
// 前端第一次存草稿時沒有版本可帶。負數是契約違規（version 有 chk_stp_counters_nonneg）。
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

// parseOptionalCount 解析可缺席的非負計數（extend_days）。
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

// parseOptionalISO8601 解析可缺席的 ISO 8601 UTC 時間（approval_valid_until）。
//
// 只收 RFC3339：收寬一點（例如同時接受 "YYYY-MM-DD"）會讓「2027-01-01」被讀成
// UTC 午夜，而 ADM 想的是台北時間的那一天——8 小時的差在核定效期上不會有人發現。
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

// toTaxDecisionData 折出端點 40／41 共用的回應。
func toTaxDecisionData(res usecase.TaxDecisionResult) *pb.DecideTaxReviewForAdminData {
	return &pb.DecideTaxReviewForAdminData{
		StoreId:      fmtID(res.StoreID),
		ReviewStatus: res.ReviewStatus,
		Version:      fmtInt64(res.Version),
		// audit_log_id 是 bigserial（不是 TSID）⇒ 用 fmtID 讓 0 變空字串，
		// 而不是回一個看起來像 id 的 "0"。
		AuditLogId: fmtID(res.AuditLogID),
		// 只有 REVOKE 有值（T-C04 的分界時點）；其餘決策省略。
		RevokedEffectiveAt: fmtTime(res.RevokedEffectiveAt),
	}
}

// taxProfileErr 把 usecase 的錯誤折成「賣家側四支」的回應。
func (s *InvoiceServer) taxProfileErr(ctx context.Context, event string, err error) (*pb.SellerTaxProfileResponse, error) {
	if st, ok := taxInvalidArgument(err); ok {
		return nil, st
	}
	if code, msg, ok := bizOf(err); ok {
		return &pb.SellerTaxProfileResponse{Code: code, Message: msg}, nil
	}
	return nil, s.internal(ctx, event, err)
}

// taxDecisionErr 把 usecase 的錯誤折成「ADM 側兩支」的回應。
func (s *InvoiceServer) taxDecisionErr(ctx context.Context, event string, err error) (*pb.DecideTaxReviewForAdminResponse, error) {
	if st, ok := taxInvalidArgument(err); ok {
		return nil, st
	}
	if code, msg, ok := bizOf(err); ok {
		return &pb.DecideTaxReviewForAdminResponse{Code: code, Message: msg}, nil
	}
	return nil, s.internal(ctx, event, err)
}

// taxInvalidArgument 把契約違規映成 InvalidArgument。
//
// 🔴 清單是白名單不是黑名單：漏掉一個具名錯誤的後果是它掉進 s.internal ⇒ 500，
// 而賣家看到的是「系統錯誤」而不是「請至少上傳一份稅籍證明文件」。
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
