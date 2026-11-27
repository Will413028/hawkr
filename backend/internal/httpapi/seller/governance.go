package seller

import (
	"context"
	"errors"
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
	"github.com/Will413028/hawkr/backend/internal/modules/social"
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
	MarkLinkedAccountReviewed(context.Context, model.MarkLinkedAccountReviewedInput) error
	DecideAppeal(context.Context, model.DecideAppealInput) (model.Measure, error)
	DecideRemediation(context.Context, model.DecideRemediationInput) (model.Measure, error)
	CreateMeasureNote(context.Context, model.CreateMeasureNoteInput) (model.GovernanceNote, error)
	MeasureNotes(context.Context, model.MeasureNoteListInput) (model.NotePage, error)
	Violation(ctx context.Context, violationID uuid.UUID) (model.Violation, error)
	AdminViolations(context.Context, model.ViolationListInput) (model.ViolationPage, error)
}

// Governance 是 SellerGovernanceAdminService 的 handler。授權（管理員、每支方法的權限）由 adminauth 的 interceptor 檢查。
type Governance struct {
	m         GovernanceModule
	images    rpc.Images
	snapshots ContentSnapshots
	links     LinkAvailability
}

// ContentSnapshots 回報一則內容此刻的歸屬與內容快照，給後台在建立內容下架前一次讀取、同時核對歸屬與取得處分依據的內容版本（social.Module 實作）。
type ContentSnapshots interface {
	ContentSnapshot(ctx context.Context, t social.Target) (social.ContentSnapshot, error)
}

var _ sellerv1alpha1connect.SellerGovernanceAdminServiceHandler = (*Governance)(nil)

// GovernanceDeps 是 NewGovernance 的相依，全部必填。
type GovernanceDeps struct {
	Module GovernanceModule
	Images rpc.Images
	// ContentSnapshots 在後台建立內容下架前核對內容歸屬並取得內容快照。
	ContentSnapshots ContentSnapshots
	// Links 回報措施內容快照的連結目標還在不在（讀措施時用）。
	Links LinkAvailability
}

// NewGovernance 建立後台治理的 handler；Images 給轉報資料裡店家的 logo 與封面。後台直接建立內容下架時，內容須是這家店現存（沒有刪除）的內容，由 handler 向 social 讀取快照核對並取得內容版本；social 的同 transaction 訂閱者另擋別家店的內容。
func NewGovernance(d GovernanceDeps) (*Governance, error) {
	if d.Module == nil {
		return nil, errors.New("seller governance：需要 Module")
	}
	if d.ContentSnapshots == nil {
		return nil, errors.New("seller governance：需要 ContentSnapshots")
	}
	if d.Links == nil {
		return nil, errors.New("seller governance：需要 Links")
	}
	return &Governance{m: d.Module, images: d.Images, snapshots: d.ContentSnapshots, links: d.Links}, nil
}

var (
	listReportsPaths       = rpc.FieldPaths{}
	triagePaths            = rpc.FieldPaths{"AssessedCategory": "assessed_category", "Target": "target", "ExpectedVersion": "etag"}
	markSeenPaths          = rpc.FieldPaths{"ExpectedVersion": "etag"}
	resolvePaths           = rpc.FieldPaths{"ExpectedVersion": "etag"}
	listCasesPaths         = rpc.FieldPaths{}
	decideCasePaths        = rpc.FieldPaths{"ExpectedVersion": "etag"}
	changeSeverityPaths    = rpc.FieldPaths{"ExpectedVersion": "etag"}
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
	revokeMeasurePaths     = rpc.FieldPaths{"ExpectedVersion": "etag"}
	scheduleReviewPaths    = rpc.FieldPaths{"ExpectedVersion": "etag"}
	decideAppealPaths      = rpc.FieldPaths{"Outcome": "decision", "ExpectedVersion": "etag"}
	decideRemediationPaths = rpc.FieldPaths{"Outcome": "decision", "ExpectedVersion": "etag"}
	listMeasuresPaths      = rpc.FieldPaths{}
	listViolationsPaths    = rpc.FieldPaths{}
)

func reportEvidenceURI(reportID uuid.UUID, position int) string {
	return "/api/files/admin/seller/reports/" + reportID.String() + "/evidence/" + strconv.Itoa(position)
}

func requestEvidenceURI(requestID uuid.UUID, position int) string {
	return "/api/files/admin/seller/measure_requests/" + requestID.String() + "/evidence/" + strconv.Itoa(position)
}

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

func toReport(r model.AdminReport) *adminpb.Report {
	outcomes := r.Availability.AvailableOutcomes()
	out := &adminpb.Report{
		Id: r.ID.String(), StoreId: rpc.FormatID(r.StoreID), StoreName: deref(r.StoreName), ProductId: deref(r.ProductID), OrderNo: deref(r.OrderNo),
		Category: rpc.EnumToProto[commonpb.PolicyCategory](r.Category), Description: r.Description,
		State: rpc.EnumToProto[adminpb.ReportState](r.State()), TriagedByAdminId: formatAdminID(r.TriagedByAdminID),
		TriageTime: optionalTimestamp(r.TriagedAt), TriageDeadline: toDeadline(r.TriageDeadline),
		ResolvedByAdminId: formatAdminID(r.ResolvedByAdminID), ResolveTime: optionalTimestamp(r.ResolvedAt), ResolutionNote: deref(r.ResolutionNote),
		SupplementCount: int32(r.SupplementCount), LatestSupplementTime: optionalTimestamp(r.LatestSupplementAt), //nolint:gosec // 補充最多 MaxSupplements 則
		Etag:      rpc.Etag(r.Version),
		CanTriage: len(r.Availability.Triage) == 0, CanResolve: len(outcomes) > 0, CanMove: len(r.Availability.Move) == 0,
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
		o := rpc.EnumToProto[pb.ReportOutcome](*r.Outcome)
		out.Outcome = &o
	}
	out.ReporterBuyerId = rpc.FormatID(r.ReporterBuyerID)
	out.Evidence = toEvidence(len(r.Evidence), func(i int) string { return reportEvidenceURI(r.ID, i) })
	for _, s := range r.Supplements {
		supplement := &adminpb.ReportSupplement{Id: s.ID.String(), Description: s.Description, CreateTime: timestamppb.New(s.CreatedAt)}
		for _, position := range s.EvidencePositions {
			supplement.EvidencePositions = append(supplement.EvidencePositions, int32(position)) //nolint:gosec // 位置最多 MaxReportEvidence
		}
		out.Supplements = append(out.Supplements, supplement)
	}
	return out
}

func toTriageTarget(t model.TriageTarget) *adminpb.TriageTarget {
	out := &adminpb.TriageTarget{Category: rpc.EnumToProto[commonpb.PolicyCategory](t.Category)}
	if c := t.Case; c != nil {
		out.CaseId, out.Severity, out.CaseEtag = c.ID.String(), rpc.EnumToProto[pb.ViolationSeverity](c.Severity), rpc.Etag(c.Version)
		out.CaseReportCount = int32(c.ReportCount) //nolint:gosec // 計數不會超過 int32
	}
	return out
}

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
		out.Reports[i] = toReport(r)
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
	adminID, err := rpc.RequireAdmin(ctx)
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
	if _, err := rpc.RequireAdmin(ctx); err != nil {
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
	adminID, err := rpc.RequireAdmin(ctx)
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
	adminID, err := rpc.RequireAdmin(ctx)
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
	adminID, err := rpc.RequireAdmin(ctx)
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
	adminID, err := rpc.RequireAdmin(ctx)
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
	adminID, err := rpc.RequireAdmin(ctx)
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

func createMeasurePaths(parts ...rpc.FieldPaths) rpc.FieldPaths {
	out := rpc.FieldPaths{"RequestID": "request_id", "StoreID": "store_id", "StatementToSeller": "statement", "Note": "note", "SourceCaseID": "source_case_id", "SourceAuthorityOrderID": "source_authority_order_id"}
	for _, p := range parts {
		maps.Copy(out, p)
	}
	return out
}

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
