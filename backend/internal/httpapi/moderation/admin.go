package moderation

import (
	"context"
	"time"

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
	CaseWithSnapshot(ctx context.Context, id uuid.UUID) (moderation.CaseDetail, *moderation.Snapshot, error)
	DecideCase(ctx context.Context, in moderation.DecideInput) (moderation.CaseDetail, error)
	ReverseHide(ctx context.Context, in moderation.ReverseInput) (moderation.CaseDetail, error)
}

// Admin 實作 ModerationAdminService。授權（只有管理員、逐方法的權限）由 rpc 的 interceptor 與 adminauth 的宣告表檢查。
type Admin struct{ cases Cases }

var _ moderationv1alpha1connect.ModerationAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(cases Cases) *Admin { return &Admin{cases: cases} }

var (
	listPaths    = rpc.FieldPaths{"State": "state", "TargetKind": "target_kind"}
	reversePaths = rpc.FieldPaths{"CaseID": "id", "Reason": "reason"}
	decidePaths  = rpc.FieldPaths{
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
	d, snap, err := h.cases.CaseWithSnapshot(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetCaseResponse{Case: casePB(d.Case), Reports: reportsPB(d.Reports), Snapshot: snapshotPB(snap)}), nil
}

func (h *Admin) ReverseHide(ctx context.Context, req *connect.Request[adminpb.ReverseHideRequest]) (*connect.Response[adminpb.ReverseHideResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	d, err := h.cases.ReverseHide(ctx, moderation.ReverseInput{
		CaseID: id, Actor: adminaudit.Actor{AdminID: admin, Action: req.Spec().Procedure}, Reason: req.Msg.GetReason(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, reversePaths)
	}
	return connect.NewResponse(&adminpb.ReverseHideResponse{Case: casePB(d.Case), Reports: reportsPB(d.Reports)}), nil
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
	if c.Reversal != nil {
		out.Reversal = &adminpb.CaseReversal{ReverseTime: timestamppb.New(c.Reversal.At), ReversedBy: rpc.FormatID(c.Reversal.AdminID), Reason: c.Reversal.Reason}
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
			Category:    rpc.EnumToProto[commonpb.PolicyCategory](r.Category),
			Description: r.Description,
			Outcome:     rpc.EnumToProto[pb.ReportOutcome](r.Outcome),
			CreateTime:  timestamppb.New(r.CreatedAt),
		}
		if r.ReporterStoreID > 0 {
			out[i].Reporter = &adminpb.CaseReport_ReporterStoreId{ReporterStoreId: rpc.FormatID(r.ReporterStoreID)}
		} else {
			out[i].Reporter = &adminpb.CaseReport_ReporterBuyerId{ReporterBuyerId: rpc.FormatID(r.ReporterBuyerID)}
		}
	}
	return out
}

// snapshotPB 把案件的內容快照轉成 oneof；沒有快照回 nil。
func snapshotPB(c *moderation.Snapshot) *adminpb.CaseSnapshot {
	switch {
	case c == nil:
		return nil
	case c.Social != nil:
		x := c.Social
		return &adminpb.CaseSnapshot{Content: &adminpb.CaseSnapshot_Social{Social: &adminpb.SocialSnapshot{
			AuthorKind: x.AuthorKind, AuthorId: rpc.FormatID(x.AuthorID), Body: x.Body, AssetIds: x.MediaRefs, AuthoredTime: optionalTime(x.AuthoredAt),
		}}}
	case c.Buyer != nil:
		return &adminpb.CaseSnapshot{Content: &adminpb.CaseSnapshot_Buyer{Buyer: &adminpb.BuyerSnapshot{DisplayName: c.Buyer.DisplayName, Avatar: c.Buyer.Avatar}}}
	case c.Review != nil:
		x := c.Review
		return &adminpb.CaseSnapshot{Content: &adminpb.CaseSnapshot_Review{Review: &adminpb.ReviewSnapshot{
			AuthorId: rpc.FormatID(x.AuthorID), StoreId: rpc.FormatID(x.SubjectStoreID), ProductId: rpc.FormatID(x.ProductID), Rating: int32(x.Rating), //nolint:gosec // 評價分數是 1 到 5
			Body: x.Body, AssetIds: x.MediaRefs, AuthoredTime: optionalTime(x.AuthoredAt), ReplyBody: x.ReplyBody,
		}}}
	case c.ReviewReply != nil:
		x := c.ReviewReply
		return &adminpb.CaseSnapshot{Content: &adminpb.CaseSnapshot_ReviewReply{ReviewReply: &adminpb.ReviewReplySnapshot{
			StoreId: rpc.FormatID(x.StoreID), ReviewId: x.ReviewID, ReviewerId: rpc.FormatID(x.ReviewerID), Body: x.Body, AuthoredTime: optionalTime(x.AuthoredAt),
		}}}
	case c.Chat != nil:
		x := c.Chat
		out := &adminpb.ChatSnapshot{
			AuthorKind: x.AuthorKind, AuthorId: rpc.FormatID(x.AuthorID), Body: x.Body, AssetIds: x.MediaRefs, AuthoredTime: optionalTime(x.AuthoredAt),
			ConversationId: x.ConversationID, Seq: x.Seq, Context: make([]*adminpb.ChatContextMessage, len(x.Context)),
		}
		for i, m := range x.Context {
			out.Context[i] = &adminpb.ChatContextMessage{
				Seq: m.Seq, SenderKind: m.SenderKind, SenderId: rpc.FormatID(m.SenderID), Type: m.Type, Body: m.Body, AssetIds: m.MediaRefs,
				SendTime: timestamppb.New(m.SentAt), Hidden: m.Hidden, Erased: m.Erased,
			}
		}
		return &adminpb.CaseSnapshot{Content: &adminpb.CaseSnapshot_Chat{Chat: out}}
	}
	return nil
}

func optionalTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
