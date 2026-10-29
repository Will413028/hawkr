package seller

import (
	"context"
	"maps"
	"strconv"
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
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// GovernanceModule 是後台治理 handler 使用的模組方法（ADR 2026-10-18-seller-governance-consolidated）。
type GovernanceModule interface {
	AdminReports(context.Context, model.ReportListInput) (model.AdminReportPage, error)
	Report(ctx context.Context, reportID uuid.UUID) (model.AdminReport, error)
	TriageReport(context.Context, model.TriageReportInput) (model.AdminReport, error)
	MarkReportSupplementsSeen(context.Context, model.MarkSupplementsSeenInput) (model.AdminReport, error)
	ResolveReport(context.Context, model.ResolveReportInput) (model.AdminReport, error)
	MoveReport(context.Context, model.MoveReportInput) (model.MoveReportResult, error)
	AdminCases(context.Context, model.CaseListInput) (model.AdminCasePage, error)
	Case(ctx context.Context, caseID uuid.UUID) (model.AdminCase, error)
	DecideCase(context.Context, model.DecideCaseInput) (model.AdminCase, error)
	ChangeCaseSeverity(context.Context, model.ChangeCaseSeverityInput) (model.AdminCase, error)
	CreateCaseInquiry(context.Context, model.CreateCaseInquiryInput) (model.CreateCaseInquiryResult, error)
	CaseInquiries(context.Context, model.CaseInquiryListInput) (model.AdminInquiryPage, error)
	RecordCaseInquiryReply(context.Context, model.RecordCaseInquiryReplyInput) (model.RecordCaseInquiryReplyResult, error)
	WithdrawCaseInquiry(context.Context, model.WithdrawCaseInquiryInput) (model.WithdrawCaseInquiryResult, error)
	CreateAuthorityOrder(context.Context, model.CreateAuthorityOrderInput) (model.CreateAuthorityOrderResult, error)
	AuthorityOrders(context.Context, model.AuthorityOrderListInput) (model.AdminAuthorityOrderPage, error)
	AuthorityOrder(ctx context.Context, id uuid.UUID) (model.AdminAuthorityOrder, error)
	CompleteAuthorityOrder(context.Context, model.CompleteAuthorityOrderInput) (model.AdminAuthorityOrder, error)
	ExtendAuthorityOrderHold(context.Context, model.ExtendAuthorityOrderHoldInput) (model.AdminAuthorityOrder, error)
	RecordAuthorityTargetNotice(context.Context, model.RecordAuthorityTargetNoticeInput) (model.AdminAuthorityOrder, error)
	RevokeAuthorityOrder(context.Context, model.RevokeAuthorityOrderInput) (model.RevokeAuthorityOrderResult, error)
	CreateCaseNote(context.Context, model.CreateCaseNoteInput) (model.GovernanceNote, error)
	CaseNotes(context.Context, model.CaseNoteListInput) (model.NotePage, error)
	CreateMeasure(context.Context, model.CreateMeasureInput) (model.CreateMeasureResult, error)
	Measure(ctx context.Context, measureID uuid.UUID) (model.Measure, error)
	CurrentMeasure(ctx context.Context, measureID uuid.UUID) (model.Measure, error)
	AdminMeasures(context.Context, model.AdminMeasureListInput) (model.MeasurePage, error)
	UpdateMeasure(context.Context, model.UpdateMeasureInput) (model.Measure, error)
	EndMeasure(context.Context, model.EndMeasureInput) (model.Measure, error)
	RevokeMeasure(context.Context, model.RevokeMeasureInput) (model.Measure, error)
	ScheduleMeasureReview(context.Context, model.ScheduleReviewInput) (model.Measure, error)
	DecideAppeal(context.Context, model.DecideAppealInput) (model.Measure, error)
	DecideRemediation(context.Context, model.DecideRemediationInput) (model.Measure, error)
	CreateMeasureNote(context.Context, model.CreateMeasureNoteInput) (model.GovernanceNote, error)
	MeasureNotes(context.Context, model.MeasureNoteListInput) (model.NotePage, error)
	Violation(ctx context.Context, violationID uuid.UUID) (model.Violation, error)
	AdminViolations(context.Context, model.ViolationListInput) (model.ViolationPage, error)
}

// Governance 是 SellerGovernanceAdminService 的 handler。授權（管理員、每支方法的權限）由 adminauth 的 interceptor 檢查。
type Governance struct {
	m      GovernanceModule
	images rpc.Images
}

var _ sellerv1alpha1connect.SellerGovernanceAdminServiceHandler = (*Governance)(nil)

// NewGovernance 建立後台治理的 handler；images 給轉報資料裡店家的 logo 與封面。
func NewGovernance(m GovernanceModule, images rpc.Images) *Governance {
	return &Governance{m: m, images: images}
}

// 模組輸入的欄位對應到請求的路徑。
var (
	listReportsPaths       = rpc.FieldPaths{"State": "state", "Category": "category", "StoreID": "store_id", "CaseID": "case_id"}
	triagePaths            = rpc.FieldPaths{"AssessedCategory": "assessed_category", "Target": "target", "ExpectedVersion": "etag"}
	markSeenPaths          = rpc.FieldPaths{"ExpectedVersion": "etag"}
	resolvePaths           = rpc.FieldPaths{"Outcome": "outcome", "Note": "note", "ExpectedVersion": "etag"}
	listCasesPaths         = rpc.FieldPaths{"State": "state", "Category": "category", "StoreID": "store_id", "AwaitingSeller": "awaiting_seller"}
	decideCasePaths        = rpc.FieldPaths{"Decision": "decision", "Note": "note", "ExpectedVersion": "etag"}
	changeSeverityPaths    = rpc.FieldPaths{"Severity": "severity", "Note": "note", "ExpectedVersion": "etag"}
	createNotePaths        = rpc.FieldPaths{"Body": "note.body"}
	createMeasureNotePaths = createNotePaths
	createWarningPaths     = createMeasurePaths(violationPaths("Violation", "warning.violation"))
	createSuspensionPaths  = createMeasurePaths(violationPaths("Violation", "suspension.violation"), rpc.FieldPaths{"Risk.Ground": "suspension.risk.ground", "EndsAt": "suspension.end_time", "ReviewBy": "suspension.review_by"})
	createTerminationPaths = createMeasurePaths(violationPaths("Termination.Violation", "termination.violation"), rpc.FieldPaths{
		"Termination": "termination", "Termination.Ground": "termination.ground", "Termination.SourceMeasureID": "termination.source_measure_id",
	})
	createTakedownPaths = createMeasurePaths(violationPaths("Violation", "content_takedown.violation"), rpc.FieldPaths{
		"Takedown": "content_takedown", "Takedown.Contents": "content_takedown.contents", "Takedown.Contents[]": "content_takedown.contents[]",
	})
	updateMeasurePaths     = rpc.FieldPaths{"StatementToSeller": "measure.statement", "EndsAt": "measure.end_time", "ExpectedVersion": "measure.etag"}
	endMeasurePaths        = rpc.FieldPaths{"ExpectedVersion": "etag"}
	revokeMeasurePaths     = rpc.FieldPaths{"Reason": "reason", "RevokeFindings": "revoke_findings", "ExpectedVersion": "etag"}
	scheduleReviewPaths    = rpc.FieldPaths{"ReviewBy": "review_by", "Note": "note", "ExpectedVersion": "etag"}
	decideAppealPaths      = rpc.FieldPaths{"Reason": "reason", "Outcome": "decision", "RevokeFindings": "revoke_findings", "ExpectedVersion": "etag"}
	decideRemediationPaths = rpc.FieldPaths{"Reason": "reason", "Outcome": "decision", "ExpectedVersion": "etag"}
	listMeasuresPaths      = rpc.FieldPaths{
		"StoreID": "store_id", "AccountID": "account_id", "State": "state", "Basis": "basis", "RequestState": "request_state", "RequestKind": "request_kind",
		"SourceCaseID": "source_case_id", "SourceReportID": "source_report_id", "SourceAuthorityOrderID": "source_authority_order_id",
	}
	listViolationsPaths = rpc.FieldPaths{"StoreID": "store_id", "AccountID": "account_id", "Status": "status"}
)

// reportEvidenceURI 與 requestEvidenceURI 是佐證的檢視網址（EvidenceReportRoute、EvidenceRequestRoute 的實例）。
func reportEvidenceURI(reportID uuid.UUID, position int) string {
	return "/api/files/admin/seller/reports/" + reportID.String() + "/evidence/" + strconv.Itoa(position)
}

func requestEvidenceURI(requestID uuid.UUID, position int) string {
	return "/api/files/admin/seller/measure_requests/" + requestID.String() + "/evidence/" + strconv.Itoa(position)
}

// toEvidence 只帶位置與檢視網址：asset ID 與 object key 不對外（佐證是機敏檔，只經擁有者的端點讀）。
func toEvidence(n int, uri func(int) string) []*adminpb.Evidence {
	out := make([]*adminpb.Evidence, n)
	for i := range out {
		out[i] = &adminpb.Evidence{Position: int32(i), ViewUri: uri(i)} //nolint:gosec // 佐證最多 MaxEvidence 張（通報加補充最多 MaxReportEvidence 張）
	}
	return out
}

func formatAdminID(id *int64) string {
	if id == nil {
		return ""
	}
	return rpc.FormatID(*id)
}

// toDeadline 把模組的期限轉成契約的 Deadline；義務不在（nil）時回 nil，欄位不設。
func toDeadline(d *model.Deadline) *commonpb.Deadline {
	if d == nil {
		return nil
	}
	return &commonpb.Deadline{StartTime: timestamppb.New(d.Start), DueTime: timestamppb.New(d.Due)}
}

func optionalTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// toReport 把後台看到的通報轉成契約：處理狀態與 can_*、available_outcomes 取自同一個 Availability（不輸出原因）。補充與佐證只在讀單筆與匯出時載入（清單的是空的）。
func toReport(r model.AdminReport) *adminpb.Report {
	outcomes := r.Availability.AvailableOutcomes()
	out := &adminpb.Report{
		Id: r.ID.String(), StoreId: rpc.FormatID(r.StoreID), StoreName: deref(r.StoreName), ProductId: deref(r.ProductID), OrderNo: deref(r.OrderNo),
		Category: rpc.EnumToProto[commonpb.PolicyCategory](r.Category), Description: r.Description,
		State: rpc.EnumToProto[adminpb.ReportState](r.State()), TriagedByAdminId: formatAdminID(r.TriagedByAdminID),
		TriageTime: optionalTimestamp(r.TriagedAt), TriageDeadline: toDeadline(r.TriageDeadline),
		ResolvedByAdminId: formatAdminID(r.ResolvedByAdminID), ResolveTime: optionalTimestamp(r.ResolvedAt), ResolutionNote: deref(r.ResolutionNote),
		SupplementCount: int32(r.SupplementCount), LatestSupplementTime: optionalTimestamp(r.LatestSupplementAt), //nolint:gosec // 補充最多 MaxSupplements 則
		DiscardedSubmissionCount: int32(r.DiscardedSubmissions), //nolint:gosec // 計數不會超過 int32
		Etag:                     rpc.Etag(r.Version),
		CanTriage:                len(r.Availability.Triage) == 0, CanResolve: len(outcomes) > 0, CanMove: len(r.Availability.Move) == 0,
		CanCiteAsSource: len(r.Availability.CiteAsSource) == 0, EffectiveCategory: rpc.EnumToProto[commonpb.PolicyCategory](r.EffectiveCategory()),
		HasUnseenSupplements: r.HasUnseenSupplements(),
		CreateTime:           timestamppb.New(r.CreatedAt), UpdateTime: timestamppb.New(r.UpdatedAt),
	}
	for _, o := range outcomes {
		out.AvailableOutcomes = append(out.AvailableOutcomes, &adminpb.OutcomeOption{Outcome: rpc.EnumToProto[pb.ReportOutcome](o), NegatesSource: o.Negates()})
	}
	for _, t := range r.Targets {
		out.TriageTargets = append(out.TriageTargets, toTriageTarget(t))
	}
	if r.CaseID != nil {
		out.CaseId = r.CaseID.String()
	}
	if r.AssessedCategory != nil {
		out.AssessedCategory = rpc.EnumToProto[commonpb.PolicyCategory](*r.AssessedCategory)
	}
	if r.Outcome != nil {
		out.Outcome = rpc.EnumToProto[pb.ReportOutcome](*r.Outcome)
	}
	if r.ReporterBuyerID != nil {
		out.ReporterBuyerId = rpc.FormatID(*r.ReporterBuyerID)
	}
	if c := r.Contact; c != nil {
		out.Contact = &adminpb.ReporterContact{Phone: deref(c.Phone), Email: deref(c.Email), Name: deref(c.Name)}
	}
	if r.ContactErasedAt != nil {
		out.ContactEraseTime = timestamppb.New(*r.ContactErasedAt)
	}
	out.Evidence = toEvidence(len(r.Evidence), func(i int) string { return reportEvidenceURI(r.ID, i) })
	for _, s := range r.Supplements {
		supplement := &adminpb.ReportSupplement{Id: s.ID.String(), Description: s.Description, CreateTime: timestamppb.New(s.CreatedAt)}
		for _, position := range s.EvidencePositions {
			supplement.EvidencePositions = append(supplement.EvidencePositions, int32(position)) //nolint:gosec // 位置最多 MaxReportEvidence
		}
		if s.MatchedBy != nil {
			supplement.MatchedBy = rpc.EnumToProto[adminpb.SupplementMatch](*s.MatchedBy)
		}
		out.Supplements = append(out.Supplements, supplement)
	}
	return out
}

// toTriageTarget 把分案或移動的去處轉成契約：有進行中的案件就帶它的 id、嚴重度與 etag，沒有就只有類別（開新案件）。
func toTriageTarget(t model.TriageTarget) *adminpb.TriageTarget {
	out := &adminpb.TriageTarget{Category: rpc.EnumToProto[commonpb.PolicyCategory](t.Category)}
	if c := t.Case; c != nil {
		out.CaseId, out.Severity, out.CaseEtag = c.ID.String(), rpc.EnumToProto[pb.ViolationSeverity](c.Severity), rpc.Etag(c.Version)
		out.CaseReportCount = int32(c.ReportCount) //nolint:gosec // 計數不會超過 int32
	}
	return out
}

// caseTargetFrom 解析請求的去處（oneof）；沒有設時回空的 CaseTarget，由模組以欄位錯誤回應。
func caseTargetFrom(t *adminpb.CaseTarget) (model.CaseTarget, error) {
	switch v := t.GetTarget().(type) {
	case *adminpb.CaseTarget_Join:
		caseID, err := rpc.ParseUUID("target.join.case_id", v.Join.GetCaseId())
		if err != nil {
			return model.CaseTarget{}, err
		}
		version, err := rpc.RequiredVersionField(v.Join.GetEtag(), "target.join.etag")
		if err != nil {
			return model.CaseTarget{}, err
		}
		return model.CaseTarget{Join: &model.JoinCase{CaseID: caseID, ExpectedVersion: version}}, nil
	case *adminpb.CaseTarget_OpenNew:
		severity := rpc.EnumFromProto[model.Severity](v.OpenNew.GetSeverity())
		return model.CaseTarget{OpenNew: &severity}, nil
	}
	return model.CaseTarget{}, nil
}

// civilDate 把 google.type.Date 轉成 UTC 午夜的 time.Time（模組的日期形狀）；沒設是 nil。不是有效日期（年月日超出範圍或不存在的日子）回零值，
// 模組把零值當成「早於任何日期」，回該欄位原本的範圍句子，所以不另加一種錯誤。
func civilDate(d *date.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
	if t.Year() != int(d.GetYear()) || int(t.Month()) != int(d.GetMonth()) || t.Day() != int(d.GetDay()) || d.GetYear() < 1 {
		return &time.Time{}
	}
	return &t
}

// toCase 把後台看到的案件轉成契約：can_*、decision_options 取自同一個 Availability（不輸出原因）。
func toCase(c model.AdminCase) *adminpb.Case {
	options := c.Availability.DecisionOptions()
	out := &adminpb.Case{
		Id: c.ID.String(), StoreId: rpc.FormatID(c.StoreID), StoreName: deref(c.StoreName), Category: rpc.EnumToProto[commonpb.PolicyCategory](c.Category),
		Severity: rpc.EnumToProto[pb.ViolationSeverity](c.Severity), State: rpc.EnumToProto[adminpb.CaseState](c.State()),
		DecisionNote: deref(c.DecisionNote), OpenedByAdminId: rpc.FormatID(c.OpenedByAdminID), OpenTime: timestamppb.New(c.OpenedAt),
		ClosedByAdminId: formatAdminID(c.ClosedByAdminID), CloseTime: optionalTimestamp(c.ClosedAt),
		ReportCount: int32(c.ReportCount), UnresolvedReportCount: int32(c.UnresolvedReportCount), UnseenSupplementReportCount: int32(c.UnseenSupplementReports), //nolint:gosec // 計數不會超過 int32
		UpdateTime: timestamppb.New(c.UpdatedAt),
		Etag:       rpc.Etag(c.Version), CanDecide: len(options) > 0, CanChangeSeverity: len(c.Availability.ChangeSeverity) == 0,
		CanLinkMeasure: len(c.Availability.LinkMeasure) == 0, CanCreateMeasure: len(c.Availability.CreateMeasure) == 0,
		AwaitingSeller: c.AwaitingSeller(), CanCreateInquiry: len(c.Availability.CreateInquiry) == 0,
		DecideDeadline: toDeadline(c.DecideDeadline),
	}
	if c.AwaitingSeller() {
		out.OpenInquiryId = c.OpenInquiryID.String()
	}
	if c.CloseReason != nil {
		out.CloseReason = rpc.EnumToProto[adminpb.CaseCloseReason](*c.CloseReason)
	}
	if c.Decision != nil {
		out.Decision = rpc.EnumToProto[adminpb.CaseDecision](*c.Decision)
	}
	for _, d := range options {
		out.DecisionOptions = append(out.DecisionOptions, &adminpb.DecisionOption{
			Decision: rpc.EnumToProto[adminpb.CaseDecision](d), DefaultOutcome: rpc.EnumToProto[pb.ReportOutcome](d.DefaultOutcome()),
		})
	}
	for _, m := range c.Moves {
		out.Moves = append(out.Moves, &adminpb.ReportMove{
			ReportId: m.ReportID.String(), FromCaseId: m.FromCaseID.String(), ToCaseId: m.ToCaseID.String(),
			MovedByAdminId: rpc.FormatID(m.AdminID), MoveTime: timestamppb.New(m.MovedAt),
		})
	}
	return out
}

// parseOptionalStoreID 解析選填的店家 ID 篩選：空字串是不篩（0）。
func parseOptionalStoreID(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	return rpc.ParseID("store_id", raw)
}

func (h *Governance) ListReports(ctx context.Context, req *connect.Request[adminpb.ListReportsRequest]) (*connect.Response[adminpb.ListReportsResponse], error) {
	msg := req.Msg
	in := model.ReportListInput{PageSize: msg.GetPageSize()}
	if state := rpc.EnumFromProto[model.ReportState](msg.GetState()); state != "" {
		in.State = &state
	}
	if category := rpc.EnumFromProto[policy.Category](msg.GetCategory()); category != "" {
		in.Category = &category
	}
	var err error
	if in.StoreID, err = parseOptionalStoreID(msg.GetStoreId()); err != nil {
		return nil, err
	}
	if msg.GetCaseId() != "" {
		caseID, err := rpc.ParseUUID("case_id", msg.GetCaseId())
		if err != nil {
			return nil, err
		}
		in.CaseID = &caseID
	}
	in.CitableAsSource = msg.GetCitableAsSource()
	// page token 只能用在同一組篩選：換了篩選就從第一頁開始。
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListReportsProcedure, msg.GetState().String(), msg.GetCategory().String(), msg.GetStoreId(), msg.GetCaseId(), strconv.FormatBool(msg.GetCitableAsSource()))
	after, err := rpc.ValidCursor[model.ReportCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.AdminReports(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listReportsPaths)
	}
	out := &adminpb.ListReportsResponse{Reports: make([]*adminpb.Report, len(page.Items))}
	for i, r := range page.Items {
		// 清單不帶匿名檢舉人的聯絡方式（個資只在讀單筆時出現，與佐證相同）。
		out.Reports[i] = toReport(r)
		out.Reports[i].Contact = nil
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Governance) GetReport(ctx context.Context, req *connect.Request[adminpb.GetReportRequest]) (*connect.Response[adminpb.GetReportResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	r, err := h.m.Report(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetReportResponse{Report: toReport(r)}), nil
}

func (h *Governance) TriageReport(ctx context.Context, req *connect.Request[adminpb.TriageReportRequest]) (*connect.Response[adminpb.TriageReportResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	target, err := caseTargetFrom(msg.GetTarget())
	if err != nil {
		return nil, err
	}
	in := model.TriageReportInput{
		ReportID: id, AdminID: adminID, AssessedCategory: rpc.EnumFromProto[policy.Category](msg.GetAssessedCategory()), Target: target, ExpectedVersion: version,
	}
	r, err := h.m.TriageReport(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, triagePaths)
	}
	return connect.NewResponse(&adminpb.TriageReportResponse{Report: toReport(r)}), nil
}

func (h *Governance) MarkReportSupplementsSeen(ctx context.Context, req *connect.Request[adminpb.MarkReportSupplementsSeenRequest]) (*connect.Response[adminpb.MarkReportSupplementsSeenResponse], error) {
	if _, err := adminFrom(ctx); err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	r, err := h.m.MarkReportSupplementsSeen(ctx, model.MarkSupplementsSeenInput{ReportID: id, ExpectedVersion: version})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, markSeenPaths)
	}
	return connect.NewResponse(&adminpb.MarkReportSupplementsSeenResponse{Report: toReport(r)}), nil
}

func (h *Governance) ResolveReport(ctx context.Context, req *connect.Request[adminpb.ResolveReportRequest]) (*connect.Response[adminpb.ResolveReportResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	r, err := h.m.ResolveReport(ctx, model.ResolveReportInput{
		ReportID: id, AdminID: adminID, Outcome: rpc.EnumFromProto[model.Outcome](msg.GetOutcome()), Note: msg.GetNote(), ExpectedVersion: version,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, resolvePaths)
	}
	return connect.NewResponse(&adminpb.ResolveReportResponse{Report: toReport(r)}), nil
}

func (h *Governance) MoveReport(ctx context.Context, req *connect.Request[adminpb.MoveReportRequest]) (*connect.Response[adminpb.MoveReportResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	target, err := caseTargetFrom(msg.GetTarget())
	if err != nil {
		return nil, err
	}
	in := model.MoveReportInput{
		ReportID: id, AdminID: adminID, AssessedCategory: rpc.EnumFromProto[policy.Category](msg.GetAssessedCategory()), Target: target, ExpectedVersion: version,
	}
	result, err := h.m.MoveReport(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, triagePaths)
	}
	return connect.NewResponse(&adminpb.MoveReportResponse{Report: toReport(result.Report), SourceCaseClosed: result.SourceCaseClosed}), nil
}

func (h *Governance) ListCases(ctx context.Context, req *connect.Request[adminpb.ListCasesRequest]) (*connect.Response[adminpb.ListCasesResponse], error) {
	msg := req.Msg
	in := model.CaseListInput{PageSize: msg.GetPageSize()}
	if state := rpc.EnumFromProto[model.CaseState](msg.GetState()); state != "" {
		in.State = &state
	}
	if category := rpc.EnumFromProto[policy.Category](msg.GetCategory()); category != "" {
		in.Category = &category
	}
	var err error
	if in.StoreID, err = parseOptionalStoreID(msg.GetStoreId()); err != nil {
		return nil, err
	}
	in.AwaitingSeller = msg.GetAwaitingSeller()
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListCasesProcedure, msg.GetState().String(), msg.GetCategory().String(), msg.GetStoreId(), strconv.FormatBool(msg.GetAwaitingSeller()))
	after, err := rpc.ValidCursor[model.CaseCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.AdminCases(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listCasesPaths)
	}
	out := &adminpb.ListCasesResponse{Cases: make([]*adminpb.Case, len(page.Items))}
	for i, c := range page.Items {
		out.Cases[i] = toCase(c)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Governance) GetCase(ctx context.Context, req *connect.Request[adminpb.GetCaseRequest]) (*connect.Response[adminpb.GetCaseResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	c, err := h.m.Case(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetCaseResponse{Case: toCase(c)}), nil
}

func (h *Governance) DecideCase(ctx context.Context, req *connect.Request[adminpb.DecideCaseRequest]) (*connect.Response[adminpb.DecideCaseResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	c, err := h.m.DecideCase(ctx, model.DecideCaseInput{
		CaseID: id, AdminID: adminID, Decision: rpc.EnumFromProto[model.Decision](msg.GetDecision()), Note: msg.GetNote(), ExpectedVersion: version,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, decideCasePaths)
	}
	return connect.NewResponse(&adminpb.DecideCaseResponse{Case: toCase(c)}), nil
}

func (h *Governance) ChangeCaseSeverity(ctx context.Context, req *connect.Request[adminpb.ChangeCaseSeverityRequest]) (*connect.Response[adminpb.ChangeCaseSeverityResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	c, err := h.m.ChangeCaseSeverity(ctx, model.ChangeCaseSeverityInput{
		CaseID: id, AdminID: adminID, Severity: rpc.EnumFromProto[model.Severity](msg.GetSeverity()), Note: msg.GetNote(), ExpectedVersion: version,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, changeSeverityPaths)
	}
	return connect.NewResponse(&adminpb.ChangeCaseSeverityResponse{Case: toCase(c)}), nil
}

// CreateCaseNote 新增案件的內部註記：不帶 etag，也不改案件的任何 etag；severity_change 只由 ChangeCaseSeverity 寫，這裡不採用。
func (h *Governance) CreateCaseNote(ctx context.Context, req *connect.Request[adminpb.CreateCaseNoteRequest]) (*connect.Response[adminpb.CreateCaseNoteResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("case_id", req.Msg.GetCaseId())
	if err != nil {
		return nil, err
	}
	note, err := h.m.CreateCaseNote(ctx, model.CreateCaseNoteInput{CaseID: id, AdminID: adminID, Body: req.Msg.GetNote().GetBody()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createNotePaths)
	}
	return connect.NewResponse(&adminpb.CreateCaseNoteResponse{Note: toNote(note)}), nil
}

// ListCaseNotes 是案件內部註記的分頁讀取（舊到新）。page token 綁定案件，換一個案件就從第一頁開始。
func (h *Governance) ListCaseNotes(ctx context.Context, req *connect.Request[adminpb.ListCaseNotesRequest]) (*connect.Response[adminpb.ListCaseNotesResponse], error) {
	msg := req.Msg
	id, err := rpc.ParseUUID("case_id", msg.GetCaseId())
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListCaseNotesProcedure, msg.GetCaseId())
	after, err := rpc.ValidCursor[model.NoteCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.CaseNotes(ctx, model.CaseNoteListInput{CaseID: id, PageSize: msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListCaseNotesResponse{Notes: make([]*adminpb.GovernanceNote, len(page.Items))}
	for i, n := range page.Items {
		out.Notes[i] = toNote(n)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

// createMeasurePaths 是建立措施的欄位對應：共同的欄位加上這一種措施專屬的對應（模組的欄位名不分種類，proto 路徑依設定的是哪一支輸入）。
// SourceCaseID 是來源案件（任何種類與依據都可以帶）：模組在案件不存在或不是這家店的時候以它回 invalid_argument。
func createMeasurePaths(parts ...rpc.FieldPaths) rpc.FieldPaths {
	out := rpc.FieldPaths{"RequestID": "request_id", "StoreID": "store_id", "StatementToSeller": "statement", "Note": "note", "SourceCaseID": "source_case_id", "SourceAuthorityOrderID": "source_authority_order_id"}
	for _, p := range parts {
		maps.Copy(out, p)
	}
	return out
}

// violationPaths 是違規依據（ViolationBasis）的欄位對應：domain 是模組裡依據的欄位名，proto 是它在請求裡的路徑。
func violationPaths(domain, proto string) rpc.FieldPaths {
	return rpc.FieldPaths{
		domain:                                        proto,
		domain + ".CitedFindingIDs[]":                 proto + ".cited_finding_ids[]",
		domain + ".Finding.Category":                  proto + ".finding.category",
		domain + ".Finding.Severity":                  proto + ".finding.severity",
		domain + ".Finding.SourceReportIDs[]":         proto + ".finding.source_report_ids[]",
		domain + ".Finding.SourceAuthorityOrderIDs[]": proto + ".finding.source_authority_order_ids[]",
	}
}
