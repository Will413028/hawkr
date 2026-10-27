package grpc

import (
	"context"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔是 ADM 工作佇列與申訴裁決寫入面的 Layer 5（§6.1.14 端點 52／53／56、
// §6.1.15 端點 57、§6.1.16 端點 60）。
//
// 三條規則（與 complaint.go 同源）：
//
//	① **業務錯誤走 envelope、故障走 gRPC error**。GetCode() 是 Gateway 的
//	   EnvelopeGuardInterceptor 唯一的判定依據。
//	② **帶 data 的非零碼必須真的帶 data**：9342（取消被回絕）與 9347（重複開票）
//	   走 §6.1.0(13) 的 preserveData，HTTP 恆 200。少了 data，前端拿不到
//	   lastProviderCheckAt／documentId，只能顯示一句紅字。
//	③ **長度／必填／格式是契約違規不是業務結果** ⇒ 回 InvalidArgument（400），
//	   不借業務碼：借一個近似的碼會讓前端把「號碼格式不對」顯示成別的意思。

// RetryInvoiceJobForAdmin 是 ADM 安全重試（端點 52）。
func (s *InvoiceServer) RetryInvoiceJobForAdmin(
	ctx context.Context, req *pb.RetryInvoiceJobForAdminRequest,
) (*pb.InvoiceJobMutationResponse, error) {
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

	return s.jobMutation(ctx, "invoice.admin.job.retry.failed", func() (usecase.JobMutationResult, error) {
		return s.adminJobs.Retry(ctx, usecase.RetryJobInput{
			ActorAccountID:  actor,
			JobID:           jobID,
			Reason:          req.GetReason(),
			ExpectedVersion: version,
			IdempotencyKey:  req.GetIdempotencyKey(),
			FundingDecision: req.GetFundingDecision(),
		})
	})
}

// CancelInvoiceJobForAdmin 是 ADM 受控取消（端點 53）。
func (s *InvoiceServer) CancelInvoiceJobForAdmin(
	ctx context.Context, req *pb.CancelInvoiceJobForAdminRequest,
) (*pb.InvoiceJobMutationResponse, error) {
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

	return s.jobMutation(ctx, "invoice.admin.job.cancel.failed", func() (usecase.JobMutationResult, error) {
		return s.adminJobs.Cancel(ctx, usecase.CancelJobInput{
			ActorAccountID:               actor,
			JobID:                        jobID,
			ReasonCode:                   req.GetReasonCode(),
			ReasonDetail:                 req.GetReasonDetail(),
			AllowManualAfterCancellation: req.GetAllowManualAfterCancellation(),
			ExpectedVersion:              version,
			IdempotencyKey:               req.GetIdempotencyKey(),
		})
	})
}

// ResumeInvoiceDocumentForAdmin 是 ADM 把「停在人工出口且零 operation」的文件送回
// 自動線（端點 73）。
//
// 🔴 以 **document_id** 為鍵而不是 job_id：這條路徑上還沒有 job，那正是它存在的理由
// （見 usecase.ResumeDocumentFromManual 的紅字）。回應共用 InvoiceJobMutationResponse
// ——它的產出就是一件新的 job，ADM 拿 job_id 直接導去 A06。
func (s *InvoiceServer) ResumeInvoiceDocumentForAdmin(
	ctx context.Context, req *pb.ResumeInvoiceDocumentForAdminRequest,
) (*pb.InvoiceJobMutationResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}

	return s.jobMutation(ctx, "invoice.admin.document.resume.failed", func() (usecase.JobMutationResult, error) {
		return s.adminJobs.ResumeDocumentFromManual(ctx, usecase.ResumeDocumentInput{
			ActorAccountID:  actor,
			DocumentID:      documentID,
			Reason:          req.GetReason(),
			ExpectedVersion: version,
			IdempotencyKey:  req.GetIdempotencyKey(),
			FundingDecision: req.GetFundingDecision(),
		})
	})
}

// ReportManualResultForAdmin 是 ADM 代錄賣家自行開立的結果（端點 56）。
func (s *InvoiceServer) ReportManualResultForAdmin(
	ctx context.Context, req *pb.ReportManualResultForAdminRequest,
) (*pb.InvoiceJobMutationResponse, error) {
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

	return s.jobMutation(ctx, "invoice.admin.job.manual_result.failed", func() (usecase.JobMutationResult, error) {
		return s.adminJobs.ReportManualResult(ctx, usecase.ReportManualResultInput{
			ActorAccountID:  actor,
			JobID:           jobID,
			InvoiceNo:       req.GetInvoiceNo(),
			InvoiceDate:     req.GetInvoiceDate(),
			TotalAmount:     req.GetTotalAmount(),
			Note:            req.GetNote(),
			ExpectedVersion: version,
			IdempotencyKey:  req.GetIdempotencyKey(),
		})
	})
}

// jobMutation 是三支 job 端點共用的錯誤分流與回應組裝。
//
// 🔴 Blocked（9342）走 **HTTP 200 ＋ preserveData**：那不是錯誤，是「現在還不能取消」，
// 而 data 帶的三個欄位就是 ADM 的下一步。把它折成 error 會讓前端只看得到一句紅字。
func (s *InvoiceServer) jobMutation(
	ctx context.Context, event string, run func() (usecase.JobMutationResult, error),
) (*pb.InvoiceJobMutationResponse, error) {
	res, err := run()
	if err != nil {
		if st, ok := adminInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceJobMutationResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, event, err)
	}
	data := &pb.InvoiceJobMutationData{
		JobId:   fmtInt64(res.JobID),
		Status:  res.Status,
		Version: fmtInt64(res.Version),
		// audit_log_id 目前是 InsertInvoiceAdminAuditLog 的 rows 值而不是新列的 id
		// （該 query 是 :execrows）。缺口登記在交付回報；此處照實帶出，不自造一個假 id。
		AuditLogId: fmtID(res.AuditLogID),
	}
	if res.Blocked {
		data.LastProviderCheckAt = fmtTime(res.LastProviderCheckAt)
		data.ProviderRequestId = res.ProviderRequestID
		data.NextStep = res.NextStep
		return &pb.InvoiceJobMutationResponse{
			Code:    bizcode.InvoiceJobCancelBlockedUnknown,
			Message: bizcode.Message(bizcode.InvoiceJobCancelBlockedUnknown),
			Data:    data,
		}, nil
	}
	return &pb.InvoiceJobMutationResponse{Code: 0, Message: msgSuccess, Data: data}, nil
}

// CreateManualIssuanceForAdmin 是 ADM 手動開立發票（端點 57）。
func (s *InvoiceServer) CreateManualIssuanceForAdmin(
	ctx context.Context, req *pb.CreateManualIssuanceForAdminRequest,
) (*pb.CreateManualIssuanceForAdminResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	in, err := manualIssuanceInput(actor, req)
	if err != nil {
		return nil, err
	}

	res, err := s.adminIssuance.Create(ctx, in)
	if err != nil {
		if st, ok := adminInvalidArgument(err); ok {
			return nil, st
		}
		if errors.Is(err, usecase.ErrIssuanceOrderSnapshotUnavailable) {
			// 能力缺口不是業務結果：回 Unimplemented 而不是一個看起來成功的空回應，
			// 也不借 9346（那會告訴 ADM「這個組合不合法」，而它其實合法、只是還沒接線）。
			return nil, status.Errorf(codes.Unimplemented, "%v", err)
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.CreateManualIssuanceForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.admin.issuance.create.failed", err)
	}

	data := &pb.CreateManualIssuanceForAdminData{
		JobId:      fmtID(res.JobID),
		DocumentId: fmtInt64(res.DocumentID),
		Status:     res.Status,
		AuditLogId: fmtID(res.AuditLogID),
	}
	if res.Duplicate {
		// 9347 ＋ 既有 documentId（preserveData）：前端導向那一份，不是再開一張。
		return &pb.CreateManualIssuanceForAdminResponse{
			Code: bizcode.InvoiceDuplicate, Message: bizcode.Message(bizcode.InvoiceDuplicate), Data: data,
		}, nil
	}
	return &pb.CreateManualIssuanceForAdminResponse{Code: 0, Message: msgSuccess, Data: data}, nil
}

// manualIssuanceInput 把 proto 的三段輸入攤成 usecase 的值。
//
// 🔴 每一個數字都 fail-loud：strconv 失敗回 InvalidArgument 而不是零值。金額吞成 0
// 的後果是開出一張金額錯誤的發票，而那不可回收。
func manualIssuanceInput(actor int64, req *pb.CreateManualIssuanceForAdminRequest) (usecase.ManualIssuanceInput, error) {
	in := usecase.ManualIssuanceInput{
		ActorAccountID: actor,
		DocumentKind:   req.GetDocumentKind(),
		Reason:         req.GetReason(),
		Confirmed:      req.GetConfirmed(),
		IdempotencyKey: req.GetIdempotencyKey(),
	}

	issuer := req.GetIssuer()
	in.Issuer.Kind = issuer.GetKind()
	if issuer.GetStoreId() != "" {
		v, err := parseID(issuer.GetStoreId())
		if err != nil {
			return in, status.Errorf(codes.InvalidArgument, "issuer.store_id: %v", err)
		}
		in.Issuer.StoreID = v
	}

	r := req.GetRecipient()
	in.Recipient = usecase.ManualIssuanceRecipient{
		Kind:         r.GetKind(),
		Name:         r.GetName(),
		TaxID:        r.GetTaxId(),
		Email:        r.GetEmail(),
		CarrierType:  r.GetCarrierType(),
		CarrierValue: r.GetCarrierValue(),
	}
	if r.GetBuyerId() != "" {
		v, err := parseID(r.GetBuyerId())
		if err != nil {
			return in, status.Errorf(codes.InvalidArgument, "recipient.buyer_id: %v", err)
		}
		in.Recipient.BuyerID = v
	}
	if r.GetStoreId() != "" {
		v, err := parseID(r.GetStoreId())
		if err != nil {
			return in, status.Errorf(codes.InvalidArgument, "recipient.store_id: %v", err)
		}
		in.Recipient.StoreID = v
	}

	src := req.GetSource()
	in.Source.Kind = src.GetKind()
	in.Source.OrderNo = src.GetOrderNo()
	for i, l := range src.GetLines() {
		qty, err := parseInt32(l.GetQuantity())
		if err != nil {
			return in, status.Errorf(codes.InvalidArgument, "source.lines[%d].quantity: %v", i, err)
		}
		unit, err := parseSignedAmount(l.GetUnitPrice())
		if err != nil {
			return in, status.Errorf(codes.InvalidArgument, "source.lines[%d].unit_price: %v", i, err)
		}
		net, err := parseSignedAmount(l.GetNetAmount())
		if err != nil {
			return in, status.Errorf(codes.InvalidArgument, "source.lines[%d].net_amount: %v", i, err)
		}
		in.Source.Lines = append(in.Source.Lines, usecase.ManualIssuanceLine{
			LineType:    l.GetLineType(),
			Name:        l.GetName(),
			Quantity:    qty,
			UnitPrice:   unit,
			NetAmount:   net,
			TaxCategory: l.GetTaxCategory(),
		})
	}
	return in, nil
}

// ActOnInvoiceComplaintForAdmin 是 ADM 申訴處理動作（端點 60）。
func (s *InvoiceServer) ActOnInvoiceComplaintForAdmin(
	ctx context.Context, req *pb.ActOnInvoiceComplaintForAdminRequest,
) (*pb.ActOnInvoiceComplaintForAdminResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	complaintID, err := parseID(req.GetComplaintId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}

	res, err := s.adminComplaints.Act(ctx, usecase.ComplaintActionInputAdmin{
		ActorAccountID:  actor,
		ComplaintID:     complaintID,
		Action:          req.GetAction(),
		Reason:          req.GetReason(),
		Confirmed:       req.GetConfirmed(),
		ExpectedVersion: version,
		IdempotencyKey:  req.GetIdempotencyKey(),
	})
	if err != nil {
		if st, ok := adminInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.ActOnInvoiceComplaintForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.admin.complaint.act.failed", err)
	}

	return &pb.ActOnInvoiceComplaintForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ActOnInvoiceComplaintForAdminData{
			ComplaintId:    fmtInt64(res.ComplaintID),
			Status:         res.Status,
			Version:        fmtInt64(res.Version),
			Recommendation: toSuspensionRecommendation(res.Recommendation),
			AuditLogId:     fmtID(res.AuditLogID),
		},
	}, nil
}

// toSuspensionRecommendation 把建議物件折成 proto。
//
// nil ⇒ 回 nil：只有 DECIDE_VIOLATION 與 RECOMMEND_SUSPENSION 會有建議，
// 給其他三個 action 回一個 should_suspend=false 的空物件會讓 A08 誤以為「系統評估過、
// 認為不用停權」，而實際上根本沒評估。
func toSuspensionRecommendation(r *usecase.SuspensionRecommendation) *pb.SuspensionRecommendation {
	if r == nil {
		return nil
	}
	return &pb.SuspensionRecommendation{
		ShouldSuspend:           r.ShouldSuspend,
		StoreId:                 fmtInt64(r.StoreID),
		ViolationCountIn_90Days: fmtInt64(r.ViolationCountIn90Days),
		ThresholdReached:        r.ThresholdReached,
		SuggestedSuspendDays:    fmtInt32(int32(r.SuggestedSuspendDays)),
		SuggestedReason:         r.SuggestedReason,
	}
}

// adminInvalidArgument 把 usecase 的型別邊界錯誤映成 400。
//
// 它們刻意不是 DomainError（usecase 側註解逐字）：沒有任何業務碼描述「你的字太長」
// 或「第 3 行金額對不起來」，而 bizOf 會把 DomainError 折成 envelope 的業務碼。
func adminInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrJobReasonRequired),
		errors.Is(err, usecase.ErrJobReasonTooLong),
		errors.Is(err, usecase.ErrJobNoteTooLong),
		errors.Is(err, usecase.ErrJobCancelReasonCodeInvalid),
		errors.Is(err, usecase.ErrJobCancelDetailRequired),
		errors.Is(err, usecase.ErrJobFundingDecisionInvalid),
		errors.Is(err, usecase.ErrJobFundingDecisionMissing),
		errors.Is(err, usecase.ErrExternalInvoiceNoInvalid),
		errors.Is(err, usecase.ErrExternalInvoiceDateInvalid),
		errors.Is(err, usecase.ErrExternalInvoiceDateTooEarly),
		errors.Is(err, usecase.ErrExternalTotalAmountInvalid),
		errors.Is(err, usecase.ErrIssuanceConfirmRequired),
		errors.Is(err, usecase.ErrIssuanceReasonRequired),
		errors.Is(err, usecase.ErrIssuanceDocumentKindInvalid),
		errors.Is(err, usecase.ErrIssuanceSourceKindInvalid),
		errors.Is(err, usecase.ErrIssuanceLinesRequired),
		errors.Is(err, usecase.ErrIssuanceLineInvalid),
		errors.Is(err, usecase.ErrIssuanceLineSignInvalid),
		errors.Is(err, usecase.ErrIssuanceLineTypeInvalid),
		errors.Is(err, usecase.ErrIssuanceLineQuantityInvalid),
		errors.Is(err, usecase.ErrIssuanceHeaderNegative),
		errors.Is(err, usecase.ErrIssuanceLineTaxInvalid),
		errors.Is(err, usecase.ErrIssuanceRecipientTaxID),
		errors.Is(err, usecase.ErrIssuanceRecipientNameLong),
		errors.Is(err, usecase.ErrIssuanceTotalNotPositive),
		errors.Is(err, usecase.ErrComplaintActionUnknown),
		errors.Is(err, usecase.ErrComplaintConfirmRequired),
		errors.Is(err, usecase.ErrComplaintAdminReasonRequired),
		errors.Is(err, usecase.ErrComplaintReasonTooLong):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}

// parseSignedAmount 解析可為負的元整數（DISCOUNT 行的 net_amount 恆為負）。
//
// 🔴 空字串不視同 0：明細金額是必填，把缺席折成 0 會靜默開出一張少算一行的發票。
func parseSignedAmount(s string) (int64, error) {
	if s == "" {
		return 0, errMissingAmount
	}
	return strconv.ParseInt(s, 10, 64)
}
