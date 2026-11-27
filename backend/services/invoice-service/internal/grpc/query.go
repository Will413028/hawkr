package grpc

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)



// ListBuyerInvoiceProfiles 是 B08「我的發票資料」列表（端點 5）。
func (s *InvoiceServer) ListBuyerInvoiceProfiles(
	ctx context.Context, req *pb.ListBuyerInvoiceProfilesRequest,
) (*pb.ListBuyerInvoiceProfilesResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	page, pageSize, err := parsePaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListBuyerProfiles(ctx, buyerID, page, pageSize)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListBuyerInvoiceProfilesResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_buyer_profiles.failed", err)
	}

	records := make([]*pb.InvoiceProfileInfo, 0, len(out.Records))
	for _, p := range out.Records {
		records = append(records, toProfileInfo(p))
	}
	return &pb.ListBuyerInvoiceProfilesResponse{
		Code:    0,
		Message: msgSuccess,
		Data: &pb.ListBuyerInvoiceProfilesData{
			Records:  records,
			Total:    fmtInt32(out.Total),
			Page:     fmtInt32(out.Page),
			PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// GetInvoiceComplaint 是買家看自己的一件申訴（端點 15）。
func (s *InvoiceServer) GetInvoiceComplaint(
	ctx context.Context, req *pb.GetInvoiceComplaintRequest,
) (*pb.InvoiceComplaintResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	complaintID, err := parseID(req.GetComplaintId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}

	view, err := s.query.GetBuyerComplaint(ctx, buyerID, req.GetOrderNo(), complaintID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_buyer_complaint.failed", err)
	}
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: toBuyerComplaintInfo(view)}, nil
}


// GetSellerTaxProfile 是 S02 稅務身分現況（端點 20）。
func (s *InvoiceServer) GetSellerTaxProfile(
	ctx context.Context, req *pb.GetSellerTaxProfileRequest,
) (*pb.SellerTaxProfileResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	view, err := s.query.GetSellerTaxProfile(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.SellerTaxProfileResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_tax_profile.failed", err)
	}
	return &pb.SellerTaxProfileResponse{Code: 0, Message: msgSuccess, Data: s.toSellerTaxProfileData(ctx, view)}, nil
}

// GetSelfIssueAttestation 是 S04 自行開票責任條款的現況（端點 23）。
func (s *InvoiceServer) GetSelfIssueAttestation(
	ctx context.Context, req *pb.GetSelfIssueAttestationRequest,
) (*pb.SelfIssueAttestationResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	view, err := s.query.GetSellerTaxProfile(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.SelfIssueAttestationResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_self_issue_attestation.failed", err)
	}
	return &pb.SelfIssueAttestationResponse{
		Code: 0, Message: msgSuccess, Data: toSelfIssueAttestation(view.Profile),
	}, nil
}

// GetSellerProviderAccount 是 S03 AMEGO 憑證現況（端點 24）。
func (s *InvoiceServer) GetSellerProviderAccount(
	ctx context.Context, req *pb.GetSellerProviderAccountRequest,
) (*pb.SellerProviderAccountResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	view, err := s.query.GetSellerProviderAccount(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.SellerProviderAccountResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_provider_account.failed", err)
	}
	return &pb.SellerProviderAccountResponse{Code: 0, Message: msgSuccess, Data: toProviderAccountData(view)}, nil
}


// GetTaxReviewForAdmin 是 A03 稅務身分審核詳情（端點 39）。
func (s *InvoiceServer) GetTaxReviewForAdmin(
	ctx context.Context, req *pb.GetTaxReviewForAdminRequest,
) (*pb.GetTaxReviewForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	view, err := s.query.GetTaxReviewForAdmin(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetTaxReviewForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_tax_review_admin.failed", err)
	}
	return &pb.GetTaxReviewForAdminResponse{Code: 0, Message: msgSuccess, Data: s.toSellerTaxProfileData(ctx, view)}, nil
}

// GetProviderAccountForAdmin 是 A04 AMEGO 串接監控詳情（端點 42）。
func (s *InvoiceServer) GetProviderAccountForAdmin(
	ctx context.Context, req *pb.GetProviderAccountForAdminRequest,
) (*pb.GetProviderAccountForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	view, err := s.query.GetProviderAccountForAdmin(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetProviderAccountForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_provider_account_admin.failed", err)
	}
	return &pb.GetProviderAccountForAdminResponse{Code: 0, Message: msgSuccess, Data: toProviderAccountData(view)}, nil
}

// GetInvoiceDocumentForAdmin 是 A06 發票詳情（端點 48）。
//
// 這一支會揭露 P 類資料（買受人 email／載具明碼、provider 往返原文）⇒ usecase 在回資料
// 之前先寫一列 INVOICE_PII_REVEAL 稽核，寫不進去就不回（fail-closed，§8.1.3(c) 裁定 2）。
func (s *InvoiceServer) GetInvoiceDocumentForAdmin(
	ctx context.Context, req *pb.GetInvoiceDocumentForAdminRequest,
) (*pb.GetInvoiceDocumentForAdminResponse, error) {
	actorID, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}
	view, err := s.query.GetDocumentForAdmin(ctx, actorID, documentID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceDocumentForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_document_admin.failed", err)
	}
	return &pb.GetInvoiceDocumentForAdminResponse{
		Code: 0, Message: msgSuccess, Data: s.toAdminDocumentDetail(view),
	}, nil
}

// GetInvoiceJobForAdmin 是 A06 開票作業詳情（端點 51）。同樣留 PII 揭露稽核。
func (s *InvoiceServer) GetInvoiceJobForAdmin(
	ctx context.Context, req *pb.GetInvoiceJobForAdminRequest,
) (*pb.GetInvoiceJobForAdminResponse, error) {
	actorID, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	jobID, err := parseID(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "job_id: %v", err)
	}
	view, err := s.query.GetJobForAdmin(ctx, actorID, jobID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceJobForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_job_admin.failed", err)
	}
	return &pb.GetInvoiceJobForAdminResponse{Code: 0, Message: msgSuccess, Data: s.toAdminJobDetail(ctx, view)}, nil
}

// GetInvoiceComplaintForAdmin 是 A07/A08 申訴詳情（端點 59）。同樣留 PII 揭露稽核。
func (s *InvoiceServer) GetInvoiceComplaintForAdmin(
	ctx context.Context, req *pb.GetInvoiceComplaintForAdminRequest,
) (*pb.InvoiceComplaintResponse, error) {
	actorID, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	complaintID, err := parseID(req.GetComplaintId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}
	view, err := s.query.GetComplaintForAdmin(ctx, actorID, complaintID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_complaint_admin.failed", err)
	}
	info := toComplaintInfo(view)
	info.StoreId = fmtID(view.StoreID)
	s.fillOneBuyerName(ctx, info, view)
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: info}, nil
}


const msgSuccess = "success"

func requirePrincipal(v int64, field string) (int64, error) {
	if v <= 0 {
		return 0, status.Errorf(codes.InvalidArgument, "%s: %v", field, errNonPositiveID)
	}
	return v, nil
}

func parsePaging(page, pageSize string) (int32, int32, error) {
	p, err := parseInt32(page)
	if err != nil {
		return 0, 0, status.Errorf(codes.InvalidArgument, "page: %v", err)
	}
	ps, err := parseInt32(pageSize)
	if err != nil {
		return 0, 0, status.Errorf(codes.InvalidArgument, "page_size: %v", err)
	}
	return p, ps, nil
}

func parseInt32(s string) (int32, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

func bizOf(err error) (int32, string, bool) {
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) {
		return 0, "", false
	}
	return int32(de.Code), de.DomainMessage(), true
}

func (s *InvoiceServer) internal(_ context.Context, event string, err error) error {
	s.logger().Error(event, zap.Error(err))
	return status.Error(codes.Internal, event)
}

func fmtInt32(v int32) string { return strconv.FormatInt(int64(v), 10) }
func fmtInt64(v int64) string { return strconv.FormatInt(v, 10) }

func fmtID(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

func fmtIDPtr(v *int64) string {
	if v == nil {
		return ""
	}
	return fmtID(*v)
}

func fmtTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return timeutil.FormatISO8601(*t)
}

func fmtTimeValue(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return timeutil.FormatISO8601(t)
}

func fmtDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func firstNonEmptyStr(vals ...*string) string {
	for _, v := range vals {
		if s := str(v); s != "" {
			return s
		}
	}
	return ""
}


func toComplaintInfo(v usecase.ComplaintView) *pb.InvoiceComplaintInfo {
	c := v.Complaint
	out := &pb.InvoiceComplaintInfo{
		ComplaintId: fmtID(c.ID),
		CaseNo:  fmtID(c.ID),
		OrderNo: str(c.OrderNo),
		StoreId:     "",
		Status:      c.Status,
		DueAt:       fmtTimeValue(c.SlaDueAt),
		Description: str(c.Description),
		Version:     fmtInt64(c.Version),
		CreatedAt:   fmtTimeValue(c.CreatedAt),
		ReopenCount: fmtInt64(int64(c.ReopenCount)),
		ExpectedDocKind: v.ExpectedDocKind,
	}
	if v := v.RemainingBusinessDays; v != nil {
		out.RemainingBusinessDays = fmtInt32(*v)
	}
	if v := v.OverdueBusinessDays; v != nil {
		out.OverdueBusinessDays = fmtInt32(*v)
	}
	if c.Status == complaintStatusSellerResponded {
		out.BuyerReviewDueAt = fmtTime(c.BuyerReviewDueAt)
	}
	if c.Status == complaintStatusResolved {
		out.ReopenAllowedAfter = fmtTime(c.ReopenAllowedAfter)
	}
	for _, e := range v.Events {
		out.Responses = append(out.Responses, &pb.ComplaintResponseItem{
			ResponseId: fmtID(e.ID),
			Actor:      e.ActorType,
			Action:     str(e.Action),
			Note:       str(e.Message),
			OccurredAt: fmtTimeValue(e.OccurredAt),
		})
	}
	return out
}

const (
	complaintStatusSellerResponded = "SELLER_RESPONDED"
	complaintStatusResolved        = "RESOLVED"
)


func (s *InvoiceServer) toSellerTaxProfileData(ctx context.Context, v usecase.SellerTaxProfileView) *pb.SellerTaxProfileData {
	p := v.Profile
	out := &pb.SellerTaxProfileData{
		StoreId:        fmtID(v.StoreID),
		ReviewStatus:   p.ReviewStatus,
		Version:        fmtInt64(p.Version),
		SubmittedAt:    fmtTime(p.SubmittedAt),
		DeadlineAt:     fmtTime(p.PendingValidUntil),
		ExtensionCount: fmtInt64(int64(p.ExtensionCount)),
		CanRequestExtension:  usecase.CanRequestTaxExtension(p),
		ReapplyCooldownUntil: fmtTime(p.CooldownUntil),
		TaxIdentity:          toSellerTaxIdentity(p),
		SelfIssueAttestation: toSelfIssueAttestation(p),
		IntakeLimit:          toProfileIntakeLimit(p),
		Documents:            s.toTaxDocuments(ctx, p),
		ReceiptAuthorized:   p.ReceiptAuthorizedAt != nil,
		ReceiptAuthorizedAt: fmtTime(p.ReceiptAuthorizedAt),
	}
	if req := pendingExtensionOf(ctx, s.query, p); req != nil {
		out.PendingExtensionRequest = &pb.TaxExtensionRequestInfo{
			RequestedAt: fmtTime(&req.RequestedAt),
			Reason:      req.Reason,
		}
	}
	if note := str(p.ReviewNote); note != "" {
		out.AdmNotes = []*pb.TaxReviewNote{{
			NoteId:  fmtID(p.ID),
			Content: note,
			CreatedAt: fmtTime(p.ReviewNoteAt),
		}}
	}
	return out
}

func addressDetailOf(p domain.SellerTaxProfile) string {
	full := str(p.BusinessAddress)
	city, district := str(p.AddressCity), str(p.AddressDistrict)
	if city == "" {
		return full
	}
	rest := strings.TrimPrefix(full, city)
	rest = strings.TrimPrefix(rest, district)
	return rest
}

func toSellerTaxIdentity(p domain.SellerTaxProfile) *pb.SellerTaxIdentity {
	return &pb.SellerTaxIdentity{
		EntityType: str(p.EntityTaxType),
		TaxId:           str(p.TaxID),
		LegalName:       str(p.LegalName),
		ResponsibleName: str(p.ResponsiblePerson),
		AddressCity:     str(p.AddressCity),
		AddressDistrict: str(p.AddressDistrict),
		AddressDetail:   addressDetailOf(p),
		ClaimedStatus:   p.ClaimedObligation,
	}
}

func toSelfIssueAttestation(p domain.SellerTaxProfile) *pb.SelfIssueAttestationInfo {
	return &pb.SelfIssueAttestationInfo{
		TermsVersion:        str(p.AttestationVersion),
		AcceptedAt:          fmtTime(p.AttestedAt),
		AcceptedByAccountId: fmtIDPtr(p.AttestedByAccountID),
		Effective:           p.SelfIssueEffective(),
		CurrentTermsVersion: usecase.SelfIssueTermsVersion,
	}
}

func toProfileIntakeLimit(p domain.SellerTaxProfile) *pb.SellerIntakeLimit {
	if !usecase.IntakeLimitApplies(p) {
		return nil
	}
	return &pb.SellerIntakeLimit{
		OrderCountUsed:  fmtInt64(int64(p.PendingOrderCount)),
		OrderCountLimit: fmtInt64(int64(p.PendingOrderLimitCount)),
		AmountUsed:      fmtInt64(p.PendingOrderAmount),
		AmountLimit:     fmtInt64(p.PendingOrderLimitAmount),
		Blocked: p.PendingOrderCount >= p.PendingOrderLimitCount ||
			p.PendingOrderAmount >= p.PendingOrderLimitAmount,
	}
}

func (s *InvoiceServer) toTaxDocuments(ctx context.Context, p domain.SellerTaxProfile) []*pb.TaxDocumentInfo {
	keys := p.EvidenceKeys()
	if len(keys) == 0 {
		return nil
	}
	meta := s.resolveEvidenceMetadata(ctx, keys, p.ID)

	out := make([]*pb.TaxDocumentInfo, 0, len(keys))
	for i, k := range keys {
		doc := &pb.TaxDocumentInfo{
			DocumentId: fmtInt64(int64(i + 1)),
			FileKey:    k,
		}
		if m, ok := meta[k]; ok {
			doc.FileAssetId = m.AssetID
			doc.FileName = m.FileName
			doc.FileSize = m.SizeBytes
			doc.UploadedAt = m.UploadedAt
		}
		out = append(out, doc)
	}
	return out
}


func toProviderAccountData(v usecase.ProviderAccountView) *pb.SellerProviderAccountData {
	a := v.Account
	out := &pb.SellerProviderAccountData{
		Provider:        a.Provider,
		Status:          a.Status,
		StatusNotice:    str(a.StatusReason),
		Version:         fmtInt64(a.Version),
		SellerTaxId:     a.ProviderTaxID,
		CredentialLast4: a.CredentialLast4,
		CredentialUpdatedAt: fmtTime(a.CredentialUpdatedAt),
		LastVerifiedAt:      fmtTime(a.VerifiedAt),
		AutomationCutoverAt: fmtTime(a.AutomationCutoverAt),
	}
	if v.HasProfile {
		out.PendingOrderCount = fmtInt64(int64(v.Profile.PendingOrderCount))
		out.ConsentVersion = str(v.Profile.ReceiptConsentVersion)
	}
	out.LastRequestId = v.LastRequestID
	return out
}


func (s *InvoiceServer) toAdminDocumentDetail(v usecase.AdminDocumentView) *pb.AdminInvoiceDocumentDetail {
	out := &pb.AdminInvoiceDocumentDetail{
		SellerView: s.toSellerDocumentDetail(v),
	}
	for _, ex := range v.Exchanges {
		out.ProviderExchanges = append(out.ProviderExchanges, toProviderExchange(ex))
	}
	for _, op := range v.Operations {
		out.RelatedJobs = append(out.RelatedJobs, s.toJobInfo(op, v.Document, v.Issuer))
	}
	if len(v.Operations) > 0 {
		out.IdempotencyKey = v.Operations[0].OperationKey
	}
	out.DocumentVersion = fmtInt64(v.Document.Version)
	out.CanResumeFromManual = usecase.CanResumeDocumentFromManual(v.Document.Status, len(v.Operations))
	return out
}

func (s *InvoiceServer) toSellerDocumentDetail(v usecase.AdminDocumentView) *pb.SellerInvoiceDocumentDetail {
	doc := v.Document
	latestJob := ""
	if len(v.Operations) > 0 {
		latestJob = fmtID(v.Operations[0].ID)
	}
	return &pb.SellerInvoiceDocumentDetail{
		Detail:            s.toDocumentDetail(v),
		FundedBy:          fundedByUndetermined,
		CutoverVersion:    fmtIDPtr(doc.ProviderAccountVersion),
		ProviderRequestId: str(doc.ProviderRequestNo),
		ManualReasonCode:  str(doc.ManualReasonCode),
		LatestJobId:       latestJob,
	}
}

const fundedByUndetermined = "UNDETERMINED"

func (s *InvoiceServer) toDocumentDetail(v usecase.AdminDocumentView) *pb.InvoiceDocumentDetail {
	doc := v.Document
	out := &pb.InvoiceDocumentDetail{
		Summary:        s.toDocumentSummary(v),
		IssuerName:     doc.IssuerNameSnapshot,
		IssuerTaxId:    str(doc.IssuerTaxIDSnapshot),
		RecipientName:  doc.RecipientName,
		RecipientTaxId: doc.RecipientTaxID,
		SalesAmount:    fmtInt64(doc.SalesAmount),
		TaxAmount:      fmtInt64(doc.TaxAmount),
		TotalAmount:    fmtInt64(doc.TotalAmount),
		TaxCategory:    fmtInt64(int64(doc.TaxType)),
		InvoiceFileKey: str(doc.PdfObjectKey),
	}
	for _, l := range v.Lines {
		out.Lines = append(out.Lines, &pb.InvoiceLine{
			LineType:    l.LineType,
			Name:        l.Description,
			Quantity:    fmtInt64(int64(l.Quantity)),
			UnitPrice:   fmtInt64(l.UnitPrice),
			NetAmount:   fmtInt64(l.NetAmount),
			TaxCategory: fmtInt64(int64(l.TaxCategory)),
		})
	}
	for _, h := range v.Timeline {
		out.Timeline = append(out.Timeline, &pb.InvoiceTimelineItem{
			OccurredAt: fmtTimeValue(h.OccurredAt),
			Event:      h.ToStatus,
			Note:       str(h.ReasonDetail),
		})
	}
	return out
}

func (s *InvoiceServer) toDocumentSummary(v usecase.AdminDocumentView) *pb.InvoiceDocumentSummary {
	carrier, carrierType := v.RevealedCarrierID, ""
	if carrier == "" {
		carrier, carrierType = carrierSummaryFallback(v.Document)
	} else if kind, ok := domain.CarrierTypeFromAmego(str(v.Document.CarrierType)); ok {
		carrierType = kind.Value
	}
	return s.toSummaryWithCarrier(v.Document, carrier, carrierType)
}

func firstNonEmptyID(vals ...*int64) string {
	for _, v := range vals {
		if s := fmtIDPtr(v); s != "" {
			return s
		}
	}
	return ""
}

func (s *InvoiceServer) toAdminJobDetail(ctx context.Context, v usecase.AdminJobView) *pb.AdminInvoiceJobDetail {
	out := &pb.AdminInvoiceJobDetail{
		Job:            s.toJobInfo(v.Operation, v.Document, v.Issuer),
		IdempotencyKey: v.Operation.OperationKey,
	}
	if len(v.Document.SourceSnapshot) > 0 {
		out.OrderSnapshotJson = string(v.Document.SourceSnapshot)
	}
	if amt, ok := usecase.ParseSnapshotAmounts(v.Document.SourceSnapshot); ok {
		out.AmountBreakdown = &pb.InvoiceAmountBreakdown{
			ProductAmount:  fmtInt64(amt.ProductAmount),
			ShippingAmount: fmtInt64(amt.ShippingAmount),
			DiscountAmount: fmtInt64(amt.DiscountAmount),
			InvoiceAmount:  fmtInt64(amt.InvoiceAmount),
			TaxCategory:    fmtInt64(int64(amt.TaxCategory)),
		}
	}
	for _, ex := range v.Exchanges {
		out.ProviderExchanges = append(out.ProviderExchanges, toProviderExchange(ex))
	}
	for _, l := range v.AuditLogs {
		out.AuditLogs = append(out.AuditLogs, toAuditLogItem(l))
	}
	if orderNo := out.Job.GetOrderNo(); orderNo != "" {
		statuses, degraded := s.resolveShipmentStatuses(ctx, []string{orderNo})
		out.ShipmentStatus = statuses[orderNo]
		out.ShipmentStatusDegraded = degraded
	}
	if v.ManualUnlock != nil {
		out.ManualUnlock = toManualUnlockInfo(*v.ManualUnlock)
	}
	return out
}

func (s *InvoiceServer) toJobInfo(op domain.Operation, doc domain.Document, issuer domain.Issuer) *pb.InvoiceJobInfo {
	orderNo := ""
	if doc.SourceType == sourceTypeOrder {
		orderNo = str(doc.SourceRef)
	}
	return s.toJobInfoView(domain.OperationView{
		Operation:   op,
		IssuerRefID: issuer.IssuerRefID,
		DocumentKind:             doc.DocumentKind,
		DocumentStatus:           doc.Status,
		DocumentSourceRef:        orderNo,
		DocumentManualReasonCode: str(doc.ManualReasonCode),
		ExternalInvoiceNo:        str(doc.ExternalInvoiceNo),
		ExternalInvoiceDate:      doc.ExternalInvoiceDate,
		ExternalReportedAt:       doc.ExternalReportedAt,
	})
}

const (
	sourceTypeOrder                     = "ORDER"
	operationStatusManualProcessAllowed = "MANUAL_PROCESS_ALLOWED"
	shipmentUnaffectedNotice        = "發票開立與出貨進度互不影響，出貨不會因為發票異常而停下。"
	shipmentUnaffectedNoticeReceipt = "收據開立與出貨進度互不影響，出貨不會因為收據異常而停下。"
)

func noticeForKind(documentKind string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return shipmentUnaffectedNoticeReceipt
	}
	return shipmentUnaffectedNotice
}

func toProviderExchange(e domain.ProviderAuditLogEntry) *pb.ProviderExchangeItem {
	return &pb.ProviderExchangeItem{
		ExchangeId:   fmtID(e.ID),
		OccurredAt:   fmtTimeValue(e.OccurredAt),
		Endpoint:     e.Endpoint,
		ProviderCode: str(e.ProviderCode),
		ProviderMessage: str(e.ProviderMessage),
		RequestId:          str(e.CorrelationID),
		MaskedRequestBody:  string(e.RequestMasked),
		MaskedResponseBody: string(e.ResponseMasked),
	}
}

func toAuditLogItem(l domain.AdminAuditLogEntry) *pb.InvoiceAuditLogItem {
	return &pb.InvoiceAuditLogItem{
		AuditLogId:     fmtID(l.ID),
		OccurredAt:     fmtTimeValue(l.OccurredAt),
		ActorAccountId: fmtID(l.ActorAccountID),
		OperationType:  l.Action,
		TargetKind:     l.ResourceType,
		TargetId:       fmtID(l.ResourceID),
		BeforeJson:     string(l.BeforeState),
		AfterJson:      string(l.AfterState),
		Reason:         l.ReasonDetail,
		RequestId:      str(l.RequestID),
	}
}

func pendingExtensionOf(ctx context.Context, q *usecase.Query, p domain.SellerTaxProfile) *usecase.TaxExtensionRequest {
	if q == nil {
		return nil
	}
	return q.PendingTaxExtensionRequest(ctx, p)
}
