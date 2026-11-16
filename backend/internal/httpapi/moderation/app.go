// Package moderation 是 ModerationAppService 的 handler：登入的買家送出檢舉與讀自己的檢舉。
//
// 檢舉人一律是呼叫者的買家身分（由伺服器推導，請求不帶）；管理員等非顧客帳號的身分回 permission_denied。回應不露案件 ID（A7-D28 第 4 點）。
// 停權的帳號可以讀自己的檢舉、不能送出（動作性質宣告在 internal/httpapi/effects.go）。
package moderation

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Reports 是檢舉的來源（*moderation.Module）。
type Reports interface {
	SubmitReport(ctx context.Context, in moderation.SubmitInput) (moderation.SubmitResult, error)
	MyReports(ctx context.Context, buyerID int64, after *moderation.ReportCursor, size int32) (moderation.ReportPage, error)
}

// Buyers 把登入的帳號對到買家身分（*buyer.Module）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
}

// App 實作 ModerationAppService。
type App struct {
	reports Reports
	buyers  Buyers
}

var _ moderationv1alpha1connect.ModerationAppServiceHandler = (*App)(nil)

// NewApp 建立 handler。
func NewApp(reports Reports, buyers Buyers) *App { return &App{reports: reports, buyers: buyers} }

var submitPaths = rpc.FieldPaths{
	"Target": "target", "Category": "category", "Description": "description", "RequestID": "request_id",
}

// target 把契約的對象轉成模組的 Target；種類沒給或 ID 的格式不對是呼叫方的錯（invalid_argument，指向 target 的欄位）。
func target(t *pb.ReportTarget) (moderation.Target, error) {
	kind := rpc.EnumFromProto[moderation.TargetKind](t.GetKind())
	if kind == "" {
		return moderation.Target{}, rpc.RequiredField("target.kind", "target.kind 必須是有效的對象種類")
	}
	if kind == moderation.TargetBuyer {
		id, err := rpc.ParseID("target.id", t.GetId())
		return moderation.Target{Kind: kind, BuyerID: id}, err
	}
	id, err := rpc.ParseUUID("target.id", t.GetId())
	return moderation.Target{Kind: kind, ContentID: id}, err
}

func (h *App) SubmitReport(ctx context.Context, req *connect.Request[pb.SubmitReportRequest]) (*connect.Response[pb.SubmitReportResponse], error) {
	buyerID, err := rpc.AccountBuyerID(ctx, h.buyers)
	if err != nil {
		return nil, err
	}
	t, err := target(req.Msg.GetTarget())
	if err != nil {
		return nil, err
	}
	requestID, err := rpc.ParseRequestID(req.Msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	res, err := h.reports.SubmitReport(ctx, moderation.SubmitInput{
		ReporterBuyerID: buyerID,
		Target:          t,
		Category:        rpc.EnumFromProto[policy.Category](req.Msg.GetCategory()),
		Description:     req.Msg.GetDescription(),
		RequestID:       requestID,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, submitPaths)
	}
	return connect.NewResponse(&pb.SubmitReportResponse{
		ReportId:   res.Report.ID.String(),
		CreateTime: timestamppb.New(res.Report.CreatedAt),
	}), nil
}

func (h *App) ListMyReports(ctx context.Context, req *connect.Request[pb.ListMyReportsRequest]) (*connect.Response[pb.ListMyReportsResponse], error) {
	buyerID, err := rpc.AccountBuyerID(ctx, h.buyers)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(moderationv1alpha1connect.ModerationAppServiceListMyReportsProcedure, buyerID)
	after, err := rpc.ValidCursor[moderation.ReportCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.reports.MyReports(ctx, buyerID, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	out := &pb.ListMyReportsResponse{Reports: make([]*pb.ReportRecord, len(page.Items))}
	for i, it := range page.Items {
		id := it.Target.ContentID.String()
		if it.Target.Kind == moderation.TargetBuyer {
			id = rpc.FormatID(it.Target.BuyerID)
		}
		rec := &pb.ReportRecord{
			Id:          it.Report.ID.String(),
			Target:      &pb.ReportTarget{Kind: rpc.EnumToProto[pb.ReportTargetKind](it.Target.Kind), Id: id},
			Category:    rpc.EnumToProto[commonpb.PolicyCategory](it.Report.Category),
			Description: it.Report.Description,
			Outcome:     rpc.EnumToProto[pb.ReportOutcome](it.Report.Outcome),
			CreateTime:  timestamppb.New(it.Report.CreatedAt),
		}
		if it.ClosedAt != nil {
			rec.ResolveTime = timestamppb.New(*it.ClosedAt)
		}
		out.Reports[i] = rec
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}
