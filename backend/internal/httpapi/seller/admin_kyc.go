package seller

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 後台的身分認證（KYC）審核與 PayUni 方法（遷移計畫 A3b 步驟 8f）。後台的訊息與前台的 KYCApplication 分開：前台的有寫入用的
// 明文欄位，後台的型別沒有任何欄位能放明文；全文只由 RevealKYCSensitive 回傳。

// 模組輸入的欄位對應到請求的路徑。
var (
	listKYCPaths    = rpc.FieldPaths{"Status": "status"}
	approveKYCPaths = rpc.FieldPaths{"ExpectedVersion": "etag", "Comment": "comment"}
	rejectKYCPaths  = rpc.FieldPaths{
		"ExpectedVersion": "etag", "Reason": "reason", "InternalNote": "internal_note", "Issues": "issues",
		"Issues[].Field": "issues[].field", "Issues[].DocumentAssetID": "issues[].document_asset_id",
		"Issues[].Code": "issues[].code", "Issues[].Message": "issues[].message",
	}
	revealKYCPaths = rpc.FieldPaths{"Reason": "reason"}
)

// kycFieldNames 是契約的 KYCField 對應到模組的欄位路徑（model.KYCFieldNames 的反向，kycFieldPB 的逆運算）。
var kycFieldNames = func() map[pb.KYCField]string {
	out := map[pb.KYCField]string{}
	for _, name := range model.KYCFieldNames() {
		out[kycFieldPB(name)] = name
	}
	return out
}()

// issuesFromPB 把退回的問題轉成模組的輸入；UNSPECIFIED 與不認得的欄位或原因碼是 invalid_argument（指到 issues[i] 的欄位），
// 不往下傳。其餘的規則（說明長度、文件是不是這份申請的）由模組檢查。
func issuesFromPB(issues []*pb.KYCIssue) ([]model.KYCIssue, error) {
	out := make([]model.KYCIssue, len(issues))
	for i, issue := range issues {
		field, ok := kycFieldNames[issue.GetField()]
		if !ok {
			return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{fmt.Sprintf("Issues[%d].Field", i)}, "issues 有不認得的欄位"), rejectKYCPaths)
		}
		code := rpc.EnumFromProto[model.KYCIssueCode](issue.GetCode())
		if !slices.Contains(model.KYCIssueCodeValues(), code) {
			return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{fmt.Sprintf("Issues[%d].Code", i)}, "issues 有不認得的原因碼"), rejectKYCPaths)
		}
		out[i] = model.KYCIssue{Field: field, DocumentAssetID: issue.GetDocumentAssetId(), Code: code, Message: issue.GetMessage()}
	}
	return out, nil
}

func (h *Admin) ListKYCApplications(ctx context.Context, req *connect.Request[adminpb.ListKYCApplicationsRequest]) (*connect.Response[adminpb.ListKYCApplicationsResponse], error) {
	msg := req.Msg
	in := model.KYCListInput{Query: msg.GetQuery(), PageSize: msg.GetPageSize()}
	if status := rpc.EnumFromProto[model.KYCStatus](msg.GetStatus()); status != "" {
		in.Status = &status
	}
	// page token 只能用在同一組篩選：換了篩選就從第一頁開始（排序方向也跟著篩選變）。
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerAdminServiceListKYCApplicationsProcedure, msg.GetStatus().String(), in.Query)
	after, err := rpc.ValidCursor[model.KYCCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.AdminKYCApplications(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listKYCPaths)
	}
	out := &adminpb.ListKYCApplicationsResponse{Applications: make([]*adminpb.KYCApplicationSummary, len(page.Items))}
	for i, item := range page.Items {
		out.Applications[i] = toKYCSummary(item)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) GetKYCApplication(ctx context.Context, req *connect.Request[adminpb.GetKYCApplicationRequest]) (*connect.Response[adminpb.GetKYCApplicationResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	application, err := h.m.AdminKYCApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetKYCApplicationResponse{Application: toAdminKYC(application)}), nil
}

// ApproveKYC 的身分比對沒有通過時，模組回固定文字的 failed_precondition（不透露欄位或帳號）；這裡原樣交回，不加任何細節。
func (h *Admin) ApproveKYC(ctx context.Context, req *connect.Request[adminpb.ApproveKYCRequest]) (*connect.Response[adminpb.ApproveKYCResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	v, seq, err := parseKYCAdminEtag(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	approved, err := h.m.ApproveKYC(ctx, model.ApproveKYCInput{ApplicationID: id, ExpectedVersion: v, ExpectedCheckSeq: seq, AdminID: adminID, Comment: req.Msg.GetComment()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, approveKYCPaths)
	}
	return connect.NewResponse(&adminpb.ApproveKYCResponse{Application: toAdminKYC(approved)}), nil
}

func (h *Admin) RejectKYC(ctx context.Context, req *connect.Request[adminpb.RejectKYCRequest]) (*connect.Response[adminpb.RejectKYCResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	v, seq, err := parseKYCAdminEtag(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	issues, err := issuesFromPB(req.Msg.GetIssues())
	if err != nil {
		return nil, err
	}
	rejected, err := h.m.RejectKYC(ctx, model.RejectKYCInput{
		ApplicationID: id, ExpectedVersion: v, ExpectedCheckSeq: seq, AdminID: adminID, Reason: req.Msg.GetReason(),
		Issues: issues, InternalNote: req.Msg.GetInternalNote(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, rejectKYCPaths)
	}
	return connect.NewResponse(&adminpb.RejectKYCResponse{Application: toAdminKYC(rejected)}), nil
}

// RevealKYCSensitive 是寫入稽核的動作（POST）：理由必填，稽核寫入失敗就不回傳明文。回應不可快取由 httpcache 對所有後台
// 回應統一設定（private, no-store）。
func (h *Admin) RevealKYCSensitive(ctx context.Context, req *connect.Request[adminpb.RevealKYCSensitiveRequest]) (*connect.Response[adminpb.RevealKYCSensitiveResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	got, err := h.m.RevealKYCSensitive(ctx, model.RevealKYCInput{AdminID: adminID, ApplicationID: id, Reason: req.Msg.GetReason()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, revealKYCPaths)
	}
	out := &adminpb.RevealKYCSensitiveResponse{
		IdCardNo: deref(got.IDCardNo), RepIdCardNo: deref(got.RepIDCardNo), BankAccount: deref(got.BankAccount),
		ExpireTime: timestamppb.New(got.ExpiresAt),
	}
	for _, d := range got.Documents {
		out.Documents = append(out.Documents, &adminpb.KYCDocumentView{
			Field: documentFieldPB(d.Slot), Position: int32(d.Position), ViewUri: documentURI(id, d), AssetId: d.AssetID, //nolint:gosec // position < 10
		})
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) ListKYCReveals(ctx context.Context, req *connect.Request[adminpb.ListKYCRevealsRequest]) (*connect.Response[adminpb.ListKYCRevealsResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	scope := auditScope(sellerv1alpha1connect.SellerAdminServiceListKYCRevealsProcedure, id)
	after, err := rpc.ValidCursor[model.KYCAuditCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListKYCReveals(ctx, model.KYCAuditListInput{ApplicationID: id, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListKYCRevealsResponse{Reveals: make([]*adminpb.KYCReveal, len(page.Items))}
	for i, r := range page.Items {
		out.Reveals[i] = &adminpb.KYCReveal{
			Id: r.ID.String(), AdminId: rpc.FormatID(r.AdminID), Reason: r.Reason, RevealTime: timestamppb.New(r.RevealedAt), ExpireTime: timestamppb.New(r.ExpiresAt),
		}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) ListKYCDocumentAccesses(ctx context.Context, req *connect.Request[adminpb.ListKYCDocumentAccessesRequest]) (*connect.Response[adminpb.ListKYCDocumentAccessesResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	scope := auditScope(sellerv1alpha1connect.SellerAdminServiceListKYCDocumentAccessesProcedure, id)
	after, err := rpc.ValidCursor[model.KYCAuditCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListKYCDocumentAccesses(ctx, model.KYCAuditListInput{ApplicationID: id, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListKYCDocumentAccessesResponse{DocumentAccesses: make([]*adminpb.KYCDocumentAccess, len(page.Items))}
	for i, acc := range page.Items {
		out.DocumentAccesses[i] = &adminpb.KYCDocumentAccess{
			Id: acc.ID.String(), RevealId: acc.RevealID.String(), AdminId: rpc.FormatID(acc.AdminID),
			Field: documentFieldPB(acc.Slot), Position: int32(acc.Position), AccessTime: timestamppb.New(acc.AccessedAt), //nolint:gosec // position < 10
		}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

// auditScope 是一筆申請的稽核紀錄的 page token scope（方法加申請 ID）。
func auditScope(procedure string, applicationID uuid.UUID) string {
	return procedure + "\x00" + applicationID.String()
}

func (h *Admin) GetPayUniRegistration(ctx context.Context, req *connect.Request[adminpb.GetPayUniRegistrationRequest]) (*connect.Response[adminpb.GetPayUniRegistrationResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	reg, err := h.m.PayUniRegistrationOf(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetPayUniRegistrationResponse{Registration: toAdminPayUni(reg)}), nil
}

// RebindPayUni 沒有 etag 也不要理由：「只有綁定失敗才能重綁」的狀態檢查在模組，重送是冪等的。
func (h *Admin) RebindPayUni(ctx context.Context, req *connect.Request[adminpb.RebindPayUniRequest]) (*connect.Response[adminpb.RebindPayUniResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	reg, err := h.m.RebindPayUni(ctx, model.RebindPayUniInput{StoreID: storeID, AdminID: adminID})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RebindPayUniResponse{Registration: toAdminPayUni(reg)}), nil
}

func (h *Admin) ListPayUniObservations(ctx context.Context, req *connect.Request[adminpb.ListPayUniObservationsRequest]) (*connect.Response[adminpb.ListPayUniObservationsResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	// 沒有這家店時模組回 not_found，不是空的清單。
	scope := rpc.PageScope(sellerv1alpha1connect.SellerAdminServiceListPayUniObservationsProcedure, storeID)
	after, err := rpc.ValidCursor[model.PayUniObservationCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.PayUniObservations(ctx, model.PayUniObservationListInput{StoreID: storeID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListPayUniObservationsResponse{Observations: make([]*adminpb.PayUniObservation, len(page.Items))}
	for i, o := range page.Items {
		out.Observations[i] = toObservation(o)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

// RequeryKYCRegistration 排入一次登記資料查詢（回應不等結果），回傳這筆進行中的查詢（帶期限）；這一輪還沒查完的舊查詢被取代。
func (h *Admin) RequeryKYCRegistration(ctx context.Context, req *connect.Request[adminpb.RequeryKYCRegistrationRequest]) (*connect.Response[adminpb.RequeryKYCRegistrationResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	check, err := h.m.RequeryKYCRegistration(ctx, model.RequeryKYCRegistrationInput{ApplicationID: id, AdminID: adminID})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RequeryKYCRegistrationResponse{RegistrationCheck: toRegistrationCheck(check)}), nil
}

func (h *Admin) RefreshPayUniStatus(ctx context.Context, req *connect.Request[adminpb.RefreshPayUniStatusRequest]) (*connect.Response[adminpb.RefreshPayUniStatusResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	if err := h.m.RefreshPayUniStatus(ctx, storeID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RefreshPayUniStatusResponse{}), nil
}

func toKYCSummary(a model.KYCApplication) *adminpb.KYCApplicationSummary {
	out := &adminpb.KYCApplicationSummary{
		Id: a.ID.String(), StoreId: rpc.FormatID(a.StoreID), AccountId: rpc.FormatID(a.AccountID),
		Status: rpc.EnumToProto[pb.KYCStatus](a.Status), SellerType: rpc.EnumToProto[adminpb.KYCSellerType](a.SellerType),
		RealName: deref(a.RealName), CompanyName: deref(a.CompanyName), TaxId: deref(a.TaxID),
	}
	if a.SubmittedAt != nil {
		out.SubmitTime = timestamppb.New(*a.SubmittedAt)
	}
	if a.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*a.DecidedAt)
	}
	return out
}

// toAdminKYC 轉成後台的申請：只有遮蔽後的值（模組的型別沒有明文欄位）；證件只有位置。
// documentFieldPB 把證件位置轉成它所屬的欄位（後台契約用欄位表示文件，位置只留在模組內部）；不認得的位置是 UNSPECIFIED。
func documentFieldPB(slot model.KYCDocumentSlot) pb.KYCField {
	field, ok := model.KYCSlotField(slot)
	if !ok {
		return pb.KYCField_KYC_FIELD_UNSPECIFIED
	}
	return kycFieldPB(field)
}

func toAdminKYC(a model.KYCAdminApplication) *adminpb.KYCApplication {
	out := &adminpb.KYCApplication{
		Id: a.ID.String(), StoreId: rpc.FormatID(a.StoreID), AccountId: rpc.FormatID(a.AccountID),
		Status: rpc.EnumToProto[pb.KYCStatus](a.Status), Etag: kycAdminEtag(a.Version, a.RegistrationCheckSeq()),
		RealName: deref(a.RealName), BirthDate: toDate(a.BirthDate),
		IdCardIssueDate: toDate(a.IDCardIssueDate), IdCardIssueType: deref(a.IDCardIssueType), IdCardIssueLocation: deref(a.IDCardIssueLocation),
		EnglishName: deref(a.EnglishName), BankName: deref(a.BankName), BankCode: deref(a.BankCode), BankAccountMasked: deref(a.BankAccountMasked),
		ProductType:                            deref(a.ProductType),
		WebsiteUrl:                             deref(a.WebsiteURL),
		SharesBankAccountWithSanctionedAccount: a.SharesBankAccountWithSanctionedAccount,
		OutstandingIssues:                      toKYCIssues(a.OutstandingIssues),
		CanApprove:                             len(a.ApproveUnavailable) == 0,
		ApproveUnavailabilities:                toStoreActionUnavailabilities(a.ApproveUnavailable, model.StoreActionApproveKYC),
		CanReject:                              len(a.RejectUnavailable) == 0,
		RejectUnavailabilities:                 toStoreActionUnavailabilities(a.RejectUnavailable, model.StoreActionRejectKYC),
		CanRequeryRegistration:                 len(a.RequeryUnavailable) == 0,
		RequeryRegistrationUnavailabilities:    toStoreActionUnavailabilities(a.RequeryUnavailable, model.StoreActionRequeryKYC),
		CreateTime:                             timestamppb.New(a.CreatedAt), UpdateTime: timestamppb.New(a.UpdatedAt),
	}
	switch a.SellerType {
	case model.KYCIndividual:
		out.Subject = &adminpb.KYCApplication_Individual{Individual: &adminpb.IndividualKYC{IdCardNoMasked: deref(a.IDCardNoMasked)}}
	case model.KYCCompany:
		b := &adminpb.BusinessKYC{
			CompanyName: deref(a.CompanyName), TaxId: deref(a.TaxID),
			CompanyRegisterDate: toDate(a.CompanyRegisterDate), CompanyPhone: deref(a.CompanyPhone), RepIdCardNoMasked: deref(a.RepIDCardNoMasked),
		}
		if a.CompanyTownCode != nil {
			b.CompanyAddress = &commonpb.PostalAddress{
				TownCode:      *a.CompanyTownCode,
				AddressDetail: deref(a.CompanyAddressDetail),
			}
		}
		out.Subject = &adminpb.KYCApplication_Business{Business: b}
	}
	for _, f := range a.IssuableFields {
		out.IssuableFields = append(out.IssuableFields, kycFieldPB(f))
	}
	for _, d := range a.Documents {
		out.Documents = append(out.Documents, &adminpb.KYCDocument{Field: documentFieldPB(d.Slot), Position: int32(d.Position)}) //nolint:gosec // position < 10
	}
	if a.Flow != nil {
		out.Flow = toFlow(*a.Flow)
	}
	if a.PendingRegistrationCheck != nil {
		out.PendingRegistrationCheck = toRegistrationCheck(*a.PendingRegistrationCheck)
	}
	if a.RegistrationCheck != nil {
		out.RegistrationCheck = toRegistrationCheck(*a.RegistrationCheck)
	}
	for _, r := range a.Reviews {
		review := &adminpb.KYCReview{
			Id: r.ID.String(), Action: rpc.EnumToProto[adminpb.KYCReviewAction](r.Action), Reason: deref(r.Reason),
			DecideTime: timestamppb.New(r.DecidedAt), Issues: toKYCIssues(r.Issues), InternalNote: deref(r.InternalNote),
		}
		if r.ReviewerAdminID != nil {
			review.ReviewerAdminId = rpc.FormatID(*r.ReviewerAdminID)
		}
		if r.RegistrationCheckSeq > 0 {
			review.RegistrationCheckSeq = r.RegistrationCheckSeq
			if r.RegistrationCheckQueriedAt != nil {
				review.RegistrationCheckQueryTime = timestamppb.New(*r.RegistrationCheckQueriedAt)
			}
		}
		out.Reviews = append(out.Reviews, review)
	}
	if a.SubmittedAt != nil {
		out.SubmitTime = timestamppb.New(*a.SubmittedAt)
	}
	if a.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*a.DecidedAt)
	}
	return out
}

// toRegistrationCheck 轉成登記資料查詢的契約型別；只有 FOUND 帶登記資料與比對結果。進行中（PENDING）的查詢沒有結果（outcome 是
// UNSPECIFIED），只有序號、要求時間與期限。
func toRegistrationCheck(c model.RegistrationCheck) *adminpb.KYCRegistrationCheck {
	out := &adminpb.KYCRegistrationCheck{Seq: c.Seq}
	if c.Outcome != model.RegistrationPending {
		out.Outcome = rpc.EnumToProto[adminpb.KYCRegistrationOutcome](c.Outcome)
	}
	if !c.RequestedAt.IsZero() {
		out.RequestTime = timestamppb.New(c.RequestedAt)
	}
	if c.Deadline != nil {
		out.DeadlineTime = timestamppb.New(*c.Deadline)
	}
	if c.QueriedAt != nil {
		out.QueryTime = timestamppb.New(*c.QueriedAt)
	}
	for _, f := range c.MismatchedFields {
		out.MismatchedFields = append(out.MismatchedFields, kycFieldPB(f))
	}
	for _, f := range c.UnverifiableFields {
		out.UnverifiableFields = append(out.UnverifiableFields, kycFieldPB(f))
	}
	r := c.Registration
	if r == nil {
		return out
	}
	reg := &adminpb.KYCRegistration{
		TaxId: r.TaxID, Name: r.Name, Address: r.Address, SetupDate: toCalendarDate(r.SetupDate), OrganizationType: r.OrganizationType,
		Status: rpc.EnumToProto[adminpb.KYCRegistrationStatus](r.Status), StatusText: r.StatusText,
		SuspendStartDate: toCalendarDate(r.SuspendedFrom), SuspendEndDate: toCalendarDate(r.SuspendedUntil),
	}
	if c.QueriedAt != nil {
		reg.SuspendedAtQuery = r.SuspendedAt(*c.QueriedAt)
	}
	if r.TownCode != "" {
		reg.PostalAddress = &commonpb.PostalAddress{TownCode: r.TownCode, AddressDetail: r.AddressDetail}
	}
	out.Registration = reg
	return out
}

// toCalendarDate 把日曆日（UTC 午夜）轉成 Date；零值是沒有。
func toCalendarDate(t time.Time) *date.Date {
	if t.IsZero() {
		return nil
	}
	return &date.Date{Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
}

func toAdminPayUni(v model.AdminPayUniRegistration) *adminpb.PayUniRegistration {
	r := v.PayUniRegistration
	out := &adminpb.PayUniRegistration{
		StoreId: rpc.FormatID(r.StoreID), MerId: r.MerID, ApplicationId: r.ApplicationID.String(),
		BindStatus: rpc.EnumToProto[pb.PayUniBindStatus](r.BindStatus), BindErrorCode: deref(r.BindErrorCode),
		BindAttempts: int32(r.BindAttempts), //nolint:gosec // 嘗試次數遠小於 int32
		StatusCodes: &adminpb.PayUniStatusCodes{
			MerStatus: deref(r.Status.MerStatus), MemberStatus: deref(r.Status.MemberStatus), CreditStatus: deref(r.Status.CreditStatus),
			ApplePayStatus: deref(r.Status.ApplePayStatus), GooglePayStatus: deref(r.Status.GooglePayStatus), AtmStatus: deref(r.Status.ATMStatus),
			Reason: deref(r.Status.Reason),
		},
		Problem: toProblem(model.PayUniProblemOf(r)), FlowStatus: rpc.EnumToProto[pb.KYCFlowStatus](v.FlowStatus),
		CanRebind: len(v.RebindUnavailable) == 0, RebindUnavailabilities: toStoreActionUnavailabilities(v.RebindUnavailable, model.StoreActionRebindPayUni),
		SyncPending: r.SyncPending, SyncErrorCode: deref(r.SyncErrorCode),
		CreateTime: timestamppb.New(r.CreatedAt), UpdateTime: timestamppb.New(r.UpdatedAt),
	}
	if r.BoundAt != nil {
		out.BindTime = timestamppb.New(*r.BoundAt)
	}
	if r.Status.ObservedAt != nil {
		out.StatusTime = timestamppb.New(*r.Status.ObservedAt)
	}
	if r.Status.CheckedAt != nil {
		out.CheckTime = timestamppb.New(*r.Status.CheckedAt)
	}
	return out
}

func toObservation(o model.PayUniObservation) *adminpb.PayUniObservation {
	return &adminpb.PayUniObservation{
		Id: o.ID.String(), Source: rpc.EnumToProto[adminpb.PayUniObservationSource](o.Source), EventKey: deref(o.EventKey),
		StatusCodes: &adminpb.PayUniStatusCodes{
			MerStatus: deref(o.MerStatus), MemberStatus: deref(o.MemberStatus), CreditStatus: deref(o.CreditStatus),
			ApplePayStatus: deref(o.ApplePayStatus), GooglePayStatus: deref(o.GooglePayStatus), AtmStatus: deref(o.ATMStatus), Reason: deref(o.Reason),
		},
		ExecuteTime: timestamppb.New(o.ExecutedAt), ReceiveTime: timestamppb.New(o.ReceivedAt),
	}
}
