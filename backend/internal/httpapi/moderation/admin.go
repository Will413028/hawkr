package moderation

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1/moderationv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Cases 是案件的來源與處理者（*moderation.Module）。
type Cases interface {
	Cases(ctx context.Context, f moderation.CaseFilter, after *moderation.CaseCursor, size int32) (moderation.CasePage, error)
	Case(ctx context.Context, id uuid.UUID) (moderation.CaseDetail, error)
	DecideCase(ctx context.Context, in moderation.DecideInput) (moderation.CaseDetail, error)
}

// Admin 實作 ModerationAdminService。授權（只有管理員、逐方法的權限）由 rpc 的 interceptor 與 adminauth 的宣告表檢查。
type Admin struct{ cases Cases }

var _ moderationv1alpha1connect.ModerationAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(cases Cases) *Admin { return &Admin{cases: cases} }

var (
	listPaths   = rpc.FieldPaths{"State": "state", "TargetKind": "target_kind"}
	decidePaths = rpc.FieldPaths{
		"CaseID": "id", "Version": "etag", "Decision": "decision", "Category": "category", "Action": "action",
		"Text": "reason", "Severity": "severity",
	}
)

func (h *Admin) ListCases(ctx context.Context, req *connect.Request[adminpb.ListCasesRequest]) (*connect.Response[adminpb.ListCasesResponse], error) {
	filter := moderation.CaseFilter{
		State:      rpc.EnumFromProto[moderation.CaseState](req.Msg.GetState()),
		TargetKind: rpc.EnumFromProto[moderation.TargetKind](req.Msg.GetTargetKind()),
	}
	scope := rpc.FilterScope(moderationv1alpha1connect.ModerationAdminServiceListCasesProcedure, string(filter.State), string(filter.TargetKind))
	after, err := rpc.ValidCursor[moderation.CaseCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.cases.Cases(ctx, filter, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	out := &adminpb.ListCasesResponse{Cases: make([]*adminpb.Case, len(page.Cases))}
	for i, c := range page.Cases {
		out.Cases[i] = casePB(c)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) GetCase(ctx context.Context, req *connect.Request[adminpb.GetCaseRequest]) (*connect.Response[adminpb.GetCaseResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	d, err := h.cases.Case(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetCaseResponse{Case: casePB(d.Case), Reports: reportsPB(d.Reports)}), nil
}

func (h *Admin) DecideCase(ctx context.Context, req *connect.Request[adminpb.DecideCaseRequest]) (*connect.Response[adminpb.DecideCaseResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
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
	d, err := h.cases.DecideCase(ctx, moderation.DecideInput{
		CaseID: id, Version: version, Actor: adminaudit.Actor{AdminID: admin, Action: req.Spec().Procedure},
		Decision: rpc.EnumFromProto[moderation.Decision](msg.GetDecision()),
		Category: rpc.EnumFromProto[policy.Category](msg.GetCategory()),
		Action:   rpc.EnumFromProto[moderation.Action](msg.GetAction()),
		Text:     msg.GetReason(),
		Severity: rpc.EnumFromProto[string](msg.GetSeverity()),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, decidePaths)
	}
	return connect.NewResponse(&adminpb.DecideCaseResponse{Case: casePB(d.Case), Reports: reportsPB(d.Reports)}), nil
}

func casePB(c moderation.Case) *adminpb.Case {
	out := &adminpb.Case{
		Id:          c.ID.String(),
		Target:      targetPB(c.Target),
		State:       adminpb.CaseState_CASE_STATE_OPEN,
		CreateTime:  timestamppb.New(c.OpenedAt),
		Etag:        rpc.Etag(c.Version),
		ReportCount: c.ReportCount,
	}
	if c.MeasureID != nil {
		id := c.MeasureID.String()
		out.MeasureId = &id
	}
	if c.StoreID != nil {
		id := rpc.FormatID(*c.StoreID)
		out.StoreId = &id
	}
	if c.Decision != nil {
		out.Decision = rpc.EnumToProto[adminpb.CaseDecision](*c.Decision)
	}
	if c.Category != nil {
		out.Category = rpc.EnumToProto[commonpb.PolicyCategory](*c.Category)
	}
	if c.Action != nil {
		out.Action = rpc.EnumToProto[adminpb.CaseAction](*c.Action)
	}
	if c.DecidedByAdmin != nil {
		out.DecidedBy = rpc.FormatID(*c.DecidedByAdmin)
	}
	if c.ClosedAt != nil {
		out.State = adminpb.CaseState_CASE_STATE_CLOSED
		out.ResolveTime = timestamppb.New(*c.ClosedAt)
	}
	return out
}

func reportsPB(reports []moderation.Report) []*adminpb.CaseReport {
	out := make([]*adminpb.CaseReport, len(reports))
	for i, r := range reports {
		out[i] = &adminpb.CaseReport{
			Id:          r.ID.String(),
			ReporterId:  rpc.FormatID(r.ReporterBuyerID),
			Category:    rpc.EnumToProto[commonpb.PolicyCategory](r.Category),
			Description: r.Description,
			Outcome:     rpc.EnumToProto[pb.ReportOutcome](r.Outcome),
			CreateTime:  timestamppb.New(r.CreatedAt),
		}
	}
	return out
}
